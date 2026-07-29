package ratelimit

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func alwaysAppliesKey(key string) func(*http.Request) (string, bool) {
	return func(*http.Request) (string, bool) { return key, true }
}

func TestMiddleware_NilLimiterIsNoOp(t *testing.T) {
	m := NewMiddleware(nil, testLogger())
	handler := m.Wrap("profile", Rule{Name: "ip", Limit: 0, Window: time.Minute, KeyFunc: alwaysAppliesKey("x")})(okHandler)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (nil limiter must not block requests)", rec.Code)
	}
}

func TestMiddleware_NoRulesIsNoOp(t *testing.T) {
	l, _ := newTestLimiter(t)
	m := NewMiddleware(l, testLogger())
	handler := m.Wrap("profile")(okHandler)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestMiddleware_AllowsThenRejectsWithRetryAfter(t *testing.T) {
	l, _ := newTestLimiter(t)
	m := NewMiddleware(l, testLogger())
	rule := Rule{Name: "ip", Limit: 2, Window: time.Minute, KeyFunc: alwaysAppliesKey("caller-1")}
	handler := m.Wrap("profile", rule)(okHandler)

	for i := 1; i <= 2; i++ {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected a Retry-After header on 429")
	}
}

func TestMiddleware_RuleIsolationBetweenPrincipals(t *testing.T) {
	l, _ := newTestLimiter(t)
	m := NewMiddleware(l, testLogger())

	principal := "shared-profile-key"
	handlerFor := func(caller string) http.HandlerFunc {
		rule := Rule{Name: "principal", Limit: 1, Window: time.Minute, KeyFunc: alwaysAppliesKey(caller)}
		return m.Wrap("profile", rule)(okHandler)
	}

	// Exhaust principal A's quota.
	recA1 := httptest.NewRecorder()
	handlerFor(principal+":a")(recA1, httptest.NewRequest(http.MethodGet, "/", nil))
	if recA1.Code != http.StatusOK {
		t.Fatalf("principal A first request: status = %d, want 200", recA1.Code)
	}
	recA2 := httptest.NewRecorder()
	handlerFor(principal+":a")(recA2, httptest.NewRequest(http.MethodGet, "/", nil))
	if recA2.Code != http.StatusTooManyRequests {
		t.Fatalf("principal A second request: status = %d, want 429", recA2.Code)
	}

	// Principal B must be unaffected.
	recB := httptest.NewRecorder()
	handlerFor(principal+":b")(recB, httptest.NewRequest(http.MethodGet, "/", nil))
	if recB.Code != http.StatusOK {
		t.Fatalf("principal B first request: status = %d, want 200 (isolated from A)", recB.Code)
	}
}

func TestMiddleware_SkipsRuleThatDoesNotApply(t *testing.T) {
	l, _ := newTestLimiter(t)
	m := NewMiddleware(l, testLogger())
	neverApplies := Rule{Name: "phone", Limit: 0, Window: time.Minute, KeyFunc: func(*http.Request) (string, bool) { return "", false }}
	handler := m.Wrap("profile", neverApplies)(okHandler)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (a limit=0 rule that never applies must not block)", rec.Code)
	}
}

func TestMiddleware_BackendErrorReturns503(t *testing.T) {
	mr := newFakeUnavailableRedis(t)
	m := NewMiddleware(mr, testLogger())
	rule := Rule{Name: "ip", Limit: 5, Window: time.Minute, KeyFunc: alwaysAppliesKey("x")}
	handler := m.Wrap("profile", rule)(okHandler)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (limiter backend down must fail closed, not admit the request)", rec.Code)
	}
}

// newFakeUnavailableRedis returns a Limiter whose backend is already
// unreachable, for testing Wrap's fail-closed 503 path.
func newFakeUnavailableRedis(t *testing.T) *Limiter {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	mr.Close()
	return New(rdb)
}
