package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

type loyaltyConfigResponseBody struct {
	StoreID            int64  `json:"store_id"`
	Mechanic           string `json:"mechanic"`
	AccrualPercent     string `json:"accrual_percent"`
	MinPurchaseAmount  string `json:"min_purchase_amount"`
	MinBalanceToRedeem int64  `json:"min_balance_to_redeem"`
	MaxRedeemPercent   string `json:"max_redeem_percent"`
	PointsExchangeRate string `json:"points_exchange_rate"`
}

func loyaltyConfigToBody(cfg *domain.LoyaltyConfig) loyaltyConfigResponseBody {
	return loyaltyConfigResponseBody{
		StoreID: cfg.StoreID, Mechanic: cfg.Mechanic,
		AccrualPercent: cfg.AccrualPercent.String(), MinPurchaseAmount: cfg.MinPurchaseAmount.String(),
		MinBalanceToRedeem: cfg.MinBalanceToRedeem, MaxRedeemPercent: cfg.MaxRedeemPercent.String(),
		PointsExchangeRate: cfg.PointsExchangeRate.String(),
	}
}

// handleGetLoyaltyConfig returns a handler for GET
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config. Must
// run behind RequireStaff(loyalty_config.read, storeScopeFromPath).
func handleGetLoyaltyConfig(stores domain.StoreRepository, configs domain.LoyaltyConfigRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, _, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		cfg, err := configs.GetByStore(r.Context(), store.ID)
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "loyalty config not set")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "load loyalty config", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "load loyalty config")
			return
		}

		writeJSON(w, http.StatusOK, loyaltyConfigToBody(cfg))
	}
}

type putLoyaltyConfigBody struct {
	Mechanic           string          `json:"mechanic"`
	AccrualPercent     decimal.Decimal `json:"accrual_percent"`
	MinPurchaseAmount  decimal.Decimal `json:"min_purchase_amount"`
	MinBalanceToRedeem int64           `json:"min_balance_to_redeem"`
	MaxRedeemPercent   decimal.Decimal `json:"max_redeem_percent"`
	PointsExchangeRate decimal.Decimal `json:"points_exchange_rate"`
}

// handlePutLoyaltyConfig returns a handler for PUT
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config. Must
// run behind RequireStaff(loyalty_config.write, storeScopeFromPath).
func handlePutLoyaltyConfig(stores domain.StoreRepository, configs domain.LoyaltyConfigRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, _, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body putLoyaltyConfigBody
		if err := dec.Decode(&body); err != nil || body.Mechanic == "" ||
			body.AccrualPercent.IsNegative() || body.MinPurchaseAmount.IsNegative() ||
			body.MinBalanceToRedeem < 0 || body.MaxRedeemPercent.IsNegative() || body.PointsExchangeRate.IsNegative() {
			writeError(w, http.StatusBadRequest, "invalid loyalty config")
			return
		}

		cfg := &domain.LoyaltyConfig{
			StoreID: store.ID, Mechanic: body.Mechanic,
			AccrualPercent: body.AccrualPercent, MinPurchaseAmount: body.MinPurchaseAmount,
			MinBalanceToRedeem: body.MinBalanceToRedeem, MaxRedeemPercent: body.MaxRedeemPercent,
			PointsExchangeRate: body.PointsExchangeRate,
		}
		if err := configs.Upsert(r.Context(), cfg); err != nil {
			log.ErrorContext(r.Context(), "upsert loyalty config", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "save loyalty config")
			return
		}

		writeJSON(w, http.StatusOK, loyaltyConfigToBody(cfg))
	}
}
