package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// staffLoginReq builds a staff OIDC login request from remoteAddr — the
// body is intentionally malformed (missing id_token, 400) since these
// tests only care about the pre-auth IP rate limit, which wraps outside
// request validation.
func staffLoginReq(remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/staff/auth/oidc", jsonBodyAny(map[string]string{}))
	req.RemoteAddr = remoteAddr
	return req
}

func TestRateLimit_StaffLoginByIPExceeded(t *testing.T) {
	cfg := testRateLimitConfig()
	deps, _, _ := newTestDepsWithRateLimit(t, cfg)
	handler := New(deps).Handler

	for i := 1; i <= cfg.StaffLoginIPLimit; i++ {
		rec := doRequest(handler, staffLoginReq("203.0.113.1:1111"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("request %d: status = %d, want 400 (under the rate limit, request validation still runs)", i, rec.Code)
		}
	}

	rec := doRequest(handler, staffLoginReq("203.0.113.1:1111"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (exceeded staff_login IP limit)", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected a Retry-After header on 429")
	}
}

func TestRateLimit_StaffLoginIsolatedByIP(t *testing.T) {
	cfg := testRateLimitConfig()
	deps, _, _ := newTestDepsWithRateLimit(t, cfg)
	handler := New(deps).Handler

	for i := 1; i <= cfg.StaffLoginIPLimit; i++ {
		doRequest(handler, staffLoginReq("203.0.113.2:2222"))
	}
	// A different caller IP must have its own, untouched quota.
	rec := doRequest(handler, staffLoginReq("203.0.113.3:3333"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (a different IP must not share the exhausted caller's quota)", rec.Code)
	}
}

func TestRateLimit_AdminStaffPrincipalExceeded(t *testing.T) {
	cfg := testRateLimitConfig()
	deps, fakes, _ := newTestDepsWithRateLimit(t, cfg)
	handler := New(deps).Handler

	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := func() *http.Request {
		r := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID), token, nil)
		r.SetPathValue("orgID", itoa(org.ID))
		r.SetPathValue("storeID", itoa(store.ID))
		return r
	}

	for i := 1; i <= cfg.AdminStaffLimit; i++ {
		rec := doRequest(handler, req())
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 (body=%s)", i, rec.Code, rec.Body.String())
		}
	}

	rec := doRequest(handler, req())
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (exceeded admin staff-user limit)", rec.Code)
	}
}

func TestRateLimit_AdminStaffPrincipalIsolatedBetweenStaffUsers(t *testing.T) {
	cfg := testRateLimitConfig()
	deps, fakes, _ := newTestDepsWithRateLimit(t, cfg)
	handler := New(deps).Handler

	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	tokenA := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	tokenB := issueStaffToken(t, fakes, 2, org.ID, nil, domain.RoleRetailerAdmin)

	reqWith := func(token string) *http.Request {
		r := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID), token, nil)
		r.SetPathValue("orgID", itoa(org.ID))
		r.SetPathValue("storeID", itoa(store.ID))
		// Same IP for both staff users — only the per-staff-user dimension
		// should isolate them, not IP.
		r.RemoteAddr = "203.0.113.9:9999"
		return r
	}

	for i := 1; i <= cfg.AdminStaffLimit; i++ {
		if rec := doRequest(handler, reqWith(tokenA)); rec.Code != http.StatusOK {
			t.Fatalf("staff A request %d: status = %d, want 200", i, rec.Code)
		}
	}
	if rec := doRequest(handler, reqWith(tokenA)); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("staff A over limit: status = %d, want 429", rec.Code)
	}

	// Staff B, same IP, must have an independent quota.
	if rec := doRequest(handler, reqWith(tokenB)); rec.Code != http.StatusOK {
		t.Fatalf("staff B request: status = %d, want 200 (isolated from staff A's exhausted quota)", rec.Code)
	}
}

func TestRateLimit_AccrualPrincipalIsolatedByStoreAPIKey(t *testing.T) {
	cfg := testRateLimitConfig()
	deps, fakes, _ := newTestDepsWithRateLimit(t, cfg)
	handler := New(deps).Handler

	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, KeyID: "key-id-a", Scopes: []string{domain.ScopeTransactionsWrite}}, "key-a")
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 2, StoreID: 1, KeyID: "key-id-b", Scopes: []string{domain.ScopeTransactionsWrite}}, "key-b")
	seedPointsConfig(fakes, 1)

	makeReq := func(key, txID string) *http.Request {
		r := accrueReq(key, map[string]any{"transaction_id": txID, "phone": "+79261234567", "amount": "10.00"})
		r.RemoteAddr = "203.0.113.10:1010"
		return r
	}

	for i := 1; i <= cfg.AccrualPrincipalLimit; i++ {
		rec := doRequest(handler, makeReq("key-a", "tx-a-"+itoa(int64(i))))
		if rec.Code != http.StatusOK {
			t.Fatalf("key-a request %d: status = %d, want 200 (body=%s)", i, rec.Code, rec.Body.String())
		}
	}
	if rec := doRequest(handler, makeReq("key-a", "tx-a-over")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("key-a over limit: status = %d, want 429", rec.Code)
	}

	// A different store API key for the same store has its own quota.
	if rec := doRequest(handler, makeReq("key-b", "tx-b-1")); rec.Code != http.StatusOK {
		t.Fatalf("key-b request: status = %d, want 200 (isolated from key-a's exhausted quota)", rec.Code)
	}
}

func TestRateLimit_ClientLookupPhoneDimensionExceeded(t *testing.T) {
	cfg := testRateLimitConfig()
	deps, fakes, _ := newTestDepsWithRateLimit(t, cfg)
	handler := New(deps).Handler

	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store A"}
	fakes.Stores.byID[2] = &domain.Store{ID: 2, OrganizationID: 1, Name: "Store B"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeClientsLookup}}, "key-a")
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 2, StoreID: 2, Scopes: []string{domain.ScopeClientsLookup}}, "key-b")
	fakes.Clients.seed(&domain.Client{StoreID: 1, Phone: "+79261234567"})
	fakes.Clients.seed(&domain.Client{StoreID: 2, Phone: "+79261234567"})

	lookupReq := func(key string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=%2B79261234567", nil)
		r.Header.Set("Authorization", "Bearer "+key)
		r.RemoteAddr = "203.0.113.11:1111"
		return r
	}

	// The phone dimension is shared across principals (anti-enumeration):
	// alternating keys still exhausts the shared phone-hash bucket.
	for i := 1; i <= cfg.ClientLookupPhoneLimit; i++ {
		key := "key-a"
		if i%2 == 0 {
			key = "key-b"
		}
		rec := doRequest(handler, lookupReq(key))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d (key=%s): status = %d, want 200 (body=%s)", i, key, rec.Code, rec.Body.String())
		}
	}

	rec := doRequest(handler, lookupReq("key-a"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (shared phone-hash quota exhausted across both keys)", rec.Code)
	}
}

func TestRateLimit_BackendUnavailableReturns503(t *testing.T) {
	cfg := testRateLimitConfig()
	deps, _, mr := newTestDepsWithRateLimit(t, cfg)
	mr.Close() // simulate the limiter's Redis backend going down mid-flight

	handler := New(deps).Handler
	rec := doRequest(handler, staffLoginReq("203.0.113.20:2020"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (rate limiter backend down must fail closed)", rec.Code)
	}
}

func TestRateLimit_RedisKeysContainNoRawPhoneOrIP(t *testing.T) {
	cfg := testRateLimitConfig()
	deps, fakes, mr := newTestDepsWithRateLimit(t, cfg)
	handler := New(deps).Handler

	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeClientsLookup}}, "key-a")
	fakes.Clients.seed(&domain.Client{StoreID: 1, Phone: "+79261234567"})

	const rawIP = "198.51.100.42"
	const rawPhone = "+79261234567"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=%2B79261234567", nil)
	req.Header.Set("Authorization", "Bearer key-a")
	req.RemoteAddr = rawIP + ":4242"
	doRequest(handler, req)

	for _, key := range mr.Keys() {
		if strings.Contains(key, rawIP) {
			t.Errorf("Redis key %q contains the raw caller IP", key)
		}
		if strings.Contains(key, rawPhone) {
			t.Errorf("Redis key %q contains the raw phone number", key)
		}
		if strings.Contains(key, "key-a") {
			t.Errorf("Redis key %q contains the raw store API key secret", key)
		}
	}
}
