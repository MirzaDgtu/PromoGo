// Package httpserver provides PromoGo's HTTP server: health/readiness
// endpoints, the store-scoped 1C webhook API (accrual, redemption, balance
// lookup), the mobile customer auth/profile API, and the staff/admin API.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/ratelimit"
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

	// RateLimiter backs the distributed rate-limit profiles applied per
	// routeTable entry (see ratelimit_rules.go). Nil disables rate limiting
	// entirely — a deployment that simply hasn't configured one, distinct
	// from a configured limiter whose Redis backend is unreachable at
	// request time (which fails closed with 503; see ratelimit.Middleware.Wrap).
	RateLimiter    *ratelimit.Limiter
	RateLimit      config.RateLimitConfig
	TrustedProxies []netip.Prefix

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

	requireStoreKey := RequireStoreAPIKey(deps.Stores, deps.StoreAPIKeys, deps.Log)
	requireCustomer := RequireCustomerSession(deps.CustomerAccessTokenSecret)
	rl := ratelimit.NewMiddleware(deps.RateLimiter, deps.Log)

	for _, route := range routeTable {
		h := handlerFor(route.OperationID, deps)

		// Post-auth rate-limit rules (by staff user, store API key, ...)
		// need the principal that auth middleware is about to set in the
		// request context, so they wrap the handler *before* that
		// middleware is applied below — see rateLimitRulesFor's doc comment
		// on this ordering.
		preAuth, postAuth := rateLimitRulesFor(route, deps.RateLimit, deps.TrustedProxies)
		h = rl.Wrap(route.RateLimitProfile, postAuth...)(h)

		switch route.Contour {
		case contourPublic:
			// No auth middleware.
		case contourStoreKey:
			h = requireStoreKey(requireScope(route.APIKeyScope)(h))
		case contourCustomer:
			h = requireCustomer(h)
		case contourStaff:
			if route.StaffGlobal {
				h = RequireGlobalStaffPermission(deps.StaffAccessTokenSecret, deps.StaffAuth, route.StaffPermission)(h)
			} else {
				scopeFn := storeScopeFromPath
				if route.StaffScope == staffScopeOrg {
					scopeFn = orgScopeFromPath
				}
				h = RequireStaff(deps.StaffAccessTokenSecret, deps.StaffAuth, route.StaffPermission, scopeFn)(h)
			}
		}

		// Pre-auth rate-limit rules (by caller IP) wrap outermost, so a
		// flood of invalid credentials never even reaches the (comparatively
		// expensive) token/JWKS verification or DB principal lookup below.
		h = rl.Wrap(route.RateLimitProfile, preAuth...)(h)

		mux.HandleFunc(route.Method+" "+route.Path, h)
	}

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
