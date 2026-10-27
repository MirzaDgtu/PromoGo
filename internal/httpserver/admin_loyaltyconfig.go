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
	// ExpectedVersion, if set, rejects the write with 409 unless it's still
	// the store's current version (0 = no config yet).
	ExpectedVersion *int64 `json:"expected_version"`
}

const errConfigChanged = "loyalty config was changed by someone else; reload and try again"

// fitsNumeric reports whether d fits a Postgres NUMERIC(precision, scale)
// column exactly — without it, extra decimals would be silently rounded on
// write (the response would echo a value that isn't what got stored) and
// too many integer digits would fail the write with a 500.
func fitsNumeric(d decimal.Decimal, precision, scale int32) bool {
	if !d.Equal(d.Truncate(scale)) {
		return false
	}
	limit := decimal.New(1, precision-scale)
	return d.Abs().LessThan(limit)
}

// checkLoyaltyConfigColumns mirrors loyalty_configs' column types
// (migrations/sql/00005_create_loyalty_configs.sql).
func checkLoyaltyConfigColumns(b putLoyaltyConfigBody) string {
	switch {
	case !fitsNumeric(b.AccrualPercent, 5, 2):
		return "accrual_percent allows at most 2 decimal places"
	case !fitsNumeric(b.MinPurchaseAmount, 20, 2):
		return "min_purchase_amount allows at most 2 decimal places and 18 integer digits"
	case !fitsNumeric(b.MaxRedeemPercent, 5, 2):
		return "max_redeem_percent allows at most 2 decimal places"
	case !fitsNumeric(b.PointsExchangeRate, 10, 4):
		return "points_exchange_rate allows at most 4 decimal places and 6 integer digits"
	}
	return ""
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
			PointsExchangeRate: body.PointsExchangeRate, ExpectedVersion: body.ExpectedVersion,
		}
		if err := cfg.Validate(); err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if msg := checkLoyaltyConfigColumns(body); msg != "" {
			writeError(w, http.StatusUnprocessableEntity, msg)
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
		err := configs.Upsert(r.Context(), cfg, actorID)
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, http.StatusConflict, errConfigChanged)
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "upsert loyalty config", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "save loyalty config")
			return
		}
		auditCreateWithMetadata(r.Context(), audit, log, domain.AuditActorStaff, actorID, domain.AuditActionLoyaltyConfigChanged, &orgID, &store.ID, "loyalty_config", &store.ID,
			map[string]any{"version": cfg.Version}, clientIP(r), r.UserAgent())

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
	Version         int64  `json:"version"`
	ExpectedVersion *int64 `json:"expected_version"`
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
			PointsExchangeRate: target.PointsExchangeRate, ExpectedVersion: body.ExpectedVersion,
		}
		actorID := actorStaffUserID(r)
		err = configs.Upsert(r.Context(), cfg, actorID)
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, http.StatusConflict, errConfigChanged)
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "rollback loyalty config", "store_id", store.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "rollback loyalty config")
			return
		}
		auditCreateWithMetadata(r.Context(), audit, log, domain.AuditActorStaff, actorID, domain.AuditActionLoyaltyConfigRolledBack, &orgID, &store.ID, "loyalty_config", &store.ID,
			map[string]any{"version": cfg.Version, "restored_version": body.Version}, clientIP(r), r.UserAgent())

		writeJSON(w, http.StatusOK, loyaltyConfigToBody(cfg))
	}
}
