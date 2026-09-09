# Legal minimum for the pilot (152-FZ)

Status: **proposed** — the policy text, retention periods, and localization
choices below need actual legal review before the pilot goes live with real
customer data; this document defines the *engineering* minimum needed to
support whatever legal ultimately requires, and records exactly what is
built vs. still a gap. Filed per `docs/audit-remediation-prompt.md` Phase 5,
sixth bullet. See `docs/adr/0001-customer-account-scoping.md` for how
consent scoping interacts with tenant boundaries.

## What exists today

- `CustomerConsent` (`internal/domain/customer.go:89-105`) — an immutable,
  append-only record of one consent grant: `DocumentVersion`, `GrantedAt`,
  `Source`, `IP`, `UserAgent`, tied to `CustomerAccountID`. A changed
  consent is a new row, never an update — this is already the right shape
  for "prove what the customer agreed to and when."
- `CustomerAccountStatus` includes `CustomerAccountDeleted`
  (`customer.go:11-15`) — the lifecycle state exists; the deletion
  *workflow* (what gets scrubbed, what's retained for legal/audit reasons,
  who can trigger it) does not yet exist as a runnable path.
- `audit_events` (Phase 1 remediation) is append-only at the DB level
  (trigger-enforced — see the db-migrate skill's audit_events migration),
  which is the retention-side guarantee a real deletion/export flow will
  need to reconcile against: audit rows referencing a deleted customer
  can't simply cascade-delete without destroying the audit trail itself.

## What's missing (gaps, each with an owner/target)

1. **Consent capture is not yet organization-scoped.** Per ADR-0001, a
   single global consent grant is not sufficient once multiple retailers
   are legally distinct data controllers. Adding `organization_id` to
   `customer_consents` is ADR-0001's migration step #1 — not yet applied.
   *Owner: backend. Target: before onboarding a second retailer* (the
   pilot's single-organization scope makes this non-blocking today).
2. **No data export endpoint.** A customer (or their retailer, on their
   behalf) has no way to request "everything you hold on this phone
   number" as a machine-readable export. *Owner: backend. Target: before
   pilot go-live if legal confirms 152-FZ requires self-service export at
   this stage* — otherwise a manual DBA-run export satisfies the
   requirement for a single-digit-customer-count pilot and this can slip
   to the first real multi-tenant rollout.
3. **No data deletion workflow.** `CustomerAccountDeleted` exists as a
   status but nothing sets it, and nothing defines what "deleted" actually
   scrubs: phone number, OTP history, and consent grants are candidates
   for hard deletion; `transactions`/`balances`/`audit_events` rows likely
   must be retained (financial/audit record) but with the customer
   identifier anonymized rather than the row deleted, matching the
   audit-append-only guarantee above. *Owner: backend + legal (retention
   vs. deletion conflict needs a legal call, not an engineering guess).
   Target: before pilot go-live* — 152-FZ's right-to-deletion is not
   optional even for a small pilot once real phone numbers are collected.
4. **Retention periods are undefined** (Q-P0-074, open). Nothing currently
   expires or archives `transactions`, `audit_events`, `customer_consents`,
   or `otp_attempts`-style tables. *Owner: legal to set the period;
   backend to implement a retention job once set. Target: before pilot
   go-live for the periods; the job itself can follow shortly after if
   volume is low enough that no data has actually aged out yet.*
5. **No published privacy policy or public offer document.** `MVP-scope.md`
   names this requirement but no artifact exists in this repository (nor
   should it — this is legal/product content, not code). `Source`/
   `DocumentVersion` on `CustomerConsent` are ready to record acceptance of
   whatever text legal produces. *Owner: product/legal. Target: before
   pilot go-live — consent capture is meaningless without a document to
   consent to.*
6. **Data-controller/processor roles undefined** (Q-P0-072, open): is the
   platform operator the "operator" (controller) under 152-FZ, or is each
   retailer, with the platform as processor? This determines who is
   legally answerable for a deletion request and who must be named in the
   privacy policy. *Owner: legal. Target: before pilot go-live* — this is
   a prerequisite for gap #5's document, not independent of it.
7. **Data residency undefined** (Q-P0-073, open): physical location of the
   production database, backups, logs, and any Firebase/FCM-adjacent
   storage. 152-FZ has data-localization requirements for Russian
   citizens' personal data specifically. *Owner: legal + infra (depends on
   `docs/adr/0003-multi-tenant-deployment.md`'s eventual platform choice).
   Target: before pilot go-live if the pilot's customers are Russian
   citizens (assumed yes, given 152-FZ's applicability at all) — this
   gates the platform choice, not just a config value.*
8. **Field-level encryption scope undefined** (Q-P0-076, open): which
   columns (phone, IP addresses in `customer_sessions`/`customer_consents`)
   need encryption at the application/DB level beyond whatever the hosting
   platform provides at rest. *Owner: backend + legal. Target: before
   pilot go-live for phone number at minimum, given it's the primary
   personal identifier in this schema.*
9. **Localization** — no localization gap specific to legal text has been
   identified beyond "the policy documents themselves must exist in
   Russian" (covered by gap #5); no separate engineering work is implied
   unless legal requires multi-language consent flows, which is not
   expected for a Russian-market pilot.

## What "pilot go-live" legal-minimum actually requires (summary)

Gaps #1 is non-blocking at single-organization pilot scale. Gaps #5, #6,
#7 are legal-team deliverables this document cannot close by itself —
engineering is blocked on them, not the other way around. Gaps #3, #4, #8
need a legal *decision* (retention period, deletion scope, encryption
scope) before the engineering implementation in each can start; none of
the three requires new schema beyond what's already described. Gap #2 can
reasonably slip past go-live for a pilot this small if legal agrees a
manual export process is acceptable at this scale — record that agreement
explicitly if taken, rather than letting the gap go unacknowledged.
