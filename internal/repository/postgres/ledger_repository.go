package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// LedgerRepository is a pgx-backed implementation of domain.LedgerRepository.
type LedgerRepository struct {
	pool *pgxpool.Pool
}

// NewLedgerRepository creates a LedgerRepository backed by pool.
func NewLedgerRepository(pool *pgxpool.Pool) *LedgerRepository {
	return &LedgerRepository{pool: pool}
}

// insertOutboxEntry inserts notify inside the caller's already-open dbTx —
// the transactional-outbox guarantee lives entirely in this being called
// before dbTx.Commit, never after. ON CONFLICT (dedupe_key) DO NOTHING
// makes a double-call with the same DedupeKey a harmless no-op rather than
// a constraint-violation error, since the ledger write it accompanies is
// itself already idempotent on the same logical key.
func insertOutboxEntry(ctx context.Context, dbTx pgx.Tx, notify *domain.NotificationOutboxEntry) error {
	if notify == nil {
		return nil
	}
	const insert = `
		INSERT INTO notification_outbox (client_id, template_id, template_version, message, dedupe_key)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (dedupe_key) DO NOTHING`
	if _, err := dbTx.Exec(ctx, insert, notify.ClientID, notify.TemplateID, notify.TemplateVersion, notify.Message, notify.DedupeKey); err != nil {
		return fmt.Errorf("enqueue notification outbox entry: %w", err)
	}
	return nil
}

// Post implements domain.LedgerRepository: inserting the transaction and
// adjusting the balance happen in one database transaction, so the two can
// never diverge (see domain.LedgerRepository's doc comment).
//
// The balance update deliberately isn't a single
// "INSERT ... ON CONFLICT DO UPDATE SET points = balances.points + EXCLUDED.points"
// statement: Postgres validates the balances_points_check CHECK constraint
// against the raw EXCLUDED values before conflict resolution picks the
// UPDATE branch, so a negative points_delta on an existing row with a
// sufficient balance would still spuriously fail the check against the
// negative delta alone (confirmed against Postgres 16). Ensuring the row
// exists first, then applying the delta with a plain UPDATE, checks the
// constraint against the real post-update value instead.
func (r *LedgerRepository) Post(ctx context.Context, tx *domain.Transaction, notify *domain.NotificationOutboxEntry) (*domain.Transaction, *domain.Balance, error) {
	dbTx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("post transaction %d/%s: begin tx: %w", tx.StoreID, tx.ExternalTxID, err)
	}
	defer dbTx.Rollback(ctx)

	const ensureBalanceRow = `INSERT INTO balances (client_id, points) VALUES ($1, 0) ON CONFLICT (client_id) DO NOTHING`
	if _, err := dbTx.Exec(ctx, ensureBalanceRow, tx.ClientID); err != nil {
		return nil, nil, fmt.Errorf("post transaction %d/%s: ensure balance row: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	// The "AND points + $2 >= 0" guard makes redemption atomic: without it,
	// a concurrent redemption could pass the service layer's balance
	// pre-check and still race here, tripping the balances_points_check
	// CHECK constraint and surfacing as a raw DB error instead of
	// domain.ErrInsufficientBalance. For accrual (non-negative delta) the
	// guard is always true given the points>=0 invariant, so it's a no-op.
	// This runs before the transaction row is inserted so the row can carry
	// its resulting balance_after directly, rather than needing a second
	// UPDATE after computing it.
	const adjustBalance = `UPDATE balances SET points = points + $2 WHERE client_id = $1 AND points + $2 >= 0 RETURNING points`
	balance := &domain.Balance{ClientID: tx.ClientID}
	err = dbTx.QueryRow(ctx, adjustBalance, tx.ClientID, tx.PointsDelta).Scan(&balance.Points)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, domain.ErrInsufficientBalance
	}
	if err != nil {
		return nil, nil, fmt.Errorf("post transaction %d/%s: adjust balance: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	if tx.Currency == "" {
		tx.Currency = domain.DefaultCurrency
	}
	const insert = `
		INSERT INTO transactions (store_id, client_id, external_tx_id, amount, currency, type, points_delta, balance_after, request_fingerprint, created_at, rule_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), $10)
		RETURNING id, created_at`
	err = dbTx.QueryRow(ctx, insert, tx.StoreID, tx.ClientID, tx.ExternalTxID, tx.Amount, tx.Currency, tx.Type, tx.PointsDelta, balance.Points, tx.RequestFingerprint, tx.RuleVersion).
		Scan(&tx.ID, &tx.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, nil, fmt.Errorf("post transaction %d/%s: %w", tx.StoreID, tx.ExternalTxID, domain.ErrConflict)
		}
		return nil, nil, fmt.Errorf("post transaction %d/%s: insert: %w", tx.StoreID, tx.ExternalTxID, err)
	}
	tx.BalanceAfter = balance.Points

	if err := insertOutboxEntry(ctx, dbTx, notify); err != nil {
		return nil, nil, fmt.Errorf("post transaction %d/%s: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("post transaction %d/%s: commit: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	return tx, balance, nil
}

const selectTransactionColumns = `id, store_id, client_id, external_tx_id, amount, currency, type, points_delta, balance_after, request_fingerprint, created_at, original_transaction_id, refunded_amount, refunded_points, refund_cumulative_amount, refund_fully_refunded, rule_version`

func scanTransactionRow(row pgx.Row) (*domain.Transaction, error) {
	tx := &domain.Transaction{}
	err := row.Scan(
		&tx.ID, &tx.StoreID, &tx.ClientID, &tx.ExternalTxID, &tx.Amount, &tx.Currency, &tx.Type, &tx.PointsDelta, &tx.BalanceAfter, &tx.RequestFingerprint, &tx.CreatedAt,
		&tx.OriginalTransactionID, &tx.RefundedAmount, &tx.RefundedPoints, &tx.RefundCumulativeAmount, &tx.RefundFullyRefunded, &tx.RuleVersion,
	)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// PostRefund implements domain.LedgerRepository.PostRefund — see its doc
// comment for the full contract. All point-delta math happens here, under
// the original row's lock, rather than in the caller: it depends on
// refunded_amount, which is only safe to read at the moment of writing.
func (r *LedgerRepository) PostRefund(ctx context.Context, refund *domain.Transaction, buildNotify func(pointsDelta int64) *domain.NotificationOutboxEntry) (*domain.Transaction, *domain.Transaction, *domain.Balance, error) {
	dbTx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: begin tx: %w", refund.StoreID, refund.ExternalTxID, err)
	}
	defer dbTx.Rollback(ctx)

	if refund.OriginalTransactionID == nil {
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: original_transaction_id is required", refund.StoreID, refund.ExternalTxID)
	}

	const lockOriginal = `SELECT ` + selectTransactionColumns + `
		FROM transactions
		WHERE id = $1 AND store_id = $2
		FOR UPDATE`
	original, err := scanTransactionRow(dbTx.QueryRow(ctx, lockOriginal, *refund.OriginalTransactionID, refund.StoreID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: original transaction %d: %w", refund.StoreID, refund.ExternalTxID, *refund.OriginalTransactionID, domain.ErrNotFound)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: lock original: %w", refund.StoreID, refund.ExternalTxID, err)
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
		// This refund completes the original: take the exact remaining
		// points rather than a rounded share, so proportional rounding on
		// earlier partial refunds never strands unreturned/unrecovered
		// points.
		cumulativePoints = originalPoints
	} else if original.Amount.IsPositive() {
		cumulativePoints = decimal.NewFromInt(originalPoints).
			Mul(newRefundedAmount).
			Div(original.Amount).
			Floor().IntPart()
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
	default:
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: original transaction has unrefundable type %q", refund.StoreID, refund.ExternalTxID, original.Type)
	}
	refund.PointsDelta = pointsDelta
	refund.ClientID = original.ClientID

	const updateOriginal = `UPDATE transactions SET refunded_amount = $2, refunded_points = $3 WHERE id = $1`
	if _, err := dbTx.Exec(ctx, updateOriginal, original.ID, newRefundedAmount, cumulativePoints); err != nil {
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: update original cumulative refund: %w", refund.StoreID, refund.ExternalTxID, err)
	}
	original.RefundedAmount = newRefundedAmount
	original.RefundedPoints = cumulativePoints

	const ensureBalanceRow = `INSERT INTO balances (client_id, points) VALUES ($1, 0) ON CONFLICT (client_id) DO NOTHING`
	if _, err := dbTx.Exec(ctx, ensureBalanceRow, refund.ClientID); err != nil {
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: ensure balance row: %w", refund.StoreID, refund.ExternalTxID, err)
	}

	balance := &domain.Balance{ClientID: refund.ClientID}
	if allowNegativeBalance {
		const adjustBalanceUnguarded = `UPDATE balances SET points = points + $2 WHERE client_id = $1 RETURNING points`
		if err := dbTx.QueryRow(ctx, adjustBalanceUnguarded, refund.ClientID, pointsDelta).Scan(&balance.Points); err != nil {
			return nil, nil, nil, fmt.Errorf("post refund %d/%s: adjust balance: %w", refund.StoreID, refund.ExternalTxID, err)
		}
	} else {
		const adjustBalanceGuarded = `UPDATE balances SET points = points + $2 WHERE client_id = $1 AND points + $2 >= 0 RETURNING points`
		err := dbTx.QueryRow(ctx, adjustBalanceGuarded, refund.ClientID, pointsDelta).Scan(&balance.Points)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil, domain.ErrInsufficientBalance
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("post refund %d/%s: adjust balance: %w", refund.StoreID, refund.ExternalTxID, err)
		}
	}

	fullyRefunded := newRefundedAmount.Equal(original.Amount)
	refund.Currency = original.Currency

	const insert = `
		INSERT INTO transactions (store_id, client_id, external_tx_id, amount, currency, type, points_delta, balance_after, request_fingerprint, created_at, original_transaction_id, refund_cumulative_amount, refund_fully_refunded)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), $10, $11, $12)
		RETURNING id, created_at`
	err = dbTx.QueryRow(ctx, insert, refund.StoreID, refund.ClientID, refund.ExternalTxID, refund.Amount, refund.Currency, domain.TransactionRefund, pointsDelta, balance.Points, refund.RequestFingerprint, original.ID, newRefundedAmount, fullyRefunded).
		Scan(&refund.ID, &refund.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, nil, nil, fmt.Errorf("post refund %d/%s: %w", refund.StoreID, refund.ExternalTxID, domain.ErrConflict)
		}
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: insert: %w", refund.StoreID, refund.ExternalTxID, err)
	}
	refund.Type = domain.TransactionRefund
	refund.BalanceAfter = balance.Points
	refund.OriginalTransactionID = &original.ID
	refund.RefundCumulativeAmount = newRefundedAmount
	refund.RefundFullyRefunded = fullyRefunded

	var notify *domain.NotificationOutboxEntry
	if buildNotify != nil {
		notify = buildNotify(pointsDelta)
	}
	if err := insertOutboxEntry(ctx, dbTx, notify); err != nil {
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: %w", refund.StoreID, refund.ExternalTxID, err)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("post refund %d/%s: commit: %w", refund.StoreID, refund.ExternalTxID, err)
	}

	return refund, original, balance, nil
}

// PostRedeemChecked implements domain.LedgerRepository.PostRedeemChecked —
// Post's redemption path with an atomic daily-limit check added. Locking
// the balance row for the duration of the transaction (rather than relying
// solely on the guarded UPDATE, as Post does) is what makes the window-sum
// check and the write atomic together: without the explicit lock, two
// concurrent redemptions could both read a window sum below the limit
// before either commits.
func (r *LedgerRepository) PostRedeemChecked(ctx context.Context, tx *domain.Transaction, minBalance, dailyLimit int64, window time.Duration, notify *domain.NotificationOutboxEntry) (*domain.Transaction, *domain.Balance, error) {
	dbTx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("post redeem %d/%s: begin tx: %w", tx.StoreID, tx.ExternalTxID, err)
	}
	defer dbTx.Rollback(ctx)

	const ensureBalanceRow = `INSERT INTO balances (client_id, points) VALUES ($1, 0) ON CONFLICT (client_id) DO NOTHING`
	if _, err := dbTx.Exec(ctx, ensureBalanceRow, tx.ClientID); err != nil {
		return nil, nil, fmt.Errorf("post redeem %d/%s: ensure balance row: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	const lockBalance = `SELECT points FROM balances WHERE client_id = $1 FOR UPDATE`
	var currentPoints int64
	if err := dbTx.QueryRow(ctx, lockBalance, tx.ClientID).Scan(&currentPoints); err != nil {
		return nil, nil, fmt.Errorf("post redeem %d/%s: lock balance: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	if currentPoints < minBalance {
		return nil, nil, domain.ErrInsufficientBalance
	}

	if dailyLimit > 0 {
		const windowRedeemed = `
			SELECT COALESCE(SUM(-points_delta), 0)
			FROM transactions
			WHERE client_id = $1 AND type = 'redeem' AND created_at > now() - $2::interval`
		var redeemedInWindow int64
		if err := dbTx.QueryRow(ctx, windowRedeemed, tx.ClientID, window.String()).Scan(&redeemedInWindow); err != nil {
			return nil, nil, fmt.Errorf("post redeem %d/%s: sum window redemptions: %w", tx.StoreID, tx.ExternalTxID, err)
		}
		if redeemedInWindow-tx.PointsDelta > dailyLimit {
			return nil, nil, domain.ErrDailyRedeemLimitExceeded
		}
	}

	const adjustBalance = `UPDATE balances SET points = points + $2 WHERE client_id = $1 AND points + $2 >= 0 RETURNING points`
	balance := &domain.Balance{ClientID: tx.ClientID}
	err = dbTx.QueryRow(ctx, adjustBalance, tx.ClientID, tx.PointsDelta).Scan(&balance.Points)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, domain.ErrInsufficientBalance
	}
	if err != nil {
		return nil, nil, fmt.Errorf("post redeem %d/%s: adjust balance: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	if tx.Currency == "" {
		tx.Currency = domain.DefaultCurrency
	}
	const insert = `
		INSERT INTO transactions (store_id, client_id, external_tx_id, amount, currency, type, points_delta, balance_after, request_fingerprint, created_at, rule_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), $10)
		RETURNING id, created_at`
	err = dbTx.QueryRow(ctx, insert, tx.StoreID, tx.ClientID, tx.ExternalTxID, tx.Amount, tx.Currency, tx.Type, tx.PointsDelta, balance.Points, tx.RequestFingerprint, tx.RuleVersion).
		Scan(&tx.ID, &tx.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, nil, fmt.Errorf("post redeem %d/%s: %w", tx.StoreID, tx.ExternalTxID, domain.ErrConflict)
		}
		return nil, nil, fmt.Errorf("post redeem %d/%s: insert: %w", tx.StoreID, tx.ExternalTxID, err)
	}
	tx.BalanceAfter = balance.Points

	if err := insertOutboxEntry(ctx, dbTx, notify); err != nil {
		return nil, nil, fmt.Errorf("post redeem %d/%s: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("post redeem %d/%s: commit: %w", tx.StoreID, tx.ExternalTxID, err)
	}

	return tx, balance, nil
}
