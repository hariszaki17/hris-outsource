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
	leavedom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
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
		PlacementID: row.ID,
		CompanyID:   row.ClientCompanyID,
		StartDate:   pgDateToTime(row.StartDate),
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
// Uses raw SQL so site/geo columns are available before make gen regenerates the
// sqlc ListScheduleByAgentRow type.
func (r *ScheduleRepo) ListScheduleByAgent(ctx context.Context, employeeID string, start, end time.Time) ([]domain.ScheduleEntry, error) {
	rows, err := r.pool.Pool.Query(ctx,
		`SELECT se.id, se.employee_id, se.placement_id,
		        se.shift_master_id, se.start_time, se.end_time, se.cross_midnight,
		        se.work_date, se.status, se.is_day_off, se.replaced_entry_id,
		        se.created_by, se.created_at, se.updated_at,
		        e.full_name AS employee_name,
		        p.client_company_id AS company_id,
		        c.name AS company_name,
		        sm.name AS shift_master_name,
		        cs.id AS site_id,
		        cs.name AS site_name,
		        cs.geo_lat AS site_geo_lat,
		        cs.geo_lng AS site_geo_lng
		 FROM schedule_entries se
		 JOIN placements p             ON p.id  = se.placement_id
		 LEFT JOIN client_companies c  ON c.id  = p.client_company_id
		 LEFT JOIN employees e         ON e.id  = se.employee_id
		 LEFT JOIN shift_masters sm    ON sm.id = se.shift_master_id
		 LEFT JOIN client_sites cs     ON cs.id = p.site_id
		 WHERE se.deleted_at IS NULL
		   AND se.employee_id = $1
		   AND se.work_date BETWEEN $2 AND $3
		 ORDER BY se.work_date ASC, se.start_time ASC, se.id ASC`,
		employeeID, start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ScheduleEntry
	for rows.Next() {
		var e domain.ScheduleEntry
		var wo time.Time
		if err := rows.Scan(
			&e.ID, &e.EmployeeID, &e.PlacementID,
			&e.ShiftMasterID, &e.StartTime, &e.EndTime, &e.CrossMidnight,
			&wo, &e.Status, &e.IsDayOff, &e.ReplacedEntryID,
			&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
			&e.EmployeeName,
			&e.CompanyID,
			&e.CompanyName,
			&e.ShiftMasterName,
			&e.SiteID,
			&e.SiteName,
			&e.SiteGeoLat,
			&e.SiteGeoLng,
		); err != nil {
			return nil, err
		}
		e.WorkDate = wo
		out = append(out, e)
	}
	return out, rows.Err()
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

// CancelFutureSchedulesForEmployee cancels all future live schedule entries for an
// employee after a given date. Used by the placement service on :end to clear the
// agent's future roster.
func (r *ScheduleRepo) CancelFutureSchedulesForEmployee(ctx context.Context, tx pgx.Tx, employeeID string, afterDate time.Time) error {
	_, err := r.pool.Pool.Exec(ctx,
		`UPDATE schedule_entries SET status = 'CANCELLED', updated_at = now()
		 WHERE employee_id = $1 AND work_date > $2::date
		   AND status IN ('SCHEDULED', 'OFF') AND deleted_at IS NULL`,
		employeeID, afterDate,
	)
	return err
}

// ListScheduleForAggregate returns schedule entries for a company over a date range
// with employee and shift names for aggregation.
func (r *ScheduleRepo) ListScheduleForAggregate(ctx context.Context, companyID string, start, end time.Time) ([]domain.ScheduleEntry, error) {
	rows, err := r.pool.Pool.Query(ctx,
		`SELECT se.id, se.employee_id, se.shift_master_id, se.work_date, se.start_time, se.end_time,
		        se.status, se.is_day_off,
		        e.full_name AS employee_name,
		        sm.name AS shift_name
		 FROM schedule_entries se
		 LEFT JOIN employees e ON e.id = se.employee_id
		 LEFT JOIN shift_masters sm ON sm.id = se.shift_master_id
		 WHERE se.company_id = $1
		   AND se.work_date BETWEEN $2 AND $3
		   AND se.deleted_at IS NULL
		 ORDER BY se.work_date, se.start_time`,
		companyID, start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ScheduleEntry
	for rows.Next() {
		var e domain.ScheduleEntry
		if err := rows.Scan(&e.ID, &e.EmployeeID, &e.ShiftMasterID, &e.WorkDate,
			&e.StartTime, &e.EndTime, &e.Status, &e.IsDayOff,
			&e.EmployeeName, &e.ShiftMasterName); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
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

// CountLeaveDuration is the server-authoritative F6.2 leave duration, HYBRID
// (2026-06-18): when the agent HAS rostered shifts in [start,end], every scheduled
// non-day-off day counts (incl. weekend/holiday shifts — a holiday shift is real work
// the agent is excused from). When the range has NO roster at all (e.g. leave requested
// before the roster is published), it falls back to calendar working-days — Mon–Fri in
// [start,end] minus public holidays — so future leave still charges a sensible quota
// instead of 0. Backs leavesvc.SchedulePort so the leave Create path never re-implements
// a naive day-count.
func (r *ScheduleRepo) CountLeaveDuration(ctx context.Context, employeeID string, start, end time.Time) (int, error) {
	const q = `
WITH rostered AS (
  SELECT count(*) AS n
  FROM schedule_entries
  WHERE employee_id = $1
    AND work_date BETWEEN $2 AND $3
    AND NOT is_day_off
    AND status <> 'CANCELLED_BY_LEAVE'
    AND deleted_at IS NULL
),
working AS (
  SELECT count(*) AS n
  FROM generate_series($2::date, $3::date, interval '1 day') AS d
  WHERE extract(isodow FROM d) < 6                       -- Mon–Fri
    AND d::date NOT IN (
      SELECT holiday_date FROM holidays WHERE deleted_at IS NULL
    )
)
SELECT CASE WHEN (SELECT n FROM rostered) > 0
            THEN (SELECT n FROM rostered)
            ELSE (SELECT n FROM working) END`
	var n int64
	if err := r.pool.Pool.QueryRow(ctx, q, employeeID, timeToPgDate(start), timeToPgDate(end)).Scan(&n); err != nil {
		return 0, err
	}
	return int(n), nil
}

// InsertApprovedLeaveDay upserts the INV-3 production approved-leave row (the real
// leave_requests.id replaces the Phase-6 fixture). ON CONFLICT keeps it idempotent.
// hadShift / isPayable carry per-day payability (migr. 00064).
func (r *ScheduleRepo) InsertApprovedLeaveDay(ctx context.Context, tx pgx.Tx, employeeID string, date time.Time, leaveRequestID, leaveType string, hadShift bool, isPayable *bool) error {
	lrID := leaveRequestID
	lt := leaveType
	return r.q.WithTx(tx).InsertApprovedLeaveDay(ctx, sqlcgen.InsertApprovedLeaveDayParams{
		EmployeeID:     employeeID,
		LeaveDate:      timeToPgDate(date),
		LeaveRequestID: &lrID,
		LeaveType:      &lt,
		HadShift:       hadShift,
		IsPayable:      isPayable,
	})
}

// ListApprovedLeaveDays returns the per-day payability breakdown for a leave request.
func (r *ScheduleRepo) ListApprovedLeaveDays(ctx context.Context, leaveRequestID string) ([]leavedom.LeaveDayPayability, error) {
	lrID := leaveRequestID
	rows, err := r.q.ListApprovedLeaveDaysForRequest(ctx, &lrID)
	if err != nil {
		return nil, err
	}
	out := make([]leavedom.LeaveDayPayability, 0, len(rows))
	for _, row := range rows {
		out = append(out, leavedom.LeaveDayPayability{
			Date:      pgDateToTime(row.LeaveDate),
			LeaveType: strDeref(row.LeaveType),
			HadShift:  row.HadShift,
			IsPayable: row.IsPayable,
		})
	}
	return out, nil
}

// strDeref returns the pointed-to string, or "" when nil.
func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// SetLeaveDayPayable flags a NO-SHIFT approved-leave day payable/non-payable (the SQL
// guards had_shift=false). Returns domain.ErrNotFound when not a flaggable day.
func (r *ScheduleRepo) SetLeaveDayPayable(ctx context.Context, tx pgx.Tx, leaveRequestID string, date time.Time, payable bool) (leavedom.LeaveDayPayability, error) {
	lrID := leaveRequestID
	p := payable
	row, err := r.q.WithTx(tx).SetApprovedLeaveDayPayable(ctx, sqlcgen.SetApprovedLeaveDayPayableParams{
		IsPayable:      &p,
		LeaveRequestID: &lrID,
		LeaveDate:      timeToPgDate(date),
	})
	if err != nil {
		return leavedom.LeaveDayPayability{}, mapErr(err)
	}
	return leavedom.LeaveDayPayability{
		Date:      pgDateToTime(row.LeaveDate),
		LeaveType: strDeref(row.LeaveType),
		HadShift:  row.HadShift,
		IsPayable: row.IsPayable,
	}, nil
}
