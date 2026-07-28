package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// CustomerSessionRepository is a pgx-backed implementation of
// domain.CustomerSessionRepository.
type CustomerSessionRepository struct {
	pool *pgxpool.Pool
}

// NewCustomerSessionRepository creates a CustomerSessionRepository backed by pool.
func NewCustomerSessionRepository(pool *pgxpool.Pool) *CustomerSessionRepository {
	return &CustomerSessionRepository{pool: pool}
}

const customerSessionColumns = `id, customer_account_id, refresh_token_hash, replaced_by_id, issued_at, expires_at, revoked_at, user_agent, ip`

func scanCustomerSession(row pgx.Row) (*domain.CustomerSession, error) {
	s := &domain.CustomerSession{}
	err := row.Scan(&s.ID, &s.CustomerAccountID, &s.RefreshTokenHash, &s.ReplacedByID, &s.IssuedAt, &s.ExpiresAt, &s.RevokedAt, &s.UserAgent, &s.IP)
	return s, err
}

func (r *CustomerSessionRepository) Create(ctx context.Context, session *domain.CustomerSession) error {
	const query = `
		INSERT INTO customer_sessions (customer_account_id, refresh_token_hash, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, issued_at`

	err := r.pool.QueryRow(ctx, query, session.CustomerAccountID, session.RefreshTokenHash, session.ExpiresAt, session.UserAgent, session.IP).
		Scan(&session.ID, &session.IssuedAt)
	if err != nil {
		return fmt.Errorf("create customer session for account %d: %w", session.CustomerAccountID, err)
	}

	return nil
}

func (r *CustomerSessionRepository) GetByRefreshTokenHash(ctx context.Context, hash string) (*domain.CustomerSession, error) {
	query := `SELECT ` + customerSessionColumns + ` FROM customer_sessions WHERE refresh_token_hash = $1`

	session, err := scanCustomerSession(r.pool.QueryRow(ctx, query, hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("customer session: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get customer session: %w", err)
	}

	return session, nil
}

func (r *CustomerSessionRepository) Rotate(ctx context.Context, sessionID, replacedByID int64) error {
	const query = `UPDATE customer_sessions SET revoked_at = now(), replaced_by_id = $2 WHERE id = $1`

	if _, err := r.pool.Exec(ctx, query, sessionID, replacedByID); err != nil {
		return fmt.Errorf("rotate customer session %d: %w", sessionID, err)
	}

	return nil
}

func (r *CustomerSessionRepository) Revoke(ctx context.Context, sessionID int64) error {
	const query = `UPDATE customer_sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`

	if _, err := r.pool.Exec(ctx, query, sessionID); err != nil {
		return fmt.Errorf("revoke customer session %d: %w", sessionID, err)
	}

	return nil
}

func (r *CustomerSessionRepository) RevokeAllForAccount(ctx context.Context, customerAccountID int64) error {
	const query = `UPDATE customer_sessions SET revoked_at = now() WHERE customer_account_id = $1 AND revoked_at IS NULL`

	if _, err := r.pool.Exec(ctx, query, customerAccountID); err != nil {
		return fmt.Errorf("revoke all customer sessions for account %d: %w", customerAccountID, err)
	}

	return nil
}
