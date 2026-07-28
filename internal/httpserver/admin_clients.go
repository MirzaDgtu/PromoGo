package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

type adminClientResponseBody struct {
	ClientID int64  `json:"client_id"`
	Phone    string `json:"phone"`
	Balance  int64  `json:"balance"`
}

// handleAdminLookupClient returns a handler for GET
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/clients/lookup?phone=...
// Phone is masked for a caller whose only membership in this scope is
// support_viewer. Must run behind RequireStaff(clients.read, storeScopeFromPath).
func handleAdminLookupClient(stores domain.StoreRepository, clients domain.ClientRepository, balances domain.BalanceRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, orgID, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		phone := r.URL.Query().Get("phone")
		if phone == "" {
			writeError(w, http.StatusBadRequest, "phone is required")
			return
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

		masked := false
		if principal, ok := staffFromContext(r.Context()); ok {
			masked = principal.IsSupportViewerOnly(orgID, &store.ID)
		}

		writeJSON(w, http.StatusOK, adminClientResponseBody{
			ClientID: client.ID, Phone: maskPhoneIf(client.Phone, masked), Balance: balance.Points,
		})
	}
}

type adminTransactionResponseBody struct {
	ID           int64     `json:"id"`
	ExternalTxID string    `json:"external_tx_id"`
	Type         string    `json:"type"`
	PointsDelta  int64     `json:"points_delta"`
	BalanceAfter int64     `json:"balance_after"`
	CreatedAt    time.Time `json:"created_at"`
}

// handleAdminListClientTransactions returns a handler for GET
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/clients/{clientID}/transactions.
// 404s if clientID doesn't belong to storeID. Must run behind
// RequireStaff(transactions.read, storeScopeFromPath).
func handleAdminListClientTransactions(stores domain.StoreRepository, clients domain.ClientRepository, txs domain.TransactionRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, _, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		clientID, err := strconv.ParseInt(r.PathValue("clientID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid client id")
			return
		}

		client, err := clients.GetByID(r.Context(), clientID)
		if errors.Is(err, domain.ErrNotFound) || (err == nil && client.StoreID != store.ID) {
			writeError(w, http.StatusNotFound, "client not found")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "load client", "client_id", clientID, "error", err)
			writeError(w, http.StatusInternalServerError, "load client")
			return
		}

		list, err := txs.ListByClient(r.Context(), client.ID)
		if err != nil {
			log.ErrorContext(r.Context(), "list transactions", "client_id", client.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "list transactions")
			return
		}

		out := make([]adminTransactionResponseBody, 0, len(list))
		for _, tx := range list {
			out = append(out, adminTransactionResponseBody{
				ID: tx.ID, ExternalTxID: tx.ExternalTxID, Type: string(tx.Type),
				PointsDelta: tx.PointsDelta, BalanceAfter: tx.BalanceAfter, CreatedAt: tx.CreatedAt,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"transactions": out})
	}
}
