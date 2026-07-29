package httpserver

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

const (
	defaultTransactionsLimit = 20
	maxTransactionsLimit     = 100
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
// the customer has a linked Client in, newest first, keyset-paginated via
// ?limit=&cursor= (see encodeTransactionCursor/decodeTransactionCursor).
// Must run behind RequireCustomerSession.
func handleGetMyTransactions(clients domain.ClientRepository, txs domain.TransactionRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerAccountID, ok := customerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		limit := parseTransactionsLimit(r.URL.Query().Get("limit"))
		cursor, err := decodeTransactionCursor(r.URL.Query().Get("cursor"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}

		myClients, err := clients.ListByCustomerAccount(r.Context(), customerAccountID)
		if err != nil {
			log.ErrorContext(r.Context(), "list clients for customer account", "customer_account_id", customerAccountID, "error", err)
			writeError(w, http.StatusInternalServerError, "load transactions")
			return
		}
		if len(myClients) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"transactions": []meTransactionItem{}})
			return
		}

		clientIDs := make([]int64, len(myClients))
		for i, c := range myClients {
			clientIDs[i] = c.ID
		}

		// Fetch one extra row to detect whether a next page exists.
		clientTxs, err := txs.ListByClientIDs(r.Context(), clientIDs, limit+1, cursor)
		if err != nil {
			log.ErrorContext(r.Context(), "list transactions", "customer_account_id", customerAccountID, "error", err)
			writeError(w, http.StatusInternalServerError, "load transactions")
			return
		}

		var nextCursor string
		if len(clientTxs) > limit {
			last := clientTxs[limit-1]
			nextCursor = encodeTransactionCursor(last.CreatedAt, last.ID)
			clientTxs = clientTxs[:limit]
		}

		items := make([]meTransactionItem, 0, len(clientTxs))
		for _, tx := range clientTxs {
			items = append(items, meTransactionItem{
				StoreID: tx.StoreID, ClientID: tx.ClientID, Type: string(tx.Type),
				PointsDelta: tx.PointsDelta, BalanceAfter: tx.BalanceAfter, CreatedAt: tx.CreatedAt,
			})
		}

		resp := map[string]any{"transactions": items}
		if nextCursor != "" {
			resp["next_cursor"] = nextCursor
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// parseTransactionsLimit parses the ?limit= query param, defaulting and
// clamping rather than rejecting — it's a display knob, not input that can
// corrupt state, so a malformed or out-of-range value just falls back to a
// sane bound instead of failing the request.
func parseTransactionsLimit(raw string) int {
	if raw == "" {
		return defaultTransactionsLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultTransactionsLimit
	}
	if n > maxTransactionsLimit {
		return maxTransactionsLimit
	}
	return n
}

// encodeTransactionCursor and decodeTransactionCursor turn a
// domain.TransactionCursor into an opaque base64 string safe to hand to
// mobile clients and back, in the form "<created_at_unix_nano>_<id>".
func encodeTransactionCursor(createdAt time.Time, id int64) string {
	raw := fmt.Sprintf("%d_%d", createdAt.UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeTransactionCursor returns (nil, nil) for an empty raw cursor (the
// first page), or an error if raw is non-empty but malformed — a bad
// cursor must fail loudly rather than silently restart from the top or
// skip data.
func decodeTransactionCursor(raw string) (*domain.TransactionCursor, error) {
	if raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}
	parts := strings.SplitN(string(decoded), "_", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed cursor")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("malformed cursor timestamp: %w", err)
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("malformed cursor id: %w", err)
	}
	return &domain.TransactionCursor{CreatedAt: time.Unix(0, nanos), ID: id}, nil
}
