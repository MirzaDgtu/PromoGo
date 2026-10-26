package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

const maxNameLen = 255

// normalizeName trims surrounding whitespace and reports whether the result
// is a usable organization/store name. The limit counts characters, not
// bytes, to match the OpenAPI maxLength — a byte limit would reject
// Cyrillic names at roughly half the documented length.
func normalizeName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	return name, name != "" && utf8.RuneCountInString(name) <= maxNameLen
}

type createOrganizationBody struct {
	Name string `json:"name"`
}

type organizationResponseBody struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// handleCreateOrganization returns a handler for POST
// /api/v1/admin/organizations. Must run behind
// RequireGlobalStaffPermission(organizations.manage) — creating a tenant
// can't be scoped to a tenant that doesn't exist yet.
func handleCreateOrganization(orgs domain.OrganizationRepository, audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body createOrganizationBody
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		name, ok := normalizeName(body.Name)
		if !ok {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		org := &domain.Organization{Name: name}
		if err := orgs.Create(r.Context(), org); err != nil {
			log.ErrorContext(r.Context(), "create organization", "error", err)
			writeError(w, http.StatusInternalServerError, "create organization")
			return
		}
		auditCreate(r.Context(), audit, log, domain.AuditActorStaff, actorStaffUserID(r), domain.AuditActionOrganizationCreated, &org.ID, nil, "organization", &org.ID, clientIP(r), r.UserAgent())

		writeJSON(w, http.StatusCreated, organizationResponseBody{ID: org.ID, Name: org.Name, CreatedAt: org.CreatedAt})
	}
}

const (
	maxAddressLen      = 500
	maxExternalCodeLen = 64
)

type storeResponseBody struct {
	ID             int64   `json:"id"`
	OrganizationID int64   `json:"organization_id"`
	Name           string  `json:"name"`
	Address        string  `json:"address"`
	ExternalCode   *string `json:"external_code"`
}

func storeToBody(s *domain.Store) storeResponseBody {
	return storeResponseBody{ID: s.ID, OrganizationID: s.OrganizationID, Name: s.Name, Address: s.Address, ExternalCode: s.ExternalCode}
}

// storeProfileBody carries the editable store fields. On create, Name is
// required and absent optional fields are empty; on PATCH, an absent field
// keeps its current value and an empty external_code clears it.
type storeProfileBody struct {
	Name         *string `json:"name"`
	Address      *string `json:"address"`
	ExternalCode *string `json:"external_code"`
}

// applyTo validates the present fields and copies them onto store, or
// returns a client-facing error message.
func (b storeProfileBody) applyTo(store *domain.Store) (errMsg string) {
	if b.Name != nil {
		name, ok := normalizeName(*b.Name)
		if !ok {
			return "name is required"
		}
		store.Name = name
	}
	if b.Address != nil {
		address := strings.TrimSpace(*b.Address)
		if utf8.RuneCountInString(address) > maxAddressLen {
			return "address is too long"
		}
		store.Address = address
	}
	if b.ExternalCode != nil {
		code := strings.TrimSpace(*b.ExternalCode)
		if utf8.RuneCountInString(code) > maxExternalCodeLen {
			return "external_code is too long"
		}
		if code == "" {
			store.ExternalCode = nil
		} else {
			store.ExternalCode = &code
		}
	}
	return ""
}

const errExternalCodeTaken = "external_code is already used by another store in this organization"

// handleCreateStore returns a handler for POST
// /api/v1/admin/organizations/{orgID}/stores. Must run behind
// RequireStaff(stores.manage, orgScopeFromPath).
func handleCreateStore(stores domain.StoreRepository, audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body storeProfileBody
		if err := dec.Decode(&body); err != nil || body.Name == nil {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		store := &domain.Store{OrganizationID: orgID}
		if msg := body.applyTo(store); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}

		err = stores.Create(r.Context(), store)
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, http.StatusConflict, errExternalCodeTaken)
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "create store", "organization_id", orgID, "error", err)
			writeError(w, http.StatusInternalServerError, "create store")
			return
		}
		auditCreate(r.Context(), audit, log, domain.AuditActorStaff, actorStaffUserID(r), domain.AuditActionStoreCreated, &orgID, &store.ID, "store", &store.ID, clientIP(r), r.UserAgent())

		writeJSON(w, http.StatusCreated, storeToBody(store))
	}
}

// handleUpdateStore returns a handler for PATCH
// /api/v1/admin/organizations/{orgID}/stores/{storeID}. Must run behind
// RequireStaff(stores.manage, storeScopeFromPath); 404s if the store
// doesn't belong to orgID.
func handleUpdateStore(stores domain.StoreRepository, audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, orgID, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body storeProfileBody
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid store")
			return
		}
		if msg := body.applyTo(store); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}

		err := stores.UpdateProfile(r.Context(), store)
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, http.StatusConflict, errExternalCodeTaken)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "store not found")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "update store", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "update store")
			return
		}
		auditCreate(r.Context(), audit, log, domain.AuditActorStaff, actorStaffUserID(r), domain.AuditActionStoreUpdated, &orgID, &store.ID, "store", &store.ID, clientIP(r), r.UserAgent())

		writeJSON(w, http.StatusOK, storeToBody(store))
	}
}

// handleListStores returns a handler for GET
// /api/v1/admin/organizations/{orgID}/stores. Must run behind
// RequireStaffIdentity: unlike stores.manage/read, "which stores can I see
// in this org" depends on whether the caller's membership in orgID is
// organization-wide or narrowed to one store, which RequireStaff's
// per-route scope check can't express — so this handler resolves its own
// visibility instead. A platform_admin or an organization-wide membership
// sees every store in orgID; a store-scoped membership sees only its own
// store(s); a caller with no membership in orgID at all gets 403.
func handleListStores(stores domain.StoreRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		principal, ok := staffFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		orgWide := principalIsPlatformAdmin(principal)
		allowedStoreIDs := map[int64]bool{}
		matchedOrg := orgWide
		for _, m := range principal.Memberships {
			if m.OrganizationID != orgID {
				continue
			}
			matchedOrg = true
			if m.StoreID == nil {
				orgWide = true
			} else {
				allowedStoreIDs[*m.StoreID] = true
			}
		}
		if !matchedOrg {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		list, err := stores.ListByOrganization(r.Context(), orgID)
		if err != nil {
			log.ErrorContext(r.Context(), "list stores", "organization_id", orgID, "error", err)
			writeError(w, http.StatusInternalServerError, "list stores")
			return
		}

		out := make([]storeResponseBody, 0, len(list))
		for _, s := range list {
			if !orgWide && !allowedStoreIDs[s.ID] {
				continue
			}
			out = append(out, storeToBody(s))
		}
		writeJSON(w, http.StatusOK, map[string]any{"stores": out})
	}
}

// handleGetStore returns a handler for GET
// /api/v1/admin/organizations/{orgID}/stores/{storeID}. Must run behind
// RequireStaff(stores.read, storeScopeFromPath); 404s if the store doesn't
// belong to orgID, so one organization's staff can't probe another's store
// ids.
func handleGetStore(stores domain.StoreRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization id")
			return
		}
		storeID, err := strconv.ParseInt(r.PathValue("storeID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid store id")
			return
		}

		store, err := stores.GetByID(r.Context(), storeID)
		if errors.Is(err, domain.ErrNotFound) || (err == nil && store.OrganizationID != orgID) {
			writeError(w, http.StatusNotFound, "store not found")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "load store", "store_id", storeID, "error", err)
			writeError(w, http.StatusInternalServerError, "load store")
			return
		}

		writeJSON(w, http.StatusOK, storeToBody(store))
	}
}
