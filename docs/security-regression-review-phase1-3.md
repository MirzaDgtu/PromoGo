# Security / regression review materials: Phase 1–3 (2026-09-09)

Prepared for the human security/regression review step of the pilot
release gate, per [[DEC-019]]/[[DEC-020]] — this document builds on the
confirmed clean CI baseline (`docs/ci-clean-checkout-baseline-2026-09-09.md`,
`docs/ci-clean-checkout-baseline-2026-09-09-followup.md`, 8/8 green at
`ab6909a`). Scope is exactly `docs/audit-remediation-prompt.md`'s Phase 1
(security/tenant-isolation), Phase 2 (loyalty/concurrency correctness) and
Phase 3 (QR, notifications, availability) — the changes that touch trust
boundaries, money, and PII. Phase 4 (infra/CI/ops) and Phase 5 (product/ADR
decisions) are out of scope here; they're covered by the CI baseline and
the ADR sign-off step of the release gate respectively.

This is **materials for** a human reviewer, not the review itself: each
item states the original threat, what changed and where, what a reviewer
should independently re-check, and what residual risk remains even after
the fix. Nothing here should be read as "already reviewed and approved."

## How to use this document

Each item below maps to one commit (or a tight commit group) and one
`knowledge/Decisions.md` entry. For each: reproduce the original defect
mentally (or, where practical, by checking out the parent commit and
re-running the cited test against it — the tests are written to fail
against the pre-fix code), read the diff at the cited files, then work the
manual checklist. The consolidated checklist at the end assumes every
per-item checklist has been worked first.

---

## Phase 1: security and tenant isolation

### 1. Store API key hashing mismatch and scope enforcement

- **Original defect**: `GenerateAPIKey` stored the hash of only the
  *secret* half of `<keyID>.<secret>`, while middleware hashed and
  compared the *full* bearer value — a mismatch that (per the audit
  prompt) meant key verification wasn't doing what it appeared to do.
  There was also no scope enforcement — any valid store key could call
  any store-scoped endpoint.
- **Fixing commit / DEC**: `33c61bc` ("Add customer OTP auth, staff
  OIDC+RBAC, and store API-key rotation") introduced the current
  `store_api_keys` table and `RequireStoreAPIKey` middleware; `c795606`
  ("...fix store API key and staff RBAC IDOR bugs") hardened it further,
  including making `Revoke` cross-tenant-safe. Documented in [[#DEC-004]]
  and [[#DEC-008]].
- **Trust boundaries / files**: 1C/POS → API boundary (external, no prior
  auth). `internal/httpserver/middleware.go` (`RequireStoreAPIKey`,
  `requireScope`, `constantTimeHashEqual`), `internal/auth/*` (key
  generation/hashing), `internal/repository/postgres/store_api_key_repository.go`,
  migrations `00019`/`00021`.
- **Tests (incl. negative/concurrent)**:
  `TestHandleCreateStoreAPIKey_PlaintextAuthenticatesRealRequest` (the
  plaintext returned on creation actually authenticates — the exact
  end-to-end check the original mismatch would have failed),
  `TestHandleCreateStoreAPIKey_InvalidScope`,
  `TestHandleRevokeStoreAPIKey_CrossStoreRejected` (IDOR),
  `TestHandleRevokeStoreAPIKey_Success` — all in
  `internal/httpserver/admin_route_test.go`. Concurrency note: the
  `fakeStoreAPIKeyRepo` test double these tests share had its own,
  unrelated data race, fixed separately in `3cf98ba` (`Q-P0-126` — see the
  CI follow-up baseline). That race was in test infrastructure, not in
  `RequireStoreAPIKey` or the repository itself.
- **Migration / rollout risk**: `stores.api_key_hash` (legacy single-key
  column) is preserved and still fully trusted with no scope checks
  (`requireScope` short-circuits for legacy keys) — this is a deliberate
  compatibility bridge, not an oversight, but it means **any store that
  hasn't rotated onto `store_api_keys` has an unscoped, unrotatable key**
  in production. No forced-migration or expiry path exists yet.
- **Residual risk**: legacy key holders bypass scope enforcement entirely.
  Plaintext key is returned once over HTTPS on creation — confirm the
  pilot's actual key-distribution channel (who reads the creation
  response, is it logged anywhere upstream of this service, e.g. an API
  gateway or a support ticketing tool) doesn't defeat the "shown once"
  property.
- **Manual checklist**:
  - [ ] Confirm no code path logs `apiKey.KeyHash`, the raw bearer value,
        or the legacy `api_key_hash` at any log level.
    - [ ] Trace one full key-creation → key-use round trip against a
        real (not fake) Postgres and confirm the plaintext returned at
        creation is exactly what a subsequent request must send.
  - [ ] Confirm every store-scoped admin/POS route that isn't explicitly
        legacy-only actually calls `requireScope` with the right
        permission constant (grep every `RequireStoreAPIKey(` call site).
  - [ ] Decide/record whether legacy `api_key_hash` gets a sunset date
        before the pilot goes wide (this is a real, currently-open gap,
        not resolved by this review).

### 2. RBAC / tenant IDOR

- **Original defect**: no structural prevention of a `retailer_admin`
  granting/obtaining `platform_admin`; staff membership reads/writes not
  reliably scoped by `organization_id`/`store_id`; API-key revoke
  reachable cross-tenant by guessing a numeric ID; nullable uniqueness
  permitting duplicate org-wide memberships.
- **Fixing commit / DEC**: `33c61bc` established the RBAC/permission model
  ([[#DEC-005]]); `c795606` is the dedicated IDOR-fix commit (title says so
  explicitly) — self-escalation and cross-org/cross-store IDOR guards.
- **Trust boundaries / files**: staff-facing admin API (OIDC-authenticated
  humans, one org's staff must never reach another org's data).
  `internal/httpserver/admin.go` (or equivalent handler file — verify
  current name), `internal/service/staffauth.go`, `internal/domain/rbac.go`,
  migrations that added partial-unique/composite constraints for
  membership uniqueness.
- **Tests (incl. negative/concurrent)**: all in
  `internal/httpserver/admin_route_test.go` —
  `TestHandleCreateStaffMembership_RetailerAdminCannotGrantPlatformAdmin`,
  `TestHandleCreateStaffMembership_PlatformAdminCanGrantPlatformAdmin`
  (positive control — proves the negative test isn't just "everything
  403s"), `TestHandleUpdateStaffMembership_RetailerAdminCannotEscalateToPlatformAdmin`,
  `TestHandleUpdateStaffMembership_RetailerAdminCannotDemoteOrDisablePlatformAdmin`,
  `TestHandleUpdateStaffMembership_CrossOrgIDORRejected`,
  `TestHandleUpdateStaffMembership_CannotModifyOwnMembership`,
  `TestHandleGetStore_NotFoundWrongOrg`,
  `TestHandleAdminListClientTransactions_NotFoundWrongStore`. No dedicated
  concurrency test for RBAC itself (role checks are stateless reads per
  request, not a shared-mutable-state race) — reviewer judgment: is that
  gap acceptable, or does "last active admin" bookkeeping (audit-prompt's
  "removal/disable of the last active administrator" requirement) need a
  concurrent-disable test that doesn't currently exist?
- **Migration / rollout risk**: existing memberships created before the
  uniqueness constraint tightened need a one-time check for pre-existing
  duplicates that the new constraint would now reject on write — confirm
  the migration that adds the constraint either finds none (fresh
  pilot database) or has a documented cleanup step.
- **Residual risk**: `StaffPrincipal` is resolved from the DB on every
  request (by design, per [[#DEC-005]], for immediate revocation) — this
  is good for revocation speed but means every admin request pays a DB
  round trip; not a security gap, but worth the reviewer confirming it's
  accounted for in the Phase 4 SLO numbers, not silently absorbed.
- **Manual checklist**:
  - [ ] Independently attempt (against a local stack, not just reading
        the test) a `retailer_admin` → `platform_admin` self-escalation
        via every membership-mutating endpoint, not just the ones with a
        named test.
  - [ ] Confirm "cannot modify own membership" doesn't create a
        no-recovery lockout path (e.g., last platform_admin disabling
        themselves through some other endpoint the tests don't cover).
  - [ ] Check the "last active administrator" policy referenced by the
        audit prompt is actually implemented somewhere, or confirm it was
        consciously deferred and is tracked in `Project Questions.md`.

### 3. OTP and customer sessions

- **Original defect**: OTP verify/consume not atomic (lost-update risk on
  attempt counters); resend/issue cooldown not atomic; refresh-token
  rotation not one transaction (reuse could mint multiple sessions);
  blocked/deleted account status not enforced on refresh.
- **Fixing commit / DEC**: `33c61bc` (initial OTP/session implementation,
  [[#DEC-007]]); `6739d5f` ("Make OTP verification, refresh-token
  rotation, and account status checks atomic") is the dedicated
  atomicity-fix commit.
- **Trust boundaries / files**: unauthenticated → authenticated boundary
  (anyone with a phone number can attempt OTP). `internal/service/otp_store.go`
  (Redis Lua atomicity), `internal/service/customerauth.go` (refresh
  rotation as one Postgres transaction), `internal/repository/postgres/customer_session_repository.go`.
- **Tests (incl. negative/concurrent)**: `internal/service/customerauth_test.go` —
  negative: `TestCustomerAuth_VerifyOTPWrongCodeRejected`,
  `TestCustomerAuth_OTPExpiresAfterTTL`,
  `TestCustomerAuth_OTPMaxAttemptsLocksChallenge`,
  `TestCustomerAuth_InvalidPhoneRejected`; concurrent (the ones that
  directly prove the atomicity fix did what it claims):
  `TestCustomerAuth_ConcurrentOTPVerifySameCodeExactlyOneSuccess`,
  `TestCustomerAuth_ConcurrentRefreshSameTokenExactlyOneSuccess`,
  `TestCustomerAuth_ConcurrentRequestOTPCooldownRace`,
  `TestCustomerAuth_ConcurrentAccountCreateRaceRecovered`,
  `TestCustomerAuth_RefreshReuseDetectionRevokesChain` (a stolen/replayed
  refresh token revokes the whole session chain, not just itself).
- **Migration / rollout risk**: none schema-breaking noted; this was
  logic-only inside existing tables per the DEC.
- **Residual risk**: [[#DEC-013]] notes OTP hashes have no HMAC/pepper —
  a 6-digit code space is small; if Redis itself were ever exposed
  (misconfiguration, backup leak), stored hashes are crackable offline.
  Deferred per the DEC as an accepted MVP tradeoff, not fixed.
- **Manual checklist**:
  - [ ] Read `TestCustomerAuth_ConcurrentOTPVerifySameCodeExactlyOneSuccess`
        and `TestCustomerAuth_ConcurrentRefreshSameTokenExactlyOneSuccess`
        closely enough to confirm they'd actually fail against the
        pre-`6739d5f` code (i.e., they exercise the real race window, not
        an artificially serialized one) — concurrency tests are the
        easiest kind to accidentally write as non-races.
  - [ ] Confirm blocked/deleted status enforcement covers *every*
        protected customer route, not only refresh (re-check
        `internal/httpserver/middleware.go`'s customer-auth path).
  - [ ] Decide whether the missing OTP hash pepper is acceptable for the
        pilot's actual Redis deployment (network-isolated? backed up
        anywhere accessible to more people than production DB access?).

### 4. Configuration and transport security

- **Original defect**: SMS provider not strictly validated (could
  silently fall back to a log-only sender in production); no enforced
  HTTPS/TLS for SMS, Postgres, Redis outside development; inconsistent
  client-IP trust across OTP throttling / audit / request logs;
  unescaped credential interpolation into DSNs.
- **Fixing commit / DEC**: `894d828` ("Harden transport/config security:
  safe DSN, strict SMS enum, TLS, unified client-IP trust"). [[#DEC-014]]
  covers the SMS/FCM provider-agnostic, fail-fast side of this.
- **Trust boundaries / files**: config-load-time boundary (fail fast, not
  silently insecure, outside `development`). `internal/config/config.go`,
  `internal/httpserver/remoteaddr.go` (trusted-proxy CIDR parsing),
  `internal/notification/httpsms/httpsms.go`.
- **Tests (incl. negative/concurrent)**: `internal/httpserver/remoteaddr_test.go` —
  `TestParseTrustedProxies_MalformedCIDRFails` (fail-fast on bad config),
  `TestResolveClientIP_UntrustedPeerIgnoresXFF` (the core anti-spoofing
  property — an untrusted peer cannot forge its apparent IP via
  `X-Forwarded-For`), `TestResolveClientIP_TrustedPeerUsesXFF`,
  `TestClientIP_AuditLogHonorsTrustedProxy`. No concurrency test needed
  here (pure per-request parsing, no shared state).
- **Migration / rollout risk**: any pilot deployment that was relying on
  an unset/misconfigured trusted-proxy list will now **fail startup**
  instead of silently trusting the wrong header — confirm the actual
  pilot deployment's reverse-proxy topology is documented and the
  `PROMOGO_HTTP_TRUSTED_PROXIES`-equivalent config is set correctly
  *before* cutover, not discovered via a crash-looping pod.
  `docs/deployment.md` should state the exact required topology — verify
  it does.
- **Residual risk**: TLS *requirement* enforcement outside development is
  config-level (fail startup if not configured) — it does not itself
  terminate or verify TLS; actual certificate validity/rotation is an
  operational concern outside this repository's scope.
- **Manual checklist**:
  - [ ] Independently verify (not just read) that starting the app in a
        non-development environment with `provider=log` or missing FCM
        credentials actually fails startup — this is a "prove it
        crashes" check, not a "confirm the code looks like it would."
  - [ ] Confirm the pilot's actual edge topology (load balancer / reverse
        proxy chain) matches what's documented and what
        `resolveClientIP` is configured to trust.
  - [ ] Grep for any remaining raw `fmt.Sprintf`-built DSN string
        anywhere in the codebase (config, migrations tooling, scripts)
        that this fix might have missed.

### 5. PII and audit trail

- **Original defect**: raw phone numbers/tokens/credentials could reach
  logs and repository errors; audit records not durable (could be lost on
  a crash between mutation and audit write); audit rows had no DB-level
  append-only enforcement.
- **Fixing commit / DEC**: `1900e45` ("Mask phone numbers in logs/errors;
  make audit_events append-only"). Related: `33c61bc` introduced
  `audit_events` itself (migration `00020`).
- **Trust boundaries / files**: everything that touches a phone number or
  writes an audit row. `internal/httpserver/clients.go`,
  `internal/repository/postgres/client_repository.go`,
  `internal/repository/postgres/customer_account_repository.go`,
  migration `00024` (append-only trigger/index).
- **Tests**: `TestHandleAdminLookupClient_MaskedForSupportViewer` /
  `TestHandleAdminLookupClient_UnmaskedForRetailerAdmin` (role-gated
  unmasking, not a blanket mask), `TestHandleListAuditEvents_Success`.
  No dedicated "audit row survives a mid-transaction crash" test found —
  durability here relies on writing the audit row in the same DB
  transaction as the mutation (a structural guarantee, not something a
  unit test can directly exercise without fault injection).
- **Migration / rollout risk**: migration `00024` adds append-only
  enforcement (per the db-migrate skill's own documented gotcha pattern:
  a `BEFORE UPDATE OR DELETE` trigger that raises). Confirm the down
  migration's behavior against existing audit data is understood — an
  append-only trigger that can't be cleanly reverted without dropping
  data is a one-way door, which may be intentional but should be a
  reviewed decision, not a surprise.
- **Residual risk**: "masked for support_viewer, unmasked for
  retailer_admin" is a role-based disclosure boundary — confirm this
  matches the actual legal-minimum data-access policy being negotiated in
  the Phase 5 legal-minimum sign-off (this review can't itself confirm
  legal correctness, only that the code matches whatever the intended
  policy is).
- **Manual checklist**:
  - [ ] Grep the full repo for `phone` used in any `log.`/`slog.`/error
        string construction outside the masking helper, to catch any
        call site the original fix might have missed in a later commit.
  - [ ] Attempt (against a real Postgres, not a fake) an `UPDATE` or
        `DELETE` directly against `audit_events` as the application's DB
        role and confirm it's actually rejected at the database level,
        not just "no code path calls it."
  - [ ] Confirm which roles can call `TestHandleListAuditEvents_Success`'s
        endpoint and whether that matches the intended audit-access
        policy.

---

## Phase 2: loyalty and concurrency correctness

### 6. Refund replay drift and redemption invariants

- **Original defect**: replaying an already-processed partial refund
  could report an incorrect (drifted) cumulative refunded amount instead
  of the stable original result; `MinBalanceToRedeem` eligibility was
  checked outside the locked ledger transaction, opening a TOCTOU window
  under concurrent redemptions.
- **Fixing commit / DEC**: `872bdec` ("Fix refund-replay drift and
  enforce redemption invariants atomically (Phase 2)"). Builds on
  [[#DEC-012]] (refund semantics) and [[#DEC-013]] (daily limit
  atomicity).
- **Trust boundaries / files**: money-correctness boundary — any bug here
  is a direct financial-integrity issue, not just an access-control one.
  `internal/service/loyalty.go`, `internal/repository/postgres/ledger_repository.go`,
  migration `00025` (refund replay snapshot storage).
- **Tests (incl. negative/concurrent)**: `internal/service/loyalty_test.go` —
  `TestRefund_ReplayOfEarlierPartialRefundReturnsStableSnapshot` (the
  exact drift bug, direct regression test), `TestRefund_MultiplePartialRefundsWithRoundingCorrection`,
  `TestRefund_OverRefundRejected`, `TestRefund_RefundOfRefundRejected`,
  `TestRefund_CrossStoreReferenceRejected`, `TestRefund_IdempotentReplay`,
  `TestRefund_FingerprintConflictOnMismatchedAmount`,
  `TestRedeem_CapAppliedBeforeBalanceCheck`,
  `TestRedeem_DailyLimitAllowsUnderAndAtBoundary` /
  `TestRedeem_DailyLimitExceeded` /
  `TestRedeem_DailyLimitRollingWindowExpires` /
  `TestRedeem_DailyLimitIsolatedPerClient` /
  `TestRedeem_DailyLimitConsidersCappedPointsNotRequested`. No dedicated
  Go-level goroutine-concurrency test for the redemption TOCTOU window
  found in this file — the atomicity guarantee here rests on
  `SELECT ... FOR UPDATE` at the Postgres level (verified structurally by
  reading `ledger_repository.go`, not by a race-style test). Reviewer
  should judge whether that's sufficient or whether a real concurrent
  double-redeem integration test against Postgres is warranted before
  pilot.
- **Migration / rollout risk**: migration `00025` adds a snapshot column
  for stable replay results — a schema change with a clear
  expand-only footprint (additive), but confirm any refund transactions
  created *before* this migration (if any test/dev data exists) don't
  have a null snapshot that breaks the new replay logic on first re-read.
- **Residual risk**: rounding correction logic ("last refund takes the
  exact remainder instead of a rounded value") is a subtle invariant —
  worth an independent hand-trace of the arithmetic in
  `TestRefund_MultiplePartialRefundsWithRoundingCorrection` against the
  actual formula in `loyalty.go`, not just trusting the test's expected
  values are correct.
- **Manual checklist**:
  - [ ] Hand-verify the rounding formula in `loyalty.go` against the test
        expectations in `TestRefund_MultiplePartialRefundsWithRoundingCorrection`
        line by line — don't just confirm the test passes.
  - [ ] Judge whether a real-Postgres concurrent-redeem integration test
        (two goroutines redeeming against the same balance at the
        boundary) should be added before pilot, given no such test
        currently exists for this specific invariant.
  - [ ] Confirm `balances_points_check` being dropped (per [[#DEC-012]])
        and replaced by a guarded `UPDATE ... WHERE points + $2 >= 0` is
        still correctly enforced everywhere points are decremented — grep
        for any other UPDATE on `balances.points` that might bypass the
        guard.

### 7. Loyalty config versioning, history, and rollback

- **Original defect**: loyalty configuration changes weren't versioned —
  no way to reconstruct which rule version applied to a historical
  transaction, no audit/rollback for config changes; validation of
  mechanic/percentage/rate bounds wasn't enforced at the boundary
  (invalid config could produce a `500` instead of a `400`/`422`).
- **Fixing commit / DEC**: `8f2d39f` ("Add loyalty rule versioning, config
  history, and rollback (Phase 2)").
- **Trust boundaries / files**: staff admin → business-rule boundary
  (a misconfigured or malicious config change directly changes how money
  moves for every future transaction). `internal/repository/postgres/loyalty_config_repository.go`,
  `internal/service/loyalty.go`, migration `00026`.
- **Tests**: `internal/httpserver/admin_route_test.go` —
  `TestHandlePutLoyaltyConfig_NegativeValueRejected`,
  `TestHandlePutLoyaltyConfig_MalformedJSONRejectedWith400`,
  `TestHandlePutLoyaltyConfig_UnknownMechanicRejected`,
  `TestHandlePutLoyaltyConfig_PercentAbove100Rejected`,
  `TestHandlePutLoyaltyConfig_ZeroExchangeRateRejected`,
  `TestHandlePutLoyaltyConfig_VersionIncrementsAndHistoryRecorded`,
  `TestHandleRollbackLoyaltyConfig_ReappliesOldVersionAsNewVersion`,
  `TestHandleRollbackLoyaltyConfig_UnknownVersionRejected`; and
  `TestAccrueAndRedeem_StampRuleVersionFromEffectiveConfig` in
  `internal/service/loyalty_test.go` (proves a transaction actually
  records which rule version it used, the core auditability property).
- **Migration / rollout risk**: migration `00026` introduces config
  history — confirm it backfills a version-1 row for any pre-existing
  loyalty config (if the pilot database already has one from before this
  change) rather than leaving it versionless.
- **Residual risk**: rollback re-applies an old version *as a new version*
  (append-only history, not an in-place revert) — this is the safer
  design, but confirm staff-facing UI/API documentation makes clear that
  "rollback" doesn't erase the interim (bad) version from history, since
  that has audit implications reviewers of *that* audit trail should
  understand.
- **Manual checklist**:
  - [ ] Confirm every numeric config field a store admin can set has a
        validation test (cross-check the OpenAPI schema's stated bounds
        against `admin_route_test.go`'s coverage — look for a field with
        a bound in the spec but no corresponding rejection test).
  - [ ] Independently trigger a config PUT with each rejected-value case
        against a running instance and confirm the actual HTTP status
        code (400 vs 422 vs 500) matches what the audit prompt required.

### 8. Transaction/customer-history enrichment

- **Original defect**: transaction/customer history lacked currency, POS
  occurrence time, and a store label — required MVP fields per the audit
  prompt, and a gap that could cause ambiguity in downstream
  reconciliation (which currency was a given amount actually in?).
- **Fixing commit / DEC**: `5546094` ("Enrich transaction/customer-history
  data: currency and store label (Phase 2)").
- **Trust boundaries / files**: read-path data-integrity item, not an
  access-control one. `internal/repository/postgres/ledger_repository.go`,
  `internal/httpserver/routes.go`, migration `00027`.
- **Tests**: `internal/httpserver/me_test.go` (updated assertions for the
  new fields present in `/me/transactions` responses).
- **Migration / rollout risk**: migration `00027` adds a currency column —
  confirm existing rows get a sane default (the pilot is presumably
  single-currency; verify the default matches that currency, not an
  empty/null value that would break clients expecting it populated).
- **Residual risk**: low — this is additive data enrichment, not a
  behavior change to money movement. Included here for completeness since
  it's still in the Phase 2 scope, not because it carries meaningful
  security risk.
- **Manual checklist**:
  - [ ] Confirm the currency backfill default for pre-existing rows.
  - [ ] Confirm OpenAPI documents the new fields (should already be
        covered by the `verify` job's Redocly lint / route parity test,
        but worth an independent glance at `docs/openapi/openapi.yaml`).

---

## Phase 3: QR, notifications, and availability

### 9. QR issue/consume atomicity and retry-safety

- **Original defect**: QR issue/consume cooldown acquisition wasn't
  atomic (TOCTOU race on cooldown checks); a valid QR token was
  irreversibly deleted (`GETDEL`) *before* the database work it enabled
  could be confirmed to succeed — a transient DB failure after `GETDEL`
  would burn a customer's QR token for nothing; consume-throttling used
  store identity instead of the actual API-key principal.
- **Fixing commit / DEC**: `13df1e0` ("Make QR issue/consume cooldowns
  atomic and consume retry-safe (Phase 3)") for cooldown atomicity;
  `273e8c6` ("Implement transactional outbox...") for the full
  claim/finalize/release redesign replacing bare `GETDEL`. Both
  documented together in [[#DEC-015]] (building on [[#DEC-011]]'s
  original QR design).
- **Trust boundaries / files**: POS-scan boundary — a QR token
  represents a live, single-use identification credential; losing one to
  a transient failure is a customer-facing availability bug, and a
  cooldown race is a throttling-bypass bug. `internal/service/qr.go`,
  `internal/service/qr_store.go` (issue/checkConsumeCooldown via
  `SET NX`; claim/finalize/release state machine),
  `resolveQRConsumePrincipal` in the httpserver layer.
- **Tests (incl. negative/concurrent)**: `internal/service/qr_test.go` —
  `TestQR_PayloadContainsNoPII`, `TestQR_AtomicSingleUse`,
  `TestQR_ConcurrentConsumeOnlyOneSucceeds` (the core atomicity
  guarantee, direct concurrency test),
  `TestQR_ExpiredMalformedUnknownReplayClassifiedCorrectly` (the
  deliberate "same 410 for all three cases" non-disclosure design from
  [[#DEC-011]]), `TestQR_IssueCooldownRejectsRapidReissue`,
  `TestQR_StoreIsolation_SameAccountDifferentStoresGetDifferentClients`
  (cross-store IDOR check for QR specifically),
  `TestQR_TransientFailureReleasesTokenForRetry` (the exact "don't burn
  a token on a transient DB failure" regression test),
  `TestQR_BusinessFailureFinalizesTokenNoRetry` (the flip side — a
  genuine business rejection *does* consume the token, doesn't allow
  unlimited retries), `TestQR_RedisDownReturnsErrorNotPanic`,
  `TestQR_GoneAccountDoesNotLeakExistence`.
- **Migration / rollout risk**: none schema-breaking — this is Redis-side
  state-machine logic, not a Postgres migration. Rollout risk is
  operational: any in-flight QR tokens issued under the *old* consume
  logic at the moment of a deploy could behave unexpectedly if the
  claim/finalize/release keys don't match the old bare-token format —
  worth confirming a deploy mid-QR-lifecycle (TTL default 2 minutes) is
  actually safe, or whether a short "no new pilot store traffic during
  deploy" window is warranted. QR tokens are short-lived enough that this
  is likely a non-issue, but should be a stated assumption, not an
  unstated one.
- **Residual risk**: `claim`/`finalize`/`release` adds real state-machine
  complexity versus the original one-shot `GETDEL` — more moving parts
  means more edge cases (e.g., a worker crash mid-claim, before finalize
  or release). Confirm there's a lease-TTL or equivalent self-recovery
  for a claimed-but-never-finalized token (analogous to the outbox
  worker's lease-TTL pattern per [[#DEC-015]]) — the DEC references this
  pattern by analogy but a reviewer should confirm it's actually applied
  to QR claims specifically, not just described as similar.
- **Manual checklist**:
  - [ ] Confirm (by reading `qr_store.go`, not assuming from the DEC)
        that a claimed QR token has its own lease/expiry independent of
        the original TTL, so a crashed request handler can't strand a
        token in "claimed" limbo forever.
  - [ ] Independently walk `TestQR_ConcurrentConsumeOnlyOneSucceeds` to
        confirm it actually launches concurrent goroutines against a
        real shared Redis/fake with realistic timing, not a
        artificially-serialized simulation.
  - [ ] Verify `resolveQRConsumePrincipal`'s fallback-to-store-scoped
        throttling for legacy API keys doesn't reintroduce the original
        "throttled by store, not by actual caller" gap for stores that
        haven't rotated to `store_api_keys` — cross-reference with item 1
        above (same legacy-key compatibility bridge, same residual gap).

### 10. Transactional outbox for notifications

- **Original defect**: FCM/SMS delivery happened synchronously inside the
  accrual/redeem/refund request path — a slow or failing notification
  provider directly slowed or could fail a money-moving request; push
  delivery looped sequentially per device (up to 5s each, unbounded).
- **Fixing commit / DEC**: `273e8c6` ("Implement transactional outbox for
  accrual/redeem/refund notifications (Phase 3)") and `e1f69e8` ("Batch
  FCM push delivery via SendEach instead of a sequential per-device
  loop"). [[#DEC-015]].
- **Trust boundaries / files**: availability/reliability boundary more
  than an access-control one — a notification-provider outage must not
  be able to degrade or block loyalty transaction processing.
  `internal/repository/postgres/ledger_repository.go` (writes the outbox
  row in the *same* DB transaction as the ledger entry — the core
  guarantee), `internal/service/outbox_worker.go`,
  `internal/notification/fcmchannel/fcmchannel.go`, migration `00028`.
- **Tests (incl. negative/concurrent)**: `internal/service/loyalty_test.go` —
  `TestAccrueRedeemRefund_EnqueueTransactionalOutboxNotification`,
  `TestAccrue_ZeroPointsEarnedDoesNotEnqueueNotification`,
  `TestAccrue_ReplayDoesNotDoubleEnqueueNotification` (idempotency
  applies to the notification too, not just the ledger entry);
  `internal/service/outbox_worker_test.go` —
  `TestOutboxWorker_SuccessfulDeliveryMarksDelivered`,
  `TestOutboxWorker_FailureSchedulesBackoffRetry`,
  `TestOutboxWorker_ExhaustedAttemptsDeadLetters`,
  `TestOutboxWorker_BackoffDoublesAndCapsAtMaxBackoff`,
  `TestOutboxWorker_ConcurrentDeliveryBoundedByMaxConcurrency` (proves
  the "bounded concurrency" requirement, not just "eventually delivers"),
  `TestOutboxWorker_InFlightPollSurvivesRunCtxCancellation` (graceful
  shutdown doesn't drop in-flight work — cross-references Phase 4's
  `8ca22fd` graceful-shutdown fix, worth the reviewer checking that
  connection explicitly since the two commits are in different phases).
- **Migration / rollout risk**: migration `00028` adds the
  `notification_outbox` table — purely additive. Deploy-order risk: the
  outbox worker must actually be running (as a goroutine inside the same
  process, per the current design, not a separate deploy) for queued
  notifications to ever drain — confirm this is documented in
  `docs/deployment.md` so an operator doesn't assume notifications are
  handled by something external that isn't actually running.
- **Residual risk**: dead-lettered notifications (exhausted retry
  attempts) — confirm there's an operational answer to "what happens to
  a dead-lettered accrual notification" (alerting? a manual replay path?
  silently dropped?) before pilot, since a customer silently never
  getting notified of a real accrual is a support-burden risk even though
  it's not a security bug.
- **Manual checklist**:
  - [ ] Confirm (by reading `ledger_repository.go`) the outbox insert is
        genuinely inside the same DB transaction/commit as the ledger
        write for all three of accrual, redeem, and refund — not just
        for one of them.
  - [ ] Ask/confirm the operational answer for dead-lettered
        notifications — this review can surface the gap but can't itself
        decide the policy.
  - [ ] Confirm `TestOutboxWorker_ConcurrentDeliveryBoundedByMaxConcurrency`'s
        asserted bound matches the value actually configured for the
        pilot deployment (a test proving *a* bound is enforced doesn't
        prove it's the *right* bound for expected pilot volume — that's
        also what Phase 4's load-testing baseline should have exercised;
        cross-check against `docs/slo.md` and the soak-test record).

---

## Consolidated manual review checklist

Everything below duplicates and organizes the per-item checklists so a
reviewer can work top-to-bottom without cross-referencing each section
back and forth. Each line references its item number above for full
context.

**Structural / re-run**
- [ ] Confirm CI is green at the commit under review (this document
      assumes `ab6909a`, 8/8 green — re-check if the review happens later
      against a newer commit).
- [ ] For each concurrency test cited above, confirm by reading it that
      it exercises a real race window (concurrent goroutines against
      shared state) rather than an artificially serialized sequence that
      would pass even against the pre-fix code.

**Access control (items 1, 2, 9)**
- [ ] Attempt real (not just test-suite) IDOR/self-escalation attempts
      against a running local stack: cross-store API key revoke,
      cross-org staff membership mutation, retailer_admin → platform_admin
      escalation via every membership-mutating endpoint.
- [ ] Grep every `RequireStoreAPIKey(`/`RequireStaff(` call site and
      confirm each specifies the permission/scope a reviewer would
      independently expect for that route — don't rely on the diff
      alone, check the current `routes.go`.
- [ ] Confirm the legacy `stores.api_key_hash` bridge's unscoped-trust gap
      (items 1 and 9) is a knowingly accepted pilot risk, not an
      oversight — get an explicit answer, even if the answer is "accepted
      for pilot, tracked for post-pilot."

**Money correctness (items 6, 7)**
- [ ] Hand-verify the refund rounding formula against test expectations,
      not just "tests pass."
- [ ] Judge whether a real-Postgres concurrent double-redeem integration
      test is needed before pilot (currently absent).
- [ ] Independently trigger each loyalty-config validation rejection case
      against a running instance and confirm actual HTTP status codes.

**PII / audit / data handling (items 5, 8)**
- [ ] Grep for any phone-number logging call site outside the masking
      helper.
- [ ] Attempt a direct `UPDATE`/`DELETE` against `audit_events` as the
      application's DB role against a real Postgres; confirm rejection.
- [ ] Confirm currency backfill default for pre-existing transaction rows.

**Availability / operational (items 3, 4, 9, 10)**
- [ ] Verify (don't just read) that non-development startup actually
      fails without proper SMS/FCM/TLS/trusted-proxy configuration.
- [ ] Confirm the pilot's real edge/reverse-proxy topology matches what's
      documented and configured.
- [ ] Confirm the outbox insert is in the same transaction as the ledger
      write for accrual, redeem, *and* refund (all three, independently).
- [ ] Get an explicit operational answer for dead-lettered notifications.
- [ ] Confirm a claimed-but-never-finalized QR token has its own
      self-recovery (lease/expiry), not just a description-by-analogy in
      the decision log.

**Sign-off**
- [ ] Record the outcome of this review (approved / approved-with-noted-risks
      / blocked) in `knowledge/Decisions.md` as a new DEC entry, listing
      which checklist items were worked, which were waived and why, and
      by whom — this document is materials for that review, not a
      substitute for recording its outcome.
