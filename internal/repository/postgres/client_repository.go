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

// ClientRepository is a pgx-backed implementation of domain.ClientRepository.
type ClientRepository struct {
	pool *pgxpool.Pool
}

// NewClientRepository creates a ClientRepository backed by pool.
func NewClientRepository(pool *pgxpool.Pool) *ClientRepository {
	return &ClientRepository{pool: pool}
}

func (r *ClientRepository) GetByPhone(ctx context.Context, storeID int64, phone string) (*domain.Client, error) {
	const query = `SELECT id, store_id, phone, created_at FROM clients WHERE store_id = $1 AND phone = $2`

	client := &domain.Client{}
	err := r.pool.QueryRow(ctx, query, storeID, phone).Scan(&client.ID, &client.StoreID, &client.Phone, &client.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("client %d/%s: %w", storeID, phone, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get client %d/%s: %w", storeID, phone, err)
	}

	return client, nil
}

func (r *ClientRepository) GetByID(ctx context.Context, id int64) (*domain.Client, error) {
	const query = `SELECT id, store_id, phone, created_at FROM clients WHERE id = $1`

	client := &domain.Client{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&client.ID, &client.StoreID, &client.Phone, &client.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("client %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get client %d: %w", id, err)
	}

	return client, nil
}

// Create returns domain.ErrConflict if storeID already has a client with
// this phone number (unique index clients_store_id_phone_key).
func (r *ClientRepository) Create(ctx context.Context, client *domain.Client) error {
	const query = `INSERT INTO clients (store_id, phone, created_at) VALUES ($1, $2, now()) RETURNING id, created_at`

	err := r.pool.QueryRow(ctx, query, client.StoreID, client.Phone).Scan(&client.ID, &client.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("create client %d/%s: %w", client.StoreID, client.Phone, domain.ErrConflict)
		}
		return fmt.Errorf("create client %d/%s: %w", client.StoreID, client.Phone, err)
	}

	return nil
}
