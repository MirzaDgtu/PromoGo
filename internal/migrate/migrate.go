// Package migrate applies PromoGo's embedded goose SQL migrations against
// Postgres. Run is the historical startup-coupled path (still the default —
// see internal/app/app.go and AppConfig.SkipStartupMigrations); Verify is
// the read-only counterpart a replica uses instead when migrations are
// applied as a separate deployment step (`promogo migrate`, see
// cmd/promogo/migrate.go) — expand/migrate/contract deployments need the
// schema change to land once, out of band, before any replica serving the
// new code starts, not re-applied redundantly by every replica.
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

// Verify reports an error if the database at dsn has any pending migration
// this binary knows about — i.e. the schema was not brought current by a
// separate `promogo migrate` step before this replica started. It takes no
// lock and applies nothing; a replica that fails this check should fail
// startup (and therefore /readyz) rather than serve traffic against a
// schema it doesn't match, the same safety Run's callers relied on when
// migrations were applied inline.
func Verify(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	defer provider.Close()

	pending, err := provider.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("check pending migrations: %w", err)
	}
	if pending {
		return fmt.Errorf("database schema is behind: pending migrations exist (run `promogo migrate` first)")
	}

	return nil
}
