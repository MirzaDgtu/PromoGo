package httpserver

import (
	"net/http"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

func getOverview(t *testing.T, handler http.Handler, orgID, storeID int64, token string) (int, storeOverviewBody) {
	t.Helper()
	req := adminReq(http.MethodGet, "/api/v1/admin/organizations/"+itoa(orgID)+"/stores/"+itoa(storeID)+"/overview", token, nil)
	req.SetPathValue("orgID", itoa(orgID))
	req.SetPathValue("storeID", itoa(storeID))
	rec := doRequest(handler, req)
	var resp storeOverviewBody
	if rec.Code == http.StatusOK {
		decodeJSON(t, rec, &resp)
	}
	return rec.Code, resp
}

func TestHandleGetStoreOverview_ChecklistAndActivity(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	seedPointsConfig(fakes, store.ID)
	used := time.Now().Add(-time.Hour)
	past := time.Now().Add(-time.Hour)
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 1, StoreID: store.ID, LastUsedAt: &used}, "a")
	fakes.StoreAPIKeys.add(&domain.StoreAPIKey{ID: 2, StoreID: store.ID, RevokedAt: &past}, "b")
	client := fakes.Clients.seed(&domain.Client{StoreID: store.ID, Phone: "+79261234567"})
	now := time.Now()
	fakes.Transactions.all = append(fakes.Transactions.all,
		&domain.Transaction{StoreID: store.ID, ClientID: client.ID, Type: domain.TransactionAccrual, Amount: decimal.NewFromInt(1000), PointsDelta: 50, CreatedAt: now.Add(-time.Hour)},
		&domain.Transaction{StoreID: store.ID, ClientID: client.ID, Type: domain.TransactionRedeem, Amount: decimal.NewFromInt(300), PointsDelta: -20, CreatedAt: now.Add(-48 * time.Hour)},
		&domain.Transaction{StoreID: store.ID, ClientID: client.ID, Type: domain.TransactionAccrual, Amount: decimal.NewFromInt(5000), PointsDelta: 250, CreatedAt: now.Add(-30 * 24 * time.Hour)},
	)
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	code, resp := getOverview(t, handler, org.ID, store.ID, token)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !resp.LoyaltyConfig.Configured || resp.LoyaltyConfig.Version == nil {
		t.Errorf("loyalty_config = %+v, want configured with a version", resp.LoyaltyConfig)
	}
	if resp.APIKeys == nil || resp.APIKeys.Active != 1 || resp.APIKeys.LastUsedAt == nil {
		t.Errorf("api_keys = %+v, want 1 active key with a last use", resp.APIKeys)
	}
	if resp.Clients != 1 || resp.LastTransactionAt == nil {
		t.Errorf("clients/last = %d/%v, want 1 and a last operation", resp.Clients, resp.LastTransactionAt)
	}
	if resp.Last24h != (activityWindowBody{Operations: 1, PointsAccrued: 50, SalesAmount: "1000"}) {
		t.Errorf("last_24h = %+v", resp.Last24h)
	}
	if resp.Last7d != (activityWindowBody{Operations: 2, PointsAccrued: 50, PointsRedeemed: 20, SalesAmount: "1000"}) {
		t.Errorf("last_7d = %+v", resp.Last7d)
	}
}

func TestHandleGetStoreOverview_UnconfiguredStoreAndKeysHiddenFromStoreManager(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, &store.ID, domain.RoleStoreManager)

	code, resp := getOverview(t, handler, org.ID, store.ID, token)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if resp.LoyaltyConfig.Configured || resp.APIKeys != nil {
		t.Errorf("resp = %+v, want unconfigured and no api_keys for store_manager", resp)
	}
}

func TestHandleGetStoreOverview_CrossOrganizationRejected(t *testing.T) {
	handler, fakes := newTestServer(t)
	org := seedOrganization(fakes, "Acme")
	other := seedOrganization(fakes, "Other")
	foreign := seedStore(fakes, other.ID, "Foreign")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	if code, _ := getOverview(t, handler, org.ID, foreign.ID, token); code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for another organization's store", code)
	}
}
