package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// LoyaltyConfigRepository is a pgx-backed implementation of
// domain.LoyaltyConfigRepository.
type LoyaltyConfigRepository struct {
	pool *pgxpool.Pool
}

// NewLoyaltyConfigRepository creates a LoyaltyConfigRepository backed by pool.
func NewLoyaltyConfigRepository(pool *pgxpool.Pool) *LoyaltyConfigRepository {
	return &LoyaltyConfigRepository{pool: pool}
}

func (r *LoyaltyConfigRepository) GetByStore(ctx context.Context, storeID int64) (*domain.LoyaltyConfig, error) {
	const query = `
		SELECT store_id, mechanic, accrual_percent, min_purchase_amount, min_balance_to_redeem, max_redeem_percent, points_exchange_rate
		FROM loyalty_configs
		WHERE store_id = $1`

	cfg := &domain.LoyaltyConfig{}
	err := r.pool.QueryRow(ctx, query, storeID).Scan(
		&cfg.StoreID, &cfg.Mechanic, &cfg.AccrualPercent, &cfg.MinPurchaseAmount,
		&cfg.MinBalanceToRedeem, &cfg.MaxRedeemPercent, &cfg.PointsExchangeRate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("loyalty config for store %d: %w", storeID, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get loyalty config for store %d: %w", storeID, err)
	}

	return cfg, nil
}

func (r *LoyaltyConfigRepository) Upsert(ctx context.Context, cfg *domain.LoyaltyConfig) error {
	const query = `
		INSERT INTO loyalty_configs (store_id, mechanic, accrual_percent, min_purchase_amount, min_balance_to_redeem, max_redeem_percent, points_exchange_rate)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (store_id) DO UPDATE SET
			mechanic = EXCLUDED.mechanic,
			accrual_percent = EXCLUDED.accrual_percent,
			min_purchase_amount = EXCLUDED.min_purchase_amount,
			min_balance_to_redeem = EXCLUDED.min_balance_to_redeem,
			max_redeem_percent = EXCLUDED.max_redeem_percent,
			points_exchange_rate = EXCLUDED.points_exchange_rate`

	_, err := r.pool.Exec(ctx, query,
		cfg.StoreID, cfg.Mechanic, cfg.AccrualPercent, cfg.MinPurchaseAmount,
		cfg.MinBalanceToRedeem, cfg.MaxRedeemPercent, cfg.PointsExchangeRate,
	)
	if err != nil {
		return fmt.Errorf("upsert loyalty config for store %d: %w", cfg.StoreID, err)
	}

	return nil
}
