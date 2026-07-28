package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

type meResponseBody struct {
	CustomerAccountID int64     `json:"customer_account_id"`
	Phone             string    `json:"phone"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

// handleGetMe returns a handler for GET /api/v1/me. Must run behind
// RequireCustomerSession.
func handleGetMe(customers domain.CustomerAccountRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerAccountID, ok := customerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		account, err := customers.GetByID(r.Context(), customerAccountID)
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "load customer account", "customer_account_id", customerAccountID, "error", err)
			writeError(w, http.StatusInternalServerError, "load account")
			return
		}

		writeJSON(w, http.StatusOK, meResponseBody{
			CustomerAccountID: account.ID,
			Phone:             account.Phone,
			Status:            string(account.Status),
			CreatedAt:         account.CreatedAt,
		})
	}
}

type meBalanceItem struct {
	StoreID  int64 `json:"store_id"`
	ClientID int64 `json:"client_id"`
	Balance  int64 `json:"balance"`
}

// handleGetMyBalance returns a handler for GET /api/v1/me/balance. Returns
// one entry per store the customer has a linked Client in — a single entry
// for the current single-store MVP, a list once multi-store linking is in
// play. Must run behind RequireCustomerSession.
func handleGetMyBalance(clients domain.ClientRepository, balances domain.BalanceRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerAccountID, ok := customerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		myClients, err := clients.ListByCustomerAccount(r.Context(), customerAccountID)
		if err != nil {
			log.ErrorContext(r.Context(), "list clients for customer account", "customer_account_id", customerAccountID, "error", err)
			writeError(w, http.StatusInternalServerError, "load balance")
			return
		}

		items := make([]meBalanceItem, 0, len(myClients))
		for _, c := range myClients {
			balance, err := balances.Get(r.Context(), c.ID)
			if err != nil {
				log.ErrorContext(r.Context(), "load balance", "client_id", c.ID, "error", err)
				writeError(w, http.StatusInternalServerError, "load balance")
				return
			}
			items = append(items, meBalanceItem{StoreID: c.StoreID, ClientID: c.ID, Balance: balance.Points})
		}

		writeJSON(w, http.StatusOK, map[string]any{"balances": items})
	}
}

type meTransactionItem struct {
	StoreID      int64     `json:"store_id"`
	ClientID     int64     `json:"client_id"`
	Type         string    `json:"type"`
	PointsDelta  int64     `json:"points_delta"`
	BalanceAfter int64     `json:"balance_after"`
	CreatedAt    time.Time `json:"created_at"`
}

// handleGetMyTransactions returns a handler for GET
// /api/v1/me/transactions. Merges transaction history across every store
// the customer has a linked Client in, newest first. Must run behind
// RequireCustomerSession.
func handleGetMyTransactions(clients domain.ClientRepository, txs domain.TransactionRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerAccountID, ok := customerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		myClients, err := clients.ListByCustomerAccount(r.Context(), customerAccountID)
		if err != nil {
			log.ErrorContext(r.Context(), "list clients for customer account", "customer_account_id", customerAccountID, "error", err)
			writeError(w, http.StatusInternalServerError, "load transactions")
			return
		}

		var items []meTransactionItem
		for _, c := range myClients {
			clientTxs, err := txs.ListByClient(r.Context(), c.ID)
			if err != nil {
				log.ErrorContext(r.Context(), "list transactions", "client_id", c.ID, "error", err)
				writeError(w, http.StatusInternalServerError, "load transactions")
				return
			}
			for _, tx := range clientTxs {
				items = append(items, meTransactionItem{
					StoreID: tx.StoreID, ClientID: tx.ClientID, Type: string(tx.Type),
					PointsDelta: tx.PointsDelta, BalanceAfter: tx.BalanceAfter, CreatedAt: tx.CreatedAt,
				})
			}
		}

		sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })

		writeJSON(w, http.StatusOK, map[string]any{"transactions": items})
	}
}
