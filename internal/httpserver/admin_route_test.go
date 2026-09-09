package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
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

func TestHandleGetStaffMe_ReturnsProfileAndResolvedPermissions(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleStoreManager)
	fakes.StaffUsers.byID[1].Email = "manager@example.com"
	fakes.StaffUsers.byID[1].DisplayName = "Manager"

	req := adminReq(http.MethodGet, "/api/v1/staff/me", token, nil)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body staffMeResponseBody
	decodeJSON(t, rec, &body)
	if body.Email != "manager@example.com" || body.DisplayName != "Manager" {
		t.Fatalf("profile = %+v, want email/display_name populated", body)
	}
	if len(body.Memberships) != 1 || body.Memberships[0].OrganizationID != org.ID {
		t.Fatalf("memberships = %+v, want one membership in org %d", body.Memberships, org.ID)
	}
	if len(body.Memberships[0].Permissions) == 0 {
		t.Fatalf("permissions = empty, want store_manager's resolved permissions")
	}
}

func TestHandleGetStaffMe_NoMembershipReturns200WithEmptyList(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.StaffUsers.byID[1] = &domain.StaffUser{ID: 1, ExternalSubject: "sub-1", Status: domain.StaffActive}
	fakes.StaffUsers.nextID = 1
	token, err := auth.IssueStaffAccessToken(testStaffSecret, 1, time.Hour)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}

	req := adminReq(http.MethodGet, "/api/v1/staff/me", token, nil)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body staffMeResponseBody
	decodeJSON(t, rec, &body)
	if len(body.Memberships) != 0 {
		t.Fatalf("memberships = %+v, want empty", body.Memberships)
	}
}

func TestHandleListOrganizations_PlatformAdminSeesAll(t *testing.T) {
	handler, fakes := newTestServer(t)
	orgA := seedOrganization(fakes, "Org A")
	_ = seedOrganization(fakes, "Org B")
	token := issueStaffToken(t, fakes, 1, orgA.ID, nil, domain.RolePlatformAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations", token, nil)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Organizations []organizationResponseBody `json:"organizations"`
	}
	decodeJSON(t, rec, &body)
	if len(body.Organizations) != 2 {
		t.Fatalf("organizations = %+v, want both org A and org B for a platform_admin", body.Organizations)
	}
}

func TestHandleListOrganizations_RetailerAdminSeesOnlyOwnOrg(t *testing.T) {
	handler, fakes := newTestServer(t)
	orgA := seedOrganization(fakes, "Org A")
	_ = seedOrganization(fakes, "Org B")
	token := issueStaffToken(t, fakes, 1, orgA.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations", token, nil)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Organizations []organizationResponseBody `json:"organizations"`
	}
	decodeJSON(t, rec, &body)
	if len(body.Organizations) != 1 || body.Organizations[0].ID != orgA.ID {
		t.Fatalf("organizations = %+v, want only org A", body.Organizations)
	}
}

func TestHandleListStores_OrgWideMembershipSeesAllStores(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	_ = seedStore(fakes, org.ID, "Store 1")
	_ = seedStore(fakes, org.ID, "Store 2")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Stores []storeResponseBody `json:"stores"`
	}
	decodeJSON(t, rec, &body)
	if len(body.Stores) != 2 {
		t.Fatalf("stores = %+v, want both store 1 and store 2", body.Stores)
	}
}

func TestHandleListStores_StoreScopedMembershipSeesOnlyOwnStore(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	ownStore := seedStore(fakes, org.ID, "Own store")
	_ = seedStore(fakes, org.ID, "Other store")
	token := issueStaffToken(t, fakes, 1, org.ID, &ownStore.ID, domain.RoleStoreManager)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Stores []storeResponseBody `json:"stores"`
	}
	decodeJSON(t, rec, &body)
	if len(body.Stores) != 1 || body.Stores[0].ID != ownStore.ID {
		t.Fatalf("stores = %+v, want only the caller's own store %d", body.Stores, ownStore.ID)
	}
}

func TestHandleListStores_NoMembershipInOrgRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	orgA := seedOrganization(fakes, "Org A")
	orgB := seedOrganization(fakes, "Org B")
	seedStore(fakes, orgB.ID, "Store in B")
	// Caller only has a membership in org A, and tries to list org B's stores.
	token := issueStaffToken(t, fakes, 1, orgA.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(orgB.ID)+"/stores", token, nil)
	req.SetPathValue("orgID", itoa(orgB.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (cross-org IDOR)", rec.Code)
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

// TestHandleCreateStaffMembership_RetailerAdminCannotGrantPlatformAdmin: a
// retailer_admin holds staff.manage (same as platform_admin), but must
// never be able to mint a platform_admin membership for anyone, including
// themselves — that would be a privilege escalation out of their own
// organization scope.
func TestHandleCreateStaffMembership_RetailerAdminCannotGrantPlatformAdmin(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff", token, map[string]any{
		"external_subject": "oidc|wannabe-admin", "role": "platform_admin",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (retailer_admin must not grant platform_admin)", rec.Code)
	}
}

func TestHandleCreateStaffMembership_PlatformAdminCanGrantPlatformAdmin(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RolePlatformAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff", token, map[string]any{
		"external_subject": "oidc|new-admin", "role": "platform_admin",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (platform_admin may grant platform_admin, body=%s)", rec.Code, rec.Body.String())
	}
}

// TestHandleUpdateStaffMembership_RetailerAdminCannotEscalateToPlatformAdmin
// covers the same escalation via the update path, not just create.
func TestHandleUpdateStaffMembership_RetailerAdminCannotEscalateToPlatformAdmin(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	target := fakes.StaffMemberships.seed(&domain.StaffMembership{
		StaffUserID: 2, OrganizationID: org.ID, Role: domain.RoleStoreManager, Status: domain.StaffActive,
	})

	req := adminReq(http.MethodPatch, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff/"+itoa(target.ID), token, map[string]string{
		"role": "platform_admin",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("membershipID", itoa(target.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (retailer_admin must not escalate to platform_admin)", rec.Code)
	}
}

// TestHandleUpdateStaffMembership_RetailerAdminCannotDemoteOrDisablePlatformAdmin
// covers revocation: a retailer_admin with staff.manage in the same
// organization must not be able to demote or disable an existing
// platform_admin membership.
func TestHandleUpdateStaffMembership_RetailerAdminCannotDemoteOrDisablePlatformAdmin(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	target := fakes.StaffMemberships.seed(&domain.StaffMembership{
		StaffUserID: 2, OrganizationID: org.ID, Role: domain.RolePlatformAdmin, Status: domain.StaffActive,
	})

	req := adminReq(http.MethodPatch, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff/"+itoa(target.ID), token, map[string]string{
		"status": "disabled",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("membershipID", itoa(target.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (retailer_admin must not disable a platform_admin)", rec.Code)
	}
}

// TestHandleUpdateStaffMembership_CrossOrgIDORRejected: a membership id
// from a different organization must 404, not apply, even though the
// caller has staff.manage for the org named in the path — the path's orgID
// is not a proof that membershipID belongs to it.
func TestHandleUpdateStaffMembership_CrossOrgIDORRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	orgA := seedOrganization(fakes, "Org A")
	orgB := seedOrganization(fakes, "Org B")
	token := issueStaffToken(t, fakes, 1, orgA.ID, nil, domain.RoleRetailerAdmin)
	targetInB := fakes.StaffMemberships.seed(&domain.StaffMembership{
		StaffUserID: 2, OrganizationID: orgB.ID, Role: domain.RoleStoreManager, Status: domain.StaffActive,
	})

	req := adminReq(http.MethodPatch, "/api/v1/admin/organizations/"+itoa(orgA.ID)+"/staff/"+itoa(targetInB.ID), token, map[string]string{
		"status": "disabled",
	})
	req.SetPathValue("orgID", itoa(orgA.ID))
	req.SetPathValue("membershipID", itoa(targetInB.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (org A must not touch org B's membership by id)", rec.Code)
	}
	if targetInB.Status != domain.StaffActive {
		t.Errorf("org B's membership status = %q, want unchanged active", targetInB.Status)
	}
}

// TestHandleUpdateStaffMembership_CannotModifyOwnMembership guards against
// self-escalation/self-disable: a caller must not be able to change their
// own membership's role or status through this endpoint.
func TestHandleUpdateStaffMembership_CannotModifyOwnMembership(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	own := fakes.StaffMemberships.byID[fakes.StaffMemberships.nextID] // the membership issueStaffToken just seeded for staff user 1

	req := adminReq(http.MethodPatch, "/api/v1/admin/organizations/"+itoa(org.ID)+"/staff/"+itoa(own.ID), token, map[string]string{
		"status": "disabled",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("membershipID", itoa(own.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (cannot modify own membership)", rec.Code)
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

// TestHandleCreateStoreAPIKey_PlaintextAuthenticatesRealRequest is the
// create-then-use regression test for the key_id/key_hash mismatch that
// once made every freshly created store API key permanently unable to
// authenticate (GenerateAPIKey hashed only the secret half; the middleware
// hashed "<keyID>.<secret>" whole and could never find a match). It drives
// the real admin HTTP handler to create a key, then sends the exact
// plaintext_key it returns through RequireStoreAPIKey/requireScope on a
// real store-scoped route.
func TestHandleCreateStoreAPIKey_PlaintextAuthenticatesRealRequest(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	adminToken := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	seedPointsConfig(fakes, store.ID)

	createReq := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/api-keys", adminToken, map[string]any{
		"name": "1C webhook", "scopes": []string{domain.ScopeTransactionsWrite},
	})
	createReq.SetPathValue("orgID", itoa(org.ID))
	createReq.SetPathValue("storeID", itoa(store.ID))
	createRec := doRequest(handler, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body=%s)", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID           int64  `json:"id"`
		PlaintextKey string `json:"plaintext_key"`
	}
	decodeJSON(t, createRec, &created)
	if created.PlaintextKey == "" {
		t.Fatal("expected a non-empty plaintext_key")
	}

	// Permitted request: the plaintext key returned at creation must
	// authenticate a real transactions.write-scoped webhook call.
	rec := doRequest(handler, accrueReq(created.PlaintextKey, map[string]any{
		"transaction_id": "tx-created-key", "phone": "+79261234567", "amount": "50.00",
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("accrual with freshly created key: status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	// Missing scope: same store, a key without transactions.write must be
	// rejected for the accrual route.
	unscopedReq := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/api-keys", adminToken, map[string]any{
		"name": "read-only", "scopes": []string{domain.ScopeBalancesRead},
	})
	unscopedReq.SetPathValue("orgID", itoa(org.ID))
	unscopedReq.SetPathValue("storeID", itoa(store.ID))
	unscopedRec := doRequest(handler, unscopedReq)
	var unscoped struct {
		PlaintextKey string `json:"plaintext_key"`
	}
	decodeJSON(t, unscopedRec, &unscoped)
	rec = doRequest(handler, accrueReq(unscoped.PlaintextKey, map[string]any{
		"transaction_id": "tx-unscoped", "phone": "+79261234567", "amount": "50.00",
	}))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("accrual with balances.read-only key: status = %d, want 403", rec.Code)
	}

	// Modified secret: flipping the last character of a real, valid key
	// must be rejected, not silently accepted or 500.
	tampered := created.PlaintextKey[:len(created.PlaintextKey)-1] + "x"
	if tampered == created.PlaintextKey {
		tampered = created.PlaintextKey[:len(created.PlaintextKey)-1] + "y"
	}
	rec = doRequest(handler, accrueReq(tampered, map[string]any{
		"transaction_id": "tx-tampered", "phone": "+79261234567", "amount": "50.00",
	}))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("accrual with tampered key: status = %d, want 401", rec.Code)
	}

	// Revoked: revoke the originally created key via the admin API, then
	// confirm it can no longer authenticate.
	revokeReq := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/api-keys/"+itoa(created.ID)+"/revoke", adminToken, nil)
	revokeReq.SetPathValue("orgID", itoa(org.ID))
	revokeReq.SetPathValue("storeID", itoa(store.ID))
	revokeReq.SetPathValue("keyID", itoa(created.ID))
	revokeRec := doRequest(handler, revokeReq)
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, want 204 (body=%s)", revokeRec.Code, revokeRec.Body.String())
	}

	rec = doRequest(handler, accrueReq(created.PlaintextKey, map[string]any{
		"transaction_id": "tx-after-revoke", "phone": "+79261234567", "amount": "50.00",
	}))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("accrual with revoked key: status = %d, want 401", rec.Code)
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

// TestHandleRevokeStoreAPIKey_CrossStoreRejected: a key belonging to store B
// must not be revocable through store A's path, even though both stores
// are in the same organization and the caller's org-scoped role would pass
// resolveScopedStore for store A. Guards the storeID-scoped Revoke query.
func TestHandleRevokeStoreAPIKey_CrossStoreRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	storeA := seedStore(fakes, org.ID, "Store A")
	storeB := seedStore(fakes, org.ID, "Store B")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 9, StoreID: storeB.ID, Name: "store-b-key"}, "store-b-secret")

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(storeA.ID)+"/api-keys/9/revoke", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(storeA.ID))
	req.SetPathValue("keyID", "9")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (store A must not revoke store B's key)", rec.Code)
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
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (negative accrual_percent)", rec.Code)
	}
}

func TestHandlePutLoyaltyConfig_MalformedJSONRejectedWith400(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", stringBody("{not json"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (malformed JSON, distinct from a 422 validation failure)", rec.Code)
	}
}

func TestHandlePutLoyaltyConfig_UnknownMechanicRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPut, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", token, map[string]any{
		"mechanic": "punch_card_not_implemented", "accrual_percent": "10", "min_purchase_amount": "0",
		"min_balance_to_redeem": 0, "max_redeem_percent": "100", "points_exchange_rate": "1",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (unknown mechanic, rejected at config-write time instead of surfacing as a 500 on the next accrual)", rec.Code)
	}
}

func TestHandlePutLoyaltyConfig_PercentAbove100Rejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPut, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", token, map[string]any{
		"mechanic": "points", "accrual_percent": "150", "min_purchase_amount": "0",
		"min_balance_to_redeem": 0, "max_redeem_percent": "100", "points_exchange_rate": "1",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (accrual_percent=150 is out of [0,100])", rec.Code)
	}
}

func TestHandlePutLoyaltyConfig_ZeroExchangeRateRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPut, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", token, map[string]any{
		"mechanic": "points", "accrual_percent": "10", "min_purchase_amount": "0",
		"min_balance_to_redeem": 0, "max_redeem_percent": "100", "points_exchange_rate": "0",
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (points_exchange_rate must be positive)", rec.Code)
	}
}

func TestHandlePutLoyaltyConfig_VersionIncrementsAndHistoryRecorded(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	put := func(accrualPercent string) *httptest.ResponseRecorder {
		req := adminReq(http.MethodPut, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", token, map[string]any{
			"mechanic": "points", "accrual_percent": accrualPercent, "min_purchase_amount": "0",
			"min_balance_to_redeem": 0, "max_redeem_percent": "100", "points_exchange_rate": "1",
		})
		req.SetPathValue("orgID", itoa(org.ID))
		req.SetPathValue("storeID", itoa(store.ID))
		return doRequest(handler, req)
	}

	first := put("10")
	if first.Code != http.StatusOK {
		t.Fatalf("first put status = %d, want 200 (body=%s)", first.Code, first.Body.String())
	}
	var firstResp loyaltyConfigResponseBody
	decodeJSON(t, first, &firstResp)
	if firstResp.Version != 1 {
		t.Fatalf("first put Version = %d, want 1", firstResp.Version)
	}

	second := put("20")
	if second.Code != http.StatusOK {
		t.Fatalf("second put status = %d, want 200 (body=%s)", second.Code, second.Body.String())
	}
	var secondResp loyaltyConfigResponseBody
	decodeJSON(t, second, &secondResp)
	if secondResp.Version != 2 {
		t.Fatalf("second put Version = %d, want 2", secondResp.Version)
	}

	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config/history", token, nil)
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("history status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var histResp struct {
		History []loyaltyConfigVersionResponseBody `json:"history"`
	}
	decodeJSON(t, rec, &histResp)
	if len(histResp.History) != 2 {
		t.Fatalf("history length = %d, want 2", len(histResp.History))
	}
	// Newest version first.
	if histResp.History[0].Version != 2 || histResp.History[0].AccrualPercent != "20" {
		t.Errorf("history[0] = %+v, want version=2 accrual_percent=20", histResp.History[0])
	}
	if histResp.History[1].Version != 1 || histResp.History[1].AccrualPercent != "10" {
		t.Errorf("history[1] = %+v, want version=1 accrual_percent=10", histResp.History[1])
	}
}

func TestHandleRollbackLoyaltyConfig_ReappliesOldVersionAsNewVersion(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	put := func(accrualPercent string) {
		req := adminReq(http.MethodPut, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config", token, map[string]any{
			"mechanic": "points", "accrual_percent": accrualPercent, "min_purchase_amount": "0",
			"min_balance_to_redeem": 0, "max_redeem_percent": "100", "points_exchange_rate": "1",
		})
		req.SetPathValue("orgID", itoa(org.ID))
		req.SetPathValue("storeID", itoa(store.ID))
		if rec := doRequest(handler, req); rec.Code != http.StatusOK {
			t.Fatalf("put(%q) status = %d, want 200 (body=%s)", accrualPercent, rec.Code, rec.Body.String())
		}
	}
	put("10") // version 1
	put("20") // version 2

	rollbackReq := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config/rollback", token, map[string]any{"version": 1})
	rollbackReq.SetPathValue("orgID", itoa(org.ID))
	rollbackReq.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, rollbackReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp loyaltyConfigResponseBody
	decodeJSON(t, rec, &resp)
	// Rollback creates version 3 carrying version 1's values — history stays
	// forward-only, never rewritten.
	if resp.Version != 3 {
		t.Fatalf("rollback Version = %d, want 3", resp.Version)
	}
	if resp.AccrualPercent != "10" {
		t.Fatalf("rollback AccrualPercent = %q, want 10 (version 1's value)", resp.AccrualPercent)
	}
}

func TestHandleRollbackLoyaltyConfig_UnknownVersionRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/loyalty-config/rollback", token, map[string]any{"version": 99})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (no config/history exists yet)", rec.Code)
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
