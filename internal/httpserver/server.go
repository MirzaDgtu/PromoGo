// Package httpserver provides PromoGo's HTTP server: health/readiness
// endpoints, the store-scoped 1C webhook API (accrual, redemption, balance
// lookup), the mobile customer auth/profile API, and the staff/admin API.
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

	Stores         domain.StoreRepository
	Clients        domain.ClientRepository
	Balances       domain.BalanceRepository
	Transactions   domain.TransactionRepository
	LoyaltyConfigs domain.LoyaltyConfigRepository
	StoreAPIKeys   domain.StoreAPIKeyRepository

	Organizations    domain.OrganizationRepository
	CustomerAccounts domain.CustomerAccountRepository
	StaffUsers       domain.StaffUserRepository
	StaffMemberships domain.StaffMembershipRepository
	AuditEvents      domain.AuditEventRepository

	Loyalty      *service.LoyaltyService
	CustomerAuth *service.CustomerAuthService
	StaffAuth    *service.StaffAuthService

	// CustomerAccessTokenSecret and StaffAccessTokenSecret verify the
	// respective access tokens (see internal/auth). Both may be the same
	// underlying secret — the token's embedded type claim, not secret
	// separation, is what prevents a customer token from being accepted as
	// a staff token or vice versa (see internal/auth/tokens.go).
	CustomerAccessTokenSecret []byte
	StaffAccessTokenSecret    []byte

	// Ready is called on each /readyz request and should return an error if
	// any dependency is unavailable.
	Ready func(ctx context.Context) error
}

// New builds the application's HTTP server.
//
// Unauthenticated: GET /healthz, GET /readyz, POST /api/v1/auth/otp/request,
// POST /api/v1/auth/otp/verify, POST /api/v1/auth/refresh, POST
// /api/v1/auth/logout, POST /api/v1/staff/auth/oidc.
//
// Store-scoped (Authorization: Bearer <store-api-key>, see
// RequireStoreAPIKey): POST /api/v1/transactions (1C accrual webhook), POST
// /api/v1/transactions/redeem, GET /api/v1/clients/lookup?phone=...,
// GET /api/v1/clients/{id}/balance.
//
// Customer-scoped (Authorization: Bearer <customer-access-token>, see
// RequireCustomerSession): POST /api/v1/auth/logout-all, GET /api/v1/me,
// GET /api/v1/me/balance, GET /api/v1/me/transactions.
//
// Staff-scoped (Authorization: Bearer <staff-access-token>, see
// RequireStaff): POST /api/v1/admin/organizations, and everything under
// /api/v1/admin/organizations/{orgID}/...
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

	// --- 1C / POS service principal (store API key) ---

	requireStoreKey := RequireStoreAPIKey(deps.Stores, deps.StoreAPIKeys, deps.Log)

	mux.HandleFunc("POST /api/v1/transactions",
		requireStoreKey(requireScope(domain.ScopeTransactionsWrite)(handleAccrueTransaction(deps.Loyalty, deps.Log))))
	mux.HandleFunc("POST /api/v1/transactions/redeem",
		requireStoreKey(requireScope(domain.ScopeTransactionsWrite)(handleRedeemTransaction(deps.Loyalty, deps.Log))))
	mux.HandleFunc("GET /api/v1/clients/lookup",
		requireStoreKey(requireScope(domain.ScopeClientsLookup)(handleLookupClientByPhone(deps.Clients, deps.Balances, deps.Log))))
	mux.HandleFunc("GET /api/v1/clients/{id}/balance",
		requireStoreKey(requireScope(domain.ScopeBalancesRead)(handleGetClientBalance(deps.Clients, deps.Balances, deps.Log))))

	// --- Mobile customer (OTP + sessions) ---

	requireCustomer := RequireCustomerSession(deps.CustomerAccessTokenSecret)

	mux.HandleFunc("POST /api/v1/auth/otp/request", handleRequestOTP(deps.CustomerAuth, deps.Log))
	mux.HandleFunc("POST /api/v1/auth/otp/verify", handleVerifyOTP(deps.CustomerAuth, deps.Log))
	mux.HandleFunc("POST /api/v1/auth/refresh", handleRefreshCustomerSession(deps.CustomerAuth, deps.Log))
	mux.HandleFunc("POST /api/v1/auth/logout", handleCustomerLogout(deps.CustomerAuth, deps.Log))
	mux.HandleFunc("POST /api/v1/auth/logout-all", requireCustomer(handleCustomerLogoutAll(deps.CustomerAuth, deps.Log)))
	mux.HandleFunc("GET /api/v1/me", requireCustomer(handleGetMe(deps.CustomerAccounts, deps.Log)))
	mux.HandleFunc("GET /api/v1/me/balance", requireCustomer(handleGetMyBalance(deps.Clients, deps.Balances, deps.Log)))
	mux.HandleFunc("GET /api/v1/me/transactions", requireCustomer(handleGetMyTransactions(deps.Clients, deps.Transactions, deps.Log)))

	// --- Retailer staff / platform admin (OIDC + RBAC) ---

	mux.HandleFunc("POST /api/v1/staff/auth/oidc", handleStaffOIDCLogin(deps.StaffAuth, deps.Log))

	requireStaff := func(perm domain.Permission, scope func(*http.Request) (staffScope, error)) func(http.HandlerFunc) http.HandlerFunc {
		return RequireStaff(deps.StaffAccessTokenSecret, deps.StaffAuth, perm, scope)
	}
	scopeFromStaffMembershipPath := func(r *http.Request) (staffScope, error) { return orgScopeFromPath(r) }

	mux.HandleFunc("POST /api/v1/admin/organizations",
		RequireGlobalStaffPermission(deps.StaffAccessTokenSecret, deps.StaffAuth, domain.PermOrganizationsManage)(
			handleCreateOrganization(deps.Organizations, deps.Log)))

	mux.HandleFunc("GET /api/v1/admin/organizations/{orgID}/stores/{storeID}",
		requireStaff(domain.PermStoresRead, storeScopeFromPath)(handleGetStore(deps.Stores, deps.Log)))
	mux.HandleFunc("POST /api/v1/admin/organizations/{orgID}/stores",
		requireStaff(domain.PermStoresManage, orgScopeFromPath)(handleCreateStore(deps.Stores, deps.Log)))

	mux.HandleFunc("GET /api/v1/admin/organizations/{orgID}/staff",
		requireStaff(domain.PermStaffManage, scopeFromStaffMembershipPath)(handleListStaffMemberships(deps.StaffMemberships, deps.Log)))
	mux.HandleFunc("POST /api/v1/admin/organizations/{orgID}/staff",
		requireStaff(domain.PermStaffManage, scopeFromStaffMembershipPath)(handleCreateStaffMembership(deps.StaffUsers, deps.StaffMemberships, deps.AuditEvents, deps.Log)))
	mux.HandleFunc("PATCH /api/v1/admin/organizations/{orgID}/staff/{membershipID}",
		requireStaff(domain.PermStaffManage, scopeFromStaffMembershipPath)(handleUpdateStaffMembership(deps.StaffMemberships, deps.AuditEvents, deps.Log)))

	mux.HandleFunc("GET /api/v1/admin/organizations/{orgID}/stores/{storeID}/clients/lookup",
		requireStaff(domain.PermClientsRead, storeScopeFromPath)(handleAdminLookupClient(deps.Stores, deps.Clients, deps.Balances, deps.Log)))
	mux.HandleFunc("GET /api/v1/admin/organizations/{orgID}/stores/{storeID}/clients/{clientID}/transactions",
		requireStaff(domain.PermTransactionsRead, storeScopeFromPath)(handleAdminListClientTransactions(deps.Stores, deps.Clients, deps.Transactions, deps.Log)))

	mux.HandleFunc("GET /api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config",
		requireStaff(domain.PermLoyaltyConfigRead, storeScopeFromPath)(handleGetLoyaltyConfig(deps.Stores, deps.LoyaltyConfigs, deps.Log)))
	mux.HandleFunc("PUT /api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config",
		requireStaff(domain.PermLoyaltyConfigWrite, storeScopeFromPath)(handlePutLoyaltyConfig(deps.Stores, deps.LoyaltyConfigs, deps.Log)))

	mux.HandleFunc("GET /api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys",
		requireStaff(domain.PermAPIKeysRead, storeScopeFromPath)(handleListStoreAPIKeys(deps.Stores, deps.StoreAPIKeys, deps.Log)))
	mux.HandleFunc("POST /api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys",
		requireStaff(domain.PermAPIKeysRotate, storeScopeFromPath)(handleCreateStoreAPIKey(deps.Stores, deps.StoreAPIKeys, deps.AuditEvents, deps.Log)))
	mux.HandleFunc("POST /api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys/{keyID}/revoke",
		requireStaff(domain.PermAPIKeysRotate, storeScopeFromPath)(handleRevokeStoreAPIKey(deps.Stores, deps.StoreAPIKeys, deps.AuditEvents, deps.Log)))

	mux.HandleFunc("GET /api/v1/admin/organizations/{orgID}/audit",
		requireStaff(domain.PermAuditRead, orgScopeFromPath)(handleListAuditEvents(deps.AuditEvents, deps.Log)))

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
