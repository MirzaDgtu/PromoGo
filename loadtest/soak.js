// Soak test: the same traffic mix as load.js but held at ~50% of the
// load-test peak for 15 minutes, watching for anything that only shows up
// over time — the outbox backlog growing unboundedly, latency drifting
// upward (connection/goroutine leaks), memory growth — that a short load.js
// burst wouldn't reveal. See docs/load-testing.md and docs/slo.md.
import { check, sleep } from 'k6';
import { accrue, redeem, balance } from './common.js';

export const options = {
  scenarios: {
    pos: {
      executor: 'constant-arrival-rate',
      rate: 10, // ~50% of load.js's 20 req/s steady-state target
      timeUnit: '1s',
      duration: '15m',
      preAllocatedVUs: 40,
      maxVUs: 80,
      exec: 'posFlow',
    },
    reads: {
      executor: 'constant-arrival-rate',
      rate: 5,
      timeUnit: '1s',
      duration: '15m',
      preAllocatedVUs: 20,
      maxVUs: 40,
      exec: 'readFlow',
      startTime: '5s',
    },
  },
  thresholds: {
    // Same latency/error bars as load.js (docs/slo.md) — a soak run must
    // not just "eventually finish", it must hold the SLO the whole time.
    'http_req_duration{name:accrual}': ['p(95)<300', 'p(99)<800'],
    'http_req_duration{name:redeem}': ['p(95)<300', 'p(99)<800'],
    'http_req_duration{name:balance}': ['p(95)<500'],
    checks: ['rate>0.999'],
  },
};

export function setup() {
  const res = accrue('setup', 0, 1000);
  if (res.status !== 200) {
    throw new Error(`setup: seed accrual failed with status ${res.status}: ${res.body}`);
  }
  return { clientID: JSON.parse(res.body).client_id };
}

export function posFlow() {
  const accrualRes = accrue(__VU, __ITER, 1000);
  // See load.js's posFlow for why 429 is accepted here — run this against
  // an instance with raised ratelimit.accrual_* limits (docs/load-testing.md).
  const accrualOK = check(accrualRes, { 'accrual handled': (r) => r.status === 200 || r.status === 429 });
  if (!accrualOK || accrualRes.status !== 200) {
    return;
  }
  const body = JSON.parse(accrualRes.body);
  if (body.balance >= 10) {
    const redeemRes = redeem(__VU, __ITER, body.client_id, 10, 10);
    check(redeemRes, { 'redeem handled': (r) => r.status === 200 || r.status === 422 || r.status === 409 || r.status === 429 });
  }
}

export function readFlow(data) {
  check(balance(data.clientID), { 'balance handled': (r) => r.status === 200 || r.status === 429 });
  sleep(0.1);
}
