// Load test: ramps to this pilot's load-test targets (docs/slo.md) and
// asserts the SLOs from that document as k6 thresholds — a non-zero exit
// code means an SLO was violated, not just "something looked slow" in a
// report a human has to interpret. See docs/load-testing.md for how to run
// this and where the recorded baseline run's results live.
import { check, sleep } from 'k6';
import { accrue, redeem, balance } from './common.js';

export const options = {
  scenarios: {
    // POS accrual/redeem: ramps 0 -> 20 req/s (docs/slo.md's load-test
    // target for this flow), holds, then a 40 req/s burst.
    pos: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: 60,
      maxVUs: 120,
      exec: 'posFlow',
      stages: [
        { target: 5, duration: '20s' },
        { target: 20, duration: '40s' },
        { target: 20, duration: '90s' },
        { target: 40, duration: '20s' }, // burst
        { target: 0, duration: '20s' },
      ],
    },
    // Client balance/lookup reads, held steady through the same window.
    reads: {
      executor: 'constant-arrival-rate',
      rate: 10,
      timeUnit: '1s',
      duration: '190s',
      preAllocatedVUs: 20,
      maxVUs: 40,
      exec: 'readFlow',
      startTime: '10s',
    },
  },
  thresholds: {
    // docs/slo.md: POS webhook p95<300ms, p99<800ms.
    'http_req_duration{name:accrual}': ['p(95)<300', 'p(99)<800'],
    'http_req_duration{name:redeem}': ['p(95)<300', 'p(99)<800'],
    // docs/slo.md: client lookup/balance p95<500ms.
    'http_req_duration{name:balance}': ['p(95)<500'],
    // docs/slo.md: <0.1% error rate under normal load. A 429 (rate-limit
    // rejection) is by-design, not a failure — see docs/slo.md — so this
    // checks the response-status-derived expected_response tag, which k6
    // marks false only for a >=400 we didn't explicitly accept via `check`.
    checks: ['rate>0.999'],
  },
};

// setup() runs once, in its own VU, before any scenario starts — used here
// to guarantee at least one client already exists (with a known ID) before
// the "reads" scenario's VUs (which never run posFlow themselves, since k6
// isolates each scenario's VU pool) need one to query.
export function setup() {
  const res = accrue('setup', 0, 1000);
  if (res.status !== 200) {
    throw new Error(`setup: seed accrual failed with status ${res.status}: ${res.body}`);
  }
  return { clientID: JSON.parse(res.body).client_id };
}

export function posFlow() {
  const accrualRes = accrue(__VU, __ITER, 1000);
  // 429 is the rate limiter working as designed (see docs/slo.md's
  // exclusion), not an SLO violation — but this scenario's target rate is
  // meant to genuinely load the request path, not repeatedly trip
  // per-IP/per-key limits from a single load-generator source. Run this
  // script against an instance with raised ratelimit.accrual_* limits (see
  // docs/load-testing.md) so a 429 here is rare, not the common case; the
  // check still accepts it so an occasional one doesn't fail the run.
  const accrualOK = check(accrualRes, { 'accrual handled': (r) => r.status === 200 || r.status === 429 });
  if (!accrualOK || accrualRes.status !== 200) {
    return;
  }
  const body = JSON.parse(accrualRes.body);

  if (body.balance >= 10) {
    const redeemRes = redeem(__VU, __ITER, body.client_id, 10, 10);
    // 200 = redeemed; 422/409 = a business rule correctly rejected this
    // particular redemption (e.g. daily limit); 429 = rate limiter — all
    // three are a *working* response, only a 5xx/timeout counts against
    // the SLO here.
    check(redeemRes, { 'redeem handled': (r) => r.status === 200 || r.status === 422 || r.status === 409 || r.status === 429 });
  }
}

export function readFlow(data) {
  check(balance(data.clientID), { 'balance handled': (r) => r.status === 200 || r.status === 429 });
  sleep(0.05);
}
