package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ActivityWindow aggregates one store's loyalty operations over a trailing
// time window. PointsAccrued and PointsRedeemed are both non-negative;
// SalesAmount sums accrual purchase amounts (refunds are counted in
// Operations but not subtracted — this is a traffic gauge, not accounting).
type ActivityWindow struct {
	Operations     int64
	PointsAccrued  int64
	PointsRedeemed int64
	SalesAmount    decimal.Decimal
}

// StoreActivity is the Admin Web overview's summary of a store: how many
// clients it has, when it last saw an operation, and its recent traffic.
type StoreActivity struct {
	Clients           int64
	LastTransactionAt *time.Time
	Last24h           ActivityWindow
	Last7d            ActivityWindow
}

// StoreActivityRepository computes StoreActivity. Windows are trailing
// (now-24h, now-7d) rather than calendar days, so the numbers don't depend
// on which timezone "today" would be in.
type StoreActivityRepository interface {
	StoreActivity(ctx context.Context, storeID int64, now time.Time) (*StoreActivity, error)
}
