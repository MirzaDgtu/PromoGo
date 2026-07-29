package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleStaffOIDCLogin_MalformedBody(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/staff/auth/oidc", jsonBodyAny(map[string]string{}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (missing id_token)", rec.Code)
	}
}

// TestHandleStaffOIDCLogin_InvalidToken exercises the 401 path with a
// malformed JWT — jwt.ParseWithClaims fails before ever needing to fetch a
// JWKS, so this doesn't require a live OIDC provider (see
// auth.OIDCVerifier.VerifyIDToken).
func TestHandleStaffOIDCLogin_InvalidToken(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/staff/auth/oidc", jsonBodyAny(map[string]string{"id_token": "not-a-real-jwt"}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
