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

const clientColumns = `id, store_id, phone, customer_account_id, created_at`

func scanClient(row pgx.Row) (*domain.Client, error) {
	client := &domain.Client{}
	err := row.Scan(&client.ID, &client.StoreID, &client.Phone, &client.CustomerAccountID, &client.CreatedAt)
	return client, err
}

func (r *ClientRepository) GetByPhone(ctx context.Context, storeID int64, phone string) (*domain.Client, error) {
	query := `SELECT ` + clientColumns + ` FROM clients WHERE store_id = $1 AND phone = $2`

	client, err := scanClient(r.pool.QueryRow(ctx, query, storeID, phone))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("client %d/%s: %w", storeID, phone, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get client %d/%s: %w", storeID, phone, err)
	}

	return client, nil
}

func (r *ClientRepository) GetByID(ctx context.Context, id int64) (*domain.Client, error) {
	query := `SELECT ` + clientColumns + ` FROM clients WHERE id = $1`

	client, err := scanClient(r.pool.QueryRow(ctx, query, id))
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

func (r *ClientRepository) ListUnlinkedByPhone(ctx context.Context, phone string) ([]*domain.Client, error) {
	query := `SELECT ` + clientColumns + ` FROM clients WHERE phone = $1 AND customer_account_id IS NULL`

	rows, err := r.pool.Query(ctx, query, phone)
	if err != nil {
		return nil, fmt.Errorf("list unlinked clients %s: %w", phone, err)
	}
	defer rows.Close()

	var clients []*domain.Client
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, fmt.Errorf("scan unlinked client %s: %w", phone, err)
		}
		clients = append(clients, client)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list unlinked clients %s: %w", phone, err)
	}

	return clients, nil
}

func (r *ClientRepository) LinkCustomerAccount(ctx context.Context, clientID, customerAccountID int64) error {
	const query = `UPDATE clients SET customer_account_id = $2 WHERE id = $1`

	if _, err := r.pool.Exec(ctx, query, clientID, customerAccountID); err != nil {
		return fmt.Errorf("link client %d to customer account %d: %w", clientID, customerAccountID, err)
	}

	return nil
}

func (r *ClientRepository) ListByCustomerAccount(ctx context.Context, customerAccountID int64) ([]*domain.Client, error) {
	query := `SELECT ` + clientColumns + ` FROM clients WHERE customer_account_id = $1`

	rows, err := r.pool.Query(ctx, query, customerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list clients for customer account %d: %w", customerAccountID, err)
	}
	defer rows.Close()

	var clients []*domain.Client
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, fmt.Errorf("scan client for customer account %d: %w", customerAccountID, err)
		}
		clients = append(clients, client)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list clients for customer account %d: %w", customerAccountID, err)
	}

	return clients, nil
}
