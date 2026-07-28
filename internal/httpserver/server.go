// Package httpserver provides PromoGo's HTTP server: health/readiness
// endpoints and the store-scoped 1C webhook API (accrual, redemption,
// balance lookup).
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/service"
)

// Deps holds the dependencies for New.
type Deps struct {
	App  config.AppConfig
	HTTP config.HTTPConfig
	Log  *slog.Logger

	Stores   domain.StoreRepository
	Clients  domain.ClientRepository
	Balances domain.BalanceRepository

	Loyalty *service.LoyaltyService

	// Ready is called on each /readyz request and should return an error if
	// any dependency is unavailable.
	Ready func(ctx context.Context) error
}

// New builds the application's HTTP server.
//
// Unauthenticated: GET /healthz, GET /readyz.
//
// Store-scoped (Authorization: Bearer <store-api-key>, see
// RequireStoreAPIKey): POST /api/v1/transactions (1C accrual webhook), POST
// /api/v1/transactions/redeem, GET /api/v1/clients/{id}/balance.
func New(deps Deps) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := deps.Ready(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	requireStoreKey := RequireStoreAPIKey(deps.Stores)

	mux.HandleFunc("POST /api/v1/transactions", requireStoreKey(handleAccrueTransaction(deps.Loyalty, deps.Log)))
	mux.HandleFunc("POST /api/v1/transactions/redeem", requireStoreKey(handleRedeemTransaction(deps.Loyalty, deps.Log)))
	mux.HandleFunc("GET /api/v1/clients/{id}/balance", requireStoreKey(handleGetClientBalance(deps.Clients, deps.Balances, deps.Log)))

	var handler http.Handler = mux
	handler = loggingMW(deps.Log)(handler)
	handler = http.MaxBytesHandler(handler, 1<<20)

	return &http.Server{
		Addr:              deps.HTTP.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		// 1C's webhook SLA target is <300ms (Idea.md); a generous timeout
		// here still bounds worst-case connection lifetime without cutting
		// off a slow-but-healthy request.
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}
