// Package app wires together configuration, infrastructure clients, and
// services, and owns their lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/httpserver"
	"github.com/MirzaDgtu/PromoGo/internal/logger"
	"github.com/MirzaDgtu/PromoGo/internal/migrate"
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
	log    *slog.Logger
	pgPool *pgxpool.Pool
	redis  *redis.Client
	http   *http.Server
}

// New constructs an App and its dependencies: it connects to Postgres,
// applies pending migrations (see internal/migrate), connects to Redis,
// builds the repositories and services, and configures the HTTP server. A
// successful return means the schema is current, so /readyz reporting
// healthy is meaningful.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log := logger.New(cfg.Logger)

	pgPool, err := postgres.NewPool(ctx, cfg.Postgres.DSN(), cfg.Postgres.MaxConns)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := migrate.Run(ctx, cfg.Postgres.DSN()); err != nil {
		pgPool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
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

	orgRepo := postgres.NewOrganizationRepository(pgPool)
	customerAccountRepo := postgres.NewCustomerAccountRepository(pgPool)
	customerSessionRepo := postgres.NewCustomerSessionRepository(pgPool)
	customerConsentRepo := postgres.NewCustomerConsentRepository(pgPool)
	staffUserRepo := postgres.NewStaffUserRepository(pgPool)
	staffMembershipRepo := postgres.NewStaffMembershipRepository(pgPool)
	storeAPIKeyRepo := postgres.NewStoreAPIKeyRepository(pgPool)
	auditEventRepo := postgres.NewAuditEventRepository(pgPool)

	// TODO(add-notification-channel): swap for a real FCM/SMS channel once
	// cfg.FCM.CredentialsJSON is set; logchannel/logsms just log in the
	// meantime, so device-facing flows are fully testable before push/SMS
	// is wired up.
	notifier := logchannel.New(log)
	smsSender := logsms.New(log)

	loyaltyService := service.New(log, clientRepo, txRepo, balanceRepo, ledgerRepo, configRepo, notifier)

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
		StaffUsers:       staffUserRepo,
		StaffMemberships: staffMembershipRepo,
		AuditEvents:      auditEventRepo,

		Loyalty:      loyaltyService,
		CustomerAuth: customerAuthService,
		StaffAuth:    staffAuthService,

		CustomerAccessTokenSecret: accessTokenSecret,
		StaffAccessTokenSecret:    accessTokenSecret,

		RateLimiter:    ratelimit.New(redisClient),
		RateLimit:      cfg.RateLimit,
		TrustedProxies: httpserver.ParseTrustedProxies(cfg.HTTP.TrustedProxies),

		Ready: func(ctx context.Context) error {
			if err := pgPool.Ping(ctx); err != nil {
				return fmt.Errorf("postgres: %w", err)
			}
			if err := redisClient.Ping(ctx).Err(); err != nil {
				return fmt.Errorf("redis: %w", err)
			}
			return nil
		},
	})

	return &App{log: log, pgPool: pgPool, redis: redisClient, http: httpServer}, nil
}

// Run starts the HTTP server, blocking until ctx is canceled or the HTTP
// server fails.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		a.log.Info("http server listening", "addr", a.http.Addr)
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := a.http.Shutdown(shutdownCtx); err != nil {
			a.log.Error("http server shutdown", "error", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

// Close releases infrastructure resources. It must be called once after a
// successful call to New.
func (a *App) Close() {
	a.pgPool.Close()
	if err := a.redis.Close(); err != nil {
		a.log.Error("close redis", "error", err)
	}
}
