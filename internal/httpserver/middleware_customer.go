package httpserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// RequireCustomerSession resolves the CustomerAccountID carried by a
// customer access token (Authorization: Bearer <access-token>) and injects
// it into the request context (see customerFromContext). 401s on a missing
// or invalid/expired token.
//
// It also loads the account's current status on every request — like
// RequireStaff's fresh ResolvePrincipal call, this deliberately never
// trusts the token's claims alone: the access token is a stateless,
// unrevocable-by-itself JWT, so blocking/deleting an account must take
// effect faster than its (up to AccessTokenTTL) remaining lifetime. A
// missing account is treated the same as blocked/deleted — no legitimate
// access token can name an account that doesn't exist, so this only
// happens if the account row itself was removed out from under a live
// token.
func RequireCustomerSession(accessTokenSecret []byte, accounts domain.CustomerAccountRepository) func(http.HandlerFunc) http.HandlerFunc {
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

			account, err := accounts.GetByID(r.Context(), customerAccountID)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if account.Status != domain.CustomerAccountActive {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			ctx := context.WithValue(r.Context(), customerContextKey{}, customerAccountID)
			next(w, r.WithContext(ctx))
		}
	}
}
