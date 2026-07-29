package httpserver

import (
	"net/http"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// contour is a route's authentication/authorization boundary — which
// middleware New wraps its handler with.
type contour string

const (
	// contourPublic routes require no credential at all (health/readiness,
	// and the customer auth endpoints that are themselves how a caller
	// obtains a credential).
	contourPublic contour = "public"
	// contourStoreKey routes require a 1C/POS store API key
	// (RequireStoreAPIKey + requireScope).
	contourStoreKey contour = "store_key"
	// contourCustomer routes require a customer access token
	// (RequireCustomerSession).
	contourCustomer contour = "customer"
	// contourStaff routes require a staff access token plus an RBAC
	// permission check (RequireStaff / RequireGlobalStaffPermission).
	contourStaff contour = "staff"
)

// staffScopeKind selects which of staffScope's path-resolvers RequireStaff
// should use for a contourStaff route. Unused when StaffGlobal is set.
type staffScopeKind string

const (
	staffScopeOrg   staffScopeKind = "org"
	staffScopeStore staffScopeKind = "store"
)

// Rate-limit profile names — see internal/ratelimit. A route's profile
// determines which principal/IP dimensions its requests are counted
// against; the zero value means "no dedicated limiter beyond what its
// contour's middleware already applies" (e.g. OTP's own Redis limiter).
const (
	rlProfileStaffLogin   = "staff_login"
	rlProfileAdmin        = "admin"
	rlProfileClientLookup = "client_lookup"
	rlProfileAccrual      = "accrual"
)

// routeMeta declares one HTTP operation's method, path, security contour,
// and (for scoped contours) the scope/permission it requires. It is the
// single source of truth New() registers handlers from and that
// openapi_parity_test.go checks against docs/openapi/openapi.yaml — every
// entry here must have a matching OpenAPI operation, and vice versa.
//
// routeMeta intentionally holds no handler funcs or deps: it stays a plain,
// comparable value so the parity test can inspect it without constructing a
// full Deps.
type routeMeta struct {
	Method      string
	Path        string
	OperationID string
	Contour     contour

	// APIKeyScope applies to contourStoreKey routes.
	APIKeyScope string

	// StaffPermission, StaffGlobal, and StaffScope apply to contourStaff
	// routes. StaffGlobal routes (organization creation) use
	// RequireGlobalStaffPermission and ignore StaffScope.
	StaffPermission domain.Permission
	StaffGlobal     bool
	StaffScope      staffScopeKind

	RateLimitProfile string
}

// routeTable is every HTTP operation this server exposes. Order matches
// server.go's historical registration order; it has no behavioral
// significance (http.ServeMux dispatches by method+pattern, not
// registration order) but keeps diffs readable.
var routeTable = []routeMeta{
	{Method: http.MethodGet, Path: "/healthz", OperationID: "getHealthz", Contour: contourPublic},
	{Method: http.MethodGet, Path: "/readyz", OperationID: "getReadyz", Contour: contourPublic},

	// --- 1C / POS service principal (store API key) ---
	{Method: http.MethodPost, Path: "/api/v1/transactions", OperationID: "createTransaction",
		Contour: contourStoreKey, APIKeyScope: domain.ScopeTransactionsWrite, RateLimitProfile: rlProfileAccrual},
	{Method: http.MethodPost, Path: "/api/v1/transactions/redeem", OperationID: "redeemTransaction",
		Contour: contourStoreKey, APIKeyScope: domain.ScopeTransactionsWrite, RateLimitProfile: rlProfileAccrual},
	{Method: http.MethodGet, Path: "/api/v1/clients/lookup", OperationID: "lookupClientByPhone",
		Contour: contourStoreKey, APIKeyScope: domain.ScopeClientsLookup, RateLimitProfile: rlProfileClientLookup},
	{Method: http.MethodGet, Path: "/api/v1/clients/{id}/balance", OperationID: "getClientBalance",
		Contour: contourStoreKey, APIKeyScope: domain.ScopeBalancesRead, RateLimitProfile: rlProfileClientLookup},

	// --- Mobile customer (OTP + sessions) ---
	{Method: http.MethodPost, Path: "/api/v1/auth/otp/request", OperationID: "requestOTP", Contour: contourPublic},
	{Method: http.MethodPost, Path: "/api/v1/auth/otp/verify", OperationID: "verifyOTP", Contour: contourPublic},
	{Method: http.MethodPost, Path: "/api/v1/auth/refresh", OperationID: "refreshCustomerSession", Contour: contourPublic},
	{Method: http.MethodPost, Path: "/api/v1/auth/logout", OperationID: "customerLogout", Contour: contourPublic},
	{Method: http.MethodPost, Path: "/api/v1/auth/logout-all", OperationID: "customerLogoutAll", Contour: contourCustomer},
	{Method: http.MethodGet, Path: "/api/v1/me", OperationID: "getMe", Contour: contourCustomer},
	{Method: http.MethodGet, Path: "/api/v1/me/balance", OperationID: "getMyBalance",
		Contour: contourCustomer, RateLimitProfile: rlProfileClientLookup},
	{Method: http.MethodGet, Path: "/api/v1/me/transactions", OperationID: "getMyTransactions", Contour: contourCustomer},

	// --- Retailer staff / platform admin (OIDC + RBAC) ---
	{Method: http.MethodPost, Path: "/api/v1/staff/auth/oidc", OperationID: "staffOIDCLogin",
		Contour: contourPublic, RateLimitProfile: rlProfileStaffLogin},

	{Method: http.MethodPost, Path: "/api/v1/admin/organizations", OperationID: "createOrganization",
		Contour: contourStaff, StaffGlobal: true, StaffPermission: domain.PermOrganizationsManage, RateLimitProfile: rlProfileAdmin},

	{Method: http.MethodGet, Path: "/api/v1/admin/organizations/{orgID}/stores/{storeID}", OperationID: "getStore",
		Contour: contourStaff, StaffPermission: domain.PermStoresRead, StaffScope: staffScopeStore, RateLimitProfile: rlProfileAdmin},
	{Method: http.MethodPost, Path: "/api/v1/admin/organizations/{orgID}/stores", OperationID: "createStore",
		Contour: contourStaff, StaffPermission: domain.PermStoresManage, StaffScope: staffScopeOrg, RateLimitProfile: rlProfileAdmin},

	{Method: http.MethodGet, Path: "/api/v1/admin/organizations/{orgID}/staff", OperationID: "listStaffMemberships",
		Contour: contourStaff, StaffPermission: domain.PermStaffManage, StaffScope: staffScopeOrg, RateLimitProfile: rlProfileAdmin},
	{Method: http.MethodPost, Path: "/api/v1/admin/organizations/{orgID}/staff", OperationID: "createStaffMembership",
		Contour: contourStaff, StaffPermission: domain.PermStaffManage, StaffScope: staffScopeOrg, RateLimitProfile: rlProfileAdmin},
	{Method: http.MethodPatch, Path: "/api/v1/admin/organizations/{orgID}/staff/{membershipID}", OperationID: "updateStaffMembership",
		Contour: contourStaff, StaffPermission: domain.PermStaffManage, StaffScope: staffScopeOrg, RateLimitProfile: rlProfileAdmin},

	{Method: http.MethodGet, Path: "/api/v1/admin/organizations/{orgID}/stores/{storeID}/clients/lookup", OperationID: "adminLookupClient",
		Contour: contourStaff, StaffPermission: domain.PermClientsRead, StaffScope: staffScopeStore, RateLimitProfile: rlProfileAdmin},
	{Method: http.MethodGet, Path: "/api/v1/admin/organizations/{orgID}/stores/{storeID}/clients/{clientID}/transactions", OperationID: "adminListClientTransactions",
		Contour: contourStaff, StaffPermission: domain.PermTransactionsRead, StaffScope: staffScopeStore, RateLimitProfile: rlProfileAdmin},

	{Method: http.MethodGet, Path: "/api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config", OperationID: "getLoyaltyConfig",
		Contour: contourStaff, StaffPermission: domain.PermLoyaltyConfigRead, StaffScope: staffScopeStore, RateLimitProfile: rlProfileAdmin},
	{Method: http.MethodPut, Path: "/api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config", OperationID: "putLoyaltyConfig",
		Contour: contourStaff, StaffPermission: domain.PermLoyaltyConfigWrite, StaffScope: staffScopeStore, RateLimitProfile: rlProfileAdmin},

	{Method: http.MethodGet, Path: "/api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys", OperationID: "listStoreAPIKeys",
		Contour: contourStaff, StaffPermission: domain.PermAPIKeysRead, StaffScope: staffScopeStore, RateLimitProfile: rlProfileAdmin},
	{Method: http.MethodPost, Path: "/api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys", OperationID: "createStoreAPIKey",
		Contour: contourStaff, StaffPermission: domain.PermAPIKeysRotate, StaffScope: staffScopeStore, RateLimitProfile: rlProfileAdmin},
	{Method: http.MethodPost, Path: "/api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys/{keyID}/revoke", OperationID: "revokeStoreAPIKey",
		Contour: contourStaff, StaffPermission: domain.PermAPIKeysRotate, StaffScope: staffScopeStore, RateLimitProfile: rlProfileAdmin},

	{Method: http.MethodGet, Path: "/api/v1/admin/organizations/{orgID}/audit", OperationID: "listAuditEvents",
		Contour: contourStaff, StaffPermission: domain.PermAuditRead, StaffScope: staffScopeOrg, RateLimitProfile: rlProfileAdmin},
}

// handlerFor builds the concrete, unwrapped business-logic handler for a
// routeTable entry's OperationID. New wraps the result with the middleware
// its Contour calls for.
func handlerFor(op string, deps Deps) http.HandlerFunc {
	switch op {
	case "getHealthz":
		return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	case "getReadyz":
		return func(w http.ResponseWriter, r *http.Request) {
			if err := deps.Ready(r.Context()); err != nil {
				writeError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			w.WriteHeader(http.StatusOK)
		}

	case "createTransaction":
		return handleAccrueTransaction(deps.Loyalty, deps.Log)
	case "redeemTransaction":
		return handleRedeemTransaction(deps.Loyalty, deps.Log)
	case "lookupClientByPhone":
		return handleLookupClientByPhone(deps.Clients, deps.Balances, deps.Log)
	case "getClientBalance":
		return handleGetClientBalance(deps.Clients, deps.Balances, deps.Log)

	case "requestOTP":
		return handleRequestOTP(deps.CustomerAuth, deps.Log)
	case "verifyOTP":
		return handleVerifyOTP(deps.CustomerAuth, deps.Log)
	case "refreshCustomerSession":
		return handleRefreshCustomerSession(deps.CustomerAuth, deps.Log)
	case "customerLogout":
		return handleCustomerLogout(deps.CustomerAuth, deps.Log)
	case "customerLogoutAll":
		return handleCustomerLogoutAll(deps.CustomerAuth, deps.Log)
	case "getMe":
		return handleGetMe(deps.CustomerAccounts, deps.Log)
	case "getMyBalance":
		return handleGetMyBalance(deps.Clients, deps.Balances, deps.Log)
	case "getMyTransactions":
		return handleGetMyTransactions(deps.Clients, deps.Transactions, deps.Log)

	case "staffOIDCLogin":
		return handleStaffOIDCLogin(deps.StaffAuth, deps.Log)
	case "createOrganization":
		return handleCreateOrganization(deps.Organizations, deps.Log)
	case "getStore":
		return handleGetStore(deps.Stores, deps.Log)
	case "createStore":
		return handleCreateStore(deps.Stores, deps.Log)
	case "listStaffMemberships":
		return handleListStaffMemberships(deps.StaffMemberships, deps.Log)
	case "createStaffMembership":
		return handleCreateStaffMembership(deps.StaffUsers, deps.StaffMemberships, deps.AuditEvents, deps.Log)
	case "updateStaffMembership":
		return handleUpdateStaffMembership(deps.StaffMemberships, deps.AuditEvents, deps.Log)
	case "adminLookupClient":
		return handleAdminLookupClient(deps.Stores, deps.Clients, deps.Balances, deps.Log)
	case "adminListClientTransactions":
		return handleAdminListClientTransactions(deps.Stores, deps.Clients, deps.Transactions, deps.Log)
	case "getLoyaltyConfig":
		return handleGetLoyaltyConfig(deps.Stores, deps.LoyaltyConfigs, deps.Log)
	case "putLoyaltyConfig":
		return handlePutLoyaltyConfig(deps.Stores, deps.LoyaltyConfigs, deps.Log)
	case "listStoreAPIKeys":
		return handleListStoreAPIKeys(deps.Stores, deps.StoreAPIKeys, deps.Log)
	case "createStoreAPIKey":
		return handleCreateStoreAPIKey(deps.Stores, deps.StoreAPIKeys, deps.AuditEvents, deps.Log)
	case "revokeStoreAPIKey":
		return handleRevokeStoreAPIKey(deps.Stores, deps.StoreAPIKeys, deps.AuditEvents, deps.Log)
	case "listAuditEvents":
		return handleListAuditEvents(deps.AuditEvents, deps.Log)

	default:
		panic("httpserver: no handler registered for operation " + op)
	}
}
