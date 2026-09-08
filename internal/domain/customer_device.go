package domain

import (
	"context"
	"time"
)

// CustomerDevice is one FCM push-token registration for a CustomerAccount
// (Q-P0-103/DEC-014). A customer may have several active devices at once;
// RevokedAt non-nil means push delivery no longer targets this row —
// self-healing (see internal/notification/fcmchannel) revokes a device
// whose token FCM reports as unregistered/invalid, without any manual
// cleanup step.
type CustomerDevice struct {
	ID                int64
	CustomerAccountID int64
	Platform          string
	PushToken         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	RevokedAt         *time.Time
}

// CustomerDeviceRepository persists and retrieves CustomerDevice rows.
type CustomerDeviceRepository interface {
	// Upsert registers device.PushToken for device.CustomerAccountID, or
	// refreshes it (platform, updated_at, and re-associating it to this
	// account) if that token is already active under any account — a
	// reinstalled app re-registering the same OS-issued token must not
	// collide with a stale row from a previous owner. Sets device.ID.
	Upsert(ctx context.Context, device *CustomerDevice) error
	// ListActiveByCustomerAccount returns every non-revoked device for
	// customerAccountID.
	ListActiveByCustomerAccount(ctx context.Context, customerAccountID int64) ([]*CustomerDevice, error)
	// Revoke marks deviceID revoked, scoped to customerAccountID — returns
	// domain.ErrNotFound if deviceID doesn't exist or belongs to a
	// different account, so a customer can never revoke someone else's
	// device (and can't distinguish "not found" from "not yours").
	Revoke(ctx context.Context, deviceID, customerAccountID int64) error
	// RevokeByToken marks the active device row for pushToken revoked, if
	// any — used by internal/notification/fcmchannel to self-heal a token
	// FCM reports as unregistered/invalid. A no-op (nil error) if no active
	// row has that token.
	RevokeByToken(ctx context.Context, pushToken string) error
}
