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

	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/httpserver"
	"github.com/MirzaDgtu/PromoGo/internal/logger"
	"github.com/MirzaDgtu/PromoGo/internal/notification/logchannel"
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

// New constructs an App and its dependencies: it connects to Postgres and
// Redis, builds the repositories and services, and configures the HTTP
// server.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log := logger.New(cfg.Logger)

	pgPool, err := postgres.NewPool(ctx, cfg.Postgres.DSN(), cfg.Postgres.MaxConns)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
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

	// TODO(add-notification-channel): swap for a real FCM/SMS channel once
	// cfg.FCM.CredentialsJSON is set; logchannel just logs in the meantime,
	// so accrual/redemption flows are fully testable before push is wired up.
	notifier := logchannel.New(log)

	loyaltyService := service.New(log, clientRepo, txRepo, balanceRepo, ledgerRepo, configRepo, notifier)

	httpServer := httpserver.New(httpserver.Deps{
		App:  cfg.App,
		HTTP: cfg.HTTP,
		Log:  log,

		Stores:   storeRepo,
		Clients:  clientRepo,
		Balances: balanceRepo,

		Loyalty: loyaltyService,

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
