package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

const maxAPIKeyNameLen = 255

// resolveScopedStore loads storeID and verifies it belongs to orgID,
// 404ing otherwise so one organization's staff can't reach another
// organization's store by guessing a store id.
func resolveScopedStore(w http.ResponseWriter, r *http.Request, stores domain.StoreRepository, log *slog.Logger) (*domain.Store, int64, bool) {
	orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid organization id")
		return nil, 0, false
	}
	storeID, err := strconv.ParseInt(r.PathValue("storeID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid store id")
		return nil, 0, false
	}

	store, err := stores.GetByID(r.Context(), storeID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && store.OrganizationID != orgID) {
		writeError(w, http.StatusNotFound, "store not found")
		return nil, 0, false
	}
	if err != nil {
		log.ErrorContext(r.Context(), "load store", "store_id", storeID, "error", err)
		writeError(w, http.StatusInternalServerError, "load store")
		return nil, 0, false
	}

	return store, orgID, true
}

type storeAPIKeyResponseBody struct {
	ID         int64      `json:"id"`
	KeyID      string     `json:"key_id"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func apiKeyToBody(k *domain.StoreAPIKey) storeAPIKeyResponseBody {
	return storeAPIKeyResponseBody{
		ID: k.ID, KeyID: k.KeyID, Name: k.Name, Scopes: k.Scopes,
		CreatedAt: k.CreatedAt, ExpiresAt: k.ExpiresAt, RevokedAt: k.RevokedAt, LastUsedAt: k.LastUsedAt,
	}
}

// handleListStoreAPIKeys returns a handler for GET
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys. Never
// returns KeyHash. Must run behind RequireStaff(api_keys.read, storeScopeFromPath).
func handleListStoreAPIKeys(stores domain.StoreRepository, keys domain.StoreAPIKeyRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, _, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		list, err := keys.ListByStore(r.Context(), store.ID)
		if err != nil {
			log.ErrorContext(r.Context(), "list store api keys", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "list api keys")
			return
		}

		out := make([]storeAPIKeyResponseBody, 0, len(list))
		for _, k := range list {
			out = append(out, apiKeyToBody(k))
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_keys": out})
	}
}

type createAPIKeyBody struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

var validAPIKeyScopes = map[string]bool{
	domain.ScopeTransactionsWrite: true,
	domain.ScopeClientsLookup:     true,
	domain.ScopeBalancesRead:      true,
}

// handleCreateStoreAPIKey returns a handler for POST
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys. Returns
// the plaintext key exactly once — it is never persisted or logged, only
// its hash. Must run behind RequireStaff(api_keys.rotate, storeScopeFromPath).
func handleCreateStoreAPIKey(stores domain.StoreRepository, keys domain.StoreAPIKeyRepository, audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, orgID, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body createAPIKeyBody
		if err := dec.Decode(&body); err != nil || body.Name == "" || len(body.Name) > maxAPIKeyNameLen || len(body.Scopes) == 0 {
			writeError(w, http.StatusBadRequest, "name and at least one scope are required")
			return
		}
		for _, scope := range body.Scopes {
			if !validAPIKeyScopes[scope] {
				writeError(w, http.StatusBadRequest, "invalid scope: "+scope)
				return
			}
		}

		plaintext, keyID, hash, err := auth.GenerateAPIKey()
		if err != nil {
			log.ErrorContext(r.Context(), "generate api key", "error", err)
			writeError(w, http.StatusInternalServerError, "generate api key")
			return
		}

		principal, _ := staffFromContext(r.Context())
		var createdBy *int64
		if principal != nil {
			id := principal.StaffUserID
			createdBy = &id
		}

		key := &domain.StoreAPIKey{
			StoreID: store.ID, KeyID: keyID, KeyHash: hash, Name: body.Name,
			Scopes: body.Scopes, ExpiresAt: body.ExpiresAt, CreatedBy: createdBy,
		}
		if err := keys.Create(r.Context(), key); err != nil {
			log.ErrorContext(r.Context(), "create store api key", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "create api key")
			return
		}

		auditCreate(r.Context(), audit, log, domain.AuditActorStaff, createdBy, domain.AuditActionAPIKeyCreated, &orgID, &store.ID, "store_api_key", &key.ID, clientIP(r), r.UserAgent())

		resp := struct {
			storeAPIKeyResponseBody
			PlaintextKey string `json:"plaintext_key"`
		}{storeAPIKeyResponseBody: apiKeyToBody(key), PlaintextKey: plaintext}
		writeJSON(w, http.StatusCreated, resp)
	}
}

// handleRevokeStoreAPIKey returns a handler for POST
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys/{keyID}/revoke.
// Must run behind RequireStaff(api_keys.rotate, storeScopeFromPath).
func handleRevokeStoreAPIKey(stores domain.StoreRepository, keys domain.StoreAPIKeyRepository, audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, orgID, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		keyID, err := strconv.ParseInt(r.PathValue("keyID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid key id")
			return
		}

		if err := keys.Revoke(r.Context(), keyID); err != nil {
			log.ErrorContext(r.Context(), "revoke store api key", "key_id", keyID, "error", err)
			writeError(w, http.StatusInternalServerError, "revoke api key")
			return
		}

		principal, _ := staffFromContext(r.Context())
		var actorID *int64
		if principal != nil {
			id := principal.StaffUserID
			actorID = &id
		}
		auditCreate(r.Context(), audit, log, domain.AuditActorStaff, actorID, domain.AuditActionAPIKeyRevoked, &orgID, &store.ID, "store_api_key", &keyID, clientIP(r), r.UserAgent())

		w.WriteHeader(http.StatusNoContent)
	}
}
