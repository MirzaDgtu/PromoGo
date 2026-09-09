package httpserver

import (
	"context"
	"sync"
)

// BackgroundTracker lets a request handler fire best-effort work that must
// outlive the response (e.g. RequireStoreAPIKey's TouchLastUsed) without it
// becoming an untracked goroutine that graceful shutdown can't wait for and
// that infrastructure teardown (closing the Postgres pool, the Redis client)
// can race against.
type BackgroundTracker struct {
	wg sync.WaitGroup
}

// NewBackgroundTracker returns an empty BackgroundTracker.
func NewBackgroundTracker() *BackgroundTracker {
	return &BackgroundTracker{}
}

// Go runs f in a new goroutine, tracked so Wait can block for it.
func (b *BackgroundTracker) Go(f func()) {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		f()
	}()
}

// Wait blocks until every tracked goroutine started with Go has returned, or
// ctx is done, whichever comes first — callers should pass a context with a
// deadline so a stuck background task can't hang shutdown indefinitely.
func (b *BackgroundTracker) Wait(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}
