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
	RejectAttendance(ctx context.Context, tx pgx.Tx, id string, rejectedBy *string, reason string) (att.Attendance, int64, error)
	ApplyCorrectionToAttendance(ctx context.Context, tx pgx.Tx, p ApplyCorrectionParams) (att.Attendance, error)
}

// ApplyCorrectionParams carries the whitelisted COALESCE update for apply-on-approve.
// Status/IsLate/LateMinutes are the BR CR-9 re-evaluation outputs (nil = leave as-is);
// they are set only when a CHECK_IN correction resolves an absence.
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
func (s *AttendanceService) Verify(ctx context.Context, id string, note string) (att.Attendance, error) {
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
		updated, n, verr := s.repo.VerifyAttendance(ctx, tx, id, actor)
		if verr != nil {
			return verr
		}
		if n == 0 {
			return terminalConflict(rec.VerificationStatus)
		}
		out = updated
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
		rec, err := s.Verify(ctx, id, note)
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
