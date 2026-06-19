// Package calendar — Adapters that implement the calendar service ports using
// the pgx pool directly. Each adapter satisfies a per-day port interface.
package calendar

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
)

// --- schedule adapter (uses existing sqlc FindLiveEntryForAgentDate) ---

type scheduleCalendarAdapter struct {
	q *sqlcgen.Queries
}

func NewScheduleCalendarAdapter(pool *db.Pool) ScheduleCalendarPort {
	return &scheduleCalendarAdapter{q: sqlcgen.New(pool.Pool)}
}

func (a *scheduleCalendarAdapter) GetScheduleForAgentDate(ctx context.Context, employeeID string, date time.Time) (*ScheduleCalendarEntry, error) {
	row, err := a.q.FindLiveEntryForAgentDate(ctx, sqlcgen.FindLiveEntryForAgentDateParams{
		EmployeeID: employeeID,
		WorkDate:   toPgDate(date),
	})
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &ScheduleCalendarEntry{
		ID:            row.ID,
		ShiftMasterID: row.ShiftMasterID,
		Status:        row.Status,
		IsDayOff:      row.IsDayOff,
	}, nil
}

// --- leave adapter (direct pool query for approved leave covering a date) ---

type leaveCalendarAdapter struct {
	pool *db.Pool
}

func NewLeaveCalendarAdapter(pool *db.Pool) LeaveCalendarPort {
	return &leaveCalendarAdapter{pool: pool}
}

const leaveDayQuery = `
SELECT lr.id, lr.status,
       lt.name, lt.code,
       lr.start_date, lr.end_date
FROM leave_requests lr
JOIN leave_types lt ON lt.id = lr.leave_type_id
WHERE lr.employee_id = $1
  AND $2::date BETWEEN lr.start_date AND lr.end_date
  AND lr.status = 'APPROVED'
  AND lr.deleted_at IS NULL
LIMIT 1`

func (a *leaveCalendarAdapter) GetLeaveForAgentDate(ctx context.Context, employeeID string, date time.Time) (*LeaveCalendarEntry, error) {
	var e LeaveCalendarEntry
	var startDate, endDate pgtype.Date
	err := a.pool.Pool.QueryRow(ctx, leaveDayQuery, employeeID, toPgDate(date)).Scan(
		&e.LeaveRequestID, &e.Status,
		&e.LeaveTypeName, &e.LeaveTypeCode,
		&startDate, &endDate,
	)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	if startDate.Valid {
		e.StartDate = startDate.Time
	}
	if endDate.Valid {
		e.EndDate = endDate.Time
	}
	return &e, nil
}

// --- holiday adapter (uses existing sqlc GetHolidayForDate) ---

type holidayCalendarAdapter struct {
	q *sqlcgen.Queries
}

func NewHolidayCalendarAdapter(pool *db.Pool) HolidayCalendarPort {
	return &holidayCalendarAdapter{q: sqlcgen.New(pool.Pool)}
}

func (a *holidayCalendarAdapter) GetHolidayForDate(ctx context.Context, date time.Time) (*HolidayCalendarEntry, error) {
	row, err := a.q.GetHolidayForDate(ctx, toPgDate(date))
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &HolidayCalendarEntry{
		ID:   row.ID,
		Name: row.Name,
	}, nil
}

// --- attendance adapter (direct pool query for attendance on a date) ---

type attendanceCalendarAdapter struct {
	pool *db.Pool
}

func NewAttendanceCalendarAdapter(pool *db.Pool) AttendanceCalendarPort {
	return &attendanceCalendarAdapter{pool: pool}
}

const attendanceDayQuery = `
SELECT id, status, check_in_at, check_out_at
FROM attendance
WHERE employee_id = $1
  AND (COALESCE(shift_start_at, check_in_at) AT TIME ZONE 'Asia/Jakarta')::date = $2::date
  AND deleted_at IS NULL
ORDER BY check_in_at DESC
LIMIT 1`

func (a *attendanceCalendarAdapter) GetAttendanceForAgentDate(ctx context.Context, employeeID string, date time.Time) (*AttendanceCalendarEntry, error) {
	var (
		id       string
		status   string
		checkIn  *time.Time
		checkOut *time.Time
	)
	err := a.pool.Pool.QueryRow(ctx, attendanceDayQuery, employeeID, toPgDate(date)).Scan(
		&id, &status, &checkIn, &checkOut,
	)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	e := &AttendanceCalendarEntry{ID: id, Status: status}
	if checkIn != nil {
		s := checkIn.UTC().Format("15:04")
		e.ClockIn = &s
	}
	if checkOut != nil {
		s := checkOut.UTC().Format("15:04")
		e.ClockOut = &s
	}
	return e, nil
}

// --- helpers ---

func toPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return err == pgx.ErrNoRows ||
		err == domain.ErrNotFound ||
		err.Error() == "no rows in result set"
}
