package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// OrganizationRepository is a pgx-backed implementation of
// domain.OrganizationRepository.
type OrganizationRepository struct {
	pool *pgxpool.Pool
}

// NewOrganizationRepository creates an OrganizationRepository backed by pool.
func NewOrganizationRepository(pool *pgxpool.Pool) *OrganizationRepository {
	return &OrganizationRepository{pool: pool}
}

func (r *OrganizationRepository) GetByID(ctx context.Context, id int64) (*domain.Organization, error) {
	const query = `SELECT id, name, created_at FROM organizations WHERE id = $1`

	org := &domain.Organization{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&org.ID, &org.Name, &org.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("organization %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get organization %d: %w", id, err)
	}

	return org, nil
}

func (r *OrganizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	const query = `INSERT INTO organizations (name) VALUES ($1) RETURNING id, created_at`

	if err := r.pool.QueryRow(ctx, query, org.Name).Scan(&org.ID, &org.CreatedAt); err != nil {
		return fmt.Errorf("create organization %q: %w", org.Name, err)
	}

	return nil
}
