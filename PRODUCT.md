# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Two audiences, equally important from the pilot onward (confirmed 2026-10-29):

- **Retailer staff** — owner or manager of a store network. Comes to Admin Web
  periodically to onboard a store, issue a cash-register API key, add staff,
  tune the loyalty mechanic, and look at clients and their balances.
- **Platform admin** — the PromoGo team. Works across all organizations
  (DEC-025), sets up retailers on their behalf, investigates via the audit log.

End customers never see Admin Web; they live in the mobile app. Cashiers have
no UI at all: identification happens via QR, card, or phone number at the POS.

## Product Purpose

PromoGo is an independent loyalty platform for retail. 1C is only an event
source: it sends completed sales to the loyalty API, and the loyalty core
(ledger, accrual/redemption rules, anti-fraud) lives in its own service and
database. Admin Web is the operating tool for configuring and observing that
core without code changes.

Pilot success (MVP-scope.md): accrual and redemption match 1C data with no
discrepancies, a target number of customers registers in the first month, and
the retailer and cashiers give feedback usable as a sales case.

## Positioning

The loyalty program is not a 1C plug-in: the retailer owns a separate
loyalty system with its own ledger, so the program survives ERP changes and
works across stores. Customer identity is mobile-first (dynamic QR with TTL),
with phone number as the cashier fallback.

## Operating Context

- Desktop-first, tablet supported; phones are out of scope for Admin Web.
- Light and dark theme follow `prefers-color-scheme`; there is no toggle.
- Interface language is Russian; all strings live in `web/admin/src/i18n/strings.ts`.
- Staff log in via OIDC (provider still open, DEC-004); access is
  permission-based, not role-name-based (DEC-005).
- Typical tasks: store onboarding checklist (DEC-031), one-time display of a
  new store API key secret, adding staff via access code (DEC-029), editing
  the loyalty config under optimistic locking (DEC-027), paging and filtering
  the audit log (DEC-030).

## Capabilities and Constraints

- Screens: Login, Overview (store onboarding + activity), Organizations and
  Stores, API keys, Staff and access, Clients (read-only balances),
  Loyalty config, Audit, Coming-soon placeholders.
- MVP mechanic is points only (% of purchase, minimum purchase, redemption
  rules: max % of receipt, minimum balance, exchange rate). Tiers, punch
  cards, cashback, promos, analytics dashboards are out of MVP scope.
- A store with no loyalty config still accepts sales at 0 points (DEC-028).
- Stack: React 19, Vite, react-router 7, TanStack Query, react-hook-form + zod,
  plain CSS with tokens in `src/index.css`, no UI library. The API client is
  generated from `docs/openapi/openapi.yaml`.
- Multi-store network modes, billing, and multi-tenancy pricing are undecided.

## Brand Commitments

None. No logo, color, or typeface is binding (confirmed 2026-10-29); the
current favicon and blue accent are not commitments.

## Evidence on Hand

Real pilot data is not yet available. There are no customer testimonials,
metrics, or case studies; do not fabricate them. A local dev store («Океан»)
and the `possim` POS simulator exist for generating test activity.

## Product Principles

1. The ledger is the truth: every balance-affecting number shown must be
   traceable to transactions and the audit log.
2. Secrets are shown once and handled deliberately (API keys, access codes).
3. Configuration without code: a retailer should change mechanics safely,
   with conflicts surfaced rather than silently overwritten.
4. Serve both the occasional retailer and the cross-organization operator
   without separate products.

## Accessibility & Inclusion

WCAG 2.2 AA is required (confirmed 2026-10-29): AA contrast in both themes,
visible focus, full keyboard operation, and `prefers-reduced-motion` respected.
