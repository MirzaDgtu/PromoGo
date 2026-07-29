package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// lastOTPCode extracts the OTP code from the most recent message
// fakeSMSSender captured — see CustomerAuthService.RequestOTP's
// "PromoGo: ваш код подтверждения %s" template. Tests have no other way to
// learn the code: only its hash is ever stored (see internal/service/otp_store.go).
func lastOTPCode(t *testing.T, fakes *testFakes) string {
	t.Helper()
	fakes.SMS.mu.Lock()
	defer fakes.SMS.mu.Unlock()
	if len(fakes.SMS.sent) == 0 {
		t.Fatal("no SMS sent")
	}
	_, message, ok := strings.Cut(fakes.SMS.sent[len(fakes.SMS.sent)-1], ":")
	if !ok {
		t.Fatalf("malformed captured SMS: %q", fakes.SMS.sent[len(fakes.SMS.sent)-1])
	}
	fields := strings.Fields(message)
	return fields[len(fields)-1]
}

func otpRequestReq(phone string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", jsonBodyAny(map[string]string{"phone": phone}))
}

func TestHandleRequestOTP_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	rec := doRequest(handler, otpRequestReq("+79261234567"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if len(fakes.SMS.sent) != 1 {
		t.Fatalf("SMS sent count = %d, want 1", len(fakes.SMS.sent))
	}
}

func TestHandleRequestOTP_MalformedBody(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", jsonBodyAny(map[string]string{}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (missing phone)", rec.Code)
	}
}

func TestHandleRequestOTP_ResendCooldownReturns429(t *testing.T) {
	handler, _ := newTestServer(t)
	first := doRequest(handler, otpRequestReq("+79261234567"))
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", first.Code)
	}
	second := doRequest(handler, otpRequestReq("+79261234567"))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second immediate request status = %d, want 429 (resend cooldown)", second.Code)
	}
}

func TestHandleVerifyOTP_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	doRequest(handler, otpRequestReq("+79261234567"))
	code := lastOTPCode(t, fakes)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", jsonBodyAny(map[string]string{
		"phone": "+79261234567", "code": code,
	}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp authTokensResponseBody
	decodeJSON(t, rec, &resp)
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Errorf("expected non-empty access/refresh tokens, got %+v", resp)
	}
	if resp.CustomerAccountID == 0 {
		t.Errorf("expected a non-zero customer_account_id")
	}
}

func TestHandleVerifyOTP_WrongCodeRejected(t *testing.T) {
	handler, _ := newTestServer(t)
	doRequest(handler, otpRequestReq("+79261234567"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", jsonBodyAny(map[string]string{
		"phone": "+79261234567", "code": "000000",
	}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandleVerifyOTP_MalformedBody(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", jsonBodyAny(map[string]string{"phone": "+79261234567"}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (missing code)", rec.Code)
	}
}

func TestHandleVerifyOTP_AccountBlocked(t *testing.T) {
	handler, fakes := newTestServer(t)
	blocked := fakes.CustomerAccounts.seed("+79261234567")
	blocked.Status = domain.CustomerAccountBlocked

	doRequest(handler, otpRequestReq("+79261234567"))
	code := lastOTPCode(t, fakes)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", jsonBodyAny(map[string]string{
		"phone": "+79261234567", "code": code,
	}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (account blocked)", rec.Code)
	}
}

func verifyOTPAndGetTokens(t *testing.T, handler http.Handler, fakes *testFakes, phone string) authTokensResponseBody {
	t.Helper()
	doRequest(handler, otpRequestReq(phone))
	code := lastOTPCode(t, fakes)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", jsonBodyAny(map[string]string{"phone": phone, "code": code}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify otp status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp authTokensResponseBody
	decodeJSON(t, rec, &resp)
	return resp
}

func TestHandleRefreshCustomerSession_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	tokens := verifyOTPAndGetTokens(t, handler, fakes, "+79261234567")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", jsonBodyAny(map[string]string{"refresh_token": tokens.RefreshToken}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp authTokensResponseBody
	decodeJSON(t, rec, &resp)
	if resp.RefreshToken == tokens.RefreshToken {
		t.Error("expected refresh to rotate the refresh token, got the same one back")
	}
}

func TestHandleRefreshCustomerSession_InvalidTokenRejected(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", jsonBodyAny(map[string]string{"refresh_token": "not-a-real-refresh-token"}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandleRefreshCustomerSession_MalformedBody(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", jsonBodyAny(map[string]string{}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleCustomerLogout_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	tokens := verifyOTPAndGetTokens(t, handler, fakes, "+79261234567")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", jsonBodyAny(map[string]string{"refresh_token": tokens.RefreshToken}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", rec.Code, rec.Body.String())
	}

	// The session is now revoked — a refresh must be rejected.
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", jsonBodyAny(map[string]string{"refresh_token": tokens.RefreshToken}))
	refreshRec := doRequest(handler, refreshReq)
	if refreshRec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout status = %d, want 401", refreshRec.Code)
	}
}

func TestHandleCustomerLogout_UnknownTokenIsIdempotent(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", jsonBodyAny(map[string]string{"refresh_token": "never-issued-token"}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (idempotent logout)", rec.Code)
	}
}

func TestHandleCustomerLogout_MalformedBody(t *testing.T) {
	handler, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", jsonBodyAny(map[string]string{}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleCustomerLogoutAll_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	tokens := verifyOTPAndGetTokens(t, handler, fakes, "+79261234567")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout-all", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", rec.Code, rec.Body.String())
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", jsonBodyAny(map[string]string{"refresh_token": tokens.RefreshToken}))
	refreshRec := doRequest(handler, refreshReq)
	if refreshRec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout-all status = %d, want 401", refreshRec.Code)
	}
}

// --- me.go: handleGetMe / handleGetMyBalance ---

func TestHandleGetMe_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	account := fakes.CustomerAccounts.seed("+79261234567")
	token := issueCustomerToken(t, account.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp meResponseBody
	decodeJSON(t, rec, &resp)
	if resp.Phone != "+79261234567" {
		t.Errorf("Phone = %q, want +79261234567", resp.Phone)
	}
}

func TestHandleGetMe_AccountNotFound(t *testing.T) {
	handler, _ := newTestServer(t)
	token := issueCustomerToken(t, 999999)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleGetMyBalance_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	account := fakes.CustomerAccounts.seed("+79261234567")
	client := fakes.Clients.seed(&domain.Client{StoreID: 1, Phone: "+79261234567"})
	if err := fakes.Clients.LinkCustomerAccount(context.Background(), client.ID, account.ID); err != nil {
		t.Fatalf("LinkCustomerAccount() error = %v", err)
	}
	fakes.Balances.set(client.ID, 15)
	token := issueCustomerToken(t, account.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/balance", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Balances []meBalanceItem `json:"balances"`
	}
	decodeJSON(t, rec, &resp)
	if len(resp.Balances) != 1 || resp.Balances[0].Balance != 15 {
		t.Fatalf("Balances = %+v, want one entry with Balance=15", resp.Balances)
	}
}

func TestHandleGetMyBalance_NoLinkedClients(t *testing.T) {
	handler, fakes := newTestServer(t)
	account := fakes.CustomerAccounts.seed("+79269999999")
	token := issueCustomerToken(t, account.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/balance", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Balances []meBalanceItem `json:"balances"`
	}
	decodeJSON(t, rec, &resp)
	if len(resp.Balances) != 0 {
		t.Fatalf("Balances = %+v, want empty", resp.Balances)
	}
}
