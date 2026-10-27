package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// StoreActivityRepository is a pgx-backed implementation of
// domain.StoreActivityRepository.
type StoreActivityRepository struct {
	pool *pgxpool.Pool
}

// NewStoreActivityRepository creates a StoreActivityRepository backed by pool.
func NewStoreActivityRepository(pool *pgxpool.Pool) *StoreActivityRepository {
	return &StoreActivityRepository{pool: pool}
}

// StoreActivity reads both windows in one pass over the last 7 days of the
// store's transactions (idx_transactions_store_created); the last-operation
// time and the client count are separate index lookups.
func (r *StoreActivityRepository) StoreActivity(ctx context.Context, storeID int64, now time.Time) (*domain.StoreActivity, error) {
	const query = `
		SELECT
			(SELECT count(*) FROM clients WHERE store_id = $1),
			(SELECT max(created_at) FROM transactions WHERE store_id = $1),
			count(*) FILTER (WHERE created_at >= $2),
			coalesce(sum(points_delta) FILTER (WHERE created_at >= $2 AND type = 'accrual'), 0)::bigint,
			coalesce(-sum(points_delta) FILTER (WHERE created_at >= $2 AND type = 'redeem'), 0)::bigint,
			coalesce(sum(amount) FILTER (WHERE created_at >= $2 AND type = 'accrual'), 0),
			count(*),
			coalesce(sum(points_delta) FILTER (WHERE type = 'accrual'), 0)::bigint,
			coalesce(-sum(points_delta) FILTER (WHERE type = 'redeem'), 0)::bigint,
			coalesce(sum(amount) FILTER (WHERE type = 'accrual'), 0)
		FROM transactions
		WHERE store_id = $1 AND created_at >= $3`

	a := &domain.StoreActivity{}
	err := r.pool.QueryRow(ctx, query, storeID, now.Add(-24*time.Hour), now.Add(-7*24*time.Hour)).Scan(
		&a.Clients, &a.LastTransactionAt,
		&a.Last24h.Operations, &a.Last24h.PointsAccrued, &a.Last24h.PointsRedeemed, &a.Last24h.SalesAmount,
		&a.Last7d.Operations, &a.Last7d.PointsAccrued, &a.Last7d.PointsRedeemed, &a.Last7d.SalesAmount,
	)
	if err != nil {
		return nil, fmt.Errorf("store activity for store %d: %w", storeID, err)
	}
	return a, nil
}
