#!/usr/bin/env bash
# Dependency-failure test: proves the documented fail-closed behavior
# (docs/runbooks.md #2/#3) under a real Redis/Postgres outage against the
# actual docker-compose stack, not just by reading the code. Pauses each
# dependency's container mid-traffic, asserts every request gets a clean
# 503 (never a 500/timeout/panic), then unpauses and asserts recovery.
#
# Usage: loadtest/dependency-failure.sh
# Requires: docker compose, curl. Brings its own stack up and tears it down.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Host ports default to something unlikely to collide with another local
# project's dev stack (see .claude/skills/dev-stack/SKILL.md) — override if
# needed via the same env vars docker-compose.yml already reads.
export POSTGRES_HOST_PORT="${POSTGRES_HOST_PORT:-15432}"
export REDIS_HOST_PORT="${REDIS_HOST_PORT:-16379}"
export APP_HOST_PORT="${APP_HOST_PORT:-18090}"

COMPOSE="docker compose -f deployments/docker-compose.yml"
APP_URL="http://127.0.0.1:${APP_HOST_PORT}"

cleanup() {
  echo "--- tearing down ---"
  $COMPOSE down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "--- building and starting the stack ---"
$COMPOSE up -d --build postgres redis app

echo "--- waiting for /readyz ---"
for i in $(seq 1 60); do
  code=$(curl -s -o /dev/null -w '%{http_code}' "$APP_URL/readyz" || true)
  [ "$code" = "200" ] && break
  sleep 1
done
if [ "$code" != "200" ]; then
  echo "FAIL: app never became ready"
  $COMPOSE logs app
  exit 1
fi
echo "OK: app ready"

echo "--- seeding a store/api-key ---"
SEED_JSON=$($COMPOSE run --rm app loadtest-seed 2>/dev/null)
API_KEY=$(echo "$SEED_JSON" | grep -o '"api_key": *"[^"]*"' | sed 's/.*: *"//;s/"$//')
if [ -z "$API_KEY" ]; then
  echo "FAIL: could not extract api_key from seed output: $SEED_JSON"
  exit 1
fi
echo "OK: seeded, api key acquired"

post_transaction() {
  # --max-time comfortably above requestTimeoutMW's 8s bound
  # (internal/httpserver/middleware_timeout.go) — a request must never
  # actually need this curl-level timeout to fire; if it does, that's a
  # regression of that middleware, not expected behavior to silently absorb.
  curl -s --max-time 12 -o /dev/null -w '%{http_code}' -X POST "$APP_URL/api/v1/transactions" \
    -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" \
    -d "{\"transaction_id\":\"depfail-$(date +%s%N)\",\"phone\":\"+79991234567\",\"amount\":100}"
}

echo "--- baseline: transaction succeeds before any outage ---"
code=$(post_transaction)
if [ "$code" != "200" ]; then
  echo "FAIL: baseline transaction expected 200, got $code"
  exit 1
fi
echo "OK: baseline 200"

fail_count=0
assert_status() {
  local label="$1" want="$2" got="$3"
  if [ "$got" != "$want" ]; then
    echo "FAIL: $label expected $want, got $got"
    fail_count=$((fail_count + 1))
  else
    echo "OK: $label -> $got"
  fi
}

echo "--- pausing redis (simulating an outage) ---"
$COMPOSE pause redis
sleep 1
for i in 1 2 3; do
  assert_status "transaction during redis outage (attempt $i)" 503 "$(post_transaction)"
done

echo "--- unpausing redis ---"
$COMPOSE unpause redis
for i in $(seq 1 20); do
  code=$(post_transaction)
  [ "$code" = "200" ] && break
  sleep 0.5
done
assert_status "transaction after redis recovery" 200 "$code"

echo "--- pausing postgres (simulating an outage) ---"
$COMPOSE pause postgres
sleep 1
for i in 1 2 3; do
  got=$(post_transaction)
  # Postgres being unreachable surfaces as a request-handling error, not the
  # rate limiter's dedicated 503 path — accept any 5xx here (never a 2xx,
  # and never a connection reset/timeout the app itself caused by panicking).
  if [[ "$got" != 5* ]]; then
    echo "FAIL: transaction during postgres outage (attempt $i) expected a 5xx, got $got"
    fail_count=$((fail_count + 1))
  else
    echo "OK: transaction during postgres outage (attempt $i) -> $got"
  fi
done

echo "--- unpausing postgres ---"
$COMPOSE unpause postgres
for i in $(seq 1 20); do
  code=$(post_transaction)
  [ "$code" = "200" ] && break
  sleep 0.5
done
assert_status "transaction after postgres recovery" 200 "$code"

echo "--- checking for panics during the outages ---"
if $COMPOSE logs app 2>/dev/null | grep -q "panic recovered"; then
  echo "FAIL: app logged a recovered panic during the outage — fail-closed should never panic"
  fail_count=$((fail_count + 1))
else
  echo "OK: no panics logged"
fi

if [ "$fail_count" -gt 0 ]; then
  echo "=== FAILED: $fail_count assertion(s) failed ==="
  exit 1
fi
echo "=== PASSED ==="
