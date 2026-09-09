package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/migrate"
)

// runMigrate applies pending schema migrations and exits — the dedicated
// deployment step a multi-replica production deployment runs once, out of
// band, before rolling out a new version's replicas (see AppConfig.
// SkipStartupMigrations and docs/deployment.md). It shares migrate.Run with
// the historical startup-coupled path (docker-compose's default), so the
// two never apply migrations differently.
func runMigrate() {
	cfg, err := config.Load(resolveConfigPath())
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := migrate.Run(ctx, cfg.Postgres.DSN()); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}
	log.Println("migrations applied")
}
