# Backup, restore, RPO/RTO

**Status: proposed targets, needs business sign-off** — same caveat as
`docs/slo.md`: these are engineering-judgment defaults for a single-pilot
MVP, not a committed contract. `knowledge/Project Questions.md` should
track this as open until a business owner confirms the targets and the
retention numbers (which have a compliance angle — 152-FZ — this document
does not attempt to resolve; see `docs/audit-remediation-prompt.md` Phase
5's legal-minimum item).

## What actually needs backing up

**Postgres holds every durable record**: stores, organizations, clients,
customer accounts, transactions/balances, loyalty configs + history,
staff/RBAC, store API keys, audit events, customer devices, customer
sessions (refresh tokens), customer consents, the notification outbox. A
Postgres backup covers all of it.

**Redis holds only ephemeral, TTL'd state**: OTP challenges, OTP/login rate
limit counters, QR one-time tokens, the distributed rate limiter's request
counters (`internal/ratelimit`). None of it is durable business data — a
lost Redis instance means in-flight OTP codes and unconsumed QR tokens are
gone (customer re-requests one) and rate-limit counters reset (briefly more
permissive, not a data-loss event). **Redis does not need a backup/restore
story** — its recovery story is "start a new empty instance", which is
already exactly what `docs/runbooks.md` #2 describes.

## RPO/RTO targets (proposed)

| | Target | Why |
|---|---|---|
| RPO (Postgres) | **24 hours** | A daily `pg_dump` is the minimum viable backup for a single-pilot MVP with no managed-Postgres PITR. Tightening this (continuous WAL archiving via `pgBackRest`/`wal-g`, or a managed Postgres offering point-in-time recovery) is a real infrastructure investment appropriate once there's more than one pilot store's worth of data at stake — not before. |
| RTO (Postgres) | **< 30 minutes** | Time from "restore starts" to "a fresh instance is serving traffic with the migrated schema, verified against the dump's row counts". The drill below measured actual restore time on this developer's machine — see the recorded numbers — as a data point, not a promise about production hardware. |
| Retention | **30 days** of daily dumps, pending the Phase 5 legal-minimum item for how long transaction/audit history must be retained for compliance (152-FZ) rather than operational-restore purposes — those are two different retention clocks and this document only sets the operational one. |

## Backup procedure

```bash
# Logical backup (schema + data), custom format (supports parallel restore,
# selective table restore, and is what pg_restore expects):
pg_dump -Fc --no-owner --no-privileges \
  -h "$PGHOST" -U "$PGUSER" -d "$PGDATABASE" \
  -f "promogo-$(date +%Y%m%d-%H%M%S).dump"
```

Automate this daily (cron, a CI/CD scheduled pipeline, or the managed
database provider's own snapshot feature if hosted there) and ship the
resulting file off the database host — a backup that lives only on the
machine it backs up isn't a backup.

## Restore procedure

```bash
# Against a fresh, empty Postgres instance (never the live one — restore
# to a new target, verify, then cut over):
pg_restore -h "$NEW_PGHOST" -U "$PGUSER" -d "$PGDATABASE" \
  --no-owner --no-privileges -j 4 promogo-<timestamp>.dump

# Then let the app's own migration check confirm the schema is what this
# binary expects (this also validates that a restore from an older dump,
# taken before the current version's migrations existed, is caught rather
# than silently served against):
promogo migrate    # or, with SkipStartupMigrations, promogo itself refuses
                    # to start until this has run — see docs/deployment.md
```

## Restore drill — actually performed, not just described

A backup nobody has ever restored is a hope, not a backup. This was run
for real (throwaway Postgres containers, no shared infrastructure at risk):

```bash
# 1. Seed a source database with real data through the actual app (not
#    hand-written SQL), so the drill restores what a real deployment would
#    actually contain.
docker run -d --name drill-pg-src -e POSTGRES_USER=promogo \
  -e POSTGRES_PASSWORD=promogo -e POSTGRES_DB=promogo -p 15432:5432 \
  postgres:16-alpine
go build -o /tmp/promogo ./cmd/promogo
PROMOGO_POSTGRES_HOST=localhost PROMOGO_POSTGRES_PORT=15432 \
  PROMOGO_POSTGRES_PASSWORD=promogo /tmp/promogo migrate
PROMOGO_POSTGRES_HOST=localhost PROMOGO_POSTGRES_PORT=15432 \
  PROMOGO_POSTGRES_PASSWORD=promogo /tmp/promogo loadtest-seed
# ...then a few real transactions via curl against a running instance.

# 2. Back it up.
time pg_dump -h localhost -p 15432 -U promogo -Fc -f /tmp/drill.dump promogo

# 3. Restore into a brand new, empty instance.
docker run -d --name drill-pg-dst -e POSTGRES_USER=promogo \
  -e POSTGRES_PASSWORD=promogo -e POSTGRES_DB=promogo -p 15433:5432 \
  postgres:16-alpine
time pg_restore -h localhost -p 15433 -U promogo -d promogo \
  --no-owner --no-privileges /tmp/drill.dump

# 4. Verify: row counts match between source and restored target for every
#    table that matters, and the app itself starts cleanly against the
#    restored database and serves a real request.
```

**Results from the actual run** (2026-09-09, this developer's machine,
Docker Desktop, `postgres:16.10-alpine` — not production hardware, see
`docs/load-testing.md`'s same caveat), source data seeded through the real
app (`loadtest-seed` + 5 accruals + 1 refund, exercising the ledger and the
transactional outbox, not synthetic SQL):

| Step | Duration | Result |
|---|---|---|
| `pg_dump -Fc` (1 store, 5 clients, 6 transactions, 5 balances, 6 outbox rows, 2 orgs) | **0.08s**, 60KB dump | succeeded |
| `pg_restore` into a brand new, empty instance | **0.51s** | succeeded, schema + data present |
| Row-count verification (`stores`, `clients`, `transactions`, `balances`, `notification_outbox`, `organizations`) | — | **matched source exactly**, all 6 tables |
| App startup against the restored database (`promogo migrate` → `promogo` with `SkipStartupMigrations=true` → `/readyz`) | — | **200**, no manual intervention; `GET /clients/1/balance` returned the correct post-refund balance (0) from the restored data |

At pilot scale (one store, a handful of megabytes of data), both `pg_dump`
and `pg_restore` complete in low single-digit seconds — nowhere near the
30-minute RTO target. That target exists for when this scales past a
single pilot store's data volume; re-run this drill against a realistic
multi-store data volume before relying on the same number at that scale.

## What this drill does not cover

- **Point-in-time recovery** — the drill restores the latest dump, not "to
  a specific moment between backups". PITR requires WAL archiving, which
  this deployment doesn't have yet (see the RPO row above).
- **Restoring under load / during an incident** — the drill restores to an
  idle, empty instance. A real incident restore competes for I/O with
  whatever else is happening operationally at the time.
- **Redis** — deliberately out of scope; see "What actually needs backing
  up" above.
