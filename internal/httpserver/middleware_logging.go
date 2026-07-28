package httpserver

import (
	"log/slog"
	"net/http"
	"time"
)

// loggingMW logs each request's method, path, status, and duration.
func loggingMW(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r)

			log.InfoContext(r.Context(), "http request",
				"method", r.Method, "path", r.URL.Path,
				"status", sw.status, "duration", time.Since(start),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
