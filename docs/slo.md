# Pilot workload model and SLOs

**Status: proposed, needs retailer/business sign-off** — these numbers are
derived from `MVP-scope.md`/`Idea.md`'s stated pilot scope (one store,
webhook target <300ms) plus reasonable engineering judgment for the parts
those docs don't quantify (availability, error rate, outbox lag). Treat
every number below as a starting point for a real SLO conversation with the
business owner, not a committed contract — see `knowledge/Project
Questions.md` for how this should be tracked as an open decision until
someone with product authority signs off.

## Workload model

The pilot (`MVP-scope.md`) is **one retail store**, one cash register flow,
one integrated 1C instance. Realistic peak load for a single physical
store: a handful of concurrent checkout lanes, each producing at most one
webhook call per completed sale (rarely faster than one every 10-30s per
lane in practice). That puts genuine pilot peak load at roughly:

| Flow | Realistic pilot peak | Load-test target (with headroom) |
|---|---|---|
| POS accrual/redeem webhook (`POST /api/v1/transactions`, `/redeem`) | ~1-3 req/s | **20 req/s** sustained, 40 req/s burst |
| Client lookup/balance (`GET /api/v1/clients/...`) | <1 req/s | 10 req/s |
| Mobile balance/history reads (`GET /api/v1/me/...`) | a few hundred registered customers polling occasionally | 10 req/s |
| OTP issue/verify | bursty, low absolute volume | 5 req/s |
| QR issue/resolve | bursty around checkout | 10 req/s |

The load-test targets deliberately carry 5-10x headroom over the realistic
pilot peak: `MVP-scope.md`'s stated goal is "готовый кейс для продажи
следующим клиентам" (a case to sell to the next customers), so validating
against a small-chain-scale load, not just the single pilot store's actual
traffic, is what makes a pass here worth anything for that pitch. It is
**not** validated against a large multi-store chain's traffic — that would
need its own workload model once such a deal is real.

## Proposed SLOs

**Availability** — POS ingestion path (`/api/v1/transactions`,
`/api/v1/transactions/redeem`): **99.5% monthly**, measured as the
fraction of requests that receive a non-5xx, non-503 response. Deliberately
*not* 99.9%+: this deployment has no HA Postgres/Redis (single instance
each, see `docs/runbooks.md` #2 and #3) and fails closed on Redis
unavailability — a higher number would be a promise the current
infrastructure can't back up. Getting to 99.9%+ requires HA
Postgres/Redis, which is out of scope for a single-pilot MVP.

**Latency** (`Idea.md`'s <300ms target, expressed as percentiles):
- POS webhook (accrual/redeem/refund): **p95 < 300ms, p99 < 800ms**.
- Client lookup/balance, mobile `/me/*` reads: **p95 < 500ms**.
- OTP issue/verify, QR issue/resolve: **p95 < 500ms**.

**Error rate:** **< 0.1%** 5xx responses under normal load (excludes 429s —
a rejected-by-design rate-limit response is not a failure) and excludes the
Redis/Postgres-outage windows covered by `docs/runbooks.md` #2/#3 (fail-
closed 503s during a genuine dependency outage are correct behavior, not an
SLO violation of *this* service — they're an availability cost of the
dependency's own outage).

**Notification outbox lag** (Phase 3's transactional outbox,
`internal/service/outbox_worker.go`): under normal load, **p95 delivery
within 30s of enqueue**, **0% dead-lettered**. `promogo_outbox_backlog`
(see `internal/metrics`) should return to baseline within 2 minutes of a
load spike ending.

## How these are checked

Not just asserted here — `loadtest/` has runnable k6 scripts that assert
these exact thresholds and fail non-zero if violated:

- `loadtest/smoke.js` — minimal single-VU correctness check (every request
  succeeds, no thresholds beyond "no errors") — see `docs/load-testing.md`.
- `loadtest/load.js` — ramps to the load-test targets above and asserts
  every latency/error-rate SLO in this document as a k6 `thresholds` block.
- `loadtest/soak.js` — the same traffic mix at ~50% of peak, sustained
  15 minutes, watching for the outbox backlog growing unboundedly, memory/
  connection leaks (indirectly, via latency drift over the run), or
  anything degrading over time that a short burst wouldn't reveal.
- `loadtest/dependency-failure.sh` — orchestrates pausing/unpausing the
  Redis and Postgres containers mid-load and asserts the documented
  fail-closed behavior (503s, not 500s or panics; timely recovery once the
  dependency returns — see `docs/runbooks.md` #2/#3).

See `docs/load-testing.md` for how to run these and the baseline run this
repo has recorded evidence for.
