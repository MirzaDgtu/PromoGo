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

const maxNameLen = 255

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
func handleCreateOrganization(orgs domain.OrganizationRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body createOrganizationBody
		if err := dec.Decode(&body); err != nil || body.Name == "" || len(body.Name) > maxNameLen {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		org := &domain.Organization{Name: body.Name}
		if err := orgs.Create(r.Context(), org); err != nil {
			log.ErrorContext(r.Context(), "create organization", "error", err)
			writeError(w, http.StatusInternalServerError, "create organization")
			return
		}

		writeJSON(w, http.StatusCreated, organizationResponseBody{ID: org.ID, Name: org.Name, CreatedAt: org.CreatedAt})
	}
}

type storeResponseBody struct {
	ID             int64  `json:"id"`
	OrganizationID int64  `json:"organization_id"`
	Name           string `json:"name"`
}

type createStoreBody struct {
	Name string `json:"name"`
}

// handleCreateStore returns a handler for POST
// /api/v1/admin/organizations/{orgID}/stores. Must run behind
// RequireStaff(stores.manage, orgScopeFromPath).
func handleCreateStore(stores domain.StoreRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body createStoreBody
		if err := dec.Decode(&body); err != nil || body.Name == "" || len(body.Name) > maxNameLen {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		store := &domain.Store{OrganizationID: orgID, Name: body.Name}
		if err := stores.Create(r.Context(), store); err != nil {
			log.ErrorContext(r.Context(), "create store", "organization_id", orgID, "error", err)
			writeError(w, http.StatusInternalServerError, "create store")
			return
		}

		writeJSON(w, http.StatusCreated, storeResponseBody{ID: store.ID, OrganizationID: store.OrganizationID, Name: store.Name})
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

		writeJSON(w, http.StatusOK, storeResponseBody{ID: store.ID, OrganizationID: store.OrganizationID, Name: store.Name})
	}
}
