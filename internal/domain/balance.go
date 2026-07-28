package domain

import "context"

// Balance is a client's current point balance. Points are a count, not
// money, so they're an int64 rather than decimal.Decimal — the exchange rate
// to currency lives in LoyaltyConfig.PointsExchangeRate.
type Balance struct {
	ClientID int64
	Points   int64
}

// BalanceRepository reads Balance rows. Adjustments go through
// LedgerRepository, atomically with the transaction that causes them — see
// its doc comment.
type BalanceRepository interface {
	// Get returns the current balance for clientID. If no row exists yet,
	// it returns a zero Balance, not domain.ErrNotFound — every client
	// implicitly starts at zero points.
	Get(ctx context.Context, clientID int64) (*Balance, error)
}
