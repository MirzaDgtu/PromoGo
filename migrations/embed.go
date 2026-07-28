// Package migrations embeds PromoGo's goose SQL migration files so
// internal/migrate can apply them at application startup without the
// migrations directory needing to exist on disk at runtime. The .sql files
// remain the single source of truth for both this embed and the goose CLI
// (see the Makefile's migrate-* targets).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
