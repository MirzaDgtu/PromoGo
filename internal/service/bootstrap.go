package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// BootstrapAdminInput describes the first platform_admin to create.
type BootstrapAdminInput struct {
	Subject     string // OIDC subject (sub claim)
	Email       string
	DisplayName string
	OrgName     string // used when OrgID == 0
	OrgID       int64  // takes precedence over OrgName when non-zero
}

// BootstrapAdminResult reports the resolved/created rows so a caller (the
// bootstrap-admin CLI) can record IDs for the operator without a manual SQL
// query.
type BootstrapAdminResult struct {
	Organization      *domain.Organization
	StaffUser         *domain.StaffUser
	Membership        *domain.StaffMembership
	UserCreated       bool
	MembershipCreated bool
}

// BootstrapPlatformAdmin creates (or reuses) a StaffUser for in.Subject and
// grants it an organization-wide platform_admin StaffMembership. This is
// the only path around the chicken-and-egg problem where every admin API
// endpoint able to grant a StaffMembership itself requires a caller who
// already holds staff.manage (see knowledge/Decisions.md DEC-009). It is
// idempotent — re-running with the same subject+organization is a safe
// no-op — reusing the ErrConflict-as-idempotent pattern from
// StaffAuthService.resolveOrCreateUser.
func BootstrapPlatformAdmin(
	ctx context.Context,
	orgs domain.OrganizationRepository,
	users domain.StaffUserRepository,
	memberships domain.StaffMembershipRepository,
	in BootstrapAdminInput,
) (*BootstrapAdminResult, error) {
	org, err := resolveOrganization(ctx, orgs, in)
	if err != nil {
		return nil, fmt.Errorf("bootstrap admin: resolve organization: %w", err)
	}

	user, userCreated, err := resolveOrCreateStaffUser(ctx, users, in)
	if err != nil {
		return nil, fmt.Errorf("bootstrap admin: resolve staff user: %w", err)
	}

	membership, membershipCreated, err := resolveOrCreatePlatformAdminMembership(ctx, memberships, user.ID, org.ID)
	if err != nil {
		return nil, fmt.Errorf("bootstrap admin: resolve membership: %w", err)
	}

	return &BootstrapAdminResult{
		Organization:      org,
		StaffUser:         user,
		Membership:        membership,
		UserCreated:       userCreated,
		MembershipCreated: membershipCreated,
	}, nil
}

func resolveOrganization(ctx context.Context, orgs domain.OrganizationRepository, in BootstrapAdminInput) (*domain.Organization, error) {
	if in.OrgID != 0 {
		return orgs.GetByID(ctx, in.OrgID)
	}
	return orgs.GetByName(ctx, in.OrgName)
}

func resolveOrCreateStaffUser(ctx context.Context, users domain.StaffUserRepository, in BootstrapAdminInput) (*domain.StaffUser, bool, error) {
	existing, err := users.GetByExternalSubject(ctx, in.Subject)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, false, err
	}

	user := &domain.StaffUser{
		ExternalSubject: in.Subject,
		Email:           in.Email,
		DisplayName:     in.DisplayName,
		Status:          domain.StaffActive,
	}
	if err := users.Create(ctx, user); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			// Lost a race with a concurrent bootstrap run.
			refetched, getErr := users.GetByExternalSubject(ctx, in.Subject)
			if getErr != nil {
				return nil, false, getErr
			}
			return refetched, false, nil
		}
		return nil, false, err
	}
	return user, true, nil
}

// resolveOrCreatePlatformAdminMembership finds or creates the org-wide
// (StoreID nil) StaffMembership for staffUserID in organizationID, promoting
// it to platform_admin if it already exists with a lesser role — the tool's
// entire purpose is guaranteeing a platform_admin exists, so silently
// succeeding while leaving the wrong role in place would defeat it.
func resolveOrCreatePlatformAdminMembership(ctx context.Context, memberships domain.StaffMembershipRepository, staffUserID, organizationID int64) (*domain.StaffMembership, bool, error) {
	existing, err := memberships.ListByStaffUser(ctx, staffUserID)
	if err != nil {
		return nil, false, err
	}
	for _, m := range existing {
		if m.OrganizationID != organizationID || m.StoreID != nil {
			continue
		}
		if m.Role != domain.RolePlatformAdmin {
			if err := memberships.UpdateRole(ctx, m.ID, domain.RolePlatformAdmin); err != nil {
				return nil, false, err
			}
			m.Role = domain.RolePlatformAdmin
		}
		return m, false, nil
	}

	membership := &domain.StaffMembership{
		StaffUserID:    staffUserID,
		OrganizationID: organizationID,
		StoreID:        nil,
		Role:           domain.RolePlatformAdmin,
		Status:         domain.StaffActive,
	}
	if err := memberships.Create(ctx, membership); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			// Lost a race with a concurrent bootstrap run; re-resolve.
			return resolveOrCreatePlatformAdminMembership(ctx, memberships, staffUserID, organizationID)
		}
		return nil, false, err
	}
	return membership, true, nil
}
