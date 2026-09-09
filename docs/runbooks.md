# Operational runbooks

Scoped to what this service actually implements today (see
`knowledge/Decisions.md` for the design decisions these runbooks assume).
This is not a substitute for load/soak/dependency-failure testing or a full
incident-response program — see `docs/audit-remediation-prompt.md` Phase 4
for what's still open.

All metrics below are served at `GET /metrics` (Prometheus text format,
`internal/metrics`). All logs are structured (`log/slog`, JSON in
production) and, since Phase 4's request-ID work, every HTTP-originated log
line carries a `request_id` that matches the `X-Request-Id` response header
— ask the reporter (1C ops, mobile team, a curl repro) for that header
value to correlate their report with server logs.

## 1. Notification outbox backlog growing

**Symptom:** `promogo_outbox_backlog` climbing over time, or
`promogo_outbox_delivery_outcomes_total{outcome="retried"}` /
`{outcome="dead_letter"}` rate increasing.

**What this means:** Accrual/redeem/refund transactions are still
processing normally (the outbox is deliberately decoupled from the request
path — see Phase 3/DEC-014/015) but push notifications are falling behind
or failing. Customers may not see a balance-change notification promptly,
or at all if entries are being dead-lettered.

**Diagnose:**
1. Check `promogo_outbox_delivery_outcomes_total` by outcome — a spike in
   `dead_letter` vs. steady `retried` tells you whether this is transient
   (FCM slow/flaky) or a hard failure (bad credentials, a broken template).
2. Grep logs for `"notification delivery failed"` / `"notification
   dead-lettered"` (`internal/service/outbox_worker.go`) — `error` field
   has the underlying cause.
3. Check `promogo_http_request_duration_seconds` is *not* affected — if it
   is, this is not an outbox-only problem (see runbook 2).

**Mitigate:**
- FCM credential/quota issue: fix credentials or quota; the worker will
  drain the backlog automatically on the next few poll cycles (2s
  interval, exponential backoff up to 5 minutes — see
  `internal/app/app.go`'s `OutboxWorkerConfig`) once delivery starts
  succeeding again.
- A dead-lettered entry is not retried automatically (see the `dead_letter`
  status in `migrations/sql/00028_notification_outbox.sql`) — there is no
  admin replay endpoint yet; a stuck dead-letter requires a manual
  `UPDATE notification_outbox SET status = 'pending', attempts = 0,
  next_attempt_at = now() WHERE id = ...` if the underlying cause is fixed
  and the notification is still worth sending.

**Follow-up:** if `dead_letter` volume is non-trivial, this is a product
question (should the mobile app show a "missed notifications" indicator?)
as much as an ops one — flag to product rather than just clearing the
backlog.

## 2. Redis outage

**What this means (by design, not by accident — DEC-015):** every
Redis-gated security-sensitive path fails **closed**:
- POS transaction processing (accrual/redeem/refund) is fully unavailable
  — `internal/ratelimit.Middleware.Wrap` returns 503 the moment its
  backend errors, and it wraps every route.
- OTP issue/verify, QR issue/consume are unavailable for the same reason.
- The notification outbox worker itself doesn't depend on Redis (only
  Postgres), so once Redis is back, no notifications were lost — the
  transactional-outbox guarantee holds regardless of Redis's availability.

**This is a real availability gap for 1C integration** until 1C's offline
retry queue is implemented and tested (out of this repo's scope — see
DEC-015 and the Idea.md's "1C queues locally and replays" design). Today,
a Redis outage means POS purchases are not being recorded until Redis
recovers, unless 1C's own client-side retry buffers the webhook calls.

**Diagnose:**
1. `promogo_http_request_duration_seconds{status="503"}` spiking across
   store-key/customer routes.
2. Logs: `"rate limiter backend unavailable"` (`internal/ratelimit/
   middleware.go`) with the Redis error.
3. `/readyz` will report unhealthy (`Ready` checks `redisClient.Ping`).

**Mitigate:**
- Restore Redis (failover to replica, restart, fix network partition).
- There is no manual override to bypass the fail-closed rate limiter —
  this is deliberate (see DEC-015's rationale); do not patch around it
  under incident pressure without a security sign-off, since it exists
  specifically to prevent an unbounded-request/credential-stuffing window.

**Follow-up:** if Redis outages are frequent enough to matter, prioritize
1C's offline retry queue (Phase 5 territory) rather than weakening the
fail-closed behavior here.

## 3. Postgres unavailable / `/readyz` failing

**Symptom:** `/readyz` returns 503, `Ready` handler logs `"postgres: ..."`.

**Diagnose:**
1. Check Postgres itself (connection count, disk space, replication lag if
   applicable) — `internal/repository/postgres.NewPool`'s `MaxConns` bounds
   how many connections this service alone can hold.
2. Every transaction write (accrual/redeem/refund, the notification
   outbox's `insertOutboxEntry`) is already inside a DB transaction — a
   Postgres outage mid-request fails the request cleanly (no partial
   writes), it does not corrupt the ledger.

**Mitigate:** restore Postgres. There is no read-replica fallback or
degraded-mode operation — this service has a hard dependency on Postgres
for every write path and most reads.

**Follow-up:** see `docs/audit-remediation-prompt.md` Phase 4's backup/
restore/RPO/RTO item — not yet documented for this deployment.

## 4. Migration failure on deploy

**Symptom:** the new revision's Postgres pool constructs but
`migrate.Run` (`internal/app/app.go`) fails at startup — the process exits
non-zero before serving traffic, so old replicas (if any are still
running) keep serving on the old schema.

**Diagnose:** the migration error is logged (and the process exits) before
the HTTP server starts — check the deploy's startup logs, not `/readyz`
(the process never gets that far).

**Mitigate:**
- If the failed migration is a genuine bug (bad SQL, a constraint that
  can't be satisfied by existing data): fix the migration, don't hand-edit
  the `goose_db_version` table unless you fully understand the resulting
  state — see the db-migrate skill (`.claude/skills/db-migrate/SKILL.md`)
  for this repo's migration conventions.
- If the migration is fine but timed out or hit a lock: check for a long-
  running transaction holding a conflicting lock on the target table, and
  retry the deploy once it clears.

**Known gap:** every replica currently runs migrations on startup
(`internal/app/app.go`'s `New`) — with more than one replica, a rolling
deploy can race two replicas' migration attempts. Postgres advisory locks
(the earlier audit pass, per `MEMORY.md`/`CLAUDE.md`) prevent them from
corrupting the schema by running concurrently, but this still means a
schema change ships coupled to app deployment rather than as a separate,
reviewable step — see `docs/audit-remediation-prompt.md` Phase 4's
"decouple migration from every replica" item (not yet done).

## 5. Rate-limit rejections spike

**Symptom:** `promogo_rate_limit_rejections_total{profile=...}` climbing.

**Diagnose:**
1. Which `profile` (`staff_login`, `admin`, `client_lookup`, `accrual`,
   `qr_resolve` — see `internal/httpserver/routes.go`) is spiking tells you
   which flow is affected.
2. Logs: `"rate limit exceeded"` with `rule` and `retry_after_seconds`
   (`internal/ratelimit/middleware.go`).
3. Distinguish a real abuse pattern (many distinct IPs/keys, all near the
   limit) from a single noisy caller (e.g. a 1C instance retrying too
   aggressively after a transient error) from a limit that's simply too
   tight for legitimate peak traffic (e.g. a retail chain's opening-hour
   rush).

**Mitigate:**
- Single noisy caller: contact them (or their integration owner) — this is
  usually a bug in their retry logic, not a capacity problem here.
- Limit too tight: adjust the relevant `config.RateLimitConfig` value and
  redeploy — there's no live-tunable admin knob for this yet.

## 6. Panic spike

**Symptom:** log lines `"panic recovered"` (`internal/httpserver/
middleware_recover.go`) appearing, or `promogo_http_request_duration_seconds
{status="500"}` rate increasing without a corresponding application-level
error log.

**Diagnose:** the panic log line includes `panic` (the recovered value),
`stack`, `method`, `path`, and `request_id` — this is a genuine bug (a nil
dereference, an out-of-range index, a type assertion that should have been
a checked cast), not a normal error path, since normal errors are already
returned as values everywhere else in this codebase's style.

**Mitigate:** the request itself already got a clean 500 (recoverMW
prevents the panic from taking down the connection/process); this runbook
step is about the underlying bug, not about restoring service. Reproduce
from the stack trace and the request path, file a fix.

**Follow-up:** a panic that reveals a missing input validation is worth a
regression test at the layer that should have caught it (usually the HTTP
handler decoding the request), not just a `recover()`-level fix.
