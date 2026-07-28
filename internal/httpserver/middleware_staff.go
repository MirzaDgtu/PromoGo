package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/service"
)

// staffScope is the (organization, optional store) a staff request targets,
// resolved per-route from the URL — see the /api/v1/admin/organizations/{orgID}/...
// and .../stores/{storeID}/... route shapes in server.go.
type staffScope struct {
	OrganizationID int64
	StoreID        *int64
}

// orgScopeFromPath resolves staffScope from a route's {orgID} path value —
// an organization-level resource.
func orgScopeFromPath(r *http.Request) (staffScope, error) {
	orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
	if err != nil {
		return staffScope{}, errors.New("invalid organization id")
	}
	return staffScope{OrganizationID: orgID}, nil
}

// storeScopeFromPath resolves staffScope from a route's {orgID} and
// {storeID} path values — a store-level resource within an organization.
func storeScopeFromPath(r *http.Request) (staffScope, error) {
	orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
	if err != nil {
		return staffScope{}, errors.New("invalid organization id")
	}
	storeID, err := strconv.ParseInt(r.PathValue("storeID"), 10, 64)
	if err != nil {
		return staffScope{}, errors.New("invalid store id")
	}
	return staffScope{OrganizationID: orgID, StoreID: &storeID}, nil
}

// staffPrincipalResolver is the subset of *service.StaffAuthService that
// RequireStaff needs — narrowed to an interface so handlers/middleware
// don't depend on the concrete service type more than necessary.
type staffPrincipalResolver interface {
	ResolvePrincipal(ctx context.Context, staffUserID int64) (*service.StaffPrincipal, error)
}

// RequireStaff resolves the caller's staff access token, loads their
// current memberships fresh from the database (see
// StaffAuthService.ResolvePrincipal — never trusts stale token claims for
// permission data), resolves the request's (organization, store) scope via
// resolveScope, and 403s unless the caller holds perm in that scope. 401s
// on a missing/invalid token or a disabled account.
func RequireStaff(accessTokenSecret []byte, staffAuth staffPrincipalResolver, perm domain.Permission, resolveScope func(*http.Request) (staffScope, error)) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing access token")
				return
			}

			staffUserID, err := auth.ParseStaffAccessToken(accessTokenSecret, token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid access token")
				return
			}

			principal, err := staffAuth.ResolvePrincipal(r.Context(), staffUserID)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			scope, err := resolveScope(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}

			if !principal.HasPermission(perm, scope.OrganizationID, scope.StoreID) {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}

			ctx := context.WithValue(r.Context(), staffContextKey{}, principal)
			next(w, r.WithContext(ctx))
		}
	}
}

// RequireGlobalStaffPermission is RequireStaff without an organization/store
// scope: it grants access if perm is held by any of the caller's active
// memberships, anywhere. Reserved for the small set of operations that are
// inherently platform-wide rather than organization-scoped — currently only
// organization creation (a platform_admin bootstrapping a new tenant, which
// by definition can't yet be scoped to that tenant's own organization id).
func RequireGlobalStaffPermission(accessTokenSecret []byte, staffAuth staffPrincipalResolver, perm domain.Permission) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing access token")
				return
			}

			staffUserID, err := auth.ParseStaffAccessToken(accessTokenSecret, token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid access token")
				return
			}

			principal, err := staffAuth.ResolvePrincipal(r.Context(), staffUserID)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			allowed := false
			for _, m := range principal.Memberships {
				if domain.HasPermission(m.Role, perm) {
					allowed = true
					break
				}
			}
			if !allowed {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}

			ctx := context.WithValue(r.Context(), staffContextKey{}, principal)
			next(w, r.WithContext(ctx))
		}
	}
}
