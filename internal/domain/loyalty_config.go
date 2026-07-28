package domain

import (
	"context"

	"github.com/shopspring/decimal"
)

// LoyaltyConfig holds one store's configurable loyalty-mechanic parameters
// (Idea.md's "Конфигуратор"). MVP supports only the "points" mechanic —
// AccrualPercent/MinPurchaseAmount govern accrual, the remaining fields
// govern redemption. One row per store.
type LoyaltyConfig struct {
	StoreID int64

	// Mechanic selects the domain.Mechanic implementation to run on
	// accrual (see internal/mechanicbuild). MVP only wires up "points".
	Mechanic string

	// AccrualPercent is the % of Transaction.Amount credited as points.
	AccrualPercent decimal.Decimal
	// MinPurchaseAmount is the minimum purchase amount required to earn
	// any points at all.
	MinPurchaseAmount decimal.Decimal

	// MinBalanceToRedeem is the minimum point balance required before any
	// redemption is allowed.
	MinBalanceToRedeem int64
	// MaxRedeemPercent caps redemption at this % of the transaction amount.
	MaxRedeemPercent decimal.Decimal
	// PointsExchangeRate is how much currency one point is worth when
	// redeemed (e.g. 1 point = 1.00 currency unit).
	PointsExchangeRate decimal.Decimal
}

// LoyaltyConfigRepository persists and retrieves LoyaltyConfig rows.
type LoyaltyConfigRepository interface {
	// GetByStore returns domain.ErrNotFound if storeID has no configured
	// loyalty mechanic yet.
	GetByStore(ctx context.Context, storeID int64) (*LoyaltyConfig, error)
	Upsert(ctx context.Context, cfg *LoyaltyConfig) error
}
