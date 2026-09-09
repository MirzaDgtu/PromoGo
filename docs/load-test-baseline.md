# Load/soak/dependency-failure baseline run

Recorded **2026-09-09**, against commit `c0976bb91067314e652590b5a098c9d4f8bc599e`
(plus the `requestTimeoutMW` fix this run itself motivated, folded into the
same commit batch — see below). Environment: this developer's machine,
Docker Desktop for Windows, native Go build for the app (not the container
image) for `smoke.js`/`load.js`/`soak.js`; the actual `docker-compose.yml`
stack (containerized app included) for `dependency-failure.sh`. Treat
absolute latency numbers as a floor on real production-grade hardware, not
a ceiling — see `docs/load-testing.md`.

Re-run and update this file whenever a change plausibly affects
request-path performance (ledger locking, rate-limit rules, the outbox
worker's concurrency) — see `docs/load-testing.md`'s closing note.

## smoke.js

```
checks_total.......: 20      17.366984/s
checks_succeeded...: 100.00% 20 out of 20
✓ healthz 200
✓ accrual 200
✓ accrual has client_id
✓ balance 200
http_req_duration..: avg=9.24ms p(95)=29.39ms
```

## load.js

First attempt (default rate limits) failed — not an SLO violation but a
test-setup bug: a single k6 container is one caller (one IP, one store
key), and `ratelimit.accrual_ip_limit`/`accrual_principal_limit` default
to 5-10 req/s, sized for one pilot store's realistic traffic, not the
20 req/s load-test target. Most requests correctly got 429'd by the rate
limiter — proving the rate limiter works, but not exercising the request
path at the target rate. Fixed by (a) raising those limits for the
load-test run specifically (`docs/load-testing.md`) and (b) correcting
`loadtest/load.js`'s `check()`s to treat 429 as a handled response, not a
failure, matching `docs/slo.md`'s own stated policy — both real, needed
fixes, not the result being discarded.

**Re-run with load-test-scale rate limits — result: all `docs/slo.md`
thresholds passed with wide margin:**

```
█ THRESHOLDS
  checks
  ✓ 'rate>0.999' rate=100.00%
  http_req_duration{name:accrual}
  ✓ 'p(95)<300' p(95)=13.18ms
  ✓ 'p(99)<800' p(99)=15.69ms
  http_req_duration{name:balance}
  ✓ 'p(95)<500' p(95)=4.12ms
  http_req_duration{name:redeem}
  ✓ 'p(95)<300' p(95)=10.88ms
  ✓ 'p(99)<800' p(99)=13.24ms

█ TOTAL RESULTS
  checks_total.......: 8619    43.077924/s
  checks_succeeded...: 100.00% 8619 out of 8619
  checks_failed......: 0.00%   0 out of 8619
  http_reqs..........: 8620    43.082922/s (over 3m20s, ramping 1->40 req/s)
```

(`http_req_failed` in the raw k6 output shows 16.74% — that's k6's own
status-code-based tag, which counts every non-2xx including an accepted
429 as "failed" regardless of `check()` results. The `checks` metric above
is what actually reflects `docs/slo.md`'s policy, and it's 100%.)

**Reading this**: at pilot scale (and the 5-10x headroom this target
already carries), POS webhook latency has enormous margin under the
<300ms/p95 target — p95 was 13ms, about 4% of budget. This says nothing
about behavior at real multi-store-chain scale or under resource
contention this single-machine test can't produce (see caveats above);
it says the code path itself isn't the bottleneck at anything like pilot
traffic.

## soak.js

15-minute run at ~50% of load.js's peak (10 req/s POS + 5 req/s reads).
*(Filled in after this document's first commit — see the follow-up commit
for the actual run's thresholds and the `promogo_outbox_backlog` samples
taken every minute during it.)*

## dependency-failure.sh

```
--- baseline: transaction succeeds before any outage ---
OK: baseline 200
--- pausing redis (simulating an outage) ---
OK: transaction during redis outage (attempt 1) -> 503
OK: transaction during redis outage (attempt 2) -> 503
OK: transaction during redis outage (attempt 3) -> 503
--- unpausing redis ---
OK: transaction after redis recovery -> 200
--- pausing postgres (simulating an outage) ---
OK: transaction during postgres outage (attempt 1) -> 500
OK: transaction during postgres outage (attempt 2) -> 500
OK: transaction during postgres outage (attempt 3) -> 500
--- unpausing postgres ---
OK: transaction after postgres recovery -> 200
--- checking for panics during the outages ---
OK: no panics logged
=== PASSED ===
```

This run is *after* the `requestTimeoutMW` fix. Before it: pausing
Postgres (a real SIGSTOP-level freeze, not a refused connection) hung the
request indefinitely instead of the 500 seen above — no context deadline
existed on the request path, unlike Redis calls (go-redis has default
dial/read/write timeouts, which is why the Redis-pause case already
returned a clean 503 on the very first attempt at this test). A few such
hangs would have exhausted `Postgres.MaxConns` and taken the *entire*
service down for every request, not just ones touching the frozen
dependency — a real cascading-failure risk this test surfaced and the fix
closes. See `knowledge/Decisions.md` DEC-016 and
`internal/httpserver/middleware_timeout.go`.

## Migration-deploy-model (separate from this document's four scripts, but
same "reproducible, not just documented" bar)

Covered by CI job `migration-deploy-model`
(`.github/workflows/ci.yml`) and `docs/deployment.md`, which includes its
own local-repro transcript.
