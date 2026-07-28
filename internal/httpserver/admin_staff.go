package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

const maxExternalSubjectLen = 255

type staffMembershipResponseBody struct {
	ID             int64     `json:"id"`
	StaffUserID    int64     `json:"staff_user_id"`
	OrganizationID int64     `json:"organization_id"`
	StoreID        *int64    `json:"store_id,omitempty"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// handleListStaffMemberships returns a handler for GET
// /api/v1/admin/organizations/{orgID}/staff. Must run behind
// RequireStaff(staff.manage, orgScopeFromPath).
func handleListStaffMemberships(memberships domain.StaffMembershipRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		list, err := memberships.ListByOrganization(r.Context(), orgID)
		if err != nil {
			log.ErrorContext(r.Context(), "list staff memberships", "organization_id", orgID, "error", err)
			writeError(w, http.StatusInternalServerError, "list staff memberships")
			return
		}

		out := make([]staffMembershipResponseBody, 0, len(list))
		for _, m := range list {
			out = append(out, membershipToBody(m))
		}
		writeJSON(w, http.StatusOK, map[string]any{"memberships": out})
	}
}

func membershipToBody(m *domain.StaffMembership) staffMembershipResponseBody {
	return staffMembershipResponseBody{
		ID: m.ID, StaffUserID: m.StaffUserID, OrganizationID: m.OrganizationID, StoreID: m.StoreID,
		Role: string(m.Role), Status: string(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type createStaffMembershipBody struct {
	// ExternalSubject is the target staff member's OIDC subject. A
	// StaffUser row is created for it if one doesn't exist yet — the rest
	// of their profile (email/display name) fills in on their first actual
	// OIDC login (see service.StaffAuthService.resolveOrCreateUser).
	ExternalSubject string `json:"external_subject"`
	StoreID         *int64 `json:"store_id,omitempty"`
	Role            string `json:"role"`
}

// handleCreateStaffMembership returns a handler for POST
// /api/v1/admin/organizations/{orgID}/staff. Must run behind
// RequireStaff(staff.manage, orgScopeFromPath).
func handleCreateStaffMembership(users domain.StaffUserRepository, memberships domain.StaffMembershipRepository, audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body createStaffMembershipBody
		if err := dec.Decode(&body); err != nil ||
			body.ExternalSubject == "" || len(body.ExternalSubject) > maxExternalSubjectLen ||
			!validRole(body.Role) {
			writeError(w, http.StatusBadRequest, "external_subject and a valid role are required")
			return
		}

		user, err := users.GetByExternalSubject(r.Context(), body.ExternalSubject)
		if errors.Is(err, domain.ErrNotFound) {
			user = &domain.StaffUser{ExternalSubject: body.ExternalSubject, Status: domain.StaffActive}
			if createErr := users.Create(r.Context(), user); createErr != nil {
				log.ErrorContext(r.Context(), "create staff user", "error", createErr)
				writeError(w, http.StatusInternalServerError, "create staff user")
				return
			}
		} else if err != nil {
			log.ErrorContext(r.Context(), "load staff user", "error", err)
			writeError(w, http.StatusInternalServerError, "load staff user")
			return
		}

		membership := &domain.StaffMembership{
			StaffUserID: user.ID, OrganizationID: orgID, StoreID: body.StoreID,
			Role: domain.Role(body.Role), Status: domain.StaffActive,
		}
		if err := memberships.Create(r.Context(), membership); err != nil {
			if errors.Is(err, domain.ErrConflict) {
				writeError(w, http.StatusConflict, "membership already exists")
				return
			}
			log.ErrorContext(r.Context(), "create staff membership", "error", err)
			writeError(w, http.StatusInternalServerError, "create staff membership")
			return
		}

		principal, _ := staffFromContext(r.Context())
		var actorID *int64
		if principal != nil {
			id := principal.StaffUserID
			actorID = &id
		}
		auditCreate(r.Context(), audit, log, domain.AuditActorStaff, actorID, domain.AuditActionStaffMembershipAdded, &orgID, nil, "staff_membership", &membership.ID, clientIP(r), r.UserAgent())

		writeJSON(w, http.StatusCreated, membershipToBody(membership))
	}
}

func validRole(role string) bool {
	switch domain.Role(role) {
	case domain.RolePlatformAdmin, domain.RoleRetailerAdmin, domain.RoleStoreManager, domain.RoleSupportViewer:
		return true
	default:
		return false
	}
}

type updateStaffMembershipBody struct {
	Role   string `json:"role,omitempty"`
	Status string `json:"status,omitempty"`
}

// handleUpdateStaffMembership returns a handler for PATCH
// /api/v1/admin/organizations/{orgID}/staff/{membershipID}. Must run behind
// RequireStaff(staff.manage, orgScopeFromPath).
func handleUpdateStaffMembership(memberships domain.StaffMembershipRepository, audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization id")
			return
		}
		membershipID, err := strconv.ParseInt(r.PathValue("membershipID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid membership id")
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body updateStaffMembershipBody
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Role == "" && body.Status == "" {
			writeError(w, http.StatusBadRequest, "role or status is required")
			return
		}

		principal, _ := staffFromContext(r.Context())
		var actorID *int64
		if principal != nil {
			id := principal.StaffUserID
			actorID = &id
		}

		if body.Role != "" {
			if !validRole(body.Role) {
				writeError(w, http.StatusBadRequest, "invalid role")
				return
			}
			if err := memberships.UpdateRole(r.Context(), membershipID, domain.Role(body.Role)); err != nil {
				log.ErrorContext(r.Context(), "update staff membership role", "membership_id", membershipID, "error", err)
				writeError(w, http.StatusInternalServerError, "update membership")
				return
			}
			auditCreate(r.Context(), audit, log, domain.AuditActorStaff, actorID, domain.AuditActionStaffRoleChanged, &orgID, nil, "staff_membership", &membershipID, clientIP(r), r.UserAgent())
		}

		if body.Status != "" {
			if body.Status != string(domain.StaffActive) && body.Status != string(domain.StaffDisabled) {
				writeError(w, http.StatusBadRequest, "invalid status")
				return
			}
			if err := memberships.UpdateStatus(r.Context(), membershipID, domain.StaffStatus(body.Status)); err != nil {
				log.ErrorContext(r.Context(), "update staff membership status", "membership_id", membershipID, "error", err)
				writeError(w, http.StatusInternalServerError, "update membership")
				return
			}
			auditCreate(r.Context(), audit, log, domain.AuditActorStaff, actorID, domain.AuditActionStaffStatusChanged, &orgID, nil, "staff_membership", &membershipID, clientIP(r), r.UserAgent())
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
