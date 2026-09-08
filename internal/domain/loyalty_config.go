package domain

import (
	"context"
	"fmt"

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

// Validate checks the numeric invariants a LoyaltyConfig must satisfy
// regardless of Mechanic (which mechanic names are valid is checked
// separately, by internal/mechanicbuild — domain can't depend on it without
// an import cycle). Returns an error wrapping ErrInvalidLoyaltyConfig,
// which the HTTP layer maps to 400/422, never 500.
func (c LoyaltyConfig) Validate() error {
	switch {
	case c.Mechanic == "":
		return fmt.Errorf("mechanic is required: %w", ErrInvalidLoyaltyConfig)
	case c.AccrualPercent.IsNegative() || c.AccrualPercent.GreaterThan(decimal.NewFromInt(100)):
		return fmt.Errorf("accrual_percent must be between 0 and 100: %w", ErrInvalidLoyaltyConfig)
	case c.MinPurchaseAmount.IsNegative():
		return fmt.Errorf("min_purchase_amount must not be negative: %w", ErrInvalidLoyaltyConfig)
	case c.MinBalanceToRedeem < 0:
		return fmt.Errorf("min_balance_to_redeem must not be negative: %w", ErrInvalidLoyaltyConfig)
	case c.MaxRedeemPercent.IsNegative() || c.MaxRedeemPercent.GreaterThan(decimal.NewFromInt(100)):
		return fmt.Errorf("max_redeem_percent must be between 0 and 100: %w", ErrInvalidLoyaltyConfig)
	case !c.PointsExchangeRate.IsPositive():
		// Zero/negative would make points either worthless or divide-by-zero
		// undefined wherever redemption converts points back to currency
		// (see internal/service/loyalty.go's Redeem).
		return fmt.Errorf("points_exchange_rate must be positive: %w", ErrInvalidLoyaltyConfig)
	}
	return nil
}

// LoyaltyConfigRepository persists and retrieves LoyaltyConfig rows.
type LoyaltyConfigRepository interface {
	// GetByStore returns domain.ErrNotFound if storeID has no configured
	// loyalty mechanic yet.
	GetByStore(ctx context.Context, storeID int64) (*LoyaltyConfig, error)
	Upsert(ctx context.Context, cfg *LoyaltyConfig) error
}
