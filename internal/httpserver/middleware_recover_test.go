package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverMW_PanicBecomes500(t *testing.T) {
	h := recoverMW(testLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if body := rec.Body.String(); strings.Contains(body, "boom") {
		t.Fatalf("response body leaked the panic value: %q", body)
	}
}

func TestRecoverMW_NoPanicPassesThrough(t *testing.T) {
	h := recoverMW(testLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d (recoverMW must not interfere with a non-panicking handler)", rec.Code, http.StatusTeapot)
	}
}
