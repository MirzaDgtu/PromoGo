package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeStoreRepo is a minimal in-memory domain.StoreRepository for
// middleware tests.
type fakeStoreRepo struct {
	byID     map[int64]*domain.Store
	byAPIKey map[string]*domain.Store
}

func newFakeStoreRepo() *fakeStoreRepo {
	return &fakeStoreRepo{byID: map[int64]*domain.Store{}, byAPIKey: map[string]*domain.Store{}}
}

func (f *fakeStoreRepo) GetByID(_ context.Context, id int64) (*domain.Store, error) {
	s, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (f *fakeStoreRepo) GetByAPIKeyHash(_ context.Context, hash string) (*domain.Store, error) {
	s, ok := f.byAPIKey[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (f *fakeStoreRepo) Create(_ context.Context, s *domain.Store) error {
	f.byID[s.ID] = s
	if s.APIKeyHash != "" {
		f.byAPIKey[s.APIKeyHash] = s
	}
	return nil
}

func (f *fakeStoreRepo) addLegacy(store *domain.Store, plaintextKey string) {
	store.APIKeyHash = sha256Hex(plaintextKey)
	f.byID[store.ID] = store
	f.byAPIKey[store.APIKeyHash] = store
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// fakeStoreAPIKeyRepo is a minimal in-memory domain.StoreAPIKeyRepository.
type fakeStoreAPIKeyRepo struct {
	byHash map[string]*domain.StoreAPIKey
}

func newFakeStoreAPIKeyRepo() *fakeStoreAPIKeyRepo {
	return &fakeStoreAPIKeyRepo{byHash: map[string]*domain.StoreAPIKey{}}
}

func (f *fakeStoreAPIKeyRepo) GetByHash(_ context.Context, hash string) (*domain.StoreAPIKey, error) {
	k, ok := f.byHash[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return k, nil
}

func (f *fakeStoreAPIKeyRepo) ListByStore(_ context.Context, storeID int64) ([]*domain.StoreAPIKey, error) {
	var out []*domain.StoreAPIKey
	for _, k := range f.byHash {
		if k.StoreID == storeID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeStoreAPIKeyRepo) Create(_ context.Context, k *domain.StoreAPIKey) error {
	f.byHash[k.KeyHash] = k
	return nil
}

func (f *fakeStoreAPIKeyRepo) Revoke(_ context.Context, id int64) error {
	for _, k := range f.byHash {
		if k.ID == id {
			now := time.Now()
			k.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeStoreAPIKeyRepo) TouchLastUsed(_ context.Context, id int64, at time.Time) error {
	for _, k := range f.byHash {
		if k.ID == id {
			k.LastUsedAt = &at
		}
	}
	return nil
}

func (f *fakeStoreAPIKeyRepo) add(k *domain.StoreAPIKey, plaintextSecret string) {
	k.KeyHash = sha256Hex(plaintextSecret)
	f.byHash[k.KeyHash] = k
}

func okHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func TestRequireStoreAPIKey_LegacyKeyStillWorks(t *testing.T) {
	stores := newFakeStoreRepo()
	apiKeys := newFakeStoreAPIKeyRepo()
	stores.addLegacy(&domain.Store{ID: 1, OrganizationID: 1, Name: "Legacy Store"}, "legacy-plaintext-key")

	var sawStore *domain.Store
	handler := RequireStoreAPIKey(stores, apiKeys, testLogger())(func(w http.ResponseWriter, r *http.Request) {
		sawStore, _ = storeFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer legacy-plaintext-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if sawStore == nil || sawStore.ID != 1 {
		t.Fatalf("store in context = %+v, want store 1", sawStore)
	}
}

func TestRequireStoreAPIKey_MultiKeyTakesPrecedenceOverLegacy(t *testing.T) {
	stores := newFakeStoreRepo()
	apiKeys := newFakeStoreAPIKeyRepo()
	store := &domain.Store{ID: 2, OrganizationID: 1, Name: "Store"}
	stores.byID[2] = store
	apiKeys.add(&domain.StoreAPIKey{ID: 10, StoreID: 2, Scopes: []string{domain.ScopeTransactionsWrite}}, "new-plaintext-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer new-plaintext-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRequireStoreAPIKey_RevokedKeyRejected(t *testing.T) {
	stores := newFakeStoreRepo()
	apiKeys := newFakeStoreAPIKeyRepo()
	stores.byID[3] = &domain.Store{ID: 3, OrganizationID: 1, Name: "Store"}
	now := time.Now()
	apiKeys.add(&domain.StoreAPIKey{ID: 11, StoreID: 3, RevokedAt: &now}, "revoked-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer revoked-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireStoreAPIKey_ExpiredKeyRejected(t *testing.T) {
	stores := newFakeStoreRepo()
	apiKeys := newFakeStoreAPIKeyRepo()
	stores.byID[4] = &domain.Store{ID: 4, OrganizationID: 1, Name: "Store"}
	past := time.Now().Add(-time.Hour)
	apiKeys.add(&domain.StoreAPIKey{ID: 12, StoreID: 4, ExpiresAt: &past}, "expired-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer expired-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireStoreAPIKey_MissingKeyRejected(t *testing.T) {
	handler := RequireStoreAPIKey(newFakeStoreRepo(), newFakeStoreAPIKeyRepo(), testLogger())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireStoreAPIKey_UnknownKeyRejected(t *testing.T) {
	handler := RequireStoreAPIKey(newFakeStoreRepo(), newFakeStoreAPIKeyRepo(), testLogger())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer nonexistent-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireScope_LegacyKeyBypassesScopeCheck(t *testing.T) {
	stores := newFakeStoreRepo()
	apiKeys := newFakeStoreAPIKeyRepo()
	stores.addLegacy(&domain.Store{ID: 5, OrganizationID: 1, Name: "Legacy"}, "legacy-key-2")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger())(
		requireScope(domain.ScopeTransactionsWrite)(okHandler()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer legacy-key-2")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (legacy key treated as fully trusted)", rec.Code)
	}
}

func TestRequireScope_MissingScopeRejected(t *testing.T) {
	stores := newFakeStoreRepo()
	apiKeys := newFakeStoreAPIKeyRepo()
	stores.byID[6] = &domain.Store{ID: 6, OrganizationID: 1, Name: "Store"}
	apiKeys.add(&domain.StoreAPIKey{ID: 13, StoreID: 6, Scopes: []string{domain.ScopeClientsLookup}}, "scoped-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger())(
		requireScope(domain.ScopeTransactionsWrite)(okHandler()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer scoped-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (key lacks transactions.write scope)", rec.Code)
	}
}

func TestRequireScope_WithScopeAllowed(t *testing.T) {
	stores := newFakeStoreRepo()
	apiKeys := newFakeStoreAPIKeyRepo()
	stores.byID[7] = &domain.Store{ID: 7, OrganizationID: 1, Name: "Store"}
	apiKeys.add(&domain.StoreAPIKey{ID: 14, StoreID: 7, Scopes: []string{domain.ScopeTransactionsWrite}}, "correctly-scoped-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger())(
		requireScope(domain.ScopeTransactionsWrite)(okHandler()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer correctly-scoped-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
