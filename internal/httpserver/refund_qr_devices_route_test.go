package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/service"
)

// --- refund.go ---

func refundReq(storeKey string, body any) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/refund", jsonBodyAny(body))
	req.Header.Set("Authorization", "Bearer "+storeKey)
	return req
}

func TestHandleRefundTransaction_FullAccrualRefund(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)

	acc := doRequest(handler, accrueReq("key-1.key", map[string]any{
		"transaction_id": "rcpt-1", "phone": "+79261234567", "amount": "200.00",
	}))
	if acc.Code != http.StatusOK {
		t.Fatalf("accrue status = %d, want 200 (body=%s)", acc.Code, acc.Body.String())
	}

	rec := doRequest(handler, refundReq("key-1.key", map[string]any{
		"transaction_id": "refund-1", "original_transaction_id": "rcpt-1", "amount": "200.00",
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp refundResponseBody
	decodeJSON(t, rec, &resp)
	if resp.PointsReversed != -20 || resp.Balance != 0 || !resp.FullyRefunded {
		t.Errorf("resp = %+v, want PointsReversed=-20 Balance=0 FullyRefunded=true", resp)
	}
}

func TestHandleRefundTransaction_OverRefundRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)

	doRequest(handler, accrueReq("key-1.key", map[string]any{
		"transaction_id": "rcpt-2", "phone": "+79261234567", "amount": "100.00",
	}))

	rec := doRequest(handler, refundReq("key-1.key", map[string]any{
		"transaction_id": "refund-2", "original_transaction_id": "rcpt-2", "amount": "150.00",
	}))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body=%s)", rec.Code, rec.Body.String())
	}
	var errBody map[string]string
	decodeJSON(t, rec, &errBody)
	if errBody["code"] != "over_refund" {
		t.Errorf("error code = %q, want %q", errBody["code"], "over_refund")
	}
}

func TestHandleRefundTransaction_RefundOfRefundRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)

	doRequest(handler, accrueReq("key-1.key", map[string]any{
		"transaction_id": "rcpt-3", "phone": "+79261234567", "amount": "100.00",
	}))
	doRequest(handler, refundReq("key-1.key", map[string]any{
		"transaction_id": "refund-3", "original_transaction_id": "rcpt-3", "amount": "100.00",
	}))

	// original_transaction_type only accepts "accrual"/"redeem" at the HTTP
	// boundary — a refund can never even be named as an original via this
	// endpoint's input validation, independent of the service-level
	// domain.ErrCannotRefundRefund check (exercised directly in
	// internal/service/loyalty_test.go's TestRefund_RefundOfRefundRejected).
	rec := doRequest(handler, refundReq("key-1.key", map[string]any{
		"transaction_id": "refund-3b", "original_transaction_id": "refund-3",
		"original_transaction_type": "refund", "amount": "50.00",
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (original_transaction_type=refund must be rejected as input, body=%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleRefundTransaction_OriginalNotFound(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")

	rec := doRequest(handler, refundReq("key-1.key", map[string]any{
		"transaction_id": "refund-4", "original_transaction_id": "does-not-exist", "amount": "10.00",
	}))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleRefundTransaction_IdempotencyConflict(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)

	doRequest(handler, accrueReq("key-1.key", map[string]any{
		"transaction_id": "rcpt-5", "phone": "+79261234567", "amount": "100.00",
	}))
	first := doRequest(handler, refundReq("key-1.key", map[string]any{
		"transaction_id": "refund-5", "original_transaction_id": "rcpt-5", "amount": "40.00",
	}))
	if first.Code != http.StatusOK {
		t.Fatalf("first refund status = %d, want 200", first.Code)
	}

	second := doRequest(handler, refundReq("key-1.key", map[string]any{
		"transaction_id": "refund-5", "original_transaction_id": "rcpt-5", "amount": "50.00",
	}))
	if second.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (reused transaction_id, different amount)", second.Code)
	}
}

func TestHandleRefundTransaction_MissingScopeRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	// clients.lookup only — not transactions.write.
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeClientsLookup}}, "key")

	rec := doRequest(handler, refundReq("key-1.key", map[string]any{
		"transaction_id": "refund-6", "original_transaction_id": "rcpt-x", "amount": "10.00",
	}))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (missing transactions.write scope)", rec.Code)
	}
}

// --- qr.go ---

func TestHandleIssueQR_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	acc := fakes.CustomerAccounts.seedActive(t, "+79261234567")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/qr", nil)
	req.Header.Set("Authorization", "Bearer "+issueCustomerToken(t, acc.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp issueQRResponseBody
	decodeJSON(t, rec, &resp)
	if resp.Payload == "" {
		t.Errorf("Payload is empty")
	}
	if resp.ExpiresIn <= 0 {
		t.Errorf("ExpiresIn = %d, want > 0", resp.ExpiresIn)
	}
}

func TestHandleIssueQR_Unauthenticated(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/qr", nil)
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandleIssueQR_CooldownReturns429WithRetryAfter(t *testing.T) {
	deps, fakes := newTestDeps(t)
	deps.QR = service.NewQRService(deps.Log, fakes.Redis, service.QRConfig{
		TTL: time.Minute, IssueCooldown: time.Minute, ConsumeCooldown: 0,
	}, fakes.Clients, fakes.CustomerAccounts, fakes.Balances, fakes.AuditEvents)
	handler := New(deps).Handler

	acc := fakes.CustomerAccounts.seedActive(t, "+79261234568")
	token := "Bearer " + issueCustomerToken(t, acc.ID)

	first := httptest.NewRequest(http.MethodPost, "/api/v1/me/qr", nil)
	first.Header.Set("Authorization", token)
	if rec := doRequest(handler, first); rec.Code != http.StatusOK {
		t.Fatalf("first issue status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	second := httptest.NewRequest(http.MethodPost, "/api/v1/me/qr", nil)
	second.Header.Set("Authorization", token)
	rec := doRequest(handler, second)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (body=%s)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Errorf("Retry-After header missing")
	}
}

func TestHandleResolveQR_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeClientsLookup}}, "key")
	acc := fakes.CustomerAccounts.seedActive(t, "+79261234569")

	issueReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/qr", nil)
	issueReq.Header.Set("Authorization", "Bearer "+issueCustomerToken(t, acc.ID))
	issueRec := doRequest(handler, issueReq)
	var issued issueQRResponseBody
	decodeJSON(t, issueRec, &issued)

	rec := doRequest(handler, refundReqPOST("/api/v1/clients/resolve-qr", "key-1.key", map[string]any{"payload": issued.Payload}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp resolveQRResponseBody
	decodeJSON(t, rec, &resp)
	if resp.ClientID == 0 {
		t.Errorf("ClientID = 0, want a resolved client")
	}
}

func TestHandleResolveQR_Malformed(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeClientsLookup}}, "key")

	rec := doRequest(handler, refundReqPOST("/api/v1/clients/resolve-qr", "key-1.key", map[string]any{"payload": "not-a-valid-qr"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleResolveQR_UnknownTokenGone(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeClientsLookup}}, "key")

	rec := doRequest(handler, refundReqPOST("/api/v1/clients/resolve-qr", "key-1.key",
		map[string]any{"payload": "v1.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}))
	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleResolveQR_MissingScopeRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	// transactions.write only — not clients.lookup.
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")

	rec := doRequest(handler, refundReqPOST("/api/v1/clients/resolve-qr", "key-1.key", map[string]any{"payload": "v1.x"}))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (missing clients.lookup scope)", rec.Code)
	}
}

// refundReqPOST builds an authenticated store-key POST request to path —
// shared by resolve-qr tests here (refund.go's refundReq is specific to
// the refund endpoint's path).
func refundReqPOST(path, storeKey string, body any) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, jsonBodyAny(body))
	req.Header.Set("Authorization", "Bearer "+storeKey)
	return req
}

// --- devices.go ---

func TestHandleRegisterDevice_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	acc := fakes.CustomerAccounts.seedActive(t, "+79261234570")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/devices", jsonBodyAny(map[string]any{
		"platform": "android", "push_token": "tok-1",
	}))
	req.Header.Set("Authorization", "Bearer "+issueCustomerToken(t, acc.ID))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp registerDeviceResponseBody
	decodeJSON(t, rec, &resp)
	if resp.DeviceID == 0 {
		t.Errorf("DeviceID = 0, want a registered device")
	}
}

func TestHandleRegisterDevice_Unauthenticated(t *testing.T) {
	handler, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/devices", jsonBodyAny(map[string]any{
		"platform": "android", "push_token": "tok-1",
	}))
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandleRevokeDevice_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	acc := fakes.CustomerAccounts.seedActive(t, "+79261234571")
	token := "Bearer " + issueCustomerToken(t, acc.ID)

	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/devices", jsonBodyAny(map[string]any{
		"platform": "ios", "push_token": "tok-2",
	}))
	regReq.Header.Set("Authorization", token)
	regRec := doRequest(handler, regReq)
	var reg registerDeviceResponseBody
	decodeJSON(t, regRec, &reg)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/me/devices/"+itoa(reg.DeviceID), nil)
	delReq.SetPathValue("deviceID", itoa(reg.DeviceID))
	delReq.Header.Set("Authorization", token)
	delRec := doRequest(handler, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", delRec.Code, delRec.Body.String())
	}
}

func TestHandleRevokeDevice_ForeignDeviceNotFound(t *testing.T) {
	handler, fakes := newTestServer(t)
	owner := fakes.CustomerAccounts.seedActive(t, "+79261234572")
	attacker := fakes.CustomerAccounts.seedActive(t, "+79261234573")

	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/devices", jsonBodyAny(map[string]any{
		"platform": "ios", "push_token": "tok-3",
	}))
	regReq.Header.Set("Authorization", "Bearer "+issueCustomerToken(t, owner.ID))
	regRec := doRequest(handler, regReq)
	var reg registerDeviceResponseBody
	decodeJSON(t, regRec, &reg)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/me/devices/"+itoa(reg.DeviceID), nil)
	delReq.SetPathValue("deviceID", itoa(reg.DeviceID))
	delReq.Header.Set("Authorization", "Bearer "+issueCustomerToken(t, attacker.ID))
	delRec := doRequest(handler, delReq)
	if delRec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (a device must never be revocable by another account)", delRec.Code)
	}
}

// seedActive creates and stores an active, phone-verified CustomerAccount,
// mirroring fakeCustomerAccountRepo.seed's role for the other fakes in this
// package (there is no pre-existing seed helper on this fake — see
// testsupport_test.go's fakeCustomerAccountRepo).
func (f *fakeCustomerAccountRepo) seedActive(t *testing.T, phone string) *domain.CustomerAccount {
	t.Helper()
	acc := &domain.CustomerAccount{Phone: phone, Status: domain.CustomerAccountActive, PhoneVerifiedAt: time.Now()}
	if err := f.Create(context.Background(), acc); err != nil {
		t.Fatalf("seed customer account: %v", err)
	}
	return acc
}
