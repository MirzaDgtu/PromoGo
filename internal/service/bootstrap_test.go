package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// fakeOrganizationRepo is an in-memory domain.OrganizationRepository.
type fakeOrganizationRepo struct {
	mu     sync.Mutex
	byID   map[int64]*domain.Organization
	nextID int64
}

func newFakeOrganizationRepo() *fakeOrganizationRepo {
	return &fakeOrganizationRepo{byID: map[int64]*domain.Organization{}}
}

func (f *fakeOrganizationRepo) GetByID(_ context.Context, id int64) (*domain.Organization, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	org, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return org, nil
}

func (f *fakeOrganizationRepo) GetByName(_ context.Context, name string) (*domain.Organization, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, org := range f.byID {
		if org.Name == name {
			return org, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeOrganizationRepo) Create(_ context.Context, org *domain.Organization) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	org.ID = f.nextID
	org.CreatedAt = time.Now()
	f.byID[org.ID] = org
	return nil
}

func TestBootstrapPlatformAdmin_FreshCreateSucceeds(t *testing.T) {
	orgs := newFakeOrganizationRepo()
	_ = orgs.Create(context.Background(), &domain.Organization{Name: "Default Organization"})
	users := newFakeStaffUserRepo()
	memberships := newFakeStaffMembershipRepo()

	result, err := BootstrapPlatformAdmin(context.Background(), orgs, users, memberships, BootstrapAdminInput{
		Subject:     "auth0|admin",
		Email:       "admin@example.com",
		DisplayName: "Test Admin",
		OrgName:     "Default Organization",
	})
	if err != nil {
		t.Fatalf("BootstrapPlatformAdmin: %v", err)
	}
	if !result.UserCreated {
		t.Error("expected UserCreated = true")
	}
	if !result.MembershipCreated {
		t.Error("expected MembershipCreated = true")
	}
	if result.Membership.Role != domain.RolePlatformAdmin {
		t.Errorf("Membership.Role = %q, want %q", result.Membership.Role, domain.RolePlatformAdmin)
	}
	if result.Membership.StoreID != nil {
		t.Error("expected Membership.StoreID = nil (org-wide)")
	}
	if result.Membership.Status != domain.StaffActive {
		t.Errorf("Membership.Status = %q, want %q", result.Membership.Status, domain.StaffActive)
	}
}

func TestBootstrapPlatformAdmin_RerunIsIdempotent(t *testing.T) {
	orgs := newFakeOrganizationRepo()
	_ = orgs.Create(context.Background(), &domain.Organization{Name: "Default Organization"})
	users := newFakeStaffUserRepo()
	memberships := newFakeStaffMembershipRepo()

	in := BootstrapAdminInput{
		Subject:     "auth0|admin",
		Email:       "admin@example.com",
		DisplayName: "Test Admin",
		OrgName:     "Default Organization",
	}

	first, err := BootstrapPlatformAdmin(context.Background(), orgs, users, memberships, in)
	if err != nil {
		t.Fatalf("first BootstrapPlatformAdmin: %v", err)
	}

	second, err := BootstrapPlatformAdmin(context.Background(), orgs, users, memberships, in)
	if err != nil {
		t.Fatalf("second BootstrapPlatformAdmin: %v", err)
	}

	if second.UserCreated {
		t.Error("expected UserCreated = false on re-run")
	}
	if second.MembershipCreated {
		t.Error("expected MembershipCreated = false on re-run")
	}
	if second.StaffUser.ID != first.StaffUser.ID {
		t.Errorf("StaffUser.ID changed across re-run: %d -> %d", first.StaffUser.ID, second.StaffUser.ID)
	}
	if second.Membership.ID != first.Membership.ID {
		t.Errorf("Membership.ID changed across re-run: %d -> %d", first.Membership.ID, second.Membership.ID)
	}
	if len(memberships.byID) != 1 {
		t.Errorf("len(memberships.byID) = %d, want 1", len(memberships.byID))
	}
}

func TestBootstrapPlatformAdmin_UnknownOrgNameReturnsError(t *testing.T) {
	orgs := newFakeOrganizationRepo()
	users := newFakeStaffUserRepo()
	memberships := newFakeStaffMembershipRepo()

	_, err := BootstrapPlatformAdmin(context.Background(), orgs, users, memberships, BootstrapAdminInput{
		Subject:     "auth0|admin",
		Email:       "admin@example.com",
		DisplayName: "Test Admin",
		OrgName:     "Nonexistent Org",
	})
	if err == nil {
		t.Fatal("expected error for unknown org name, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected error wrapping domain.ErrNotFound, got: %v", err)
	}
}

func TestBootstrapPlatformAdmin_ByOrgID(t *testing.T) {
	orgs := newFakeOrganizationRepo()
	_ = orgs.Create(context.Background(), &domain.Organization{Name: "Some Org"})
	users := newFakeStaffUserRepo()
	memberships := newFakeStaffMembershipRepo()

	result, err := BootstrapPlatformAdmin(context.Background(), orgs, users, memberships, BootstrapAdminInput{
		Subject:     "auth0|admin",
		Email:       "admin@example.com",
		DisplayName: "Test Admin",
		OrgID:       1,
	})
	if err != nil {
		t.Fatalf("BootstrapPlatformAdmin: %v", err)
	}
	if result.Organization.ID != 1 {
		t.Errorf("Organization.ID = %d, want 1", result.Organization.ID)
	}
}

func TestBootstrapPlatformAdmin_UpgradesExistingLesserRoleMembership(t *testing.T) {
	orgs := newFakeOrganizationRepo()
	_ = orgs.Create(context.Background(), &domain.Organization{Name: "Default Organization"})
	users := newFakeStaffUserRepo()
	memberships := newFakeStaffMembershipRepo()

	user := &domain.StaffUser{ExternalSubject: "auth0|admin", Email: "admin@example.com", DisplayName: "Test Admin", Status: domain.StaffActive}
	if err := users.Create(context.Background(), user); err != nil {
		t.Fatalf("seed staff user: %v", err)
	}
	existing := &domain.StaffMembership{
		StaffUserID:    user.ID,
		OrganizationID: 1,
		StoreID:        nil,
		Role:           domain.RoleStoreManager,
		Status:         domain.StaffActive,
	}
	if err := memberships.Create(context.Background(), existing); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	result, err := BootstrapPlatformAdmin(context.Background(), orgs, users, memberships, BootstrapAdminInput{
		Subject:     "auth0|admin",
		Email:       "admin@example.com",
		DisplayName: "Test Admin",
		OrgName:     "Default Organization",
	})
	if err != nil {
		t.Fatalf("BootstrapPlatformAdmin: %v", err)
	}
	if result.MembershipCreated {
		t.Error("expected MembershipCreated = false (promoted existing membership)")
	}
	if result.Membership.ID != existing.ID {
		t.Errorf("Membership.ID = %d, want existing ID %d", result.Membership.ID, existing.ID)
	}
	if result.Membership.Role != domain.RolePlatformAdmin {
		t.Errorf("Membership.Role = %q, want %q", result.Membership.Role, domain.RolePlatformAdmin)
	}
	if len(memberships.byID) != 1 {
		t.Errorf("len(memberships.byID) = %d, want 1", len(memberships.byID))
	}
}
