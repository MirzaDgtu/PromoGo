package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

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

// RequireStoreAPIKey resolves the store whose webhook API key matches the
// Authorization: Bearer <key> header and injects it into the request
// context (see storeFromContext). 401s on a missing or unrecognized key.
func RequireStoreAPIKey(stores domain.StoreRepository) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if key == "" {
				writeError(w, http.StatusUnauthorized, "missing api key")
				return
			}

			store, err := stores.GetByAPIKeyHash(r.Context(), hashAPIKey(key))
			if errors.Is(err, domain.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "invalid api key")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "resolve store")
				return
			}

			ctx := context.WithValue(r.Context(), storeContextKey{}, store)
			next(w, r.WithContext(ctx))
		}
	}
}
