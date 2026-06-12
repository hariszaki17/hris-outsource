// Package scheduling implements the E4 service layer: shift-master CRUD +
// deactivate/reactivate, schedule-entry CRUD, the shared conflict engine, the
// side-effect-free :check dry-run, and the per-cell-atomic :bulk-apply.
//
// Mirrors the Phase-5 placement slice (consumer-defined repo interface,
// TxRunner.InTx for multi-write+audit, FOR UPDATE re-check + 23505 backstop,
// Clock for tests, apperr codes with explicit HTTP status overrides).
//
// The conflict engine (this file) is the single pure evaluator shared by
// create / update / :check / :bulk-apply. It runs the FIVE contract checks IN
// ORDER and short-circuits on the first failure, returning the exact code +
// HTTP status + ConflictDetails the openapi specifies.
package scheduling

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// --- dependency interfaces (consumer-defined) ---

// TxRunner runs fn inside a database transaction (mirrors placement.TxRunner).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock is the time source (overridable in tests).
type Clock func() time.Time

// PlacementCover is the slim projection of the active placement covering a date
// (INV-2 anchor + scope company).
type PlacementCover struct {
	PlacementID string
	CompanyID   string
	StartDate   time.Time
	EndDate     *time.Time
}

// LiveEntry is the existing live schedule entry for an agent on a date
// (DOUBLE_SHIFT pre-check / replace lookup).
type LiveEntry struct {
	ID            string
	ShiftMasterID *string
	ShiftName     *string
	Status        string
	IsDayOff      bool
}

// ApprovedLeave is one approved-leave day covering a date (SHIFT_OVER_LEAVE).
type ApprovedLeave struct {
	LeaveRequestID *string
	LeaveType      *string
}

// ConflictRepo is the read surface the conflict engine needs. The schedule
// repository implements it on the pool (engine reads are not row-locked; the
// service re-checks DOUBLE_SHIFT under the partial-unique index inside the tx).
type ConflictRepo interface {
	// FindActivePlacementForAgentDate resolves the ACTIVE/EXPIRING placement
	// whose window covers date (INV-2). domain.ErrNotFound when none.
	FindActivePlacementForAgentDate(ctx context.Context, employeeID string, date time.Time) (PlacementCover, error)
	// GetShiftMaster returns the master (for is_active + time snapshot).
	// domain.ErrNotFound when missing/soft-deleted.
	GetShiftMaster(ctx context.Context, id string) (domain.ShiftMaster, error)
	// FindApprovedLeaveForAgentDate returns the approved-leave row covering date
	// (SHIFT_OVER_LEAVE). domain.ErrNotFound when none.
	FindApprovedLeaveForAgentDate(ctx context.Context, employeeID string, date time.Time) (ApprovedLeave, error)
	// FindLiveEntryForAgentDate returns the live entry for an agent on a date
	// (DOUBLE_SHIFT pre-check / replace lookup). domain.ErrNotFound when none.
	FindLiveEntryForAgentDate(ctx context.Context, employeeID string, date time.Time) (LiveEntry, error)
}

// --- engine input / output ---

// ConflictInput is one (agent × date) cell to evaluate.
type ConflictInput struct {
	EmployeeID    string
	ShiftMasterID *string // nil when IsDayOff
	Date          time.Time
	IsDayOff      bool
	ForceReplace  bool
}

// ConflictResult carries the verdict + the resolved write context. Code == ""
// means the cell passed all checks and may be persisted.
type ConflictResult struct {
	Code    string // "" = OK
	Status  int    // HTTP status for Code
	Fields  map[string]string
	Details map[string]any // ConflictDetails subset (openapi)

	// Resolved context for a successful write:
	PlacementID   string
	CompanyID     string
	StartTime     *string
	EndTime       *string
	CrossMidnight bool

	// ExistingEntryID is set when DOUBLE_SHIFT (block) OR when force_replace
	// resolves to a replace (the old entry to supersede).
	ExistingEntryID *string
	ExistingShift   *string // name, for the DOUBLE_SHIFT detail
}

// OK reports whether the cell passed all five checks.
func (r ConflictResult) OK() bool { return r.Code == "" }

// AsError renders a failed ConflictResult into the matching *apperr.Error
// (exact code + HTTP status + details) for the create/update enforce path.
func (r ConflictResult) AsError() error {
	if r.OK() {
		return nil
	}
	return &apperr.Error{
		Code:       r.Code,
		Fields:     r.Fields,
		Details:    r.Details,
		HTTPStatus: r.Status,
	}
}

// Evaluate runs the FIVE conflict checks IN ORDER, short-circuiting on the first
// failure. Order (openapi POST /schedule description):
//
//  1. OUT_OF_SCOPE              403  leader company != agent placement company (INV-3)
//  2. OUTSIDE_PLACEMENT_PERIOD  422  date outside active placement window      (INV-2)
//  3. SHIFT_DEACTIVATED         422  picked master is_active=false             (SM-5)
//  4. SHIFT_OVER_LEAVE          409  approved leave covers date                (EPICS §8)
//  5. DOUBLE_SHIFT              409  live entry exists & !force_replace         (INV-1)
//
// Resolution: the active placement is resolved FIRST (scope needs its company);
// if no active placement exists at all, OUTSIDE_PLACEMENT_PERIOD is emitted.
// When a placement is found, scope is checked against its company BEFORE any
// 422 — matching the openapi "rule 1 = OUT_OF_SCOPE".
func Evaluate(ctx context.Context, repo ConflictRepo, in ConflictInput) (ConflictResult, error) {
	// Resolve the active placement covering the date (INV-2 anchor + scope src).
	cover, err := repo.FindActivePlacementForAgentDate(ctx, in.EmployeeID, in.Date)
	if err != nil {
		if isNotFound(err) {
			// No active placement on this date → OUTSIDE_PLACEMENT_PERIOD.
			return ConflictResult{
				Code:   "OUTSIDE_PLACEMENT_PERIOD",
				Status: 422,
				Fields: map[string]string{"date": "Tanggal di luar periode penempatan aktif agen."},
			}, nil
		}
		return ConflictResult{}, apperr.Internal(err)
	}

	// 1. SCOPE (OUT_OF_SCOPE 403) — leader acting outside their own company.
	if serr := rbac.GuardCompany(ctx, cover.CompanyID); serr != nil {
		details := map[string]any{"agent_company_id": cover.CompanyID}
		if p, ok := principalCompany(ctx); ok && p != "" {
			details["leader_company_id"] = p
		}
		return ConflictResult{
			Code:    "OUT_OF_SCOPE",
			Status:  403,
			Details: details,
		}, nil
	}

	res := ConflictResult{
		PlacementID: cover.PlacementID,
		CompanyID:   cover.CompanyID,
	}

	if !in.IsDayOff {
		// Need the master for deactivation + time snapshot.
		if in.ShiftMasterID == nil {
			return ConflictResult{
				Code:   "INVALID_REQUEST",
				Status: 400,
				Fields: map[string]string{"shift_master_id": "Wajib diisi kecuali is_day_off=true."},
			}, nil
		}
		master, merr := repo.GetShiftMaster(ctx, *in.ShiftMasterID)
		if merr != nil {
			if isNotFound(merr) {
				return ConflictResult{
					Code:   "INVALID_REQUEST",
					Status: 400,
					Fields: map[string]string{"shift_master_id": "Shift tidak ditemukan."},
				}, nil
			}
			return ConflictResult{}, apperr.Internal(merr)
		}

		// 3. SHIFT_DEACTIVATED (422) — picked master inactive.
		if !master.IsActive {
			return ConflictResult{
				Code:   "SHIFT_DEACTIVATED",
				Status: 422,
			}, nil
		}

		// Snapshot the master window onto the result (day-off → nil/false).
		st := master.StartTime
		et := master.EndTime
		res.StartTime = &st
		res.EndTime = &et
		res.CrossMidnight = master.CrossMidnight
	}

	// 4. SHIFT_OVER_LEAVE (409) — approved leave covers the date.
	leave, lerr := repo.FindApprovedLeaveForAgentDate(ctx, in.EmployeeID, in.Date)
	if lerr == nil {
		details := map[string]any{}
		if leave.LeaveRequestID != nil {
			details["leave_request_id"] = *leave.LeaveRequestID
		}
		if leave.LeaveType != nil {
			details["leave_type"] = *leave.LeaveType
		}
		return ConflictResult{
			Code:    "SHIFT_OVER_LEAVE",
			Status:  409,
			Fields:  map[string]string{"date": "Agen sedang cuti yang disetujui pada tanggal ini."},
			Details: details,
		}, nil
	} else if !isNotFound(lerr) {
		return ConflictResult{}, apperr.Internal(lerr)
	}

	// 5. DOUBLE_SHIFT (409) — live entry exists.
	live, eerr := repo.FindLiveEntryForAgentDate(ctx, in.EmployeeID, in.Date)
	if eerr == nil {
		if in.ForceReplace {
			// Replace path: remember the entry to supersede, no conflict.
			id := live.ID
			res.ExistingEntryID = &id
			res.ExistingShift = live.ShiftName
		} else {
			details := map[string]any{"existing_entry_id": live.ID}
			if live.ShiftName != nil {
				details["existing_shift_name"] = *live.ShiftName
			}
			return ConflictResult{
				Code:    "DOUBLE_SHIFT",
				Status:  409,
				Fields:  map[string]string{"date": "Agen sudah memiliki shift di tanggal ini."},
				Details: details,
			}, nil
		}
	} else if !isNotFound(eerr) {
		return ConflictResult{}, apperr.Internal(eerr)
	}

	// All five passed.
	return res, nil
}

func isNotFound(err error) bool {
	return err == domain.ErrNotFound
}

// principalCompany reads the leader's company id from context for the
// OUT_OF_SCOPE detail. Returns ("", false) when there is no principal.
func principalCompany(ctx context.Context) (string, bool) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return "", false
	}
	return p.CompanyID, true
}
