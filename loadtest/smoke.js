// Minimal correctness check: one VU, a handful of iterations, every
// request must succeed. Run this before load.js/soak.js to catch a broken
// seed/config before spending time on a full run. See docs/load-testing.md.
import { check, sleep } from 'k6';
import { accrue, balance, healthz } from './common.js';

export const options = {
  vus: 1,
  iterations: 5,
  thresholds: {
    http_req_failed: ['rate==0'],
  },
};

export default function () {
  check(healthz(), { 'healthz 200': (r) => r.status === 200 });

  const res = accrue(__VU, __ITER, 1000);
  const ok = check(res, {
    'accrual 200': (r) => r.status === 200,
    'accrual has client_id': (r) => JSON.parse(r.body).client_id > 0,
  });
  if (ok) {
    const clientID = JSON.parse(res.body).client_id;
    check(balance(clientID), { 'balance 200': (r) => r.status === 200 });
  }

  sleep(0.2);
}
