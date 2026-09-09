package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// NotificationOutboxRepository is a pgx-backed implementation of
// domain.NotificationOutboxRepository — the worker-side read/write path.
// Enqueueing lives in LedgerRepository instead (see its doc comment); this
// type never inserts a row, only claims and updates ones already there.
type NotificationOutboxRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationOutboxRepository creates a NotificationOutboxRepository
// backed by pool.
func NewNotificationOutboxRepository(pool *pgxpool.Pool) *NotificationOutboxRepository {
	return &NotificationOutboxRepository{pool: pool}
}

// claimLease bounds how long a claimed ('processing') row can stay
// unresolved before it's treated as an abandoned claim (the worker that
// claimed it crashed or hung) and becomes reclaimable again — see the
// migration's comment on the 'processing' status.
const claimLease = 60 * time.Second

// ClaimBatch implements domain.NotificationOutboxRepository.ClaimBatch as
// a single atomic UPDATE ... FROM (SELECT ... FOR UPDATE SKIP LOCKED):
// the SELECT and the status transition to 'processing' happen in one
// statement, so there's no window between "select candidate rows" and
// "mark them claimed" for a second worker replica to select the same rows
// — unlike a bare SELECT FOR UPDATE, whose locks release as soon as that
// single-statement implicit transaction ends, before this method's caller
// ever gets to mark anything. FOR UPDATE SKIP LOCKED is what lets multiple
// replicas run concurrently with no coordination beyond the database
// itself: a row already locked by a concurrent claim is simply skipped.
func (r *NotificationOutboxRepository) ClaimBatch(ctx context.Context, limit int) ([]*domain.NotificationOutboxEntry, error) {
	const query = `
		UPDATE notification_outbox
		SET status = 'processing', next_attempt_at = now() + $2::interval
		WHERE id IN (
			SELECT id FROM notification_outbox
			WHERE (status = 'pending' OR status = 'processing') AND next_attempt_at <= now()
			ORDER BY next_attempt_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, client_id, template_id, template_version, message, dedupe_key, status, attempts, next_attempt_at, COALESCE(last_error, ''), created_at, delivered_at`

	rows, err := r.pool.Query(ctx, query, limit, claimLease.String())
	if err != nil {
		return nil, fmt.Errorf("claim notification outbox batch: %w", err)
	}
	defer rows.Close()

	var out []*domain.NotificationOutboxEntry
	for rows.Next() {
		e := &domain.NotificationOutboxEntry{}
		if err := rows.Scan(
			&e.ID, &e.ClientID, &e.TemplateID, &e.TemplateVersion, &e.Message, &e.DedupeKey,
			&e.Status, &e.Attempts, &e.NextAttemptAt, &e.LastError, &e.CreatedAt, &e.DeliveredAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification outbox entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim notification outbox batch: %w", err)
	}

	return out, nil
}

func (r *NotificationOutboxRepository) MarkDelivered(ctx context.Context, id int64) error {
	const query = `UPDATE notification_outbox SET status = 'delivered', delivered_at = now() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("mark notification outbox entry %d delivered: %w", id, err)
	}
	return nil
}

func (r *NotificationOutboxRepository) MarkFailed(ctx context.Context, id int64, lastErr string, nextAttemptAt time.Time) error {
	const query = `UPDATE notification_outbox SET status = 'pending', attempts = attempts + 1, last_error = $2, next_attempt_at = $3 WHERE id = $1`
	if _, err := r.pool.Exec(ctx, query, id, lastErr, nextAttemptAt); err != nil {
		return fmt.Errorf("mark notification outbox entry %d failed: %w", id, err)
	}
	return nil
}

func (r *NotificationOutboxRepository) MarkDeadLetter(ctx context.Context, id int64, lastErr string) error {
	const query = `UPDATE notification_outbox SET status = 'dead_letter', attempts = attempts + 1, last_error = $2 WHERE id = $1`
	if _, err := r.pool.Exec(ctx, query, id, lastErr); err != nil {
		return fmt.Errorf("mark notification outbox entry %d dead-lettered: %w", id, err)
	}
	return nil
}
