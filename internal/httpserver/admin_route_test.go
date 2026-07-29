package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

func adminReq(method, path, token string, body any) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, jsonBodyAny(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// --- admin_organizations.go ---

func TestHandleCreateOrganization_MalformedBody(t *testing.T) {
	handler, fakes := newTestServer(t)
	token := issueStaffToken(t, fakes, 1, 1, nil, domain.RolePlatformAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations", token, map[string]string{})
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (empty name)", rec.Code)
	}
}

func TestHandleCreateStore_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores", token, map[string]string{"name": "New Store"})
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateStore_MalformedBody(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores", token, map[string]string{"name": ""})
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleGetStore_NotFoundWrongOrg(t *testing.T) {
	handler, fakes := newTestServer(t)
	orgA := seedOrganization(fakes, "Org A")
	orgB := seedOrganization(fakes, "Org B")
	storeInB := seedStore(fakes, orgB.ID, "Store in B")
	// Staff with a platform_admin membership can pass the permission check
	// for org A but the store itself belongs to org B — must 404, not leak
	// the store's real organization.
	token := issueStaffToken(t, fakes, 1, orgA.ID, nil, domain.RolePlatformAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(orgA.ID)+"/stores/"+itoa(storeInB.ID), token, nil)
	req.SetPathValue("orgID", itoa(orgA.ID))
	req.SetPathValue("storeID", itoa(storeInB.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// --- admin_staff.go ---

func TestHandleListStaffMemberships_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateStaffMembership_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff", token, map[string]any{
		"external_subject": "oidc|new-employee", "role": "store_manager",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateStaffMembership_InvalidRole(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff", token, map[string]any{
		"external_subject": "oidc|new-employee", "role": "super_admin",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (invalid role)", rec.Code)
	}
}

func TestHandleCreateStaffMembership_ConflictOnDuplicate(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	body := map[string]any{"external_subject": "oidc|dup", "role": "store_manager"}
	first := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff", token, body)
	first.SetPathValue("orgID", itoa(org.ID))
	if rec := doRequest(handler, first); rec.Code != http.StatusCreated {
		t.Fatalf("first request status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
	}

	second := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff", token, body)
	second.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, second)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (duplicate membership)", rec.Code)
	}
}

func TestHandleUpdateStaffMembership_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	target := fakes.StaffMemberships.seed(&domain.StaffMembership{
		StaffUserID: 2, OrganizationID: org.ID, Role: domain.RoleStoreManager, Status: domain.StaffActive,
	})

	req := adminReq(http.MethodPatch, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff/"+itoa(target.ID), token, map[string]string{
		"status": "disabled",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("membershipID", itoa(target.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", rec.Code, rec.Body.String())
	}
	if target.Status != domain.StaffDisabled {
		t.Errorf("membership status = %q, want disabled", target.Status)
	}
}

func TestHandleUpdateStaffMembership_InvalidStatus(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	target := fakes.StaffMemberships.seed(&domain.StaffMembership{
		StaffUserID: 2, OrganizationID: org.ID, Role: domain.RoleStoreManager, Status: domain.StaffActive,
	})

	req := adminReq(http.MethodPatch, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff/"+itoa(target.ID), token, map[string]string{
		"status": "not-a-status",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("membershipID", itoa(target.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleUpdateStaffMembership_MissingRoleAndStatus(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	target := fakes.StaffMemberships.seed(&domain.StaffMembership{
		StaffUserID: 2, OrganizationID: org.ID, Role: domain.RoleStoreManager, Status: domain.StaffActive,
	})

	req := adminReq(http.MethodPatch, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff/"+itoa(target.ID), token, map[string]string{})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("membershipID", itoa(target.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- admin_apikeys.go ---

func TestHandleListStoreAPIKeys_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: store.ID, Name: "1C"}, "existing-key")

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/api-keys", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateStoreAPIKey_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/api-keys", token, map[string]any{
		"name": "1C webhook", "scopes": []string{domain.ScopeTransactionsWrite},
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		PlaintextKey string `json:"plaintext_key"`
	}
	decodeJSON(t, rec, &resp)
	if resp.PlaintextKey == "" {
		t.Error("expected a plaintext_key in the response")
	}
}

func TestHandleCreateStoreAPIKey_InvalidScope(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/api-keys", token, map[string]any{
		"name": "bad", "scopes": []string{"not.a.real.scope"},
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleRevokeStoreAPIKey_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 5, StoreID: store.ID, Name: "old"}, "to-be-revoked")

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/api-keys/5/revoke", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	req.SetPathValue("keyID", "5")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", rec.Code, rec.Body.String())
	}
}

// --- admin_clients.go ---

func TestHandleAdminLookupClient_MaskedForSupportViewer(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	client := fakes.Clients.seed(&domain.Client{StoreID: store.ID, Phone: "+79261234567"})
	fakes.Balances.set(client.ID, 10)
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleSupportViewer)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/clients/lookup?phone=%2B79261234567", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp adminClientResponseBody
	decodeJSON(t, rec, &resp)
	if resp.Phone == "+79261234567" {
		t.Errorf("phone = %q, want masked for support_viewer", resp.Phone)
	}
}

func TestHandleAdminLookupClient_UnmaskedForRetailerAdmin(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	client := fakes.Clients.seed(&domain.Client{StoreID: store.ID, Phone: "+79261234567"})
	fakes.Balances.set(client.ID, 10)
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/clients/lookup?phone=%2B79261234567", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp adminClientResponseBody
	decodeJSON(t, rec, &resp)
	if resp.Phone != "+79261234567" {
		t.Errorf("phone = %q, want unmasked full phone for retailer_admin", resp.Phone)
	}
}

func TestHandleAdminLookupClient_NotFound(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/clients/lookup?phone=+79260000000", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleAdminListClientTransactions_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	client := fakes.Clients.seed(&domain.Client{StoreID: store.ID, Phone: "+79261234567"})
	fakes.Transactions.all = append(fakes.Transactions.all, &domain.Transaction{
		ID: 1, StoreID: store.ID, ClientID: client.ID, ExternalTxID: "tx-1", Type: domain.TransactionAccrual, PointsDelta: 10, BalanceAfter: 10,
	})
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/clients/"+itoa(client.ID)+"/transactions", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	req.SetPathValue("clientID", itoa(client.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Transactions []adminTransactionResponseBody `json:"transactions"`
	}
	decodeJSON(t, rec, &resp)
	if len(resp.Transactions) != 1 {
		t.Fatalf("len(Transactions) = %d, want 1", len(resp.Transactions))
	}
}

func TestHandleAdminListClientTransactions_NotFoundWrongStore(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	otherStore := seedStore(fakes, org.ID, "Other Store")
	clientInOtherStore := fakes.Clients.seed(&domain.Client{StoreID: otherStore.ID, Phone: "+79261111111"})
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/clients/"+itoa(clientInOtherStore.ID)+"/transactions", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	req.SetPathValue("clientID", itoa(clientInOtherStore.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (client belongs to a different store)", rec.Code)
	}
}

// --- admin_loyaltyconfig.go ---

func TestHandleGetLoyaltyConfig_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	seedPointsConfig(fakes, store.ID)
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp loyaltyConfigResponseBody
	decodeJSON(t, rec, &resp)
	if resp.Mechanic != "points" {
		t.Errorf("Mechanic = %q, want points", resp.Mechanic)
	}
}

func TestHandleGetLoyaltyConfig_NotFound(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandlePutLoyaltyConfig_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPut, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", token, map[string]any{
		"mechanic": "points", "accrual_percent": "10", "min_purchase_amount": "0",
		"min_balance_to_redeem": 0, "max_redeem_percent": "100", "points_exchange_rate": "1",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestHandlePutLoyaltyConfig_NegativeValueRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPut, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", token, map[string]any{
		"mechanic": "points", "accrual_percent": "-5", "min_purchase_amount": "0",
		"min_balance_to_redeem": 0, "max_redeem_percent": "100", "points_exchange_rate": "1",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (negative accrual_percent)", rec.Code)
	}
}

// --- admin_audit.go ---

func TestHandleListAuditEvents_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	orgID := org.ID
	_ = fakes.AuditEvents.Create(context.Background(), &domain.AuditEvent{
		OrganizationID: &orgID, ActorType: domain.AuditActorStaff, Action: domain.AuditActionStaffLogin,
	})

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/audit", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Events []auditEventResponseBody `json:"events"`
	}
	decodeJSON(t, rec, &resp)
	if len(resp.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(resp.Events))
	}
}
