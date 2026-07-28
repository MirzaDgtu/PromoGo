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

// StaffUserRepository is a pgx-backed implementation of
// domain.StaffUserRepository.
type StaffUserRepository struct {
	pool *pgxpool.Pool
}

// NewStaffUserRepository creates a StaffUserRepository backed by pool.
func NewStaffUserRepository(pool *pgxpool.Pool) *StaffUserRepository {
	return &StaffUserRepository{pool: pool}
}

const staffUserColumns = `id, external_subject, email, display_name, status, created_at`

func scanStaffUser(row pgx.Row) (*domain.StaffUser, error) {
	u := &domain.StaffUser{}
	err := row.Scan(&u.ID, &u.ExternalSubject, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt)
	return u, err
}

func (r *StaffUserRepository) GetByExternalSubject(ctx context.Context, subject string) (*domain.StaffUser, error) {
	query := `SELECT ` + staffUserColumns + ` FROM staff_users WHERE external_subject = $1`

	u, err := scanStaffUser(r.pool.QueryRow(ctx, query, subject))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("staff user %s: %w", subject, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get staff user %s: %w", subject, err)
	}

	return u, nil
}

func (r *StaffUserRepository) GetByID(ctx context.Context, id int64) (*domain.StaffUser, error) {
	query := `SELECT ` + staffUserColumns + ` FROM staff_users WHERE id = $1`

	u, err := scanStaffUser(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("staff user %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get staff user %d: %w", id, err)
	}

	return u, nil
}

// Create returns domain.ErrConflict if ExternalSubject is already registered.
func (r *StaffUserRepository) Create(ctx context.Context, user *domain.StaffUser) error {
	const query = `
		INSERT INTO staff_users (external_subject, email, display_name, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	err := r.pool.QueryRow(ctx, query, user.ExternalSubject, user.Email, user.DisplayName, user.Status).
		Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("create staff user %s: %w", user.ExternalSubject, domain.ErrConflict)
		}
		return fmt.Errorf("create staff user %s: %w", user.ExternalSubject, err)
	}

	return nil
}

func (r *StaffUserRepository) UpdateProfile(ctx context.Context, id int64, email, displayName string) error {
	const query = `UPDATE staff_users SET email = $2, display_name = $3 WHERE id = $1`

	if _, err := r.pool.Exec(ctx, query, id, email, displayName); err != nil {
		return fmt.Errorf("update staff user %d profile: %w", id, err)
	}

	return nil
}
