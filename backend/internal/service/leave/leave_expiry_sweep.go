// Package leave — leave-expiry sweep (F6.1). A single-binary, in-process cron (wired
// in cmd/api) that periodically releases dangling pending_days on grant-lots whose
// expires_at has passed. Remaining is DERIVED (amount-consumed-pending) and a lot is
// ACTIVE only while now < expires_at, so an expired lot already contributes 0 to the
// computed balance — no zeroing of consumed/amount is needed. What CAN linger is a
// pending reservation that was never committed/released before the lot lapsed (e.g. a
// PENDING request whose lot expired mid-flight); the sweep zeroes those so the lot's
// pending_days don't misreport. Each release is audited in its own tx; one bad row
// aborts the tick and is returned (mirrors the absence-sweep). A later phase may
// graduate this to River.
package leave

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
)

// defaultExpiryBatch bounds one expiry-sweep tick.
const defaultExpiryBatch = 500

// LeaveExpirySweepService releases dangling pending on lapsed grant-lots.
type LeaveExpirySweepService struct {
	repo  GrantRepository
	txm   TxRunner
	now   Clock
	batch int
}

// NewLeaveExpirySweepService wires the sweep. batch bounds one tick (<=0 ⇒ default).
func NewLeaveExpirySweepService(repo GrantRepository, txm TxRunner, batch int) *LeaveExpirySweepService {
	if batch <= 0 {
		batch = defaultExpiryBatch
	}
	return &LeaveExpirySweepService{repo: repo, txm: txm, now: time.Now, batch: batch}
}

// SetClock overrides the time source (tests only).
func (s *LeaveExpirySweepService) SetClock(c Clock) { s.now = c }

// Sweep releases dangling pending on one batch of expired lots and returns the number
// of lots swept. Today is the Asia/Jakarta-neutral date boundary (lots expire at the
// day granularity of expires_at).
func (s *LeaveExpirySweepService) Sweep(ctx context.Context) (int, error) {
	today := s.now().UTC().Truncate(24 * time.Hour)
	lots, err := s.repo.FindExpiredLotsWithPending(ctx, today, s.batch)
	if err != nil {
		return 0, err
	}
	swept := 0
	for _, lot := range lots {
		lot := lot
		txErr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
			if zerr := s.repo.ZeroLotPending(ctx, tx, lot.ID); zerr != nil {
				return zerr
			}
			return audit.Record(ctx, tx, audit.Entry{
				Action:     audit.ActionUpdate,
				EntityType: "leave_grant",
				EntityID:   lot.ID,
				Before:     map[string]any{"pending_days": lot.PendingDays},
				After: map[string]any{
					"pending_days": 0, "reason": "expiry_sweep_released_dangling_pending",
					"expires_at": lot.ExpiresAt.Format("2006-01-02"),
				},
			})
		})
		if txErr != nil {
			slog.Error("leave-expiry-sweep: release failed", "grant_id", lot.ID, "err", txErr)
			return swept, txErr
		}
		swept++
	}
	slog.Info("leave-expiry-sweep complete", "lots", len(lots), "swept", swept, "today", today.Format("2006-01-02"))
	return swept, nil
}
