package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

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
