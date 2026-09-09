package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
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

func (f *fakeStoreRepo) ListByOrganization(_ context.Context, organizationID int64) ([]*domain.Store, error) {
	var out []*domain.Store
	for _, s := range f.byID {
		if s.OrganizationID == organizationID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
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
//
// It stores domain.StoreAPIKey by value (not by pointer) and hands callers
// fresh copies on every read, guarded by mu. RequireStoreAPIKey reads the
// key it resolved (in the request goroutine) while its background
// TouchLastUsed call (see BackgroundTracker.Go in background.go) mutates
// the same key concurrently; sharing one *domain.StoreAPIKey between the
// two used to race under `go test -race` (found in the
// 2026-09-09 clean-checkout CI baseline, Q-P0-126) because both the map
// access and the pointed-to struct's fields were touched without
// synchronization. Copy-out-on-read plus a mutex around the map removes
// both races: no goroutine ever mutates a struct another goroutine holds.
type fakeStoreAPIKeyRepo struct {
	mu      sync.Mutex
	byKeyID map[string]domain.StoreAPIKey
	nextID  int64
}

func newFakeStoreAPIKeyRepo() *fakeStoreAPIKeyRepo {
	return &fakeStoreAPIKeyRepo{byKeyID: map[string]domain.StoreAPIKey{}}
}

func (f *fakeStoreAPIKeyRepo) GetByKeyID(_ context.Context, keyID string) (*domain.StoreAPIKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k, ok := f.byKeyID[keyID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := k
	return &out, nil
}

func (f *fakeStoreAPIKeyRepo) ListByStore(_ context.Context, storeID int64) ([]*domain.StoreAPIKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.StoreAPIKey
	for _, k := range f.byKeyID {
		if k.StoreID == storeID {
			kk := k
			out = append(out, &kk)
		}
	}
	return out, nil
}

func (f *fakeStoreAPIKeyRepo) Create(_ context.Context, k *domain.StoreAPIKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if k.ID == 0 {
		f.nextID++
		k.ID = f.nextID
	}
	k.CreatedAt = time.Now()
	f.byKeyID[k.KeyID] = *k
	return nil
}

func (f *fakeStoreAPIKeyRepo) Revoke(_ context.Context, storeID, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for keyID, k := range f.byKeyID {
		if k.ID == id && k.StoreID == storeID {
			now := time.Now()
			k.RevokedAt = &now
			f.byKeyID[keyID] = k
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakeStoreAPIKeyRepo) TouchLastUsed(_ context.Context, id int64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for keyID, k := range f.byKeyID {
		if k.ID == id {
			k.LastUsedAt = &at
			f.byKeyID[keyID] = k
		}
	}
	return nil
}

// add stores k under a generated KeyID and hashes plaintextSecret into
// KeyHash, mirroring auth.GenerateAPIKey's "<keyID>.<secret>" split. It
// returns the full bearer-token plaintext a client would send.
func (f *fakeStoreAPIKeyRepo) add(k *domain.StoreAPIKey, plaintextSecret string) string {
	if k.KeyID == "" {
		k.KeyID = fmt.Sprintf("key-%d", k.ID)
	}
	k.KeyHash = sha256Hex(plaintextSecret)
	f.mu.Lock()
	f.byKeyID[k.KeyID] = *k
	f.mu.Unlock()
	return k.KeyID + "." + plaintextSecret
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
	handler := RequireStoreAPIKey(stores, apiKeys, testLogger(), NewBackgroundTracker())(func(w http.ResponseWriter, r *http.Request) {
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
	bearer := apiKeys.add(&domain.StoreAPIKey{ID: 10, StoreID: 2, Scopes: []string{domain.ScopeTransactionsWrite}}, "new-plaintext-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger(), NewBackgroundTracker())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
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
	bearer := apiKeys.add(&domain.StoreAPIKey{ID: 11, StoreID: 3, RevokedAt: &now}, "revoked-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger(), NewBackgroundTracker())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
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
	bearer := apiKeys.add(&domain.StoreAPIKey{ID: 12, StoreID: 4, ExpiresAt: &past}, "expired-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger(), NewBackgroundTracker())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireStoreAPIKey_MissingKeyRejected(t *testing.T) {
	handler := RequireStoreAPIKey(newFakeStoreRepo(), newFakeStoreAPIKeyRepo(), testLogger(), NewBackgroundTracker())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireStoreAPIKey_UnknownKeyRejected(t *testing.T) {
	handler := RequireStoreAPIKey(newFakeStoreRepo(), newFakeStoreAPIKeyRepo(), testLogger(), NewBackgroundTracker())(okHandler())

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

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger(), NewBackgroundTracker())(
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
	bearer := apiKeys.add(&domain.StoreAPIKey{ID: 13, StoreID: 6, Scopes: []string{domain.ScopeClientsLookup}}, "scoped-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger(), NewBackgroundTracker())(
		requireScope(domain.ScopeTransactionsWrite)(okHandler()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
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
	bearer := apiKeys.add(&domain.StoreAPIKey{ID: 14, StoreID: 7, Scopes: []string{domain.ScopeTransactionsWrite}}, "correctly-scoped-key")

	handler := RequireStoreAPIKey(stores, apiKeys, testLogger(), NewBackgroundTracker())(
		requireScope(domain.ScopeTransactionsWrite)(okHandler()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
