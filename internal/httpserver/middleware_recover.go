package httpserver

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// recoverMW turns a panic anywhere in the handler chain below it into a
// logged error and a plain 500 response, instead of crashing the whole
// process (net/http's own per-connection recovery would otherwise just drop
// the connection, taking out every other in-flight request on it, and
// logs nothing).
//
// The response body never includes the panic value or stack trace — only the
// server log does — since a panic can carry data (a SQL error with a query
// fragment, a decoded request body) that must not reach the caller.
func recoverMW(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.ErrorContext(r.Context(), "panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
						"method", r.Method, "path", r.URL.Path,
						"request_id", requestIDFromContext(r.Context()),
					)
					// The panic may have happened after headers were already
					// written (e.g. mid-stream encode failure); WriteHeader
					// on a ResponseWriter that already committed a status is
					// a harmless no-op, not a second response.
					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
