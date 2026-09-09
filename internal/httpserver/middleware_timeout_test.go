package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestTimeoutMW_ContextCarriesADeadline(t *testing.T) {
	var hadDeadline bool
	h := requestTimeoutMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadDeadline = r.Context().Deadline()
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !hadDeadline {
		t.Fatal("expected the request context to carry a deadline")
	}
}

func TestRequestTimeoutMW_DoesNotInterfereWithANormalRequest(t *testing.T) {
	h := requestTimeoutMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}
