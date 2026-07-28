package domain

import (
	"context"
	"time"
)

// AuditActorType distinguishes which of PromoGo's three auth contours
// produced an AuditEvent.
type AuditActorType string

const (
	AuditActorCustomer    AuditActorType = "customer"
	AuditActorStaff       AuditActorType = "staff"
	AuditActorStoreAPIKey AuditActorType = "store_api_key"
	AuditActorSystem      AuditActorType = "system"
)

// Audit actions recorded by this change. Kept as a small fixed set so
// audit-log consumers can rely on exact string matching rather than free text.
const (
	AuditActionCustomerOTPRequested = "customer.otp_requested"
	AuditActionCustomerOTPFailed    = "customer.otp_failed"
	AuditActionCustomerLogin        = "customer.login"
	AuditActionCustomerLogout       = "customer.logout"
	AuditActionCustomerLogoutAll    = "customer.logout_all"
	AuditActionCustomerRefreshReuse = "customer.refresh_reuse_detected"

	AuditActionStaffLogin           = "staff.login"
	AuditActionStaffMembershipAdded = "staff.membership_added"
	AuditActionStaffRoleChanged     = "staff.role_changed"
	AuditActionStaffStatusChanged   = "staff.status_changed"

	AuditActionAPIKeyCreated = "api_key.created"
	AuditActionAPIKeyRotated = "api_key.rotated"
	AuditActionAPIKeyRevoked = "api_key.revoked"
)

// AuditEvent is one append-only record in the security audit trail. Never
// updated or deleted.
type AuditEvent struct {
	ID             int64
	OccurredAt     time.Time
	ActorType      AuditActorType
	ActorID        *int64
	OrganizationID *int64
	StoreID        *int64
	Action         string
	TargetType     string
	TargetID       *int64
	Metadata       map[string]any
	IP             string
	UserAgent      string
}

// AuditEventRepository appends and lists AuditEvent rows.
type AuditEventRepository interface {
	Create(ctx context.Context, event *AuditEvent) error
	// ListByOrganization returns the most recent events for organizationID,
	// newest first, capped at limit.
	ListByOrganization(ctx context.Context, organizationID int64, limit int) ([]*AuditEvent, error)
}
