# ADR-0001: CustomerAccount identity scope (global vs. tenant-scoped)

Status: **proposed** — needs product/legal sign-off before the "decided"
behavior below can be treated as final. Filed per
`docs/audit-remediation-prompt.md` Phase 5, first bullet.

## Context

`CustomerAccount` (`internal/domain/customer.go:17-30`) is today a single
global row keyed by phone number, created only via OTP verification
(`internal/service/customerauth.go`). `Client` (`internal/domain/client.go`)
is the store-scoped participant: one row per `(store_id, phone)`, optionally
linked to a `CustomerAccount`. On `VerifyOTP`, `linkExistingClients`
(`customerauth.go:250`) finds and links **every** `Client` row across
**every** organization that shares the verified phone number — there is no
organization boundary in the linking query today.

This repo serves multiple, unrelated retailers (`Organization` is already
the tenant boundary for staff RBAC — DEC-004). A phone number that has
shopped at Retailer A and Retailer B currently resolves to the *same*
`CustomerAccount.ID` and the *same* `CustomerSession`/refresh-token chain
across both. `CustomerConsent` (`customer.go:89-100`) is also a single
global table with no organization column, so a consent grant is recorded
once per phone, not once per retailer relationship.

The concrete risk this ADR must resolve, per the remediation prompt: does
any staff- or store-facing surface let one retailer infer that a customer
also transacts at another retailer, and is a single global consent record
legally sufficient for 152-FZ purposes when the customer is really entering
into separate relationships with separate data controllers (each
`Organization` is presumably its own legal entity/operator)?

## Options considered

**A. Fully global identity (current behavior, unmodified).** One
`CustomerAccount`, one login, one consent, auto-linked across every
retailer. Simplest; matches "one mobile app, works everywhere" UX. Fails
the tenant-isolation and per-controller-consent requirements outright: a
retailer's staff tooling that resolves `Client → CustomerAccountID →
CustomerAccount` has no schema-level barrier stopping it from being
extended to enumerate a customer's other stores, and 152-FZ consent
recorded once cannot legally stand in for consent to *each* retailer's
processing.

**B. Fully tenant-scoped identity.** Drop `CustomerAccount` entirely;
identity becomes `(organization_id, phone)`. A customer re-verifies via OTP
per retailer and gets a separate login/session/balance history at each.
Cleanest isolation, but destroys the actual product value proposition (one
app, one phone number, no re-registration friction per store) and
duplicates OTP-delivery cost (SMS) per retailer for the same human.

**C. Mixed: global authentication identity, tenant-scoped visibility and
consent (recommended).** Keep exactly one `CustomerAccount` per phone as
the **authentication** anchor (one OTP flow, one refresh-token chain, one
mobile login) — this is infrastructure, not retailer-visible data. But:

- Every retailer/staff-facing read path must resolve data through `Client`
  scoped to `Client.StoreID`'s `Organization`, never through
  `CustomerAccountID` directly across organizations. No endpoint may accept
  a `CustomerAccountID` and return data (profile, balance, transaction
  history) belonging to a `Client` in a different organization than the
  caller's.
- `linkExistingClients` keeps its current behavior (link every existing
  `Client` with the phone) because that behavior is what makes "one login
  everywhere" work — but linking is an identity operation, not a data
  grant. It must never be read as authorizing cross-organization data
  disclosure.
- `CustomerConsent` gains an `organization_id` column (nullable initially
  for migration; nullable = "platform-level consent," e.g. account
  deletion terms) and consent is captured **per organization the customer
  transacts with**, not once globally. A single global privacy-policy
  consent at OTP signup covers the platform operator's own processing
  (phone number storage, OTP delivery); each retailer's loyalty-program
  terms require their own consent event scoped to that `organization_id`,
  captured on first transaction/registration at that retailer.
- Right-to-deletion and data export (see `docs/legal-minimum.md`) operate
  at two granularities: delete/export "everything for this phone"
  (platform-wide) or delete/export "my data at Organization X" (single
  `Client` + its `CustomerConsent` rows + anonymize/detach that
  `Client.CustomerAccountID`), so a customer can exit one retailer's
  program without losing their account at others.
- Account merging (two `CustomerAccount`s later found to be the same human
  — e.g. after a phone-number change) is out of scope for this ADR; MVP
  has no merge path and none is being built. Phone-number change is
  tracked as Q-P0-053 (open, deferred — see
  `knowledge/Project Questions.md`).

## Decision

**Option C**, proposed pending product/legal sign-off, specifically on:
whether each `Organization` is legally a distinct data controller under
152-FZ (if yes, per-organization consent is a hard requirement, not a
nice-to-have), and whether the product wants "one account, invisible
cross-retailer linking under the hood" to be customer-disclosed in the
privacy policy.

## Consequences / migration plan

1. Add `organization_id BIGINT NULL REFERENCES organizations(id)` to
   `customer_consents` (new migration; nullable, not backfilled — existing
   rows stay platform-level). New consent-capture call sites pass the
   organization the customer is registering/transacting with.
2. Audit every handler in `internal/httpserver` that reads customer/client
   data reachable from a `CustomerAccountID` and confirm it filters by the
   caller's `Organization` via `Client.StoreID`, never by
   `CustomerAccountID` alone. Add a regression test asserting a store-scoped
   API key from Org A cannot retrieve a `Client`/balance/transaction row
   belonging to Org B even when both share a `CustomerAccountID` (extends
   the existing cross-store isolation tests referenced in Phase 1's
   remediation).
3. Document the "one login, org-scoped visibility" model in the privacy
   policy (`docs/legal-minimum.md`).
4. Leave `linkExistingClients`'s current cross-org linking query unchanged
   — it is the mechanism that makes Option C's "one login" property work.

## Not decided here

- Whether a customer can *opt out* of cross-retailer identity linking
  (e.g. request that their phone not auto-link at a new retailer) — flagged
  as a follow-up product question, not blocking pilot.
- Account merging UX/flow.
