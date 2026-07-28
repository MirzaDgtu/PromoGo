// Package migrate applies PromoGo's embedded goose SQL migrations against
// Postgres at startup, so a successful app.New (and therefore a healthy
// /readyz) implies the schema is actually current — see internal/app/app.go.
package migrate

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/MirzaDgtu/PromoGo/migrations"
)

// Run applies all pending migrations embedded in the migrations package
// against the Postgres database at dsn. It opens and closes its own
// database/sql connection — goose's library API operates over *sql.DB, not
// the application's pgxpool.Pool.
//
// It takes a Postgres session-level advisory lock for the duration of the
// run (lock.NewPostgresSessionLocker), so multiple app instances starting
// concurrently apply the schema exactly once instead of racing on the same
// DDL — a plain goose.Up call has no such guard.
func Run(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		db.Close()
		return fmt.Errorf("create migration lock: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithSessionLocker(locker))
	if err != nil {
		db.Close()
		return fmt.Errorf("create migration provider: %w", err)
	}
	defer provider.Close()

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
