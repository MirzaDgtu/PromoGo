package ratelimit

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Rule is one dimension a profile is rate-limited by (e.g. "by caller IP",
// "by staff user", "by store API key"). KeyFunc returns the dimension's raw
// key material and whether the rule applies to this request at all —
// returning false skips the rule entirely (e.g. a phone-hash rule on a
// route with no phone query param).
//
// KeyFunc must never return a raw phone number, bearer token, or API key —
// callers are expected to hash any caller-supplied secret/PII material
// before returning it (see internal/auth.HashOpaqueToken), since the
// returned value becomes part of a Redis key and a log field.
type Rule struct {
	Name    string
	Limit   int
	Window  time.Duration
	KeyFunc func(r *http.Request) (key string, applies bool)
}

// Middleware applies a profile's Rules against a Limiter.
type Middleware struct {
	limiter *Limiter
	log     *slog.Logger
}

// NewMiddleware constructs a Middleware. A nil limiter makes every Wrap a
// no-op passthrough — the deployment simply has no rate limiter configured,
// as opposed to a configured limiter whose backend is unavailable at
// request time (which fails closed with 503; see Wrap).
func NewMiddleware(limiter *Limiter, log *slog.Logger) *Middleware {
	return &Middleware{limiter: limiter, log: log}
}

// Wrap checks every rule in order under profile's Redis namespace before
// calling next. The first rule that either errors (limiter backend down)
// or is exceeded short-circuits the request: a backend error responds 503
// (fail closed — a rate limiter that can't be checked must never silently
// stop protecting the route), an exceeded limit responds 429 with
// Retry-After set to the window's remaining time.
func (m *Middleware) Wrap(profile string, rules ...Rule) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		if m.limiter == nil || len(rules) == 0 {
			return next
		}
		return func(w http.ResponseWriter, r *http.Request) {
			for _, rule := range rules {
				raw, applies := rule.KeyFunc(r)
				if !applies {
					continue
				}

				redisKey := "ratelimit:" + profile + ":" + rule.Name + ":" + raw
				allowed, retryAfter, err := m.limiter.Allow(r.Context(), redisKey, rule.Limit, rule.Window)
				if err != nil {
					m.log.ErrorContext(r.Context(), "rate limiter backend unavailable",
						"profile", profile, "rule", rule.Name, "error", err)
					writeError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
					return
				}
				if !allowed {
					seconds := int(retryAfter.Round(time.Second) / time.Second)
					if seconds < 1 {
						seconds = 1
					}
					w.Header().Set("Retry-After", strconv.Itoa(seconds))
					m.log.WarnContext(r.Context(), "rate limit exceeded",
						"profile", profile, "rule", rule.Name, "retry_after_seconds", seconds)
					writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
					return
				}
			}
			next(w, r)
		}
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
