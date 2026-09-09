# Pilot end-to-end test

`internal/e2e/pilot_test.go`'s `TestPilotEndToEnd` proves the full pilot
chain against a real, running app instance backed by a real Postgres and
Redis — not fakes (contrast `internal/httpserver/testsupport_test.go`,
which is thorough at the route level but never touches real
infrastructure). It's the concrete answer to
`docs/audit-remediation-prompt.md` Phase 5's "build a real pilot E2E test
covering 1C/POS event, accrual, mobile visibility, QR/phone identification,
redemption, offline retry and duplicate delivery" and to
`knowledge/Project Questions.md`'s `Q-P0-095`.

## What it proves, in order

1. **1C/POS accrual webhook** (`POST /api/v1/transactions`) — a purchase
   event identifies the client by phone (no prior registration needed).
2. **Offline retry / duplicate delivery** — the exact same webhook
   payload (same `transaction_id`) resent immediately returns
   `replayed: true` and the same balance, not a double accrual.
3. **Mobile OTP login** — `POST /api/v1/auth/otp/request` +
   `POST /api/v1/auth/otp/verify`. The OTP code itself is never
   observable through the API (Redis stores only its hash — see
   `internal/service/otp_store.go` — and `internal/notification/logsms`
   deliberately never logs it), so the test substitutes a capturing
   `domain.SMSSender` via `app.WithSMSSender` (test-only; production code
   never passes this option).
4. **Mobile visibility** — `GET /api/v1/me/balance`, right after login,
   already shows the accrual the store posted before the customer ever
   logged in (proves `linkExistingClients`' cross-session linking works
   end to end, not just at the service-layer unit-test level).
5. **QR identification at a second POS interaction** —
   `POST /api/v1/me/qr` (mobile) then `POST /api/v1/clients/resolve-qr`
   (store), confirming the same client/balance the accrual established.
6. **Redemption**, identified via the QR-resolved `client_id`.
7. **Mobile transaction history** reflects both prior operations.
8. **Refund** of the original accrual (allowed to drive the balance
   negative — DEC-012), plus its own duplicate-delivery replay check.
9. **Mobile visibility of the refund.**

## Running it locally

Needs a real, reachable Postgres and Redis — the test skips itself (not
fails) if it can't reach them, so it's safe to run `go test -tags e2e ./...`
without a stack up; you just won't get coverage.

```bash
docker run -d --name e2e-pg -e POSTGRES_USER=promogo -e POSTGRES_PASSWORD=promogo \
  -e POSTGRES_DB=promogo -p 25434:5432 postgres:16-alpine
docker run -d --name e2e-redis -p 26381:6379 redis:7-alpine

PROMOGO_POSTGRES_HOST=127.0.0.1 PROMOGO_POSTGRES_PORT=25434 \
PROMOGO_POSTGRES_USER=promogo PROMOGO_POSTGRES_PASSWORD=promogo PROMOGO_POSTGRES_DBNAME=promogo \
PROMOGO_REDIS_ADDR=127.0.0.1:26381 \
PROMOGO_HTTP_HOST=127.0.0.1 PROMOGO_HTTP_PORT=18097 \
go test -tags e2e -run TestPilotEndToEnd -v -timeout 90s ./internal/e2e/...

docker rm -f e2e-pg e2e-redis
```

Ports above are arbitrary/throwaway — pick any free ones, they don't need
to avoid colliding with `deployments/docker-compose.yml`'s ports since
this test creates its own throwaway Postgres/Redis containers, not the
dev stack.

## CI

`.github/workflows/ci.yml`'s `e2e-pilot` job runs this on every push,
using real Postgres/Redis service containers (same pattern as the
`migration-deploy-model` and `load-test.yml` jobs) — unlike
`loadtest/*.js`, this isn't gated behind `workflow_dispatch`: it's a
correctness proof, not a performance benchmark, and finishes in about a
second against the seeded data (excluding container startup).

## What this test seeds vs. what it doesn't

Seeds a throwaway organization/store/API-key/loyalty-config directly
against Postgres (the same bypass-HTTP pattern as
`cmd/promogo/loadtest_seed.go`) — never run it against anything but a
disposable database. It does not exercise real FCM push delivery (uses
the default `logchannel` stub, since `environment: development` is the
config default and no Firebase credentials are supplied) — push delivery
itself is covered separately by `internal/notification/fcmchannel`'s unit
tests and DEC-015's outbox design.
