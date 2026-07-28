package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// fakeStaffUserRepo is an in-memory domain.StaffUserRepository.
type fakeStaffUserRepo struct {
	mu     sync.Mutex
	byID   map[int64]*domain.StaffUser
	nextID int64
}

func newFakeStaffUserRepo() *fakeStaffUserRepo {
	return &fakeStaffUserRepo{byID: map[int64]*domain.StaffUser{}}
}

func (f *fakeStaffUserRepo) GetByExternalSubject(_ context.Context, subject string) (*domain.StaffUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.byID {
		if u.ExternalSubject == subject {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeStaffUserRepo) GetByID(_ context.Context, id int64) (*domain.StaffUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeStaffUserRepo) Create(_ context.Context, u *domain.StaffUser) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.byID {
		if existing.ExternalSubject == u.ExternalSubject {
			return domain.ErrConflict
		}
	}
	f.nextID++
	u.ID = f.nextID
	u.CreatedAt = time.Now()
	f.byID[u.ID] = u
	return nil
}

func (f *fakeStaffUserRepo) UpdateProfile(_ context.Context, id int64, email, displayName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	u.Email = email
	u.DisplayName = displayName
	return nil
}

// fakeStaffMembershipRepo is an in-memory domain.StaffMembershipRepository.
type fakeStaffMembershipRepo struct {
	mu     sync.Mutex
	byID   map[int64]*domain.StaffMembership
	nextID int64
}

func newFakeStaffMembershipRepo() *fakeStaffMembershipRepo {
	return &fakeStaffMembershipRepo{byID: map[int64]*domain.StaffMembership{}}
}

func (f *fakeStaffMembershipRepo) ListByStaffUser(_ context.Context, staffUserID int64) ([]*domain.StaffMembership, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.StaffMembership
	for _, m := range f.byID {
		if m.StaffUserID == staffUserID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeStaffMembershipRepo) ListByOrganization(_ context.Context, orgID int64) ([]*domain.StaffMembership, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.StaffMembership
	for _, m := range f.byID {
		if m.OrganizationID == orgID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeStaffMembershipRepo) Create(_ context.Context, m *domain.StaffMembership) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	m.ID = f.nextID
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	f.byID[m.ID] = m
	return nil
}

func (f *fakeStaffMembershipRepo) UpdateStatus(_ context.Context, id int64, status domain.StaffStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	m.Status = status
	m.UpdatedAt = time.Now()
	return nil
}

func (f *fakeStaffMembershipRepo) UpdateRole(_ context.Context, id int64, role domain.Role) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	m.Role = role
	m.UpdatedAt = time.Now()
	return nil
}

const (
	testStaffIssuer   = "https://idp.example.test/"
	testStaffAudience = "promogo-staff"
	testStaffKid      = "staff-test-key"
)

func startStaffTestJWKSServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()

	jwk := struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	}{
		Kty: "RSA", Kid: testStaffKid,
		N: base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}), // 65537
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func signStaffTestIDToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = testStaffKid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign test id token: %v", err)
	}
	return signed
}

type staffAuthTestDeps struct {
	svc         *StaffAuthService
	users       *fakeStaffUserRepo
	memberships *fakeStaffMembershipRepo
	audit       *fakeAuditEventRepo
	key         *rsa.PrivateKey
}

func newStaffAuthTestDeps(t *testing.T) *staffAuthTestDeps {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	srv := startStaffTestJWKSServer(t, key)
	verifier := auth.NewOIDCVerifier(testStaffIssuer, testStaffAudience, srv.URL, time.Minute)

	users := newFakeStaffUserRepo()
	memberships := newFakeStaffMembershipRepo()
	audit := &fakeAuditEventRepo{}

	svc := NewStaffAuthService(discardLogger(), users, memberships, audit, verifier,
		StaffAuthConfig{AccessTokenSecret: []byte("test-secret-at-least-32-bytes-long!"), AccessTokenTTL: time.Minute})

	return &staffAuthTestDeps{svc: svc, users: users, memberships: memberships, audit: audit, key: key}
}

func (d *staffAuthTestDeps) validIDToken(t *testing.T, subject, email, name string) string {
	t.Helper()
	now := time.Now()
	return signStaffTestIDToken(t, d.key, jwt.MapClaims{
		"iss": testStaffIssuer, "aud": testStaffAudience, "sub": subject,
		"email": email, "name": name,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
}

func TestStaffAuth_FirstLoginHasNoMembershipYet(t *testing.T) {
	deps := newStaffAuthTestDeps(t)
	ctx := context.Background()

	_, _, err := deps.svc.AuthenticateOIDC(ctx, deps.validIDToken(t, "sub-1", "a@example.test", "Alice"), "1.2.3.4", "test-agent")
	if !errors.Is(err, ErrNoActiveMembership) {
		t.Fatalf("AuthenticateOIDC() first login error = %v, want ErrNoActiveMembership", err)
	}

	// The StaffUser row must exist now even though login was refused.
	user, err := deps.users.GetByExternalSubject(ctx, "sub-1")
	if err != nil {
		t.Fatalf("GetByExternalSubject() error = %v", err)
	}
	if user.Email != "a@example.test" {
		t.Errorf("user.Email = %q, want %q", user.Email, "a@example.test")
	}
}

func TestStaffAuth_LoginSucceedsWithActiveMembership(t *testing.T) {
	deps := newStaffAuthTestDeps(t)
	ctx := context.Background()

	// Bootstrap: first login creates the StaffUser (and is refused).
	_, _, _ = deps.svc.AuthenticateOIDC(ctx, deps.validIDToken(t, "sub-2", "b@example.test", "Bob"), "1.2.3.4", "")
	user, err := deps.users.GetByExternalSubject(ctx, "sub-2")
	if err != nil {
		t.Fatalf("GetByExternalSubject() error = %v", err)
	}

	if err := deps.memberships.Create(ctx, &domain.StaffMembership{
		StaffUserID: user.ID, OrganizationID: 1, Role: domain.RoleRetailerAdmin, Status: domain.StaffActive,
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	token, principal, err := deps.svc.AuthenticateOIDC(ctx, deps.validIDToken(t, "sub-2", "b@example.test", "Bob"), "1.2.3.4", "")
	if err != nil {
		t.Fatalf("AuthenticateOIDC() error = %v", err)
	}
	if token == "" {
		t.Error("AuthenticateOIDC() returned empty token")
	}
	if len(principal.Memberships) != 1 || principal.Memberships[0].Role != domain.RoleRetailerAdmin {
		t.Errorf("principal.Memberships = %+v, want one retailer_admin membership", principal.Memberships)
	}

	parsedID, err := auth.ParseStaffAccessToken([]byte("test-secret-at-least-32-bytes-long!"), token)
	if err != nil {
		t.Fatalf("ParseStaffAccessToken() error = %v", err)
	}
	if parsedID != user.ID {
		t.Errorf("token subject = %d, want %d", parsedID, user.ID)
	}
}

func TestStaffAuth_DisabledAccountRejected(t *testing.T) {
	deps := newStaffAuthTestDeps(t)
	ctx := context.Background()

	if err := deps.users.Create(ctx, &domain.StaffUser{ExternalSubject: "sub-3", Status: domain.StaffDisabled}); err != nil {
		t.Fatalf("seed disabled user: %v", err)
	}

	_, _, err := deps.svc.AuthenticateOIDC(ctx, deps.validIDToken(t, "sub-3", "c@example.test", "Carol"), "1.2.3.4", "")
	if !errors.Is(err, ErrStaffDisabled) {
		t.Fatalf("AuthenticateOIDC() for disabled account error = %v, want ErrStaffDisabled", err)
	}
}

func TestStaffAuth_DisabledMembershipExcludedFromPrincipal(t *testing.T) {
	deps := newStaffAuthTestDeps(t)
	ctx := context.Background()

	if err := deps.users.Create(ctx, &domain.StaffUser{ExternalSubject: "sub-4", Status: domain.StaffActive}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	user, _ := deps.users.GetByExternalSubject(ctx, "sub-4")

	_ = deps.memberships.Create(ctx, &domain.StaffMembership{StaffUserID: user.ID, OrganizationID: 1, Role: domain.RoleStoreManager, Status: domain.StaffDisabled})
	_ = deps.memberships.Create(ctx, &domain.StaffMembership{StaffUserID: user.ID, OrganizationID: 2, Role: domain.RoleSupportViewer, Status: domain.StaffActive})

	principal, err := deps.svc.ResolvePrincipal(ctx, user.ID)
	if err != nil {
		t.Fatalf("ResolvePrincipal() error = %v", err)
	}
	if len(principal.Memberships) != 1 || principal.Memberships[0].OrganizationID != 2 {
		t.Errorf("principal.Memberships = %+v, want only the active org=2 membership", principal.Memberships)
	}
}

func TestStaffAuth_OIDCInvalidTokenRejectedBeforeTouchingRepos(t *testing.T) {
	deps := newStaffAuthTestDeps(t)
	ctx := context.Background()

	now := time.Now()
	badToken := signStaffTestIDToken(t, deps.key, jwt.MapClaims{
		"iss": "https://attacker.example/", "aud": testStaffAudience, "sub": "sub-5",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})

	_, _, err := deps.svc.AuthenticateOIDC(ctx, badToken, "1.2.3.4", "")
	if err == nil {
		t.Fatal("AuthenticateOIDC() with wrong-issuer token succeeded, want error")
	}
	if _, lookupErr := deps.users.GetByExternalSubject(ctx, "sub-5"); !errors.Is(lookupErr, domain.ErrNotFound) {
		t.Error("a StaffUser row was created despite OIDC verification failing")
	}
}

// TestStaffPrincipal_HasPermission_ScopeRules exercises every MVP role
// against org-wide vs store-scoped membership boundaries.
func TestStaffPrincipal_HasPermission_ScopeRules(t *testing.T) {
	store5 := int64(5)
	store6 := int64(6)

	orgWideAdmin := &StaffPrincipal{Memberships: []*domain.StaffMembership{
		{OrganizationID: 1, StoreID: nil, Role: domain.RoleRetailerAdmin, Status: domain.StaffActive},
	}}
	storeScopedManager := &StaffPrincipal{Memberships: []*domain.StaffMembership{
		{OrganizationID: 1, StoreID: &store5, Role: domain.RoleStoreManager, Status: domain.StaffActive},
	}}
	platformAdmin := &StaffPrincipal{Memberships: []*domain.StaffMembership{
		{OrganizationID: 1, StoreID: nil, Role: domain.RolePlatformAdmin, Status: domain.StaffActive},
	}}
	supportViewer := &StaffPrincipal{Memberships: []*domain.StaffMembership{
		{OrganizationID: 1, StoreID: nil, Role: domain.RoleSupportViewer, Status: domain.StaffActive},
	}}

	// Org-wide membership grants a store-scoped permission for any store in the org.
	if !orgWideAdmin.HasPermission(domain.PermStoresRead, 1, &store5) {
		t.Error("org-wide retailer_admin should have stores.read for any store in org 1")
	}
	// Org-wide membership grants org-level permissions.
	if !orgWideAdmin.HasPermission(domain.PermStaffManage, 1, nil) {
		t.Error("org-wide retailer_admin should have staff.manage at org level")
	}
	// Wrong organization is never granted.
	if orgWideAdmin.HasPermission(domain.PermStoresRead, 2, &store5) {
		t.Error("retailer_admin of org 1 should not have permissions in org 2")
	}

	// Store-scoped membership grants store-level permissions only for its own store.
	if !storeScopedManager.HasPermission(domain.PermLoyaltyConfigWrite, 1, &store5) {
		t.Error("store-scoped store_manager should have loyalty_config.write for its own store")
	}
	if storeScopedManager.HasPermission(domain.PermLoyaltyConfigWrite, 1, &store6) {
		t.Error("store-scoped store_manager should NOT have loyalty_config.write for a different store")
	}
	// Store-scoped membership never grants an org-level permission.
	if storeScopedManager.HasPermission(domain.PermStaffManage, 1, nil) {
		t.Error("store_manager (who never has staff.manage per RolePermissions) should not have it at org level")
	}
	if storeScopedManager.HasPermission(domain.PermStoresManage, 1, nil) {
		t.Error("store-scoped membership should not grant an org-level permission even if the role has it")
	}

	// platform_admin has every permission.
	for _, perm := range []domain.Permission{
		domain.PermOrganizationsManage, domain.PermStoresManage, domain.PermStaffManage,
		domain.PermAPIKeysRotate, domain.PermAuditRead,
	} {
		if !platformAdmin.HasPermission(perm, 1, nil) {
			t.Errorf("platform_admin should have permission %q", perm)
		}
	}

	// support_viewer never has a write permission.
	if supportViewer.HasPermission(domain.PermLoyaltyConfigWrite, 1, &store5) {
		t.Error("support_viewer should never have loyalty_config.write")
	}
	if !supportViewer.HasPermission(domain.PermClientsRead, 1, &store5) {
		t.Error("support_viewer should have clients.read")
	}
}

func TestStaffPrincipal_IsSupportViewerOnly(t *testing.T) {
	store5 := int64(5)

	viewerOnly := &StaffPrincipal{Memberships: []*domain.StaffMembership{
		{OrganizationID: 1, StoreID: nil, Role: domain.RoleSupportViewer, Status: domain.StaffActive},
	}}
	if !viewerOnly.IsSupportViewerOnly(1, &store5) {
		t.Error("IsSupportViewerOnly() = false for a support_viewer-only principal, want true")
	}

	mixed := &StaffPrincipal{Memberships: []*domain.StaffMembership{
		{OrganizationID: 1, StoreID: nil, Role: domain.RoleSupportViewer, Status: domain.StaffActive},
		{OrganizationID: 1, StoreID: &store5, Role: domain.RoleStoreManager, Status: domain.StaffActive},
	}}
	if mixed.IsSupportViewerOnly(1, &store5) {
		t.Error("IsSupportViewerOnly() = true when a non-support_viewer membership also matches, want false")
	}

	noMatch := &StaffPrincipal{Memberships: []*domain.StaffMembership{
		{OrganizationID: 2, StoreID: nil, Role: domain.RoleSupportViewer, Status: domain.StaffActive},
	}}
	if noMatch.IsSupportViewerOnly(1, &store5) {
		t.Error("IsSupportViewerOnly() = true for an unrelated organization, want false")
	}
}
