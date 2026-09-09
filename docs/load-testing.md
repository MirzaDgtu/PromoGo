# Load, soak, and dependency-failure testing

Runnable evidence for the SLOs in `docs/slo.md`, not just a claim that the
service "should" hit <300ms. Everything here is scriptable and CI-runnable;
see the end of this document for the recorded baseline run.

## Tools

- [k6](https://k6.io) (`grafana/k6`, pinned by digest — no local install
  needed) for `load.js`/`soak.js`/`smoke.js`.
- `bash` + `docker compose` for `dependency-failure.sh`.

## 1. Start the app with load-test-scale rate limits

`load.js`/`soak.js` deliberately drive more traffic than one caller's
default rate limit allows (`ratelimit.accrual_ip_limit`/
`accrual_principal_limit` default to 300/600 per minute — 5-10 req/s — sized
for one pilot store's realistic traffic, not the 20 req/s load-test target
from `docs/slo.md`, which exists to validate scale-up headroom). A k6
container is a single caller (one IP, one store key), so at the default
limits it mostly gets 429s instead of exercising the request path. Raise
the limits for the load-test run specifically — this changes what's being
measured (backend capacity vs. rate-limiter behavior, which
`dependency-failure.sh` and the unit tests in `internal/ratelimit` already
cover), not a claim that these are the right limits for real pilot traffic:

```bash
make docker-up && make migrate-up
PROMOGO_RATELIMIT_ACCRUAL_IP_LIMIT=6000 PROMOGO_RATELIMIT_ACCRUAL_IP_WINDOW=1m \
  PROMOGO_RATELIMIT_ACCRUAL_PRINCIPAL_LIMIT=6000 PROMOGO_RATELIMIT_ACCRUAL_PRINCIPAL_WINDOW=1m \
  go run ./cmd/promogo
```

## 2. Seed a store and API key

Load tests need a working store credential. Don't hand-write SQL against a
schema that's moved on since `.claude/skills/dev-stack/SKILL.md` was
written — use the dedicated seed command instead, against a disposable
database:

```bash
go run ./cmd/promogo loadtest-seed
# {
#   "organization_id": 2,
#   "store_id": 1,
#   "api_key": "<key-id>.<secret>"  # e.g. "kid_example.secret_example_not_a_real_key"
# }
export API_KEY='<the api_key value above>'
```

Never run `loadtest-seed` against anything but a throwaway/dev/CI database
— it creates real rows with no cleanup.

## 3. Smoke test

Run this first — it catches a bad seed/config in seconds instead of
discovering it 3 minutes into `load.js`:

```bash
docker run --rm --network host -e BASE_URL=http://localhost:8080 -e API_KEY="$API_KEY" \
  -v "$PWD/loadtest:/loadtest" grafana/k6:1.1.0 run /loadtest/smoke.js
```

On Windows/Mac Docker Desktop, `--network host` doesn't reach a
`localhost`-published port the way it does on Linux (Docker Desktop runs
containers in a Linux VM) — drop `--network host` and use
`-e BASE_URL=http://host.docker.internal:8080` instead. This is how the
recorded baseline run (below) was actually executed.

## 4. Load test

```bash
docker run --rm --network host -e BASE_URL=http://localhost:8080 -e API_KEY="$API_KEY" \
  -v "$PWD/loadtest:/loadtest" grafana/k6:1.1.0 run /loadtest/load.js
```

Exits non-zero if any `docs/slo.md` threshold is violated. ~4 minutes.

## 5. Soak test

Same command, `soak.js` instead of `load.js`. ~15 minutes. Watch
`promogo_outbox_backlog` (`GET /metrics`) during the run — it should track
enqueue rate, not climb monotonically.

```bash
docker run --rm --network host -e BASE_URL=http://localhost:8080 -e API_KEY="$API_KEY" \
  -v "$PWD/loadtest:/loadtest" grafana/k6:1.1.0 run /loadtest/soak.js
```

## 6. Dependency-failure test

Brings its own docker-compose stack up (on alternate host ports so it
doesn't collide with another local project's dev stack — see the script),
seeds it, pauses Redis mid-traffic and asserts every request gets a clean
503 (docs/runbooks.md #2's fail-closed behavior — never a 500, timeout, or
panic), unpauses and asserts recovery, then repeats for Postgres, then
tears everything down:

```bash
loadtest/dependency-failure.sh
```

## Baseline run

Recorded results from the last full run of all four scripts against this
repo's own docker-compose stack (native Docker Desktop, no dedicated load
generator — treat absolute numbers as a floor, not a ceiling, on real
production-grade hardware) are in `docs/load-test-baseline.md`, alongside
the exact commit these were run against. Re-run and update that file
whenever a change plausibly affects request-path performance (ledger
locking, rate-limit rules, the outbox worker's concurrency) — a stale
baseline is worse than none, since it invites trusting a number that no
longer reflects the code.
