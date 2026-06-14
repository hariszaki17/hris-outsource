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
}

// NewCorrectionService wires the correction service. It needs the attendance repo
// to apply approved corrections to the target record in the same tx.
func NewCorrectionService(repo CorrectionRepository, attRepo AttendanceRepository, txm TxRunner) *CorrectionService {
	return &CorrectionService{repo: repo, attRepo: attRepo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *CorrectionService) SetClock(c Clock) { s.now = c }

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

	// 7-day window (HR/super exempt). Basis: the target shift date, falling back to
	// the clock-in, then now() for an unscheduled record.
	isHR := principal.Role == auth.RoleHRAdmin || principal.Role == auth.RoleSuperAdmin
	var shiftDate time.Time
	switch {
	case rec.ShiftStartAt != nil:
		shiftDate = *rec.ShiftStartAt
	case rec.CheckInAt != nil:
		shiftDate = *rec.CheckInAt
	default:
		shiftDate = s.now()
	}
	if werr := CheckCorrectionWindow(shiftDate, isHR, s.now()); werr != nil {
		return att.Correction{}, werr
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

	// Per-type field validation (openapi CorrectionWriteRequest). evidence_file_id is
	// intentionally OPTIONAL for this MVP — see TODO below.
	if verr := validateCorrectionInput(in); verr != nil {
		return att.Correction{}, verr
	}
	// TODO(CLOCK-03): enforce evidence_file_id for CHECK_IN/CHECK_OUT once photo-upload lands.

	shiftDateOnly := time.Date(shiftDate.Year(), shiftDate.Month(), shiftDate.Day(), 0, 0, 0, 0, time.UTC)
	var newID string
	err = s.txm.InTx(ctx, func(tx pgx.Tx) error {
		id, cerr := s.repo.CreateCorrection(ctx, tx, CreateCorrectionParams{
			AttendanceID:             in.AttendanceID,
			RequesterID:              principal.EmployeeID,
			CompanyID:                rec.CompanyID,
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
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "attendance_correction",
			EntityID:   id,
			After: map[string]any{
				"attendance_id": in.AttendanceID,
				"type":          in.Type,
				"status":        "PENDING",
				"requester_id":  principal.EmployeeID,
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
	default:
		fields["type"] = "Tidak valid (CHECK_IN, CHECK_OUT, atau CODE)."
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

// Approve applies a PENDING correction: scope guard (OUT_OF_SCOPE), terminal
// guard (409 if !PENDING), window guard (OUTSIDE_CORRECTION_WINDOW, HR-exempt),
// then ApplyCorrectionToAttendance (COALESCE whitelist + CORRECTED flag) and
// ApproveCorrection (status→APPLIED). Returns the updated correction + attendance.
func (s *CorrectionService) Approve(ctx context.Context, id string, note string) (att.Correction, att.Attendance, error) {
	actor := actorEmployeeID(ctx)
	isHR := callerIsHR(ctx)
	var outCor att.Correction
	var outAtt att.Attendance
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cor, lerr := s.repo.GetCorrectionForUpdate(ctx, tx, id)
		if errors.Is(lerr, domain.ErrNotFound) {
			return apperr.NotFound()
		}
		if lerr != nil {
			return lerr
		}
		if serr := rbac.GuardCompany(ctx, cor.CompanyID); serr != nil {
			return serr // OUT_OF_SCOPE 403
		}
		if cor.Status != att.CorrectionStatusPending {
			return correctionTerminalConflict(cor.Status)
		}
		if werr := CheckCorrectionWindow(cor.AttendanceShiftDate, isHR, s.now()); werr != nil {
			return werr
		}

		// Read the target attendance (locked) for the BR CR-9 re-eval inputs:
		// shift_start_at + the pre-correction status / check_in.
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
		// BR CR-9: a CHECK_IN correction that resolves an absence (record was ABSENT
		// or had no clock-in) re-evaluates status/is_late/late_minutes from
		// shift_start_at + the 15-min grace. Recomputed in Go and applied in the same tx.
		if cor.ProposedCheckInAt != nil && (attRec.Status == att.StatusAbsent || attRec.CheckInAt == nil) {
			status, isLate, lateMin := reevalCheckIn(*cor.ProposedCheckInAt, attRec.ShiftStartAt)
			params.Status = &status
			params.IsLate = &isLate
			params.LateMinutes = &lateMin
		}

		applied, aerr := s.attRepo.ApplyCorrectionToAttendance(ctx, tx, params)
		if aerr != nil {
			if errors.Is(aerr, domain.ErrNotFound) {
				return apperr.NotFound()
			}
			return aerr
		}
		outAtt = applied

		updatedCor, n, cerr := s.repo.ApproveCorrection(ctx, tx, id, actor)
		if cerr != nil {
			return cerr
		}
		if n == 0 {
			return correctionTerminalConflict(cor.Status)
		}
		outCor = updatedCor

		// TODO(Phase-11): enqueue NotificationArgs ("correction approved" + E7/E10
		// recompute listeners) in this tx (PRD F5.4 C-4 downstream propagation).
		if aerr := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "attendance_correction",
			EntityID:   id,
			Before:     map[string]any{"status": string(cor.Status)},
			After:      map[string]any{"status": "APPLIED", "decided_by": ptrStr(actor), "note": note},
		}); aerr != nil {
			return aerr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "attendance",
			EntityID:   cor.AttendanceID,
			After:      map[string]any{"last_correction_id": cor.ID, "applied_correction": cor.ID},
		})
	})
	if err != nil {
		return att.Correction{}, att.Attendance{}, asAppErr(err)
	}
	// Re-read both for denormalized names on the DTO.
	if full, gerr := s.repo.GetCorrection(ctx, outCor.ID); gerr == nil {
		full.Diff = buildDiff(full)
		outCor = full
	}
	if fullAtt, gerr := s.attRepo.GetAttendance(ctx, outAtt.ID); gerr == nil {
		outAtt = fullAtt
	}
	return outCor, outAtt, nil
}

// Reject rejects a PENDING correction (reason required, user-visible). Scope +
// terminal guards; audit + notify stub.
func (s *CorrectionService) Reject(ctx context.Context, id string, reason string) (att.Correction, error) {
	if len([]rune(reason)) < 5 {
		return att.Correction{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."})
	}
	actor := actorEmployeeID(ctx)
	var out att.Correction
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cor, lerr := s.repo.GetCorrectionForUpdate(ctx, tx, id)
		if errors.Is(lerr, domain.ErrNotFound) {
			return apperr.NotFound()
		}
		if lerr != nil {
			return lerr
		}
		if serr := rbac.GuardCompany(ctx, cor.CompanyID); serr != nil {
			return serr
		}
		if cor.Status != att.CorrectionStatusPending {
			return correctionTerminalConflict(cor.Status)
		}
		updated, n, rerr := s.repo.RejectCorrection(ctx, tx, id, actor, reason)
		if rerr != nil {
			return rerr
		}
		if n == 0 {
			return correctionTerminalConflict(cor.Status)
		}
		out = updated
		// TODO(Phase-11): enqueue NotificationArgs ("correction rejected") in this tx.
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "attendance_correction",
			EntityID:   id,
			Before:     map[string]any{"status": string(cor.Status)},
			After:      map[string]any{"status": "REJECTED", "decided_by": ptrStr(actor), "reject_reason": reason},
		})
	})
	if err != nil {
		return att.Correction{}, asAppErr(err)
	}
	if full, gerr := s.repo.GetCorrection(ctx, out.ID); gerr == nil {
		full.Diff = buildDiff(full)
		return full, nil
	}
	return out, nil
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
