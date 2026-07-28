package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/service"
)

// Field-length caps for the customer auth endpoints, mirroring the pattern
// in transactions.go: generous enough for any realistic input, tight
// enough to reject pathological input at the boundary.
const (
	maxOTPPhoneLen     = 32
	maxOTPCodeLen      = 12
	maxRefreshTokenLen = 512
	maxConsentFieldLen = 128
)

type otpRequestBody struct {
	Phone string `json:"phone"`
}

// handleRequestOTP returns a handler for POST /api/v1/auth/otp/request. It
// always responds 200 with the same body whether or not phone is already a
// registered CustomerAccount — see service.CustomerAuthService.RequestOTP.
func handleRequestOTP(customerAuth *service.CustomerAuthService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body otpRequestBody
		if err := dec.Decode(&body); err != nil || body.Phone == "" || len(body.Phone) > maxOTPPhoneLen {
			writeError(w, http.StatusBadRequest, "phone is required")
			return
		}

		err := customerAuth.RequestOTP(r.Context(), body.Phone, clientIP(r))
		switch {
		case errors.Is(err, service.ErrInvalidPhone):
			writeError(w, http.StatusBadRequest, "invalid phone number")
			return
		case errors.Is(err, service.ErrOTPCooldown), errors.Is(err, service.ErrOTPRateLimited):
			writeError(w, http.StatusTooManyRequests, "too many requests, try again later")
			return
		case err != nil:
			log.ErrorContext(r.Context(), "request otp", "error", err)
			writeError(w, http.StatusInternalServerError, "request otp")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
	}
}

type otpVerifyBody struct {
	Phone                  string `json:"phone"`
	Code                   string `json:"code"`
	ConsentDocumentVersion string `json:"consent_document_version,omitempty"`
	ConsentSource          string `json:"consent_source,omitempty"`
}

type authTokensResponseBody struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
	CustomerAccountID     int64     `json:"customer_account_id,omitempty"`
}

// handleVerifyOTP returns a handler for POST /api/v1/auth/otp/verify.
func handleVerifyOTP(customerAuth *service.CustomerAuthService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body otpVerifyBody
		if err := dec.Decode(&body); err != nil ||
			body.Phone == "" || len(body.Phone) > maxOTPPhoneLen ||
			body.Code == "" || len(body.Code) > maxOTPCodeLen ||
			len(body.ConsentDocumentVersion) > maxConsentFieldLen || len(body.ConsentSource) > maxConsentFieldLen {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		tokens, account, err := customerAuth.VerifyOTP(r.Context(), service.VerifyOTPRequest{
			Phone: body.Phone, Code: body.Code, IP: clientIP(r), UserAgent: r.UserAgent(),
			ConsentDocumentVersion: body.ConsentDocumentVersion, ConsentSource: body.ConsentSource,
		})
		switch {
		case errors.Is(err, service.ErrInvalidPhone):
			writeError(w, http.StatusBadRequest, "invalid phone number")
			return
		case errors.Is(err, service.ErrOTPInvalid):
			writeError(w, http.StatusUnauthorized, "invalid or expired code")
			return
		case errors.Is(err, service.ErrOTPLocked):
			writeError(w, http.StatusTooManyRequests, "too many attempts, request a new code")
			return
		case errors.Is(err, service.ErrAccountBlocked):
			writeError(w, http.StatusForbidden, "account blocked")
			return
		case err != nil:
			log.ErrorContext(r.Context(), "verify otp", "error", err)
			writeError(w, http.StatusInternalServerError, "verify otp")
			return
		}

		writeJSON(w, http.StatusOK, authTokensResponseBody{
			AccessToken: tokens.AccessToken, AccessTokenExpiresAt: tokens.AccessTokenExpiresAt,
			RefreshToken: tokens.RefreshToken, RefreshTokenExpiresAt: tokens.RefreshTokenExpiresAt,
			CustomerAccountID: account.ID,
		})
	}
}

type refreshTokenBody struct {
	RefreshToken string `json:"refresh_token"`
}

// handleRefreshCustomerSession returns a handler for POST
// /api/v1/auth/refresh. Rotates the presented refresh token; a token reused
// after rotation is treated as compromise and revokes the caller's entire
// session chain (see service.CustomerAuthService.Refresh).
func handleRefreshCustomerSession(customerAuth *service.CustomerAuthService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body refreshTokenBody
		if err := dec.Decode(&body); err != nil || body.RefreshToken == "" || len(body.RefreshToken) > maxRefreshTokenLen {
			writeError(w, http.StatusBadRequest, "refresh_token is required")
			return
		}

		tokens, err := customerAuth.Refresh(r.Context(), body.RefreshToken, clientIP(r), r.UserAgent())
		if errors.Is(err, service.ErrSessionInvalid) {
			writeError(w, http.StatusUnauthorized, "invalid session")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "refresh session", "error", err)
			writeError(w, http.StatusInternalServerError, "refresh session")
			return
		}

		writeJSON(w, http.StatusOK, authTokensResponseBody{
			AccessToken: tokens.AccessToken, AccessTokenExpiresAt: tokens.AccessTokenExpiresAt,
			RefreshToken: tokens.RefreshToken, RefreshTokenExpiresAt: tokens.RefreshTokenExpiresAt,
		})
	}
}

// handleCustomerLogout returns a handler for POST /api/v1/auth/logout: it
// revokes the single session identified by the presented refresh token.
// Presenting the token is itself the proof of ownership, so this route runs
// unauthenticated by access token (an already-expired access token
// shouldn't block a customer from logging out). Idempotent: an
// already-revoked or unknown token still returns success.
func handleCustomerLogout(customerAuth *service.CustomerAuthService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body refreshTokenBody
		if err := dec.Decode(&body); err != nil || body.RefreshToken == "" || len(body.RefreshToken) > maxRefreshTokenLen {
			writeError(w, http.StatusBadRequest, "refresh_token is required")
			return
		}

		if err := customerAuth.Logout(r.Context(), body.RefreshToken); err != nil {
			log.ErrorContext(r.Context(), "logout", "error", err)
			writeError(w, http.StatusInternalServerError, "logout")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// handleCustomerLogoutAll returns a handler for POST
// /api/v1/auth/logout-all. Must run behind RequireCustomerSession: revoking
// every session for an account requires proof of a currently valid access
// token, not just possession of one refresh token.
func handleCustomerLogoutAll(customerAuth *service.CustomerAuthService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerAccountID, ok := customerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if err := customerAuth.LogoutAll(r.Context(), customerAccountID); err != nil {
			log.ErrorContext(r.Context(), "logout all", "customer_account_id", customerAccountID, "error", err)
			writeError(w, http.StatusInternalServerError, "logout all")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
