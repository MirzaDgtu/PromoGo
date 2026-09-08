// Package domain holds PromoGo's core entities and the interfaces every
// other layer implements. It has no external dependencies.
package domain

import "errors"

// ErrNotFound is returned by repository lookups that find no matching row.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a write would violate a uniqueness
// constraint — a duplicate (store_id, external_tx_id) transaction, or a
// phone number already registered for a store's client.
var ErrConflict = errors.New("conflict")

// ErrInsufficientBalance is returned by Redeem when the client's balance
// cannot cover the requested redemption.
var ErrInsufficientBalance = errors.New("insufficient balance")

// ErrIdempotencyConflict is returned when a request reuses a (store_id,
// type, external_tx_id) key that a prior request already processed, but
// with different parameters (client or amount) — so it can't be treated as
// a replay of that prior request.
var ErrIdempotencyConflict = errors.New("idempotency conflict")

// ErrCannotRefundRefund is returned when a refund's original transaction
// reference itself points at a refund — refunds may only reverse an
// accrual or a redeem, never chain off another refund.
var ErrCannotRefundRefund = errors.New("cannot refund a refund")

// ErrOverRefund is returned when a refund's amount, alone or combined with
// previously posted partial refunds against the same original transaction,
// would exceed that transaction's original amount.
var ErrOverRefund = errors.New("refund exceeds original transaction amount")

// ErrAmbiguousOriginalTransaction is returned when a refund omits the
// original transaction's type and external_tx_id matches both an accrual
// and a redeem under the same store — external_tx_id uniqueness is scoped
// per type, so both can legitimately exist and the caller must disambiguate.
var ErrAmbiguousOriginalTransaction = errors.New("ambiguous original transaction: specify original_transaction_type")

// ErrDailyRedeemLimitExceeded is returned when posting a redemption would
// push the client's rolling-window redeemed-points total past the
// configured anti-fraud daily limit (see AntiFraudConfig).
var ErrDailyRedeemLimitExceeded = errors.New("daily redeem limit exceeded")
