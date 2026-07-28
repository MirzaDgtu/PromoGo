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

// StaffMembershipRepository is a pgx-backed implementation of
// domain.StaffMembershipRepository.
type StaffMembershipRepository struct {
	pool *pgxpool.Pool
}

// NewStaffMembershipRepository creates a StaffMembershipRepository backed by pool.
func NewStaffMembershipRepository(pool *pgxpool.Pool) *StaffMembershipRepository {
	return &StaffMembershipRepository{pool: pool}
}

const staffMembershipColumns = `id, staff_user_id, organization_id, store_id, role, status, created_at, updated_at`

func scanStaffMembership(row pgx.Row) (*domain.StaffMembership, error) {
	m := &domain.StaffMembership{}
	err := row.Scan(&m.ID, &m.StaffUserID, &m.OrganizationID, &m.StoreID, &m.Role, &m.Status, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

func (r *StaffMembershipRepository) ListByStaffUser(ctx context.Context, staffUserID int64) ([]*domain.StaffMembership, error) {
	query := `SELECT ` + staffMembershipColumns + ` FROM staff_memberships WHERE staff_user_id = $1`
	return queryStaffMemberships(ctx, r.pool, query, staffUserID)
}

func (r *StaffMembershipRepository) ListByOrganization(ctx context.Context, organizationID int64) ([]*domain.StaffMembership, error) {
	query := `SELECT ` + staffMembershipColumns + ` FROM staff_memberships WHERE organization_id = $1`
	return queryStaffMemberships(ctx, r.pool, query, organizationID)
}

func queryStaffMemberships(ctx context.Context, pool *pgxpool.Pool, query string, arg int64) ([]*domain.StaffMembership, error) {
	rows, err := pool.Query(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("list staff memberships: %w", err)
	}
	defer rows.Close()

	var memberships []*domain.StaffMembership
	for rows.Next() {
		m, err := scanStaffMembership(rows)
		if err != nil {
			return nil, fmt.Errorf("scan staff membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list staff memberships: %w", err)
	}

	return memberships, nil
}

// Create returns domain.ErrConflict if this exact
// (staff_user_id, organization_id, store_id) membership already exists.
func (r *StaffMembershipRepository) Create(ctx context.Context, membership *domain.StaffMembership) error {
	const query = `
		INSERT INTO staff_memberships (staff_user_id, organization_id, store_id, role, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`

	err := r.pool.QueryRow(ctx, query, membership.StaffUserID, membership.OrganizationID, membership.StoreID, membership.Role, membership.Status).
		Scan(&membership.ID, &membership.CreatedAt, &membership.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("create staff membership: %w", domain.ErrConflict)
		}
		return fmt.Errorf("create staff membership: %w", err)
	}

	return nil
}

func (r *StaffMembershipRepository) UpdateStatus(ctx context.Context, id int64, status domain.StaffStatus) error {
	const query = `UPDATE staff_memberships SET status = $2, updated_at = now() WHERE id = $1`

	if _, err := r.pool.Exec(ctx, query, id, status); err != nil {
		return fmt.Errorf("update staff membership %d status: %w", id, err)
	}

	return nil
}

func (r *StaffMembershipRepository) UpdateRole(ctx context.Context, id int64, role domain.Role) error {
	const query = `UPDATE staff_memberships SET role = $2, updated_at = now() WHERE id = $1`

	if _, err := r.pool.Exec(ctx, query, id, role); err != nil {
		return fmt.Errorf("update staff membership %d role: %w", id, err)
	}

	return nil
}
