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

func (f *fakeClientRepo) ListUnlinkedByPhone(_ context.Context, phone string) ([]*domain.Client, error) {
	var out []*domain.Client
	for _, c := range f.byID {
		if c.Phone == phone && c.CustomerAccountID == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeClientRepo) LinkCustomerAccount(_ context.Context, clientID, customerAccountID int64) error {
	c, ok := f.byID[clientID]
	if !ok {
		return domain.ErrNotFound
	}
	c.CustomerAccountID = &customerAccountID
	return nil
}

func (f *fakeClientRepo) ListByCustomerAccount(_ context.Context, customerAccountID int64) ([]*domain.Client, error) {
	var out []*domain.Client
	for _, c := range f.byID {
		if c.CustomerAccountID != nil && *c.CustomerAccountID == customerAccountID {
			out = append(out, c)
		}
	}
	return out, nil
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

func (f *fakeTxRepo) GetByID(_ context.Context, id int64) (*domain.Transaction, error) {
	for _, tx := range f.all {
		if tx.ID == id {
			return tx, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeTxRepo) ListByClient(context.Context, int64) ([]*domain.Transaction, error) {
	return nil, nil
}

func (f *fakeTxRepo) ListByClientIDs(context.Context, []int64, int, *domain.TransactionCursor) ([]*domain.Transaction, error) {
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

// PostRedeemChecked mirrors the real repository's atomic daily-limit check
// (see internal/repository/postgres/ledger_repository.go), summing this
// fake store's own redeem history in the trailing window.
func (f *fakeLedgerRepo) PostRedeemChecked(ctx context.Context, tx *domain.Transaction, minBalance, dailyLimit int64, window time.Duration) (*domain.Transaction, *domain.Balance, error) {
	if f.balances.points[tx.ClientID] < minBalance {
		return nil, nil, domain.ErrInsufficientBalance
	}
	if dailyLimit > 0 {
		var redeemedInWindow int64
		cutoff := time.Now().Add(-window)
		for _, existing := range f.txs.all {
			if existing.ClientID == tx.ClientID && existing.Type == domain.TransactionRedeem && existing.CreatedAt.After(cutoff) {
				redeemedInWindow += -existing.PointsDelta
			}
		}
		if redeemedInWindow-tx.PointsDelta > dailyLimit {
			return nil, nil, domain.ErrDailyRedeemLimitExceeded
		}
	}
	return f.Post(ctx, tx)
}

// PostRefund mirrors the real repository's atomic refund posting (see
// internal/repository/postgres/ledger_repository.go's PostRefund) against
// this fake store's in-memory transactions/balances.
func (f *fakeLedgerRepo) PostRefund(_ context.Context, refund *domain.Transaction) (*domain.Transaction, *domain.Transaction, *domain.Balance, error) {
	for _, existing := range f.txs.all {
		if existing.StoreID == refund.StoreID && existing.Type == domain.TransactionRefund && existing.ExternalTxID == refund.ExternalTxID {
			return nil, nil, nil, domain.ErrConflict
		}
	}

	if refund.OriginalTransactionID == nil {
		return nil, nil, nil, fmt.Errorf("fakeLedgerRepo: PostRefund requires OriginalTransactionID")
	}
	var original *domain.Transaction
	for _, tx := range f.txs.all {
		if tx.ID == *refund.OriginalTransactionID && tx.StoreID == refund.StoreID {
			original = tx
			break
		}
	}
	if original == nil {
		return nil, nil, nil, domain.ErrNotFound
	}
	if original.Type == domain.TransactionRefund {
		return nil, nil, nil, domain.ErrCannotRefundRefund
	}

	newRefundedAmount := original.RefundedAmount.Add(refund.Amount)
	if newRefundedAmount.GreaterThan(original.Amount) {
		return nil, nil, nil, domain.ErrOverRefund
	}

	originalPoints := original.PointsDelta
	if originalPoints < 0 {
		originalPoints = -originalPoints
	}

	var cumulativePoints int64
	if newRefundedAmount.Equal(original.Amount) {
		cumulativePoints = originalPoints
	} else if original.Amount.IsPositive() {
		cumulativePoints = decimal.NewFromInt(originalPoints).Mul(newRefundedAmount).Div(original.Amount).Floor().IntPart()
	}
	thisRefundPoints := cumulativePoints - original.RefundedPoints

	var pointsDelta int64
	var allowNegativeBalance bool
	switch original.Type {
	case domain.TransactionAccrual:
		pointsDelta = -thisRefundPoints
		allowNegativeBalance = true
	case domain.TransactionRedeem:
		pointsDelta = thisRefundPoints
		allowNegativeBalance = false
	}

	original.RefundedAmount = newRefundedAmount
	original.RefundedPoints = cumulativePoints

	newBalance := f.balances.points[original.ClientID] + pointsDelta
	if !allowNegativeBalance && newBalance < 0 {
		return nil, nil, nil, domain.ErrInsufficientBalance
	}
	f.balances.points[original.ClientID] = newBalance

	f.nextID++
	posted := *refund
	posted.ID = f.nextID
	posted.ClientID = original.ClientID
	posted.Type = domain.TransactionRefund
	posted.PointsDelta = pointsDelta
	posted.BalanceAfter = newBalance
	posted.CreatedAt = time.Now()
	posted.OriginalTransactionID = &original.ID
	posted.RefundCumulativeAmount = newRefundedAmount
	posted.RefundFullyRefunded = newRefundedAmount.Equal(original.Amount)
	f.txs.all = append(f.txs.all, &posted)

	return &posted, original, &domain.Balance{ClientID: original.ClientID, Points: newBalance}, nil
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

func (f *fakeConfigRepo) Upsert(_ context.Context, cfg *domain.LoyaltyConfig, _ *int64) error {
	cfg.Version++
	f.cfg = cfg
	return nil
}

func (f *fakeConfigRepo) ListHistory(_ context.Context, storeID int64) ([]*domain.LoyaltyConfigVersion, error) {
	return nil, nil
}

type testDeps struct {
	svc      *LoyaltyService
	clients  *fakeClientRepo
	txs      *fakeTxRepo
	balances *fakeBalanceRepo
	configs  *fakeConfigRepo
}

func newTestService(cfg *domain.LoyaltyConfig) *testDeps {
	return newTestServiceWithAntiFraud(cfg, AntiFraudConfig{
		DailyRedeemPointsLimit: 1_000_000,
		DailyRedeemWindow:      24 * time.Hour,
	})
}

// newTestServiceWithAntiFraud is newTestService with a caller-supplied
// AntiFraudConfig, for tests that need to exercise the daily redeem limit
// itself rather than treat it as effectively unlimited.
func newTestServiceWithAntiFraud(cfg *domain.LoyaltyConfig, antiFraud AntiFraudConfig) *testDeps {
	clients := newFakeClientRepo()
	txs := &fakeTxRepo{}
	balances := newFakeBalanceRepo()
	configs := &fakeConfigRepo{cfg: cfg}
	ledger := &fakeLedgerRepo{txs: txs, balances: balances}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return &testDeps{
		svc:      New(log, clients, txs, balances, ledger, configs, nil, antiFraud),
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

// TestAccrueAndRedeem_StampRuleVersionFromEffectiveConfig guards Phase 2's
// rule-versioning requirement (docs/audit-remediation-prompt.md): a posted
// transaction must record which LoyaltyConfig.Version was in effect, so the
// calculation stays reconstructable after later config changes.
func TestAccrueAndRedeem_StampRuleVersionFromEffectiveConfig(t *testing.T) {
	cfg := pointsConfig(1)
	cfg.Version = 7
	deps := newTestService(cfg)
	ctx := context.Background()

	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-rv-1", Phone: "+70000000200", Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}
	accrual, err := deps.txs.GetByExternalID(ctx, 1, domain.TransactionAccrual, "rcpt-rv-1")
	if err != nil {
		t.Fatalf("GetByExternalID(accrual): %v", err)
	}
	if accrual.RuleVersion == nil || *accrual.RuleVersion != 7 {
		t.Fatalf("accrual RuleVersion = %v, want 7", accrual.RuleVersion)
	}

	client, err := deps.clients.GetByPhone(ctx, 1, "+70000000200")
	if err != nil {
		t.Fatalf("GetByPhone: %v", err)
	}
	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-rv-1", ClientID: client.ID, Points: 5, Amount: decimal.NewFromInt(50),
	}); err != nil {
		t.Fatalf("Redeem() error = %v", err)
	}
	redeem, err := deps.txs.GetByExternalID(ctx, 1, domain.TransactionRedeem, "redeem-rv-1")
	if err != nil {
		t.Fatalf("GetByExternalID(redeem): %v", err)
	}
	if redeem.RuleVersion == nil || *redeem.RuleVersion != 7 {
		t.Fatalf("redeem RuleVersion = %v, want 7", redeem.RuleVersion)
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

func TestRedeem_IdempotencyConflictOnMismatchedPoints(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000007", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	deps.balances.set(client.ID, 100)

	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-4", ClientID: client.ID, Points: 50, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("first Redeem() error = %v", err)
	}

	// Same transaction_id, same client and amount, but a different
	// requested points value — must not be treated as a replay of the
	// original 50-point redemption.
	_, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-4", ClientID: client.ID, Points: 10, Amount: decimal.NewFromInt(100),
	})
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("Redeem() with reused transaction_id but different points: error = %v, want domain.ErrIdempotencyConflict", err)
	}
}

func TestAccrue_ConflictingPhoneDoesNotCreateOrphanClient(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-4", Phone: "+70000000008", Amount: decimal.NewFromInt(200),
	}); err != nil {
		t.Fatalf("first Accrue() error = %v", err)
	}
	clientsBefore := len(deps.clients.byID)

	// Same transaction_id, but a phone that has never been seen before —
	// must be rejected as a conflict without registering a new client.
	_, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-4", Phone: "+70000000009", Amount: decimal.NewFromInt(200),
	})
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("Accrue() with reused transaction_id but a new phone: error = %v, want domain.ErrIdempotencyConflict", err)
	}
	if len(deps.clients.byID) != clientsBefore {
		t.Fatalf("client count = %d, want %d (conflicting request must not create an orphan client)", len(deps.clients.byID), clientsBefore)
	}
}

func TestRedeem_CapAppliedBeforeBalanceCheck(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000010", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	deps.balances.set(client.ID, 60)

	// balance=60, requested=80, cap (MaxRedeemPercent=50% of amount=100)=50:
	// balance can't cover the raw request but can cover the capped one, so
	// this must succeed, not report insufficient balance.
	result, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-5", ClientID: client.ID, Points: 80, Amount: decimal.NewFromInt(100),
	})
	if err != nil {
		t.Fatalf("Redeem() error = %v, want the cap to be applied before the balance check", err)
	}
	if result.PointsRedeemed != 50 || result.Balance != 10 {
		t.Fatalf("Redeem() = %+v, want PointsRedeemed=50 Balance=10", result)
	}
}

// --- Refund (DEC-012) ---

func TestRefund_FullAccrualRefund(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000100", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-100", Phone: client.Phone, Amount: decimal.NewFromInt(200),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}

	result, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-100", OriginalExternalTxID: "rcpt-100", Amount: decimal.NewFromInt(200),
	})
	if err != nil {
		t.Fatalf("Refund() error = %v", err)
	}
	if result.PointsReversed != -20 || result.Balance != 0 {
		t.Fatalf("Refund() = %+v, want PointsReversed=-20 Balance=0", result)
	}
	if !result.FullyRefunded {
		t.Fatalf("Refund().FullyRefunded = false, want true for a full refund")
	}
	if !result.RefundedAmountTotal.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("Refund().RefundedAmountTotal = %s, want 200", result.RefundedAmountTotal)
	}
}

func TestRefund_MultiplePartialRefundsWithRoundingCorrection(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000101", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	// AccrualPercent=10 on amount=100 -> 10 points earned.
	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-101", Phone: client.Phone, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}

	// Three uneven partial refunds: floor(10*33.34/100)=3, floor(10*66.67/100)=6,
	// then the final refund takes the exact remainder (4) rather than a
	// rounded share, so the three sum to exactly 10, not 3+3+3=9.
	r1, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-101a", OriginalExternalTxID: "rcpt-101", Amount: decimal.RequireFromString("33.34"),
	})
	if err != nil {
		t.Fatalf("first partial Refund() error = %v", err)
	}
	if r1.PointsReversed != -3 || r1.FullyRefunded {
		t.Fatalf("first partial Refund() = %+v, want PointsReversed=-3 FullyRefunded=false", r1)
	}

	r2, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-101b", OriginalExternalTxID: "rcpt-101", Amount: decimal.RequireFromString("33.33"),
	})
	if err != nil {
		t.Fatalf("second partial Refund() error = %v", err)
	}
	if r2.PointsReversed != -3 || r2.FullyRefunded {
		t.Fatalf("second partial Refund() = %+v, want PointsReversed=-3 FullyRefunded=false", r2)
	}

	r3, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-101c", OriginalExternalTxID: "rcpt-101", Amount: decimal.RequireFromString("33.33"),
	})
	if err != nil {
		t.Fatalf("third (completing) partial Refund() error = %v", err)
	}
	if r3.PointsReversed != -4 {
		t.Fatalf("third partial Refund().PointsReversed = %d, want -4 (rounding remainder absorbed)", r3.PointsReversed)
	}
	if !r3.FullyRefunded {
		t.Fatalf("third partial Refund().FullyRefunded = false, want true")
	}
	if r3.Balance != 0 {
		t.Fatalf("Refund() final balance = %d, want 0 (10 accrued, 3+3+4=10 reversed)", r3.Balance)
	}
}

func TestRefund_RedeemRefundReturnsPoints(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000102", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	deps.balances.set(client.ID, 20)

	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-102", ClientID: client.ID, Points: 10, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Redeem() error = %v", err)
	}

	result, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-102", OriginalExternalTxID: "redeem-102",
		OriginalType: domain.TransactionRedeem, Amount: decimal.NewFromInt(100),
	})
	if err != nil {
		t.Fatalf("Refund() error = %v", err)
	}
	if result.PointsReversed != 10 || result.Balance != 20 {
		t.Fatalf("Refund() = %+v, want PointsReversed=10 Balance=20 (points returned)", result)
	}
}

func TestRefund_RefundOfRefundRejected(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000103", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-103", Phone: client.Phone, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}
	if _, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-103", OriginalExternalTxID: "rcpt-103", Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Refund() error = %v", err)
	}

	// original_transaction_type=refund is rejected by the HTTP layer's input
	// validation before it ever reaches the service (see refund.go), but
	// the service itself must independently refuse to chain a refund off
	// another refund if ever called this way directly.
	_, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-103b", OriginalExternalTxID: "refund-103",
		OriginalType: domain.TransactionRefund, Amount: decimal.NewFromInt(50),
	})
	if !errors.Is(err, domain.ErrCannotRefundRefund) {
		t.Fatalf("Refund() of a refund: error = %v, want domain.ErrCannotRefundRefund", err)
	}
}

func TestRefund_CrossStoreReferenceRejected(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000104", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-104", Phone: client.Phone, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}

	_, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 2, ExternalTxID: "refund-104", OriginalExternalTxID: "rcpt-104", Amount: decimal.NewFromInt(100),
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Refund() referencing another store's transaction: error = %v, want domain.ErrNotFound", err)
	}
}

func TestRefund_OverRefundRejected(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000105", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-105", Phone: client.Phone, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}

	_, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-105", OriginalExternalTxID: "rcpt-105", Amount: decimal.NewFromInt(150),
	})
	if !errors.Is(err, domain.ErrOverRefund) {
		t.Fatalf("Refund() exceeding original amount: error = %v, want domain.ErrOverRefund", err)
	}
}

func TestRefund_IdempotentReplay(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000106", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-106", Phone: client.Phone, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}

	first, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-106", OriginalExternalTxID: "rcpt-106", Amount: decimal.NewFromInt(40),
	})
	if err != nil {
		t.Fatalf("first Refund() error = %v", err)
	}
	if first.Replayed {
		t.Fatalf("first Refund().Replayed = true, want false")
	}

	second, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-106", OriginalExternalTxID: "rcpt-106", Amount: decimal.NewFromInt(40),
	})
	if err != nil {
		t.Fatalf("replayed Refund() error = %v", err)
	}
	if !second.Replayed {
		t.Fatalf("replayed Refund().Replayed = false, want true")
	}
	if second.PointsReversed != first.PointsReversed || second.Balance != first.Balance {
		t.Fatalf("replayed Refund() = %+v, want it to match original %+v", second, first)
	}

	balance, _ := deps.balances.Get(ctx, client.ID)
	if balance.Points != 6 {
		t.Fatalf("balance after replay = %d, want 6 (10 accrued - 4 reversed once, not twice)", balance.Points)
	}
}

// TestRefund_ReplayOfEarlierPartialRefundReturnsStableSnapshot guards against
// two distinct regressions in a replayed partial refund's RefundedAmountTotal:
// (1) reporting just that one refund's own Amount instead of the cumulative
// total as of when it was posted, and (2) reporting the ORIGINAL row's
// current cumulative total, which drifts once later refunds are posted
// against it. Both would make a replayed response disagree with what the
// caller received the first time.
func TestRefund_ReplayOfEarlierPartialRefundReturnsStableSnapshot(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000109", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-109", Phone: client.Phone, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}

	first, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-109a", OriginalExternalTxID: "rcpt-109", Amount: decimal.NewFromInt(30),
	})
	if err != nil {
		t.Fatalf("first partial Refund() error = %v", err)
	}
	if !first.RefundedAmountTotal.Equal(decimal.NewFromInt(30)) || first.FullyRefunded {
		t.Fatalf("first partial Refund() = %+v, want RefundedAmountTotal=30 FullyRefunded=false", first)
	}

	// A second, later partial refund against the same original advances its
	// cumulative refunded_amount from 30 to 70.
	if _, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-109b", OriginalExternalTxID: "rcpt-109", Amount: decimal.NewFromInt(40),
	}); err != nil {
		t.Fatalf("second partial Refund() error = %v", err)
	}

	// Replaying the FIRST refund's external_tx_id must still report the
	// cumulative total as of that refund (30), not the original refund's own
	// amount (also 30, coincidentally) and not the original row's now-current
	// total (70) — so replay this again with an amount that would make the
	// three numbers distinguishable is unnecessary here since Amount==30
	// already differs from the post-second-refund total of 70.
	replay, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-109a", OriginalExternalTxID: "rcpt-109", Amount: decimal.NewFromInt(30),
	})
	if err != nil {
		t.Fatalf("replay of first partial Refund() error = %v", err)
	}
	if !replay.Replayed {
		t.Fatalf("replay of first partial Refund().Replayed = false, want true")
	}
	if !replay.RefundedAmountTotal.Equal(decimal.NewFromInt(30)) {
		t.Fatalf("replay of first partial Refund().RefundedAmountTotal = %s, want 30 (snapshot at time of that refund, not the drifted current total of 70)", replay.RefundedAmountTotal)
	}
	if replay.FullyRefunded {
		t.Fatalf("replay of first partial Refund().FullyRefunded = true, want false (it wasn't fully refunded when this refund was posted)")
	}

	// And replaying the SECOND refund must report its own snapshot (70, fully refunded), not 30.
	replay2, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-109b", OriginalExternalTxID: "rcpt-109", Amount: decimal.NewFromInt(40),
	})
	if err != nil {
		t.Fatalf("replay of second partial Refund() error = %v", err)
	}
	if !replay2.RefundedAmountTotal.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("replay of second partial Refund().RefundedAmountTotal = %s, want 70", replay2.RefundedAmountTotal)
	}
}

func TestRefund_FingerprintConflictOnMismatchedAmount(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000107", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-107", Phone: client.Phone, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}
	if _, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-107", OriginalExternalTxID: "rcpt-107", Amount: decimal.NewFromInt(40),
	}); err != nil {
		t.Fatalf("first Refund() error = %v", err)
	}

	_, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-107", OriginalExternalTxID: "rcpt-107", Amount: decimal.NewFromInt(50),
	})
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("Refund() with reused transaction_id but different amount: error = %v, want domain.ErrIdempotencyConflict", err)
	}
}

func TestRefund_AccrualRefundCanDriveBalanceNegative_OrdinaryRedeemCannot(t *testing.T) {
	deps := newTestService(pointsConfig(1))
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000108", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	// Accrue 10 points, spend 5, leaving a balance of 5.
	if _, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-108", Phone: client.Phone, Amount: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}
	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-108", ClientID: client.ID, Points: 5, Amount: decimal.NewFromInt(10),
	}); err != nil {
		t.Fatalf("Redeem() error = %v", err)
	}

	// Refunding the full original accrual reverses all 10 points even
	// though only 5 remain — balance must be allowed to go negative.
	refundResult, err := deps.svc.Refund(ctx, RefundRequest{
		StoreID: 1, ExternalTxID: "refund-108", OriginalExternalTxID: "rcpt-108", Amount: decimal.NewFromInt(100),
	})
	if err != nil {
		t.Fatalf("Refund() error = %v, want a refund of accrual to be allowed to go negative", err)
	}
	if refundResult.Balance != -5 {
		t.Fatalf("Refund().Balance = %d, want -5", refundResult.Balance)
	}

	// An ordinary redeem must still be rejected while the balance is negative.
	_, err = deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-108b", ClientID: client.ID, Points: 1, Amount: decimal.NewFromInt(10),
	})
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("Redeem() against a negative balance: error = %v, want domain.ErrInsufficientBalance", err)
	}

	// A subsequent accrual pays the negative balance down first.
	accrueResult, err := deps.svc.Accrue(ctx, AccrueRequest{
		StoreID: 1, ExternalTxID: "rcpt-108c", Phone: client.Phone, Amount: decimal.NewFromInt(100),
	})
	if err != nil {
		t.Fatalf("Accrue() error = %v", err)
	}
	if accrueResult.Balance != 5 {
		t.Fatalf("Accrue().Balance = %d, want 5 (-5 + 10 newly accrued)", accrueResult.Balance)
	}
}

// --- Anti-fraud: daily redeem limit (DEC-013) ---

func TestRedeem_DailyLimitAllowsUnderAndAtBoundary(t *testing.T) {
	deps := newTestServiceWithAntiFraud(pointsConfig(1), AntiFraudConfig{DailyRedeemPointsLimit: 30, DailyRedeemWindow: 24 * time.Hour})
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000200", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	deps.balances.set(client.ID, 100)

	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-200a", ClientID: client.ID, Points: 20, Amount: decimal.NewFromInt(1000),
	}); err != nil {
		t.Fatalf("Redeem() under the limit: error = %v", err)
	}

	// Exactly at the boundary: 20 + 10 = 30 == limit.
	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-200b", ClientID: client.ID, Points: 10, Amount: decimal.NewFromInt(1000),
	}); err != nil {
		t.Fatalf("Redeem() exactly at the limit boundary: error = %v", err)
	}
}

func TestRedeem_DailyLimitExceeded(t *testing.T) {
	deps := newTestServiceWithAntiFraud(pointsConfig(1), AntiFraudConfig{DailyRedeemPointsLimit: 30, DailyRedeemWindow: 24 * time.Hour})
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000201", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	deps.balances.set(client.ID, 100)

	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-201a", ClientID: client.ID, Points: 25, Amount: decimal.NewFromInt(1000),
	}); err != nil {
		t.Fatalf("first Redeem() error = %v", err)
	}

	_, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-201b", ClientID: client.ID, Points: 10, Amount: decimal.NewFromInt(1000),
	})
	if !errors.Is(err, domain.ErrDailyRedeemLimitExceeded) {
		t.Fatalf("Redeem() exceeding the daily limit: error = %v, want domain.ErrDailyRedeemLimitExceeded", err)
	}
}

func TestRedeem_DailyLimitRollingWindowExpires(t *testing.T) {
	deps := newTestServiceWithAntiFraud(pointsConfig(1), AntiFraudConfig{DailyRedeemPointsLimit: 30, DailyRedeemWindow: 24 * time.Hour})
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000202", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	deps.balances.set(client.ID, 100)

	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-202a", ClientID: client.ID, Points: 25, Amount: decimal.NewFromInt(1000),
	}); err != nil {
		t.Fatalf("first Redeem() error = %v", err)
	}
	// Backdate the redemption outside the 24h window, simulating time
	// having passed — a second redemption should no longer see it counted.
	deps.txs.all[len(deps.txs.all)-1].CreatedAt = time.Now().Add(-25 * time.Hour)

	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-202b", ClientID: client.ID, Points: 25, Amount: decimal.NewFromInt(1000),
	}); err != nil {
		t.Fatalf("Redeem() after the prior redemption aged out of the window: error = %v", err)
	}
}

func TestRedeem_DailyLimitIsolatedPerClient(t *testing.T) {
	deps := newTestServiceWithAntiFraud(pointsConfig(1), AntiFraudConfig{DailyRedeemPointsLimit: 30, DailyRedeemWindow: 24 * time.Hour})
	ctx := context.Background()

	clientA := &domain.Client{StoreID: 1, Phone: "+70000000203", CreatedAt: time.Now()}
	clientB := &domain.Client{StoreID: 1, Phone: "+70000000204", CreatedAt: time.Now()}
	for _, c := range []*domain.Client{clientA, clientB} {
		if err := deps.clients.Create(ctx, c); err != nil {
			t.Fatalf("seed client: %v", err)
		}
		deps.balances.set(c.ID, 100)
	}

	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-203", ClientID: clientA.ID, Points: 30, Amount: decimal.NewFromInt(1000),
	}); err != nil {
		t.Fatalf("client A Redeem() error = %v", err)
	}

	// Client B has spent nothing yet — must not be blocked by client A's usage.
	if _, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-204", ClientID: clientB.ID, Points: 30, Amount: decimal.NewFromInt(1000),
	}); err != nil {
		t.Fatalf("client B Redeem() error = %v, want isolation from client A's usage", err)
	}
}

func TestRedeem_DailyLimitConsidersCappedPointsNotRequested(t *testing.T) {
	// MaxRedeemPercent=50 means a request for 80 points against a 100-unit
	// purchase is capped to 50 actually-redeemed points — the daily limit
	// must be checked (and accumulate) against that capped, actually-spent
	// value, not the raw requested 80.
	deps := newTestServiceWithAntiFraud(pointsConfig(1), AntiFraudConfig{DailyRedeemPointsLimit: 50, DailyRedeemWindow: 24 * time.Hour})
	ctx := context.Background()

	client := &domain.Client{StoreID: 1, Phone: "+70000000205", CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, client); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	deps.balances.set(client.ID, 200)

	result, err := deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-205", ClientID: client.ID, Points: 80, Amount: decimal.NewFromInt(100),
	})
	if err != nil {
		t.Fatalf("Redeem() error = %v", err)
	}
	if result.PointsRedeemed != 50 {
		t.Fatalf("Redeem().PointsRedeemed = %d, want 50 (capped)", result.PointsRedeemed)
	}

	// Having actually spent exactly 50 (the cap), the daily limit (50) is
	// already exhausted — a second redemption of even 1 point must fail.
	_, err = deps.svc.Redeem(ctx, RedeemRequest{
		StoreID: 1, ExternalTxID: "redeem-205b", ClientID: client.ID, Points: 1, Amount: decimal.NewFromInt(100),
	})
	if !errors.Is(err, domain.ErrDailyRedeemLimitExceeded) {
		t.Fatalf("Redeem() after capped usage exhausted the limit: error = %v, want domain.ErrDailyRedeemLimitExceeded", err)
	}
}
