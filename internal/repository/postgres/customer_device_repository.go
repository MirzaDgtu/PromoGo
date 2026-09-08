package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// CustomerDeviceRepository is a pgx-backed implementation of
// domain.CustomerDeviceRepository.
type CustomerDeviceRepository struct {
	pool *pgxpool.Pool
}

// NewCustomerDeviceRepository creates a CustomerDeviceRepository backed by pool.
func NewCustomerDeviceRepository(pool *pgxpool.Pool) *CustomerDeviceRepository {
	return &CustomerDeviceRepository{pool: pool}
}

const customerDeviceColumns = `id, customer_account_id, platform, push_token, created_at, updated_at, revoked_at`

func scanCustomerDevice(row pgx.Row) (*domain.CustomerDevice, error) {
	d := &domain.CustomerDevice{}
	err := row.Scan(&d.ID, &d.CustomerAccountID, &d.Platform, &d.PushToken, &d.CreatedAt, &d.UpdatedAt, &d.RevokedAt)
	return d, err
}

// Upsert relies on idx_customer_devices_active_token (a partial unique
// index on push_token WHERE revoked_at IS NULL) as its conflict target — a
// token already active under any account (including this one) is
// reassigned/refreshed rather than duplicated.
func (r *CustomerDeviceRepository) Upsert(ctx context.Context, device *domain.CustomerDevice) error {
	const query = `
		INSERT INTO customer_devices (customer_account_id, platform, push_token, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (push_token) WHERE revoked_at IS NULL
		DO UPDATE SET customer_account_id = EXCLUDED.customer_account_id, platform = EXCLUDED.platform, updated_at = now()
		RETURNING id, created_at, updated_at`

	err := r.pool.QueryRow(ctx, query, device.CustomerAccountID, device.Platform, device.PushToken).
		Scan(&device.ID, &device.CreatedAt, &device.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert customer device: %w", err)
	}

	return nil
}

func (r *CustomerDeviceRepository) ListActiveByCustomerAccount(ctx context.Context, customerAccountID int64) ([]*domain.CustomerDevice, error) {
	query := `SELECT ` + customerDeviceColumns + ` FROM customer_devices WHERE customer_account_id = $1 AND revoked_at IS NULL ORDER BY created_at`

	rows, err := r.pool.Query(ctx, query, customerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list customer devices for account %d: %w", customerAccountID, err)
	}
	defer rows.Close()

	var devices []*domain.CustomerDevice
	for rows.Next() {
		d, err := scanCustomerDevice(rows)
		if err != nil {
			return nil, fmt.Errorf("scan customer device: %w", err)
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list customer devices for account %d: %w", customerAccountID, err)
	}

	return devices, nil
}

func (r *CustomerDeviceRepository) Revoke(ctx context.Context, deviceID, customerAccountID int64) error {
	const query = `UPDATE customer_devices SET revoked_at = now(), updated_at = now() WHERE id = $1 AND customer_account_id = $2 AND revoked_at IS NULL`

	tag, err := r.pool.Exec(ctx, query, deviceID, customerAccountID)
	if err != nil {
		return fmt.Errorf("revoke customer device %d: %w", deviceID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("customer device %d: %w", deviceID, domain.ErrNotFound)
	}

	return nil
}

func (r *CustomerDeviceRepository) RevokeByToken(ctx context.Context, pushToken string) error {
	const query = `UPDATE customer_devices SET revoked_at = now(), updated_at = now() WHERE push_token = $1 AND revoked_at IS NULL`

	if _, err := r.pool.Exec(ctx, query, pushToken); err != nil {
		return fmt.Errorf("revoke customer device by token: %w", err)
	}

	return nil
}
