---
name: dev-stack
description: Start/stop PromoGo's local dev stack (Postgres + Redis via docker compose), run the backend, and verify it's healthy. Use when asked to run the app locally, test it end-to-end, or debug startup/connectivity issues.
---

# Running PromoGo locally

## Start dependencies

```bash
make docker-up    # docker compose -f deployments/docker-compose.yml up -d postgres redis
make migrate-up   # apply goose migrations (needs Postgres up first)
```

Postgres: `localhost:5432`, db/user/password all `promogo` (see `deployments/docker-compose.yml`). Redis: `localhost:6379`.

If those ports are already taken by another local project's dev stack, override the published host ports without editing the compose file:
```bash
POSTGRES_HOST_PORT=5433 REDIS_HOST_PORT=6380 docker compose -f deployments/docker-compose.yml up -d postgres redis
```
and point `PROMOGO_POSTGRES_PORT`/`PROMOGO_REDIS_ADDR` at the same alternate ports when running the app (see below).

## Seed a test store

The webhook API is store-scoped (`Authorization: Bearer <store-api-key>`, see `internal/httpserver/middleware.go`'s `RequireStoreAPIKey`). There's no signup endpoint yet — insert a store directly:
```bash
KEY="test-api-key"
HASH=$(printf '%s' "$KEY" | sha256sum | cut -d' ' -f1)
docker exec -i promogo-postgres-1 psql -U promogo -d promogo -c \
  "INSERT INTO stores (name, api_key_hash) VALUES ('Pilot Store', '$HASH') RETURNING id;"
docker exec -i promogo-postgres-1 psql -U promogo -d promogo -c \
  "INSERT INTO loyalty_configs (store_id, mechanic, accrual_percent, min_purchase_amount, min_balance_to_redeem, max_redeem_percent, points_exchange_rate)
   VALUES (1, 'points', 5.00, 100.00, 0, 50.00, 1.0000);"
```

## Run the backend

```bash
go run ./cmd/promogo      # reads configs/config.yaml, override with PROMOGO_CONFIG_FILE
# or: make run
```

Config is Viper-based (`internal/config/config.go`), overridable via `PROMOGO_<SECTION>_<KEY>` env vars (dots → underscores), e.g. `PROMOGO_POSTGRES_PASSWORD`, `PROMOGO_HTTP_PORT`.

## Verify it's running

- `GET /healthz` — always 200 (liveness).
- `GET /readyz` — checks Postgres + Redis connectivity.
- `POST /api/v1/transactions` — the 1C accrual webhook.
- `GET /api/v1/clients/{id}/balance` — client point balance.
- `POST /api/v1/transactions/redeem` — points redemption.

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

curl -X POST http://localhost:8080/api/v1/transactions \
  -H "Authorization: Bearer test-api-key" -H "Content-Type: application/json" \
  -d '{"transaction_id":"t1","phone":"+79990000000","amount":1500.00}'
# -> {"points_earned":75,"balance":75}

# Replaying the same transaction_id must NOT double-credit (idempotency guarantee,
# see migrations/00004_create_transactions.sql and internal/domain/ledger.go):
curl -X POST http://localhost:8080/api/v1/transactions \
  -H "Authorization: Bearer test-api-key" -H "Content-Type: application/json" \
  -d '{"transaction_id":"t1","phone":"+79990000000","amount":1500.00}'
# -> {"points_earned":75,"balance":75,"replayed":true}

curl http://localhost:8080/api/v1/clients/1/balance -H "Authorization: Bearer test-api-key"
```

No notification/SMS provider is wired up yet — `internal/notification/logchannel` just logs
`"msg":"notification"` lines instead of sending a real push. Watch the app's stdout (`slog`,
JSON by default) for those lines, and for `"http request"` entries showing status/duration per call.

## Stop

```bash
make docker-down
```
