package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// TransactionType distinguishes an accrual from a redemption or a refund.
type TransactionType string

const (
	TransactionAccrual TransactionType = "accrual"
	TransactionRedeem  TransactionType = "redeem"
	TransactionRefund  TransactionType = "refund"
)

// Transaction records one purchase/redemption event and its point effect.
// ExternalTxID is the transaction_id 1C sends with the webhook — the unique
// index on (StoreID, Type, ExternalTxID) is what makes Accrue/Redeem
// idempotent under webhook retries (see Idea.md's "Идемпотентность"). Type
// is part of that key because accrual and redemption are separate
// operations that may legitimately reuse the same receipt/check number.
type Transaction struct {
	ID           int64
	StoreID      int64
	ClientID     int64
	ExternalTxID string
	Amount       decimal.Decimal
	Type         TransactionType
	PointsDelta  int64
	// BalanceAfter is the client's point balance immediately after this
	// transaction was posted, snapshotted by LedgerRepository.Post. Replays
	// return this stored value rather than the client's current balance, so
	// a replayed request's response never drifts from the original one.
	BalanceAfter int64
	// RequestFingerprint is a canonical encoding of the request parameters
	// that produced this transaction (see internal/service's
	// accrualFingerprint/redeemFingerprint), snapshotted at write time. A
	// replayed (StoreID, Type, ExternalTxID) request must recompute the same
	// fingerprint to be treated as a genuine replay — a mismatch means the
	// ID was reused for a materially different request.
	RequestFingerprint string
	CreatedAt          time.Time
}

// TransactionCursor is a keyset-pagination continuation point: the
// (CreatedAt, ID) of the last transaction returned on the previous page.
// Using both fields (not just CreatedAt) breaks ties when multiple
// transactions share a timestamp.
type TransactionCursor struct {
	CreatedAt time.Time
	ID        int64
}

// TransactionRepository reads Transaction rows. Writes go through
// LedgerRepository, which posts a transaction and its balance effect
// atomically — see its doc comment for why a plain Create here would be
// unsafe.
type TransactionRepository interface {
	// GetByExternalID returns domain.ErrNotFound if no transaction exists
	// for (storeID, txType, externalTxID) yet. Callers use this to detect a
	// replayed webhook and skip re-running the accrual/redemption logic.
	GetByExternalID(ctx context.Context, storeID int64, txType TransactionType, externalTxID string) (*Transaction, error)
	ListByClient(ctx context.Context, clientID int64) ([]*Transaction, error)
	// ListByClientIDs returns up to limit transactions across every client
	// in clientIDs, newest first (created_at DESC, id DESC as tiebreaker).
	// If before is non-nil, only rows strictly older than that cursor are
	// returned — the continuation point from a prior page's last item. Used
	// by the customer-facing /me/transactions endpoint to merge a
	// customer's history across every store they have a linked Client in,
	// in one query instead of one query per store plus an in-memory merge.
	ListByClientIDs(ctx context.Context, clientIDs []int64, limit int, before *TransactionCursor) ([]*Transaction, error)
}
