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

// StoreRepository is a pgx-backed implementation of domain.StoreRepository.
type StoreRepository struct {
	pool *pgxpool.Pool
}

// NewStoreRepository creates a StoreRepository backed by pool.
func NewStoreRepository(pool *pgxpool.Pool) *StoreRepository {
	return &StoreRepository{pool: pool}
}

// storeColumns is the SELECT list scanStore reads, in order.
const storeColumns = `id, organization_id, name, address, external_code, sms_invite_enabled, COALESCE(api_key_hash, '')`

func scanStore(row pgx.Row) (*domain.Store, error) {
	store := &domain.Store{}
	err := row.Scan(&store.ID, &store.OrganizationID, &store.Name, &store.Address, &store.ExternalCode, &store.SMSInviteEnabled, &store.APIKeyHash)
	return store, err
}

// GetByID returns domain.ErrNotFound if no store has that id.
func (r *StoreRepository) GetByID(ctx context.Context, id int64) (*domain.Store, error) {
	const query = `SELECT ` + storeColumns + ` FROM stores WHERE id = $1`

	store, err := scanStore(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get store %d: %w", id, err)
	}

	return store, nil
}

func (r *StoreRepository) GetByAPIKeyHash(ctx context.Context, apiKeyHash string) (*domain.Store, error) {
	const query = `SELECT ` + storeColumns + ` FROM stores WHERE api_key_hash = $1`

	store, err := scanStore(r.pool.QueryRow(ctx, query, apiKeyHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("store by api key: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get store by api key: %w", err)
	}

	return store, nil
}

// ListByOrganization returns every store belonging to organizationID,
// ordered by id.
func (r *StoreRepository) ListByOrganization(ctx context.Context, organizationID int64) ([]*domain.Store, error) {
	const query = `SELECT ` + storeColumns + ` FROM stores WHERE organization_id = $1 ORDER BY id`

	rows, err := r.pool.Query(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list stores for organization %d: %w", organizationID, err)
	}
	defer rows.Close()

	var stores []*domain.Store
	for rows.Next() {
		store, err := scanStore(rows)
		if err != nil {
			return nil, fmt.Errorf("scan store: %w", err)
		}
		stores = append(stores, store)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list stores for organization %d: %w", organizationID, err)
	}
	return stores, nil
}

// Create inserts store. An empty store.APIKeyHash is stored as NULL — new
// stores created via the admin API rely solely on StoreAPIKeyRepository
// (migrations/00019, migrations/00021) and never populate the legacy
// single-key column. Returns domain.ErrConflict on a duplicate
// ExternalCode within the organization.
func (r *StoreRepository) Create(ctx context.Context, store *domain.Store) error {
	const query = `INSERT INTO stores (organization_id, name, address, external_code, sms_invite_enabled, api_key_hash)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	var apiKeyHash any
	if store.APIKeyHash != "" {
		apiKeyHash = store.APIKeyHash
	}

	err := r.pool.QueryRow(ctx, query, store.OrganizationID, store.Name, store.Address, store.ExternalCode, store.SMSInviteEnabled, apiKeyHash).Scan(&store.ID)
	if isUniqueViolation(err) {
		return fmt.Errorf("create store %q: %w", store.Name, domain.ErrConflict)
	}
	if err != nil {
		return fmt.Errorf("create store %q: %w", store.Name, err)
	}

	return nil
}

// UpdateProfile saves Name, Address, ExternalCode and SMSInviteEnabled only.
func (r *StoreRepository) UpdateProfile(ctx context.Context, store *domain.Store) error {
	const query = `UPDATE stores SET name = $2, address = $3, external_code = $4, sms_invite_enabled = $5 WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, store.ID, store.Name, store.Address, store.ExternalCode, store.SMSInviteEnabled)
	if isUniqueViolation(err) {
		return fmt.Errorf("update store %d: %w", store.ID, domain.ErrConflict)
	}
	if err != nil {
		return fmt.Errorf("update store %d: %w", store.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update store %d: %w", store.ID, domain.ErrNotFound)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
