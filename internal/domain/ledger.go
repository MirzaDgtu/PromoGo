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
	//
	// If notify is non-nil, its NotificationOutboxEntry is inserted in the
	// SAME database transaction as tx — the transactional-outbox guarantee
	// (Phase 3 of docs/audit-remediation-prompt.md): a notification is
	// queued if and only if the ledger write it describes actually
	// committed. Pass nil to post without queuing anything (e.g. a
	// zero-point accrual, which the caller decides isn't worth notifying
	// about).
	Post(ctx context.Context, tx *Transaction, notify *NotificationOutboxEntry) (*Transaction, *Balance, error)

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
	//
	// Unlike Post/PostRedeemChecked, the notification here is a builder
	// function, not a pre-built entry: PostRefund's point delta is only
	// known after this method computes it under the original row's lock
	// (see above), so the caller cannot render a message like "reversed N
	// points" before calling this. buildNotify is invoked with the final
	// pointsDelta once computed, still inside the same transaction as
	// everything else — its return value (nil to skip) is enqueued exactly
	// as Post's notify parameter is. May be nil to skip entirely.
	PostRefund(ctx context.Context, refund *Transaction, buildNotify func(pointsDelta int64) *NotificationOutboxEntry) (posted *Transaction, original *Transaction, balance *Balance, err error)

	// PostRedeemChecked is Post for the redemption path, with atomic
	// anti-fraud and eligibility checks added: it locks the balance row,
	// rejects with domain.ErrInsufficientBalance — without writing anything
	// — if the locked pre-redemption balance is below minBalance (a
	// service-layer pre-check against the same threshold is only a
	// fast-fail optimization; this locked check is the one that actually
	// prevents a concurrent redemption from letting the balance dip below
	// the store's configured minimum), sums this client's already-redeemed
	// points over the trailing window, and returns
	// domain.ErrDailyRedeemLimitExceeded if tx.PointsDelta (negative) would
	// push that rolling-window total past dailyLimit. Refund rows are
	// excluded from the window sum (refunds don't consume the daily limit,
	// per DEC-013). notify behaves as in Post.
	PostRedeemChecked(ctx context.Context, tx *Transaction, minBalance, dailyLimit int64, window time.Duration, notify *NotificationOutboxEntry) (*Transaction, *Balance, error)
}
