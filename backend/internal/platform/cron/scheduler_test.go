package cron

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestRunnerFiresAndStops verifies the runner fires the job at least once on its
// interval and returns promptly once its context is cancelled.
func TestRunnerFiresAndStops(t *testing.T) {
	var runs int32
	fired := make(chan struct{}, 1)
	job := func(ctx context.Context) error {
		atomic.AddInt32(&runs, 1)
		select {
		case fired <- struct{}{}:
		default:
		}
		return nil
	}

	r := NewRunner("test", 5*time.Millisecond, job)
	r.FirstRunDelay = time.Millisecond // fire almost immediately

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Start(ctx)
		close(done)
	}()

	select {
	case <-fired:
		// good — the job ran at least once
	case <-time.After(time.Second):
		t.Fatal("job never fired")
	}

	cancel()
	select {
	case <-done:
		// good — Start returned on ctx cancel
	case <-time.After(time.Second):
		t.Fatal("runner did not stop on context cancel")
	}

	if atomic.LoadInt32(&runs) < 1 {
		t.Fatalf("runs = %d, want >= 1", runs)
	}
}

// TestRunnerSkipsOverlap verifies a tick that lands while the previous run is still
// in flight is skipped (no overlapping execution).
func TestRunnerSkipsOverlap(t *testing.T) {
	var concurrent int32
	var maxConcurrent int32
	release := make(chan struct{})
	entered := make(chan struct{}, 1)

	job := func(ctx context.Context) error {
		n := atomic.AddInt32(&concurrent, 1)
		for {
			m := atomic.LoadInt32(&maxConcurrent)
			if n <= m || atomic.CompareAndSwapInt32(&maxConcurrent, m, n) {
				break
			}
		}
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release // hold the run so subsequent ticks must skip
		atomic.AddInt32(&concurrent, -1)
		return nil
	}

	r := NewRunner("overlap", time.Millisecond, job)
	r.FirstRunDelay = time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.Start(ctx)

	<-entered                         // first run is now blocked inside the job
	time.Sleep(20 * time.Millisecond) // many ticks fire while the first run is held
	close(release)                    // let the held run finish

	cancel()
	if got := atomic.LoadInt32(&maxConcurrent); got > 1 {
		t.Fatalf("maxConcurrent = %d, want 1 (overlap must be skipped)", got)
	}
}
