// Package attendance — correction service (F5.4 / ATT-02). List/detail of the
// corrections queue, approve (applies whitelisted proposed fields to the target
// attendance + flips status to APPLIED), and reject (reason → REJECTED). Scope
// (OUT_OF_SCOPE), terminal-state (409 CONFLICT), and the OUTSIDE_CORRECTION_WINDOW
// 7-day guard are enforced here; audit-in-tx + notify stub on every write.
package attendance

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// correctionWindowDays is the OUTSIDE_CORRECTION_WINDOW bound (CR rule): a non-HR
// caller may not act on a correction whose shift date is older than this.
const correctionWindowDays = 7

// lateGraceMinutes is the lateness grace (EPICS §8 / 2026-05-29): a clock-in
// strictly after shift_start + grace is LATE; within grace is on-time.
const lateGraceMinutes = 15

// reevalCheckIn recomputes (status, is_late, late_minutes) for a check-in that
// resolves an absence (BR CR-9). Within grace (≤ shift_start + 15m) → PRESENT,
// not late, 0 minutes. Strictly after → LATE with late_minutes = ceil(delay from
// shift_start) (matches the openapi Attendance.late_minutes + the seed convention).
// Unscheduled (nil shiftStart) stays PRESENT.
func reevalCheckIn(checkIn time.Time, shiftStart *time.Time) (status string, isLate bool, lateMinutes int) {
	if shiftStart == nil {
		return string(att.StatusPresent), false, 0
	}
	graceEnd := shiftStart.Add(lateGraceMinutes * time.Minute)
	if !checkIn.After(graceEnd) {
		return string(att.StatusPresent), false, 0
	}
	delay := checkIn.Sub(*shiftStart)
	mins := int(delay / time.Minute)
	if delay%time.Minute != 0 {
		mins++ // ceil
	}
	return string(att.StatusLate), true, mins
}

// CorrectionService implements the correction-queue business logic.
type CorrectionService struct {
	repo    CorrectionRepository
	attRepo AttendanceRepository
	txm     TxRunner
	now     Clock
	engine  approval.Engine
	// PC-9 post-lock carry-forward: when a correction applies to a LOCKED payroll
	// month, the locked summary is NOT mutated — instead a PayrollAdjustment is
	// emitted to the next open run. Both ports are consumer-defined here (avoid an
	// import cycle with the payroll service) + nil-safe (no F8.5 wiring = no-op).
	locked   LockedMonthChecker
	adjuster AdjustmentEmitter
}

// AdjustmentEmitter emits a PENDING, unvalued PayrollAdjustment for a post-lock
// change (E8 F8.5 / PC-9). Runs INSIDE the caller's tx so the adjustment commits
// atomically with the correction. Defined here (consumer side) to avoid an import
// cycle with the payroll service; satisfied by payroll.ClarificationService.
type AdjustmentEmitter interface {
	EmitCorrectionAdjustment(ctx context.Context, tx pgx.Tx, employeeID, sourceID string, originYear, originMonth int) error
}

// NewCorrectionService wires the correction service. It needs the attendance repo
// to apply approved corrections to the target record in the same tx.
func NewCorrectionService(repo CorrectionRepository, attRepo AttendanceRepository, txm TxRunner) *CorrectionService {
	return &CorrectionService{repo: repo, attRepo: attRepo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *CorrectionService) SetClock(c Clock) { s.now = c }

// SetApprovalEngine injects the E11 engine (corrections route through it on submit;
// the OnApproved/OnRejected hooks fire on the engine's terminal transition).
func (s *CorrectionService) SetApprovalEngine(e approval.Engine) { s.engine = e }

// SetPayrollPeriodGuard wires the F8.5 locked-month guard + adjustment emitter
// (PC-9). Additive + nil-safe: without it, an approved correction never emits an
// adjustment (pre-F8.5 behavior).
func (s *CorrectionService) SetPayrollPeriodGuard(c LockedMonthChecker, a AdjustmentEmitter) {
	s.locked, s.adjuster = c, a
}

// --- list / get ---

// List returns the corrections queue page. Leader scope is forced to their led
// company; a client-supplied company_id outside scope yields 403 OUT_OF_SCOPE.
func (s *CorrectionService) List(ctx context.Context, f CorrectionFilter) ([]att.Correction, *string, bool, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, nil, false, apperr.Unauthenticated()
	}
	if p.Role == auth.RoleShiftLeader {
		if f.CompanyID != nil && *f.CompanyID != p.CompanyID {
			return nil, nil, false, apperr.OutOfScope()
		}
		cid := p.CompanyID
		f.CompanyID = &cid
	}

	limit := clampLimit(f.Limit)
	f.Limit = limit + 1
	rows, err := s.repo.ListCorrections(ctx, f)
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
		c, cerr := encodeCorrectionCursor(last.CreatedAt, last.ID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// Get loads a single correction with a server-rendered diff[]; out-of-scope is
// hidden as 404.
func (s *CorrectionService) Get(ctx context.Context, id string) (att.Correction, error) {
	cor, err := s.repo.GetCorrection(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return att.Correction{}, apperr.NotFound()
	}
	if err != nil {
		return att.Correction{}, apperr.Internal(err)
	}
	if serr := rbac.GuardCompany(ctx, cor.CompanyID); serr != nil {
		return att.Correction{}, apperr.NotFound()
	}
	cor.Diff = buildDiff(cor)
	return cor, nil
}

// --- create (agent / leader file a correction, F5.4) ---

// CreateCorrectionInput is the decoded POST /corrections request (openapi
// CorrectionWriteRequest). Times are RFC3339-parsed at the handler boundary.
type CreateCorrectionInput struct {
	AttendanceID             string
	WorkDate                 *time.Time // required iff Type == NEW_ENTRY
	Type                     string
	ProposedCheckInAt        *time.Time
	ProposedCheckOutAt       *time.Time
	ProposedAttendanceCodeID *string
	Reason                   string
	EvidenceFileID           *string
}

// Create files a new PENDING correction against a target attendance (F5.4).
// Scope: an agent may only correct their own record (cross-record hidden as 404,
// no existence leak); a shift_leader is company-scoped via GuardCompany; HR/super
// are global. Then: the 7-day OUTSIDE_CORRECTION_WINDOW guard (HR-exempt), a
// single-active-PENDING dedupe (409 CORRECTION_ALREADY_PENDING), and per-type
// field validation. Inserts + audits in one tx; re-reads for denormalized names.
func (s *CorrectionService) Create(ctx context.Context, in CreateCorrectionInput) (att.Correction, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return att.Correction{}, apperr.Unauthenticated()
	}

	// Per-type field validation (openapi CorrectionWriteRequest). evidence_file_id is
	// intentionally OPTIONAL for this MVP — see TODO below.
	if verr := validateCorrectionInput(in); verr != nil {
		return att.Correction{}, verr
	}
	// TODO(CLOCK-03): enforce evidence_file_id for CHECK_IN/CHECK_OUT once photo-upload lands.

	isHR := principal.Role == auth.RoleHRAdmin || principal.Role == auth.RoleSuperAdmin
	isNewEntry := att.CorrectionType(in.Type) == att.CorrectionTypeNewEntry

	// companyID drives the E11 template + leader scope; shiftDate is the window basis +
	// the denormalized attendance_shift_date. Both resolved per branch below.
	var companyID string
	var shiftDate time.Time

	if isNewEntry {
		// NEW_ENTRY: no target record. Resolve the requester's active placement on
		// work_date (company/site/position) and ensure no record already exists that day.
		workDate := *in.WorkDate
		af, found, aerr := s.attRepo.GetManualAutofillData(ctx, principal.EmployeeID, workDate)
		if aerr != nil {
			return att.Correction{}, apperr.Internal(aerr)
		}
		if !found {
			return att.Correction{}, apperr.Rule("NO_ACTIVE_PLACEMENT", nil)
		}
		if af.ExistingAttendanceID != nil && *af.ExistingAttendanceID != "" {
			return att.Correction{}, apperr.ConflictWithDetails(
				"ATTENDANCE_ALREADY_EXISTS",
				map[string]string{"attendance_id": *af.ExistingAttendanceID, "work_date": workDate.Format("2006-01-02")},
				nil,
			)
		}
		// Non-agent filers (SL/HR on behalf) are company-scoped; agents file their own.
		if principal.Role != auth.RoleAgent {
			if serr := rbac.GuardCompany(ctx, af.CompanyID); serr != nil {
				return att.Correction{}, serr
			}
		}
		companyID = af.CompanyID
		shiftDate = workDate
	} else {
		rec, err := s.attRepo.GetAttendance(ctx, in.AttendanceID)
		if errors.Is(err, domain.ErrNotFound) {
			return att.Correction{}, apperr.NotFound()
		}
		if err != nil {
			return att.Correction{}, apperr.Internal(err)
		}
		// Scope. Agent: own-record only (else 404, no leak). Leader: company-scoped.
		// HR/super: global (GuardCompany passes them through).
		switch principal.Role {
		case auth.RoleAgent:
			if principal.EmployeeID == "" || principal.EmployeeID != rec.EmployeeID {
				return att.Correction{}, apperr.NotFound()
			}
		default:
			if serr := rbac.GuardCompany(ctx, rec.CompanyID); serr != nil {
				return att.Correction{}, serr // OUT_OF_SCOPE 403
			}
		}
		// One active PENDING correction per attendance (409 with the open correction id).
		if pid, found, perr := s.repo.GetPendingCorrectionForAttendance(ctx, in.AttendanceID); perr != nil {
			return att.Correction{}, apperr.Internal(perr)
		} else if found {
			return att.Correction{}, apperr.ConflictWithDetails(
				"CORRECTION_ALREADY_PENDING",
				map[string]string{"correction_id": pid},
				nil,
			)
		}
		companyID = rec.CompanyID
		switch {
		case rec.ShiftStartAt != nil:
			shiftDate = *rec.ShiftStartAt
		case rec.CheckInAt != nil:
			shiftDate = *rec.CheckInAt
		default:
			shiftDate = s.now()
		}
	}

	// 7-day window (HR/super exempt). Basis: the target shift date for existing records,
	// or work_date for NEW_ENTRY.
	if werr := CheckCorrectionWindow(shiftDate, isHR, s.now()); werr != nil {
		return att.Correction{}, werr
	}

	shiftDateOnly := time.Date(shiftDate.Year(), shiftDate.Month(), shiftDate.Day(), 0, 0, 0, 0, time.UTC)
	var newID string
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		id, cerr := s.repo.CreateCorrection(ctx, tx, CreateCorrectionParams{
			AttendanceID:             in.AttendanceID,
			WorkDate:                 in.WorkDate,
			RequesterID:              principal.EmployeeID,
			CompanyID:                companyID,
			Type:                     in.Type,
			ProposedCheckInAt:        in.ProposedCheckInAt,
			ProposedCheckOutAt:       in.ProposedCheckOutAt,
			ProposedAttendanceCodeID: in.ProposedAttendanceCodeID,
			Reason:                   in.Reason,
			EvidenceFileID:           in.EvidenceFileID,
			AttendanceShiftDate:      shiftDateOnly,
		})
		if cerr != nil {
			return cerr
		}
		newID = id

		// Route the correction through the E11 approval engine (request_type=CORRECTION):
		// open an ApprovalInstance on the same tx and link it. Decisions then happen via
		// POST /approval-instances/{id}:approve|:reject — the OnApproved/OnRejected hooks
		// fire on the terminal transition (mirrors leave/overtime).
		if s.engine == nil {
			return apperr.Rule("RULE_VIOLATION", map[string]string{"approval": "Approval engine tidak aktif."})
		}
		instanceID, ierr := s.engine.CreateInstance(ctx, tx, approval.CreateInstanceInput{
			RequestType: approval.RequestTypeCorrection,
			RequestID:   id,
			CompanyID:   companyID,
			RequesterID: principal.EmployeeID,
		})
		if ierr != nil {
			return ierr
		}
		if lerr := s.repo.SetCorrectionApprovalInstance(ctx, tx, id, instanceID); lerr != nil {
			return lerr
		}

		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "attendance_correction",
			EntityID:   id,
			After: map[string]any{
				"attendance_id":        in.AttendanceID,
				"type":                 in.Type,
				"status":               "PENDING",
				"requester_id":         principal.EmployeeID,
				"approval_instance_id": instanceID,
			},
		})
	})
	if err != nil {
		return att.Correction{}, asAppErr(err)
	}
	// Re-read for the denormalized requester/company names on the DTO.
	return s.GetCorrection(ctx, newID)
}

// GetCorrection re-reads a correction by id for the create path (no scope guard:
// the caller has already passed the create-time scope checks). Diff is rendered
// for consistency with the detail endpoint.
func (s *CorrectionService) GetCorrection(ctx context.Context, id string) (att.Correction, error) {
	cor, err := s.repo.GetCorrection(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return att.Correction{}, apperr.NotFound()
	}
	if err != nil {
		return att.Correction{}, apperr.Internal(err)
	}
	cor.Diff = buildDiff(cor)
	return cor, nil
}

// validateCorrectionInput enforces the per-type required-field rules + reason
// length (openapi CorrectionWriteRequest). evidence_file_id is optional (MVP).
func validateCorrectionInput(in CreateCorrectionInput) error {
	fields := map[string]string{}
	switch att.CorrectionType(in.Type) {
	case att.CorrectionTypeCheckIn:
		if in.ProposedCheckInAt == nil {
			fields["proposed_check_in_at"] = "Wajib diisi untuk koreksi check-in."
		}
	case att.CorrectionTypeCheckOut:
		if in.ProposedCheckOutAt == nil {
			fields["proposed_check_out_at"] = "Wajib diisi untuk koreksi check-out."
		}
	case att.CorrectionTypeCode:
		if in.ProposedAttendanceCodeID == nil || *in.ProposedAttendanceCodeID == "" {
			fields["proposed_attendance_code_id"] = "Wajib diisi untuk koreksi kode kehadiran."
		}
	case att.CorrectionTypeNewEntry:
		if in.WorkDate == nil {
			fields["work_date"] = "Wajib diisi untuk koreksi tanpa catatan (NEW_ENTRY)."
		}
		if in.ProposedCheckInAt == nil {
			fields["proposed_check_in_at"] = "Wajib diisi untuk koreksi tanpa catatan (jam masuk)."
		}
	case att.CorrectionTypeOther:
		// reason-only.
	default:
		fields["type"] = "Tidak valid (CHECK_IN, CHECK_OUT, CODE, OTHER, atau NEW_ENTRY)."
	}
	if len([]rune(in.Reason)) < 5 {
		fields["reason"] = "Wajib diisi (minimum 5 karakter)."
	}
	if len(fields) > 0 {
		return apperr.Invalid(fields)
	}
	return nil
}

// --- approve / reject ---

// OnApproved is the E11 terminal-APPROVED hook for request_type=CORRECTION. It runs
// INSIDE the engine's tx (the approver's auth/line-clearing already happened in the
// engine). Applies the proposed change to the target attendance (ApplyCorrectionToAttendance
// COALESCE whitelist + CORRECTED flag, BR CR-9 re-eval) and flips the correction APPLIED.
// Idempotent: a re-fired hook on an already-APPLIED correction is a no-op.
func (s *CorrectionService) OnApproved(ctx context.Context, tx pgx.Tx, requestID string) error {
	actor := actorEmployeeID(ctx)
	cor, lerr := s.repo.GetCorrectionForUpdate(ctx, tx, requestID)
	if errors.Is(lerr, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if lerr != nil {
		return lerr
	}
	if cor.Status == att.CorrectionStatusApplied {
		return nil // idempotent
	}
	if cor.Status != att.CorrectionStatusPending {
		return correctionTerminalConflict(cor.Status)
	}

	boolTrue := true
	// attachID is the created record's id for a NEW_ENTRY (attached to the correction on
	// APPLIED); nil for an existing-record correction.
	var attachID *string
	// adjEmployeeID / adjSourceID / adjMonth capture the PC-9 carry-forward target:
	// the affected employee + attendance record + its work-date month. adjSourceID is
	// set once the affected record id is known (the new record for NEW_ENTRY, else the
	// existing attendance id). adjMonth defaults to the work date of the change.
	var (
		adjSourceID  string
		adjMonthTime time.Time
	)

	if cor.Type == att.CorrectionTypeNewEntry {
		// NEW_ENTRY: create the attendance record for the no-shift day (CR-10). Resolve the
		// requester's active placement + (possible) schedule for work_date.
		workDate := s.now()
		if cor.WorkDate != nil {
			workDate = *cor.WorkDate
		}
		af, found, aerr := s.attRepo.GetManualAutofillData(ctx, cor.RequesterID, workDate)
		if aerr != nil {
			return aerr
		}
		if !found {
			return apperr.Rule("NO_ACTIVE_PLACEMENT", nil)
		}
		if cor.ProposedCheckInAt == nil {
			return apperr.Invalid(map[string]string{"proposed_check_in_at": "Wajib diisi."})
		}
		hadShift := af.ScheduleID != nil
		var shiftStart, shiftEnd *time.Time
		if hadShift {
			shiftStart, shiftEnd = af.ShiftStartAt, af.ShiftEndAt
		}
		status, isLate, lateMin := reevalCheckIn(*cor.ProposedCheckInAt, shiftStart)
		var workedMin *int
		if cor.ProposedCheckOutAt != nil {
			wm := int(cor.ProposedCheckOutAt.Sub(*cor.ProposedCheckInAt).Minutes())
			if wm < 0 {
				wm = 0
			}
			workedMin = &wm
		}
		// Payable (CR-12): had a scheduled shift → auto-payable; no shift → nil (pending flag).
		var payable *bool
		if hadShift {
			payable = &boolTrue
		}
		created, cerr := s.attRepo.CreateManualAttendance(ctx, tx, CreateManualAttendanceParams{
			EmployeeID:         cor.RequesterID,
			PlacementID:        af.PlacementID,
			ScheduleID:         af.ScheduleID,
			CompanyID:          af.CompanyID,
			SiteID:             af.SiteID,
			Position:           af.Position,
			ShiftStartAt:       shiftStart,
			ShiftEndAt:         shiftEnd,
			CheckInAt:          *cor.ProposedCheckInAt,
			CheckOutAt:         cor.ProposedCheckOutAt,
			WFO:                true,
			IsLate:             isLate,
			LateMinutes:        lateMin,
			WorkedMinutes:      workedMin,
			Status:             status,
			VerificationStatus: string(att.VerificationPending),
			Flags:              []string{string(att.FlagUnscheduled), string(att.FlagCorrected)},
			IsPayable:          payable,
			CreatedBy:          &cor.RequesterID,
		})
		if cerr != nil {
			return cerr
		}
		attachID = &created.ID
		adjSourceID = created.ID
		// NEW_ENTRY belongs to its work_date month (C-PC-2).
		adjMonthTime = workDate
	} else {
		// Existing-record correction: apply whitelisted proposed_* + BR CR-9 re-eval.
		attRec, arerr := s.attRepo.GetAttendanceForUpdate(ctx, tx, cor.AttendanceID)
		if arerr != nil {
			if errors.Is(arerr, domain.ErrNotFound) {
				return apperr.NotFound()
			}
			return arerr
		}
		params := ApplyCorrectionParams{
			ID:               cor.AttendanceID,
			CheckInAt:        cor.ProposedCheckInAt,
			CheckOutAt:       cor.ProposedCheckOutAt,
			AttendanceCodeID: cor.ProposedAttendanceCodeID,
			LastCorrectionID: &cor.ID,
		}
		if cor.ProposedCheckInAt != nil && (attRec.Status == att.StatusAbsent || attRec.CheckInAt == nil) {
			status, isLate, lateMin := reevalCheckIn(*cor.ProposedCheckInAt, attRec.ShiftStartAt)
			params.Status = &status
			params.IsLate = &isLate
			params.LateMinutes = &lateMin
		}
		// Payable (CR-12): a shift-backed day becomes auto-payable on apply; a no-shift day
		// is left pending (nil → COALESCE keeps the existing value).
		if attRec.ScheduleID != nil || attRec.ShiftStartAt != nil {
			params.IsPayable = &boolTrue
		}
		if _, aerr := s.attRepo.ApplyCorrectionToAttendance(ctx, tx, params); aerr != nil {
			if errors.Is(aerr, domain.ErrNotFound) {
				return apperr.NotFound()
			}
			return aerr
		}
		adjSourceID = cor.AttendanceID
		// The existing record belongs to its shift-start month (C-PC-2); fall back to
		// the clock-in instant for an unscheduled record.
		switch {
		case attRec.ShiftStartAt != nil:
			adjMonthTime = attRec.ShiftStartAt.In(jakartaLoc())
		case attRec.CheckInAt != nil:
			adjMonthTime = attRec.CheckInAt.In(jakartaLoc())
		default:
			adjMonthTime = s.now()
		}
	}

	_, n, cerr := s.repo.ApproveCorrection(ctx, tx, requestID, actor, attachID)
	if cerr != nil {
		return cerr
	}
	if n == 0 {
		return correctionTerminalConflict(cor.Status)
	}

	// PC-9: if the change lands in a LOCKED payroll month, the immutable summary is
	// NOT mutated — emit a PENDING PayrollAdjustment (origin = that month) for the
	// next open run instead. Pre-lock changes apply in place (no adjustment, C-PC-5).
	if s.locked != nil && s.adjuster != nil && !adjMonthTime.IsZero() {
		y, m := adjMonthTime.In(jakartaLoc()).Year(), int(adjMonthTime.In(jakartaLoc()).Month())
		locked, lkErr := s.locked.IsMonthLocked(ctx, y, m)
		if lkErr != nil {
			return apperr.Internal(lkErr)
		}
		if locked {
			if emErr := s.adjuster.EmitCorrectionAdjustment(ctx, tx, cor.RequesterID, adjSourceID, y, m); emErr != nil {
				return emErr
			}
		}
	}

	// TODO(Phase-11): enqueue NotificationArgs ("correction approved" + E7/E10
	// recompute listeners) in this tx (PRD F5.4 C-4 downstream propagation).
	if aerr := audit.Record(ctx, tx, audit.Entry{
		Action:     audit.ActionUpdate,
		EntityType: "attendance_correction",
		EntityID:   requestID,
		Before:     map[string]any{"status": string(cor.Status)},
		After:      map[string]any{"status": "APPLIED", "decided_by": ptrStr(actor)},
	}); aerr != nil {
		return aerr
	}
	return audit.Record(ctx, tx, audit.Entry{
		Action:     audit.ActionUpdate,
		EntityType: "attendance",
		EntityID:   cor.AttendanceID,
		After:      map[string]any{"last_correction_id": cor.ID, "applied_correction": cor.ID},
	})
}

// OnRejected is the E11 terminal-REJECTED hook for request_type=CORRECTION. Runs INSIDE
// the engine's tx: flips the correction REJECTED (the user-visible reason lives on the
// E11 approval action, not the correction). Idempotent on an already-REJECTED correction.
func (s *CorrectionService) OnRejected(ctx context.Context, tx pgx.Tx, requestID string) error {
	actor := actorEmployeeID(ctx)
	cor, lerr := s.repo.GetCorrectionForUpdate(ctx, tx, requestID)
	if errors.Is(lerr, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if lerr != nil {
		return lerr
	}
	if cor.Status == att.CorrectionStatusRejected {
		return nil // idempotent
	}
	if cor.Status != att.CorrectionStatusPending {
		return correctionTerminalConflict(cor.Status)
	}
	_, n, rerr := s.repo.RejectCorrection(ctx, tx, requestID, actor, "")
	if rerr != nil {
		return rerr
	}
	if n == 0 {
		return correctionTerminalConflict(cor.Status)
	}
	// TODO(Phase-11): enqueue NotificationArgs ("correction rejected") in this tx.
	return audit.Record(ctx, tx, audit.Entry{
		Action:     audit.ActionUpdate,
		EntityType: "attendance_correction",
		EntityID:   requestID,
		Before:     map[string]any{"status": string(cor.Status)},
		After:      map[string]any{"status": "REJECTED", "decided_by": ptrStr(actor)},
	})
}

// --- window guard (exported for the 07-03 contract test seam) ---

// CheckCorrectionWindow enforces OUTSIDE_CORRECTION_WINDOW: a non-HR caller may
// not act on a correction whose shift date is older than correctionWindowDays.
// HR/super_admin are exempt. Exposed so the 07-03 contract test can drive the
// 422 directly (the correction-CREATE endpoint is out of web scope).
func CheckCorrectionWindow(shiftDate time.Time, isHR bool, now time.Time) error {
	if isHR {
		return nil
	}
	loc := jakartaLoc()
	today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	cutoff := today.AddDate(0, 0, -correctionWindowDays)
	sd := time.Date(shiftDate.Year(), shiftDate.Month(), shiftDate.Day(), 0, 0, 0, 0, loc)
	if sd.Before(cutoff) {
		return &apperr.Error{
			Code:       "OUTSIDE_CORRECTION_WINDOW",
			HTTPStatus: 422,
			Message:    "Koreksi melewati batas waktu yang diizinkan (7 hari).",
			Fields: map[string]string{
				"attendance_date": sd.Format("2006-01-02"),
				"window_days":     "7",
			},
		}
	}
	return nil
}

// --- helpers ---

// buildDiff renders the field-by-field diff between original_snapshot and the
// proposed/applied values (openapi Correction.diff[]). Mirrors the FE
// buildDiffRows: check_in_at, check_out_at, attendance_code_id, plus any
// snapshot-carried derived fields (auto_closed, status).
func buildDiff(cor att.Correction) []att.DiffRow {
	var rows []att.DiffRow
	snap := cor.OriginalSnapshot

	add := func(field string, proposed any) {
		before, had := snap[field]
		if !had {
			before = nil
		}
		rows = append(rows, att.DiffRow{Field: field, Before: before, After: proposed})
	}

	if cor.ProposedCheckInAt != nil {
		add("check_in_at", cor.ProposedCheckInAt.UTC().Format(time.RFC3339))
	}
	if cor.ProposedCheckOutAt != nil {
		add("check_out_at", cor.ProposedCheckOutAt.UTC().Format(time.RFC3339))
	}
	if cor.ProposedAttendanceCodeID != nil {
		add("attendance_code_id", *cor.ProposedAttendanceCodeID)
	}
	// Snapshot-only derived fields (no proposed value, but carried for the UI):
	// surface auto_closed/status from the snapshot so the side-by-side shows the
	// pre-correction state (after is left as the snapshot value when unchanged).
	for _, f := range []string{"auto_closed", "status"} {
		if v, ok := snap[f]; ok {
			rows = append(rows, att.DiffRow{Field: f, Before: v, After: v})
		}
	}
	return rows
}

// correctionTerminalConflict builds the 409 returned when a correction is not PENDING.
func correctionTerminalConflict(cur att.CorrectionStatus) error {
	return &apperr.Error{
		Code:       "CONFLICT",
		HTTPStatus: 409,
		Message:    "Koreksi sudah diputuskan sebelumnya.",
		Fields:     map[string]string{"status": string(cur)},
	}
}

// callerIsHR reports whether the principal is HR/super_admin (window-exempt).
func callerIsHR(ctx context.Context) bool {
	if p, ok := auth.PrincipalFrom(ctx); ok {
		return p.Role == auth.RoleHRAdmin || p.Role == auth.RoleSuperAdmin
	}
	return false
}

func jakartaLoc() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}
