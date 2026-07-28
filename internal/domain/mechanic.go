package domain

import "context"

// Mechanic computes the points earned by a single accrual transaction. It is
// the pluggable-per-store analogue of a trading strategy: LoyaltyService
// resolves one Mechanic per store (via LoyaltyConfig.Mechanic) and calls
// Accrue on every incoming transaction. Implementations must be pure
// decision logic — no side effects (persistence, notifications) belong here,
// that's LoyaltyService's job. Additional mechanics (cashback, punch card,
// tiers — see Idea.md) are added by implementing this interface, not by
// touching LoyaltyService.
type Mechanic interface {
	// Name identifies the mechanic, matching LoyaltyConfig.Mechanic.
	Name() string
	// Accrue returns the number of points tx earns under cfg, given the
	// client's balance before this transaction (some mechanics, e.g. tiers,
	// need the current balance/tier to pick a rate). Returns 0 with a nil
	// error when the transaction doesn't qualify (e.g. below
	// MinPurchaseAmount) — that is not an error condition.
	Accrue(ctx context.Context, tx *Transaction, cfg *LoyaltyConfig, balance *Balance) (pointsEarned int64, err error)
}
