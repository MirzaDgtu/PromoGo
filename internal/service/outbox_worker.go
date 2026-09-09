package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// OutboxWorkerConfig bounds OutboxWorker's polling, batching, concurrency,
// and retry behavior.
type OutboxWorkerConfig struct {
	// PollInterval is how often the worker checks for due entries.
	PollInterval time.Duration
	// BatchSize is the max entries claimed per poll (domain.
	// NotificationOutboxRepository.ClaimBatch's limit).
	BatchSize int
	// MaxConcurrency bounds how many entries from one claimed batch are
	// delivered in parallel — the same "bounded concurrency" requirement
	// as fcmchannel's SendEach switch, applied one level up.
	MaxConcurrency int
	// MaxAttempts is how many delivery attempts (including the first)
	// an entry gets before it's dead-lettered.
	MaxAttempts int
	// BaseBackoff and MaxBackoff bound the exponential backoff applied
	// between retry attempts: attempt N waits min(BaseBackoff*2^(N-1), MaxBackoff).
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	// DeliverTimeout bounds one poll's claim+deliver+mark work, independent
	// of Run's ctx — see Run's doc comment for why a poll already underway
	// must not be aborted by the same cancellation that stops scheduling
	// new ones. Defaults to 30s if zero.
	DeliverTimeout time.Duration
}

// OutboxWorker delivers domain.NotificationOutboxEntry rows queued by
// LedgerRepository (see its doc comment for the transactional-outbox
// guarantee) via a domain.NotificationChannel, independent of and never
// blocking the accrual/redeem/refund request path that queued them —
// Phase 3 of docs/audit-remediation-prompt.md.
//
// Delivery outcomes (delivered / retried / dead-lettered) are logged as
// structured events with enough fields (template_id, attempts, id) to
// build dashboards/alerts from log aggregation — first-class metrics
// (Prometheus counters/histograms) are Phase 4's "expose... metrics"
// item, which needs the broader metrics/tracing infrastructure this
// package doesn't own; these logs are what that infrastructure would
// consume.
type OutboxWorker struct {
	log      *slog.Logger
	outbox   domain.NotificationOutboxRepository
	notifier domain.NotificationChannel
	cfg      OutboxWorkerConfig
}

// NewOutboxWorker constructs an OutboxWorker.
func NewOutboxWorker(log *slog.Logger, outbox domain.NotificationOutboxRepository, notifier domain.NotificationChannel, cfg OutboxWorkerConfig) *OutboxWorker {
	return &OutboxWorker{log: log, outbox: outbox, notifier: notifier, cfg: cfg}
}

// Run polls until ctx is canceled, then returns once any poll already
// underway finishes. Intended to be started in its own goroutine by
// internal/app, which should wait for Run to return (bounded by its own
// timeout) before closing Postgres/Redis out from under a delivery that's
// still writing to them.
//
// A poll already claimed from the outbox runs on a context detached from
// ctx's cancellation (see deliverCtx below): canceling ctx stops the ticker
// from starting a *new* poll, but must not abort in-flight
// Send/MarkDelivered/MarkFailed calls mid-request, which would otherwise
// turn a graceful shutdown into a batch of spurious delivery failures.
// DeliverTimeout still bounds that detached work so a genuinely stuck call
// can't hang shutdown forever.
func (w *OutboxWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deliverCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), w.deliverTimeout())
			w.pollOnce(deliverCtx)
			cancel()
		}
	}
}

func (w *OutboxWorker) deliverTimeout() time.Duration {
	if w.cfg.DeliverTimeout > 0 {
		return w.cfg.DeliverTimeout
	}
	return 30 * time.Second
}

// pollOnce claims one batch and delivers it with bounded concurrency,
// waiting for the whole batch to finish before the next poll tick — so
// MaxConcurrency also bounds how many deliveries are ever in flight at
// once, not just how many per batch.
func (w *OutboxWorker) pollOnce(ctx context.Context) {
	entries, err := w.outbox.ClaimBatch(ctx, w.cfg.BatchSize)
	if err != nil {
		w.log.WarnContext(ctx, "claim notification outbox batch", "error", err)
		return
	}
	if len(entries) == 0 {
		return
	}

	sem := make(chan struct{}, max(w.cfg.MaxConcurrency, 1))
	var wg sync.WaitGroup
	for _, entry := range entries {
		entry := entry
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			w.deliver(ctx, entry)
		}()
	}
	wg.Wait()
}

// deliver attempts one entry and resolves its claim: MarkDelivered on
// success, MarkDeadLetter once MaxAttempts is exhausted, otherwise
// MarkFailed with an exponential backoff before the next retry.
func (w *OutboxWorker) deliver(ctx context.Context, entry *domain.NotificationOutboxEntry) {
	err := w.notifier.Send(ctx, entry.ClientID, entry.Message)
	if err == nil {
		if markErr := w.outbox.MarkDelivered(ctx, entry.ID); markErr != nil {
			w.log.WarnContext(ctx, "mark notification outbox entry delivered", "id", entry.ID, "error", markErr)
		}
		w.log.InfoContext(ctx, "notification delivered", "id", entry.ID, "template_id", entry.TemplateID, "attempts", entry.Attempts+1)
		return
	}

	attempts := entry.Attempts + 1
	if attempts >= w.cfg.MaxAttempts {
		if markErr := w.outbox.MarkDeadLetter(ctx, entry.ID, err.Error()); markErr != nil {
			w.log.WarnContext(ctx, "mark notification outbox entry dead-lettered", "id", entry.ID, "error", markErr)
		}
		w.log.ErrorContext(ctx, "notification dead-lettered", "id", entry.ID, "template_id", entry.TemplateID, "attempts", attempts, "error", err)
		return
	}

	backoff := w.backoffFor(attempts)
	if markErr := w.outbox.MarkFailed(ctx, entry.ID, err.Error(), time.Now().Add(backoff)); markErr != nil {
		w.log.WarnContext(ctx, "mark notification outbox entry failed", "id", entry.ID, "error", markErr)
	}
	w.log.WarnContext(ctx, "notification delivery failed, will retry", "id", entry.ID, "template_id", entry.TemplateID, "attempts", attempts, "retry_in", backoff, "error", err)
}

// backoffFor returns the delay before attempt number attempts+1: doubling
// from BaseBackoff, capped at MaxBackoff.
func (w *OutboxWorker) backoffFor(attempts int) time.Duration {
	backoff := w.cfg.BaseBackoff << (attempts - 1)
	if backoff > w.cfg.MaxBackoff || backoff <= 0 {
		return w.cfg.MaxBackoff
	}
	return backoff
}
