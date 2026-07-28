package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/service"
)

var testStaffSecret = []byte("test-secret-at-least-32-bytes-long!")

// fakeStaffResolver implements staffPrincipalResolver for middleware tests.
type fakeStaffResolver struct {
	principal *service.StaffPrincipal
	err       error
}

func (f *fakeStaffResolver) ResolvePrincipal(_ context.Context, _ int64) (*service.StaffPrincipal, error) {
	return f.principal, f.err
}

func staffRequest(t *testing.T, token string, orgID, storeID string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/"+orgID+"/stores/"+storeID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("orgID", orgID)
	if storeID != "" {
		req.SetPathValue("storeID", storeID)
	}
	return req
}

func TestRequireStaff_AllowsWithPermission(t *testing.T) {
	token, err := auth.IssueStaffAccessToken(testStaffSecret, 1, time.Minute)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}
	resolver := &fakeStaffResolver{principal: &service.StaffPrincipal{
		StaffUserID: 1,
		Memberships: []*domain.StaffMembership{{OrganizationID: 1, StoreID: nil, Role: domain.RoleRetailerAdmin, Status: domain.StaffActive}},
	}}

	handler := RequireStaff(testStaffSecret, resolver, domain.PermStoresRead, storeScopeFromPath)(okHandler())

	req := staffRequest(t, token, "1", "5")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// TestRequireStaff_IDOR_WrongOrgRejected verifies a staff token scoped to
// organization 1 cannot reach organization 2's resources — the core
// cross-tenant isolation guarantee for the staff/admin contour.
func TestRequireStaff_IDOR_WrongOrgRejected(t *testing.T) {
	token, err := auth.IssueStaffAccessToken(testStaffSecret, 1, time.Minute)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}
	resolver := &fakeStaffResolver{principal: &service.StaffPrincipal{
		StaffUserID: 1,
		Memberships: []*domain.StaffMembership{{OrganizationID: 1, StoreID: nil, Role: domain.RoleRetailerAdmin, Status: domain.StaffActive}},
	}}

	handler := RequireStaff(testStaffSecret, resolver, domain.PermStoresRead, storeScopeFromPath)(okHandler())

	req := staffRequest(t, token, "2", "5") // organization 2 — not this staff member's org
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (cross-organization access)", rec.Code)
	}
}

func TestRequireStaff_MissingPermissionRejected(t *testing.T) {
	token, err := auth.IssueStaffAccessToken(testStaffSecret, 1, time.Minute)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}
	resolver := &fakeStaffResolver{principal: &service.StaffPrincipal{
		StaffUserID: 1,
		Memberships: []*domain.StaffMembership{{OrganizationID: 1, StoreID: nil, Role: domain.RoleSupportViewer, Status: domain.StaffActive}},
	}}

	handler := RequireStaff(testStaffSecret, resolver, domain.PermLoyaltyConfigWrite, storeScopeFromPath)(okHandler())

	req := staffRequest(t, token, "1", "5")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (support_viewer lacks loyalty_config.write)", rec.Code)
	}
}

func TestRequireStaff_MissingTokenRejected(t *testing.T) {
	resolver := &fakeStaffResolver{}
	handler := RequireStaff(testStaffSecret, resolver, domain.PermStoresRead, storeScopeFromPath)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/1/stores/5", nil)
	req.SetPathValue("orgID", "1")
	req.SetPathValue("storeID", "5")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireStaff_DisabledAccountRejected(t *testing.T) {
	token, err := auth.IssueStaffAccessToken(testStaffSecret, 1, time.Minute)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}
	resolver := &fakeStaffResolver{err: service.ErrStaffDisabled}

	handler := RequireStaff(testStaffSecret, resolver, domain.PermStoresRead, storeScopeFromPath)(okHandler())

	req := staffRequest(t, token, "1", "5")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (disabled staff account)", rec.Code)
	}
}

// TestRequireGlobalStaffPermission_GrantsAcrossAnyOrg verifies the
// organization-creation bootstrap path: a platform_admin membership in any
// organization is sufficient, since there is no target organization yet.
func TestRequireGlobalStaffPermission_GrantsAcrossAnyOrg(t *testing.T) {
	token, err := auth.IssueStaffAccessToken(testStaffSecret, 1, time.Minute)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}
	resolver := &fakeStaffResolver{principal: &service.StaffPrincipal{
		StaffUserID: 1,
		Memberships: []*domain.StaffMembership{{OrganizationID: 1, StoreID: nil, Role: domain.RolePlatformAdmin, Status: domain.StaffActive}},
	}}

	handler := RequireGlobalStaffPermission(testStaffSecret, resolver, domain.PermOrganizationsManage)(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRequireGlobalStaffPermission_RejectsWithoutPermission(t *testing.T) {
	token, err := auth.IssueStaffAccessToken(testStaffSecret, 1, time.Minute)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}
	resolver := &fakeStaffResolver{principal: &service.StaffPrincipal{
		StaffUserID: 1,
		Memberships: []*domain.StaffMembership{{OrganizationID: 1, StoreID: nil, Role: domain.RoleStoreManager, Status: domain.StaffActive}},
	}}

	handler := RequireGlobalStaffPermission(testStaffSecret, resolver, domain.PermOrganizationsManage)(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/organizations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
