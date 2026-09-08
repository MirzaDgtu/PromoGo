# PromoGo audit remediation prompt

Use the following prompt to implement the remediation work identified during the full PromoGo audit.

---

You are working in the PromoGo repository: a Go backend for a retail loyalty platform that receives purchase events from 1C/POS, calculates and redeems points, exposes customer-mobile APIs, and provides staff/admin APIs.

Your objective is to make the current backend safe for a production pilot while preserving its layered architecture and existing API contracts wherever possible. Work incrementally, in priority order. Do not overwrite unrelated or pre-existing changes in the working tree. Before changing code, read `AGENTS.md`, use the existing `graphify-out/graph.json` as instructed, inspect current tests and migrations, and verify every audit claim against the current source because the repository may have changed since this prompt was written.

For each independent fix:

1. reproduce the defect with a failing test when practical;
2. implement the smallest complete fix at the correct domain/repository boundary;
3. update interfaces, fakes, migrations, OpenAPI and documentation together;
4. run focused tests, then the complete verification suite;
5. report changed files, behavior changes, migration/rollout concerns and remaining risks.

Never weaken authentication, authorization, tenant isolation, idempotency or auditability to make a test pass. Avoid broad rewrites. Use PostgreSQL transactions and constraints for durable invariants and Redis Lua scripts for multi-command atomicity. Return stable public errors without exposing secrets or PII.

## Phase 0: baseline and release gates

- Confirm Go `1.25.14` is used consistently in `go.mod`, the Docker build image, CI and README. If a separate `toolchain` directive is introduced later, it must not select an older release. Do not downgrade Go.
- Run `go mod verify`, `gofmt -l .`, `go vet ./...`, `go test -shuffle=on -count=1 ./...`, `staticcheck ./...` and `govulncheck ./...`.
- Fix the invalid inline YAML near `expires_in` in `docs/openapi/openapi.yaml` and make the Redocly lint job pass. Add appropriate non-success responses for health/readiness if required by the lint policy.
- Preserve and run the route/OpenAPI security parity test.
- Validate all Goose migrations. Add a CI integration job that migrates a real PostgreSQL database up from zero and, where safe, verifies rollback/re-apply behavior.
- Treat every failing check as a release blocker or document an explicit, narrowly scoped exception with an owner and expiry.

## Phase 1: security and tenant-isolation blockers

### Store API keys

- Fix the mismatch where `GenerateAPIKey` stores the hash of only the secret half but middleware hashes the full `<keyID>.<secret>` string.
- Parse and strictly validate the key format, look up the row by the public `keyID`, hash/compare the secret in constant time, then enforce expiry, revocation and scopes.
- Do not log plaintext keys or secret hashes. Return the plaintext only once on creation.
- Preserve legacy-key compatibility only through an explicit isolated fallback. Add a migration/removal plan; legacy keys must not silently retain unlimited scopes forever.
- Add an end-to-end test: create a key through the admin API, authenticate a permitted store request, reject a missing scope, reject a revoked/expired key and reject a modified secret.

### RBAC and tenant IDOR

- A `retailer_admin` must never be able to grant, obtain or revoke `platform_admin` privileges. Only an already authorized platform admin may manage that role.
- Scope staff membership reads and mutations by `organization_id` and, where applicable, `store_id`; never update a tenant-owned row by global numeric ID alone.
- Verify that a requested `store_id` belongs to the organization in the route.
- Scope API-key revoke by both key ID and store ID. Check `RowsAffected` and return not-found for cross-tenant or absent resources.
- Prevent self-escalation, accidental self-disable and removal/disable of the last active administrator according to an explicit policy.
- Replace nullable uniqueness that permits duplicate organization-wide memberships with partial unique indexes or `NULLS NOT DISTINCT`. Use composite foreign keys/constraints where they improve tenant integrity.
- Add authorization-matrix tests covering platform admin, retailer admin, store manager and support viewer, including adversarial organization/store IDs and guessed resource IDs.

### OTP and customer sessions

- Make OTP verification/consumption atomic in one Redis Lua script: correct codes are single-use, wrong-attempt increments cannot be lost, the challenge is removed at the maximum attempt count and Redis errors are never ignored.
- Make request counters set their expiry atomically. Make issue/resend cooldown acquisition atomic with `SET NX` or Lua.
- Define safe behavior when SMS delivery fails after reserving a challenge/cooldown; do not leave the user unnecessarily locked out.
- Consider HMAC/pepper protection for six-digit OTP hashes because the code space is small.
- Make refresh-token rotation one PostgreSQL transaction. Atomically claim an unrevoked old session, create the replacement and link it. Zero affected rows must trigger reuse handling rather than mint another session.
- Add real concurrency tests proving that one OTP and one refresh token cannot produce multiple successful consumptions.
- Enforce `blocked` and `deleted` customer status during refresh and protected requests. Revoke all sessions when an account is blocked/deleted and introduce an account/session version or equivalent mechanism when immediate access-token invalidation is required.

### Configuration and transport security

- Validate SMS provider as a strict enum. Unknown production values must fail startup and must never fall back to the log sender.
- Require HTTPS for SMS endpoints outside explicit local development.
- Support and require PostgreSQL TLS and Redis TLS in non-development environments.
- Document the required edge TLS/reverse-proxy topology.
- Consolidate client-IP resolution. Apply trusted-proxy rules consistently to OTP throttling, staff/customer audit and request logs. Parse proxy chains from the trusted edge inward and fail startup on malformed trusted-proxy CIDRs.
- Validate all security-relevant TTLs, limits, ports, log levels and OIDC settings. Unknown or invalid production values must fail fast.
- Construct PostgreSQL connection settings through `pgx`/URL-safe configuration rather than interpolating unescaped credentials into a DSN.
- Centralize strict JSON decoding: reject unknown fields, trailing JSON documents and oversized bodies consistently. Normalize phone input in every equivalent customer lookup path.

### PII and audit trail

- Remove raw phone numbers, tokens and credentials from repository errors and logs. Introduce centralized masking or keyed pseudonymous identifiers and a documented logging schema/retention policy.
- Make audit recording durable for security-sensitive mutations. Prefer the same database transaction or a transactional outbox; an RBAC/key/config mutation must not succeed without a durable audit record.
- Record organization/store creation, staff changes, API-key lifecycle, loyalty-config changes, relevant client-data access, account lifecycle and administrative transaction access.
- Include actor, tenant scope, target, request/correlation ID, timestamp and safe before/after metadata.
- Protect audit rows from application-level update/delete using database privileges or append-only enforcement. Add retention/export requirements and a composite keyset-pagination index such as `(organization_id, occurred_at DESC, id DESC)`.

## Phase 2: loyalty and concurrency correctness

- Fix idempotent refund replay so it returns a stable result. A replay of an earlier partial refund must not report that refund amount as the cumulative total or drift after later refunds. Persist the result snapshot if necessary.
- Add tests for multiple partial refunds, replay before/after later refunds, the final rounding correction and over-refund attempts.
- Move `MinBalanceToRedeem` eligibility into the same locked ledger transaction as the balance change. Add concurrent redemption tests.
- Validate loyalty configuration at the HTTP/domain boundary: supported mechanic enum, percentages in `[0,100]`, positive exchange rate and internally consistent thresholds. Map validation/constraint errors to `400` or `422`, not `500`.
- Version loyalty rules and persist the rule version/effective configuration needed to reconstruct every calculation. Provide audit, history and rollback for configuration changes.
- Keep the immutable purchase/refund event distinct from ledger entries as mechanics expand. Design explicit adjustment, reservation, pending and expiration entry types before introducing them.
- Add currency, POS occurrence time, cashier/device, receipt or item reference, rule version and tenant identity to transaction/event data where required. Use additive, backward-compatible migrations.
- Include purchase amount, currency, merchant/store label and operation date in customer history as required by the MVP.

## Phase 3: QR, notifications and availability

- Make QR issue and consume cooldown acquisition atomic with Redis Lua or `SET NX`.
- Do not irreversibly `GETDEL` a valid QR before database work can succeed. Implement a claim/finalize/release state machine or another retry-safe design.
- Use the actual API-key identity as the consume principal when per-key throttling is intended; document whether throttling is per key or per store.
- Add concurrency, Redis-failure, PostgreSQL-failure, replay, expiry and cooldown tests. Return consistent errors and `Retry-After` where relevant.
- Remove synchronous SMS/FCM delivery from the transaction request path. Implement a PostgreSQL transactional outbox and worker with idempotent delivery, bounded concurrency, exponential backoff, dead-letter handling and delivery metrics.
- Batch or safely parallelize push delivery; do not perform an unbounded sequential five-second call per device.
- Replace deprecated Firebase APIs. Prefer Application Default Credentials/workload identity over raw service-account JSON where the deployment platform supports it.
- Move notification text into versioned/localizable templates and define deep-link, language and fallback behavior instead of hardcoding Russian strings in loyalty services.
- Decide and document Redis outage behavior separately for security-sensitive customer auth and POS transaction processing. If POS requests fail closed, the 1C retry/offline queue becomes mandatory and must be tested end to end. If a fallback is allowed, make it bounded, observable and explicitly accepted as a security/business tradeoff.

## Phase 4: infrastructure, CI and operations

- Add `.dockerignore` excluding `.git`, local `.env` files, graph outputs, build artifacts and unrelated local data. Ensure secrets cannot enter remote build context/cache.
- Upgrade the runtime Alpine image to a supported release, pin build/runtime images by patch version and preferably digest, and document the update process.
- Mark Docker Compose as development-only. Avoid exposing PostgreSQL/Redis to all host interfaces by default and do not rely on default credentials on shared hosts.
- Remove `@latest` tooling from repeatable Make/CI commands; pin Goose, linters and scanners.
- Add CI gates for `staticcheck`, `govulncheck`, race tests, migration integration tests, repository tests, OpenAPI lint/contract tests, container build, image vulnerability scan, SBOM and secret scanning. Pin GitHub Actions by immutable SHA where practical.
- Add request/correlation IDs, panic recovery, metrics and tracing. At minimum expose latency/error histograms and business metrics for idempotency conflicts, ledger divergence, refund failures, OTP issue/verify, QR consume, rate-limit rejection, outbox backlog and delivery outcomes.
- Centralize response writing and log unexpected encode/write failures without leaking response contents. Parse authorization schemes robustly while keeping token validation strict.
- Define SLOs for POS p95/p99 latency and availability. Run load, soak and dependency-failure tests against PostgreSQL/Redis; prove the stated sub-300-ms target rather than assuming it.
- Document backup/restore, RPO/RTO, retention/cleanup for sessions, OTP state, devices, audit and transaction history, plus restore drills.
- Reconcile graceful-shutdown duration with outbound-call limits and worker draining. Avoid untracked per-request background goroutines such as best-effort `TouchLastUsed` calls.
- Decouple production schema migration from every application replica startup. Use expand/migrate/contract deployments for incompatible schema changes.

## Phase 5: product decisions and full MVP

- Produce an ADR deciding whether `CustomerAccount` is global across all retailers or scoped by organization/tenant. Address consent, tenant isolation, right to deletion, account merging and cross-retailer data visibility. Do not silently retain the current global phone-linking behavior without explicit product/legal approval.
- Produce an ADR for shared-network, isolated-store and mixed balance modes before adding more mechanics.
- Resolve multi-tenant SaaS versus dedicated deployments and propagate the decision through schema, authorization, onboarding, billing and operational isolation.
- Keep the backend contract ready for, but do not pretend to implement, the missing Flutter app, React configurator and 1C integration in this repository. Define versioned APIs and build a real pilot E2E test covering 1C/POS event, accrual, mobile visibility, QR/phone identification, redemption, offline retry and duplicate delivery.
- Close or explicitly defer every open P0/P1 entry in `knowledge/Project Questions.md`; each deferral needs rationale, owner, risk and target milestone.
- Define the legal minimum for the pilot: consent records, privacy policy/version, data export/deletion, localization, retention and 152-FZ deployment responsibilities.
- Add or correct the repository license file so the README license claim is backed by an actual license artifact.

## Required tests and acceptance criteria

The work is complete only when all of the following are true:

- A newly created store API key authenticates correctly; cross-tenant key/membership operations are impossible.
- No retailer-scoped role can create or acquire platform-wide privileges.
- Concurrent OTP and refresh-token reuse tests yield exactly one success.
- Blocked/deleted accounts cannot refresh or use protected APIs according to the documented invalidation SLA.
- Loyalty balance, minimum-redemption and multi-partial-refund invariants hold under concurrency in real PostgreSQL integration tests.
- QR state is single-use and retry-safe across Redis/PostgreSQL failures.
- Security-sensitive mutations have durable, tenant-correct audit events and logs contain no raw phone/token data.
- Transaction endpoints do not synchronously wait for external notification providers.
- OpenAPI documents the actual status codes, schemas and security for every route and passes lint plus route parity tests.
- `gofmt`, build, vet, unit/integration tests, race detector, staticcheck, govulncheck, migration validation and OpenAPI lint all pass.
- Container scanning finds no unaccepted critical/high vulnerabilities, and no secret enters the image or build context.
- Documentation lists deliberate residual risks and operational runbooks. Do not claim production readiness while any security or data-integrity P0 remains open.

At the end, provide a concise report grouped into: completed changes, migrations/rollout, tests and evidence, behavior/API changes, unresolved product decisions, and remaining risks.

---
