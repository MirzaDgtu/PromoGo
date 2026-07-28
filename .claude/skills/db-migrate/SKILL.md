---
name: db-migrate
description: Create and apply goose SQL migrations for PromoGo's PostgreSQL schema in migrations/. Use when asked to add/alter a table, column, or index, or to run/check migration status.
---

# Database migrations (goose)

Migrations live in `migrations/`, numbered sequentially: `00001_create_stores.sql`,
`00002_create_clients.sql`, etc.

## Creating a migration

1. Name the file `<next 5-digit number>_<snake_case_description>.sql`, e.g. `00006_add_tiers.sql`.
2. Use the goose annotation format:
   ```sql
   -- +goose Up
   ALTER TABLE clients ADD COLUMN ...;

   -- +goose Down
   ALTER TABLE clients DROP COLUMN ...;
   ```
   Every `Up` must have a corresponding `Down` that reverses it.
3. **Conventions to match the existing schema** (see `migrations/00004_create_transactions.sql`,
   `migrations/00005_create_loyalty_configs.sql`):
   - Money columns (purchase amounts): `NUMERIC(20, 2)`, decoded into `shopspring/decimal` via
     `pgx-shopspring-decimal` — never `float64`. Percentages: `NUMERIC(5, 2)`.
   - Point counts are a plain count, not money: `BIGINT`, not `NUMERIC`. `balances.points` also
     carries `CHECK (points >= 0)` — see the gotcha below before writing anything that upserts it.
   - IDs are `BIGSERIAL`/`BIGINT` (no UUIDs), matching every existing table.
   - Timestamps: `TIMESTAMPTZ NOT NULL DEFAULT now()`.
   - Idempotency-critical uniqueness (e.g. `(store_id, external_tx_id)` on `transactions`) is a
     real correctness requirement, not just an index — see `internal/domain/ledger.go`'s doc
     comment for why. Any new table that records an external-system event needs the same
     pattern if that event can be retried/replayed.

## Postgres gotcha: CHECK constraints + `ON CONFLICT DO UPDATE`

Found and fixed 2026-07-28 while building this scaffold (see `internal/repository/postgres/ledger_repository.go`):
`INSERT ... ON CONFLICT (id) DO UPDATE SET col = table.col + EXCLUDED.col` validates the
CHECK constraint against the **raw `EXCLUDED` values**, not the final post-update row — so
`UPDATE ... SET points = points + (-20)` on a row with plenty of balance can still spuriously
fail `CHECK (points >= 0)` if written as a single upsert statement, because Postgres checks the
literal `-20` before conflict resolution picks the UPDATE branch. If you add another
increment-in-place column with a CHECK constraint, don't use the single-statement upsert
pattern — instead: `INSERT ... ON CONFLICT DO NOTHING` to ensure the row exists, then a plain
`UPDATE ... SET col = col + $delta ...` in the same transaction (see `ledger_repository.go`'s
`Post` method for the reference implementation).

## Running migrations

```bash
make migrate-up      # apply all pending migrations
make migrate-down     # roll back one migration
make migrate-status   # show applied/pending migrations
```

These use `DATABASE_URL`, defaulting to `postgres://promogo:promogo@localhost:5432/promogo?sslmode=disable`
(matches `deployments/docker-compose.yml`'s Postgres credentials). Override with
`DATABASE_URL=... make migrate-up` for a different environment, e.g. if you started the dev
stack on an alternate port (see the dev-stack skill):
```bash
DATABASE_URL="postgres://promogo:promogo@localhost:5433/promogo?sslmode=disable" make migrate-up
```
On Windows without `make`, run the underlying goose command directly:
```bash
go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$DATABASE_URL" up
```

## After changing the schema

- Update the corresponding repository in `internal/repository/postgres` (queries, struct scans)
  to match.
- If the change affects `Transaction`/`Balance`/`LoyaltyConfig` shape, update `internal/domain`
  first, then the repository, then the migration — keep all three consistent.
- `make docker-up` starts Postgres before running migrations against local dev.
