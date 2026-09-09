package httpserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/metrics"
)

// metricsMW records metrics.HTTPRequestDuration for one route, labeled by
// operationID/method (both fixed, low-cardinality strings from routeTable —
// never the raw URL, which would create a label series per numeric path
// parameter) and the response status code.
func metricsMW(operationID, method string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next(sw, r)
			metrics.HTTPRequestDuration.
				WithLabelValues(operationID, method, strconv.Itoa(sw.status)).
				Observe(time.Since(start).Seconds())
		}
	}
}
