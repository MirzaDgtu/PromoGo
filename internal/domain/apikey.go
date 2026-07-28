package domain

import (
	"context"
	"time"
)

// API key scopes: what a store_api_keys row is allowed to call. Checked
// per-route by httpserver.requireScope, layered on top of
// RequireStoreAPIKey's store resolution.
const (
	ScopeTransactionsWrite = "transactions.write"
	ScopeClientsLookup     = "clients.lookup"
	ScopeBalancesRead      = "balances.read"
)

// StoreAPIKey is one rotatable webhook credential for a Store. Only KeyHash
// (SHA-256 of the secret) is persisted; the plaintext key is returned once,
// at creation, and never again. KeyID is a non-secret, loggable prefix used
// to identify a key in admin UI/audit trails without ever handling the
// secret itself.
type StoreAPIKey struct {
	ID         int64
	StoreID    int64
	KeyID      string
	KeyHash    string
	Name       string
	Scopes     []string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	CreatedBy  *int64
}

// HasScope reports whether the key grants scope.
func (k *StoreAPIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// Active reports whether the key can currently be used to authenticate: not
// revoked, and not past ExpiresAt (if set).
func (k *StoreAPIKey) Active(now time.Time) bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && now.After(*k.ExpiresAt) {
		return false
	}
	return true
}

// StoreAPIKeyRepository persists and retrieves StoreAPIKey rows.
type StoreAPIKeyRepository interface {
	// GetByHash returns domain.ErrNotFound if no key hashes to keyHash,
	// active or not — callers must check Active themselves so a revoked/
	// expired key produces the same 401 as an unrecognized one.
	GetByHash(ctx context.Context, keyHash string) (*StoreAPIKey, error)
	ListByStore(ctx context.Context, storeID int64) ([]*StoreAPIKey, error)
	Create(ctx context.Context, key *StoreAPIKey) error
	Revoke(ctx context.Context, id int64) error
	// TouchLastUsed best-effort updates LastUsedAt; callers must not fail a
	// request if this errors.
	TouchLastUsed(ctx context.Context, id int64, at time.Time) error
}
