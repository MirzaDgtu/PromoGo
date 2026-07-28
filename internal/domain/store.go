package domain

import "context"

// Store is a single retail point integrated via 1C. APIKeyHash is the
// SHA-256 hash of the webhook API key issued to that store — the plaintext
// key is shown once at creation and never persisted.
type Store struct {
	ID         int64
	Name       string
	APIKeyHash string
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
