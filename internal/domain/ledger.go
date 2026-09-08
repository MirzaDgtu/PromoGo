package domain

import (
	"context"
	"time"
)

// LedgerRepository atomically posts a transaction and applies its point
// effect to the client's balance in a single database transaction. This
// must never be split into a separate TransactionRepository.Create followed
// by a BalanceRepository.Adjust call: if the first succeeds and the second
// fails, the transaction row becomes a permanent "ghost" — later replays of
// the same ExternalTxID (see TransactionRepository.GetByExternalID) would
// then find that ghost row and report it as already-processed forever,
// without the balance effect ever having actually applied.
type LedgerRepository interface {
	// Post inserts tx and adjusts tx.ClientID's balance by tx.PointsDelta
	// atomically, returning the posted transaction and resulting balance.
	// Returns domain.ErrConflict if a row already exists for
	// (tx.StoreID, tx.ExternalTxID) — a race between two concurrent
	// deliveries of the same webhook; the common idempotent-replay path is
	// detected earlier via TransactionRepository.GetByExternalID.
	Post(ctx context.Context, tx *Transaction) (*Transaction, *Balance, error)

	// PostRefund atomically posts a refund against
	// refund.OriginalTransactionID. refund.PointsDelta on input is ignored —
	// it is computed by this method, not the caller, because the correct
	// value depends on the original row's cumulative refunded_amount, which
	// can only be read safely under the row lock this method takes; a
	// caller-precomputed PointsDelta would be a stale-read race under
	// concurrent partial refunds.
	//
	// It locks the original row (SELECT ... FOR UPDATE) by
	// refund.OriginalTransactionID scoped to refund.StoreID (a mismatch —
	// wrong store or unknown ID — is domain.ErrNotFound), rejects a
	// refund-of-refund (domain.ErrCannotRefundRefund), and rejects an
	// over-refund (domain.ErrOverRefund) once refund.Amount is added to the
	// original's cumulative refunded_amount. It then derives this refund's
	// point delta proportionally from the original's point effect and the
	// new cumulative refunded amount (the final refund that completes the
	// original takes the exact remaining points rather than a rounded
	// share, so rounding never strands points), with sign and the balance
	// guard determined by the original's type: reversing an accrual is a
	// negative delta allowed to drive the balance negative (DEC-012);
	// reversing a redeem is a positive delta under the normal non-negative
	// guard. It updates the original row's cumulative refunded_amount/
	// refunded_points, adjusts the balance, and inserts the refund row —
	// all in one DB transaction, so two concurrent partial refunds against
	// the same original can never together exceed it. Returns
	// domain.ErrConflict if a row already exists for (refund.StoreID,
	// TransactionRefund, refund.ExternalTxID). original is returned
	// post-update so the caller can report refunded_amount_total and
	// fully_refunded without a second query.
	PostRefund(ctx context.Context, refund *Transaction) (posted *Transaction, original *Transaction, balance *Balance, err error)

	// PostRedeemChecked is Post for the redemption path, with an atomic
	// anti-fraud check added: it locks the balance row, sums this client's
	// already-redeemed points over the trailing window, and returns
	// domain.ErrDailyRedeemLimitExceeded — without writing anything — if
	// tx.PointsDelta (negative) would push that rolling-window total past
	// dailyLimit. Refund rows are excluded from the window sum (refunds
	// don't consume the daily limit, per DEC-013).
	PostRedeemChecked(ctx context.Context, tx *Transaction, dailyLimit int64, window time.Duration) (*Transaction, *Balance, error)
}
