package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// BalanceRepository is a pgx-backed implementation of domain.BalanceRepository.
type BalanceRepository struct {
	pool *pgxpool.Pool
}

// NewBalanceRepository creates a BalanceRepository backed by pool.
func NewBalanceRepository(pool *pgxpool.Pool) *BalanceRepository {
	return &BalanceRepository{pool: pool}
}

// Get returns a zero Balance (not domain.ErrNotFound) if clientID has no
// row yet — every client implicitly starts at zero points.
func (r *BalanceRepository) Get(ctx context.Context, clientID int64) (*domain.Balance, error) {
	const query = `SELECT points FROM balances WHERE client_id = $1`

	balance := &domain.Balance{ClientID: clientID}
	err := r.pool.QueryRow(ctx, query, clientID).Scan(&balance.Points)
	if errors.Is(err, pgx.ErrNoRows) {
		return balance, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get balance %d: %w", clientID, err)
	}

	return balance, nil
}
