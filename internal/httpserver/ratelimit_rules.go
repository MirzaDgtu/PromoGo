package httpserver

import (
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/ratelimit"
)

// ParseTrustedProxies parses HTTPConfig.TrustedProxies (CIDR strings) into
// netip.Prefix values once at startup. A malformed entry fails startup
// outright rather than being silently dropped — a typo here would
// otherwise silently narrow or misconfigure which peers are trusted to set
// X-Forwarded-For for every IP-scoped audit log, OTP limiter, and rate
// limiter (see resolveClientIP), with no visible error.
func ParseTrustedProxies(cidrs []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		p, err := netip.ParsePrefix(strings.TrimSpace(c))
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", c, err)
		}
		prefixes = append(prefixes, p)
	}
	return prefixes, nil
}

// ipRule rate-limits by caller IP, hashed via auth.HashOpaqueToken so a raw
// IP never appears in a Redis key or log line. Uses clientIP — the same
// trust-resolved IP used for audit logging and OTP throttling (see
// remoteaddr.go) — so all three can never disagree about who the caller is.
func ipRule(name string, limit int, window time.Duration) ratelimit.Rule {
	return ratelimit.Rule{
		Name: name, Limit: limit, Window: window,
		KeyFunc: func(r *http.Request) (string, bool) {
			return "ip:" + auth.HashOpaqueToken(clientIP(r)), true
		},
	}
}

// staffUserRule rate-limits by the authenticated staff principal's
// StaffUserID (not a secret — safe to use raw). Only applies once
// RequireStaff/RequireGlobalStaffPermission has already resolved a
// principal into the request context.
func staffUserRule(name string, limit int, window time.Duration) ratelimit.Rule {
	return ratelimit.Rule{
		Name: name, Limit: limit, Window: window,
		KeyFunc: func(r *http.Request) (string, bool) {
			principal, ok := staffFromContext(r.Context())
			if !ok {
				return "", false
			}
			return "staff:" + strconv.FormatInt(principal.StaffUserID, 10), true
		},
	}
}

// storeAPIKeyOrStoreRule rate-limits by the resolved StoreAPIKey's
// non-secret KeyID (a loggable prefix, see domain.StoreAPIKey), or by
// StoreID for a request authenticated via the legacy single-key column
// (which has no per-key id — see storeAPIKeyFromContext).
func storeAPIKeyOrStoreRule(name string, limit int, window time.Duration) ratelimit.Rule {
	return ratelimit.Rule{
		Name: name, Limit: limit, Window: window,
		KeyFunc: func(r *http.Request) (string, bool) {
			apiKey, ok := storeAPIKeyFromContext(r.Context())
			if !ok {
				return "", false
			}
			if apiKey != nil {
				return "key:" + apiKey.KeyID, true
			}
			if store, ok := storeFromContext(r.Context()); ok {
				return "store:" + strconv.FormatInt(store.ID, 10), true
			}
			return "", false
		},
	}
}

// principalRule rate-limits the customer-facing client-lookup/balance
// routes: by the resolved store API key/store (1C/POS callers) or by the
// authenticated customer's CustomerAccountID (the mobile app's own
// /me/balance) — whichever contour actually authenticated the request.
func principalRule(name string, limit int, window time.Duration) ratelimit.Rule {
	return ratelimit.Rule{
		Name: name, Limit: limit, Window: window,
		KeyFunc: func(r *http.Request) (string, bool) {
			if apiKey, ok := storeAPIKeyFromContext(r.Context()); ok {
				if apiKey != nil {
					return "key:" + apiKey.KeyID, true
				}
				if store, ok := storeFromContext(r.Context()); ok {
					return "store:" + strconv.FormatInt(store.ID, 10), true
				}
			}
			if customerID, ok := customerFromContext(r.Context()); ok {
				return "customer:" + strconv.FormatInt(customerID, 10), true
			}
			return "", false
		},
	}
}

// phoneQueryRule rate-limits by the normalized, hashed ?phone= query
// parameter — anti-enumeration on the client-lookup-by-phone route. Skips
// (applies=false) on any request with no phone parameter, so registering
// it as part of the client_lookup profile is safe for routes that don't
// take one (GET .../balance, GET /me/balance).
func phoneQueryRule(name string, limit int, window time.Duration) ratelimit.Rule {
	return ratelimit.Rule{
		Name: name, Limit: limit, Window: window,
		KeyFunc: func(r *http.Request) (string, bool) {
			phone := r.URL.Query().Get("phone")
			if phone == "" {
				return "", false
			}
			if normalized, err := auth.NormalizePhone(phone); err == nil {
				phone = normalized
			}
			return "phone:" + auth.HashOpaqueToken(phone), true
		},
	}
}

// rateLimitRulesFor returns route's pre-auth rules (checked before any
// credential is verified — outermost in New's middleware chain) and
// post-auth rules (checked once a principal is in context — innermost,
// right before the handler). A route with no RateLimitProfile gets no
// rules at all.
func rateLimitRulesFor(route routeMeta, cfg config.RateLimitConfig) (pre, post []ratelimit.Rule) {
	switch route.RateLimitProfile {
	case rlProfileStaffLogin:
		pre = []ratelimit.Rule{
			ipRule("ip", cfg.StaffLoginIPLimit, cfg.StaffLoginIPWindow),
		}
	case rlProfileAdmin:
		pre = []ratelimit.Rule{
			ipRule("ip", cfg.AdminIPLimit, cfg.AdminIPWindow),
		}
		post = []ratelimit.Rule{
			staffUserRule("staff", cfg.AdminStaffLimit, cfg.AdminStaffWindow),
		}
	case rlProfileClientLookup:
		post = []ratelimit.Rule{
			ipRule("ip", cfg.ClientLookupIPLimit, cfg.ClientLookupIPWindow),
			principalRule("principal", cfg.ClientLookupPrincipalLimit, cfg.ClientLookupPrincipalWindow),
			phoneQueryRule("phone", cfg.ClientLookupPhoneLimit, cfg.ClientLookupPhoneWindow),
		}
	case rlProfileAccrual:
		pre = []ratelimit.Rule{
			ipRule("ip", cfg.AccrualIPLimit, cfg.AccrualIPWindow),
		}
		post = []ratelimit.Rule{
			storeAPIKeyOrStoreRule("principal", cfg.AccrualPrincipalLimit, cfg.AccrualPrincipalWindow),
		}
	case rlProfileQRResolve:
		// Reuses the client-lookup profile's limits (DEC-011: QR resolve is
		// a brute-force-guessing target much like phone lookup) — the
		// QR-specific one-time-use and per-store consume cooldown are
		// enforced separately, inside service.QRService itself.
		pre = []ratelimit.Rule{
			ipRule("ip", cfg.ClientLookupIPLimit, cfg.ClientLookupIPWindow),
		}
		post = []ratelimit.Rule{
			storeAPIKeyOrStoreRule("principal", cfg.ClientLookupPrincipalLimit, cfg.ClientLookupPrincipalWindow),
		}
	}
	return pre, post
}
