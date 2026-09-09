// Package app wires together configuration, infrastructure clients, and
// services, and owns their lifecycle.
package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/httpserver"
	"github.com/MirzaDgtu/PromoGo/internal/logger"
	"github.com/MirzaDgtu/PromoGo/internal/migrate"
	"github.com/MirzaDgtu/PromoGo/internal/notification/fcmchannel"
	"github.com/MirzaDgtu/PromoGo/internal/notification/httpsms"
	"github.com/MirzaDgtu/PromoGo/internal/notification/logchannel"
	"github.com/MirzaDgtu/PromoGo/internal/notification/logsms"
	"github.com/MirzaDgtu/PromoGo/internal/ratelimit"
	"github.com/MirzaDgtu/PromoGo/internal/repository/postgres"
	"github.com/MirzaDgtu/PromoGo/internal/service"
)

// App wires together configuration, infrastructure clients, and services,
// and owns their lifecycle. Callers must call Close when New succeeds,
// regardless of whether Run is called.
type App struct {
	log          *slog.Logger
	pgPool       *pgxpool.Pool
	redis        *redis.Client
	http         *http.Server
	outboxWorker *service.OutboxWorker
	background   *httpserver.BackgroundTracker
}

// New constructs an App and its dependencies: it connects to Postgres,
// applies pending migrations (see internal/migrate), connects to Redis,
// builds the repositories and services, and configures the HTTP server. A
// successful return means the schema is current, so /readyz reporting
// healthy is meaningful.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log := logger.New(cfg.Logger)

	trustedProxies, err := httpserver.ParseTrustedProxies(cfg.HTTP.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("parse http.trusted_proxies: %w", err)
	}

	pgPool, err := postgres.NewPool(ctx, cfg.Postgres.DSN(), cfg.Postgres.MaxConns)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := migrate.Run(ctx, cfg.Postgres.DSN()); err != nil {
		pgPool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	redisOpts := &redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB}
	if cfg.Redis.TLSEnabled {
		redisOpts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	redisClient := redis.NewClient(redisOpts)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		pgPool.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	storeRepo := postgres.NewStoreRepository(pgPool)
	clientRepo := postgres.NewClientRepository(pgPool)
	balanceRepo := postgres.NewBalanceRepository(pgPool)
	txRepo := postgres.NewTransactionRepository(pgPool)
	ledgerRepo := postgres.NewLedgerRepository(pgPool)
	configRepo := postgres.NewLoyaltyConfigRepository(pgPool)
	outboxRepo := postgres.NewNotificationOutboxRepository(pgPool)

	orgRepo := postgres.NewOrganizationRepository(pgPool)
	customerAccountRepo := postgres.NewCustomerAccountRepository(pgPool)
	customerSessionRepo := postgres.NewCustomerSessionRepository(pgPool)
	customerConsentRepo := postgres.NewCustomerConsentRepository(pgPool)
	staffUserRepo := postgres.NewStaffUserRepository(pgPool)
	staffMembershipRepo := postgres.NewStaffMembershipRepository(pgPool)
	storeAPIKeyRepo := postgres.NewStoreAPIKeyRepository(pgPool)
	auditEventRepo := postgres.NewAuditEventRepository(pgPool)
	customerDeviceRepo := postgres.NewCustomerDeviceRepository(pgPool)

	// SMS: config.validateSMS already enforced Provider is a recognized
	// value in every environment, and (outside development) that it isn't
	// "log" and that "http" has a valid endpoint/token set — so an unknown
	// or misconfigured value is always a startup error above, never a
	// silent fallback to the dev stub here. See DEC-014.
	var smsSender domain.SMSSender
	if cfg.SMS.Provider == "http" {
		smsSender = httpsms.New(cfg.SMS, log)
	} else {
		smsSender = logsms.New(log)
	}

	// FCM: config.validateFCM already enforced (outside development) that
	// CredentialsJSON is set, so a construction failure here is always a
	// genuine startup error, never a silent fallback to logchannel in
	// production — see DEC-014.
	var notifier domain.NotificationChannel
	if cfg.FCM.CredentialsJSON != "" {
		fcmNotifier, err := fcmchannel.New(ctx, cfg.FCM.CredentialsJSON, clientRepo, customerDeviceRepo, log)
		if err != nil {
			pgPool.Close()
			return nil, fmt.Errorf("init fcm channel: %w", err)
		}
		notifier = fcmNotifier
	} else {
		notifier = logchannel.New(log)
	}

	loyaltyService := service.New(log, clientRepo, txRepo, balanceRepo, ledgerRepo, configRepo, service.AntiFraudConfig{
		DailyRedeemPointsLimit: cfg.AntiFraud.DailyRedeemPointsLimit,
		DailyRedeemWindow:      cfg.AntiFraud.DailyRedeemWindow,
	})

	// Accrual/redeem/refund notifications are queued transactionally by
	// ledgerRepo (see domain.LedgerRepository's doc comment) and delivered
	// here, off the request path — DEC-014's outbox, now implemented (see
	// docs/audit-remediation-prompt.md Phase 3). The worker outlives any
	// single request; Run starts/stops it alongside the HTTP server (see
	// App.Run/Close).
	outboxWorker := service.NewOutboxWorker(log, outboxRepo, notifier, service.OutboxWorkerConfig{
		PollInterval:   2 * time.Second,
		BatchSize:      50,
		MaxConcurrency: 10,
		MaxAttempts:    5,
		BaseBackoff:    5 * time.Second,
		MaxBackoff:     5 * time.Minute,
		// Kept below App.Run's outboxDrainTimeout so a poll in flight at
		// shutdown is normally bounded by this, not that.
		DeliverTimeout: 30 * time.Second,
	})

	qrService := service.NewQRService(log, redisClient, service.QRConfig{
		TTL:             cfg.AntiFraud.QRTTL,
		IssueCooldown:   cfg.AntiFraud.QRIssueCooldown,
		ConsumeCooldown: cfg.AntiFraud.QRConsumeCooldown,
	}, clientRepo, customerAccountRepo, balanceRepo, auditEventRepo)

	accessTokenSecret := []byte(cfg.Auth.AccessTokenSecret)
	customerAuthService := service.NewCustomerAuthService(
		log, customerAccountRepo, customerSessionRepo, customerConsentRepo, clientRepo, auditEventRepo, smsSender, redisClient,
		service.CustomerAuthConfig{
			AccessTokenSecret: accessTokenSecret,
			AccessTokenTTL:    cfg.Auth.AccessTokenTTL,
			RefreshTokenTTL:   cfg.Auth.RefreshTokenTTL,
			OTP: service.OTPConfig{
				CodeTTL:             cfg.Auth.OTPTTL,
				ResendCooldown:      cfg.Auth.OTPResendCooldown,
				MaxAttempts:         cfg.Auth.OTPMaxAttempts,
				RateLimitWindow:     cfg.Auth.OTPRateLimitWindow,
				MaxRequestsPerPhone: cfg.Auth.OTPMaxRequestsPerPhone,
				MaxRequestsPerIP:    cfg.Auth.OTPMaxRequestsPerIP,
			},
		},
	)

	oidcVerifier := auth.NewOIDCVerifier(cfg.OIDC.IssuerURL, cfg.OIDC.Audience, cfg.OIDC.JWKSURL, cfg.OIDC.JWKSCacheTTL)
	staffAuthService := service.NewStaffAuthService(
		log, staffUserRepo, staffMembershipRepo, auditEventRepo, oidcVerifier,
		service.StaffAuthConfig{AccessTokenSecret: accessTokenSecret, AccessTokenTTL: cfg.Auth.AccessTokenTTL},
	)

	background := httpserver.NewBackgroundTracker()

	httpServer := httpserver.New(httpserver.Deps{
		App:  cfg.App,
		HTTP: cfg.HTTP,
		Log:  log,

		Stores:         storeRepo,
		Clients:        clientRepo,
		Balances:       balanceRepo,
		Transactions:   txRepo,
		LoyaltyConfigs: configRepo,
		StoreAPIKeys:   storeAPIKeyRepo,

		Organizations:    orgRepo,
		CustomerAccounts: customerAccountRepo,
		CustomerDevices:  customerDeviceRepo,
		StaffUsers:       staffUserRepo,
		StaffMemberships: staffMembershipRepo,
		AuditEvents:      auditEventRepo,

		Loyalty:      loyaltyService,
		CustomerAuth: customerAuthService,
		StaffAuth:    staffAuthService,
		QR:           qrService,

		CustomerAccessTokenSecret: accessTokenSecret,
		StaffAccessTokenSecret:    accessTokenSecret,

		RateLimiter:    ratelimit.New(redisClient),
		RateLimit:      cfg.RateLimit,
		TrustedProxies: trustedProxies,

		Ready: func(ctx context.Context) error {
			if err := pgPool.Ping(ctx); err != nil {
				return fmt.Errorf("postgres: %w", err)
			}
			if err := redisClient.Ping(ctx).Err(); err != nil {
				return fmt.Errorf("redis: %w", err)
			}
			return nil
		},

		Background: background,
	})

	return &App{log: log, pgPool: pgPool, redis: redisClient, http: httpServer, outboxWorker: outboxWorker, background: background}, nil
}

// httpShutdownTimeout bounds how long a graceful HTTP shutdown waits for
// in-flight requests to finish before forcibly closing their connections.
// 1C's webhook SLA target is <300ms (Idea.md); this is a generous multiple
// of that, not a budget any healthy request should need.
const httpShutdownTimeout = 5 * time.Second

// outboxDrainTimeout bounds how long Run waits, after stopping the outbox
// worker's ticker, for a poll already in flight to finish delivering and
// marking its claimed batch. Kept above OutboxWorkerConfig.DeliverTimeout
// (see internal/service/outbox_worker.go) so that bound — not this one — is
// normally what limits an in-flight poll.
const outboxDrainTimeout = 35 * time.Second

// backgroundDrainTimeout bounds how long Run waits for tracked best-effort
// goroutines (see httpserver.BackgroundTracker) to finish after the HTTP
// server itself has stopped accepting new requests.
const backgroundDrainTimeout = 5 * time.Second

// Run starts the HTTP server and the notification outbox worker, blocking
// until ctx is canceled or the HTTP server fails.
//
// Shutdown order matters: the HTTP server stops accepting new requests and
// drains in-flight ones first, then the outbox worker's ticker is stopped
// and any poll it already had in flight is given a bounded window to finish
// (see OutboxWorker.Run), then tracked background goroutines (e.g.
// TouchLastUsed) get their own bounded window — all before Run returns, so
// Close (which closes Postgres/Redis) never races work that's still using
// them.
func (a *App) Run(ctx context.Context) error {
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		a.outboxWorker.Run(workerCtx)
	}()

	errCh := make(chan error, 1)

	go func() {
		a.log.Info("http server listening", "addr", a.http.Addr)
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
			return
		}
		errCh <- nil
	}()

	var runErr error
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
		if err := a.http.Shutdown(shutdownCtx); err != nil {
			a.log.Error("http server shutdown", "error", err)
		}
		cancel()
	case err := <-errCh:
		runErr = err
	}

	cancelWorker()
	select {
	case <-workerDone:
	case <-time.After(outboxDrainTimeout):
		a.log.Warn("outbox worker did not stop within drain timeout; proceeding with shutdown")
	}

	bgCtx, bgCancel := context.WithTimeout(context.Background(), backgroundDrainTimeout)
	a.background.Wait(bgCtx)
	bgCancel()

	return runErr
}

// Close releases infrastructure resources. It must be called once after a
// successful call to New.
func (a *App) Close() {
	a.pgPool.Close()
	if err := a.redis.Close(); err != nil {
		a.log.Error("close redis", "error", err)
	}
}
