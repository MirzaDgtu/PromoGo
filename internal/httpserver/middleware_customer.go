package httpserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
)

// RequireCustomerSession resolves the CustomerAccountID carried by a
// customer access token (Authorization: Bearer <access-token>) and injects
// it into the request context (see customerFromContext). 401s on a missing
// or invalid/expired token.
func RequireCustomerSession(accessTokenSecret []byte) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing access token")
				return
			}

			customerAccountID, err := auth.ParseCustomerAccessToken(accessTokenSecret, token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid access token")
				return
			}

			ctx := context.WithValue(r.Context(), customerContextKey{}, customerAccountID)
			next(w, r.WithContext(ctx))
		}
	}
}
