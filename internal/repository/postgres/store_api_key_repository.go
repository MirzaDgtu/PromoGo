package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// StoreAPIKeyRepository is a pgx-backed implementation of
// domain.StoreAPIKeyRepository.
type StoreAPIKeyRepository struct {
	pool *pgxpool.Pool
}

// NewStoreAPIKeyRepository creates a StoreAPIKeyRepository backed by pool.
func NewStoreAPIKeyRepository(pool *pgxpool.Pool) *StoreAPIKeyRepository {
	return &StoreAPIKeyRepository{pool: pool}
}

const storeAPIKeyColumns = `id, store_id, key_id, key_hash, name, scopes, created_at, expires_at, revoked_at, last_used_at, created_by`

func scanStoreAPIKey(row pgx.Row) (*domain.StoreAPIKey, error) {
	k := &domain.StoreAPIKey{}
	err := row.Scan(&k.ID, &k.StoreID, &k.KeyID, &k.KeyHash, &k.Name, &k.Scopes, &k.CreatedAt, &k.ExpiresAt, &k.RevokedAt, &k.LastUsedAt, &k.CreatedBy)
	return k, err
}

func (r *StoreAPIKeyRepository) GetByHash(ctx context.Context, keyHash string) (*domain.StoreAPIKey, error) {
	query := `SELECT ` + storeAPIKeyColumns + ` FROM store_api_keys WHERE key_hash = $1`

	k, err := scanStoreAPIKey(r.pool.QueryRow(ctx, query, keyHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store api key: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get store api key: %w", err)
	}

	return k, nil
}

func (r *StoreAPIKeyRepository) ListByStore(ctx context.Context, storeID int64) ([]*domain.StoreAPIKey, error) {
	query := `SELECT ` + storeAPIKeyColumns + ` FROM store_api_keys WHERE store_id = $1 ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, storeID)
	if err != nil {
		return nil, fmt.Errorf("list store api keys for store %d: %w", storeID, err)
	}
	defer rows.Close()

	var keys []*domain.StoreAPIKey
	for rows.Next() {
		k, err := scanStoreAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("scan store api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list store api keys for store %d: %w", storeID, err)
	}

	return keys, nil
}

// Create returns domain.ErrConflict if KeyID or KeyHash already exists
// (should never happen in practice given they're generated from
// crypto/rand, but guarded regardless).
func (r *StoreAPIKeyRepository) Create(ctx context.Context, key *domain.StoreAPIKey) error {
	const query = `
		INSERT INTO store_api_keys (store_id, key_id, key_hash, name, scopes, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`

	err := r.pool.QueryRow(ctx, query, key.StoreID, key.KeyID, key.KeyHash, key.Name, key.Scopes, key.ExpiresAt, key.CreatedBy).
		Scan(&key.ID, &key.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("create store api key: %w", domain.ErrConflict)
		}
		return fmt.Errorf("create store api key: %w", err)
	}

	return nil
}

func (r *StoreAPIKeyRepository) Revoke(ctx context.Context, id int64) error {
	const query = `UPDATE store_api_keys SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`

	if _, err := r.pool.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("revoke store api key %d: %w", id, err)
	}

	return nil
}

func (r *StoreAPIKeyRepository) TouchLastUsed(ctx context.Context, id int64, at time.Time) error {
	const query = `UPDATE store_api_keys SET last_used_at = $2 WHERE id = $1`

	if _, err := r.pool.Exec(ctx, query, id, at); err != nil {
		return fmt.Errorf("touch store api key %d last used: %w", id, err)
	}

	return nil
}
