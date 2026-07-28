// Package migrations embeds PromoGo's goose SQL migration files so
// internal/migrate can apply them at application startup without the
// migrations directory needing to exist on disk at runtime. The .sql files
// under sql/ remain the single source of truth for both this embed and the
// goose CLI (see the Makefile's migrate-* targets, pointed at migrations/sql).
//
// The .sql files live in a sql/ subdirectory, not next to this file, because
// `goose -dir <dir> validate` scans every file in its target directory
// looking for a migration version number — including .go files — so a bare
// embed.go next to the .sql files breaks the CLI. Splitting the embedded Go
// shim from the CLI-facing SQL directory keeps both working.
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed sql/*.sql
var raw embed.FS

// FS is rooted at sql/ itself (the "sql/" embed prefix is stripped), so
// goose.NewProvider(..., migrations.FS, ...) finds the .sql files directly
// instead of under a nested "sql" path.
var FS fs.FS = must(fs.Sub(raw, "sql"))

func must(f fs.FS, err error) fs.FS {
	if err != nil {
		panic(err)
	}
	return f
}
