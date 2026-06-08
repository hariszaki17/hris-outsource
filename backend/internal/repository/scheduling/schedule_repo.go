// Package scheduling (repository) — ScheduleRepo implements the scheduling
// schedule-entry service port (incl. the conflict engine's ConflictRepo read
// surface) over the 06-01 sqlc queries. Reads on the pool; locked re-checks +
// writes via q.WithTx(tx). Date columns convert pgtype.Date ↔ time.Time at this
// boundary (Phase-5 pattern). pgx.ErrNoRows → domain.ErrNotFound.
package scheduling

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	leavesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// ScheduleRepo is the sqlc-backed implementation of svc.ScheduleRepository.
type ScheduleRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ScheduleRepository = (*ScheduleRepo)(nil)

// NewScheduleRepo returns a ScheduleRepo backed by pool.
func NewScheduleRepo(pool *db.Pool) *ScheduleRepo {
	return &ScheduleRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- ConflictRepo (engine read surface) ---

func (r *ScheduleRepo) FindActivePlacementForAgentDate(ctx context.Context, employeeID string, date time.Time) (svc.PlacementCover, error) {
	row, err := r.q.FindActivePlacementForAgentDate(ctx, sqlcgen.FindActivePlacementForAgentDateParams{
		EmployeeID: employeeID,
		WorkDate:   timeToPgDate(date),
	})
	if err != nil {
		return svc.PlacementCover{}, mapErr(err)
	}
	cover := svc.PlacementCover{
		PlacementID:   row.ID,
		CompanyID:     row.ClientCompanyID,
		ServiceLineID: row.ServiceLineID,
		StartDate:     pgDateToTime(row.StartDate),
	}
	if row.EndDate.Valid {
		e := row.EndDate.Time
		cover.EndDate = &e
	}
	return cover, nil
}

func (r *ScheduleRepo) GetShiftMaster(ctx context.Context, id string) (domain.ShiftMaster, error) {
	row, err := r.q.GetShiftMaster(ctx, id)
	if err != nil {
		return domain.ShiftMaster{}, mapErr(err)
	}
	return mapShiftMasterFromGet(row), nil
}

func (r *ScheduleRepo) FindApprovedLeaveForAgentDate(ctx context.Context, employeeID string, date time.Time) (svc.ApprovedLeave, error) {
	row, err := r.q.FindApprovedLeaveForAgentDate(ctx, sqlcgen.FindApprovedLeaveForAgentDateParams{
		EmployeeID: employeeID,
		LeaveDate:  timeToPgDate(date),
	})
	if err != nil {
		return svc.ApprovedLeave{}, mapErr(err)
	}
	return svc.ApprovedLeave{LeaveRequestID: row.LeaveRequestID, LeaveType: row.LeaveType}, nil
}

func (r *ScheduleRepo) FindLiveEntryForAgentDate(ctx context.Context, employeeID string, date time.Time) (svc.LiveEntry, error) {
	return r.findLive(ctx, r.q, employeeID, date)
}

func (r *ScheduleRepo) FindLiveEntryForAgentDateTx(ctx context.Context, tx pgx.Tx, employeeID string, date time.Time) (svc.LiveEntry, error) {
	return r.findLive(ctx, r.q.WithTx(tx), employeeID, date)
}

// findLive resolves the live entry (and the shift name when set) for a cell.
func (r *ScheduleRepo) findLive(ctx context.Context, q *sqlcgen.Queries, employeeID string, date time.Time) (svc.LiveEntry, error) {
	row, err := q.FindLiveEntryForAgentDate(ctx, sqlcgen.FindLiveEntryForAgentDateParams{
		EmployeeID: employeeID,
		WorkDate:   timeToPgDate(date),
	})
	if err != nil {
		return svc.LiveEntry{}, mapErr(err)
	}
	live := svc.LiveEntry{
		ID:            row.ID,
		ShiftMasterID: row.ShiftMasterID,
		Status:        row.Status,
		IsDayOff:      row.IsDayOff,
	}
	// Resolve the shift name for the DOUBLE_SHIFT detail (best-effort).
	if row.ShiftMasterID != nil {
		if sm, serr := q.GetShiftMaster(ctx, *row.ShiftMasterID); serr == nil {
			name := sm.Name
			live.ShiftName = &name
		}
	}
	return live, nil
}

// --- grid read + writes ---

func (r *ScheduleRepo) ListSchedule(ctx context.Context, f domain.ScheduleFilter) ([]domain.ScheduleEntry, error) {
	rows, err := r.q.ListSchedule(ctx, sqlcgen.ListScheduleParams{
		CompanyID:  f.CompanyID,
		StartDate:  timeToPgDate(f.StartDate),
		EndDate:    timeToPgDate(f.EndDate),
		EmployeeID: f.EmployeeID,
		StatusIn:   f.StatusIn,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ScheduleEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapScheduleFromList(row))
	}
	return out, nil
}

// ListScheduleByAgent returns one agent's schedule across ALL their placements
// for the date window (F4.3 "Jadwal Saya"). No company filter — by-agent spans
// companies; scope is enforced upstream in the service.
func (r *ScheduleRepo) ListScheduleByAgent(ctx context.Context, employeeID string, start, end time.Time) ([]domain.ScheduleEntry, error) {
	rows, err := r.q.ListScheduleByAgent(ctx, sqlcgen.ListScheduleByAgentParams{
		EmployeeID: employeeID,
		StartDate:  timeToPgDate(start),
		EndDate:    timeToPgDate(end),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ScheduleEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapScheduleFromByAgent(row))
	}
	return out, nil
}

// GetActivePlacementCompanyForEmployee resolves the company an agent is currently
// placed at (shift-leader scope check for by-agent). domain.ErrNotFound when the
// agent has no active placement.
func (r *ScheduleRepo) GetActivePlacementCompanyForEmployee(ctx context.Context, employeeID string) (string, error) {
	row, err := r.q.GetActivePlacementForEmployee(ctx, employeeID)
	if err != nil {
		return "", mapErr(err)
	}
	return row.ClientCompanyID, nil
}

func (r *ScheduleRepo) GetScheduleEntry(ctx context.Context, id string) (domain.ScheduleEntry, error) {
	row, err := r.q.GetScheduleEntry(ctx, id)
	if err != nil {
		return domain.ScheduleEntry{}, mapErr(err)
	}
	return mapScheduleFromGet(row), nil
}

func (r *ScheduleRepo) GetScheduleEntryForUpdate(ctx context.Context, tx pgx.Tx, id string) (domain.ScheduleEntry, error) {
	row, err := r.q.WithTx(tx).GetScheduleEntryForUpdate(ctx, id)
	if err != nil {
		return domain.ScheduleEntry{}, mapErr(err)
	}
	return mapScheduleFromForUpdate(row), nil
}

func (r *ScheduleRepo) CreateScheduleEntry(ctx context.Context, tx pgx.Tx, p svc.CreateScheduleEntryParams) (domain.ScheduleEntry, error) {
	row, err := r.q.WithTx(tx).CreateScheduleEntry(ctx, sqlcgen.CreateScheduleEntryParams{
		EmployeeID:      p.EmployeeID,
		PlacementID:     p.PlacementID,
		ServiceLineID:   p.ServiceLineID,
		ShiftMasterID:   p.ShiftMasterID,
		StartTime:       p.StartTime,
		EndTime:         p.EndTime,
		CrossMidnight:   p.CrossMidnight,
		WorkDate:        timeToPgDate(p.WorkDate),
		Status:          p.Status,
		IsDayOff:        p.IsDayOff,
		ReplacedEntryID: p.ReplacedEntryID,
		CreatedBy:       p.CreatedBy,
	})
	if err != nil {
		return domain.ScheduleEntry{}, err
	}
	return mapScheduleFromCreate(row), nil
}

func (r *ScheduleRepo) UpdateScheduleEntry(ctx context.Context, tx pgx.Tx, p svc.UpdateScheduleEntryParams) (domain.ScheduleEntry, error) {
	row, err := r.q.WithTx(tx).UpdateScheduleEntry(ctx, sqlcgen.UpdateScheduleEntryParams{
		ShiftMasterID:   p.ShiftMasterID,
		ServiceLineID:   p.ServiceLineID,
		StartTime:       p.StartTime,
		EndTime:         p.EndTime,
		CrossMidnight:   p.CrossMidnight,
		Status:          p.Status,
		IsDayOff:        p.IsDayOff,
		ReplacedEntryID: p.ReplacedEntryID,
		ID:              p.ID,
	})
	if err != nil {
		return domain.ScheduleEntry{}, err
	}
	return mapScheduleFromUpdate(row), nil
}

func (r *ScheduleRepo) SoftDeleteScheduleEntry(ctx context.Context, tx pgx.Tx, id string) (int64, error) {
	return r.q.WithTx(tx).SoftDeleteScheduleEntry(ctx, id)
}

// --- INV-3 loop-closer (E6 / Phase 8) ---
//
// These two methods are the cross-epic write surface the E6 leave service calls
// inside its final/override approval tx. They are NOT part of svc.ScheduleRepository;
// they satisfy leavesvc.SchedulePort (defined in service/leave). The scheduling repo
// importing service/leave does NOT create a cycle (service/leave does not import
// repository/scheduling).

var _ leavesvc.SchedulePort = (*ScheduleRepo)(nil)

// CancelScheduleEntriesForLeave cancels overlapping live schedule entries on the
// leave dates (DB status='CANCELLED_BY_LEAVE' — the only value the schedule_entries
// CHECK permits for this transition; the leave service maps it to the DTO
// new_status='LEAVE'). RETURNING drives schedule_impact[].
func (r *ScheduleRepo) CancelScheduleEntriesForLeave(ctx context.Context, tx pgx.Tx, employeeID string, start, end time.Time) ([]leavesvc.ScheduleImpact, error) {
	rows, err := r.q.WithTx(tx).CancelScheduleEntriesForLeave(ctx, sqlcgen.CancelScheduleEntriesForLeaveParams{
		EmployeeID: employeeID,
		StartDate:  timeToPgDate(start),
		EndDate:    timeToPgDate(end),
	})
	if err != nil {
		return nil, err
	}
	out := make([]leavesvc.ScheduleImpact, 0, len(rows))
	for _, row := range rows {
		out = append(out, leavesvc.ScheduleImpact{
			ScheduleID: row.ID,
			Date:       pgDateToTime(row.WorkDate),
			NewStatus:  row.Status, // 'CANCELLED_BY_LEAVE'
		})
	}
	return out, nil
}

// InsertApprovedLeaveDay upserts the INV-3 production approved-leave row (the real
// leave_requests.id replaces the Phase-6 fixture). ON CONFLICT keeps it idempotent.
func (r *ScheduleRepo) InsertApprovedLeaveDay(ctx context.Context, tx pgx.Tx, employeeID string, date time.Time, leaveRequestID, leaveType string) error {
	lrID := leaveRequestID
	lt := leaveType
	return r.q.WithTx(tx).InsertApprovedLeaveDay(ctx, sqlcgen.InsertApprovedLeaveDayParams{
		EmployeeID:     employeeID,
		LeaveDate:      timeToPgDate(date),
		LeaveRequestID: &lrID,
		LeaveType:      &lt,
	})
}
