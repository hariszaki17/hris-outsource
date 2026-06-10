// Package attendance (repository) — AttendanceRepo implements the attendance
// service port over the 07-01 sqlc queries. Reads on the pool; locked re-checks +
// writes via q.WithTx(tx). pgx.ErrNoRows → domain.ErrNotFound. Verify/Reject
// return the affected-row count (0 ⇒ terminal-state, the service maps to 409).
package attendance

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// AttendanceRepo is the sqlc-backed implementation of svc.AttendanceRepository.
type AttendanceRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.AttendanceRepository = (*AttendanceRepo)(nil)

// NewAttendanceRepo returns an AttendanceRepo backed by pool.
func NewAttendanceRepo(pool *db.Pool) *AttendanceRepo {
	return &AttendanceRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func timePtrToPgDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// intPtrToI32Ptr narrows a *int to *int32 for sqlc nullable integer params.
func intPtrToI32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	n := int32(*v)
	return &n
}

func boolPtr(b bool) *bool { return &b }
func int32Ptr(n int32) *int32 { return &n }

func (r *AttendanceRepo) ListAttendance(ctx context.Context, f svc.AttendanceFilter) ([]att.Attendance, error) {
	var exceptions *bool
	if f.ExceptionsOnly {
		t := true
		exceptions = &t
	}
	rows, err := r.q.ListAttendance(ctx, sqlcgen.ListAttendanceParams{
		CompanyID:            f.CompanyID,
		EmployeeID:           f.EmployeeID,
		ServiceLine:          f.ServiceLine,
		SiteID:               f.SiteID,
		PositionID:           f.PositionID,
		VerificationStatusIn: f.VerificationStatus,
		StatusIn:             f.Status,
		DateFrom:             timePtrToPgDate(f.DateFrom),
		DateTo:               timePtrToPgDate(f.DateTo),
		Exceptions:           exceptions,
		CursorCheckInAt:      f.CursorCheckInAt,
		CursorID:             f.CursorID,
		PageLimit:            int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]att.Attendance, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAttendanceFromList(row))
	}
	return out, nil
}

func (r *AttendanceRepo) GetAttendance(ctx context.Context, id string) (att.Attendance, error) {
	row, err := r.q.GetAttendance(ctx, id)
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromGet(row), nil
}

func (r *AttendanceRepo) GetAttendanceForUpdate(ctx context.Context, tx pgx.Tx, id string) (att.Attendance, error) {
	row, err := r.q.WithTx(tx).GetAttendanceForUpdate(ctx, id)
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromForUpdate(row), nil
}

func (r *AttendanceRepo) VerifyAttendance(ctx context.Context, tx pgx.Tx, id string, verifiedBy *string) (att.Attendance, int64, error) {
	row, err := r.q.WithTx(tx).VerifyAttendance(ctx, sqlcgen.VerifyAttendanceParams{
		VerifiedBy: verifiedBy,
		ID:         id,
	})
	if err != nil {
		if isNoRows(err) {
			return att.Attendance{}, 0, nil // terminal state — service emits 409
		}
		return att.Attendance{}, 0, err
	}
	return mapAttendanceFromVerify(row), 1, nil
}

func (r *AttendanceRepo) VerifyAttendanceWithTimes(ctx context.Context, tx pgx.Tx, id string, checkInAt time.Time, checkOutAt *time.Time, status string, isLate bool, lateMinutes int, verifiedBy *string) (att.Attendance, int64, error) {
	lateMin := int32(lateMinutes)
	row, err := r.q.WithTx(tx).VerifyAttendanceWithTimes(ctx, sqlcgen.VerifyAttendanceWithTimesParams{
		CheckInAt:   checkInAt,
		CheckOutAt:  checkOutAt,
		Status:      &status,
		IsLate:      &isLate,
		LateMinutes: &lateMin,
		VerifiedBy:  verifiedBy,
		ID:          id,
	})
	if err != nil {
		if isNoRows(err) {
			return att.Attendance{}, 0, nil
		}
		return att.Attendance{}, 0, err
	}
	return mapAttendanceFromVerify(sqlcgen.VerifyAttendanceRow(sqlcgen.VerifyAttendanceWithTimesRow(row))), 1, nil
}

func (r *AttendanceRepo) RejectAttendance(ctx context.Context, tx pgx.Tx, id string, rejectedBy *string, reason string) (att.Attendance, int64, error) {
	rsn := reason
	row, err := r.q.WithTx(tx).RejectAttendance(ctx, sqlcgen.RejectAttendanceParams{
		RejectedBy:   rejectedBy,
		RejectReason: &rsn,
		ID:           id,
	})
	if err != nil {
		if isNoRows(err) {
			return att.Attendance{}, 0, nil
		}
		return att.Attendance{}, 0, err
	}
	return mapAttendanceFromReject(row), 1, nil
}

func (r *AttendanceRepo) ApplyCorrectionToAttendance(ctx context.Context, tx pgx.Tx, p svc.ApplyCorrectionParams) (att.Attendance, error) {
	row, err := r.q.WithTx(tx).ApplyCorrectionToAttendance(ctx, sqlcgen.ApplyCorrectionToAttendanceParams{
		CheckInAt:        p.CheckInAt,
		CheckOutAt:       p.CheckOutAt,
		AttendanceCodeID: p.AttendanceCodeID,
		Status:           p.Status,
		IsLate:           p.IsLate,
		LateMinutes:      intPtrToI32Ptr(p.LateMinutes),
		LastCorrectionID: p.LastCorrectionID,
		ID:               p.ID,
	})
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromApply(row), nil
}

func (r *AttendanceRepo) CreateManualAttendance(ctx context.Context, tx pgx.Tx, p svc.CreateManualAttendanceParams) (att.Attendance, error) {
	lateMin := int32(p.LateMinutes)
	var workedMin *int32
	if p.WorkedMinutes != nil {
		wm := int32(*p.WorkedMinutes)
		workedMin = &wm
	}
	checkInAt := p.CheckInAt
	row, err := r.q.WithTx(tx).CreateManualAttendance(ctx, sqlcgen.CreateManualAttendanceParams{
		EmployeeID:         p.EmployeeID,
		PlacementID:        p.PlacementID,
		ScheduleID:         p.ScheduleID,
		CompanyID:          p.CompanyID,
		ServiceLine:        p.ServiceLine,
		SiteID:             p.SiteID,
		PositionID:         p.PositionID,
		AttendanceCodeID:   p.AttendanceCodeID,
		ShiftStartAt:       p.ShiftStartAt,
		ShiftEndAt:         p.ShiftEndAt,
		CheckInAt:          &checkInAt,
		CheckOutAt:         p.CheckOutAt,
		LatIn:              nil,
		LngIn:              nil,
		LatOut:             nil,
		LngOut:             nil,
		Wfo:                p.WFO,
		IsLate:             p.IsLate,
		LateMinutes:        lateMin,
		WorkedMinutes:      workedMin,
		InGeofence:         boolPtr(true),
		InDistanceM:        int32Ptr(0),
		OutGeofence:        nil,
		OutDistanceM:       nil,
		GeofenceRadiusM:    0,
		Status:             p.Status,
		VerificationStatus: p.VerificationStatus,
		Flags:              p.Flags,
		CreatedBy:          p.CreatedBy,
	})
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromCreate(row), nil
}



func (r *AttendanceRepo) GetManualAutofillData(ctx context.Context, employeeID string, refDate time.Time) (svc.ManualAutofillData, bool, error) {
	refPG := pgtype.Date{Time: refDate, Valid: true}
	row, err := r.q.GetManualAutofillData(ctx, sqlcgen.GetManualAutofillDataParams{
		RefDate:    refPG,
		EmployeeID: employeeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return svc.ManualAutofillData{}, false, nil
		}
		return svc.ManualAutofillData{}, false, err
	}
	serviceLine := strings.ToLower(strings.ReplaceAll(row.ServiceLineName, " ", "_"))
	var shiftStart, shiftEnd *time.Time
	if row.ScheduleID != nil {
		ss := row.ShiftStartAt
		se := row.ShiftEndAt
		shiftStart = &ss
		shiftEnd = &se
	}
	return svc.ManualAutofillData{
		PlacementID:  row.PlacementID,
		CompanyID:    row.ClientCompanyID,
		ServiceLine:  serviceLine,
		SiteID:       row.SiteID,
		PositionID:   row.PositionID,
		EmployeeName: row.EmployeeName,
		CompanyName:  row.CompanyName,
		SiteName:     row.SiteName,
		PositionName: row.PositionName,
		ScheduleID:   row.ScheduleID,
		ShiftStartAt: shiftStart,
		ShiftEndAt:   shiftEnd,
	}, true, nil
}

func (r *AttendanceRepo) GetActivePlacement(ctx context.Context, employeeID string) (svc.PlacementInfo, bool, error) {
	row, err := r.q.GetActivePlacementForEmployee(ctx, employeeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return svc.PlacementInfo{}, false, nil
		}
		return svc.PlacementInfo{}, false, err
	}
	serviceLine := ""
	if row.ServiceLineName != nil {
		serviceLine = strings.ToLower(strings.ReplaceAll(*row.ServiceLineName, " ", "_"))
	}
	return svc.PlacementInfo{
		PlacementID: row.ID,
		CompanyID:   row.ClientCompanyID,
		SiteID:      row.SiteID,
		PositionID:  row.PositionID,
		ServiceLine: serviceLine,
	}, true, nil
}

func (r *AttendanceRepo) GetTodaySchedule(ctx context.Context, employeeID string, now time.Time) (string, time.Time, time.Time, bool, error) {
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

// isNoRows reports whether the error is the :one "no rows" sentinel — used by the
// state-guarded UPDATE...RETURNING queries to detect a terminal-state no-op.
func isNoRows(err error) bool {
	return err == pgx.ErrNoRows || mapErr(err) == domain.ErrNotFound
}
