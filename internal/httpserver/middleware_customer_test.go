package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
)

var testCustomerSecret = []byte("test-secret-at-least-32-bytes-long!")

func TestRequireCustomerSession_ValidToken(t *testing.T) {
	token, err := auth.IssueCustomerAccessToken(testCustomerSecret, 99, time.Minute)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}

	var sawID int64
	handler := RequireCustomerSession(testCustomerSecret)(func(w http.ResponseWriter, r *http.Request) {
		sawID, _ = customerFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if sawID != 99 {
		t.Errorf("customer id in context = %d, want 99", sawID)
	}
}

func TestRequireCustomerSession_MissingTokenRejected(t *testing.T) {
	handler := RequireCustomerSession(testCustomerSecret)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestRequireCustomerSession_StaffTokenRejected verifies a staff access
// token can never be used to authenticate as a customer, even signed with
// the same secret — see internal/auth's tokenType claim.
func TestRequireCustomerSession_StaffTokenRejected(t *testing.T) {
	staffToken, err := auth.IssueStaffAccessToken(testCustomerSecret, 99, time.Minute)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}

	handler := RequireCustomerSession(testCustomerSecret)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (staff token used as customer token)", rec.Code)
	}
}

func TestRequireCustomerSession_ExpiredTokenRejected(t *testing.T) {
	token, err := auth.IssueCustomerAccessToken(testCustomerSecret, 99, -time.Minute)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}

	handler := RequireCustomerSession(testCustomerSecret)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
