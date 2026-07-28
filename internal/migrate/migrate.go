// Package migrate applies PromoGo's embedded goose SQL migrations against
// Postgres at startup, so a successful app.New (and therefore a healthy
// /readyz) implies the schema is actually current — see internal/app/app.go.
package migrate

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/MirzaDgtu/PromoGo/migrations"
)

// Run applies all pending migrations embedded in the migrations package
// against the Postgres database at dsn. It opens and closes its own
// database/sql connection — goose's library API operates over *sql.DB, not
// the application's pgxpool.Pool.
func Run(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
