// Package points provides the MVP domain.Mechanic implementation: a flat
// percentage of the purchase amount is credited as points. It is intended
// as a scaffold for plugging in real mechanics (cashback, punch card,
// tiers), not as the final word on loyalty logic.
package points

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// Mechanic implements domain.Mechanic: pointsEarned = floor(amount *
// accrual_percent / 100), or 0 if the transaction is below
// cfg.MinPurchaseAmount.
type Mechanic struct{}

// New creates a points mechanic.
func New() *Mechanic {
	return &Mechanic{}
}

// Name returns the mechanic identifier, matching domain.LoyaltyConfig.Mechanic.
func (m *Mechanic) Name() string {
	return "points"
}

// Accrue implements domain.Mechanic.
func (m *Mechanic) Accrue(_ context.Context, tx *domain.Transaction, cfg *domain.LoyaltyConfig, _ *domain.Balance) (int64, error) {
	if tx.Amount.LessThan(cfg.MinPurchaseAmount) {
		return 0, nil
	}

	points := tx.Amount.Mul(cfg.AccrualPercent).Div(decimal.NewFromInt(100))
	return points.Floor().IntPart(), nil
}
