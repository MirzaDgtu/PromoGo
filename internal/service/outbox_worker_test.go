package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// fakeOutboxRepo is an in-memory domain.NotificationOutboxRepository.
type fakeOutboxRepo struct {
	mu      sync.Mutex
	entries map[int64]*domain.NotificationOutboxEntry
	nextID  int64
}

func newFakeOutboxRepo() *fakeOutboxRepo {
	return &fakeOutboxRepo{entries: map[int64]*domain.NotificationOutboxEntry{}}
}

func (f *fakeOutboxRepo) add(e *domain.NotificationOutboxEntry) *domain.NotificationOutboxEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	e.ID = f.nextID
	e.Status = domain.NotificationOutboxPending
	cp := *e
	f.entries[e.ID] = &cp
	return &cp
}

func (f *fakeOutboxRepo) ClaimBatch(_ context.Context, limit int) ([]*domain.NotificationOutboxEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.NotificationOutboxEntry
	for _, e := range f.entries {
		if len(out) >= limit {
			break
		}
		if e.Status == domain.NotificationOutboxPending && !e.NextAttemptAt.After(time.Now()) {
			e.Status = "processing"
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeOutboxRepo) MarkDelivered(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.NotificationOutboxDelivered
	return nil
}

func (f *fakeOutboxRepo) MarkFailed(_ context.Context, id int64, lastErr string, nextAttemptAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.NotificationOutboxPending
	e.Attempts++
	e.LastError = lastErr
	e.NextAttemptAt = nextAttemptAt
	return nil
}

func (f *fakeOutboxRepo) MarkDeadLetter(_ context.Context, id int64, lastErr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.NotificationOutboxDeadLetter
	e.Attempts++
	e.LastError = lastErr
	return nil
}

func (f *fakeOutboxRepo) get(id int64) *domain.NotificationOutboxEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := f.entries[id]
	if e == nil {
		return nil
	}
	cp := *e
	return &cp
}

// fakeChannel is an in-memory domain.NotificationChannel. failClientIDs, if
// set, makes Send fail for those client IDs (every call, until removed) —
// lets a test simulate a persistently-failing delivery.
type fakeChannel struct {
	mu            sync.Mutex
	sent          []int64
	failClientIDs map[int64]bool
}

func (f *fakeChannel) Name() string { return "fake" }

func (f *fakeChannel) Send(_ context.Context, clientID int64, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, clientID)
	if f.failClientIDs[clientID] {
		return errors.New("simulated delivery failure")
	}
	return nil
}

func testOutboxLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestOutboxWorker_SuccessfulDeliveryMarksDelivered(t *testing.T) {
	outbox := newFakeOutboxRepo()
	channel := &fakeChannel{}
	entry := outbox.add(&domain.NotificationOutboxEntry{ClientID: 1, Message: "hello", DedupeKey: "k1"})

	w := NewOutboxWorker(testOutboxLogger(), outbox, channel, OutboxWorkerConfig{
		BatchSize: 10, MaxConcurrency: 4, MaxAttempts: 3, BaseBackoff: time.Second, MaxBackoff: time.Minute,
	})
	w.pollOnce(context.Background())

	got := outbox.get(entry.ID)
	if got.Status != domain.NotificationOutboxDelivered {
		t.Fatalf("status = %q, want delivered", got.Status)
	}
	if len(channel.sent) != 1 || channel.sent[0] != 1 {
		t.Fatalf("sent = %v, want [1]", channel.sent)
	}
}

func TestOutboxWorker_FailureSchedulesBackoffRetry(t *testing.T) {
	outbox := newFakeOutboxRepo()
	channel := &fakeChannel{failClientIDs: map[int64]bool{1: true}}
	entry := outbox.add(&domain.NotificationOutboxEntry{ClientID: 1, Message: "hello", DedupeKey: "k1"})

	w := NewOutboxWorker(testOutboxLogger(), outbox, channel, OutboxWorkerConfig{
		BatchSize: 10, MaxConcurrency: 4, MaxAttempts: 5, BaseBackoff: time.Second, MaxBackoff: time.Minute,
	})
	before := time.Now()
	w.pollOnce(context.Background())

	got := outbox.get(entry.ID)
	if got.Status != domain.NotificationOutboxPending {
		t.Fatalf("status = %q, want pending (scheduled for retry)", got.Status)
	}
	if got.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1", got.Attempts)
	}
	if !got.NextAttemptAt.After(before) {
		t.Fatalf("NextAttemptAt = %v, want after %v (backoff applied)", got.NextAttemptAt, before)
	}
	if got.LastError == "" {
		t.Fatalf("LastError is empty, want the delivery error recorded")
	}
}

func TestOutboxWorker_ExhaustedAttemptsDeadLetters(t *testing.T) {
	outbox := newFakeOutboxRepo()
	channel := &fakeChannel{failClientIDs: map[int64]bool{1: true}}
	entry := outbox.add(&domain.NotificationOutboxEntry{ClientID: 1, Message: "hello", DedupeKey: "k1", Attempts: 2})

	w := NewOutboxWorker(testOutboxLogger(), outbox, channel, OutboxWorkerConfig{
		BatchSize: 10, MaxConcurrency: 4, MaxAttempts: 3, BaseBackoff: time.Second, MaxBackoff: time.Minute,
	})
	w.pollOnce(context.Background())

	got := outbox.get(entry.ID)
	if got.Status != domain.NotificationOutboxDeadLetter {
		t.Fatalf("status = %q, want dead_letter (Attempts=2 + this failed attempt = 3 = MaxAttempts)", got.Status)
	}
}

func TestOutboxWorker_BackoffDoublesAndCapsAtMaxBackoff(t *testing.T) {
	w := &OutboxWorker{cfg: OutboxWorkerConfig{BaseBackoff: time.Second, MaxBackoff: 10 * time.Second}}
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 10 * time.Second}, // would be 16s uncapped
		{10, 10 * time.Second},
	}
	for _, c := range cases {
		if got := w.backoffFor(c.attempts); got != c.want {
			t.Errorf("backoffFor(%d) = %v, want %v", c.attempts, got, c.want)
		}
	}
}

func TestOutboxWorker_ConcurrentDeliveryBoundedByMaxConcurrency(t *testing.T) {
	outbox := newFakeOutboxRepo()
	var inFlight, maxInFlight int32
	var mu sync.Mutex
	channel := &blockingChannel{
		onSend: func() {
			mu.Lock()
			inFlight++
			if inFlight > maxInFlight {
				maxInFlight = inFlight
			}
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			inFlight--
			mu.Unlock()
		},
	}
	for i := 0; i < 20; i++ {
		outbox.add(&domain.NotificationOutboxEntry{ClientID: int64(i), Message: "hi", DedupeKey: string(rune('a' + i))})
	}

	const maxConcurrency = 3
	w := NewOutboxWorker(testOutboxLogger(), outbox, channel, OutboxWorkerConfig{
		BatchSize: 20, MaxConcurrency: maxConcurrency, MaxAttempts: 3, BaseBackoff: time.Second, MaxBackoff: time.Minute,
	})
	w.pollOnce(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if maxInFlight > maxConcurrency {
		t.Fatalf("max concurrent Send calls = %d, want <= %d", maxInFlight, maxConcurrency)
	}
	if maxInFlight < 2 {
		t.Fatalf("max concurrent Send calls = %d, want > 1 (some parallelism actually happened)", maxInFlight)
	}
}

type blockingChannel struct {
	onSend func()
}

func (c *blockingChannel) Name() string { return "blocking" }
func (c *blockingChannel) Send(context.Context, int64, string) error {
	c.onSend()
	return nil
}
