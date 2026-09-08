package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// TransactionRepository is a pgx-backed implementation of
// domain.TransactionRepository.
type TransactionRepository struct {
	pool *pgxpool.Pool
}

// NewTransactionRepository creates a TransactionRepository backed by pool.
func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

const transactionColumns = `id, store_id, client_id, external_tx_id, amount, type, points_delta, balance_after, request_fingerprint, created_at, original_transaction_id, refunded_amount, refunded_points`

func scanTransaction(tx *domain.Transaction, row pgx.Row) error {
	return row.Scan(
		&tx.ID, &tx.StoreID, &tx.ClientID, &tx.ExternalTxID, &tx.Amount, &tx.Type, &tx.PointsDelta, &tx.BalanceAfter, &tx.RequestFingerprint, &tx.CreatedAt,
		&tx.OriginalTransactionID, &tx.RefundedAmount, &tx.RefundedPoints,
	)
}

func (r *TransactionRepository) GetByExternalID(ctx context.Context, storeID int64, txType domain.TransactionType, externalTxID string) (*domain.Transaction, error) {
	query := `
		SELECT ` + transactionColumns + `
		FROM transactions
		WHERE store_id = $1 AND type = $2 AND external_tx_id = $3`

	tx := &domain.Transaction{}
	err := scanTransaction(tx, r.pool.QueryRow(ctx, query, storeID, txType, externalTxID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("transaction %d/%s/%s: %w", storeID, txType, externalTxID, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction %d/%s/%s: %w", storeID, txType, externalTxID, err)
	}

	return tx, nil
}

func (r *TransactionRepository) GetByID(ctx context.Context, id int64) (*domain.Transaction, error) {
	query := `SELECT ` + transactionColumns + ` FROM transactions WHERE id = $1`

	tx := &domain.Transaction{}
	err := scanTransaction(tx, r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("transaction %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction %d: %w", id, err)
	}

	return tx, nil
}

func (r *TransactionRepository) ListByClient(ctx context.Context, clientID int64) ([]*domain.Transaction, error) {
	query := `
		SELECT ` + transactionColumns + `
		FROM transactions
		WHERE client_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, clientID)
	if err != nil {
		return nil, fmt.Errorf("list transactions for client %d: %w", clientID, err)
	}
	defer rows.Close()

	var txs []*domain.Transaction
	for rows.Next() {
		tx := &domain.Transaction{}
		if err := scanTransaction(tx, rows); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, rows.Err()
}

func (r *TransactionRepository) ListByClientIDs(ctx context.Context, clientIDs []int64, limit int, before *domain.TransactionCursor) ([]*domain.Transaction, error) {
	var rows pgx.Rows
	var err error

	if before != nil {
		query := `SELECT ` + transactionColumns + `
			FROM transactions
			WHERE client_id = ANY($1) AND (created_at, id) < ($2, $3)
			ORDER BY created_at DESC, id DESC
			LIMIT $4`
		rows, err = r.pool.Query(ctx, query, clientIDs, before.CreatedAt, before.ID, limit)
	} else {
		query := `SELECT ` + transactionColumns + `
			FROM transactions
			WHERE client_id = ANY($1)
			ORDER BY created_at DESC, id DESC
			LIMIT $2`
		rows, err = r.pool.Query(ctx, query, clientIDs, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list transactions for clients: %w", err)
	}
	defer rows.Close()

	var txs []*domain.Transaction
	for rows.Next() {
		tx := &domain.Transaction{}
		if err := scanTransaction(tx, rows); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, rows.Err()
}
