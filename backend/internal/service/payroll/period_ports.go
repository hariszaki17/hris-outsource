// Package payroll — F8.5 Payroll Period Close service ports (PC-1..PC-15). The
// consumer-defined repository interfaces + param structs the PeriodService and
// ClarificationService depend on. Reads run on the pool; guarded transitions +
// the lock snapshot run under a pgx.Tx. Mirrors the attendance / scheduling port
// convention (consumer-defined repo iface, tx-aware writes, pgx.ErrNoRows ->
// domain.ErrNotFound at the repo boundary).
package payroll

import (
	"context"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
)

// --- period repository port ---

// LockPeriodParams is the guarded LOCKED-transition payload (PC-7/PC-8). LockedBy
// is the actor user id; ForceLocked + ForceLockReason are set only on a super-admin
// override.
type LockPeriodParams struct {
	ID              string
	LockedBy        *string
	ForceLocked     bool
	ForceLockReason *string
}

// SnapshotParams drives the PC-8 immutable summary materialization for a month.
type SnapshotParams struct {
	PeriodID string
	Year     int
	Month    int
}

// PeriodRepository is the data dependency for the period service.
type PeriodRepository interface {
	// GetPeriod returns the period for a month (found=false when none yet).
	GetPeriod(ctx context.Context, year, month int) (dom.PayrollPeriod, bool, error)
	// GetOrCreatePeriod auto-creates OPEN on first access (PC-1) and returns the row.
	GetOrCreatePeriod(ctx context.Context, year, month int) (dom.PayrollPeriod, error)
	// GetPeriodForUpdate row-locks the period for a guarded transition (in tx).
	GetPeriodForUpdate(ctx context.Context, tx pgx.Tx, year, month int) (dom.PayrollPeriod, bool, error)

	// Lock performs the guarded OPEN/REOPENED -> LOCKED transition. found=false when
	// the status guard rejects (already locked).
	Lock(ctx context.Context, tx pgx.Tx, p LockPeriodParams) (dom.PayrollPeriod, bool, error)
	// Reopen performs the guarded LOCKED -> REOPENED transition. found=false when not
	// currently LOCKED.
	Reopen(ctx context.Context, tx pgx.Tx, id string, reopenedBy *string) (dom.PayrollPeriod, bool, error)

	// SnapshotSummaries materializes PeriodEmployeeSummary for every FIELD employee
	// with month activity (PC-8). Returns the rows written.
	SnapshotSummaries(ctx context.Context, tx pgx.Tx, p SnapshotParams) (int64, error)
	// VoidSummaries hard-deletes the summary set on reopen (PC-12).
	VoidSummaries(ctx context.Context, tx pgx.Tx, periodID string) (int64, error)

	// ListCompleteness returns the per-employee completeness rows (cursor-paged).
	ListCompleteness(ctx context.Context, year, month int, cursorID *string, limit int) ([]dom.EmployeeCompleteness, error)
	// CountFieldBlockers returns the per-tab FIELD-only blocker tally (PC-7 gate).
	CountFieldBlockers(ctx context.Context, year, month int) (dom.PeriodBlockerCounts, error)
	// CountOpenClarifications returns the number of OPEN clarifications for a
	// period (the "awaiting reply" badge, PC-13 / F8.5).
	CountOpenClarifications(ctx context.Context, periodID string) (int, error)
	// ListSummaries returns the immutable per-employee summary set (cursor-paged).
	ListSummaries(ctx context.Context, periodID string, cursorID *string, limit int) ([]dom.PeriodEmployeeSummary, error)
}

// --- clarification + adjustment repository port ---

// CreateClarificationParams is the insert payload for a raised clarification (PC-13).
type CreateClarificationParams struct {
	SourceType       string
	SourceID         string
	PeriodID         *string
	RaisedBy         *string
	TargetEmployeeID string
	Question         string
}

// CreateAdjustmentParams is the insert payload for a post-lock carry-forward (PC-9).
type CreateAdjustmentParams struct {
	EmployeeID  string
	SourceType  string
	SourceID    string
	OriginYear  int
	OriginMonth int
	Note        *string
}

// ClarificationRepository is the data dependency for the clarification + adjustment
// services.
type ClarificationRepository interface {
	CreateClarification(ctx context.Context, tx pgx.Tx, p CreateClarificationParams) (dom.ClarificationRequest, error)
	GetClarification(ctx context.Context, id string) (dom.ClarificationRequest, error)
	GetClarificationForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.ClarificationRequest, error)
	// Answer performs the guarded OPEN -> ANSWERED transition (found=false when not OPEN).
	Answer(ctx context.Context, tx pgx.Tx, id string, answer string, answeredBy *string) (dom.ClarificationRequest, bool, error)
	// SetStatus closes the loop: RESOLVED or CANCELLED (found=false when terminal).
	SetStatus(ctx context.Context, tx pgx.Tx, id, status string) (dom.ClarificationRequest, bool, error)

	// CreateAdjustment emits a PENDING, unvalued carry-forward ledger row (PC-9).
	CreateAdjustment(ctx context.Context, tx pgx.Tx, p CreateAdjustmentParams) (dom.PayrollAdjustment, error)
}

// --- clarification target-resolution port ---

// ClarificationTargetResolver resolves a clarification's target employee (PC-13 /
// C-PC-6): the record's company shift-leader if there is one, else the agent on the
// record. Consumer-defined so the service stays decoupled from the E3 placement repo
// internals (satisfied from the placement repo + the record's own employee/company).
type ClarificationTargetResolver interface {
	// ResolveTarget returns the employee id to notify for a record. sourceType is
	// ATTENDANCE/OVERTIME/LEAVE; it returns the record's owning employee + company so
	// the service can pick the SL (else the agent).
	ResolveTarget(ctx context.Context, sourceType, sourceID string) (ClarificationTarget, error)
}

// ClarificationTarget is the resolution result: the record's owning agent + company,
// plus the active company shift-leader (empty when none — C-PC-6 falls back to the agent).
type ClarificationTarget struct {
	AgentEmployeeID  string
	CompanyID        string
	LeaderEmployeeID string // "" when the company has no active shift leader
}
