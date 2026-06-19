// Package attendance (repository) — AttendanceRepo implements the attendance
// service port over the 07-01 sqlc queries. Reads on the pool; locked re-checks +
// writes via q.WithTx(tx). pgx.ErrNoRows → domain.ErrNotFound. Verify/Reject
// return the affected-row count (0 ⇒ terminal-state, the service maps to 409).
package attendance

import (
	"context"
	"errors"
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
var _ svc.ReconcileRepo = (*AttendanceRepo)(nil)

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

func boolPtr(b bool) *bool    { return &b }
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
		SiteID:               f.SiteID,
		Position:             f.Position,
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
		IsPayable:        p.IsPayable,
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
		SiteID:             p.SiteID,
		Position:           p.Position,
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
		Mode:               "ONSITE", // manual/correction entries default to ONSITE (migr. 00067)
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
		IsPayable:          p.IsPayable,
		CreatedBy:          p.CreatedBy,
	})
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromCreate(row), nil
}

// SetAttendancePayable flags a day payable/not (F5.4 CR-13). The no-shift guard is
// enforced in the service before this call.
func (r *AttendanceRepo) SetAttendancePayable(ctx context.Context, tx pgx.Tx, id string, payable bool) (att.Attendance, error) {
	p := payable
	row, err := r.q.WithTx(tx).SetAttendancePayable(ctx, sqlcgen.SetAttendancePayableParams{
		IsPayable: &p,
		ID:        id,
	})
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromSetPayable(row), nil
}

// ListStaleOpenAttendances finds open attendance records where shift_end_at has
// elapsed past the cutoff (auto-close sweep).
func (r *AttendanceRepo) ListStaleOpenAttendances(ctx context.Context, cutoff time.Time, limit int) ([]svc.StaleOpenAttendance, error) {
	rows, err := r.pool.Pool.Query(ctx,
		`SELECT id, check_in_at, shift_end_at, flags
		 FROM attendance
		 WHERE check_out_at IS NULL
		   AND shift_end_at IS NOT NULL
		   AND shift_end_at < $1
		   AND deleted_at IS NULL
		 ORDER BY shift_end_at ASC
		 LIMIT $2`,
		cutoff, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []svc.StaleOpenAttendance
	for rows.Next() {
		var a svc.StaleOpenAttendance
		if err := rows.Scan(&a.ID, &a.CheckInAt, &a.ShiftEndAt, &a.Flags); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
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
	var shiftStart, shiftEnd *time.Time
	if row.ScheduleID != nil {
		ss := row.ShiftStartAt
		se := row.ShiftEndAt
		shiftStart = &ss
		shiftEnd = &se
	}
	return svc.ManualAutofillData{
		PlacementID:            row.PlacementID,
		CompanyID:              row.ClientCompanyID,
		SiteID:                 row.SiteID,
		Position:               row.Position,
		EmployeeName:           row.EmployeeName,
		CompanyName:            row.CompanyName,
		SiteName:               row.SiteName,
		ScheduleID:             row.ScheduleID,
		ShiftStartAt:           shiftStart,
		ShiftEndAt:             shiftEnd,
		ExistingAttendanceID:   emptyToNil(row.ExistingAttendanceID),
		ExistingAttendanceStat: emptyToNil(row.ExistingAttendanceStatus),
		ExistingVerification:   emptyToNil(row.ExistingVerificationStatus),
	}, true, nil
}

// emptyToNil maps the COALESCE'd empty-string sentinel back to a nil pointer.
func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *AttendanceRepo) GetActivePlacement(ctx context.Context, employeeID string) (svc.PlacementInfo, bool, error) {
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

// strOrEmpty derefs a *string from a nullable join column (e.position via LEFT JOIN),
// returning "" when null — the placement always has an employee so this is rarely hit.
func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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

// --- F5.7 auto-reconcile (AR-1..AR-10) ---

// FindCandidate returns the earliest UNSCHEDULED attendance row in
// [windowStart, windowEnd] for (employeeID, placementID), FOR UPDATE under tx.
// found=false (no error) when no row matches (AR-3 — no fallback).
func (r *AttendanceRepo) FindCandidate(ctx context.Context, tx pgx.Tx, employeeID, placementID string, windowStart, windowEnd time.Time) (svc.ReconcileCandidate, bool, error) {
	row, err := r.q.WithTx(tx).FindReconcileCandidate(ctx, sqlcgen.FindReconcileCandidateParams{
		EmployeeID:  employeeID,
		PlacementID: placementID,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	})
	if err != nil {
		if isNoRows(err) {
			return svc.ReconcileCandidate{}, false, nil
		}
		return svc.ReconcileCandidate{}, false, err
	}
	checkIn := time.Time{}
	if row.CheckInAt != nil {
		checkIn = *row.CheckInAt
	}
	return svc.ReconcileCandidate{
		ID:                 row.ID,
		CheckInAt:          checkIn,
		Flags:              row.Flags,
		Status:             row.Status,
		VerificationStatus: row.VerificationStatus,
		IsPayable:          row.IsPayable,
		VerifiedBy:         row.VerifiedBy,
		RejectedBy:         row.RejectedBy,
	}, true, nil
}

// Reconcile re-derives + links a machine-owned record (AR-5/6/7/8). found=false when
// the schedule_id IS NULL guard no-ops (a concurrent link won — AR-4).
func (r *AttendanceRepo) Reconcile(ctx context.Context, tx pgx.Tx, p svc.ReconcileParams) (bool, error) {
	scheduleID := p.ScheduleID
	shiftStart := p.ShiftStartAt
	shiftEnd := p.ShiftEndAt
	_, err := r.q.WithTx(tx).ReconcileAttendance(ctx, sqlcgen.ReconcileAttendanceParams{
		ScheduleID:         &scheduleID,
		ShiftStartAt:       &shiftStart,
		ShiftEndAt:         &shiftEnd,
		Status:             p.Status,
		IsLate:             p.IsLate,
		LateMinutes:        int32(p.LateMinutes),
		Flags:              p.Flags,
		VerificationStatus: p.VerificationStatus,
		IsPayable:          p.IsPayable,
		ID:                 p.ID,
	})
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Relink attaches schedule_id + shift-snapshot lineage to a human-decided record
// (AR-9) WITHOUT re-deriving it. found=false when the guard no-ops (AR-4).
func (r *AttendanceRepo) Relink(ctx context.Context, tx pgx.Tx, p svc.RelinkParams) (bool, error) {
	scheduleID := p.ScheduleID
	shiftStart := p.ShiftStartAt
	shiftEnd := p.ShiftEndAt
	_, err := r.q.WithTx(tx).RelinkAttendanceLineage(ctx, sqlcgen.RelinkAttendanceLineageParams{
		ScheduleID:   &scheduleID,
		ShiftStartAt: &shiftStart,
		ShiftEndAt:   &shiftEnd,
		ID:           p.ID,
	})
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
