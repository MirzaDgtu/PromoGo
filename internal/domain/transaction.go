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
// index on (StoreID, ExternalTxID) is what makes Accrue idempotent under
// webhook retries (see Idea.md's "Идемпотентность").
type Transaction struct {
	ID           int64
	StoreID      int64
	ClientID     int64
	ExternalTxID string
	Amount       decimal.Decimal
	Type         TransactionType
	PointsDelta  int64
	CreatedAt    time.Time
}

// TransactionRepository reads Transaction rows. Writes go through
// LedgerRepository, which posts a transaction and its balance effect
// atomically — see its doc comment for why a plain Create here would be
// unsafe.
type TransactionRepository interface {
	// GetByExternalID returns domain.ErrNotFound if no transaction exists
	// for (storeID, externalTxID) yet. Callers use this to detect a
	// replayed webhook and skip re-running the accrual/redemption logic.
	GetByExternalID(ctx context.Context, storeID int64, externalTxID string) (*Transaction, error)
	ListByClient(ctx context.Context, clientID int64) ([]*Transaction, error)
}
