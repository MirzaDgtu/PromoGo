package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

// jsonBodyAny is jsonBody without a *testing.T, for helpers that build
// requests outside a test function body (e.g. accrueReq).
func jsonBodyAny(v any) *bytes.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(b)
}

func stringBody(s string) *strings.Reader { return strings.NewReader(s) }

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
}

// jsonBody encodes v as a JSON request body reader, for POST/PUT/PATCH
// requests across this package's route-level tests.
func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return bytes.NewReader(b)
}

func doRequest(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// --- Health / readiness ---

func TestHealthz(t *testing.T) {
	handler, _ := newTestServer(t)
	rec := doRequest(handler, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyz_Healthy(t *testing.T) {
	handler, _ := newTestServer(t)
	rec := doRequest(handler, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyz_DependencyDown(t *testing.T) {
	deps, _ := newTestDeps(t)
	deps.Ready = func(context.Context) error { return errors.New("postgres: connection refused") }
	handler := New(deps).Handler

	rec := doRequest(handler, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// --- Method-not-allowed (every route is registered under a single method) ---

func TestWrongHTTPMethod_Returns405(t *testing.T) {
	handler, _ := newTestServer(t)
	rec := doRequest(handler, httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// --- Store API key security matrix (see requireStoreKey/requireScope in server.go) ---

func TestStoreKeySecurityMatrix(t *testing.T) {
	t.Run("missing key rejected", func(t *testing.T) {
		handler, _ := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=+79261234567", nil)
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("unknown key rejected", func(t *testing.T) {
		handler, _ := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=+79261234567", nil)
		req.Header.Set("Authorization", "Bearer does-not-exist")
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("revoked key rejected", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
		now := time.Now()
		fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, RevokedAt: &now, Scopes: []string{domain.ScopeClientsLookup}}, "revoked-key")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=+79261234567", nil)
		req.Header.Set("Authorization", "Bearer revoked-key")
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("expired key rejected", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
		past := time.Now().Add(-time.Hour)
		fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, ExpiresAt: &past, Scopes: []string{domain.ScopeClientsLookup}}, "expired-key")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=+79261234567", nil)
		req.Header.Set("Authorization", "Bearer expired-key")
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("missing scope rejected", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
		fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeBalancesRead}}, "wrong-scope-key")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=+79261234567", nil)
		req.Header.Set("Authorization", "Bearer wrong-scope-key")
		rec := doRequest(handler, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403 (key lacks clients.lookup scope)", rec.Code)
		}
	})

	t.Run("correct scope allowed", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
		fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeClientsLookup}}, "correct-key")
		fakes.Clients.seed(&domain.Client{StoreID: 1, Phone: "+79261234567"})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=+79261234567", nil)
		req.Header.Set("Authorization", "Bearer correct-key")
		rec := doRequest(handler, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("one store's key cannot reach another store's client", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store A"}
		fakes.Stores.byID[2] = &domain.Store{ID: 2, OrganizationID: 1, Name: "Store B"}
		fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeBalancesRead}}, "store-a-key")
		otherStoreClient := fakes.Clients.seed(&domain.Client{StoreID: 2, Phone: "+79267654321"})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/"+itoa(otherStoreClient.ID)+"/balance", nil)
		req.SetPathValue("id", itoa(otherStoreClient.ID))
		req.Header.Set("Authorization", "Bearer store-a-key")
		rec := doRequest(handler, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (store A must not see store B's client)", rec.Code)
		}
	})
}

// --- Staff/admin security matrix (see RequireStaff/RequireGlobalStaffPermission) ---

func TestStaffSecurityMatrix(t *testing.T) {
	t.Run("missing token rejected", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		org := seedOrganization(fakes, "Acme")
		store := seedStore(fakes, org.ID, "Store")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID), nil)
		req.SetPathValue("orgID", itoa(org.ID))
		req.SetPathValue("storeID", itoa(store.ID))
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid token rejected", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		org := seedOrganization(fakes, "Acme")
		store := seedStore(fakes, org.ID, "Store")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID), nil)
		req.SetPathValue("orgID", itoa(org.ID))
		req.SetPathValue("storeID", itoa(store.ID))
		req.Header.Set("Authorization", "Bearer not-a-real-token")
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("missing global permission rejected", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		token := issueStaffToken(t, fakes, 1, 1, nil, domain.RoleRetailerAdmin)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations", jsonBody(t, map[string]string{"name": "New Org"}))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := doRequest(handler, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403 (retailer_admin lacks organizations.manage)", rec.Code)
		}
	})

	t.Run("global permission allowed", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		token := issueStaffToken(t, fakes, 1, 1, nil, domain.RolePlatformAdmin)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations", jsonBody(t, map[string]string{"name": "New Org"}))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := doRequest(handler, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("cross-organization access rejected (IDOR)", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		orgA := seedOrganization(fakes, "Org A")
		orgB := seedOrganization(fakes, "Org B")
		storeInB := seedStore(fakes, orgB.ID, "Store in B")
		token := issueStaffToken(t, fakes, 1, orgA.ID, nil, domain.RoleRetailerAdmin)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/"+itoa(orgB.ID)+"/stores/"+itoa(storeInB.ID), nil)
		req.SetPathValue("orgID", itoa(orgB.ID))
		req.SetPathValue("storeID", itoa(storeInB.ID))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := doRequest(handler, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403 (staff scoped to org A must not reach org B)", rec.Code)
		}
	})

	t.Run("correct permission and scope allowed", func(t *testing.T) {
		handler, fakes := newTestServer(t)
		org := seedOrganization(fakes, "Acme")
		store := seedStore(fakes, org.ID, "Store")
		token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID), nil)
		req.SetPathValue("orgID", itoa(org.ID))
		req.SetPathValue("storeID", itoa(store.ID))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := doRequest(handler, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
	})
}

// --- Cross-contour credential isolation ---

func TestCredentialsAreNotInterchangeable(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: store.ID, Scopes: []string{domain.ScopeClientsLookup}}, "a-store-api-key")
	staffToken := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	customerToken := issueCustomerToken(t, 1)

	t.Run("staff token rejected on customer route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+staffToken)
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("customer token rejected on staff route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID), nil)
		req.SetPathValue("orgID", itoa(org.ID))
		req.SetPathValue("storeID", itoa(store.ID))
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("store api key rejected on customer route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer a-store-api-key")
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("customer token rejected on store-key route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=+79261234567", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken)
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("staff token rejected on store-key route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=+79261234567", nil)
		req.Header.Set("Authorization", "Bearer "+staffToken)
		rec := doRequest(handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})
}

// --- Shared seeding helpers used across this package's route-level tests ---

func seedOrganization(fakes *testFakes, name string) *domain.Organization {
	org := &domain.Organization{Name: name}
	_ = fakes.Organizations.Create(context.Background(), org)
	return org
}

// seedStore inserts directly into fakeStoreRepo's map: unlike
// fakeOrganizationRepo, fakeStoreRepo.Create (middleware_test.go) doesn't
// auto-assign an ID — it's designed for tests that set one explicitly.
func seedStore(fakes *testFakes, orgID int64, name string) *domain.Store {
	id := int64(len(fakes.Stores.byID) + 1)
	store := &domain.Store{ID: id, OrganizationID: orgID, Name: name}
	fakes.Stores.byID[id] = store
	return store
}
