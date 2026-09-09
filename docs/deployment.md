# Deployment: migrations as a separate step

Historically this service applied its own schema migrations on every
startup (`internal/migrate.Run`, guarded by a Postgres session advisory
lock — see its doc comment). That's fine for docker-compose's single
instance and is still the default, but it doesn't fit a multi-replica
production rollout: every replica of a new version independently attempts
the same DDL at boot, coupling a schema change to the app deploy instead of
letting it be reviewed, applied, and verified as its own step.

## The two paths

`AppConfig.SkipStartupMigrations` (`app.skip_startup_migrations` /
`PROMOGO_APP_SKIP_STARTUP_MIGRATIONS`) selects which one `app.New` takes:

- **`false` (default — docker-compose, single instance):** `internal/
  migrate.Run` applies pending migrations inline, same as before.
- **`true` (multi-replica production):** `internal/migrate.Verify` checks
  the schema has *no* pending migration and fails startup immediately if it
  does, instead of applying anything. A replica that can't confirm the
  schema matches what it expects must never serve traffic against it.

The actual migration, in the `true` path, is applied once by a separate
step: `promogo migrate` (`cmd/promogo/migrate.go`) — the same `migrate.Run`
docker-compose uses, just invoked on its own rather than embedded in `app.
New`. Run it once (a CI/CD deploy step, a Kubernetes `Job`/`initContainer`,
a one-off `docker run`/`docker compose run --rm app promogo migrate`)
*before* rolling out any replica of the new version, then start replicas
with `PROMOGO_APP_SKIP_STARTUP_MIGRATIONS=true`.

## Verified: two replicas, one already-migrated schema, no re-migration race

CI job `migration-deploy-model` (`.github/workflows/ci.yml`) runs this
exact sequence against real Postgres + Redis service containers on every
push/PR:

1. `promogo migrate` — applies the schema once.
2. Start two `promogo` processes concurrently (`SkipStartupMigrations=true`),
   both against the now-migrated database — assert both reach `/readyz`
   200. Neither attempts to apply anything (verified by them not racing
   each other or failing on the advisory lock, which `Verify` never even
   takes).
3. Roll one migration back (`goose down`) and start a fresh `promogo`
   process the same way — assert startup fails with the pending-migrations
   error instead of exiting 0 or silently serving. Then re-apply, to leave
   the job's database in the same state its other steps expect.

This was also run manually once as a local smoke test (throwaway Postgres/
Redis containers, native `go build`) before being encoded into CI, with
identical results — see the commit that added this job for the transcript.
Reproduce locally:

```bash
docker run -d --name pg -e POSTGRES_USER=promogo -e POSTGRES_PASSWORD=promogo \
  -e POSTGRES_DB=promogo -p 15432:5432 postgres:16-alpine
docker run -d --name redis -p 16379:6379 redis:7-alpine

go build -o /tmp/promogo ./cmd/promogo
export PROMOGO_POSTGRES_HOST=localhost PROMOGO_POSTGRES_PORT=15432 \
       PROMOGO_POSTGRES_USER=promogo PROMOGO_POSTGRES_PASSWORD=promogo \
       PROMOGO_POSTGRES_DBNAME=promogo PROMOGO_REDIS_ADDR=localhost:16379 \
       PROMOGO_APP_SKIP_STARTUP_MIGRATIONS=true

/tmp/promogo migrate                      # the separate deploy step
PROMOGO_HTTP_PORT=18081 /tmp/promogo &     # replica 1
PROMOGO_HTTP_PORT=18082 /tmp/promogo &     # replica 2
curl localhost:18081/readyz; curl localhost:18082/readyz   # both 200

docker rm -f pg redis
```

## Expand → migrate → contract

This repo's migrations (`migrations/sql/`) have, to date, all followed the
**expand** half of this pattern already: every column ever removed by a
migration is removed only in that migration's `-- +goose Down` (the
rollback of an `ADD COLUMN` from the same file's `Up`) — grep
`migrations/sql/*.sql` for `DROP COLUMN` and check which section it's in to
confirm. No migration has yet needed to drop a column that shipped in an
*earlier* migration, because no column has yet been deprecated while still
in use — but the day one is, the three-phase discipline below is what makes
that safe with replicas running old and new code simultaneously (which is
the normal state during any rolling deploy, and now, with migrations
decoupled per the above, is exactly the window between `promogo migrate`
landing and every replica having rolled onto the new version):

1. **Expand:** add the new column/table nullable or with a default, and/or
   backfill it — old code (unaware of it) keeps working untouched, new code
   can start writing it. This is an ordinary migration, applied via
   `promogo migrate` before any new-code replica starts.
2. **Migrate (the application-level step, not the schema one):** ship the
   application change that writes/reads the new shape, dual-writing the old
   and new column/table if a read path still depends on the old one anywhere
   (including a replica that hasn't rolled forward yet). Only once *every*
   replica is confirmed on code that no longer reads the old shape is it
   safe to proceed.
3. **Contract:** a later migration drops the old column/table, applied via
   `promogo migrate` the same way — only after step 2 is fully rolled out,
   never in the same release as the code change that stops using it.

Concretely, for the next migration that needs to remove or narrow an
in-use column: split it into an expand migration + a code release + a
contract migration, each a separate deploy, rather than one migration that
changes shape and behavior simultaneously — that's the one guarantee this
document exists to make explicit, since nothing enforces it mechanically
today.
