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
		SELECT store_id, mechanic, accrual_percent, min_purchase_amount, min_balance_to_redeem, max_redeem_percent, points_exchange_rate, version
		FROM loyalty_configs
		WHERE store_id = $1`

	cfg := &domain.LoyaltyConfig{}
	err := r.pool.QueryRow(ctx, query, storeID).Scan(
		&cfg.StoreID, &cfg.Mechanic, &cfg.AccrualPercent, &cfg.MinPurchaseAmount,
		&cfg.MinBalanceToRedeem, &cfg.MaxRedeemPercent, &cfg.PointsExchangeRate, &cfg.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("loyalty config for store %d: %w", storeID, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get loyalty config for store %d: %w", storeID, err)
	}

	return cfg, nil
}

// Upsert implements domain.LoyaltyConfigRepository.Upsert — see its doc
// comment. Locking the current row (if any) for the duration of the
// transaction is what makes "assign the next version" and "write it" atomic
// together: without the lock, two concurrent config writes for the same
// store could both compute the same next version.
func (r *LoyaltyConfigRepository) Upsert(ctx context.Context, cfg *domain.LoyaltyConfig, changedBy *int64) error {
	dbTx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("upsert loyalty config for store %d: begin tx: %w", cfg.StoreID, err)
	}
	defer dbTx.Rollback(ctx)

	const lockCurrent = `SELECT version FROM loyalty_configs WHERE store_id = $1 FOR UPDATE`
	var currentVersion int64
	err = dbTx.QueryRow(ctx, lockCurrent, cfg.StoreID).Scan(&currentVersion)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("upsert loyalty config for store %d: lock current: %w", cfg.StoreID, err)
	}
	cfg.Version = currentVersion + 1

	const upsert = `
		INSERT INTO loyalty_configs (store_id, mechanic, accrual_percent, min_purchase_amount, min_balance_to_redeem, max_redeem_percent, points_exchange_rate, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (store_id) DO UPDATE SET
			mechanic = EXCLUDED.mechanic,
			accrual_percent = EXCLUDED.accrual_percent,
			min_purchase_amount = EXCLUDED.min_purchase_amount,
			min_balance_to_redeem = EXCLUDED.min_balance_to_redeem,
			max_redeem_percent = EXCLUDED.max_redeem_percent,
			points_exchange_rate = EXCLUDED.points_exchange_rate,
			version = EXCLUDED.version`
	_, err = dbTx.Exec(ctx, upsert,
		cfg.StoreID, cfg.Mechanic, cfg.AccrualPercent, cfg.MinPurchaseAmount,
		cfg.MinBalanceToRedeem, cfg.MaxRedeemPercent, cfg.PointsExchangeRate, cfg.Version,
	)
	if err != nil {
		return fmt.Errorf("upsert loyalty config for store %d: %w", cfg.StoreID, err)
	}

	const insertHistory = `
		INSERT INTO loyalty_config_history (store_id, version, mechanic, accrual_percent, min_purchase_amount, min_balance_to_redeem, max_redeem_percent, points_exchange_rate, changed_by_staff_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err = dbTx.Exec(ctx, insertHistory,
		cfg.StoreID, cfg.Version, cfg.Mechanic, cfg.AccrualPercent, cfg.MinPurchaseAmount,
		cfg.MinBalanceToRedeem, cfg.MaxRedeemPercent, cfg.PointsExchangeRate, changedBy,
	)
	if err != nil {
		return fmt.Errorf("upsert loyalty config for store %d: insert history: %w", cfg.StoreID, err)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return fmt.Errorf("upsert loyalty config for store %d: commit: %w", cfg.StoreID, err)
	}

	return nil
}

func (r *LoyaltyConfigRepository) ListHistory(ctx context.Context, storeID int64) ([]*domain.LoyaltyConfigVersion, error) {
	const query = `
		SELECT store_id, version, mechanic, accrual_percent, min_purchase_amount, min_balance_to_redeem, max_redeem_percent, points_exchange_rate, changed_by_staff_user_id, created_at
		FROM loyalty_config_history
		WHERE store_id = $1
		ORDER BY version DESC`

	rows, err := r.pool.Query(ctx, query, storeID)
	if err != nil {
		return nil, fmt.Errorf("list loyalty config history for store %d: %w", storeID, err)
	}
	defer rows.Close()

	var out []*domain.LoyaltyConfigVersion
	for rows.Next() {
		v := &domain.LoyaltyConfigVersion{}
		if err := rows.Scan(
			&v.StoreID, &v.Version, &v.Mechanic, &v.AccrualPercent, &v.MinPurchaseAmount,
			&v.MinBalanceToRedeem, &v.MaxRedeemPercent, &v.PointsExchangeRate, &v.ChangedByStaffUserID, &v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan loyalty config history for store %d: %w", storeID, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list loyalty config history for store %d: %w", storeID, err)
	}

	return out, nil
}
