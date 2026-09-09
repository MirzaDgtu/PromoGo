package httpserver

import (
	"context"
	"net/http"
	"time"
)

// requestTimeoutMW bounds every request's context to timeout, so a
// dependency that stops responding entirely — not erroring, not refusing
// connections, just frozen (the concrete case that motivated this: a
// paused/hung Postgres container leaves an already-pooled connection's read
// blocked with no data ever arriving) — can't hang a handler goroutine
// forever. Without a bounded context, a handful of such hangs exhausts the
// pgxpool (bounded by Postgres.MaxConns) and takes the *entire* service
// down for every request, not just ones touching the frozen dependency —
// turning one dependency's outage into this service's own cascading
// failure. pgx v5 watches context cancellation on in-flight operations and
// force-closes the affected connection when it fires (see pgx's
// ctxwatch-based cancellation), so this alone is enough to bound the hang;
// go-redis already has its own client-side dial/read/write timeouts and
// doesn't need this to fail fast.
//
// Kept comfortably under the http.Server's WriteTimeout (see server.go) so
// a handler that hits this deadline still has time to write a clean error
// response instead of the connection just being reset.
const requestTimeout = 8 * time.Second

func requestTimeoutMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
