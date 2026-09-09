// Shared helpers for loadtest/*.js — see docs/load-testing.md for how to
// run these and docs/slo.md for what the thresholds mean.
import http from 'k6/http';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
export const API_KEY = __ENV.API_KEY;

if (!API_KEY) {
  throw new Error(
    'API_KEY env var is required — seed one with `promogo loadtest-seed` ' +
    'and pass it via -e API_KEY=... (see docs/load-testing.md)'
  );
}

export function authHeaders() {
  return { Authorization: `Bearer ${API_KEY}`, 'Content-Type': 'application/json' };
}

// accrue posts a POS accrual webhook for a synthetic phone number unique to
// (vu, iter, timestamp) so repeated runs never collide on transaction_id or
// pile every VU's traffic onto one client row.
export function accrue(vu, iter, amount) {
  const phone = syntheticPhone(vu, iter);
  const txID = `loadtest-accrual-${vu}-${iter}-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
  return http.post(
    `${BASE_URL}/api/v1/transactions`,
    JSON.stringify({ transaction_id: txID, phone, amount }),
    { headers: authHeaders(), tags: { name: 'accrual' } }
  );
}

export function redeem(vu, iter, clientID, amount, points) {
  const txID = `loadtest-redeem-${vu}-${iter}-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
  return http.post(
    `${BASE_URL}/api/v1/transactions/redeem`,
    JSON.stringify({ transaction_id: txID, client_id: clientID, amount, points }),
    { headers: authHeaders(), tags: { name: 'redeem' } }
  );
}

export function balance(clientID) {
  return http.get(`${BASE_URL}/api/v1/clients/${clientID}/balance`, {
    headers: authHeaders(),
    tags: { name: 'balance' },
  });
}

export function healthz() {
  return http.get(`${BASE_URL}/healthz`, { tags: { name: 'healthz' } });
}

function syntheticPhone(vu, iter) {
  // A stable but bounded pool of synthetic phone numbers per VU (not one
  // per iteration) so repeated iterations exercise the idempotency and
  // balance-accumulation paths against a real, growing client, not just
  // first-accrual-ever every time.
  const bucket = iter % 20;
  return `+7900${String(vu).padStart(4, '0')}${String(bucket).padStart(3, '0')}`;
}
