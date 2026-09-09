package domain

import (
	"context"
	"fmt"
	"time"

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

	// Version increments on every Upsert (starting at 1 on first write).
	// Transaction.RuleVersion snapshots this at accrual/redemption time, so
	// a past calculation stays reconstructable after later config changes.
	// Set by LoyaltyConfigRepository.Upsert; ignored on input.
	Version int64
}

// LoyaltyConfigVersion is one historical snapshot of a store's
// LoyaltyConfig, appended by Upsert every time the config changes (see
// loyalty_config_history). It's the audit/history trail Phase 2 requires:
// "provide audit, history and rollback for configuration changes."
type LoyaltyConfigVersion struct {
	StoreID int64
	Version int64

	Mechanic           string
	AccrualPercent     decimal.Decimal
	MinPurchaseAmount  decimal.Decimal
	MinBalanceToRedeem int64
	MaxRedeemPercent   decimal.Decimal
	PointsExchangeRate decimal.Decimal

	// ChangedByStaffUserID is nil if the write that produced this version
	// wasn't attributable to a staff principal (e.g. a system migration).
	ChangedByStaffUserID *int64
	CreatedAt            time.Time
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
	// Upsert writes cfg as storeID's new effective configuration, assigning
	// it the next Version (1 on first write) and appending this same new
	// configuration to history under that version — atomically, so a
	// version is never skipped or duplicated under concurrent writes, and a
	// transaction's RuleVersion can always be resolved via ListHistory even
	// after later Upserts change the live row. changedBy is nil if the
	// write isn't attributable to a staff principal. cfg.Version is set to
	// the newly assigned version on return.
	Upsert(ctx context.Context, cfg *LoyaltyConfig, changedBy *int64) error
	// ListHistory returns storeID's configuration history, newest version
	// first.
	ListHistory(ctx context.Context, storeID int64) ([]*LoyaltyConfigVersion, error)
}
