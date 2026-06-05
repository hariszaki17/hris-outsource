// Package jobs is the async work layer (River, Postgres-backed — no Redis).
// It serves three needs from CONVENTIONS §16 / §7:
//   - fire-and-forget notifications (enqueued in the SAME tx as the write, so
//     they never fire for a rolled-back action and are never lost),
//   - async export jobs (202 + job id),
//   - cron "expiring-soon" detection (PeriodicJobs).
//
// The API process inserts jobs; the worker process (cmd/worker) runs them.
package jobs

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
)

// Client inserts jobs. Used by services via Enqueue / EnqueueTx.
type Client struct {
	river *river.Client[pgx.Tx]
}

// NewInsertOnlyClient builds a client for the API process: it can insert jobs
// but runs no workers itself.
func NewInsertOnlyClient(pool *db.Pool) (*Client, error) {
	rc, err := river.NewClient(riverpgxv5.New(pool.Pool), &river.Config{})
	if err != nil {
		return nil, err
	}
	return &Client{river: rc}, nil
}

// EnqueueTx inserts a job within an existing transaction — the transactional
// outbox guarantee. Call this from a service inside TxManager.InTx.
func (c *Client) EnqueueTx(ctx context.Context, tx pgx.Tx, args river.JobArgs) error {
	_, err := c.river.InsertTx(ctx, tx, args, nil)
	return err
}

// Enqueue inserts a job outside any transaction (use sparingly; prefer EnqueueTx).
func (c *Client) Enqueue(ctx context.Context, args river.JobArgs) error {
	_, err := c.river.Insert(ctx, args, nil)
	return err
}

// Worker process wiring -------------------------------------------------------

// NewWorkerClient builds the client that actually executes jobs (cmd/worker).
// Register all workers in registerWorkers; add cron jobs via PeriodicJobs.
func NewWorkerClient(pool *db.Pool) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	registerWorkers(workers)
	// The PayslipExportWorker is the FIRST worker whose Work() writes to the
	// application DB (it drives export_jobs RUNNING→DONE via UpdateExportJobStatus),
	// so it must be constructed WITH the pool. The pool is only in scope here in
	// NewWorkerClient, so it is registered here rather than in the no-dependency
	// registerWorkers below.
	river.AddWorker(workers, NewPayslipExportWorker(pool))

	return river.NewClient(riverpgxv5.New(pool.Pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 50},
		},
		Workers: workers,
		// PeriodicJobs: cron "expiring-soon" detectors (CONVENTIONS §16.2) go here
		// as the corresponding epics land, e.g. agreement/placement expiry scans.
	})
}

// registerWorkers wires every job type to its worker. One line per job kind.
func registerWorkers(w *river.Workers) {
	river.AddWorker(w, &NotificationWorker{})
}
