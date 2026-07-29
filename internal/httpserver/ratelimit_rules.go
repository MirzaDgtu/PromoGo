package httpserver

import (
	"net"
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
// netip.Prefix values once at startup. A malformed entry is dropped rather
// than failing server construction — see clientIPForRateLimit's doc comment
// on what an empty/partial list means for trust.
func ParseTrustedProxies(cidrs []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		p, err := netip.ParsePrefix(strings.TrimSpace(c))
		if err != nil {
			continue
		}
		prefixes = append(prefixes, p)
	}
	return prefixes
}

// clientIPForRateLimit resolves the caller's IP for the distributed rate
// limiter. It trusts X-Forwarded-For only when the immediate peer
// (RemoteAddr) matches a configured trusted proxy CIDR — an empty
// trustedProxies list (the default) means every caller's RemoteAddr is used
// directly, exactly matching clientIP's existing, audited behavior (see
// remoteaddr.go). This is deliberately a separate function from clientIP:
// changing what audit logs and the OTP limiter consider "the client IP" is
// out of scope here.
func clientIPForRateLimit(r *http.Request, trustedProxies []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if len(trustedProxies) == 0 {
		return host
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	trusted := false
	for _, p := range trustedProxies {
		if p.Contains(addr) {
			trusted = true
			break
		}
	}
	if !trusted {
		return host
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}
	// The left-most entry is the original client, appended to by every
	// proxy hop since; only meaningful once we've established the
	// immediate peer is itself a trusted proxy.
	first, _, _ := strings.Cut(xff, ",")
	first = strings.TrimSpace(first)
	if first == "" {
		return host
	}
	return first
}

// ipRule rate-limits by caller IP, hashed via auth.HashOpaqueToken so a raw
// IP never appears in a Redis key or log line.
func ipRule(name string, limit int, window time.Duration, trustedProxies []netip.Prefix) ratelimit.Rule {
	return ratelimit.Rule{
		Name: name, Limit: limit, Window: window,
		KeyFunc: func(r *http.Request) (string, bool) {
			return "ip:" + auth.HashOpaqueToken(clientIPForRateLimit(r, trustedProxies)), true
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
func rateLimitRulesFor(route routeMeta, cfg config.RateLimitConfig, trustedProxies []netip.Prefix) (pre, post []ratelimit.Rule) {
	switch route.RateLimitProfile {
	case rlProfileStaffLogin:
		pre = []ratelimit.Rule{
			ipRule("ip", cfg.StaffLoginIPLimit, cfg.StaffLoginIPWindow, trustedProxies),
		}
	case rlProfileAdmin:
		pre = []ratelimit.Rule{
			ipRule("ip", cfg.AdminIPLimit, cfg.AdminIPWindow, trustedProxies),
		}
		post = []ratelimit.Rule{
			staffUserRule("staff", cfg.AdminStaffLimit, cfg.AdminStaffWindow),
		}
	case rlProfileClientLookup:
		post = []ratelimit.Rule{
			ipRule("ip", cfg.ClientLookupIPLimit, cfg.ClientLookupIPWindow, trustedProxies),
			principalRule("principal", cfg.ClientLookupPrincipalLimit, cfg.ClientLookupPrincipalWindow),
			phoneQueryRule("phone", cfg.ClientLookupPhoneLimit, cfg.ClientLookupPhoneWindow),
		}
	case rlProfileAccrual:
		pre = []ratelimit.Rule{
			ipRule("ip", cfg.AccrualIPLimit, cfg.AccrualIPWindow, trustedProxies),
		}
		post = []ratelimit.Rule{
			storeAPIKeyOrStoreRule("principal", cfg.AccrualPrincipalLimit, cfg.AccrualPrincipalWindow),
		}
	}
	return pre, post
}
