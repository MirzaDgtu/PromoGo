package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// staffMeMembershipBody is one active membership in the getStaffMe response,
// including its resolved permission set so the frontend can build
// permission-aware navigation without re-deriving domain.RolePermissions
// itself.
type staffMeMembershipBody struct {
	OrganizationID int64    `json:"organization_id"`
	StoreID        *int64   `json:"store_id,omitempty"`
	Role           string   `json:"role"`
	Permissions    []string `json:"permissions"`
}

type staffMeResponseBody struct {
	StaffUserID int64                   `json:"staff_user_id"`
	Email       string                  `json:"email"`
	DisplayName string                  `json:"display_name"`
	Memberships []staffMeMembershipBody `json:"memberships"`
}

// handleGetStaffMe returns a handler for GET /api/v1/staff/me. Must run
// behind RequireStaffIdentity — this is the caller's own profile, so there
// is no organization/store to scope a permission check to; an account with
// zero active memberships still gets a 200 with an empty memberships list
// (the frontend's "no membership" state), not a 403.
func handleGetStaffMe(users domain.StaffUserRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := staffFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		user, err := users.GetByID(r.Context(), principal.StaffUserID)
		if err != nil {
			log.ErrorContext(r.Context(), "load staff user", "staff_user_id", principal.StaffUserID, "error", err)
			writeError(w, http.StatusInternalServerError, "load staff user")
			return
		}

		memberships := make([]staffMeMembershipBody, 0, len(principal.Memberships))
		for _, m := range principal.Memberships {
			perms := domain.RolePermissions[m.Role]
			permStrs := make([]string, 0, len(perms))
			for _, p := range perms {
				permStrs = append(permStrs, string(p))
			}
			memberships = append(memberships, staffMeMembershipBody{
				OrganizationID: m.OrganizationID,
				StoreID:        m.StoreID,
				Role:           string(m.Role),
				Permissions:    permStrs,
			})
		}

		writeJSON(w, http.StatusOK, staffMeResponseBody{
			StaffUserID: user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Memberships: memberships,
		})
	}
}

// handleListOrganizations returns a handler for GET
// /api/v1/admin/organizations. Must run behind RequireStaffIdentity: a
// platform_admin membership (anywhere) sees every organization; any other
// caller sees only the organizations of their own active memberships. There
// is no single (orgID, storeID) to scope this list to, which is why it
// can't run behind RequireStaff/RequireGlobalStaffPermission like the other
// admin routes.
func handleListOrganizations(orgs domain.OrganizationRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := staffFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var list []*domain.Organization
		var err error
		if principalIsPlatformAdmin(principal) {
			list, err = orgs.ListAll(r.Context())
		} else {
			seen := make(map[int64]bool)
			var ids []int64
			for _, m := range principal.Memberships {
				if !seen[m.OrganizationID] {
					seen[m.OrganizationID] = true
					ids = append(ids, m.OrganizationID)
				}
			}
			list, err = orgs.ListByIDs(r.Context(), ids)
		}
		if err != nil {
			log.ErrorContext(r.Context(), "list organizations", "error", err)
			writeError(w, http.StatusInternalServerError, "list organizations")
			return
		}

		out := make([]organizationResponseBody, 0, len(list))
		for _, o := range list {
			out = append(out, organizationResponseBody{ID: o.ID, Name: o.Name, CreatedAt: o.CreatedAt})
		}
		writeJSON(w, http.StatusOK, map[string]any{"organizations": out})
	}
}
