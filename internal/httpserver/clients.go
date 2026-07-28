package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

type balanceResponseBody struct {
	ClientID int64 `json:"client_id"`
	Balance  int64 `json:"balance"`
}

// handleGetClientBalance returns a handler for GET
// /api/v1/clients/{id}/balance. Must run behind RequireStoreAPIKey; 404s if
// the client doesn't belong to the requesting store, so one store's API key
// can't be used to probe another store's client balances.
func handleGetClientBalance(clients domain.ClientRepository, balances domain.BalanceRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := storeFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid client id")
			return
		}

		client, err := clients.GetByID(r.Context(), id)
		if errors.Is(err, domain.ErrNotFound) || (err == nil && client.StoreID != store.ID) {
			writeError(w, http.StatusNotFound, "client not found")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "load client", "client_id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "load client")
			return
		}

		balance, err := balances.Get(r.Context(), client.ID)
		if err != nil {
			log.ErrorContext(r.Context(), "load balance", "client_id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "load balance")
			return
		}

		writeJSON(w, http.StatusOK, balanceResponseBody{ClientID: client.ID, Balance: balance.Points})
	}
}

// handleLookupClientByPhone returns a handler for GET
// /api/v1/clients/lookup?phone=...: resolves a client_id (and current
// balance) for a phone already registered under the requesting store, so a
// cashier can go straight from "customer gives phone number" to
// /transactions/redeem without a prior accrual in the same session. Must
// run behind RequireStoreAPIKey.
func handleLookupClientByPhone(clients domain.ClientRepository, balances domain.BalanceRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := storeFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		phone := r.URL.Query().Get("phone")
		if phone == "" {
			writeError(w, http.StatusBadRequest, "phone is required")
			return
		}
		// Best-effort canonicalization, matching the format Accrue stores
		// new clients under (see internal/service/loyalty.go's Accrue) —
		// falls back to the raw query value on any normalization failure.
		if normalized, err := auth.NormalizePhone(phone); err == nil {
			phone = normalized
		}

		client, err := clients.GetByPhone(r.Context(), store.ID, phone)
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "client not found")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "lookup client", "phone", phone, "error", err)
			writeError(w, http.StatusInternalServerError, "lookup client")
			return
		}

		balance, err := balances.Get(r.Context(), client.ID)
		if err != nil {
			log.ErrorContext(r.Context(), "load balance", "client_id", client.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "load balance")
			return
		}

		writeJSON(w, http.StatusOK, balanceResponseBody{ClientID: client.ID, Balance: balance.Points})
	}
}
