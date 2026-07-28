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
