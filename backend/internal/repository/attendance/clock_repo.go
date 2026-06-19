// Package attendance (repository) — ClockRepo implements the agent clock-in/out
// service port (F5.1) over the clock.sql queries + the existing placement/site
// generated queries. Reads run on the pool; the clock-in INSERT / clock-out UPDATE run
// via q.WithTx(tx) so they share the audit row's transaction. The ON CONFLICT DO
// NOTHING clock-in yields pgx.ErrNoRows on a no-op → created=false. position is the
// free-text label carried on the placement (no service_line).
package attendance

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// ClockRepo is the sqlc-backed implementation of svc.ClockRepository.
type ClockRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ClockRepository = (*ClockRepo)(nil)

// NewClockRepo returns a ClockRepo backed by pool.
func NewClockRepo(pool *db.Pool) *ClockRepo {
	return &ClockRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// GetActivePlacement resolves the agent's single active placement → the denormalized
// company/site/position. found=false when no active placement exists (the service maps
// that to NO_ACTIVE_PLACEMENT). Position is the free-text label from the placement.
func (r *ClockRepo) GetActivePlacement(ctx context.Context, employeeID string) (svc.PlacementInfo, bool, error) {
	row, err := r.q.GetActivePlacementForEmployee(ctx, employeeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return svc.PlacementInfo{}, false, nil
		}
		return svc.PlacementInfo{}, false, err
	}
	return svc.PlacementInfo{
		PlacementID: row.ID,
		CompanyID:   row.ClientCompanyID,
		SiteID:      row.SiteID,
		Position:    strOrEmpty(row.Position),
	}, true, nil
}

// EmployeeType returns the employee's type ("FIELD" | "INTERNAL", migr 00067) — the
// no-shift clock-in gate (FIELD requires a rostered shift). Raw query (no dedicated sqlc).
func (r *ClockRepo) EmployeeType(ctx context.Context, employeeID string) (string, error) {
	var t string
	if err := r.pool.Pool.QueryRow(ctx,
		`SELECT employee_type FROM employees WHERE id = $1`, employeeID,
	).Scan(&t); err != nil {
		return "", err
	}
	return t, nil
}

// GetSite returns the site's geofence center + radius. found=false when the site is
// missing/soft-deleted (the service then skips the geofence check).
func (r *ClockRepo) GetSite(ctx context.Context, siteID string) (*float64, *float64, int, bool, error) {
	row, err := r.q.GetSiteByID(ctx, siteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, 0, false, nil
		}
		return nil, nil, 0, false, err
	}
	return row.GeoLat, row.GeoLng, int(row.GeofenceRadiusM), true, nil
}

// GetTodaySchedule resolves today's (Asia/Jakarta) live schedule entry. found=false
// when the agent has no work day today (the service marks the clock-in unscheduled).
func (r *ClockRepo) GetTodaySchedule(ctx context.Context, employeeID string, now time.Time) (string, time.Time, time.Time, bool, error) {
	row, err := r.q.GetTodayScheduleForEmployee(ctx, sqlcgen.GetTodayScheduleForEmployeeParams{
		EmployeeID: employeeID,
		Now:        now,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, time.Time{}, false, nil
		}
		return "", time.Time{}, time.Time{}, false, err
	}
	return row.ScheduleID, row.ShiftStartAt, row.ShiftEndAt, true, nil
}

// IsOnApprovedLeave reports whether the agent has an approved leave covering today
// (Asia/Jakarta calendar date). Reuses the shared FindApprovedLeaveForAgentDate query;
// pgx.ErrNoRows (no covering leave) → (false, nil). The service hard-blocks clock-in
// (ON_LEAVE) when this returns true.
func (r *ClockRepo) IsOnApprovedLeave(ctx context.Context, employeeID string, now time.Time) (bool, error) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	d := now.In(loc)
	jakartaDate := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	_, err = r.q.FindApprovedLeaveForAgentDate(ctx, sqlcgen.FindApprovedLeaveForAgentDateParams{
		EmployeeID: employeeID,
		LeaveDate:  pgtype.Date{Time: jakartaDate, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetOpenAttendance returns the caller's currently-open record id. found=false when
// none is open.
func (r *ClockRepo) GetOpenAttendance(ctx context.Context, employeeID string) (string, bool, error) {
	id, err := r.q.GetOpenAttendanceForEmployee(ctx, employeeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return id, true, nil
}

// ClockIn inserts one clock-in row in tx. created=false (no error) when ON CONFLICT DO
// NOTHING no-ops (a row already exists for this schedule).
func (r *ClockRepo) ClockIn(ctx context.Context, tx pgx.Tx, p svc.ClockInRow) (string, bool, error) {
	checkIn := p.CheckInAt
	latIn := p.LatIn
	lngIn := p.LngIn
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO attendance (
		     employee_id, placement_id, schedule_id, company_id,
		     site_id, position,
		     shift_start_at, shift_end_at,
		     check_in_at, lat_in, lng_in, photo_in_id,
		     wfo, is_late, late_minutes,
		     in_geofence, in_distance_m, geofence_radius_m,
		     status, verification_status, flags, mode, attendance_code_id
		 ) VALUES (
		     $1, $2, $3, $4, $5, $6,
		     $7, $8, $9, $10, $11, $12,
		     $13, $14, $15, $16, $17, $18,
		     $19, $20, $21, $22, $23
		 )
		 ON CONFLICT (schedule_id) WHERE schedule_id IS NOT NULL AND deleted_at IS NULL
		 DO NOTHING
		 RETURNING id`,
		p.EmployeeID, p.PlacementID, p.ScheduleID,
		p.CompanyID, p.SiteID, p.Position,
		p.ShiftStartAt, p.ShiftEndAt,
		&checkIn, &latIn, &lngIn, p.PhotoInID,
		p.WFO, p.IsLate, int32(p.LateMinutes),
		p.InGeofence, intPtrToI32Ptr(p.InDistanceM), int32(p.GeofenceRadiusM),
		p.Status, p.VerificationStatus, p.Flags, p.Mode, p.AttendanceCodeID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil // ON CONFLICT DO NOTHING — schedule already has a row
		}
		return "", false, err
	}
	return id, true, nil
}

// ClockOut closes the open record in tx.
func (r *ClockRepo) ClockOut(ctx context.Context, tx pgx.Tx, p svc.ClockOutRow) (string, error) {
	checkOut := p.CheckOutAt
	latOut := p.LatOut
	lngOut := p.LngOut
	worked := int32(p.WorkedMinutes)
	id, err := r.q.WithTx(tx).ClockOutAttendance(ctx, sqlcgen.ClockOutAttendanceParams{
		CheckOutAt:         &checkOut,
		LatOut:             &latOut,
		LngOut:             &lngOut,
		PhotoOutID:         p.PhotoOutID,
		OutGeofence:        p.OutGeofence,
		OutDistanceM:       intPtrToI32Ptr(p.OutDistanceM),
		WorkedMinutes:      &worked,
		Flags:              p.Flags,
		Status:             p.Status,
		VerificationStatus: p.VerificationStatus,
		ID:                 p.ID,
	})
	if err != nil {
		return "", mapErr(err)
	}
	return id, nil
}

// AutoCloseAttendance stamps a stale open record closed at its computed shift_end in tx
// (F5.1 flexible check-in). found=false (no error) when the guarded UPDATE matched no
// row — a concurrent clock-out already closed it — which the service treats as a no-op.
func (r *ClockRepo) AutoCloseAttendance(ctx context.Context, tx pgx.Tx, p svc.AutoCloseRow) (string, bool, error) {
	checkOut := p.CheckOutAt
	worked := int32(p.WorkedMinutes)
	id, err := r.q.WithTx(tx).AutoCloseAttendance(ctx, sqlcgen.AutoCloseAttendanceParams{
		CheckOutAt:         &checkOut,
		WorkedMinutes:      &worked,
		Flags:              p.Flags,
		Status:             p.Status,
		VerificationStatus: p.VerificationStatus,
		ID:                 p.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return id, true, nil
}

// GetAttendance re-reads the full record (denormalized names + assembled geofence) via
// the existing GetAttendance query + mapper.
func (r *ClockRepo) GetAttendance(ctx context.Context, id string) (att.Attendance, error) {
	row, err := r.q.GetAttendance(ctx, id)
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromGet(row), nil
}

// CountActivities returns the count of non-deleted activities on a record — the agent
// clock-out gate (AA-7 / INV-7). Reads on the pool; the clock service reads it before
// the clock-out tx so a racing add/delete resolves on the persisted count (C-12).
func (r *ClockRepo) CountActivities(ctx context.Context, attendanceID string) (int64, error) {
	return r.q.CountActivitiesByAttendance(ctx, attendanceID)
}

// GetDefaultAttendanceCode returns the id and name of the default active attendance code
// (e.g. "PRESENT"). Used at clock-in to set the attendance_code_id on the new record.
func (r *ClockRepo) GetDefaultAttendanceCode(ctx context.Context, code string) (string, string, error) {
	var id, name string
	err := r.pool.Pool.QueryRow(ctx,
		`SELECT id, name FROM attendance_codes
		 WHERE code = $1 AND status = 'ACTIVE' AND deleted_at IS NULL
		 LIMIT 1`,
		code,
	).Scan(&id, &name)
	if err != nil {
		return "", "", err
	}
	return id, name, nil
}
