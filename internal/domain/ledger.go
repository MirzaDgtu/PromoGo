package domain

import "context"

// LedgerRepository atomically posts a transaction and applies its point
// effect to the client's balance in a single database transaction. This
// must never be split into a separate TransactionRepository.Create followed
// by a BalanceRepository.Adjust call: if the first succeeds and the second
// fails, the transaction row becomes a permanent "ghost" — later replays of
// the same ExternalTxID (see TransactionRepository.GetByExternalID) would
// then find that ghost row and report it as already-processed forever,
// without the balance effect ever having actually applied.
type LedgerRepository interface {
	// Post inserts tx and adjusts tx.ClientID's balance by tx.PointsDelta
	// atomically, returning the posted transaction and resulting balance.
	// Returns domain.ErrConflict if a row already exists for
	// (tx.StoreID, tx.ExternalTxID) — a race between two concurrent
	// deliveries of the same webhook; the common idempotent-replay path is
	// detected earlier via TransactionRepository.GetByExternalID.
	Post(ctx context.Context, tx *Transaction) (*Transaction, *Balance, error)
}
