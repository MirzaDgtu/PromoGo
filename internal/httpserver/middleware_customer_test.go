package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

var testCustomerSecret = []byte("test-secret-at-least-32-bytes-long!")

// activeCustomerAccounts returns a fakeCustomerAccountRepo with an active
// account seeded at id 99 — the fixed customerAccountID these tests issue
// tokens for — so RequireCustomerSession's per-request account-status
// lookup (see its doc comment) doesn't itself reject an otherwise-valid
// token. Tests that need a non-active account seed byID[99] directly.
func activeCustomerAccounts() *fakeCustomerAccountRepo {
	accounts := newFakeCustomerAccountRepo()
	accounts.byID[99] = &domain.CustomerAccount{ID: 99, Status: domain.CustomerAccountActive}
	accounts.nextID = 99
	return accounts
}

func TestRequireCustomerSession_ValidToken(t *testing.T) {
	token, err := auth.IssueCustomerAccessToken(testCustomerSecret, 99, time.Minute)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}

	var sawID int64
	handler := RequireCustomerSession(testCustomerSecret, activeCustomerAccounts())(func(w http.ResponseWriter, r *http.Request) {
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
	handler := RequireCustomerSession(testCustomerSecret, activeCustomerAccounts())(okHandler())

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

	handler := RequireCustomerSession(testCustomerSecret, activeCustomerAccounts())(okHandler())

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

	handler := RequireCustomerSession(testCustomerSecret, activeCustomerAccounts())(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestRequireCustomerSession_BlockedAccountRejected is the regression test
// for the audit finding that customer access tokens were purely stateless
// JWTs: blocking/deleting a CustomerAccount had zero effect on its
// already-issued access tokens until they naturally expired (up to
// AccessTokenTTL later). RequireCustomerSession now loads the account on
// every request and rejects anything but CustomerAccountActive.
func TestRequireCustomerSession_BlockedAccountRejected(t *testing.T) {
	token, err := auth.IssueCustomerAccessToken(testCustomerSecret, 99, time.Minute)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}
	accounts := activeCustomerAccounts()
	accounts.byID[99].Status = domain.CustomerAccountBlocked

	handler := RequireCustomerSession(testCustomerSecret, accounts)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (blocked account's still-valid access token must be rejected)", rec.Code)
	}
}
