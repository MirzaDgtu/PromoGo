package httpserver

import (
	"net"
	"net/http"
)

// clientIP returns the caller's IP for audit logging and OTP rate
// limiting. It reads r.RemoteAddr directly rather than trusting
// X-Forwarded-For: PromoGo has no configured trusted-proxy allowlist yet,
// and honoring a client-supplied header here would let a caller spoof its
// own rate-limit bucket.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
