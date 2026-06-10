// Package attendance — E5 verification + corrections services (F5.3/F5.4 /
// ATT-01/ATT-02). The web surface is exceptions-only: list/detail, single
// verify/reject, bulk verify/reject (partial success, idempotent at the router),
// and the corrections queue (list/detail/approve/reject). Every write enforces
// company scope (OUT_OF_SCOPE), own-record (VERIFY_OWN_RECORD), terminal-state
// (409 CONFLICT), audits in-tx, and stubs the notification (TODO Phase-11).
//
// Mirrors the Phase-6 scheduling service structure (bulk partial-success, scope)
// and the Phase-5 placement state-machine pattern.
package attendance

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	reportingdom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// --- shared ports ---

// TxRunner runs a closure inside a DB transaction (db.TxManager satisfies it).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock supplies the current time (overridable in tests).
type Clock func() time.Time

// --- attendance repository port ---

// AttendanceFilter is the decoded GET /attendance query (cursor-paged).
type AttendanceFilter struct {
	CompanyID          *string
	EmployeeID         *string
	ServiceLine        *string
	SiteID             *string
	PositionID         *string
	VerificationStatus []string
	Status             []string
	DateFrom           *time.Time
	DateTo             *time.Time
	ExceptionsOnly     bool
	Limit              int
	CursorCheckInAt    *time.Time
	CursorID           *string
}

// CorrectionFilter is the decoded GET /corrections query (cursor-paged).
type CorrectionFilter struct {
	CompanyID       *string
	EmployeeID      *string
	Status          []string
	Type            []string
	DateFrom        *time.Time
	DateTo          *time.Time
	Limit           int
	CursorCreatedAt *time.Time
	CursorID        *string
}

// AttendanceRepository is the data dependency for the attendance service.
type AttendanceRepository interface {
	ListAttendance(ctx context.Context, f AttendanceFilter) ([]att.Attendance, error)
	GetAttendance(ctx context.Context, id string) (att.Attendance, error)
	GetAttendanceForUpdate(ctx context.Context, tx pgx.Tx, id string) (att.Attendance, error)
	VerifyAttendance(ctx context.Context, tx pgx.Tx, id string, verifiedBy *string) (att.Attendance, int64, error)
	VerifyAttendanceWithTimes(ctx context.Context, tx pgx.Tx, id string, checkInAt time.Time, checkOutAt *time.Time, status string, isLate bool, lateMinutes int, verifiedBy *string) (att.Attendance, int64, error)
	RejectAttendance(ctx context.Context, tx pgx.Tx, id string, rejectedBy *string, reason string) (att.Attendance, int64, error)
	ApplyCorrectionToAttendance(ctx context.Context, tx pgx.Tx, p ApplyCorrectionParams) (att.Attendance, error)
		// CreateManualAttendance inserts a manually-created record (F5.6).
		// GetActivePlacement resolves the employee's single active placement for
	// manual attendance (maps to ClockRepo.GetActivePlacement).
	GetActivePlacement(ctx context.Context, employeeID string) (PlacementInfo, bool, error)
	// GetTodaySchedule resolves today's schedule entry for lateness evaluation.
	GetTodaySchedule(ctx context.Context, employeeID string, now time.Time) (string, time.Time, time.Time, bool, error)
	CreateManualAttendance(ctx context.Context, tx pgx.Tx, p CreateManualAttendanceParams) (att.Attendance, error)
	// GetManualAutofillData resolves placement + schedule for manual form (F5.6).
	GetManualAutofillData(ctx context.Context, employeeID string, refDate time.Time) (ManualAutofillData, bool, error)
}

// ApplyCorrectionParams carries the whitelisted COALESCE update for apply-on-approve.
// Status/IsLate/LateMinutes are the BR CR-9 re-evaluation outputs (nil = leave as-is);
// they are set only when a CHECK_IN correction resolves an absence.

// CreateManualAttendanceParams carries the whitelisted fields for a manual
// attendance record (F5.6). Service pre-computes status, is_late, late_minutes,
// worked_minutes, and verification_status before calling the repo.
type CreateManualAttendanceParams struct {
	EmployeeID         string
	PlacementID        string
	ScheduleID         *string
	CompanyID          string
	ServiceLine        string
	SiteID             string
	PositionID         string
	AttendanceCodeID   *string
	ShiftStartAt       *time.Time
	ShiftEndAt         *time.Time
	CheckInAt          time.Time
	CheckOutAt         *time.Time
	Note               string
	WFO                bool
	IsLate             bool
	LateMinutes        int
	WorkedMinutes      *int
	Status             string
	VerificationStatus string
	Flags              []string
	CreatedBy          *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ApplyCorrectionParams struct {
	ID               string
	CheckInAt        *time.Time
	CheckOutAt       *time.Time
	AttendanceCodeID *string
	Status           *string
	IsLate           *bool
	LateMinutes      *int
	LastCorrectionID *string
}

// AttendanceService implements the verification business logic.
type AttendanceService struct {
	repo     AttendanceRepository
	txm      TxRunner
	now      Clock
	notifier jobs.Dispatcher // E10 (11-02): transactional-outbox notify seam (nil-safe in unit tests)
}

// NewAttendanceService wires the attendance service.
func NewAttendanceService(repo AttendanceRepository, txm TxRunner) *AttendanceService {
	return &AttendanceService{repo: repo, txm: txm, now: time.Now}
}

// SetNotifier wires the E10 notification dispatcher (11-02). Additive + nil-safe.
func (s *AttendanceService) SetNotifier(d jobs.Dispatcher) { s.notifier = d }

// SetClock overrides the time source (tests only).
func (s *AttendanceService) SetClock(c Clock) { s.now = c }

// --- list / get ---

// List returns the company × filter attendance page. Leader scope is enforced:
// a shift_leader is forced to their led company; a client-supplied company_id
// outside their scope yields 403 OUT_OF_SCOPE.
func (s *AttendanceService) List(ctx context.Context, f AttendanceFilter) ([]att.Attendance, *string, bool, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, nil, false, apperr.Unauthenticated()
	}
	if p.Role == auth.RoleShiftLeader {
		// Leader: force the company filter to their led company; reject an
		// explicit company_id outside their scope.
		if f.CompanyID != nil && *f.CompanyID != p.CompanyID {
			return nil, nil, false, apperr.OutOfScope()
		}
		cid := p.CompanyID
		f.CompanyID = &cid
	}
	if p.Role == auth.RoleAgent {
		// Agent (mobile, scope:self): force employee_id to the caller; reject an
		// explicit employee_id that is not their own.
		if f.EmployeeID != nil && *f.EmployeeID != p.EmployeeID {
			return nil, nil, false, apperr.OutOfScope()
		}
		eid := p.EmployeeID
		f.EmployeeID = &eid
	}

	limit := clampLimit(f.Limit)
	f.Limit = limit + 1 // fetch one extra to detect has_more
	rows, err := s.repo.ListAttendance(ctx, f)
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		var lastCheckIn time.Time
		if last.CheckInAt != nil {
			lastCheckIn = *last.CheckInAt
		}
		c, cerr := encodeAttendanceCursor(lastCheckIn, last.ID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// Get loads a single record; out-of-scope is hidden as 404 (openapi: no existence leak).
func (s *AttendanceService) Get(ctx context.Context, id string) (att.Attendance, error) {
	rec, err := s.repo.GetAttendance(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return att.Attendance{}, apperr.NotFound()
	}
	if err != nil {
		return att.Attendance{}, apperr.Internal(err)
	}
	// Agent (mobile, scope:self): may read only their own record; anything else is
	// hidden as 404 (no existence leak), matching the cross-scope rule below.
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleAgent {
		if p.EmployeeID != "" && p.EmployeeID == rec.EmployeeID {
			return rec, nil
		}
		return att.Attendance{}, apperr.NotFound()
	}
	if serr := rbac.GuardCompany(ctx, rec.CompanyID); serr != nil {
		// Cross-scope reads return 404 (hide existence) per openapi.
		return att.Attendance{}, apperr.NotFound()
	}
	return rec, nil
}

// --- single verify / reject ---

// Verify approves one exception record. Guards: scope (OUT_OF_SCOPE 403),
// own-record (VERIFY_OWN_RECORD 403 for a leader's own escalated record), and
// terminal-state (409 CONFLICT when zero rows update). Audits in-tx.
// When checkInAt is non-nil the record is also updated with the supplied times
// (ABSENT/INCOMPLETE fill-in path) and status/lateness is recomputed server-side.
func (s *AttendanceService) Verify(ctx context.Context, id string, note string, checkInAtStr, checkOutAtStr *string) (att.Attendance, error) {
	actor := actorEmployeeID(ctx)
	var out att.Attendance
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.repo.GetAttendanceForUpdate(ctx, tx, id)
		if errors.Is(lerr, domain.ErrNotFound) {
			return apperr.NotFound()
		}
		if lerr != nil {
			return lerr
		}
		if serr := s.guardVerifiable(ctx, rec); serr != nil {
			return serr
		}

		if checkInAtStr != nil {
			checkInAt, perr := time.Parse(time.RFC3339, *checkInAtStr)
			if perr != nil {
				return apperr.Invalid(map[string]string{"check_in_at": "Format waktu tidak valid (RFC3339)."})
			}
			var checkOutAt *time.Time
			if checkOutAtStr != nil {
				co, perr := time.Parse(time.RFC3339, *checkOutAtStr)
				if perr != nil {
					return apperr.Invalid(map[string]string{"check_out_at": "Format waktu tidak valid (RFC3339)."})
				}
				checkOutAt = &co
			}

			// Re-evaluate status and lateness (mirrors clock_decision.go rules).
			isLate := false
			lateMinutes := 0
			status := string(att.StatusPresent)
			if rec.ShiftStartAt != nil {
				diff := checkInAt.Sub(*rec.ShiftStartAt)
				if diff > 15*time.Minute { // 15 min grace
					isLate = true
					lateMinutes = int(diff.Minutes())
					status = string(att.StatusLate)
				}
			}

			updated, n, verr := s.repo.VerifyAttendanceWithTimes(ctx, tx, id, checkInAt, checkOutAt, status, isLate, lateMinutes, actor)
			if verr != nil {
				return verr
			}
			if n == 0 {
				return terminalConflict(rec.VerificationStatus)
			}
			out = updated
		} else {
			updated, n, verr := s.repo.VerifyAttendance(ctx, tx, id, actor)
			if verr != nil {
				return verr
			}
			if n == 0 {
				return terminalConflict(rec.VerificationStatus)
			}
			out = updated
		}

		// E10 (11-02): notify the record owner their attendance is verified
		// (transactional outbox). The catalog's attendance kind is
		// ATTENDANCE_VERIFY_NEEDED — there is no dedicated VERIFIED/REJECTED kind in
		// the v1 NotificationKind enum, so we reuse it as the nearest honest
		// attendance kind for the owner-facing verify/reject result (documented
		// choice; see 11-02-SUMMARY).
		if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
			NotifKind:        string(reportingdom.NotifAttendanceVerifyNeeded),
			RecipientID:      rec.EmployeeID,
			Title:            "Kehadiran diverifikasi",
			Body:             "Catatan kehadiran Anda telah diverifikasi.",
			DeepLinkEpic:     "E5",
			DeepLinkEntityID: id,
			DeepLinkPath:     "/attendance/" + id,
			ActorID:          actorUserID(ctx),
			IsCritical:       false,
		}); derr != nil {
			return derr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "attendance",
			EntityID:   id,
			Before:     map[string]any{"verification_status": string(rec.VerificationStatus)},
			After:      map[string]any{"verification_status": "VERIFIED", "verified_by": ptrStr(actor), "note": note},
		})
	})
	if err != nil {
		return att.Attendance{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// Reject rejects one exception record (reason required, user-visible). Same guards.
func (s *AttendanceService) Reject(ctx context.Context, id string, reason string) (att.Attendance, error) {
	if len([]rune(reason)) < 5 {
		return att.Attendance{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."})
	}
	actor := actorEmployeeID(ctx)
	var out att.Attendance
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.repo.GetAttendanceForUpdate(ctx, tx, id)
		if errors.Is(lerr, domain.ErrNotFound) {
			return apperr.NotFound()
		}
		if lerr != nil {
			return lerr
		}
		if serr := s.guardVerifiable(ctx, rec); serr != nil {
			return serr
		}
		updated, n, rerr := s.repo.RejectAttendance(ctx, tx, id, actor, reason)
		if rerr != nil {
			return rerr
		}
		if n == 0 {
			return terminalConflict(rec.VerificationStatus)
		}
		out = updated
		// E10 (11-02): notify the record owner their attendance was rejected. Same
		// kind reuse as verify (no dedicated reject kind in v1; documented).
		if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
			NotifKind:        string(reportingdom.NotifAttendanceVerifyNeeded),
			RecipientID:      rec.EmployeeID,
			Title:            "Kehadiran ditolak",
			Body:             "Catatan kehadiran Anda ditolak: " + reason,
			DeepLinkEpic:     "E5",
			DeepLinkEntityID: id,
			DeepLinkPath:     "/attendance/" + id,
			ActorID:          actorUserID(ctx),
			IsCritical:       false,
		}); derr != nil {
			return derr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "attendance",
			EntityID:   id,
			Before:     map[string]any{"verification_status": string(rec.VerificationStatus)},
			After:      map[string]any{"verification_status": "REJECTED", "rejected_by": ptrStr(actor), "reject_reason": reason},
		})
	})
	if err != nil {
		return att.Attendance{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// guardVerifiable enforces scope + own-record before a verify/reject. The
// own-record rule: a shift_leader cannot decide their own (auto-escalated)
// record → 403 VERIFY_OWN_RECORD. HR/super pass scope globally.
func (s *AttendanceService) guardVerifiable(ctx context.Context, rec att.Attendance) error {
	if serr := rbac.GuardCompany(ctx, rec.CompanyID); serr != nil {
		return serr // OUT_OF_SCOPE 403
	}
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleShiftLeader {
		if p.EmployeeID != "" && p.EmployeeID == rec.EmployeeID {
			return &apperr.Error{
				Code:       "VERIFY_OWN_RECORD",
				HTTPStatus: 403,
				Message:    "Catatan kehadiran Anda sendiri sudah dieskalasi ke HR.",
			}
		}
	}
	return nil
}

// --- bulk verify / reject (per-item partial success) ---

// FailedItem is one failed row in the bulk envelope.
type FailedItem struct {
	ID      string
	Code    string
	Message string
}

// BulkResult is the partial-success aggregate (openapi BulkActionResponse).
type BulkResult struct {
	Succeeded []string
	Failed    []FailedItem
}

// BulkVerify verifies each id via the single Verify path; per-item failures are
// mapped to a Failed row (apperr.As), successes appended to Succeeded. A non-
// apperr (500) aborts the whole batch. Idempotency is the router's concern.
func (s *AttendanceService) BulkVerify(ctx context.Context, ids []string, note string) (BulkResult, error) {
	var out BulkResult
	for _, id := range ids {
		rec, err := s.Verify(ctx, id, note, nil, nil)
		if err == nil {
			out.Succeeded = append(out.Succeeded, rec.ID)
			continue
		}
		if ae, ok := apperr.As(err); ok {
			out.Failed = append(out.Failed, FailedItem{ID: id, Code: ae.Code, Message: bulkMessage(ae)})
			continue
		}
		return BulkResult{}, err
	}
	return out, nil
}

// BulkReject rejects each id with the shared reason. Same partial-success shape.
func (s *AttendanceService) BulkReject(ctx context.Context, ids []string, reason string) (BulkResult, error) {
	var out BulkResult
	for _, id := range ids {
		rec, err := s.Reject(ctx, id, reason)
		if err == nil {
			out.Succeeded = append(out.Succeeded, rec.ID)
			continue
		}
		if ae, ok := apperr.As(err); ok {
			out.Failed = append(out.Failed, FailedItem{ID: id, Code: ae.Code, Message: bulkMessage(ae)})
			continue
		}
		return BulkResult{}, err
	}
	return out, nil
}

// --- ManualAutofill (F5.6) ---

// GetManualAutofillData resolves placement + schedule for the manual form.
func (s *AttendanceService) GetManualAutofillData(ctx context.Context, employeeID string, refDate time.Time) (ManualAutofillData, bool, error) {
	return s.repo.GetManualAutofillData(ctx, employeeID, refDate)
}

// --- ManualCreate (F5.6) ---

// ManualCreateRequest is the decoded body for POST /attendance:manual-create.
type ManualCreateRequest struct {
	EmployeeID       string
	CheckInAt        time.Time
	CheckOutAt       *time.Time
	Note             string
	CreatedBy        string // SWP-EMP-* of the creating HR/admin
}

// ManualAutofillData is the response for GET /attendance:manual-autofill.
type ManualAutofillData struct {
	PlacementID     string
	CompanyID       string
	ServiceLine     string
	SiteID          string
	PositionID      string
	EmployeeName    string
	CompanyName     string
	SiteName        *string
	PositionName    *string
	ScheduleID      *string
	ShiftStartAt    *time.Time
	ShiftEndAt      *time.Time
}

// ManualCreate creates an attendance record for any employee (HR/SL, F5.6).
// Shift leaders can only create attendance for agents within their own company scope.
// Bypasses GPS/geofence. Resolves placement + schedule server-side.
func (s *AttendanceService) ManualCreate(ctx context.Context, req ManualCreateRequest) (att.Attendance, error) {
	now := s.now()

	// Resolve active placement for the target employee.
	pl, found, err := s.repo.GetActivePlacement(ctx, req.EmployeeID)
	if err != nil {
		return att.Attendance{}, apperr.Internal(err)
	}
	if !found {
		return att.Attendance{}, apperr.Rule("NO_ACTIVE_PLACEMENT", nil)
	}

	// Shift leader scope enforcement: SL can only create attendance for employees
	// in their own company.
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleShiftLeader {
		if p.CompanyID == "" || p.CompanyID != pl.CompanyID {
			return att.Attendance{}, apperr.Rule("OUT_OF_SCOPE", nil)
		}
	}
	
	// Validate check_out >= check_in.
	if req.CheckOutAt != nil && req.CheckOutAt.Before(req.CheckInAt) {
		return att.Attendance{}, apperr.Invalid(map[string]string{"check_out_at": "Jam keluar harus setelah jam masuk."})
	}
	
	// Resolve schedule for the employee on check-in date.
	schedID, shiftStart, shiftEnd, schedFound, serr := s.repo.GetTodaySchedule(ctx, req.EmployeeID, now)
	if serr != nil {
		return att.Attendance{}, apperr.Internal(serr)
	}
	
	var (
		flags         []string
		isLate        bool
		lateMinutes   int
		status        = string(att.StatusPresent)
		schedulePtr   *string
		shiftStartPtr *time.Time
		shiftEndPtr   *time.Time
		workedMin     *int
	)
	
	// Always set MANUAL_ENTRY flag.
	flags = append(flags, string(att.FlagManualEntry))
	
	if schedFound {
		ss := shiftStart
		se := shiftEnd
		schedulePtr = &schedID
		shiftStartPtr = &ss
		shiftEndPtr = &se
		
		// Lateness evaluation (same logic as clock_service.go).
		if diff := req.CheckInAt.Sub(shiftStart); diff > 15*time.Minute {
			isLate = true
			lateMinutes = int(diff.Minutes())
			flags = append(flags, string(att.FlagLate))
			status = string(att.StatusLate)
		}
		
		// Early checkout evaluation.
		if req.CheckOutAt != nil && shiftEnd.After(*req.CheckOutAt) {
			if diff := shiftEnd.Sub(*req.CheckOutAt); diff > 15*time.Minute {
				flags = append(flags, string(att.FlagEarly))
			}
		}
	}
	
	// Compute worked_minutes if check_out is provided.
	if req.CheckOutAt != nil {
		wm := int(req.CheckOutAt.Sub(req.CheckInAt).Minutes())
		if wm < 0 {
			wm = 0
		}
		workedMin = &wm
	}
	
	verification := string(att.VerificationPending)
	
	p := CreateManualAttendanceParams{
		EmployeeID:         req.EmployeeID,
		PlacementID:        pl.PlacementID,
		ScheduleID:         schedulePtr,
		CompanyID:          pl.CompanyID,
		ServiceLine:        pl.ServiceLine,
		SiteID:             pl.SiteID,
		PositionID:         pl.PositionID,
		ShiftStartAt:       shiftStartPtr,
		ShiftEndAt:         shiftEndPtr,
		CheckInAt:          req.CheckInAt,
		CheckOutAt:         req.CheckOutAt,
		Note:               req.Note,
		WFO:                true,
		IsLate:             isLate,
		LateMinutes:        lateMinutes,
		WorkedMinutes:      workedMin,
		Status:             status,
		VerificationStatus: verification,
		Flags:              flags,
		CreatedBy:          &req.CreatedBy,
	}
	
	var out att.Attendance
	txErr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, cerr := s.repo.CreateManualAttendance(ctx, tx, p)
		if cerr != nil {
			return cerr
		}
		out = rec
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "attendance",
			EntityID:   rec.ID,
			Before:     nil,
			After: map[string]any{
				"employee_id":         req.EmployeeID,
				"schedule_id":         ptrStr(schedulePtr),
				"check_in_at":         req.CheckInAt,
				"check_out_at":        req.CheckOutAt,
				"status":              status,
				"verification_status": verification,
				"flags":               flags,
				"source":              "manual_entry",
			},
		})
	})
	if txErr != nil {
		return att.Attendance{}, asAppErr(txErr)
	}
	return s.reread(ctx, out)
}

// --- shared helpers ---

// reread re-loads the record for denormalized names on the DTO; falls back to
// the post-write row when the read fails.
func (s *AttendanceService) reread(ctx context.Context, fallback att.Attendance) (att.Attendance, error) {
	if full, err := s.repo.GetAttendance(ctx, fallback.ID); err == nil {
		return full, nil
	}
	return fallback, nil
}

// terminalConflict builds the 409 returned when a verify/reject updates zero rows
// (already VERIFIED/REJECTED). Carries the current verification_status as a field.
func terminalConflict(cur att.VerificationStatus) error {
	return &apperr.Error{
		Code:       "CONFLICT",
		HTTPStatus: 409,
		Message:    "Catatan sudah diputuskan sebelumnya.",
		Fields:     map[string]string{"verification_status": string(cur)},
	}
}

// bulkMessage returns the Bahasa message for a failed bulk row, preferring the
// apperr's own message when set.
func bulkMessage(ae *apperr.Error) string {
	if ae.Message != "" {
		return ae.Message
	}
	switch ae.Code {
	case "VERIFY_OWN_RECORD":
		return "Catatan milik leader sendiri — sudah dieskalasi ke HR."
	case "OUT_OF_SCOPE":
		return "Catatan berada di luar perusahaan binaan Anda."
	case "CONFLICT":
		return "Catatan sudah diputuskan sebelumnya."
	case "NOT_FOUND":
		return "Catatan tidak ditemukan."
	default:
		return "Tindakan gagal untuk catatan ini."
	}
}

// actorEmployeeID resolves the acting employee id from the principal (nil if absent).
func actorEmployeeID(ctx context.Context) *string {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.EmployeeID != "" {
		id := p.EmployeeID
		return &id
	}
	return nil
}

// actorUserID resolves the acting user id (empty if absent) — the notification
// actor (who verified/rejected).
func actorUserID(ctx context.Context) string {
	if p, ok := auth.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return ""
}

func ptrStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// clampLimit applies the documented default(50)/max(200).
func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return 50
	case limit > 200:
		return 200
	default:
		return limit
	}
}

// asAppErr passes *apperr.Error through, wrapping anything else as 500.
func asAppErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.Internal(err)
}
