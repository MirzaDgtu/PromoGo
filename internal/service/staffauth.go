package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// Errors returned by StaffAuthService.
var (
	ErrStaffDisabled      = errors.New("staff user disabled")
	ErrNoActiveMembership = errors.New("no active staff membership")
)

// StaffPrincipal is a verified staff caller and their active memberships,
// resolved fresh from the database on every request (see
// httpserver.RequireStaff) — never cached in the access token — so a
// disabled account or membership takes effect immediately instead of only
// after the short-lived token expires.
type StaffPrincipal struct {
	StaffUserID int64
	Memberships []*domain.StaffMembership
}

// HasPermission reports whether the principal holds perm within
// organizationID, and (if storeID is non-nil) specifically for that store.
// An organization-wide membership (StoreID nil) grants store-scoped
// permissions across every store in the organization; a store-scoped
// membership never grants an organization-level permission (storeID nil)
// outside its own store.
func (p *StaffPrincipal) HasPermission(perm domain.Permission, organizationID int64, storeID *int64) bool {
	for _, m := range p.Memberships {
		if m.OrganizationID != organizationID || !domain.HasPermission(m.Role, perm) {
			continue
		}
		if storeID == nil {
			if m.StoreID == nil {
				return true
			}
			continue
		}
		if m.StoreID == nil || *m.StoreID == *storeID {
			return true
		}
	}
	return false
}

// IsSupportViewerOnly reports whether every membership matching
// (organizationID, storeID) is RoleSupportViewer — used by admin handlers
// to decide whether to mask PII in a response (see httpserver.maskPII).
// Returns false if no membership matches at all (callers only reach here
// after RequireStaff already confirmed at least read access).
func (p *StaffPrincipal) IsSupportViewerOnly(organizationID int64, storeID *int64) bool {
	matched := false
	for _, m := range p.Memberships {
		if m.OrganizationID != organizationID {
			continue
		}
		if storeID != nil && m.StoreID != nil && *m.StoreID != *storeID {
			continue
		}
		matched = true
		if m.Role != domain.RoleSupportViewer {
			return false
		}
	}
	return matched
}

// StaffAuthConfig configures StaffAuthService's own access token issuance
// (distinct from the upstream OIDC provider's tokens — see AuthenticateOIDC).
type StaffAuthConfig struct {
	AccessTokenSecret []byte
	AccessTokenTTL    time.Duration
}

// StaffAuthService authenticates retailer staff / platform admins via OIDC
// and issues PromoGo's own short-lived staff access token, so staff don't
// resend the IdP's ID token on every API call. See knowledge/Decisions.md
// for why staff identity is never shared with CustomerAccount or store API
// keys.
type StaffAuthService struct {
	log         *slog.Logger
	users       domain.StaffUserRepository
	memberships domain.StaffMembershipRepository
	audit       domain.AuditEventRepository
	oidc        *auth.OIDCVerifier
	cfg         StaffAuthConfig
}

// NewStaffAuthService constructs a StaffAuthService.
func NewStaffAuthService(
	log *slog.Logger,
	users domain.StaffUserRepository,
	memberships domain.StaffMembershipRepository,
	audit domain.AuditEventRepository,
	oidc *auth.OIDCVerifier,
	cfg StaffAuthConfig,
) *StaffAuthService {
	return &StaffAuthService{log: log, users: users, memberships: memberships, audit: audit, oidc: oidc, cfg: cfg}
}

// AuthenticateOIDC verifies rawIDToken against the configured OIDC issuer,
// upserts the StaffUser by its OIDC subject, and issues a PromoGo staff
// access token. Returns ErrStaffDisabled if the account is disabled, or
// ErrNoActiveMembership if the account has no active StaffMembership yet
// (an operator must grant one via the staff.manage admin API — see
// knowledge/Decisions.md on platform_admin bootstrap).
func (s *StaffAuthService) AuthenticateOIDC(ctx context.Context, rawIDToken, ip, userAgent string) (string, *StaffPrincipal, error) {
	claims, err := s.oidc.VerifyIDToken(ctx, rawIDToken)
	if err != nil {
		return "", nil, fmt.Errorf("authenticate: %w", err)
	}

	user, err := s.resolveOrCreateUser(ctx, claims)
	if err != nil {
		return "", nil, fmt.Errorf("authenticate: resolve staff user: %w", err)
	}
	if user.Status != domain.StaffActive {
		return "", nil, ErrStaffDisabled
	}

	principal, err := s.loadPrincipal(ctx, user.ID)
	if err != nil {
		return "", nil, fmt.Errorf("authenticate: load memberships: %w", err)
	}
	if len(principal.Memberships) == 0 {
		return "", nil, ErrNoActiveMembership
	}

	token, err := auth.IssueStaffAccessToken(s.cfg.AccessTokenSecret, user.ID, s.cfg.AccessTokenTTL)
	if err != nil {
		return "", nil, fmt.Errorf("authenticate: issue token: %w", err)
	}

	userID := user.ID
	s.auditLog(ctx, &userID, domain.AuditActionStaffLogin, ip, userAgent)
	return token, principal, nil
}

// ResolvePrincipal loads the current StaffUser and active memberships for
// staffUserID — the identity carried by a verified staff access token. Used
// by httpserver.RequireStaff on every request, never cached, so a
// membership change or account disablement is enforced immediately.
func (s *StaffAuthService) ResolvePrincipal(ctx context.Context, staffUserID int64) (*StaffPrincipal, error) {
	user, err := s.users.GetByID(ctx, staffUserID)
	if err != nil {
		return nil, fmt.Errorf("resolve principal: %w", err)
	}
	if user.Status != domain.StaffActive {
		return nil, ErrStaffDisabled
	}
	return s.loadPrincipal(ctx, staffUserID)
}

func (s *StaffAuthService) loadPrincipal(ctx context.Context, staffUserID int64) (*StaffPrincipal, error) {
	all, err := s.memberships.ListByStaffUser(ctx, staffUserID)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}

	var active []*domain.StaffMembership
	for _, m := range all {
		if m.Status == domain.StaffActive {
			active = append(active, m)
		}
	}
	return &StaffPrincipal{StaffUserID: staffUserID, Memberships: active}, nil
}

func (s *StaffAuthService) resolveOrCreateUser(ctx context.Context, claims *auth.OIDCClaims) (*domain.StaffUser, error) {
	user, err := s.users.GetByExternalSubject(ctx, claims.Subject)
	if err == nil {
		if updErr := s.users.UpdateProfile(ctx, user.ID, claims.Email, claims.Name); updErr != nil {
			s.log.WarnContext(ctx, "update staff profile", "staff_user_id", user.ID, "error", updErr)
		} else {
			user.Email = claims.Email
			user.DisplayName = claims.Name
		}
		return user, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	user = &domain.StaffUser{
		ExternalSubject: claims.Subject,
		Email:           claims.Email,
		DisplayName:     claims.Name,
		Status:          domain.StaffActive,
	}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			// Lost a race with a concurrent first login of the same subject.
			return s.users.GetByExternalSubject(ctx, claims.Subject)
		}
		return nil, err
	}
	return user, nil
}

func (s *StaffAuthService) auditLog(ctx context.Context, staffUserID *int64, action, ip, userAgent string) {
	event := &domain.AuditEvent{
		ActorType: domain.AuditActorStaff,
		ActorID:   staffUserID,
		Action:    action,
		IP:        ip,
		UserAgent: userAgent,
	}
	if err := s.audit.Create(ctx, event); err != nil {
		s.log.WarnContext(ctx, "write audit event", "action", action, "error", err)
	}
}
