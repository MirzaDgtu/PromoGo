package domain

import "context"

// Store is a single retail point integrated via 1C, belonging to an
// Organization (tenant). APIKeyHash is the SHA-256 hash of the legacy
// single webhook API key issued to that store — the plaintext key is shown
// once at creation and never persisted. New stores should prefer
// StoreAPIKeyRepository (multiple rotatable keys); APIKeyHash is kept only
// so deployments that predate that table keep authenticating unchanged.
type Store struct {
	ID             int64
	OrganizationID int64
	Name           string
	// Address is a free-form display string; "" means not set.
	Address string
	// ExternalCode is the store's identifier in the retailer's 1C, unique
	// within OrganizationID; nil means not set.
	ExternalCode *string
	APIKeyHash   string
}

// StoreRepository persists and retrieves Store rows.
type StoreRepository interface {
	// GetByID returns domain.ErrNotFound if no store has that id.
	GetByID(ctx context.Context, id int64) (*Store, error)
	// GetByAPIKeyHash returns domain.ErrNotFound if no store's API key
	// hashes to apiKeyHash. Used by the webhook auth middleware.
	GetByAPIKeyHash(ctx context.Context, apiKeyHash string) (*Store, error)
	// Create returns domain.ErrConflict if store.ExternalCode is already
	// used by another store in the same organization.
	Create(ctx context.Context, store *Store) error
	// UpdateProfile saves store's Name, Address and ExternalCode (nothing
	// else). Returns domain.ErrNotFound if the store doesn't exist and
	// domain.ErrConflict on a duplicate ExternalCode within the organization.
	UpdateProfile(ctx context.Context, store *Store) error
	// ListByOrganization returns every Store belonging to organizationID,
	// ordered by id. Used by GET /api/v1/admin/organizations/{orgID}/stores;
	// callers must filter the result to a caller's store-scoped memberships
	// themselves when the caller doesn't hold an organization-wide
	// membership (this method itself does no permission filtering).
	ListByOrganization(ctx context.Context, organizationID int64) ([]*Store, error)
}
