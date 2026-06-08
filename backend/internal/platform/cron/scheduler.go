// Package cron is a tiny in-process job runner for the single-binary phase. A
// Runner fires a job once shortly after start, then on a fixed interval, skipping a
// tick if the previous run is still in flight, and stops cleanly when its context is
// cancelled (the API's graceful-shutdown path). It is deliberately minimal and
// generic — one job (the E5 absence sweep) uses it today; a later phase may graduate
// such jobs to River. Logging is structured (slog) so each run is observable.
package cron

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Job is one unit of scheduled work. A returned error is logged, not fatal — the
// next tick still fires.
type Job func(ctx context.Context) error

// DefaultFirstRunDelay is how long after Start the first run fires when a Runner
// leaves FirstRunDelay unset, so process startup is not blocked by a sweep.
const DefaultFirstRunDelay = 5 * time.Second

// Runner runs one Job on a time.Ticker. It is safe to construct with NewRunner and
// Start in a goroutine; only Start touches the ticker, the in-flight guard is mutex-
// protected so a long run never overlaps the next tick.
type Runner struct {
	name          string
	interval      time.Duration
	job           Job
	FirstRunDelay time.Duration // 0 ⇒ DefaultFirstRunDelay

	mu      sync.Mutex
	running bool
}

// NewRunner builds a Runner. name is used in log lines; interval is the tick period.
func NewRunner(name string, interval time.Duration, job Job) *Runner {
	return &Runner{name: name, interval: interval, job: job}
}

// Start runs the job loop until ctx is cancelled. It blocks, so callers run it in a
// goroutine. The first run fires after FirstRunDelay (or DefaultFirstRunDelay), then
// every interval; a tick that lands while the previous run is still going is skipped.
func (r *Runner) Start(ctx context.Context) {
	firstDelay := r.FirstRunDelay
	if firstDelay <= 0 {
		firstDelay = DefaultFirstRunDelay
	}
	first := time.NewTimer(firstDelay)
	defer first.Stop()
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	slog.Info("cron runner started", "job", r.name, "interval", r.interval.String())
	for {
		select {
		case <-ctx.Done():
			slog.Info("cron runner stopped", "job", r.name)
			return
		case <-first.C:
			r.runOnce(ctx)
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

// runOnce executes the job under the in-flight guard. If a previous run is still
// going, the tick is skipped (logged) rather than queued.
func (r *Runner) runOnce(ctx context.Context) {
	if !r.tryAcquire() {
		slog.Warn("cron run skipped (previous run still in flight)", "job", r.name)
		return
	}
	defer r.release()

	start := time.Now()
	if err := r.job(ctx); err != nil {
		slog.Error("cron run failed", "job", r.name, "err", err, "dur", time.Since(start).String())
		return
	}
	slog.Info("cron run ok", "job", r.name, "dur", time.Since(start).String())
}

func (r *Runner) tryAcquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return false
	}
	r.running = true
	return true
}

func (r *Runner) release() {
	r.mu.Lock()
	r.running = false
	r.mu.Unlock()
}
