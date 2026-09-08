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

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/mechanicbuild"
)

// AntiFraudConfig bounds LoyaltyService's redemption-side anti-fraud check
// (see DEC-013). It is a deliberately narrow view of the application-wide
// config.AntiFraudConfig — internal/service doesn't import internal/config,
// matching how CustomerAuthConfig/OTPConfig are defined locally in this
// package rather than imported; internal/app/app.go maps the loaded config
// into this type when constructing LoyaltyService.
type AntiFraudConfig struct {
	// DailyRedeemPointsLimit caps the sum of points actually redeemed by a
	// single Client over DailyRedeemWindow. Zero disables the check
	// entirely (only valid in development — see config.validateAntiFraud).
	DailyRedeemPointsLimit int64
	DailyRedeemWindow      time.Duration
}

// LoyaltyService is the core loyalty-engine use case: it loads state, runs
// the store's configured domain.Mechanic, posts the transaction and balance
// change atomically via LedgerRepository, and best-effort notifies the
// client.
type LoyaltyService struct {
	log       *slog.Logger
	clients   domain.ClientRepository
	txs       domain.TransactionRepository
	balances  domain.BalanceRepository
	ledger    domain.LedgerRepository
	configs   domain.LoyaltyConfigRepository
	notifier  domain.NotificationChannel
	antiFraud AntiFraudConfig
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
	antiFraud AntiFraudConfig,
) *LoyaltyService {
	return &LoyaltyService{
		log:       log,
		clients:   clients,
		txs:       txs,
		balances:  balances,
		ledger:    ledger,
		configs:   configs,
		notifier:  notifier,
		antiFraud: antiFraud,
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
	ClientID     int64
	PointsEarned int64
	Balance      int64
	Replayed     bool
}

// accrualFingerprint canonically encodes the parameters of an accrual
// request. It's stored on the posted domain.Transaction and recomputed on
// every (StoreID, TransactionAccrual, ExternalTxID) lookup: a match means a
// genuine replay, a mismatch means the ID was reused for a different
// request (see replayedAccrue).
func accrualFingerprint(clientID int64, amount decimal.Decimal) string {
	return fmt.Sprintf("client=%d;amount=%s", clientID, amount.StringFixed(2))
}

// Accrue implements the accrual use case. It is idempotent on
// (StoreID, Type, ExternalTxID): a replayed webhook returns the original
// result without crediting points twice.
func (s *LoyaltyService) Accrue(ctx context.Context, req AccrueRequest) (*AccrueResult, error) {
	// Best-effort canonicalization so a Client created here matches the
	// same phone format CustomerAuthService stores on CustomerAccount,
	// letting VerifyOTP's client-linking find it by exact string equality
	// (see internal/auth.NormalizePhone). Falls back to the raw phone 1C
	// sent on any normalization failure — this must never reject or alter
	// behavior for input the webhook already accepted before this existed.
	if normalized, err := auth.NormalizePhone(req.Phone); err == nil {
		req.Phone = normalized
	}

	// Idempotency is checked before resolveOrCreateClient (which can have
	// the side effect of registering a new client) so that a request
	// reusing an already-processed ExternalTxID with a different phone
	// can't create an orphan client before being rejected as a conflict.
	if result, err := s.replayedAccrue(ctx, req.StoreID, req.ExternalTxID, req.Phone, req.Amount); result != nil || err != nil {
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
		StoreID:            req.StoreID,
		ClientID:           client.ID,
		ExternalTxID:       req.ExternalTxID,
		Amount:             req.Amount,
		Type:               domain.TransactionAccrual,
		RequestFingerprint: accrualFingerprint(client.ID, req.Amount),
	}

	pointsEarned, err := mechanic.Accrue(ctx, tx, cfg, balanceBefore)
	if err != nil {
		return nil, fmt.Errorf("accrue: run mechanic %q: %w", mechanic.Name(), err)
	}
	tx.PointsDelta = pointsEarned

	posted, balance, err := s.ledger.Post(ctx, tx)
	if errors.Is(err, domain.ErrConflict) {
		// Lost a race with a concurrent replay of the same webhook.
		return s.replayedAccrue(ctx, req.StoreID, req.ExternalTxID, req.Phone, req.Amount)
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

	return &AccrueResult{ClientID: client.ID, PointsEarned: posted.PointsDelta, Balance: balance.Points}, nil
}

// replayedAccrue returns a non-nil AccrueResult if (storeID,
// TransactionAccrual, externalTxID) was already processed, nil (with a nil
// error) if not yet processed, or domain.ErrIdempotencyConflict if a
// transaction exists under that ID but its stored fingerprint doesn't match
// phone/amount — a colliding externalTxID reused for a materially different
// request, which must not be treated as a replay of the original.
//
// This is called before any client is resolved/created (see Accrue), so on
// a match it resolves the client via a read-only GetByPhone rather than
// resolveOrCreateClient — never registering a client just to check
// idempotency. If GetByPhone also can't find a client, that itself proves
// the stored row belongs to a different request (a genuine replay would
// have already registered this phone the first time).
func (s *LoyaltyService) replayedAccrue(ctx context.Context, storeID int64, externalTxID, phone string, amount decimal.Decimal) (*AccrueResult, error) {
	existing, err := s.txs.GetByExternalID(ctx, storeID, domain.TransactionAccrual, externalTxID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("accrue: check idempotency: %w", err)
	}

	client, err := s.clients.GetByPhone(ctx, storeID, phone)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrIdempotencyConflict
	}
	if err != nil {
		return nil, fmt.Errorf("accrue: check idempotency: resolve client: %w", err)
	}
	if existing.RequestFingerprint != accrualFingerprint(client.ID, amount) {
		return nil, domain.ErrIdempotencyConflict
	}

	return &AccrueResult{ClientID: existing.ClientID, PointsEarned: existing.PointsDelta, Balance: existing.BalanceAfter, Replayed: true}, nil
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
		if errors.Is(err, domain.ErrConflict) {
			// Lost a race with a concurrent registration of the same phone.
			return s.clients.GetByPhone(ctx, storeID, phone)
		}
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
// on (StoreID, Type, ExternalTxID). Returns domain.ErrNotFound if ClientID
// doesn't belong to StoreID.
func (s *LoyaltyService) Redeem(ctx context.Context, req RedeemRequest) (*RedeemResult, error) {
	client, err := s.clients.GetByID(ctx, req.ClientID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("redeem: load client: %w", err)
	}
	if client.StoreID != req.StoreID {
		// Don't distinguish "wrong store" from "doesn't exist" — a store's
		// API key must not be able to enumerate other stores' client IDs.
		return nil, domain.ErrNotFound
	}

	if result, err := s.replayedRedeem(ctx, req.StoreID, req.ExternalTxID, req.ClientID, req.Amount, req.Points); result != nil || err != nil {
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

	if balance.Points < cfg.MinBalanceToRedeem {
		return nil, domain.ErrInsufficientBalance
	}

	// Cap first, then validate the balance against the final (post-cap)
	// points value — checking balance.Points < req.Points here would reject
	// a redemption that the cap alone would already have brought within
	// reach of the balance.
	points := req.Points
	if cfg.MaxRedeemPercent.IsPositive() && cfg.PointsExchangeRate.IsPositive() {
		maxPoints := req.Amount.Mul(cfg.MaxRedeemPercent).Div(decimal.NewFromInt(100)).Div(cfg.PointsExchangeRate).Floor().IntPart()
		if points > maxPoints {
			points = maxPoints
		}
	}
	if points <= 0 || balance.Points < points {
		return nil, domain.ErrInsufficientBalance
	}

	tx := &domain.Transaction{
		StoreID:            req.StoreID,
		ClientID:           req.ClientID,
		ExternalTxID:       req.ExternalTxID,
		Amount:             req.Amount,
		Type:               domain.TransactionRedeem,
		PointsDelta:        -points,
		RequestFingerprint: redeemFingerprint(req.ClientID, req.Amount, req.Points),
	}

	posted, newBalance, err := s.ledger.PostRedeemChecked(ctx, tx, s.antiFraud.DailyRedeemPointsLimit, s.antiFraud.DailyRedeemWindow)
	if errors.Is(err, domain.ErrConflict) {
		return s.replayedRedeem(ctx, req.StoreID, req.ExternalTxID, req.ClientID, req.Amount, req.Points)
	}
	if err != nil {
		return nil, fmt.Errorf("redeem: post transaction: %w", err)
	}

	if s.notifier != nil {
		msg := fmt.Sprintf("Списано %d баллов", -posted.PointsDelta)
		if err := s.notifier.Send(ctx, req.ClientID, msg); err != nil {
			s.log.WarnContext(ctx, "send redeem notification", "client_id", req.ClientID, "error", err)
		}
	}

	return &RedeemResult{PointsRedeemed: -posted.PointsDelta, Balance: newBalance.Points}, nil
}

// redeemFingerprint canonically encodes the parameters of a redemption
// request, using the raw requested points (before MaxRedeemPercent
// capping) — a replay is defined by "same request", independent of the cap
// logic in Redeem. See accrualFingerprint for the accrual-side analogue.
func redeemFingerprint(clientID int64, amount decimal.Decimal, points int64) string {
	return fmt.Sprintf("client=%d;amount=%s;points=%d", clientID, amount.StringFixed(2), points)
}

// replayedRedeem mirrors replayedAccrue for the redemption flow. Unlike
// Accrue, Redeem already resolves and validates the client (for store
// isolation) before this is called, so no extra lookup is needed here to
// compute the fingerprint.
func (s *LoyaltyService) replayedRedeem(ctx context.Context, storeID int64, externalTxID string, clientID int64, amount decimal.Decimal, points int64) (*RedeemResult, error) {
	existing, err := s.txs.GetByExternalID(ctx, storeID, domain.TransactionRedeem, externalTxID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redeem: check idempotency: %w", err)
	}
	if existing.RequestFingerprint != redeemFingerprint(clientID, amount, points) {
		return nil, domain.ErrIdempotencyConflict
	}

	return &RedeemResult{PointsRedeemed: -existing.PointsDelta, Balance: existing.BalanceAfter, Replayed: true}, nil
}

// RefundRequest mirrors POST /api/v1/transactions/refund's payload.
// OriginalType is optional: external_tx_id uniqueness is scoped per type
// (see transaction.go), so if a store legitimately reused the same receipt
// number for both an accrual and a redeem, Refund can't tell which one
// OriginalExternalTxID refers to without it — see domain.ErrAmbiguousOriginalTransaction.
type RefundRequest struct {
	StoreID              int64
	ExternalTxID         string
	OriginalExternalTxID string
	OriginalType         domain.TransactionType
	Amount               decimal.Decimal
}

// RefundResult is the response to a refund request.
type RefundResult struct {
	ClientID            int64
	PointsReversed      int64
	Balance             int64
	RefundedAmountTotal decimal.Decimal
	FullyRefunded       bool
	Replayed            bool
}

// refundFingerprint canonically encodes the parameters of a refund request.
// Unlike accrual/redeem, a refund's identity includes the original
// transaction's ID (not just the requested amount) — two refunds against
// different originals must never be treated as replays of each other even
// if they happen to share an external_tx_id under different Type scoping
// (they can't, since Type is part of the idempotency key, but the
// fingerprint still encodes it for clarity and defense in depth).
func refundFingerprint(originalTransactionID int64, amount decimal.Decimal) string {
	return fmt.Sprintf("original=%d;amount=%s", originalTransactionID, amount.StringFixed(2))
}

// Refund implements the refund use case (DEC-012): it locates the original
// accrual/redeem transaction, then delegates the proportional point-delta
// computation, over-refund check, and atomic posting to
// LedgerRepository.PostRefund — see its doc comment for why that math can't
// safely live here. Like Accrue/Redeem, it is idempotent on
// (StoreID, TransactionRefund, ExternalTxID).
func (s *LoyaltyService) Refund(ctx context.Context, req RefundRequest) (*RefundResult, error) {
	original, err := s.resolveOriginalTransaction(ctx, req.StoreID, req.OriginalExternalTxID, req.OriginalType)
	if err != nil {
		return nil, err
	}

	if result, err := s.replayedRefund(ctx, req.StoreID, req.ExternalTxID, original.ID, req.Amount); result != nil || err != nil {
		return result, err
	}

	refundTx := &domain.Transaction{
		StoreID:               req.StoreID,
		ExternalTxID:          req.ExternalTxID,
		Amount:                req.Amount,
		Type:                  domain.TransactionRefund,
		RequestFingerprint:    refundFingerprint(original.ID, req.Amount),
		OriginalTransactionID: &original.ID,
	}

	posted, updatedOriginal, balance, err := s.ledger.PostRefund(ctx, refundTx)
	if errors.Is(err, domain.ErrConflict) {
		return s.replayedRefund(ctx, req.StoreID, req.ExternalTxID, original.ID, req.Amount)
	}
	if err != nil {
		return nil, fmt.Errorf("refund: post transaction: %w", err)
	}

	if s.notifier != nil {
		msg := fmt.Sprintf("Возврат: скорректировано %d баллов", posted.PointsDelta)
		if err := s.notifier.Send(ctx, posted.ClientID, msg); err != nil {
			s.log.WarnContext(ctx, "send refund notification", "client_id", posted.ClientID, "error", err)
		}
	}

	return &RefundResult{
		ClientID:            posted.ClientID,
		PointsReversed:      posted.PointsDelta,
		Balance:             balance.Points,
		RefundedAmountTotal: updatedOriginal.RefundedAmount,
		FullyRefunded:       updatedOriginal.RefundedAmount.Equal(updatedOriginal.Amount),
	}, nil
}

// resolveOriginalTransaction finds the accrual/redeem RefundRequest.OriginalExternalTxID
// refers to under req.StoreID. If originalType is empty it tries both
// accrual and redeem — exactly one match proceeds, zero is
// domain.ErrNotFound, and two (the same external_tx_id legitimately reused
// across an accrual and a redeem) is domain.ErrAmbiguousOriginalTransaction.
func (s *LoyaltyService) resolveOriginalTransaction(ctx context.Context, storeID int64, originalExternalTxID string, originalType domain.TransactionType) (*domain.Transaction, error) {
	if originalType != "" {
		tx, err := s.txs.GetByExternalID(ctx, storeID, originalType, originalExternalTxID)
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("refund: resolve original transaction: %w", err)
		}
		return tx, nil
	}

	accrual, err := s.txs.GetByExternalID(ctx, storeID, domain.TransactionAccrual, originalExternalTxID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("refund: resolve original transaction: %w", err)
	}
	redeem, err := s.txs.GetByExternalID(ctx, storeID, domain.TransactionRedeem, originalExternalTxID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("refund: resolve original transaction: %w", err)
	}

	switch {
	case accrual != nil && redeem != nil:
		return nil, domain.ErrAmbiguousOriginalTransaction
	case accrual != nil:
		return accrual, nil
	case redeem != nil:
		return redeem, nil
	default:
		return nil, domain.ErrNotFound
	}
}

// replayedRefund mirrors replayedAccrue/replayedRedeem: a non-nil result
// means (storeID, TransactionRefund, externalTxID) was already processed,
// nil/nil means it wasn't yet, and domain.ErrIdempotencyConflict means the
// ID was reused against a different original transaction or amount.
func (s *LoyaltyService) replayedRefund(ctx context.Context, storeID int64, externalTxID string, originalTransactionID int64, amount decimal.Decimal) (*RefundResult, error) {
	existing, err := s.txs.GetByExternalID(ctx, storeID, domain.TransactionRefund, externalTxID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("refund: check idempotency: %w", err)
	}
	if existing.RequestFingerprint != refundFingerprint(originalTransactionID, amount) {
		return nil, domain.ErrIdempotencyConflict
	}

	fullyRefunded := false
	if existing.OriginalTransactionID != nil {
		if o, oerr := s.txs.GetByID(ctx, *existing.OriginalTransactionID); oerr == nil {
			fullyRefunded = o.RefundedAmount.Equal(o.Amount)
		}
	}

	return &RefundResult{
		ClientID:            existing.ClientID,
		PointsReversed:      existing.PointsDelta,
		Balance:             existing.BalanceAfter,
		RefundedAmountTotal: existing.Amount,
		FullyRefunded:       fullyRefunded,
		Replayed:            true,
	}, nil
}
