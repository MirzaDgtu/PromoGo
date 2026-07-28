package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// fakeClientRepo is an in-memory domain.ClientRepository. createConflictOnce
// lets a test simulate a concurrent registration: the next Create for that
// (storeID, phone) returns domain.ErrConflict but still persists a client
// under the hood, as if another goroutine's Create had won the race first.
type fakeClientRepo struct {
	byID               map[int64]*domain.Client
	nextID             int64
	createConflictOnce map[string]bool
}

func newFakeClientRepo() *fakeClientRepo {
	return &fakeClientRepo{byID: map[int64]*domain.Client{}, createConflictOnce: map[string]bool{}}
}

func (f *fakeClientRepo) GetByPhone(_ context.Context, storeID int64, phone string) (*domain.Client, error) {
	for _, c := range f.byID {
		if c.StoreID == storeID && c.Phone == phone {
			return c, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeClientRepo) GetByID(_ context.Context, id int64) (*domain.Client, error) {
	c, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeClientRepo) Create(_ context.Context, client *domain.Client) error {
	key := fmt.Sprintf("%d/%s", client.StoreID, client.Phone)
	if f.createConflictOnce[key] {
		delete(f.createConflictOnce, key)
		f.nextID++
		winner := &domain.Client{ID: f.nextID, StoreID: client.StoreID, Phone: client.Phone, CreatedAt: client.CreatedAt}
		f.byID[winner.ID] = winner
		return domain.ErrConflict
	}
	f.nextID++
	client.ID = f.nextID
	f.byID[client.ID] = client
	return nil
}

// fakeTxRepo is an in-memory domain.TransactionRepository.
type fakeTxRepo struct {
	all []*domain.Transaction
}

func (f *fakeTxRepo) GetByExternalID(_ context.Context, storeID int64, txType domain.TransactionType, externalTxID string) (*domain.Transaction, error) {
	for _, tx := range f.all {
		if tx.StoreID == storeID && tx.Type == txType && tx.ExternalTxID == externalTxID {
			return tx, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeTxRepo) ListByClient(context.Context, int64) ([]*domain.Transaction, error) {
	return nil, nil
}

// fakeBalanceRepo is an in-memory domain.BalanceRepository.
type fakeBalanceRepo struct {
	points map[int64]int64
}

func newFakeBalanceRepo() *fakeBalanceRepo {
	return &fakeBalanceRepo{points: map[int64]int64{}}
}

func (f *fakeBalanceRepo) Get(_ context.Context, clientID int64) (*domain.Balance, error) {
	return &domain.Balance{ClientID: clientID, Points: f.points[clientID]}, nil
}

func (f *fakeBalanceRepo) set(clientID, points int64) {
	f.points[clientID] = points
}

// fakeLedgerRepo mirrors LedgerRepository.Post's real semantics (atomic
// insert + balance adjustment, ErrConflict on a duplicate key,
// ErrInsufficientBalance instead of letting the balance go negative)
// against the same fakeTxRepo/fakeBalanceRepo the service reads from.
type fakeLedgerRepo struct {
	txs      *fakeTxRepo
	balances *fakeBalanceRepo
	nextID   int64
}

func (f *fakeLedgerRepo) Post(_ context.Context, tx *domain.Transaction) (*domain.Transaction, *domain.Balance, error) {
	for _, existing := range f.txs.all {
		if existing.StoreID == tx.StoreID && existing.Type == tx.Type && existing.ExternalTxID == tx.ExternalTxID {
			return nil, nil, domain.ErrConflict
		}
	}

	newPoints := f.balances.points[tx.ClientID] + tx.PointsDelta
	if newPoints < 0 {
		return nil, nil, domain.ErrInsufficientBalance
	}
	f.balances.points[tx.ClientID] = newPoints

	f.nextID++
	posted := *tx
	posted.ID = f.nextID
	posted.BalanceAfter = newPoints
	posted.CreatedAt = time.Now()
	f.txs.all = append(f.txs.all, &posted)

	return &posted, &domain.Balance{ClientID: tx.ClientID, Points: newPoints}, nil
}

// fakeConfigRepo is an in-memory domain.LoyaltyConfigRepository holding a
// single store's config, which is all these tests need.
type fakeConfigRepo struct {
	cfg *domain.LoyaltyConfig
}

func (f *fakeConfigRepo) GetByStore(_ context.Context, storeID int64) (*domain.LoyaltyConfig, error) {
	if f.cfg == nil || f.cfg.StoreID != storeID {
		return nil, domain.ErrNotFound
	}
	return f.cfg, nil
}

func (f *fakeConfigRepo) Upsert(_ context.Context, cfg *domain.LoyaltyConfig) error {
	f.cfg = cfg
	return nil
}

type testDeps struct {
	svc      *LoyaltyService
	clients  *fakeClientRepo
	txs      *fakeTxRepo
	balances *fakeBalanceRepo
	configs  *fakeConfigRepo
}

func newTestService(cfg *domain.LoyaltyConfig) *testDeps {
	clients := newFakeClientRepo()
	txs := &fakeTxRepo{}
	balances := newFakeBalanceRepo()
	configs := &fakeConfigRepo{cfg: cfg}
	ledger := &fakeLedgerRepo{txs: txs, balances: balances}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return &testDeps{
		svc:      New(log, clients, txs, balances, ledger, configs, nil),
		clients:  clients,
		txs:      txs,
		balances: balances,
		configs:  configs,
	}
}

func pointsConfig(storeID int64) *domain.LoyaltyConfig {
	return &domain.LoyaltyConfig{
		StoreID:            storeID,
		Mechanic:           "points",
		AccrualPercent:     decimal.NewFromInt(10),
		MinPurchaseAmount:  decimal.Zero,
		MinBalanceToRedeem: 0,
		MaxRedeemPercent:   decimal.NewFromInt(50),
		PointsExchangeRate: decimal.NewFromInt(1),
	}
}

func TestRedeem_CrossStoreClientRejected(t *testing.T) {
	deps := newTestService(pointsConfig(2))

	client := &domain.Client{StoreID: 1, Phone: "+70000000001", CreatedAt: time.Now()}
	if err := deps.clients.Create(context.Background(), client); err != nil {
		t.Fatalf("seed client: %v", err)
	}

	_, err := deps.svc.Redeem(context.Background(), RedeemRequest{
		StoreID:      2, // a different store's API key
		ExternalTxID: "redeem-1",
		ClientID:     client.ID,
		Points:       10,
		Amount:       decimal.NewFromInt(100),
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Redeem() error = %v, want domain.ErrNotFound", err)
	}
}

func TestAccrue_ReplayReturnsOriginalResult(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()
	req := AccrueRequest{StoreID: 1, ExternalTxID: "rcpt-1", Phone: "+70000000002", Amount: decimal.NewFromInt(200)}

	first, err := deps.svc.Accrue(ctx, req)
	if err != nil {
		t.Fatalf("first Accrue() error = %v", err)
	}
	if first.Replayed {
		t.Fatalf("first Accrue().Replayed = true, want false")
	}
	if first.PointsEarned != 20 || first.Balance != 20 {
		t.Fatalf("first Accrue() = %+v, want PointsEarned=20 Balance=20", first)
	}

	second, err := deps.svc.Accrue(ctx, req)
	if err != nil {
		t.Fatalf("replayed Accrue() error = %v", err)
	}
	if !second.Replayed {
		t.Fatalf("replayed Accrue().Replayed = false, want true")
	}
	if second.PointsEarned != first.PointsEarned || second.Balance != first.Balance {
		t.Fatalf("replayed Accrue() = %+v, want it to match original %+v", second, first)
	}
	if len(deps.txs.all) != 1 {
		t.Fatalf("posted transactions = %d, want 1 (replay must not double-accrue)", len(deps.txs.all))
	}
}

func TestAccrue_IdempotencyConflictOnMismatchedAmount(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-2", Phone: "+70000000003", Amount: decimal.NewFromInt(200),
	}); err != nil {
		t.Fatalf("first Accrue() error = %v", err)
	}

	_, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-2", Phone: "+70000000003", Amount: decimal.NewFromInt(300),
	})
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("Accrue() with reused external_tx_id but different amount: error = %v, want domain.ErrIdempotencyConflict", err)
	}
}

func TestAccrue_ConcurrentClientCreateRaceRecovered(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	phone := "+70000000004"
	deps.clients.createConflictOnce[fmt.Sprintf("%d/%s", int64(1), phone)] = true

	result, err := deps.svc.Accrue(context.Background(), AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-3", Phone: phone, Amount: decimal.NewFromInt(100),
	})
	if err != nil {
		t.Fatalf("Accrue() error = %v, want the lost Create race to be recovered from", err)
	}
	if result.PointsEarned != 10 {
		t.Fatalf("Accrue().PointsEarned = %d, want 10", result.PointsEarned)
	}

	client, err := deps.clients.GetByPhone(context.Background(), 1, phone)
	if err != nil {
		t.Fatalf("client was not persisted by the concurrent winner: %v", err)
	}
	if len(deps.txs.all) != 1 || deps.txs.all[0].ClientID != client.ID {
		t.Fatalf("posted transaction's client_id doesn't match the resolved client")
	}
}

func TestRedeem_CapsAtMaxRedeemPercentAndRejectsInsufficientBalance(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000005", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	deps.balances.set(client.ID, 100)

	// MaxRedeemPercent=50, PointsExchangeRate=1: a 100-currency-unit
	// purchase caps redemption at 50 points even though 80 were requested
	// and the balance could otherwise cover it.
	result, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-2", ClientID: client.ID, Points: 80, Amount: decimal.NewFromInt(100),
	})
	if err != nil {
		t.Fatalf("Redeem() error = %v", err)
	}
	if result.PointsRedeemed != 50 || result.Balance != 50 {
		t.Fatalf("Redeem() = %+v, want PointsRedeemed=50 Balance=50", result)
	}

	poorClient := &domain.Client{StoreID: 1, Phone: "+70000000006", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, poorClient); err != nil {
		t.Fatalf("seed poor client: %v", err)
	}
	_, err = deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-3", ClientID: poorClient.ID, Points: 10, Amount: decimal.NewFromInt(100),
	})
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("Redeem() with zero balance: error = %v, want domain.ErrInsufficientBalance", err)
	}
}
