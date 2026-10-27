package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

type activityWindowBody struct {
	Operations     int64  `json:"operations"`
	PointsAccrued  int64  `json:"points_accrued"`
	PointsRedeemed int64  `json:"points_redeemed"`
	SalesAmount    string `json:"sales_amount"`
}

type storeOverviewBody struct {
	StoreID       int64 `json:"store_id"`
	LoyaltyConfig struct {
		Configured bool   `json:"configured"`
		Version    *int64 `json:"version,omitempty"`
	} `json:"loyalty_config"`
	// APIKeys is omitted for callers without api_keys.read (store_manager),
	// same as the API keys screen itself.
	APIKeys *struct {
		Active     int        `json:"active"`
		LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	} `json:"api_keys,omitempty"`
	Clients           int64              `json:"clients"`
	LastTransactionAt *time.Time         `json:"last_transaction_at,omitempty"`
	Last24h           activityWindowBody `json:"last_24h"`
	Last7d            activityWindowBody `json:"last_7d"`
}

func windowToBody(w domain.ActivityWindow) activityWindowBody {
	return activityWindowBody{
		Operations: w.Operations, PointsAccrued: w.PointsAccrued, PointsRedeemed: w.PointsRedeemed,
		SalesAmount: w.SalesAmount.String(),
	}
}

// handleGetStoreOverview returns a handler for GET
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/overview — the
// onboarding checklist and recent traffic the overview screen shows
// (DEC-031): is the loyalty program configured, does the store have a
// working cash-register key and when was it last used, and how much
// loyalty traffic went through it. Must run behind
// RequireStaff(stores.read, storeScopeFromPath).
func handleGetStoreOverview(stores domain.StoreRepository, configs domain.LoyaltyConfigRepository, keys domain.StoreAPIKeyRepository, activity domain.StoreActivityRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, orgID, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}
		now := time.Now()
		resp := storeOverviewBody{StoreID: store.ID}

		cfg, err := configs.GetByStore(r.Context(), store.ID)
		switch {
		case errors.Is(err, domain.ErrNotFound):
		case err != nil:
			log.ErrorContext(r.Context(), "load loyalty config", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "load overview")
			return
		default:
			resp.LoyaltyConfig.Configured = true
			resp.LoyaltyConfig.Version = &cfg.Version
		}

		if principal, ok := staffFromContext(r.Context()); ok && principal.HasPermission(domain.PermAPIKeysRead, orgID, &store.ID) {
			list, err := keys.ListByStore(r.Context(), store.ID)
			if err != nil {
				log.ErrorContext(r.Context(), "list api keys", "store_id", store.ID, "error", err)
				writeError(w, http.StatusInternalServerError, "load overview")
				return
			}
			resp.APIKeys = &struct {
				Active     int        `json:"active"`
				LastUsedAt *time.Time `json:"last_used_at,omitempty"`
			}{}
			for _, k := range list {
				if k.RevokedAt == nil && (k.ExpiresAt == nil || k.ExpiresAt.After(now)) {
					resp.APIKeys.Active++
				}
				// Last use of any key, revoked ones included: "when did this
				// store's register last talk to us" is the question.
				if k.LastUsedAt != nil && (resp.APIKeys.LastUsedAt == nil || k.LastUsedAt.After(*resp.APIKeys.LastUsedAt)) {
					resp.APIKeys.LastUsedAt = k.LastUsedAt
				}
			}
		}

		a, err := activity.StoreActivity(r.Context(), store.ID, now)
		if err != nil {
			log.ErrorContext(r.Context(), "store activity", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "load overview")
			return
		}
		resp.Clients = a.Clients
		resp.LastTransactionAt = a.LastTransactionAt
		resp.Last24h = windowToBody(a.Last24h)
		resp.Last7d = windowToBody(a.Last7d)

		writeJSON(w, http.StatusOK, resp)
	}
}
