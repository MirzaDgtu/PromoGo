package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// LedgerRepository is a pgx-backed implementation of domain.LedgerRepository.
type LedgerRepository struct {
	pool *pgxpool.Pool
}

// NewLedgerRepository creates a LedgerRepository backed by pool.
func NewLedgerRepository(pool *pgxpool.Pool) *LedgerRepository {
	return &LedgerRepository{pool: pool}
}

// Post implements domain.LedgerRepository: inserting the transaction and
// adjusting the balance happen in one database transaction, so the two can
// never diverge (see domain.LedgerRepository's doc comment).
//
// The balance update deliberately isn't a single
// "INSERT ... ON CONFLICT DO UPDATE SET points = balances.points + EXCLUDED.points"
// statement: Postgres validates the balances_points_check CHECK constraint
// against the raw EXCLUDED values before conflict resolution picks the
// UPDATE branch, so a negative points_delta on an existing row with a
// sufficient balance would still spuriously fail the check against the
// negative delta alone (confirmed against Postgres 16). Ensuring the row
// exists first, then applying the delta with a plain UPDATE, checks the
// constraint against the real post-update value instead.
func (r *LedgerRepository) Post(ctx context.Context, tx *domain.Transaction) (*domain.Transaction, *domain.Balance, error) {
	dbTx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("post transaction %d/%s: begin tx: %w", tx.StoreID, tx.ExternalTxID, err)
	}
	defer dbTx.Rollback(ctx)

	const ensureBalanceRow = `INSERT INTO balances (client_id, points) VALUES ($1, 0) ON CONFLICT (client_id) DO NOTHING`
	if _, err := dbTx.Exec(ctx, ensureBalanceRow, tx.ClientID); err != nil {
		return nil, nil, fmt.Errorf("post transaction %d/%s: ensure balance row: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	// The "AND points + $2 >= 0" guard makes redemption atomic: without it,
	// a concurrent redemption could pass the service layer's balance
	// pre-check and still race here, tripping the balances_points_check
	// CHECK constraint and surfacing as a raw DB error instead of
	// domain.ErrInsufficientBalance. For accrual (non-negative delta) the
	// guard is always true given the points>=0 invariant, so it's a no-op.
	// This runs before the transaction row is inserted so the row can carry
	// its resulting balance_after directly, rather than needing a second
	// UPDATE after computing it.
	const adjustBalance = `UPDATE balances SET points = points + $2 WHERE client_id = $1 AND points + $2 >= 0 RETURNING points`
	balance := &domain.Balance{ClientID: tx.ClientID}
	err = dbTx.QueryRow(ctx, adjustBalance, tx.ClientID, tx.PointsDelta).Scan(&balance.Points)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, domain.ErrInsufficientBalance
	}
	if err != nil {
		return nil, nil, fmt.Errorf("post transaction %d/%s: adjust balance: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	const insert = `
		INSERT INTO transactions (store_id, client_id, external_tx_id, amount, type, points_delta, balance_after, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		RETURNING id, created_at`
	err = dbTx.QueryRow(ctx, insert, tx.StoreID, tx.ClientID, tx.ExternalTxID, tx.Amount, tx.Type, tx.PointsDelta, balance.Points).
		Scan(&tx.ID, &tx.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, nil, fmt.Errorf("post transaction %d/%s: %w", tx.StoreID, tx.ExternalTxID, domain.ErrConflict)
		}
		return nil, nil, fmt.Errorf("post transaction %d/%s: insert: %w", tx.StoreID, tx.ExternalTxID, err)
	}
	tx.BalanceAfter = balance.Points

	if err := dbTx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("post transaction %d/%s: commit: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	return tx, balance, nil
}
