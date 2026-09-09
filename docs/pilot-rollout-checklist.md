# Pilot rollout / rollback checklist (draft)

**Status: draft, needs release-gate sign-off** — same posture as
`docs/slo.md`/`docs/backup-restore.md`: engineering-judgment defaults
grounded in this repo's actual, verified behavior (the clean CI baseline,
the security/regression review materials, the load/soak/restore-drill
records), not yet a business-approved runbook. Built to be *worked
through*, one box at a time, by whoever runs the actual pilot cutover —
not read once and filed away.

This is the last piece of the management-level release gate before a
release candidate is cut, per [[DEC-017]] through [[DEC-021]]:

1. ~~ADR sign-off (0001–0003)~~ — tracked separately, not in this
   document's scope.
2. ~~legal-minimum.md sign-off~~ — tracked separately (`docs/legal-minimum.md`).
3. ~~Full CI from a clean checkout~~ — **done**, 8/8 green at `ab6909a`
   (`docs/ci-clean-checkout-baseline-2026-09-09-followup.md`).
4. ~~Security/regression review materials for Phase 1–3~~ — **done**,
   `docs/security-regression-review-phase1-3.md` (the review itself, by a
   human, is still open).
5. **This document** — pilot rollout and rollback checklist.
6. → release candidate.

## Prerequisites (verify before starting the rollout section)

- [ ] The commit being rolled out has a green CI run from a clean checkout
      (`docs/ci-clean-checkout-baseline-2026-09-09*.md` pattern) — if the
      commit under rollout is newer than `ab6909a`, re-run the full suite,
      don't assume the old baseline still applies.
- [ ] The security/regression review (`docs/security-regression-review-phase1-3.md`)
      has an actual sign-off DEC recorded, not just materials prepared.
- [ ] ADR 0001–0003 sign-off is recorded.
- [ ] `docs/legal-minimum.md` is agreed with the 152-FZ owner.
- [ ] The one open, knowingly-accepted risk from item 1 of the security
      review (legacy `stores.api_key_hash` has no scope enforcement) has an
      explicit decision on record for whether the pilot store's key is
      created via that path or via `store_api_keys` from day one — **this
      materially changes what "authorized to do what" means for that
      store's key on day one of the pilot.**

## Environment and configuration

- [ ] `PROMOGO_APP_SKIP_STARTUP_MIGRATIONS=true` is set for every pilot
      replica (see `docs/deployment.md`) — migrations are a separate,
      explicit deploy step, never implicit on replica boot in production.
- [ ] `promogo migrate` has been run once, out of band, against the pilot
      database, *before* any replica starts — confirm via `goose status`
      or the app's own refusal-to-start-on-stale-schema behavior (verified
      in the CI baseline's `migration-deploy-model` job) rather than
      assuming the step ran.
- [ ] SMS provider is `provider=http` with real `endpoint`/`token` set —
      confirm the app actually refuses to start with `provider=log`
      outside development (checked once already in the security review's
      item 4 checklist; re-verify against the *actual pilot config*, not
      just that the code path exists).
- [ ] FCM credentials are configured and valid — same fail-fast
      expectation as SMS.
- [ ] Postgres and Redis connections use TLS per `docs/deployment.md` and
      the pilot's actual network topology (confirm whether Postgres/Redis
      are same-VPC/private-network vs. requiring TLS in transit; the code
      enforces "TLS configured" outside development, not "network is
      inherently trusted").
- [ ] `PROMOGO_HTTP_TRUSTED_PROXIES` (or equivalent) matches the pilot's
      actual reverse-proxy/load-balancer chain — re-verify against the real
      deployed topology, not the topology assumed during development
      (see the security review's item 4 checklist — this was flagged
      there as something to re-check at actual cutover time, not just in
      code review).
- [ ] `AntiFraudConfig.DailyRedeemPointsLimit`/`DailyRedeemWindow` are set
      to real, business-approved values (not left at development
      defaults) — outside `development` the app fails startup if these
      are unset per [[#DEC-013]], but a *present but wrong* value won't be
      caught by that check.
- [ ] `HTTPSERVER_MIN_COVERAGE` and other CI-only gates are irrelevant to
      runtime config — no action needed, listed here only to explicitly
      rule it out as a rollout concern.

## Data and backup readiness

- [ ] A `pg_dump` backup schedule is actually running against the pilot
      database (not just documented) — per `docs/backup-restore.md`'s
      24-hour RPO target. Confirm the *first* backup has completed and is
      stored off the database host before go-live, not just that cron is
      configured.
- [ ] The restore procedure in `docs/backup-restore.md` has been dry-run
      against this specific pilot environment's actual Postgres version/
      hosting (the recorded drill used a local `postgres:16-alpine`
      container — re-run against the real pilot Postgres if it differs in
      version, hosting, or scale, since the recorded numbers explicitly
      don't cover restoring under production-like conditions).
- [ ] Retention policy (30 days operational per `docs/backup-restore.md`,
      separately whatever `docs/legal-minimum.md` requires for
      transaction/audit history) is implemented, not just documented.
- [ ] Redis has no backup requirement (confirmed ephemeral/TTL'd per
      `docs/backup-restore.md`) — explicitly note this to whoever is
      running ops so they don't spend effort backing up Redis unnecessarily,
      or conversely assume Redis loss is recoverable from a backup that
      doesn't exist.

## Observability readiness

- [ ] `GET /metrics` is scraped by the pilot's actual monitoring stack —
      confirm the specific metrics `docs/runbooks.md` references
      (`promogo_outbox_backlog`, `promogo_outbox_delivery_outcomes_total`,
      `promogo_http_request_duration_seconds`, `promogo_rate_limit_rejections_total`)
      are visible in that stack, not just theoretically exposed.
- [ ] Alerting exists for at minimum: `/readyz` failing, outbox backlog
      growing unboundedly, 5xx rate above the SLO's error-rate budget
      (`docs/slo.md`), Postgres/Redis connection failures. If any of these
      has no alert yet, that's a gap to accept explicitly, not silently.
- [ ] Whoever is on call for the pilot window has read `docs/runbooks.md`
      end to end, not just this checklist — the six runbooks there are the
      actual incident playbook; this document is about the cutover moment,
      not ongoing operations.
- [ ] `request_id` correlation (structured logs + `X-Request-Id` header)
      is confirmed working end to end in the pilot environment — this is
      how a real incident gets diagnosed; confirm it before the pilot
      needs it, not during an incident.

## The cutover itself

- [ ] Confirm the pilot's realistic peak load (per `docs/slo.md`'s
      workload model: ~1-3 req/s POS webhook, bursty OTP/QR) versus what
      was actually load/soak-tested (`docs/load-test-baseline.md` — 5-10x
      headroom target). If actual pilot store characteristics diverge
      meaningfully from the modeled workload (much higher checkout lane
      count, a promotional-event spike expected on day one), flag before
      cutover, not after.
- [ ] `promogo migrate` run against production, verified via `goose status`
      showing no pending migrations.
- [ ] First replica started with `PROMOGO_APP_SKIP_STARTUP_MIGRATIONS=true`,
      `/healthz` and `/readyz` both confirmed 200 before routing any real
      traffic to it.
- [ ] A smoke test equivalent to `internal/e2e.TestPilotEndToEnd`
      (webhook → accrual → mobile OTP login → balance → QR issue → resolve
      → redeem → history → refund → refund replay) is run against the
      **actual pilot environment**, not just CI's `e2e-pilot` job against
      ephemeral containers — a passing CI job proves the code path works,
      not that this specific environment's config (real SMS/FCM,
      real network topology, real trusted-proxy chain) is wired correctly.
- [ ] The pilot store's actual API key is created through the intended
      path (`store_api_keys`, scoped — see the "Prerequisites" item above)
      and the plaintext is delivered to 1C/POS integration through a
      channel that doesn't re-expose it (no pasting into a shared doc,
      ticket, or chat log that outlives the "shown once" property the
      code provides).
- [ ] 1C's actual webhook retry/offline-queue behavior (per `Idea.md`'s
      design and `docs/runbooks.md` runbook #2's explicit note that this
      is "out of this repo's scope") is confirmed to exist and work on the
      1C side before cutover — this service fails closed on Redis outage
      by design ([[#DEC-015]]); if 1C has no retry buffer, a Redis blip
      becomes a lost sale, not just a delayed one.
- [ ] Real traffic is ramped, not switched all at once, if the integration
      allows it (e.g., a subset of checkout lanes first) — not a hard
      requirement given the pilot's realistic peak load is well within
      tested headroom, but reduces blast radius of anything the smoke test
      didn't catch.

## Rollback triggers and procedure

Define **before** cutover, not during an incident, what triggers a
rollback versus a forward-fix:

- [ ] **Rollback trigger — app version.** Any of: `/readyz`/`/healthz`
      failing and not self-recovering within N minutes (pick N with the
      business owner against the 99.5% availability SLO's error budget);
      a panic-recovered 500 rate spike (`docs/runbooks.md` #6) tied to a
      newly-deployed change; a security-relevant regression discovered
      post-deploy that the security review's checklist should have caught
      but didn't.
  - **Procedure:** redeploy the previous known-good container image/tag.
    Because migrations are decoupled from app startup
    (`docs/deployment.md`), rolling back the *application* does not
    require rolling back the *schema* unless the new version's migration
    is itself the problem (see the next trigger). Confirm the previous
    image is still available (registry retention) before you need it, not
    when you need it.
- [ ] **Rollback trigger — migration.** The just-applied migration is
      itself defective (bad SQL, a constraint existing pilot data can't
      satisfy, or — if the expand/migrate/contract discipline in
      `docs/deployment.md` was violated — a contract migration that
      dropped something still in use).
  - **Procedure:** do **not** hand-edit `goose_db_version` under pressure
    (per `docs/runbooks.md` #4's explicit warning) unless you fully
    understand the resulting state. If the migration followed
    expand/migrate/contract correctly, the previous app version should
    still work against the expanded (not yet contracted) schema — verify
    this is actually true for the specific migration in question before
    relying on it. If it's a genuine one-way migration with no safe
    application-level rollback, the restore-from-backup path
    (`docs/backup-restore.md`) is the fallback — know the actual RTO for
    this environment before you're mid-incident, not after.
- [ ] **Rollback trigger — data-integrity.** Any indication of ledger/
      balance drift, double-accrual, or a refund invariant violation (the
      exact classes of bug the Phase 2 review items in
      `docs/security-regression-review-phase1-3.md` cover) discovered in
      production.
  - **Procedure:** this is the most severe category — money is wrong, not
    just unavailable. Stop accepting new writes for the affected store
    (there is no built-in maintenance-mode flag today — confirm whether
    one is needed before pilot, or whether "roll back the app version"
    is judged sufficient to stop new writes through the buggy path) while
    the specific transaction(s) are hand-audited against the ledger.
    Restoring from backup loses any legitimate transactions since the
    last dump (24h RPO) — weigh that against leaving corrupted data live;
    this tradeoff needs a business-owner decision, not just an engineering
    one, made in advance if possible.
- [ ] **Explicitly not a rollback trigger:** notification delivery
      degradation alone (outbox backlog growing, FCM/SMS provider issues)
      — per `docs/runbooks.md` #1, this is decoupled from transaction
      processing by design; the ledger stays correct even if a customer's
      push notification is late or dead-lettered. Mitigate per that
      runbook, don't roll back the app for it.
- [ ] Whoever can authorize a rollback decision (on-call engineer alone,
      or does a data-integrity rollback specifically need a second
      approver given the business-owner tradeoff above?) is named and
      reachable during the pilot window — decide this before cutover.

## Post-cutover

- [ ] Re-run the smoke test (same as the cutover step) at T+1 hour and
      T+24 hours to catch anything that only manifests after sustained
      real traffic (connection pool exhaustion, a slow memory/goroutine
      leak the load test's 15-minute soak window per
      `docs/load-test-baseline.md` wouldn't have caught).
  - [ ] Confirm the soak test's 15-minute window is actually representative
        of the pilot's real usage pattern (a store open 10+ hours/day) —
        flag if a longer soak run is warranted before treating the
        existing baseline as sufficient confidence for a multi-day pilot.
- [ ] Confirm the first automated backup after cutover actually
      completed and is restorable (spot-check, don't just check the cron
      job ran).
- [ ] Record the actual rollout outcome (clean cutover / rollback invoked
      and why / issues found but not rollback-worthy) as a DEC in
      `knowledge/Decisions.md`, closing the release-gate sequence this
      document is part of.
