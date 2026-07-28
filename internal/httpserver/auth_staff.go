package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/MirzaDgtu/PromoGo/internal/service"
)

const maxOIDCTokenLen = 8192

type staffOIDCLoginBody struct {
	IDToken string `json:"id_token"`
}

type staffAuthResponseBody struct {
	AccessToken string `json:"access_token"`
}

// handleStaffOIDCLogin returns a handler for POST /api/v1/staff/auth/oidc:
// exchanges a verified OIDC ID token (from the retailer/platform's IdP) for
// a short-lived PromoGo staff access token. Public route — the OIDC token
// itself is the proof of identity.
func handleStaffOIDCLogin(staffAuth *service.StaffAuthService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body staffOIDCLoginBody
		if err := dec.Decode(&body); err != nil || body.IDToken == "" || len(body.IDToken) > maxOIDCTokenLen {
			writeError(w, http.StatusBadRequest, "id_token is required")
			return
		}

		token, _, err := staffAuth.AuthenticateOIDC(r.Context(), body.IDToken, clientIP(r), r.UserAgent())
		switch {
		case errors.Is(err, service.ErrStaffDisabled):
			writeError(w, http.StatusForbidden, "account disabled")
			return
		case errors.Is(err, service.ErrNoActiveMembership):
			writeError(w, http.StatusForbidden, "no active organization membership")
			return
		case err != nil:
			log.WarnContext(r.Context(), "staff oidc login", "error", err)
			writeError(w, http.StatusUnauthorized, "invalid id token")
			return
		}

		writeJSON(w, http.StatusOK, staffAuthResponseBody{AccessToken: token})
	}
}
