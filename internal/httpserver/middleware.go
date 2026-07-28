package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// hashAPIKey returns the hex-encoded SHA-256 hash of an API key, matching
// what's stored in stores.api_key_hash. API keys are high-entropy random
// tokens issued once by the platform, not user-chosen passwords, so a plain
// hash (no per-store salt, no slow KDF) is sufficient here — see MVP-scope.md
// ("HMAC-подпись webhook... Phase 2").
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

type storeAPIKeyContextKey struct{}

// storeAPIKeyFromContext returns the *domain.StoreAPIKey RequireStoreAPIKey
// resolved for the current request, or (nil, true) if the request
// authenticated via the legacy stores.api_key_hash column, which predates
// scopes and is therefore treated as fully trusted (see requireScope).
func storeAPIKeyFromContext(ctx context.Context) (*domain.StoreAPIKey, bool) {
	v := ctx.Value(storeAPIKeyContextKey{})
	if v == nil {
		return nil, false
	}
	key, _ := v.(*domain.StoreAPIKey)
	return key, true
}

// RequireStoreAPIKey resolves the store whose webhook API key matches the
// Authorization: Bearer <key> header and injects it into the request
// context (see storeFromContext). 401s on a missing or unrecognized key.
//
// It checks the multi-key store_api_keys table first (see
// domain.StoreAPIKeyRepository — supports rotation, scopes, expiry,
// revocation), falling back to the legacy single-key stores.api_key_hash
// column so a store that hasn't rotated onto the new table keeps
// authenticating unchanged.
func RequireStoreAPIKey(stores domain.StoreRepository, apiKeys domain.StoreAPIKeyRepository, log *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if key == "" {
				writeError(w, http.StatusUnauthorized, "missing api key")
				return
			}

			hash := hashAPIKey(key)

			if apiKey, err := apiKeys.GetByHash(r.Context(), hash); err == nil {
				if !apiKey.Active(time.Now()) {
					writeError(w, http.StatusUnauthorized, "invalid api key")
					return
				}
				store, err := stores.GetByID(r.Context(), apiKey.StoreID)
				if err != nil {
					log.ErrorContext(r.Context(), "resolve store for api key", "store_id", apiKey.StoreID, "error", err)
					writeError(w, http.StatusInternalServerError, "resolve store")
					return
				}

				go func() {
					if err := apiKeys.TouchLastUsed(context.Background(), apiKey.ID, time.Now()); err != nil {
						log.Warn("touch store api key last used", "key_id", apiKey.ID, "error", err)
					}
				}()

				ctx := context.WithValue(r.Context(), storeContextKey{}, store)
				ctx = context.WithValue(ctx, storeAPIKeyContextKey{}, apiKey)
				next(w, r.WithContext(ctx))
				return
			} else if !errors.Is(err, domain.ErrNotFound) {
				log.ErrorContext(r.Context(), "resolve store api key", "error", err)
				writeError(w, http.StatusInternalServerError, "resolve store")
				return
			}

			store, err := stores.GetByAPIKeyHash(r.Context(), hash)
			if errors.Is(err, domain.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "invalid api key")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "resolve store")
				return
			}

			ctx := context.WithValue(r.Context(), storeContextKey{}, store)
			// A typed nil here (not omitted) is what makes
			// storeAPIKeyFromContext return (nil, true) for a legacy-
			// authenticated request, matching requireScope's contract that
			// it's fully trusted rather than "unauthorized".
			ctx = context.WithValue(ctx, storeAPIKeyContextKey{}, (*domain.StoreAPIKey)(nil))
			next(w, r.WithContext(ctx))
		}
	}
}

// requireScope 403s unless the request's resolved store API key grants
// scope. A request authenticated via the legacy stores.api_key_hash column
// (storeAPIKeyFromContext returns nil, true — no per-key scopes exist for
// it) is treated as fully trusted, matching its pre-scopes behavior.
func requireScope(scope string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			apiKey, ok := storeAPIKeyFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if apiKey != nil && !apiKey.HasScope(scope) {
				writeError(w, http.StatusForbidden, "api key missing required scope: "+scope)
				return
			}
			next(w, r)
		}
	}
}
