// Package attendance — E5 attendance activity log service (F5.8 / SWP-ACT-*).
// An agent logs free-text activity notes onto their OWN OPEN attendance record; the
// list is readable by anyone who may read the parent attendance (AA-10). Agent
// clock-out is gated on >=1 non-deleted activity (INV-7 / AA-7) — that guard lives in
// the clock service (ClockOut), this service owns create / list / delete.
//
// Decisions (openapi docs/api/E5-attendance, AA-1..AA-13):
//   - note: trimmed, 1..500 chars → else 400 INVALID_REQUEST (field note) (AA-6).
//   - scope:self for writes — the record must belong to the principal (AA-2); a
//     cross-employee record is hidden as 404 (no existence leak), matching the
//     attendance Get scope rule.
//   - open-record only for writes (AA-3): a true ABSENT record (no check_in) →
//     422 ATTENDANCE_NOT_OPEN; an already-closed record (check_out set) →
//     422 ATTENDANCE_CLOSED.
//   - delete is soft + own + while-open (AA-9): rows-affected 0 → 404.
package attendance

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// activityNoteMaxLen is the trimmed note bound (AA-6).
const activityNoteMaxLen = 500

// ActivityRepository is the data dependency for the activity service.
type ActivityRepository interface {
	// CreateActivity inserts one activity (in tx) and returns the created row
	// (id + recorded_at + created_at are server-set).
	CreateActivity(ctx context.Context, tx pgx.Tx, p CreateActivityParams) (att.AttendanceActivity, error)
	// ListActivities returns the chronological (recorded_at asc, id asc) live
	// activities for one attendance, keyset-paged.
	ListActivities(ctx context.Context, f ActivityFilter) ([]att.AttendanceActivity, error)
	// SoftDeleteActivity soft-deletes the creator's own live activity (in tx) and
	// returns rows affected (0 → not found / not owner / already deleted → 404).
	SoftDeleteActivity(ctx context.Context, tx pgx.Tx, id, attendanceID, employeeID string) (int64, error)
}

// CreateActivityParams is the repo-layer insert payload.
type CreateActivityParams struct {
	AttendanceID string
	EmployeeID   string
	Note         string
}

// ActivityFilter is the decoded GET /attendance/{id}/activities query (cursor-paged).
type ActivityFilter struct {
	AttendanceID     string
	Limit            int
	CursorRecordedAt *time.Time // opaque cursor tail, nil on the first page
	CursorID         *string
}

// ActivityService implements the attendance activity log business logic. It reuses
// the attendance repo (AttendanceRepository) to resolve the parent record for the
// scope + open/closed checks.
type ActivityService struct {
	repo    ActivityRepository
	attRepo AttendanceRepository
	txm     TxRunner
}

// NewActivityService wires the activity service. It needs the attendance repo to load
// the parent record for the scope + open-record guards.
func NewActivityService(repo ActivityRepository, attRepo AttendanceRepository, txm TxRunner) *ActivityService {
	return &ActivityService{repo: repo, attRepo: attRepo, txm: txm}
}

// --- create ---

// Create logs one activity onto the caller's own open attendance record (AA-2/AA-3).
// note is trimmed + validated (1..500, AA-6); the record is loaded for the
// ownership + open-record guards; the insert + audit run in one tx.
func (s *ActivityService) Create(ctx context.Context, attendanceID, note string) (att.AttendanceActivity, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return att.AttendanceActivity{}, apperr.Unauthenticated()
	}
	if p.EmployeeID == "" {
		return att.AttendanceActivity{}, apperr.OutOfScope()
	}

	trimmed := strings.TrimSpace(note)
	if len(trimmed) < 1 || len(trimmed) > activityNoteMaxLen {
		return att.AttendanceActivity{}, apperr.Invalid(map[string]string{
			"note": "Catatan aktivitas wajib diisi, maksimal 500 karakter.",
		})
	}

	rec, gerr := s.loadOwnOpenRecord(ctx, attendanceID, p.EmployeeID)
	if gerr != nil {
		return att.AttendanceActivity{}, gerr
	}

	var out att.AttendanceActivity
	txErr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		created, cerr := s.repo.CreateActivity(ctx, tx, CreateActivityParams{
			AttendanceID: rec.ID,
			EmployeeID:   p.EmployeeID,
			Note:         trimmed,
		})
		if cerr != nil {
			return cerr
		}
		out = created
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "attendance_activity",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"attendance_id": rec.ID,
				"employee_id":   p.EmployeeID,
				"note":          trimmed,
			},
		})
	})
	if txErr != nil {
		return att.AttendanceActivity{}, asAppErr(txErr)
	}
	return out, nil
}

// --- list ---

// List returns the activity page for one attendance. Read scope mirrors attendance
// read (AA-10): the agent for their own record; shift_leader/HR/super_admin/lead per
// their attendance scope. Out-of-scope is hidden as 404 (no existence leak).
func (s *ActivityService) List(ctx context.Context, attendanceID string, limit int, cursor string) ([]att.AttendanceActivity, *string, bool, error) {
	if _, gerr := s.loadReadableRecord(ctx, attendanceID); gerr != nil {
		return nil, nil, false, gerr
	}

	cra, cid, derr := DecodeActivityCursor(cursor)
	if derr != nil {
		return nil, nil, false, derr
	}

	clamped := clampLimit(limit)
	rows, err := s.repo.ListActivities(ctx, ActivityFilter{
		AttendanceID:     attendanceID,
		Limit:            clamped + 1,
		CursorRecordedAt: cra,
		CursorID:         cid,
	})
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}
	hasMore := len(rows) > clamped
	if hasMore {
		rows = rows[:clamped]
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c, cerr := encodeActivityCursor(last.RecordedAt, last.ID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// --- delete ---

// Delete soft-deletes the caller's own activity while the record is open (AA-9).
// An already-closed record → 422 ATTENDANCE_CLOSED; rows-affected 0 (not found / not
// owner / already deleted) → 404.
func (s *ActivityService) Delete(ctx context.Context, attendanceID, activityID string) error {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apperr.Unauthenticated()
	}
	if p.EmployeeID == "" {
		return apperr.OutOfScope()
	}

	rec, gerr := s.loadOwnOpenRecord(ctx, attendanceID, p.EmployeeID)
	if gerr != nil {
		return gerr
	}

	txErr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rows, derr := s.repo.SoftDeleteActivity(ctx, tx, activityID, rec.ID, p.EmployeeID)
		if derr != nil {
			return derr
		}
		if rows == 0 {
			return apperr.NotFound()
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionDelete,
			EntityType: "attendance_activity",
			EntityID:   activityID,
			Before:     map[string]any{"attendance_id": rec.ID, "employee_id": p.EmployeeID},
			After:      nil,
		})
	})
	if txErr != nil {
		return asAppErr(txErr)
	}
	return nil
}

// --- shared guards ---

// loadOwnOpenRecord loads the attendance and enforces the write guards (AA-2/AA-3):
// not found → 404; not the caller's record → 404 (no existence leak); no check_in
// (true ABSENT) → 422 ATTENDANCE_NOT_OPEN; already clocked out → 422 ATTENDANCE_CLOSED.
func (s *ActivityService) loadOwnOpenRecord(ctx context.Context, attendanceID, employeeID string) (att.Attendance, error) {
	rec, err := s.attRepo.GetAttendance(ctx, attendanceID)
	if errors.Is(err, domain.ErrNotFound) {
		return att.Attendance{}, apperr.NotFound()
	}
	if err != nil {
		return att.Attendance{}, apperr.Internal(err)
	}
	// scope:self — a cross-employee record is hidden as 404 (AA-2).
	if rec.EmployeeID != employeeID {
		return att.Attendance{}, apperr.NotFound()
	}
	if rec.CheckInAt == nil {
		return att.Attendance{}, apperr.Rule("ATTENDANCE_NOT_OPEN", nil)
	}
	if rec.CheckOutAt != nil {
		return att.Attendance{}, apperr.Rule("ATTENDANCE_CLOSED", nil)
	}
	return rec, nil
}

// loadReadableRecord loads the attendance and enforces read scope (AA-10), mirroring
// AttendanceService.Get: the agent reads only their own record; everyone else passes
// through the company scope guard. Out-of-scope / cross-employee is hidden as 404.
func (s *ActivityService) loadReadableRecord(ctx context.Context, attendanceID string) (att.Attendance, error) {
	rec, err := s.attRepo.GetAttendance(ctx, attendanceID)
	if errors.Is(err, domain.ErrNotFound) {
		return att.Attendance{}, apperr.NotFound()
	}
	if err != nil {
		return att.Attendance{}, apperr.Internal(err)
	}
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleAgent {
		if p.EmployeeID != "" && p.EmployeeID == rec.EmployeeID {
			return rec, nil
		}
		return att.Attendance{}, apperr.NotFound()
	}
	if serr := rbac.GuardCompany(ctx, rec.CompanyID); serr != nil {
		return att.Attendance{}, apperr.NotFound()
	}
	return rec, nil
}
