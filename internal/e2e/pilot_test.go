//go:build e2e

// Package e2e contains PromoGo's real pilot end-to-end test: it drives the
// actual HTTP API, backed by a real Postgres and Redis, through the exact
// flow docs/audit-remediation-prompt.md's Phase 5 asks to prove — a 1C/POS
// purchase event, accrual, mobile visibility of that accrual, QR
// identification at the POS, redemption, and offline-retry/duplicate-
// delivery safety for both the accrual and refund webhooks.
//
// Gated behind the "e2e" build tag (see .github/workflows/ci.yml's
// e2e-pilot job and docs/e2e-testing.md) because it needs a real, reachable
// Postgres/Redis and starts a real app instance — never run as part of the
// default `go test ./...`.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/app"
	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/repository/postgres"
)

// captureSMS is a domain.SMSSender that remembers the last message sent to
// each phone number, so the test can recover the OTP code it needs to
// complete the mobile-app login leg of the pilot flow. The code is
// otherwise unrecoverable by design: Redis stores only its hash (see
// internal/service/otp_store.go), and the production log-based sender never
// logs it (internal/notification/logsms). Substituting the SMS boundary is
// the same kind of substitution a real SMS-provider integration would make
// in its own test double; nothing else about the code path under test
// changes (see internal/app.WithSMSSender).
type captureSMS struct {
	mu       sync.Mutex
	messages map[string]string
}

func newCaptureSMS() *captureSMS { return &captureSMS{messages: map[string]string{}} }

func (c *captureSMS) Send(_ context.Context, phone, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages[phone] = message
	return nil
}

// codeFor extracts the OTP code from the last message sent to phone. The
// message format is fixed by internal/service/customerauth.go's RequestOTP
// ("PromoGo: ваш код подтверждения <code>") — the code is always the last
// whitespace-separated token.
func (c *captureSMS) codeFor(t *testing.T, phone string) string {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	msg, ok := c.messages[phone]
	if !ok {
		t.Fatalf("no OTP SMS captured for %s", phone)
	}
	fields := strings.Fields(msg)
	return fields[len(fields)-1]
}

// repoRoot locates the repository root relative to this test file, so
// config.Load's YAML path resolves correctly regardless of `go test`'s
// working directory (the package directory, not the repo root).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repo root: runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// requireReachable skips the test (rather than failing it) if addr isn't
// reachable — this test needs a real stack up (docker compose, or the
// e2e-pilot CI job's service containers) and that's an environment
// precondition, not a code defect, when absent.
func requireReachable(t *testing.T, addr, what string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Skipf("%s at %s not reachable (%v) — see docs/e2e-testing.md for how to run this test locally", what, addr, err)
	}
	conn.Close()
}

func waitReady(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/readyz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("app never became ready at %s/readyz: %v", baseURL, lastErr)
}

// doJSON sends one HTTP request and decodes its JSON response into out (if
// non-nil), failing the test on a transport error, an unexpected status, or
// a decode error.
func doJSON(t *testing.T, client *http.Client, method, url string, headers map[string]string, body, out any, wantStatus int) {
	t.Helper()

	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		t.Fatalf("%s %s: status %d (want %d), body: %s", method, url, resp.StatusCode, wantStatus, buf.String())
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode response: %v", method, url, err)
		}
	}
}

// TestPilotEndToEnd proves the full pilot chain end to end against a real
// app instance: 1C/POS accrual webhook (with a duplicate/offline-retry
// delivery of the same event) → mobile OTP login → mobile visibility of the
// store-side accrual → QR identification at a second POS interaction →
// redemption → refund (with its own duplicate delivery) → mobile visibility
// of the refund. See docs/e2e-testing.md for how to run it locally, and
// knowledge/Project Questions.md's Q-P0-095 for why this exists.
func TestPilotEndToEnd(t *testing.T) {
	cfg, err := config.Load(filepath.Join(repoRoot(t), "configs", "config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	requireReachable(t, fmt.Sprintf("%s:%d", cfg.Postgres.Host, cfg.Postgres.Port), "postgres")
	requireReachable(t, cfg.Redis.Addr, "redis")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- start a real app instance with a capturing SMS sender; app.New
	// applies pending migrations, so it must run before anything seeds rows ---
	sms := newCaptureSMS()
	application, err := app.New(ctx, cfg, app.WithSMSSender(sms))
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	defer application.Close()

	// --- seed a throwaway organization/store/api-key/loyalty-config,
	// mirroring cmd/promogo/loadtest_seed.go's bypass-HTTP pattern ---
	pool, err := postgres.NewPool(ctx, cfg.Postgres.DSN(), cfg.Postgres.MaxConns)
	if err != nil {
		t.Fatalf("connect postgres for seeding: %v", err)
	}
	defer pool.Close()

	orgs := postgres.NewOrganizationRepository(pool)
	stores := postgres.NewStoreRepository(pool)
	apiKeys := postgres.NewStoreAPIKeyRepository(pool)
	loyaltyConfigs := postgres.NewLoyaltyConfigRepository(pool)

	org := &domain.Organization{Name: "E2E Pilot Org"}
	if err := orgs.Create(ctx, org); err != nil {
		t.Fatalf("create organization: %v", err)
	}
	store := &domain.Store{OrganizationID: org.ID, Name: "E2E Pilot Store"}
	if err := stores.Create(ctx, store); err != nil {
		t.Fatalf("create store: %v", err)
	}
	plaintextKey, keyID, hash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("generate api key: %v", err)
	}
	if err := apiKeys.Create(ctx, &domain.StoreAPIKey{
		StoreID: store.ID,
		KeyID:   keyID,
		KeyHash: hash,
		Name:    "e2e-pilot",
		Scopes:  []string{domain.ScopeTransactionsWrite, domain.ScopeClientsLookup, domain.ScopeBalancesRead},
	}); err != nil {
		t.Fatalf("create store api key: %v", err)
	}
	loyaltyCfg := &domain.LoyaltyConfig{
		StoreID:            store.ID,
		Mechanic:           "points",
		AccrualPercent:     decimal.NewFromInt(5),
		MinPurchaseAmount:  decimal.Zero,
		MinBalanceToRedeem: 0,
		MaxRedeemPercent:   decimal.NewFromInt(50),
		PointsExchangeRate: decimal.NewFromInt(1),
	}
	if err := loyaltyConfigs.Upsert(ctx, loyaltyCfg, nil); err != nil {
		t.Fatalf("create loyalty config: %v", err)
	}

	appDone := make(chan error, 1)
	go func() { appDone <- application.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-appDone:
		case <-time.After(10 * time.Second):
			t.Log("app did not shut down within 10s of test cleanup")
		}
	})

	host := cfg.HTTP.Host
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1" // the server binds all interfaces; the client dials loopback explicitly
	}
	baseURL := fmt.Sprintf("http://%s:%d", host, cfg.HTTP.Port)
	waitReady(t, baseURL)

	client := &http.Client{Timeout: 10 * time.Second}
	storeAuth := map[string]string{"Authorization": "Bearer " + plaintextKey}
	const phone = "+79261234567"

	// === 1. 1C/POS accrual webhook ===
	var accrual struct {
		ClientID     int64 `json:"client_id"`
		PointsEarned int64 `json:"points_earned"`
		Balance      int64 `json:"balance"`
		Replayed     bool  `json:"replayed"`
	}
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/transactions", storeAuth,
		map[string]any{"transaction_id": "e2e-accrual-1", "phone": phone, "amount": "500.00"},
		&accrual, http.StatusOK)
	if accrual.PointsEarned != 25 || accrual.Balance != 25 || accrual.Replayed {
		t.Fatalf("unexpected accrual result: %+v", accrual)
	}

	// === 2. offline retry / duplicate delivery of the same webhook must not double-accrue ===
	var accrualDup struct {
		ClientID     int64 `json:"client_id"`
		PointsEarned int64 `json:"points_earned"`
		Balance      int64 `json:"balance"`
		Replayed     bool  `json:"replayed"`
	}
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/transactions", storeAuth,
		map[string]any{"transaction_id": "e2e-accrual-1", "phone": phone, "amount": "500.00"},
		&accrualDup, http.StatusOK)
	if !accrualDup.Replayed || accrualDup.Balance != 25 || accrualDup.ClientID != accrual.ClientID {
		t.Fatalf("duplicate accrual delivery was not a safe replay: %+v", accrualDup)
	}

	// === 3. mobile app: OTP login ===
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/otp/request", nil,
		map[string]any{"phone": phone}, nil, http.StatusOK)
	code := sms.codeFor(t, phone)

	var verify struct {
		AccessToken       string `json:"access_token"`
		CustomerAccountID int64  `json:"customer_account_id"`
	}
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/otp/verify", nil,
		map[string]any{"phone": phone, "code": code}, &verify, http.StatusOK)
	if verify.AccessToken == "" {
		t.Fatalf("otp verify did not return an access token")
	}
	custAuth := map[string]string{"Authorization": "Bearer " + verify.AccessToken}

	// === 4. mobile visibility: the just-logged-in customer sees the
	// accrual that happened via the store webhook before they ever logged in ===
	var balances struct {
		Balances []struct {
			StoreID int64 `json:"store_id"`
			Balance int64 `json:"balance"`
		} `json:"balances"`
	}
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/me/balance", custAuth, nil, &balances, http.StatusOK)
	if !hasStoreBalance(balances.Balances, store.ID, 25) {
		t.Fatalf("mobile app cannot see the store's accrual: %+v", balances)
	}

	// === 5. QR identification at a second POS interaction ===
	var qr struct {
		Payload string `json:"payload"`
	}
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/me/qr", custAuth, nil, &qr, http.StatusOK)

	var resolved struct {
		ClientID int64 `json:"client_id"`
		Balance  int64 `json:"balance"`
	}
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/clients/resolve-qr", storeAuth,
		map[string]any{"payload": qr.Payload}, &resolved, http.StatusOK)
	if resolved.ClientID != accrual.ClientID || resolved.Balance != 25 {
		t.Fatalf("QR resolve did not identify the expected client/balance: %+v", resolved)
	}

	// === 6. redemption, identified via the QR-resolved client_id ===
	var redeem struct {
		PointsRedeemed int64 `json:"points_redeemed"`
		Balance        int64 `json:"balance"`
		Replayed       bool  `json:"replayed"`
	}
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/transactions/redeem", storeAuth,
		map[string]any{"transaction_id": "e2e-redeem-1", "client_id": resolved.ClientID, "points": 5, "amount": "10.00"},
		&redeem, http.StatusOK)
	if redeem.PointsRedeemed != 5 || redeem.Balance != 20 {
		t.Fatalf("unexpected redeem result: %+v", redeem)
	}

	// === 7. mobile history reflects both the accrual and the redemption ===
	var history struct {
		Transactions []struct {
			Type string `json:"type"`
		} `json:"transactions"`
	}
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/me/transactions", custAuth, nil, &history, http.StatusOK)
	if len(history.Transactions) < 2 {
		t.Fatalf("expected at least accrual+redeem in mobile history, got %d entries", len(history.Transactions))
	}

	// === 8. refund of the original accrual, and its own duplicate delivery ===
	var refund struct {
		Balance       int64 `json:"balance"`
		FullyRefunded bool  `json:"fully_refunded"`
		Replayed      bool  `json:"replayed"`
	}
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/transactions/refund", storeAuth,
		map[string]any{
			"transaction_id":            "e2e-refund-1",
			"original_transaction_id":   "e2e-accrual-1",
			"original_transaction_type": "accrual",
			"amount":                    "500.00",
		}, &refund, http.StatusOK)
	// Reversing an accrual is allowed to drive the balance negative (DEC-012):
	// 20 (after redeeming 5 of the 25 accrued) - 25 (the full accrual reversed) = -5.
	if !refund.FullyRefunded || refund.Balance != -5 {
		t.Fatalf("unexpected refund result: %+v", refund)
	}

	var refundDup struct {
		Balance  int64 `json:"balance"`
		Replayed bool  `json:"replayed"`
	}
	doJSON(t, client, http.MethodPost, baseURL+"/api/v1/transactions/refund", storeAuth,
		map[string]any{
			"transaction_id":            "e2e-refund-1",
			"original_transaction_id":   "e2e-accrual-1",
			"original_transaction_type": "accrual",
			"amount":                    "500.00",
		}, &refundDup, http.StatusOK)
	if !refundDup.Replayed || refundDup.Balance != -5 {
		t.Fatalf("duplicate refund delivery was not a safe replay: %+v", refundDup)
	}

	// === 9. mobile visibility of the refund ===
	doJSON(t, client, http.MethodGet, baseURL+"/api/v1/me/balance", custAuth, nil, &balances, http.StatusOK)
	if !hasStoreBalance(balances.Balances, store.ID, -5) {
		t.Fatalf("mobile app cannot see the refund: %+v", balances)
	}
}

func hasStoreBalance(balances []struct {
	StoreID int64 `json:"store_id"`
	Balance int64 `json:"balance"`
}, storeID, want int64) bool {
	for _, b := range balances {
		if b.StoreID == storeID {
			return b.Balance == want
		}
	}
	return false
}
