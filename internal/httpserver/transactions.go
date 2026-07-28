package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/service"
)

// Field-length caps enforced on both transaction endpoints: generous enough
// for any realistic 1C-assigned ID or phone number, tight enough to reject
// pathological input at the boundary rather than storing it.
const (
	maxExternalTxIDLen = 255
	maxPhoneLen        = 32
)

// accrueRequestBody is the 1C webhook payload (Idea.md's POST
// /api/v1/transactions), trimmed to MVP scope: no items/category rules, no
// tiers. Phone identifies the client at POS (Idea.md: QR/card/phone; MVP
// supports phone, see MVP-scope.md) and doubles as the registration flow —
// an unknown phone under this store creates a new client.
type accrueRequestBody struct {
	ExternalTxID string          `json:"transaction_id"`
	Phone        string          `json:"phone"`
	Amount       decimal.Decimal `json:"amount"`
}

// transactionResponseBody mirrors Idea.md's webhook response shape. Tier is
// omitted — tiers are out of MVP scope (see MVP-scope.md). Replayed
// surfaces idempotent-replay detection for sandbox/integration debugging;
// integrators may ignore it.
type transactionResponseBody struct {
	ClientID     int64 `json:"client_id"`
	PointsEarned int64 `json:"points_earned"`
	Balance      int64 `json:"balance"`
	Replayed     bool  `json:"replayed,omitempty"`
}

// handleAccrueTransaction returns a handler for POST /api/v1/transactions:
// the store-scoped accrual webhook called by 1C on each completed sale.
// Must run behind RequireStoreAPIKey.
func handleAccrueTransaction(loyalty *service.LoyaltyService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := storeFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body accrueRequestBody
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.ExternalTxID == "" || len(body.ExternalTxID) > maxExternalTxIDLen ||
			body.Phone == "" || len(body.Phone) > maxPhoneLen ||
			!body.Amount.IsPositive() {
			writeError(w, http.StatusBadRequest, "transaction_id, phone, and a positive amount are required")
			return
		}

		result, err := loyalty.Accrue(r.Context(), service.AccrueRequest{
			StoreID:      store.ID,
			ExternalTxID: body.ExternalTxID,
			Phone:        body.Phone,
			Amount:       body.Amount,
		})
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			writeError(w, http.StatusConflict, "transaction_id already used with different parameters")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "accrue points", "store_id", store.ID, "transaction_id", body.ExternalTxID, "error", err)
			writeError(w, http.StatusInternalServerError, "accrue points")
			return
		}

		writeJSON(w, http.StatusOK, transactionResponseBody{
			ClientID:     result.ClientID,
			PointsEarned: result.PointsEarned,
			Balance:      result.Balance,
			Replayed:     result.Replayed,
		})
	}
}

// redeemRequestBody is the payload for POST /api/v1/transactions/redeem
// (Idea.md). ClientID is the platform-assigned id returned by a prior
// accrual or balance lookup — redemption assumes the client is already
// registered, unlike accrual which can register on the fly from a phone
// number.
type redeemRequestBody struct {
	ExternalTxID string          `json:"transaction_id"`
	ClientID     int64           `json:"client_id"`
	Points       int64           `json:"points"`
	Amount       decimal.Decimal `json:"amount"`
}

type redeemResponseBody struct {
	PointsRedeemed int64 `json:"points_redeemed"`
	Balance        int64 `json:"balance"`
	Replayed       bool  `json:"replayed,omitempty"`
}

// handleRedeemTransaction returns a handler for POST
// /api/v1/transactions/redeem: confirms a points redemption at checkout.
// Must run behind RequireStoreAPIKey.
func handleRedeemTransaction(loyalty *service.LoyaltyService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := storeFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body redeemRequestBody
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.ExternalTxID == "" || len(body.ExternalTxID) > maxExternalTxIDLen ||
			body.ClientID <= 0 || body.Points <= 0 || !body.Amount.IsPositive() {
			writeError(w, http.StatusBadRequest, "transaction_id, client_id, a positive points value, and a positive amount are required")
			return
		}

		result, err := loyalty.Redeem(r.Context(), service.RedeemRequest{
			StoreID:      store.ID,
			ExternalTxID: body.ExternalTxID,
			ClientID:     body.ClientID,
			Points:       body.Points,
			Amount:       body.Amount,
		})
		if errors.Is(err, domain.ErrInsufficientBalance) {
			writeError(w, http.StatusUnprocessableEntity, "insufficient balance")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "client not found")
			return
		}
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			writeError(w, http.StatusConflict, "transaction_id already used with different parameters")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "redeem points", "store_id", store.ID, "transaction_id", body.ExternalTxID, "error", err)
			writeError(w, http.StatusInternalServerError, "redeem points")
			return
		}

		writeJSON(w, http.StatusOK, redeemResponseBody{
			PointsRedeemed: result.PointsRedeemed,
			Balance:        result.Balance,
			Replayed:       result.Replayed,
		})
	}
}
