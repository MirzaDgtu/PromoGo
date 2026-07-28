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

func (r *TransactionRepository) GetByExternalID(ctx context.Context, storeID int64, txType domain.TransactionType, externalTxID string) (*domain.Transaction, error) {
	const query = `
		SELECT id, store_id, client_id, external_tx_id, amount, type, points_delta, balance_after, request_fingerprint, created_at
		FROM transactions
		WHERE store_id = $1 AND type = $2 AND external_tx_id = $3`

	tx := &domain.Transaction{}
	err := r.pool.QueryRow(ctx, query, storeID, txType, externalTxID).Scan(
		&tx.ID, &tx.StoreID, &tx.ClientID, &tx.ExternalTxID, &tx.Amount, &tx.Type, &tx.PointsDelta, &tx.BalanceAfter, &tx.RequestFingerprint, &tx.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("transaction %d/%s/%s: %w", storeID, txType, externalTxID, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction %d/%s/%s: %w", storeID, txType, externalTxID, err)
	}

	return tx, nil
}

func (r *TransactionRepository) ListByClient(ctx context.Context, clientID int64) ([]*domain.Transaction, error) {
	const query = `
		SELECT id, store_id, client_id, external_tx_id, amount, type, points_delta, balance_after, request_fingerprint, created_at
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
		if err := rows.Scan(&tx.ID, &tx.StoreID, &tx.ClientID, &tx.ExternalTxID, &tx.Amount, &tx.Type, &tx.PointsDelta, &tx.BalanceAfter, &tx.RequestFingerprint, &tx.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, rows.Err()
}
