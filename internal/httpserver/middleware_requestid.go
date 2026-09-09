package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestIDHeader is both the inbound header this middleware will trust from
// an upstream proxy/load balancer, and the header it sets on every response.
const RequestIDHeader = "X-Request-Id"

type requestIDContextKey struct{}

// requestIDFromContext returns the request ID set by requestIDMW for the
// current request, or "" if none is set (e.g. in a test that doesn't apply
// the middleware).
func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

// requestIDMW assigns every request a correlation ID: the inbound
// X-Request-Id if the caller supplied one (so a request can be traced across
// 1C/the mobile app and this service), otherwise a freshly generated one. The
// ID is echoed on the response and stored in the request context so
// downstream logging (loggingMW) and panic recovery (recoverMW) can attach it
// to every log line for this request.
//
// The inbound value is trusted only as a correlation label, never as an
// authorization or idempotency input — it's logged and echoed back, nothing
// more.
func requestIDMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" || len(id) > 128 {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read on the standard reader does not fail in practice;
		// falling back to an empty-but-valid ID keeps the request flowing
		// rather than turning an ID-generation hiccup into a 500.
		return ""
	}
	return hex.EncodeToString(b[:])
}
