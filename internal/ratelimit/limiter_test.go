package ratelimit

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestLimiter(t *testing.T) (*Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb), mr
}

func TestLimiter_AllowsUpToLimit(t *testing.T) {
	l, _ := newTestLimiter(t)
	for i := 1; i <= 3; i++ {
		allowed, _, err := l.Allow(context.Background(), "k", 3, time.Minute)
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !allowed {
			t.Fatalf("request %d: allowed = false, want true (limit=3)", i)
		}
	}
}

func TestLimiter_RejectsAfterLimit(t *testing.T) {
	l, _ := newTestLimiter(t)
	for i := 1; i <= 3; i++ {
		if _, _, err := l.Allow(context.Background(), "k", 3, time.Minute); err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
	}
	allowed, retryAfter, err := l.Allow(context.Background(), "k", 3, time.Minute)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if allowed {
		t.Fatal("4th request allowed = true, want false (limit=3)")
	}
	if retryAfter <= 0 || retryAfter > time.Minute {
		t.Errorf("retryAfter = %v, want (0, 1m]", retryAfter)
	}
}

func TestLimiter_WindowExpiryAllowsAgain(t *testing.T) {
	l, mr := newTestLimiter(t)
	window := 200 * time.Millisecond
	for i := 1; i <= 2; i++ {
		if _, _, err := l.Allow(context.Background(), "k", 2, window); err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
	}
	if allowed, _, _ := l.Allow(context.Background(), "k", 2, window); allowed {
		t.Fatal("3rd request within window: allowed = true, want false")
	}

	mr.FastForward(window + 10*time.Millisecond)

	allowed, _, err := l.Allow(context.Background(), "k", 2, window)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if !allowed {
		t.Fatal("request after window expiry: allowed = false, want true")
	}
}

func TestLimiter_KeyIsolation(t *testing.T) {
	l, _ := newTestLimiter(t)
	for i := 1; i <= 2; i++ {
		if _, _, err := l.Allow(context.Background(), "principal-a", 2, time.Minute); err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
	}
	if allowed, _, _ := l.Allow(context.Background(), "principal-a", 2, time.Minute); allowed {
		t.Fatal("principal-a's 3rd request: allowed = true, want false")
	}

	// A different key must have its own, untouched quota.
	allowed, _, err := l.Allow(context.Background(), "principal-b", 2, time.Minute)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if !allowed {
		t.Fatal("principal-b's 1st request: allowed = false, want true (isolated from principal-a)")
	}
}

func TestLimiter_ConcurrentRequestsNeverExceedLimit(t *testing.T) {
	l, _ := newTestLimiter(t)
	const limit = 10
	const concurrency = 50

	var allowedCount int64
	done := make(chan struct{})
	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			allowed, _, err := l.Allow(context.Background(), "concurrent-key", limit, time.Minute)
			if err != nil {
				t.Errorf("Allow() error = %v", err)
				return
			}
			if allowed {
				atomic.AddInt64(&allowedCount, 1)
			}
		}()
	}
	for i := 0; i < concurrency; i++ {
		<-done
	}

	if allowedCount != limit {
		t.Errorf("allowedCount = %d, want exactly %d (limit) out of %d concurrent requests", allowedCount, limit, concurrency)
	}
}

func TestLimiter_BackendErrorPropagates(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	defer rdb.Close()
	l := New(rdb)

	mr.Close() // simulate the rate limiter's Redis backend becoming unavailable

	_, _, err := l.Allow(context.Background(), "k", 5, time.Minute)
	if err == nil {
		t.Fatal("Allow() error = nil, want non-nil once the backend is unreachable (must fail closed)")
	}
}
