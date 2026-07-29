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

func (r *OrganizationRepository) GetByName(ctx context.Context, name string) (*domain.Organization, error) {
	const query = `SELECT id, name, created_at FROM organizations WHERE name = $1 ORDER BY id`

	rows, err := r.pool.Query(ctx, query, name)
	if err != nil {
		return nil, fmt.Errorf("get organization %q: %w", name, err)
	}
	defer rows.Close()

	var orgs []*domain.Organization
	for rows.Next() {
		org := &domain.Organization{}
		if err := rows.Scan(&org.ID, &org.Name, &org.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan organization %q: %w", name, err)
		}
		orgs = append(orgs, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get organization %q: %w", name, err)
	}

	switch len(orgs) {
	case 0:
		return nil, fmt.Errorf("organization %q: %w", name, domain.ErrNotFound)
	case 1:
		return orgs[0], nil
	default:
		return nil, fmt.Errorf("organization %q: %d organizations share this name, disambiguate with --org-id", name, len(orgs))
	}
}

func (r *OrganizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	const query = `INSERT INTO organizations (name) VALUES ($1) RETURNING id, created_at`

	if err := r.pool.QueryRow(ctx, query, org.Name).Scan(&org.ID, &org.CreatedAt); err != nil {
		return fmt.Errorf("create organization %q: %w", org.Name, err)
	}

	return nil
}
