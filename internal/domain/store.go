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
	APIKeyHash     string
}

// StoreRepository persists and retrieves Store rows.
type StoreRepository interface {
	// GetByID returns domain.ErrNotFound if no store has that id.
	GetByID(ctx context.Context, id int64) (*Store, error)
	// GetByAPIKeyHash returns domain.ErrNotFound if no store's API key
	// hashes to apiKeyHash. Used by the webhook auth middleware.
	GetByAPIKeyHash(ctx context.Context, apiKeyHash string) (*Store, error)
	Create(ctx context.Context, store *Store) error
}
