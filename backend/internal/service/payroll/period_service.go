// Package payroll — F8.5 PeriodService: the month-end reconciliation gate (PC-1,
// PC-7, PC-8, PC-10, PC-12). GetOrCreatePeriod (auto-OPEN), Completeness (paged
// per-employee view), Lock (FIELD-blocker precondition + super-admin force +
// immutable summary snapshot, all in-tx + audited), Reopen (super-admin, void
// summaries), and the summary getter.
//
// Mirrors internal/service/scheduling (TxRunner.InTx for multi-write+audit, FOR
// UPDATE re-check under the tx, apperr codes with explicit HTTP status, Clock for
// tests). The F8.3 payroll-run compute is NOT built — PC-10's run precondition is
// left as the documented hasPostedRun seam.
package payroll

import (
	"context"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// Period audit actions (PC-14).
const (
	ActionPeriodLocked      audit.Action = "PERIOD_LOCKED"
	ActionPeriodForceLocked audit.Action = "PERIOD_FORCE_LOCKED"
	ActionPeriodReopened    audit.Action = "PERIOD_REOPENED"
)

// PeriodService implements the F8.5 period business logic.
type PeriodService struct {
	repo PeriodRepository
	txm  TxRunner
}

// NewPeriodService wires the period service.
func NewPeriodService(repo PeriodRepository, txm TxRunner) *PeriodService {
	return &PeriodService{repo: repo, txm: txm}
}

// --- get/create (PC-1) ---

// PeriodView bundles the period + its FIELD per-tab blocker tally for the cockpit
// header (§9 GET period).
type PeriodView struct {
	Period   dom.PayrollPeriod
	Blockers dom.PeriodBlockerCounts
}

// GetOrCreatePeriod returns the month's period (auto-creating OPEN, PC-1) together
// with its current FIELD blocker counts.
func (s *PeriodService) GetOrCreatePeriod(ctx context.Context, year, month int) (PeriodView, error) {
	if err := validYearMonth(year, month); err != nil {
		return PeriodView{}, err
	}
	period, err := s.repo.GetOrCreatePeriod(ctx, year, month)
	if err != nil {
		return PeriodView{}, apperr.Internal(err)
	}
	blockers, err := s.repo.CountFieldBlockers(ctx, year, month)
	if err != nil {
		return PeriodView{}, apperr.Internal(err)
	}
	return PeriodView{Period: period, Blockers: blockers}, nil
}

// --- completeness (PC-2..PC-5) ---

type completenessCursor struct {
	ID string `json:"i"`
}

// Completeness returns one cursor-paged page of per-employee completeness rows.
// The service computes the derived coverage_gap (PC-2) per row from the raw counts.
func (s *PeriodService) Completeness(ctx context.Context, year, month int, cursor string, limit int) ([]dom.EmployeeCompleteness, *string, error) {
	if err := validYearMonth(year, month); err != nil {
		return nil, nil, err
	}
	var cur completenessCursor
	if err := httpx.DecodeCursor(cursor, &cur); err != nil {
		return nil, nil, err
	}
	var cursorID *string
	if cur.ID != "" {
		cursorID = &cur.ID
	}

	limit = httpx.ClampLimit(limit)
	rows, err := s.repo.ListCompleteness(ctx, year, month, cursorID, limit+1)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	// Derive coverage_gap per row (PC-2): expected - (clean + on_leave), floored at 0.
	for i := range rows {
		gap := rows[i].Expected - (rows[i].Clean + rows[i].OnLeave)
		if gap < 0 {
			gap = 0
		}
		rows[i].CoverageGap = gap
	}

	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		enc, eerr := httpx.EncodeCursor(completenessCursor{ID: last.EmployeeID})
		if eerr != nil {
			return nil, nil, apperr.Internal(eerr)
		}
		next = &enc
	}
	return rows, next, nil
}

// --- lock (PC-7, PC-8) ---

// LockResult is the lock outcome surfaced to the handler.
type LockResult struct {
	Period         dom.PayrollPeriod
	SummariesCount int64
}

// Lock transitions the month OPEN/REOPENED -> LOCKED (PC-7) and snapshots the
// immutable per-FIELD-employee summary (PC-8), all in one tx + audited (PC-14).
//
//   - Computes the FIELD blocker tally; rejects 422 PERIOD_HAS_BLOCKERS when > 0 and
//     not force-locked (PC-7).
//   - force=true requires the actor be super_admin (PC-7) AND a non-empty reason; it
//     overrides a non-zero count and stores the reason (audited PERIOD_FORCE_LOCKED).
//   - On success snapshots PeriodEmployeeSummary for every FIELD employee with month
//     activity, in the SAME tx off the same aggregation (PC-8).
func (s *PeriodService) Lock(ctx context.Context, year, month int, force bool, reason string) (LockResult, error) {
	if err := validYearMonth(year, month); err != nil {
		return LockResult{}, err
	}

	p, _ := auth.PrincipalFrom(ctx)
	if force {
		if p.Role != auth.RoleSuperAdmin {
			return LockResult{}, apperr.Forbidden()
		}
		if reason == "" {
			return LockResult{}, apperr.Rule("FORCE_LOCK_REASON_REQUIRED", map[string]string{
				"reason": "Alasan wajib diisi untuk penguncian paksa.",
			})
		}
	}

	var out LockResult
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cur, found, ferr := s.repo.GetPeriodForUpdate(ctx, tx, year, month)
		if ferr != nil {
			return ferr
		}
		if !found {
			// Auto-create then re-lock would race the FOR UPDATE; instead a period must
			// have been touched (GET) first. Treat a never-seen month as a clean OPEN.
			created, cerr := s.repo.GetOrCreatePeriod(ctx, year, month)
			if cerr != nil {
				return cerr
			}
			cur = created
		}
		if !cur.CanLock() {
			// Already LOCKED.
			return apperr.Conflict("PERIOD_ALREADY_LOCKED")
		}

		// PC-7 precondition: FIELD blockers across all three tabs must be 0 unless forced.
		blockers, berr := s.repo.CountFieldBlockers(ctx, year, month)
		if berr != nil {
			return berr
		}
		if blockers.Total() > 0 && !force {
			return apperr.Rule("PERIOD_HAS_BLOCKERS", map[string]string{
				"attendance": countStr(blockers.Attendance),
				"overtime":   countStr(blockers.Overtime),
				"leave":      countStr(blockers.Leave),
			})
		}

		var reasonPtr *string
		if force {
			reasonPtr = &reason
		}
		locked, ok, lerr := s.repo.Lock(ctx, tx, LockPeriodParams{
			ID:              cur.ID,
			LockedBy:        actorUserID(ctx),
			ForceLocked:     force,
			ForceLockReason: reasonPtr,
		})
		if lerr != nil {
			return lerr
		}
		if !ok {
			// Guard no-op: a concurrent lock won the race.
			return apperr.Conflict("PERIOD_ALREADY_LOCKED")
		}

		// PC-8: snapshot the immutable per-FIELD-employee summary in the same tx.
		n, serr := s.repo.SnapshotSummaries(ctx, tx, SnapshotParams{
			PeriodID: locked.ID, Year: year, Month: month,
		})
		if serr != nil {
			return serr
		}
		out = LockResult{Period: locked, SummariesCount: n}

		action := ActionPeriodLocked
		if force {
			action = ActionPeriodForceLocked
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     action,
			EntityType: "payroll_period",
			EntityID:   locked.ID,
			Before:     map[string]any{"status": string(cur.Status)},
			After: map[string]any{
				"status":          string(locked.Status),
				"force_locked":    force,
				"reason":          reason,
				"summaries":       n,
				"blocker_total":   blockers.Total(),
				"blockers_forced": force && blockers.Total() > 0,
			},
		})
	}); err != nil {
		return LockResult{}, asAppErr(err)
	}
	return out, nil
}

// --- reopen (PC-12) ---

// Reopen transitions the month LOCKED -> REOPENED (PC-12): super-admin only, reason
// required, audited. Rejected 409 PERIOD_RUN_POSTED when a payroll run for the month
// is Posted (the run does not exist yet — hasPostedRun always returns false today).
// Voids the immutable summary set.
func (s *PeriodService) Reopen(ctx context.Context, year, month int, reason string) (dom.PayrollPeriod, error) {
	if err := validYearMonth(year, month); err != nil {
		return dom.PayrollPeriod{}, err
	}
	p, _ := auth.PrincipalFrom(ctx)
	if p.Role != auth.RoleSuperAdmin {
		return dom.PayrollPeriod{}, apperr.Forbidden()
	}
	if reason == "" {
		return dom.PayrollPeriod{}, apperr.Rule("REOPEN_REASON_REQUIRED", map[string]string{
			"reason": "Alasan wajib diisi untuk membuka kembali periode.",
		})
	}

	var out dom.PayrollPeriod
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cur, found, ferr := s.repo.GetPeriodForUpdate(ctx, tx, year, month)
		if ferr != nil {
			return ferr
		}
		if !found {
			return apperr.NotFound()
		}
		if !cur.CanReopen() {
			return apperr.Conflict("PERIOD_NOT_LOCKED")
		}

		// PC-12: a Posted run freezes history (adjustments only). The F8.3 run is not
		// built, so this guard always passes for now (documented seam).
		if hasPostedRun(ctx, year, month) {
			return apperr.Conflict("PERIOD_RUN_POSTED")
		}

		reopened, ok, rerr := s.repo.Reopen(ctx, tx, cur.ID, actorUserID(ctx))
		if rerr != nil {
			return rerr
		}
		if !ok {
			return apperr.Conflict("PERIOD_NOT_LOCKED")
		}

		// PC-12: voiding the immutable summary returns the month to a mutable state.
		voided, verr := s.repo.VoidSummaries(ctx, tx, reopened.ID)
		if verr != nil {
			return verr
		}
		out = reopened

		return audit.Record(ctx, tx, audit.Entry{
			Action:     ActionPeriodReopened,
			EntityType: "payroll_period",
			EntityID:   reopened.ID,
			Before:     map[string]any{"status": string(cur.Status)},
			After: map[string]any{
				"status":           string(reopened.Status),
				"reason":           reason,
				"summaries_voided": voided,
			},
		})
	}); err != nil {
		return dom.PayrollPeriod{}, asAppErr(err)
	}
	return out, nil
}

// hasPostedRun is the PC-10/PC-12 seam: a payroll run (F8.3) for the month is
// Posted. The run compute is NOT built yet, so this always returns false — reopen
// is therefore always permitted today. TODO(F8.3): query payroll_runs for a Posted
// run on (year, month) and return true when one exists.
func hasPostedRun(_ context.Context, _ int, _ int) bool {
	return false
}

// --- summary getter (PC-8 / §9 GET summary) ---

type summaryCursor struct {
	ID string `json:"i"`
}

// Summaries returns one cursor-paged page of the immutable per-employee summary set
// for a month (post-lock). Returns NOT_FOUND when the month was never created.
func (s *PeriodService) Summaries(ctx context.Context, year, month int, cursor string, limit int) ([]dom.PeriodEmployeeSummary, *string, error) {
	if err := validYearMonth(year, month); err != nil {
		return nil, nil, err
	}
	period, found, err := s.repo.GetPeriod(ctx, year, month)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}
	if !found {
		return nil, nil, apperr.NotFound()
	}

	var cur summaryCursor
	if derr := httpx.DecodeCursor(cursor, &cur); derr != nil {
		return nil, nil, derr
	}
	var cursorID *string
	if cur.ID != "" {
		cursorID = &cur.ID
	}

	limit = httpx.ClampLimit(limit)
	rows, err := s.repo.ListSummaries(ctx, period.ID, cursorID, limit+1)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		enc, eerr := httpx.EncodeCursor(summaryCursor{ID: last.EmployeeID})
		if eerr != nil {
			return nil, nil, apperr.Internal(eerr)
		}
		next = &enc
	}
	return rows, next, nil
}

// --- helpers ---

// validYearMonth bounds-checks the path params.
func validYearMonth(year, month int) error {
	if year < 2000 || year > 2100 || month < 1 || month > 12 {
		return apperr.Invalid(map[string]string{"month": "Tahun/bulan tidak valid."})
	}
	return nil
}

func countStr(n int) string {
	// Small, alloc-free int->string for the field message.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
