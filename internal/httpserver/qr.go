package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/service"
)

const maxQRPayloadLen = 512

type issueQRResponseBody struct {
	Payload   string    `json:"payload"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int64     `json:"expires_in"`
}

// handleIssueQR returns a handler for POST /api/v1/me/qr: mints a new
// one-time QR payload for the caller's CustomerAccount (DEC-011). Must run
// behind RequireCustomerSession.
func handleIssueQR(qr *service.QRService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerAccountID, ok := customerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		payload, expiresAt, retryAfter, err := qr.IssueQR(r.Context(), customerAccountID)
		if errors.Is(err, service.ErrQRIssueCooldown) {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Round(time.Second)/time.Second)))
			writeErrorCode(w, http.StatusTooManyRequests, "qr_issue_cooldown", "too many requests, try again later")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "issue qr", "customer_account_id", customerAccountID, "error", err)
			writeError(w, http.StatusInternalServerError, "issue qr")
			return
		}

		writeJSON(w, http.StatusOK, issueQRResponseBody{
			Payload:   payload,
			ExpiresAt: expiresAt,
			ExpiresIn: int64(time.Until(expiresAt).Round(time.Second) / time.Second),
		})
	}
}

type resolveQRRequestBody struct {
	Payload string `json:"payload"`
}

type resolveQRResponseBody struct {
	ClientID int64 `json:"client_id"`
	Balance  int64 `json:"balance"`
}

// handleResolveQR returns a handler for POST /api/v1/clients/resolve-qr:
// atomically consumes a customer-issued QR payload and returns the
// store-scoped Client/Balance it resolves to (DEC-011). Must run behind
// RequireStoreAPIKey.
func handleResolveQR(qr *service.QRService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := storeFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body resolveQRRequestBody
		if err := dec.Decode(&body); err != nil || body.Payload == "" || len(body.Payload) > maxQRPayloadLen {
			writeErrorCode(w, http.StatusBadRequest, "qr_malformed", "payload is required")
			return
		}

		client, balance, err := qr.ResolveQR(r.Context(), store.ID, body.Payload, strconv.FormatInt(store.ID, 10))
		switch {
		case errors.Is(err, service.ErrQRMalformed):
			writeErrorCode(w, http.StatusBadRequest, "qr_malformed", "malformed qr payload")
			return
		case errors.Is(err, service.ErrQRConsumeCooldown):
			writeErrorCode(w, http.StatusTooManyRequests, "qr_consume_cooldown", "too many requests, try again later")
			return
		case errors.Is(err, service.ErrQRGone):
			writeErrorCode(w, http.StatusGone, "qr_gone", "qr code expired or already used")
			return
		case err != nil:
			log.ErrorContext(r.Context(), "resolve qr", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "resolve qr")
			return
		}

		writeJSON(w, http.StatusOK, resolveQRResponseBody{ClientID: client.ID, Balance: balance.Points})
	}
}
