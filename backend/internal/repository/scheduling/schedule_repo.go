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
