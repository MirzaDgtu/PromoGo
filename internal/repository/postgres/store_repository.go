package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// StoreRepository is a pgx-backed implementation of domain.StoreRepository.
type StoreRepository struct {
	pool *pgxpool.Pool
}

// NewStoreRepository creates a StoreRepository backed by pool.
func NewStoreRepository(pool *pgxpool.Pool) *StoreRepository {
	return &StoreRepository{pool: pool}
}

// GetByID returns domain.ErrNotFound if no store has that id.
func (r *StoreRepository) GetByID(ctx context.Context, id int64) (*domain.Store, error) {
	const query = `SELECT id, organization_id, name, COALESCE(api_key_hash, '') FROM stores WHERE id = $1`

	store := &domain.Store{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&store.ID, &store.OrganizationID, &store.Name, &store.APIKeyHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get store %d: %w", id, err)
	}

	return store, nil
}

func (r *StoreRepository) GetByAPIKeyHash(ctx context.Context, apiKeyHash string) (*domain.Store, error) {
	const query = `SELECT id, organization_id, name, COALESCE(api_key_hash, '') FROM stores WHERE api_key_hash = $1`

	store := &domain.Store{}
	err := r.pool.QueryRow(ctx, query, apiKeyHash).Scan(&store.ID, &store.OrganizationID, &store.Name, &store.APIKeyHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store by api key: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get store by api key: %w", err)
	}

	return store, nil
}

// Create inserts store. An empty store.APIKeyHash is stored as NULL — new
// stores created via the admin API rely solely on StoreAPIKeyRepository
// (migrations/00019, migrations/00021) and never populate the legacy
// single-key column.
func (r *StoreRepository) Create(ctx context.Context, store *domain.Store) error {
	const query = `INSERT INTO stores (organization_id, name, api_key_hash) VALUES ($1, $2, $3) RETURNING id`

	var apiKeyHash any
	if store.APIKeyHash != "" {
		apiKeyHash = store.APIKeyHash
	}

	if err := r.pool.QueryRow(ctx, query, store.OrganizationID, store.Name, apiKeyHash).Scan(&store.ID); err != nil {
		return fmt.Errorf("create store %q: %w", store.Name, err)
	}

	return nil
}
