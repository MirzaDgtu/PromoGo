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

// DefaultCurrency is the only currency the MVP supports — every store
// transacts in rubles. See Transaction.Currency.
const DefaultCurrency = "RUB"

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
	// Currency is the ISO 4217 code Amount is denominated in. The MVP has
	// no multi-currency support, so this is always DefaultCurrency, set by
	// the service layer at write time (see internal/service/loyalty.go).
	Currency    string
	Type        TransactionType
	PointsDelta int64
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
	// RuleVersion is the LoyaltyConfig.Version in effect when this
	// accrual/redeem was posted (see LoyaltyConfigRepository.Upsert and
	// internal/service/loyalty.go's Accrue/Redeem) — resolvable back to the
	// exact configuration via LoyaltyConfigRepository.ListHistory, so a past
	// calculation stays reconstructable after later config changes. nil on
	// refund rows, which don't consult the mechanic config (see
	// LedgerRepository.PostRefund).
	RuleVersion *int64

	// OriginalTransactionID is set only when Type is TransactionRefund: the
	// ID of the accrual/redeem transaction this refund reverses. nil for
	// every other type (see transactions_refund_reference_check).
	OriginalTransactionID *int64
	// RefundedAmount and RefundedPoints are cumulative totals kept on an
	// accrual/redeem row as partial refunds are posted against it (see
	// LedgerRepository.PostRefund) — always zero on accrual/redeem rows
	// that haven't been refunded, and meaningless (left at zero) on refund
	// rows themselves.
	RefundedAmount decimal.Decimal
	RefundedPoints int64
	// RefundCumulativeAmount and RefundFullyRefunded are set only when Type
	// is TransactionRefund: a snapshot, taken at write time, of the
	// original transaction's RefundedAmount/fully-refunded status
	// immediately after this refund was posted (see
	// LedgerRepository.PostRefund). Replaying this refund request must
	// return this snapshot rather than re-reading the original row, whose
	// cumulative total can have advanced since if later refunds were
	// posted against it. Meaningless (left at zero/false) on accrual/redeem
	// rows.
	RefundCumulativeAmount decimal.Decimal
	RefundFullyRefunded    bool
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
	// GetByID returns domain.ErrNotFound if no transaction exists with this
	// ID. Used to re-read a refund's original transaction (by
	// Transaction.OriginalTransactionID) when reporting a replayed refund's
	// FullyRefunded status.
	GetByID(ctx context.Context, id int64) (*Transaction, error)
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
