package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/mechanicbuild"
)

type loyaltyConfigResponseBody struct {
	StoreID            int64  `json:"store_id"`
	Mechanic           string `json:"mechanic"`
	AccrualPercent     string `json:"accrual_percent"`
	MinPurchaseAmount  string `json:"min_purchase_amount"`
	MinBalanceToRedeem int64  `json:"min_balance_to_redeem"`
	MaxRedeemPercent   string `json:"max_redeem_percent"`
	PointsExchangeRate string `json:"points_exchange_rate"`
	Version            int64  `json:"version"`
}

func loyaltyConfigToBody(cfg *domain.LoyaltyConfig) loyaltyConfigResponseBody {
	return loyaltyConfigResponseBody{
		StoreID: cfg.StoreID, Mechanic: cfg.Mechanic,
		AccrualPercent: cfg.AccrualPercent.String(), MinPurchaseAmount: cfg.MinPurchaseAmount.String(),
		MinBalanceToRedeem: cfg.MinBalanceToRedeem, MaxRedeemPercent: cfg.MaxRedeemPercent.String(),
		PointsExchangeRate: cfg.PointsExchangeRate.String(), Version: cfg.Version,
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

// actorStaffUserID extracts the current request's staff principal's
// StaffUserID for attribution (audit events, loyalty_config_history's
// changed_by_staff_user_id), or nil if no staff principal is present.
func actorStaffUserID(r *http.Request) *int64 {
	principal, ok := staffFromContext(r.Context())
	if !ok || principal == nil {
		return nil
	}
	id := principal.StaffUserID
	return &id
}

// handlePutLoyaltyConfig returns a handler for PUT
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config. Must
// run behind RequireStaff(loyalty_config.write, storeScopeFromPath). Every
// write is versioned (see domain.LoyaltyConfigRepository.Upsert) and
// audited.
func handlePutLoyaltyConfig(stores domain.StoreRepository, configs domain.LoyaltyConfigRepository, audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, orgID, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body putLoyaltyConfigBody
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid loyalty config")
			return
		}

		cfg := &domain.LoyaltyConfig{
			StoreID: store.ID, Mechanic: body.Mechanic,
			AccrualPercent: body.AccrualPercent, MinPurchaseAmount: body.MinPurchaseAmount,
			MinBalanceToRedeem: body.MinBalanceToRedeem, MaxRedeemPercent: body.MaxRedeemPercent,
			PointsExchangeRate: body.PointsExchangeRate,
		}
		if err := cfg.Validate(); err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		// Mechanic enum membership is internal/mechanicbuild's source of
		// truth (the switch that Accrue/Redeem build a domain.Mechanic
		// from) — reject an unknown mechanic here, at config-write time,
		// rather than letting it surface as an accrual-time 500 later.
		if _, err := mechanicbuild.Build(cfg.Mechanic); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "unknown mechanic: "+cfg.Mechanic)
			return
		}

		actorID := actorStaffUserID(r)
		if err := configs.Upsert(r.Context(), cfg, actorID); err != nil {
			log.ErrorContext(r.Context(), "upsert loyalty config", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "save loyalty config")
			return
		}
		auditCreate(r.Context(), audit, log, domain.AuditActorStaff, actorID, domain.AuditActionLoyaltyConfigChanged, &orgID, &store.ID, "loyalty_config", &store.ID, clientIP(r), r.UserAgent())

		writeJSON(w, http.StatusOK, loyaltyConfigToBody(cfg))
	}
}

type loyaltyConfigVersionResponseBody struct {
	Version              int64  `json:"version"`
	Mechanic             string `json:"mechanic"`
	AccrualPercent       string `json:"accrual_percent"`
	MinPurchaseAmount    string `json:"min_purchase_amount"`
	MinBalanceToRedeem   int64  `json:"min_balance_to_redeem"`
	MaxRedeemPercent     string `json:"max_redeem_percent"`
	PointsExchangeRate   string `json:"points_exchange_rate"`
	ChangedByStaffUserID *int64 `json:"changed_by_staff_user_id,omitempty"`
	CreatedAt            string `json:"created_at"`
}

func loyaltyConfigVersionToBody(v *domain.LoyaltyConfigVersion) loyaltyConfigVersionResponseBody {
	return loyaltyConfigVersionResponseBody{
		Version: v.Version, Mechanic: v.Mechanic,
		AccrualPercent: v.AccrualPercent.String(), MinPurchaseAmount: v.MinPurchaseAmount.String(),
		MinBalanceToRedeem: v.MinBalanceToRedeem, MaxRedeemPercent: v.MaxRedeemPercent.String(),
		PointsExchangeRate: v.PointsExchangeRate.String(), ChangedByStaffUserID: v.ChangedByStaffUserID,
		CreatedAt: v.CreatedAt.Format(time.RFC3339),
	}
}

// handleListLoyaltyConfigHistory returns a handler for GET
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config/history,
// newest version first. Must run behind RequireStaff(loyalty_config.read, storeScopeFromPath).
func handleListLoyaltyConfigHistory(stores domain.StoreRepository, configs domain.LoyaltyConfigRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, _, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		history, err := configs.ListHistory(r.Context(), store.ID)
		if err != nil {
			log.ErrorContext(r.Context(), "list loyalty config history", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "list loyalty config history")
			return
		}

		out := make([]loyaltyConfigVersionResponseBody, 0, len(history))
		for _, v := range history {
			out = append(out, loyaltyConfigVersionToBody(v))
		}
		writeJSON(w, http.StatusOK, map[string]any{"history": out})
	}
}

type rollbackLoyaltyConfigBody struct {
	Version int64 `json:"version"`
}

// handleRollbackLoyaltyConfig returns a handler for POST
// /api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config/rollback.
// It re-applies a past version's values through the same Upsert path
// handlePutLoyaltyConfig uses, which assigns a new version number and
// appends a new history entry — rollback is forward-only, never rewriting
// history, matching git-revert semantics. Must run behind
// RequireStaff(loyalty_config.write, storeScopeFromPath).
func handleRollbackLoyaltyConfig(stores domain.StoreRepository, configs domain.LoyaltyConfigRepository, audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, orgID, ok := resolveScopedStore(w, r, stores, log)
		if !ok {
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body rollbackLoyaltyConfigBody
		if err := dec.Decode(&body); err != nil || body.Version <= 0 {
			writeError(w, http.StatusBadRequest, "version is required")
			return
		}

		history, err := configs.ListHistory(r.Context(), store.ID)
		if err != nil {
			log.ErrorContext(r.Context(), "list loyalty config history", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "list loyalty config history")
			return
		}
		var target *domain.LoyaltyConfigVersion
		for _, v := range history {
			if v.Version == body.Version {
				target = v
				break
			}
		}
		if target == nil {
			writeError(w, http.StatusNotFound, "version not found")
			return
		}

		cfg := &domain.LoyaltyConfig{
			StoreID: store.ID, Mechanic: target.Mechanic,
			AccrualPercent: target.AccrualPercent, MinPurchaseAmount: target.MinPurchaseAmount,
			MinBalanceToRedeem: target.MinBalanceToRedeem, MaxRedeemPercent: target.MaxRedeemPercent,
			PointsExchangeRate: target.PointsExchangeRate,
		}
		actorID := actorStaffUserID(r)
		if err := configs.Upsert(r.Context(), cfg, actorID); err != nil {
			log.ErrorContext(r.Context(), "rollback loyalty config", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "rollback loyalty config")
			return
		}
		auditCreate(r.Context(), audit, log, domain.AuditActorStaff, actorID, domain.AuditActionLoyaltyConfigRolledBack, &orgID, &store.ID, "loyalty_config", &store.ID, clientIP(r), r.UserAgent())

		writeJSON(w, http.StatusOK, loyaltyConfigToBody(cfg))
	}
}
