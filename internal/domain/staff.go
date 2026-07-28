package domain

import (
	"context"
	"time"
)

// StaffStatus is the lifecycle state of a StaffUser or StaffMembership.
type StaffStatus string

const (
	StaffActive   StaffStatus = "active"
	StaffDisabled StaffStatus = "disabled"
)

// StaffUser is a retailer employee or platform admin, authenticated via
// OIDC (internal/auth/oidc.go) — a separate identity from CustomerAccount
// and from the store API-key service principal (see knowledge/Decisions.md
// on the three-contour split). ExternalSubject is the OIDC provider's
// stable `sub` claim.
type StaffUser struct {
	ID              int64
	ExternalSubject string
	Email           string
	DisplayName     string
	Status          StaffStatus
	CreatedAt       time.Time
}

// StaffUserRepository persists and retrieves StaffUser rows.
type StaffUserRepository interface {
	// GetByExternalSubject returns domain.ErrNotFound if no staff user has
	// that OIDC subject yet.
	GetByExternalSubject(ctx context.Context, subject string) (*StaffUser, error)
	GetByID(ctx context.Context, id int64) (*StaffUser, error)
	// Create returns domain.ErrConflict if ExternalSubject is already
	// registered.
	Create(ctx context.Context, user *StaffUser) error
	// UpdateProfile refreshes Email/DisplayName from the latest OIDC claims
	// (called on every login so admin-visible staff lists stay current).
	UpdateProfile(ctx context.Context, id int64, email, displayName string) error
}

// StaffMembership grants StaffUser a Role within an Organization, optionally
// narrowed to one Store (StoreID nil means every store in the
// organization). Permission checks resolve a caller's permissions from
// active memberships matching the request's organization/store scope (see
// domain.RolePermissions, httpserver.RequireStaff).
type StaffMembership struct {
	ID             int64
	StaffUserID    int64
	OrganizationID int64
	StoreID        *int64
	Role           Role
	Status         StaffStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// StaffMembershipRepository persists and retrieves StaffMembership rows.
type StaffMembershipRepository interface {
	// ListByStaffUser returns every membership (any status) for staffUserID,
	// across all organizations.
	ListByStaffUser(ctx context.Context, staffUserID int64) ([]*StaffMembership, error)
	ListByOrganization(ctx context.Context, organizationID int64) ([]*StaffMembership, error)
	// Create returns domain.ErrConflict if this exact
	// (staff_user_id, organization_id, store_id) membership already exists.
	Create(ctx context.Context, membership *StaffMembership) error
	UpdateStatus(ctx context.Context, id int64, status StaffStatus) error
	UpdateRole(ctx context.Context, id int64, role Role) error
}
