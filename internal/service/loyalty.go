// Package service holds PromoGo's core use cases: point accrual and
// redemption for a single store's transactions.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/mechanicbuild"
)

// LoyaltyService is the core loyalty-engine use case: it loads state, runs
// the store's configured domain.Mechanic, posts the transaction and balance
// change atomically via LedgerRepository, and best-effort notifies the
// client.
type LoyaltyService struct {
	log      *slog.Logger
	clients  domain.ClientRepository
	txs      domain.TransactionRepository
	balances domain.BalanceRepository
	ledger   domain.LedgerRepository
	configs  domain.LoyaltyConfigRepository
	notifier domain.NotificationChannel
}

// New constructs a LoyaltyService.
func New(
	log *slog.Logger,
	clients domain.ClientRepository,
	txs domain.TransactionRepository,
	balances domain.BalanceRepository,
	ledger domain.LedgerRepository,
	configs domain.LoyaltyConfigRepository,
	notifier domain.NotificationChannel,
) *LoyaltyService {
	return &LoyaltyService{
		log:      log,
		clients:  clients,
		txs:      txs,
		balances: balances,
		ledger:   ledger,
		configs:  configs,
		notifier: notifier,
	}
}

// AccrueRequest mirrors 1C's webhook payload (Idea.md: POST /api/v1/transactions).
type AccrueRequest struct {
	StoreID      int64
	ExternalTxID string
	Phone        string
	Amount       decimal.Decimal
}

// AccrueResult mirrors Idea.md's webhook response shape. Replayed is true
// when ExternalTxID was already processed — the caller (1C) retried a
// timed-out request and this response reflects the original result, not a
// second accrual.
type AccrueResult struct {
	PointsEarned int64
	Balance      int64
	Replayed     bool
}

// Accrue implements the accrual use case. It is idempotent on
// (StoreID, ExternalTxID): a replayed webhook returns the original result
// without crediting points twice.
func (s *LoyaltyService) Accrue(ctx context.Context, req AccrueRequest) (*AccrueResult, error) {
	if result, err := s.replayedAccrue(ctx, req.StoreID, req.ExternalTxID); result != nil || err != nil {
		return result, err
	}

	client, err := s.resolveOrCreateClient(ctx, req.StoreID, req.Phone)
	if err != nil {
		return nil, fmt.Errorf("accrue: resolve client: %w", err)
	}

	cfg, err := s.configs.GetByStore(ctx, req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("accrue: load loyalty config: %w", err)
	}

	mechanic, err := mechanicbuild.Build(cfg.Mechanic)
	if err != nil {
		return nil, fmt.Errorf("accrue: build mechanic: %w", err)
	}

	balanceBefore, err := s.balances.Get(ctx, client.ID)
	if err != nil {
		return nil, fmt.Errorf("accrue: load balance: %w", err)
	}

	tx := &domain.Transaction{
		StoreID:      req.StoreID,
		ClientID:     client.ID,
		ExternalTxID: req.ExternalTxID,
		Amount:       req.Amount,
		Type:         domain.TransactionAccrual,
	}

	pointsEarned, err := mechanic.Accrue(ctx, tx, cfg, balanceBefore)
	if err != nil {
		return nil, fmt.Errorf("accrue: run mechanic %q: %w", mechanic.Name(), err)
	}
	tx.PointsDelta = pointsEarned

	posted, balance, err := s.ledger.Post(ctx, tx)
	if errors.Is(err, domain.ErrConflict) {
		// Lost a race with a concurrent replay of the same webhook.
		return s.replayedAccrue(ctx, req.StoreID, req.ExternalTxID)
	}
	if err != nil {
		return nil, fmt.Errorf("accrue: post transaction: %w", err)
	}

	if pointsEarned > 0 && s.notifier != nil {
		msg := fmt.Sprintf("Вам начислено %d баллов", pointsEarned)
		if err := s.notifier.Send(ctx, client.ID, msg); err != nil {
			s.log.WarnContext(ctx, "send accrual notification", "client_id", client.ID, "error", err)
		}
	}

	return &AccrueResult{PointsEarned: posted.PointsDelta, Balance: balance.Points}, nil
}

// replayedAccrue returns a non-nil AccrueResult if (storeID, externalTxID)
// was already processed, nil (with a nil error) if not yet processed, or a
// non-nil error if the lookup itself failed.
func (s *LoyaltyService) replayedAccrue(ctx context.Context, storeID int64, externalTxID string) (*AccrueResult, error) {
	existing, err := s.txs.GetByExternalID(ctx, storeID, externalTxID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("accrue: check idempotency: %w", err)
	}

	balance, err := s.balances.Get(ctx, existing.ClientID)
	if err != nil {
		return nil, fmt.Errorf("accrue: load balance for replay: %w", err)
	}
	return &AccrueResult{PointsEarned: existing.PointsDelta, Balance: balance.Points, Replayed: true}, nil
}

func (s *LoyaltyService) resolveOrCreateClient(ctx context.Context, storeID int64, phone string) (*domain.Client, error) {
	client, err := s.clients.GetByPhone(ctx, storeID, phone)
	if err == nil {
		return client, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	client = &domain.Client{StoreID: storeID, Phone: phone, CreatedAt: time.Now()}
	if err := s.clients.Create(ctx, client); err != nil {
		return nil, err
	}
	return client, nil
}

// RedeemRequest mirrors Idea.md's POST /api/v1/transactions/redeem payload.
// Amount is the purchase amount being paid down with points, used to cap
// redemption at LoyaltyConfig.MaxRedeemPercent.
type RedeemRequest struct {
	StoreID      int64
	ExternalTxID string
	ClientID     int64
	Points       int64
	Amount       decimal.Decimal
}

// RedeemResult is the response to a redemption request.
type RedeemResult struct {
	PointsRedeemed int64
	Balance        int64
	Replayed       bool
}

// Redeem implements the redemption use case. Like Accrue, it is idempotent
// on (StoreID, ExternalTxID).
func (s *LoyaltyService) Redeem(ctx context.Context, req RedeemRequest) (*RedeemResult, error) {
	if result, err := s.replayedRedeem(ctx, req.StoreID, req.ExternalTxID); result != nil || err != nil {
		return result, err
	}

	cfg, err := s.configs.GetByStore(ctx, req.StoreID)
	if err != nil {
		return nil, fmt.Errorf("redeem: load loyalty config: %w", err)
	}

	balance, err := s.balances.Get(ctx, req.ClientID)
	if err != nil {
		return nil, fmt.Errorf("redeem: load balance: %w", err)
	}

	if balance.Points < cfg.MinBalanceToRedeem || balance.Points < req.Points {
		return nil, domain.ErrInsufficientBalance
	}

	points := req.Points
	if cfg.MaxRedeemPercent.IsPositive() && cfg.PointsExchangeRate.IsPositive() {
		maxPoints := req.Amount.Mul(cfg.MaxRedeemPercent).Div(decimal.NewFromInt(100)).Div(cfg.PointsExchangeRate).Floor().IntPart()
		if points > maxPoints {
			points = maxPoints
		}
	}
	if points <= 0 {
		return nil, domain.ErrInsufficientBalance
	}

	tx := &domain.Transaction{
		StoreID:      req.StoreID,
		ClientID:     req.ClientID,
		ExternalTxID: req.ExternalTxID,
		Amount:       req.Amount,
		Type:         domain.TransactionRedeem,
		PointsDelta:  -points,
	}

	posted, newBalance, err := s.ledger.Post(ctx, tx)
	if errors.Is(err, domain.ErrConflict) {
		return s.replayedRedeem(ctx, req.StoreID, req.ExternalTxID)
	}
	if err != nil {
		return nil, fmt.Errorf("redeem: post transaction: %w", err)
	}

	return &RedeemResult{PointsRedeemed: -posted.PointsDelta, Balance: newBalance.Points}, nil
}

// replayedRedeem mirrors replayedAccrue for the redemption flow.
func (s *LoyaltyService) replayedRedeem(ctx context.Context, storeID int64, externalTxID string) (*RedeemResult, error) {
	existing, err := s.txs.GetByExternalID(ctx, storeID, externalTxID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redeem: check idempotency: %w", err)
	}

	balance, err := s.balances.Get(ctx, existing.ClientID)
	if err != nil {
		return nil, fmt.Errorf("redeem: load balance for replay: %w", err)
	}
	return &RedeemResult{PointsRedeemed: -existing.PointsDelta, Balance: balance.Points, Replayed: true}, nil
}
