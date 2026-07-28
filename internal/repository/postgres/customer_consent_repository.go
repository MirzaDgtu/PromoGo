package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// CustomerConsentRepository is a pgx-backed implementation of
// domain.CustomerConsentRepository.
type CustomerConsentRepository struct {
	pool *pgxpool.Pool
}

// NewCustomerConsentRepository creates a CustomerConsentRepository backed by pool.
func NewCustomerConsentRepository(pool *pgxpool.Pool) *CustomerConsentRepository {
	return &CustomerConsentRepository{pool: pool}
}

func (r *CustomerConsentRepository) Create(ctx context.Context, consent *domain.CustomerConsent) error {
	const query = `
		INSERT INTO customer_consents (customer_account_id, document_version, source, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, granted_at`

	err := r.pool.QueryRow(ctx, query, consent.CustomerAccountID, consent.DocumentVersion, consent.Source, consent.IP, consent.UserAgent).
		Scan(&consent.ID, &consent.GrantedAt)
	if err != nil {
		return fmt.Errorf("create customer consent for account %d: %w", consent.CustomerAccountID, err)
	}

	return nil
}
