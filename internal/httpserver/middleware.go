package httpserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// hashAPIKey returns the hex-encoded SHA-256 hash of an API key secret,
// matching what's stored in stores.api_key_hash / store_api_keys.key_hash.
// API keys are high-entropy random tokens issued once by the platform, not
// user-chosen passwords, so a plain hash (no per-store salt, no slow KDF) is
// sufficient here — see MVP-scope.md ("HMAC-подпись webhook... Phase 2").
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// parseMultiKeyAPIKey splits a store_api_keys-issued plaintext key of the
// form "<keyID>.<secret>" (see auth.GenerateAPIKey) into its two halves. A
// legacy stores.api_key_hash key is a single opaque token with no such
// separator and never matches, so it falls through to the legacy path
// below unchanged.
func parseMultiKeyAPIKey(key string) (keyID, secret string, ok bool) {
	keyID, secret, found := strings.Cut(key, ".")
	if !found || keyID == "" || secret == "" {
		return "", "", false
	}
	return keyID, secret, true
}

// constantTimeHashEqual reports whether hexHash is the SHA-256 hex digest of
// secret, comparing in constant time.
func constantTimeHashEqual(secret, hexHash string) bool {
	return subtle.ConstantTimeCompare([]byte(hashAPIKey(secret)), []byte(hexHash)) == 1
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
// revocation): the key is parsed as "<keyID>.<secret>", the row is looked
// up by the non-secret keyID, and the secret is compared against the
// stored hash in constant time. A key without that shape (or an unknown
// keyID) falls back to the legacy single-key stores.api_key_hash column so
// a store that hasn't rotated onto the new table keeps authenticating
// unchanged.
func RequireStoreAPIKey(stores domain.StoreRepository, apiKeys domain.StoreAPIKeyRepository, log *slog.Logger, bg *BackgroundTracker) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if key == "" {
				writeError(w, http.StatusUnauthorized, "missing api key")
				return
			}

			if keyID, secret, ok := parseMultiKeyAPIKey(key); ok {
				apiKey, err := apiKeys.GetByKeyID(r.Context(), keyID)
				if err != nil && !errors.Is(err, domain.ErrNotFound) {
					log.ErrorContext(r.Context(), "resolve store api key", "error", err)
					writeError(w, http.StatusInternalServerError, "resolve store")
					return
				}
				if err == nil {
					if !constantTimeHashEqual(secret, apiKey.KeyHash) || !apiKey.Active(time.Now()) {
						writeError(w, http.StatusUnauthorized, "invalid api key")
						return
					}
					store, err := stores.GetByID(r.Context(), apiKey.StoreID)
					if err != nil {
						log.ErrorContext(r.Context(), "resolve store for api key", "store_id", apiKey.StoreID, "error", err)
						writeError(w, http.StatusInternalServerError, "resolve store")
						return
					}

					bg.Go(func() {
						if err := apiKeys.TouchLastUsed(context.Background(), apiKey.ID, time.Now()); err != nil {
							log.Warn("touch store api key last used", "key_id", apiKey.ID, "error", err)
						}
					})

					ctx := context.WithValue(r.Context(), storeContextKey{}, store)
					ctx = context.WithValue(ctx, storeAPIKeyContextKey{}, apiKey)
					next(w, r.WithContext(ctx))
					return
				}
				// keyID unrecognized: fall through to the legacy check
				// below rather than 401ing immediately, since the same
				// bearer value space is shared with legacy keys.
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
