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

// refundRequestBody is the payload for POST /api/v1/transactions/refund
// (DEC-012). OriginalType is optional — see
// service.LoyaltyService.resolveOriginalTransaction's doc comment for when
// it's required to disambiguate.
type refundRequestBody struct {
	ExternalTxID         string          `json:"transaction_id"`
	OriginalExternalTxID string          `json:"original_transaction_id"`
	OriginalType         string          `json:"original_transaction_type,omitempty"`
	Amount               decimal.Decimal `json:"amount"`
}

type refundResponseBody struct {
	ClientID            int64           `json:"client_id"`
	PointsReversed      int64           `json:"points_reversed"`
	Balance             int64           `json:"balance"`
	RefundedAmountTotal decimal.Decimal `json:"refunded_amount_total"`
	FullyRefunded       bool            `json:"fully_refunded"`
	Replayed            bool            `json:"replayed,omitempty"`
}

// handleRefundTransaction returns a handler for POST
// /api/v1/transactions/refund: partially or fully reverses a prior
// accrual/redeem transaction (DEC-012). Must run behind RequireStoreAPIKey.
func handleRefundTransaction(loyalty *service.LoyaltyService, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := storeFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body refundRequestBody
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.ExternalTxID == "" || len(body.ExternalTxID) > maxExternalTxIDLen ||
			body.OriginalExternalTxID == "" || len(body.OriginalExternalTxID) > maxExternalTxIDLen ||
			!body.Amount.IsPositive() {
			writeError(w, http.StatusBadRequest, "transaction_id, original_transaction_id, and a positive amount are required")
			return
		}

		var originalType domain.TransactionType
		switch body.OriginalType {
		case "":
			// Ambiguity (if any) is resolved by the service.
		case string(domain.TransactionAccrual), string(domain.TransactionRedeem):
			originalType = domain.TransactionType(body.OriginalType)
		default:
			writeError(w, http.StatusBadRequest, "original_transaction_type must be \"accrual\" or \"redeem\" if set")
			return
		}

		result, err := loyalty.Refund(r.Context(), service.RefundRequest{
			StoreID:              store.ID,
			ExternalTxID:         body.ExternalTxID,
			OriginalExternalTxID: body.OriginalExternalTxID,
			OriginalType:         originalType,
			Amount:               body.Amount,
		})
		switch {
		case errors.Is(err, domain.ErrAmbiguousOriginalTransaction):
			writeErrorCode(w, http.StatusBadRequest, "ambiguous_original_transaction", "original_transaction_type is required: original_transaction_id matches both an accrual and a redeem")
			return
		case errors.Is(err, domain.ErrCannotRefundRefund):
			writeErrorCode(w, http.StatusBadRequest, "cannot_refund_refund", "cannot refund a refund")
			return
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "original transaction not found")
			return
		case errors.Is(err, domain.ErrOverRefund):
			writeErrorCode(w, http.StatusUnprocessableEntity, "over_refund", "refund amount exceeds the original transaction's remaining refundable amount")
			return
		case errors.Is(err, domain.ErrInsufficientBalance):
			writeError(w, http.StatusUnprocessableEntity, "insufficient balance")
			return
		case errors.Is(err, domain.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "transaction_id already used with different parameters")
			return
		case err != nil:
			log.ErrorContext(r.Context(), "refund transaction", "store_id", store.ID, "transaction_id", body.ExternalTxID, "error", err)
			writeError(w, http.StatusInternalServerError, "refund transaction")
			return
		}

		writeJSON(w, http.StatusOK, refundResponseBody{
			ClientID:            result.ClientID,
			PointsReversed:      result.PointsReversed,
			Balance:             result.Balance,
			RefundedAmountTotal: result.RefundedAmountTotal,
			FullyRefunded:       result.FullyRefunded,
			Replayed:            result.Replayed,
		})
	}
}
