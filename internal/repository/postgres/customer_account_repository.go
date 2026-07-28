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

// CustomerAccountRepository is a pgx-backed implementation of
// domain.CustomerAccountRepository.
type CustomerAccountRepository struct {
	pool *pgxpool.Pool
}

// NewCustomerAccountRepository creates a CustomerAccountRepository backed by pool.
func NewCustomerAccountRepository(pool *pgxpool.Pool) *CustomerAccountRepository {
	return &CustomerAccountRepository{pool: pool}
}

const customerAccountColumns = `id, phone, phone_verified_at, status, created_at, updated_at`

func scanCustomerAccount(row pgx.Row) (*domain.CustomerAccount, error) {
	acc := &domain.CustomerAccount{}
	err := row.Scan(&acc.ID, &acc.Phone, &acc.PhoneVerifiedAt, &acc.Status, &acc.CreatedAt, &acc.UpdatedAt)
	return acc, err
}

func (r *CustomerAccountRepository) GetByPhone(ctx context.Context, phone string) (*domain.CustomerAccount, error) {
	query := `SELECT ` + customerAccountColumns + ` FROM customer_accounts WHERE phone = $1`

	acc, err := scanCustomerAccount(r.pool.QueryRow(ctx, query, phone))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("customer account %s: %w", phone, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get customer account %s: %w", phone, err)
	}

	return acc, nil
}

func (r *CustomerAccountRepository) GetByID(ctx context.Context, id int64) (*domain.CustomerAccount, error) {
	query := `SELECT ` + customerAccountColumns + ` FROM customer_accounts WHERE id = $1`

	acc, err := scanCustomerAccount(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("customer account %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get customer account %d: %w", id, err)
	}

	return acc, nil
}

// Create returns domain.ErrConflict if phone is already registered
// (unique index on customer_accounts.phone).
func (r *CustomerAccountRepository) Create(ctx context.Context, acc *domain.CustomerAccount) error {
	const query = `
		INSERT INTO customer_accounts (phone, phone_verified_at, status)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	err := r.pool.QueryRow(ctx, query, acc.Phone, acc.PhoneVerifiedAt, acc.Status).
		Scan(&acc.ID, &acc.CreatedAt, &acc.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("create customer account %s: %w", acc.Phone, domain.ErrConflict)
		}
		return fmt.Errorf("create customer account %s: %w", acc.Phone, err)
	}

	return nil
}
