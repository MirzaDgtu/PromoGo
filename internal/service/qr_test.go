package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

func newQRTestService(t *testing.T, cfg QRConfig) (*QRService, *fakeClientRepo, *fakeCustomerAccountRepo, *fakeBalanceRepo, *fakeAuditEventRepo, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	clients := newFakeClientRepo()
	accounts := newFakeCustomerAccountRepo()
	balances := newFakeBalanceRepo()
	audit := &fakeAuditEventRepo{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewQRService(log, rdb, cfg, clients, accounts, balances, audit)
	return svc, clients, accounts, balances, audit, mr
}

func defaultQRConfig() QRConfig {
	return QRConfig{TTL: 2 * time.Minute, IssueCooldown: 30 * time.Second, ConsumeCooldown: time.Second}
}

func seedActiveAccount(t *testing.T, accounts *fakeCustomerAccountRepo, phone string) *domain.CustomerAccount {
	t.Helper()
	acc := &domain.CustomerAccount{Phone: phone, Status: domain.CustomerAccountActive, PhoneVerifiedAt: time.Now()}
	if err := accounts.Create(context.Background(), acc); err != nil {
		t.Fatalf("seed customer account: %v", err)
	}
	return acc
}

func TestQR_PayloadContainsNoPII(t *testing.T) {
	svc, _, accounts, _, _, _ := newQRTestService(t, defaultQRConfig())
	acc := seedActiveAccount(t, accounts, "+79261111111")

	payload, _, _, err := svc.IssueQR(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("IssueQR() error = %v", err)
	}
	if strings.Contains(payload, acc.Phone) {
		t.Fatalf("payload contains the raw phone number: %q", payload)
	}
	if strings.Contains(payload, "79261111111") {
		t.Fatalf("payload contains phone digits: %q", payload)
	}
}

func TestQR_IssueThenResolveHappyPath(t *testing.T) {
	svc, clients, accounts, balances, _, _ := newQRTestService(t, defaultQRConfig())
	acc := seedActiveAccount(t, accounts, "+79261111112")

	payload, expiresAt, _, err := svc.IssueQR(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("IssueQR() error = %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("IssueQR().expiresAt = %v, want in the future", expiresAt)
	}

	balances.set(1, 0) // client ID isn't known yet; balance defaults to 0 either way.

	client, balance, err := svc.ResolveQR(context.Background(), 1, payload, "store-1")
	if err != nil {
		t.Fatalf("ResolveQR() error = %v", err)
	}
	if client.StoreID != 1 {
		t.Fatalf("ResolveQR().Client.StoreID = %d, want 1", client.StoreID)
	}
	if client.CustomerAccountID == nil || *client.CustomerAccountID != acc.ID {
		t.Fatalf("ResolveQR().Client.CustomerAccountID = %v, want %d", client.CustomerAccountID, acc.ID)
	}
	if balance.ClientID != client.ID {
		t.Fatalf("ResolveQR().Balance.ClientID = %d, want %d", balance.ClientID, client.ID)
	}

	linked, err := clients.ListByCustomerAccount(context.Background(), acc.ID)
	if err != nil || len(linked) != 1 {
		t.Fatalf("ListByCustomerAccount() = %v, %v, want exactly one linked client", linked, err)
	}
}

func TestQR_AtomicSingleUse(t *testing.T) {
	svc, _, accounts, _, _, _ := newQRTestService(t, defaultQRConfig())
	acc := seedActiveAccount(t, accounts, "+79261111113")

	payload, _, _, err := svc.IssueQR(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("IssueQR() error = %v", err)
	}

	if _, _, err := svc.ResolveQR(context.Background(), 1, payload, "store-1"); err != nil {
		t.Fatalf("first ResolveQR() error = %v", err)
	}

	_, _, err = svc.ResolveQR(context.Background(), 1, payload, "store-2")
	if !errors.Is(err, ErrQRGone) {
		t.Fatalf("replayed ResolveQR(): error = %v, want ErrQRGone", err)
	}
}

func TestQR_ConcurrentConsumeOnlyOneSucceeds(t *testing.T) {
	svc, _, accounts, _, _, _ := newQRTestService(t, defaultQRConfig())
	acc := seedActiveAccount(t, accounts, "+79261111114")

	payload, _, _, err := svc.IssueQR(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("IssueQR() error = %v", err)
	}

	const attempts = 20
	var successes int32
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func() {
			defer wg.Done()
			// Each goroutine uses a distinct consumePrincipal so the
			// consume-cooldown gate (a separate protection, see DEC-011)
			// doesn't mask this test's actual target: token single-use.
			_, _, err := svc.ResolveQR(context.Background(), 1, payload, randPrincipal())
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Fatalf("concurrent ResolveQR() successes = %d, want exactly 1", successes)
	}
}

var principalCounter int64
var principalMu sync.Mutex

func randPrincipal() string {
	principalMu.Lock()
	defer principalMu.Unlock()
	principalCounter++
	return "store-" + string(rune('a'+principalCounter%26))
}

func TestQR_ExpiredMalformedUnknownReplayClassifiedCorrectly(t *testing.T) {
	svc, _, accounts, _, _, mr := newQRTestService(t, QRConfig{TTL: 50 * time.Millisecond, IssueCooldown: 0, ConsumeCooldown: 0})
	acc := seedActiveAccount(t, accounts, "+79261111115")

	// Malformed: never touches Redis, always 400-class.
	_, _, err := svc.ResolveQR(context.Background(), 1, "not-a-valid-payload", "p1")
	if !errors.Is(err, ErrQRMalformed) {
		t.Fatalf("malformed payload: error = %v, want ErrQRMalformed", err)
	}

	// Unknown: well-formed but never issued.
	_, _, err = svc.ResolveQR(context.Background(), 1, "v1.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "p2")
	if !errors.Is(err, ErrQRGone) {
		t.Fatalf("unknown token: error = %v, want ErrQRGone", err)
	}

	// Expired: issued, then TTL elapses before consume.
	payload, _, _, err := svc.IssueQR(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("IssueQR() error = %v", err)
	}
	mr.FastForward(100 * time.Millisecond)
	_, _, err = svc.ResolveQR(context.Background(), 1, payload, "p3")
	if !errors.Is(err, ErrQRGone) {
		t.Fatalf("expired token: error = %v, want ErrQRGone", err)
	}

	// Replay: issued, consumed once, consumed again.
	payload2, _, _, err := svc.IssueQR(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("second IssueQR() error = %v", err)
	}
	if _, _, err := svc.ResolveQR(context.Background(), 1, payload2, "p4"); err != nil {
		t.Fatalf("first consume of second token: error = %v", err)
	}
	_, _, err = svc.ResolveQR(context.Background(), 1, payload2, "p5")
	if !errors.Is(err, ErrQRGone) {
		t.Fatalf("replayed second token: error = %v, want ErrQRGone", err)
	}
}

func TestQR_IssueCooldownRejectsRapidReissue(t *testing.T) {
	svc, _, accounts, _, _, _ := newQRTestService(t, QRConfig{TTL: time.Minute, IssueCooldown: 30 * time.Second, ConsumeCooldown: 0})
	acc := seedActiveAccount(t, accounts, "+79261111116")

	if _, _, _, err := svc.IssueQR(context.Background(), acc.ID); err != nil {
		t.Fatalf("first IssueQR() error = %v", err)
	}

	_, _, retryAfter, err := svc.IssueQR(context.Background(), acc.ID)
	if !errors.Is(err, ErrQRIssueCooldown) {
		t.Fatalf("second immediate IssueQR(): error = %v, want ErrQRIssueCooldown", err)
	}
	if retryAfter <= 0 {
		t.Fatalf("IssueQR() cooldown retryAfter = %v, want > 0", retryAfter)
	}
}

func TestQR_StoreIsolation_SameAccountDifferentStoresGetDifferentClients(t *testing.T) {
	// IssueCooldown=0: this test issues two QRs back-to-back for the same
	// account (one per store visit) — the issue cooldown is exercised
	// separately by TestQR_IssueCooldownRejectsRapidReissue.
	svc, clients, accounts, _, _, _ := newQRTestService(t, QRConfig{TTL: time.Minute, IssueCooldown: 0, ConsumeCooldown: 0})
	acc := seedActiveAccount(t, accounts, "+79261111117")

	payloadA, _, _, err := svc.IssueQR(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("IssueQR() error = %v", err)
	}
	clientA, _, err := svc.ResolveQR(context.Background(), 100, payloadA, "store-100")
	if err != nil {
		t.Fatalf("ResolveQR() at store 100: error = %v", err)
	}

	payloadB, _, _, err := svc.IssueQR(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("second IssueQR() error = %v", err)
	}
	clientB, _, err := svc.ResolveQR(context.Background(), 200, payloadB, "store-200")
	if err != nil {
		t.Fatalf("ResolveQR() at store 200: error = %v", err)
	}

	if clientA.ID == clientB.ID {
		t.Fatalf("resolving the same account's QR at two different stores returned the same Client (%d) — expected separate store-scoped clients", clientA.ID)
	}
	if clientA.StoreID != 100 || clientB.StoreID != 200 {
		t.Fatalf("client store scoping wrong: clientA.StoreID=%d clientB.StoreID=%d", clientA.StoreID, clientB.StoreID)
	}

	linked, err := clients.ListByCustomerAccount(context.Background(), acc.ID)
	if err != nil || len(linked) != 2 {
		t.Fatalf("ListByCustomerAccount() = %v, %v, want exactly two linked clients (one per store)", linked, err)
	}
}

func TestQR_GoneAccountDoesNotLeakExistence(t *testing.T) {
	svc, _, accounts, _, _, _ := newQRTestService(t, defaultQRConfig())
	acc := seedActiveAccount(t, accounts, "+79261111118")

	payload, _, _, err := svc.IssueQR(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("IssueQR() error = %v", err)
	}

	acc.Status = domain.CustomerAccountBlocked

	_, _, err = svc.ResolveQR(context.Background(), 1, payload, "store-1")
	if !errors.Is(err, ErrQRGone) {
		t.Fatalf("ResolveQR() for a blocked account: error = %v, want ErrQRGone (same as any other gone token)", err)
	}
}
