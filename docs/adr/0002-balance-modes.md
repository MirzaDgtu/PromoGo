# ADR-0002: shared-network, isolated-store and mixed balance modes

Status: **proposed** — needs product sign-off. Filed per
`docs/audit-remediation-prompt.md` Phase 5, second bullet.

## Context

Today, exactly one balance mode is implemented, and it isn't configurable:
`Balance` (`internal/domain/balance.go`) is keyed `PRIMARY KEY` on
`client_id` (migration `00003_create_balances.sql`), and `Client` is 1:1
with `(store_id, phone)` (`internal/domain/client.go:9-10,19-25`,
explicitly documented as MVP scope). `LedgerRepository.Post`/`PostRefund`
(`internal/domain/ledger.go`) always resolve and lock a balance by
`ClientID`. So a customer who shops at two stores under the same
`Organization` today has two completely independent point balances — this
is "isolated store" mode, hard-coded, with no "network" (shared-balance)
mode existing anywhere in schema or code. Migration `00012` explicitly
notes multi-store network mode was deferred out of the org-scoping work
(Q-P1-113/114).

`Organization` already exists as the natural grouping for a "network," so
the schema is not starting from nothing — but there's a real data-model
fork depending on the decision here, and the remediation prompt requires
it be resolved *before* adding more loyalty mechanics on top of the
current single-mode assumption.

## Options considered

**A. Isolated-store only (ship what exists).** Every `Client`/`Balance`
stays store-scoped forever; a "network" retailer would need N separate
loyalty programs, one per store, with no way to unify them. Zero schema
change. Fails any retailer whose actual business model is "one loyalty
program, many storefronts" (a very common retail chain shape) — this is
plausible for the platform's target customers, not a hypothetical.

**B. Shared-network only (balance lives on the organization).** Move
`Balance`'s key from `client_id` to `(organization_id, customer_account_id)`
and drop the per-store balance entirely. Simplest single mode to reason
about, but breaks any retailer that legitimately wants isolated
storefronts (e.g. franchisees who don't want to subsidize each other's
redemptions), and is a breaking schema change for the one thing already
shipped and tested (Phase 2's concurrency-correctness work,
`internal/service/loyalty_test.go`).

**C. Mixed — configurable per organization (recommended).** Add a
`balance_mode` setting (`isolated` | `network`, default `isolated`) to
`LoyaltyConfig`/`Organization`. `isolated` keeps today's exact behavior
unchanged (`Balance` keyed by `client_id`). `network` mode introduces a
new balance keyed by `(organization_id, customer_account_id)`, and
accrual/redeem resolve which balance to lock based on the org's mode —
requiring `CustomerAccountID` to be resolved (not just `ClientID`) before
posting, which only works once the customer has verified via OTP and
linked (ADR-0001); an unlinked `Client` in a `network`-mode organization
has no balance to accrue to until OTP verification happens, which must be
a documented product constraint, not a silent bug.

## Decision

**Option C, `isolated` shipped now and made the explicit default,
`network` designed but not implemented for pilot.** Rationale: the pilot
scope (`MVP-scope.md`) is a single store, so nothing forces a decision on
network semantics today, but the schema needs the seam now — retrofitting
a balance-key change after real transaction data exists is materially
worse than adding an unused enum column now. This is `proposed` because
committing to *any* specific network-balance schema (option C's
`(organization_id, customer_account_id)` key) is a bet on how a real
multi-store chain customer will want it to work, which needs a real
customer/sales conversation, not just this ADR.

"Mixed" (some stores in an org networked, others isolated) is
**explicitly out of scope** for this decision — it roughly doubles the
resolution-logic surface (which stores in an org participate in the shared
pool) for a case no current or prospective pilot customer has asked for.
If it's needed later, it's an extension of the `network` mode's
organization-level setting to a per-store participation flag, not a new
top-level mode.

## Consequences / migration plan

1. Add `balance_mode TEXT NOT NULL DEFAULT 'isolated' CHECK (balance_mode
   IN ('isolated','network'))` to `loyalty_configs` (new migration).
   `network` is accepted by the schema but rejected at the service layer
   (`domain.ErrNotImplemented` or similar) until built — so the column
   exists and defaults correctly without a code path that can silently
   mis-post to the wrong balance.
2. When `network` mode is actually built: add a `network_balances` table
   `(organization_id, customer_account_id) PRIMARY KEY`, mirroring
   `balances`' structure (`points BIGINT CHECK (points >= 0)`, same
   upsert-then-update pattern documented in the db-migrate skill's CHECK
   constraint gotcha). `LedgerRepository.Post`/`PostRefund` gain a
   mode-aware balance-resolution step: `isolated` locks `balances` by
   `ClientID` (unchanged); `network` locks `network_balances` by
   `(OrganizationID, CustomerAccountID)`, requiring `CustomerAccountID` to
   be non-null (unlinked customers cannot accrue in network mode — surface
   this as a clear error, not a fallback to a per-store balance).
3. `GET .../balance` (Q-P1-112, open) resolves which table to read based
   on the store's organization's `balance_mode` — this endpoint's exact
   response shape should be finalized alongside `network` mode's
   implementation, not before.
4. Existing isolated-mode behavior, tests, and the Phase 2 concurrency
   guarantees are unaffected by step 1 alone.
