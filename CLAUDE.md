# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Status

PromoGo is in **active MVP backend development**. The repository contains a
Go service, PostgreSQL migrations, Docker-based local infrastructure, and a
store-authenticated loyalty API (accrual, redemption, balance/phone lookup).
The loyalty core has been through two security-audit passes: cross-store
isolation, type-scoped idempotency with request fingerprinting, atomic
redemption, and advisory-locked self-migrating startup are all in place and
covered by `internal/service/loyalty_test.go`. The Flutter application,
React configurator, OTP/QR flows, real FCM/SMS delivery, OpenAPI contract, and
production operations are not implemented yet.

Use `knowledge/Project Questions.md` as the decision backlog and
`knowledge/Decisions.md` as the record of accepted answers. Product scope lives
in `MVP-scope.md`, `Full-scope.md`, and `Idea.md`.

## Product Concept

An independent loyalty program platform for retail. Key architectural decision: **1C (the Russian ERP system) is only an event source** — it sends purchase webhooks to the loyalty API. The loyalty system is a fully separate service with its own database and rules engine.

The end customer interacts exclusively via a mobile app. There is no cashier-facing UI — identification happens via QR code, physical card, or phone number.

## Intended Tech Stack

| Component | Technology |
|---|---|
| Backend API | Go |
| Database | PostgreSQL |
| Cache / Queues | Redis |
| Mobile App | Flutter (iOS + Android) |
| Web Configurator | React |
| Push Notifications | Firebase FCM |
| SMS | SMSC / SMS.ru |
| 1C Integration | REST API (HTTP-service on 1C side) |

## Planned Architecture

### Services
- **Backend API** — loyalty core: accrual/redemption engine, REST API consumed by both 1C and the mobile app
- **Web Configurator** — admin panel for configuring loyalty mechanics without code changes; two access levels (retailer = basic settings, platform admin = complex rules)
- **Mobile App** — customer-facing: balance, dynamic QR code (TTL-limited), transaction history, push notifications

### Key Integration: 1C → Loyalty API
1C sends a webhook on every completed sale:
```
POST /api/v1/transactions
{ store_id, client_id, amount, items[], timestamp }
```
Response carries `points_earned`, updated `balance`, and current `tier`. Separate endpoints handle balance lookup and redemption confirmation.

Offline resilience: if 1C cannot reach the API, events queue locally in 1C and replay on reconnect.

### Loyalty Mechanics (all configurable)
Points, cashback, punch cards (stamp N → reward), tiers (Bronze/Silver/Gold with different accrual rates), time-limited promotions, category-based bonuses.

### Multi-store Modes
Configurable per retailer: unified network (shared balance across all stores) or isolated stores (independent programs).

### Client Identification at POS
1. Dynamic QR code from mobile app (TTL-protected)
2. Physical card (barcode / card number)
3. Phone number entered by cashier

Cashier-side registration flow: cashier enters phone → system creates account → customer receives SMS with app link.

## Full Concept
See `Idea.md` for the complete product specification including analytics requirements, marketing campaign flows, and anti-fraud rules.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. If `graphify` is not on `PATH`, use `python -m graphify`. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
