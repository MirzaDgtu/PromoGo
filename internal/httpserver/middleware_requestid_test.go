package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMW_GeneratesIDWhenAbsent(t *testing.T) {
	var seen string
	h := requestIDMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if seen == "" {
		t.Fatal("expected a generated request ID in context, got empty string")
	}
	if got := rec.Header().Get(RequestIDHeader); got != seen {
		t.Fatalf("response header %s = %q, want %q (context value)", RequestIDHeader, got, seen)
	}
}

func TestRequestIDMW_PropagatesInboundID(t *testing.T) {
	var seen string
	h := requestIDMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "caller-supplied-id-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != "caller-supplied-id-123" {
		t.Fatalf("request id = %q, want the inbound value to be propagated", seen)
	}
	if got := rec.Header().Get(RequestIDHeader); got != "caller-supplied-id-123" {
		t.Fatalf("response header = %q, want inbound value echoed", got)
	}
}

func TestRequestIDMW_RejectsOversizedInboundID(t *testing.T) {
	var seen string
	h := requestIDMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFromContext(r.Context())
	}))

	oversized := make([]byte, 200)
	for i := range oversized {
		oversized[i] = 'a'
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, string(oversized))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen == string(oversized) {
		t.Fatal("expected an oversized inbound request ID to be replaced with a generated one")
	}
	if seen == "" {
		t.Fatal("expected a generated fallback ID, got empty string")
	}
}
