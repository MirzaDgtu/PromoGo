package httpserver

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// clientIPContextKey holds the request's trust-resolved client IP (see
// withResolvedClientIP), computed once per request from the same
// trustedProxies configuration used everywhere else.
type clientIPContextKey struct{}

// withResolvedClientIP resolves each request's client IP exactly once (see
// resolveClientIP) and stores it in the request context, so clientIP,
// audit logging, and rate limiting can never disagree about which address
// is "the caller" for a given request — they all read this same value
// instead of three independent computations that could drift out of sync
// as trust rules evolve. Must wrap every route (see server.go's New),
// applied before routing so it runs regardless of which handler is
// eventually dispatched to.
func withResolvedClientIP(trustedProxies []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveClientIP(r, trustedProxies)
			ctx := context.WithValue(r.Context(), clientIPContextKey{}, ip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// clientIP returns the caller's IP for audit logging, OTP rate limiting,
// and the distributed rate limiter's IP dimension — one resolution used
// consistently everywhere (see withResolvedClientIP). Falls back to a
// direct, untrusted RemoteAddr parse if the request never passed through
// that middleware (e.g. a handler unit test built without the full server
// chain) — see resolveClientIP for the trust logic itself.
func clientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPContextKey{}).(string); ok {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// resolveClientIP resolves the caller's IP, trusting X-Forwarded-For only
// when the immediate peer (RemoteAddr) matches a configured trusted-proxy
// CIDR. An empty trustedProxies list (the default) means every caller's
// RemoteAddr is used directly — an arbitrary client must never be able to
// pick its own audit/rate-limit identity by spoofing this header.
func resolveClientIP(r *http.Request, trustedProxies []netip.Prefix) string {
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
