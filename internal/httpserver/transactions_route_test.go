package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// seedPointsConfig installs a "points" mechanic config for storeID: 10%
// accrual, no minimum purchase, redemption uncapped at 1 point = 1 currency
// unit — the simplest config that exercises the real mechanic end to end.
func seedPointsConfig(fakes *testFakes, storeID int64) {
	fakes.LoyaltyConfigs.byStore[storeID] = &domain.LoyaltyConfig{
		StoreID:            storeID,
		Mechanic:           "points",
		AccrualPercent:     decimal.NewFromInt(10),
		MinPurchaseAmount:  decimal.Zero,
		MinBalanceToRedeem: 0,
		MaxRedeemPercent:   decimal.NewFromInt(100),
		PointsExchangeRate: decimal.NewFromInt(1),
	}
}

func accrueReq(storeKey string, body any) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", jsonBodyAny(body))
	req.Header.Set("Authorization", "Bearer "+storeKey)
	return req
}

func TestHandleAccrueTransaction_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)

	rec := doRequest(handler, accrueReq("key", map[string]any{
		"transaction_id": "tx-1", "phone": "+79261234567", "amount": "100.00",
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp transactionResponseBody
	decodeJSON(t, rec, &resp)
	if resp.PointsEarned != 10 {
		t.Errorf("PointsEarned = %d, want 10 (10%% of 100)", resp.PointsEarned)
	}
	if resp.Replayed {
		t.Errorf("Replayed = true on first request, want false")
	}
}

func TestHandleAccrueTransaction_MalformedJSON(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", stringBody("{not json"))
	req.Header.Set("Authorization", "Bearer key")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleAccrueTransaction_UnknownFieldsRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)

	rec := doRequest(handler, accrueReq("key", map[string]any{
		"transaction_id": "tx-1", "phone": "+79261234567", "amount": "100.00", "unexpected_field": "x",
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (unknown field rejected)", rec.Code)
	}
}

func TestHandleAccrueTransaction_IdempotentReplay(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)

	body := map[string]any{"transaction_id": "tx-replay", "phone": "+79261234567", "amount": "50.00"}
	first := doRequest(handler, accrueReq("key", body))
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200 (body=%s)", first.Code, first.Body.String())
	}
	var firstResp transactionResponseBody
	decodeJSON(t, first, &firstResp)

	second := doRequest(handler, accrueReq("key", body))
	if second.Code != http.StatusOK {
		t.Fatalf("second request status = %d, want 200 (body=%s)", second.Code, second.Body.String())
	}
	var secondResp transactionResponseBody
	decodeJSON(t, second, &secondResp)

	if !secondResp.Replayed {
		t.Errorf("Replayed = false on second identical request, want true")
	}
	if secondResp.Balance != firstResp.Balance || secondResp.PointsEarned != firstResp.PointsEarned {
		t.Errorf("replay result = %+v, want same as first %+v", secondResp, firstResp)
	}
}

func TestHandleAccrueTransaction_IdempotencyConflictOnReusedID(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)

	first := doRequest(handler, accrueReq("key", map[string]any{
		"transaction_id": "tx-reuse", "phone": "+79261234567", "amount": "50.00",
	}))
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", first.Code)
	}

	// Same transaction_id, different amount — not a genuine replay.
	second := doRequest(handler, accrueReq("key", map[string]any{
		"transaction_id": "tx-reuse", "phone": "+79261234567", "amount": "999.00",
	}))
	if second.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (reused transaction_id with different amount)", second.Code)
	}
}

func TestHandleAccrueTransaction_ServiceErrorMissingLoyaltyConfig(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	// No loyalty config seeded for store 1.

	rec := doRequest(handler, accrueReq("key", map[string]any{
		"transaction_id": "tx-1", "phone": "+79261234567", "amount": "100.00",
	}))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (no loyalty config configured)", rec.Code)
	}
}

func TestHandleRedeemTransaction_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)
	client := fakes.Clients.seed(&domain.Client{StoreID: 1, Phone: "+79261234567"})
	fakes.Balances.set(client.ID, 100)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/redeem", jsonBodyAny(map[string]any{
		"transaction_id": "redeem-1", "client_id": client.ID, "points": 30, "amount": "30.00",
	}))
	req.Header.Set("Authorization", "Bearer key")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp redeemResponseBody
	decodeJSON(t, rec, &resp)
	if resp.PointsRedeemed != 30 || resp.Balance != 70 {
		t.Errorf("resp = %+v, want PointsRedeemed=30 Balance=70", resp)
	}
}

func TestHandleRedeemTransaction_InsufficientBalance(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)
	client := fakes.Clients.seed(&domain.Client{StoreID: 1, Phone: "+79261234567"})
	fakes.Balances.set(client.ID, 5)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/redeem", jsonBodyAny(map[string]any{
		"transaction_id": "redeem-2", "client_id": client.ID, "points": 30, "amount": "30.00",
	}))
	req.Header.Set("Authorization", "Bearer key")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}

func TestHandleRedeemTransaction_ClientNotFound(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")
	seedPointsConfig(fakes, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/redeem", jsonBodyAny(map[string]any{
		"transaction_id": "redeem-3", "client_id": 999999, "points": 10, "amount": "10.00",
	}))
	req.Header.Set("Authorization", "Bearer key")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleRedeemTransaction_MalformedBody(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeTransactionsWrite}}, "key")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/redeem", stringBody("not json"))
	req.Header.Set("Authorization", "Bearer key")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- clients.go ---

func TestHandleLookupClientByPhone_MissingPhone(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeClientsLookup}}, "key")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup", nil)
	req.Header.Set("Authorization", "Bearer key")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleLookupClientByPhone_NotFound(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeClientsLookup}}, "key")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/lookup?phone=+79269999999", nil)
	req.Header.Set("Authorization", "Bearer key")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleGetClientBalance_MalformedID(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeBalancesRead}}, "key")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/not-a-number/balance", nil)
	req.SetPathValue("id", "not-a-number")
	req.Header.Set("Authorization", "Bearer key")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleGetClientBalance_Success(t *testing.T) {
	handler, fakes := newTestServer(t)
	fakes.Stores.byID[1] = &domain.Store{ID: 1, OrganizationID: 1, Name: "Store"}
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: 1, Scopes: []string{domain.ScopeBalancesRead}}, "key")
	client := fakes.Clients.seed(&domain.Client{StoreID: 1, Phone: "+79261234567"})
	fakes.Balances.set(client.ID, 42)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/"+itoa(client.ID)+"/balance", nil)
	req.SetPathValue("id", itoa(client.ID))
	req.Header.Set("Authorization", "Bearer key")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp balanceResponseBody
	decodeJSON(t, rec, &resp)
	if resp.Balance != 42 {
		t.Errorf("Balance = %d, want 42", resp.Balance)
	}
}
