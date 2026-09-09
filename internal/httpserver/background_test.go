package httpserver

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestBackgroundTracker_WaitBlocksUntilTrackedGoroutinesFinish(t *testing.T) {
	bg := NewBackgroundTracker()
	var done atomic.Bool
	release := make(chan struct{})

	bg.Go(func() {
		<-release
		done.Store(true)
	})

	waitReturned := make(chan struct{})
	go func() {
		bg.Wait(context.Background())
		close(waitReturned)
	}()

	select {
	case <-waitReturned:
		t.Fatal("Wait returned before the tracked goroutine finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-waitReturned:
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after the tracked goroutine finished")
	}
	if !done.Load() {
		t.Fatal("tracked goroutine did not run to completion")
	}
}

func TestBackgroundTracker_WaitReturnsOnContextDeadlineEvenIfStuck(t *testing.T) {
	bg := NewBackgroundTracker()
	bg.Go(func() {
		<-make(chan struct{}) // never returns
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	bg.Wait(ctx)
	if time.Since(start) > time.Second {
		t.Fatalf("Wait took %v, want it to return promptly once ctx is done", time.Since(start))
	}
}
