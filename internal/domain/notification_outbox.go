package domain

import (
	"context"
	"time"
)

// NotificationOutboxStatus is the delivery state of a NotificationOutboxEntry.
type NotificationOutboxStatus string

const (
	NotificationOutboxPending    NotificationOutboxStatus = "pending"
	NotificationOutboxDelivered  NotificationOutboxStatus = "delivered"
	NotificationOutboxDeadLetter NotificationOutboxStatus = "dead_letter"
)

// NotificationOutboxEntry describes one notification to deliver, queued by
// LedgerRepository (see its doc comment) in the same database transaction
// as the ledger write it accompanies — the "transactional" half of
// transactional outbox. A worker (internal/service's outboxWorker) claims
// pending, due rows and delivers them via domain.NotificationChannel,
// independent of and never blocking the request that created them.
type NotificationOutboxEntry struct {
	ID int64

	ClientID        int64
	TemplateID      string
	TemplateVersion int
	Message         string
	// DedupeKey is unique per row — see LedgerRepository callers for how
	// it's derived (tied to the ledger write's own idempotency key) — so a
	// caller invoking the same posting logic twice for the same logical
	// write can never enqueue the notification twice.
	DedupeKey string

	Status        NotificationOutboxStatus
	Attempts      int
	NextAttemptAt time.Time
	LastError     string

	CreatedAt   time.Time
	DeliveredAt *time.Time
}

// NotificationOutboxRepository is the worker-side read/write path over
// notification_outbox. Enqueueing a row is NOT part of this interface —
// see LedgerRepository, which writes rows itself inside its own
// transaction; a separate Enqueue call here would reintroduce exactly the
// dual-write risk a transactional outbox exists to avoid.
type NotificationOutboxRepository interface {
	// ClaimBatch atomically selects up to limit pending, due
	// (next_attempt_at <= now) rows — oldest first — and marks them
	// claimed so a concurrent worker (another replica) can't also pick
	// them up, then returns them. Implementations use SELECT ... FOR
	// UPDATE SKIP LOCKED so multiple worker replicas can run without
	// coordinating out-of-band.
	ClaimBatch(ctx context.Context, limit int) ([]*NotificationOutboxEntry, error)
	// MarkDelivered records a successful delivery.
	MarkDelivered(ctx context.Context, id int64) error
	// MarkFailed records a failed delivery attempt: increments Attempts,
	// stores lastErr, and schedules nextAttemptAt for a retry (the caller
	// computes the backoff — see outboxWorker).
	MarkFailed(ctx context.Context, id int64, lastErr string, nextAttemptAt time.Time) error
	// MarkDeadLetter records a delivery that has exhausted its retry
	// budget — status becomes dead_letter and it's never claimed again.
	MarkDeadLetter(ctx context.Context, id int64, lastErr string) error
	// CountPending returns how many rows are currently pending or
	// processing (i.e. not yet delivered or dead-lettered) — feeds the
	// outbox backlog gauge (see internal/metrics).
	CountPending(ctx context.Context) (int64, error)
}
