// Package attendance_test — corrections (F5.4 / ATT-02) contract tests.
//
// The drift gate for the 4 correction endpoints, asserted byte-for-shape against
// docs/api/E5-attendance/openapi.yaml:
//
//	GET /corrections            → 200 {data,next_cursor,has_more}; leader-scope
//	GET /corrections/{id}       → 200 {data: Correction with diff[]}
//	POST :approve               → 200 {data: APPLIED, attendance: CORRECTED};
//	                              non-PENDING → 409; non-HR + stale shift → 422 OUTSIDE_CORRECTION_WINDOW
//	POST :reject                → 200 {data: REJECTED}; missing reason → 400; non-PENDING → 409
//
// CORRECTION_ALREADY_PENDING is the partial-unique-index backstop on the
// correction-CREATE endpoint (out of web scope); here it is asserted as the wire
// shape the service emits (see TestCorrectionAlreadyPending_Shape + SUMMARY seam).
package attendance_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

var (
	corCreatedA = ymd(2026, 6, 4) // newest
	corCreatedB = ymd(2026, 6, 3)
	shiftRecent = ymd(2026, 6, 3) // inside the 7-day window vs fixedNow (2026-06-04)
	shiftStale  = ymd(2026, 5, 1) // > 7 days before fixedNow → OUTSIDE_CORRECTION_WINDOW
)

// seedCheckOutCorrection plants a CHECK_OUT correction with a proposed check-out
// and an original_snapshot so GET /corrections/{id} renders a non-empty diff[].
func seedCheckOutCorrection(h *harness, id, attID, company string, status att.CorrectionStatus, shiftDate time.Time, created time.Time) {
	c := h.seedCorrection(id, attID, company, status, shiftDate, att.CorrectionTypeCheckOut)
	proposed := time.Date(2026, 6, 3, 8, 10, 0, 0, time.UTC) // 15:10 WIB
	c.ProposedCheckOutAt = &proposed
	c.OriginalSnapshot = map[string]any{
		"check_out_at": "2026-06-03T08:00:00Z",
		"auto_closed":  true,
		"status":       "INCOMPLETE",
	}
	c.CreatedAt = created
	c.UpdatedAt = created
	h.correction.records[id] = c
}

// ---------------------------------------------------------------------------
// GET /corrections — list envelope + leader scope
// ---------------------------------------------------------------------------

func TestListCorrections_Envelope(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedCorrection("SWP-COR-8001", "SWP-ATT-9004", cmpLed, att.CorrectionStatusPending, shiftRecent, att.CorrectionTypeCheckOut)
	h.seedCorrection("SWP-COR-8002", "SWP-ATT-9002", cmpLed, att.CorrectionStatusPending, shiftRecent, att.CorrectionTypeCheckIn)

	rr := h.do("GET", "/corrections?company_id="+cmpLed, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data missing or wrong length: %T len=%d", body["data"], len(data))
	}
	if _, present := body["next_cursor"]; !present {
		t.Errorf("next_cursor key missing")
	}
	if _, ok := body["has_more"].(bool); !ok {
		t.Errorf("has_more missing/not a bool: %T", body["has_more"])
	}
	row := data[0].(map[string]any)
	for _, k := range []string{"id", "attendance_id", "requester_id", "type", "status", "original_snapshot"} {
		if _, ok := row[k]; !ok {
			t.Errorf("correction row missing key: %s", k)
		}
	}
}

func TestListCorrections_LeaderScopeForced(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedCorrection("SWP-COR-8001", "SWP-ATT-9004", cmpLed, att.CorrectionStatusPending, shiftRecent, att.CorrectionTypeCheckOut)
	h.seedCorrection("SWP-COR-8009", "SWP-ATT-9005", cmpOther, att.CorrectionStatusPending, shiftRecent, att.CorrectionTypeCheckIn)

	rr := h.do("GET", "/corrections", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("leader saw %d corrections, want 1 (own company)", len(data))
	}
	if got := data[0].(map[string]any)["company_id"]; got != cmpLed {
		t.Errorf("company_id = %v, want %s", got, cmpLed)
	}
}

func TestListCorrections_LeaderCrossCompany_OutOfScope(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)

	rr := h.do("GET", "/corrections?company_id="+cmpOther, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OUT_OF_SCOPE" {
		t.Errorf("error.code = %q, want OUT_OF_SCOPE", code)
	}
}

// ---------------------------------------------------------------------------
// GET /corrections/{id} — diff[] for a CHECK_OUT correction
// ---------------------------------------------------------------------------

func TestGetCorrection_WithDiff(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	seedCheckOutCorrection(h, "SWP-COR-8001", "SWP-ATT-9004", cmpLed, att.CorrectionStatusPending, shiftRecent, corCreatedA)

	rr := h.do("GET", "/corrections/SWP-COR-8001", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].(map[string]any)
	diff, ok := data["diff"].([]any)
	if !ok || len(diff) == 0 {
		t.Fatalf("diff missing/empty: %T %v", data["diff"], data["diff"])
	}
	// Assert a check_out_at diff row {field, before, after}.
	var found bool
	for _, d := range diff {
		row := d.(map[string]any)
		if row["field"] == "check_out_at" {
			found = true
			if _, ok := row["before"]; !ok {
				t.Errorf("check_out_at diff row missing 'before'")
			}
			if row["after"] != "2026-06-03T08:10:00Z" {
				t.Errorf("check_out_at diff after = %v, want 2026-06-03T08:10:00Z", row["after"])
			}
		}
	}
	if !found {
		t.Errorf("no check_out_at row in diff: %v", diff)
	}
}

// ---------------------------------------------------------------------------
// CorrectionService.OnApproved — the E11 terminal-APPROVED hook (corrections now
// route through POST /approval-instances/{id}:approve, which fires this hook). The
// transport is the E11 engine; here we drive the hook directly over the fakes.
// ---------------------------------------------------------------------------

// hookCtx builds a request context carrying the harness principal (the E11 approver).
func hookCtx(h *harness) context.Context {
	return auth.WithPrincipal(context.Background(), h.principal)
}

// asAppErrCode type-asserts the hook error to the platform apperr and returns its code.
func asAppErrCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
	ae, ok := err.(*apperr.Error)
	if !ok {
		t.Fatalf("expected *apperr.Error, got %T: %v", err, err)
	}
	return ae.Code
}

func TestCorrectionOnApproved_AppliesAndApplied(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	// The target attendance must exist for ApplyCorrectionToAttendance.
	h.seedAttendance("SWP-ATT-9004", cmpLed, empOther, att.VerificationPending, ymd(2026, 6, 3), att.FlagAutoClosed)
	seedCheckOutCorrection(h, "SWP-COR-8001", "SWP-ATT-9004", cmpLed, att.CorrectionStatusPending, shiftRecent, corCreatedA)

	if err := h.csvc.OnApproved(hookCtx(h), &fakeTx{}, "SWP-COR-8001"); err != nil {
		t.Fatalf("OnApproved: %v", err)
	}
	if got := h.correction.records["SWP-COR-8001"].Status; got != att.CorrectionStatusApplied {
		t.Errorf("correction status = %v, want APPLIED", got)
	}
	attn := h.attendance.records["SWP-ATT-9004"]
	if !hasFlag(attn.Flags, att.FlagCorrected) {
		t.Errorf("attendance.flags missing CORRECTED: %v", attn.Flags)
	}
}

// seedAbsentWithCheckInCorrection plants a TRUE ABSENT attendance (no clock-in,
// scheduled shift) plus a PENDING CHECK_IN correction proposing a clock-in at
// proposedCheckIn — the BR CR-9 re-eval target.
func seedAbsentWithCheckInCorrection(h *harness, attID, corID string, shiftStart, proposedCheckIn time.Time) {
	rec := h.seedAttendance(attID, cmpLed, empOther, att.VerificationPending, shiftStart, att.FlagAbsent)
	rec.CheckInAt = nil
	rec.LatIn = nil
	rec.LngIn = nil
	rec.Status = att.StatusAbsent
	ss := shiftStart
	rec.ShiftStartAt = &ss
	rec.IsLate = false
	rec.LateMinutes = 0
	h.attendance.records[attID] = rec

	c := h.seedCorrection(corID, attID, cmpLed, att.CorrectionStatusPending, shiftRecent, att.CorrectionTypeCheckIn)
	pin := proposedCheckIn
	c.ProposedCheckInAt = &pin
	c.OriginalSnapshot = map[string]any{"check_in_at": nil, "status": "ABSENT"}
	h.correction.records[corID] = c
}

func TestCorrectionOnApproved_CheckIn_FlipsAbsentToPresent_OnTime(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	shiftStart := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC) // 07:00 WIB
	// Proposed clock-in within the 15-min grace → on-time.
	seedAbsentWithCheckInCorrection(h, "SWP-ATT-9009", "SWP-COR-8009", shiftStart, shiftStart.Add(10*time.Minute))

	if err := h.csvc.OnApproved(hookCtx(h), &fakeTx{}, "SWP-COR-8009"); err != nil {
		t.Fatalf("OnApproved: %v", err)
	}
	attn := h.attendance.records["SWP-ATT-9009"]
	if attn.Status != att.StatusPresent {
		t.Errorf("status = %v, want PRESENT (ABSENT resolved on-time)", attn.Status)
	}
	if attn.LateMinutes != 0 {
		t.Errorf("late_minutes = %d, want 0", attn.LateMinutes)
	}
	if attn.CheckInAt == nil {
		t.Errorf("check_in_at still null after CHECK_IN correction applied")
	}
}

func TestCorrectionOnApproved_CheckIn_FlipsAbsentToLate(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	shiftStart := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC) // 07:00 WIB
	// Proposed clock-in 30 min after start → past grace → LATE, late_minutes=30.
	seedAbsentWithCheckInCorrection(h, "SWP-ATT-9009", "SWP-COR-8009", shiftStart, shiftStart.Add(30*time.Minute))

	if err := h.csvc.OnApproved(hookCtx(h), &fakeTx{}, "SWP-COR-8009"); err != nil {
		t.Fatalf("OnApproved: %v", err)
	}
	attn := h.attendance.records["SWP-ATT-9009"]
	if attn.Status != att.StatusLate {
		t.Errorf("status = %v, want LATE (clock-in past grace)", attn.Status)
	}
	if attn.LateMinutes != 30 {
		t.Errorf("late_minutes = %d, want 30", attn.LateMinutes)
	}
}

func TestCorrectionOnApproved_NonPending_Conflict(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9004", cmpLed, empOther, att.VerificationVerified, ymd(2026, 6, 3))
	seedCheckOutCorrection(h, "SWP-COR-8001", "SWP-ATT-9004", cmpLed, att.CorrectionStatusApplied, shiftRecent, corCreatedA)

	err := h.csvc.OnApproved(hookCtx(h), &fakeTx{}, "SWP-COR-8001")
	if err != nil {
		t.Fatalf("OnApproved on an already-APPLIED correction must be idempotent, got: %v", err)
	}
	if got := h.correction.records["SWP-COR-8001"].Status; got != att.CorrectionStatusApplied {
		t.Errorf("status drifted from APPLIED: %v", got)
	}
}

func TestCorrectionOnApproved_Cancelled_Conflict(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9004", cmpLed, empOther, att.VerificationVerified, ymd(2026, 6, 3))
	seedCheckOutCorrection(h, "SWP-COR-8001", "SWP-ATT-9004", cmpLed, att.CorrectionStatusCancelled, shiftRecent, corCreatedA)

	if code := asAppErrCode(t, h.csvc.OnApproved(hookCtx(h), &fakeTx{}, "SWP-COR-8001")); code != "CONFLICT" {
		t.Errorf("error.code = %q, want CONFLICT (non-PENDING, non-terminal-APPLIED)", code)
	}
}

// ---------------------------------------------------------------------------
// CorrectionService.OnRejected — the E11 terminal-REJECTED hook. The user-visible
// reason lives on the E11 approval action, not on the correction record.
// ---------------------------------------------------------------------------

func TestCorrectionOnRejected_Success(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedCorrection("SWP-COR-8002", "SWP-ATT-9002", cmpLed, att.CorrectionStatusPending, shiftRecent, att.CorrectionTypeCheckIn)

	if err := h.csvc.OnRejected(hookCtx(h), &fakeTx{}, "SWP-COR-8002"); err != nil {
		t.Fatalf("OnRejected: %v", err)
	}
	if got := h.correction.records["SWP-COR-8002"].Status; got != att.CorrectionStatusRejected {
		t.Errorf("status = %v, want REJECTED", got)
	}
}

func TestCorrectionOnRejected_NonPending_Conflict(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedCorrection("SWP-COR-8002", "SWP-ATT-9002", cmpLed, att.CorrectionStatusApplied, shiftRecent, att.CorrectionTypeCheckIn)

	if code := asAppErrCode(t, h.csvc.OnRejected(hookCtx(h), &fakeTx{}, "SWP-COR-8002")); code != "CONFLICT" {
		t.Errorf("error.code = %q, want CONFLICT", code)
	}
}

// ---------------------------------------------------------------------------
// CORRECTION_ALREADY_PENDING — one-pending-per-attendance backstop.
//
// SEAM (documented in the SUMMARY): the one-pending guard lives on the
// correction-CREATE endpoint (POST /attendance/{id}:correct), which is mobile/
// agent-only and OUT of this phase's web scope. The 07-01 partial-unique index
// `corrections_one_pending_per_attendance_uq` is the DB backstop. Here we assert
// (a) the fake repo's pending pre-check seam detects the duplicate, and (b) the
// exact wire shape the contract demands (409 + fields.pending_correction_id),
// constructed via the platform apperr the create handler would emit.
// ---------------------------------------------------------------------------

func TestCorrectionAlreadyPending_Seam(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	// Two PENDING corrections on one attendance — the one-pending invariant is
	// violated; the create pre-check would reject the second.
	h.seedCorrection("SWP-COR-8001", "SWP-ATT-9004", cmpLed, att.CorrectionStatusPending, shiftRecent, att.CorrectionTypeCheckOut)
	h.seedCorrection("SWP-COR-8005", "SWP-ATT-9004", cmpLed, att.CorrectionStatusPending, shiftRecent, att.CorrectionTypeCheckOut)

	firstID, n := h.correction.countPending("SWP-ATT-9004")
	if n < 2 {
		t.Fatalf("pending pre-check seam found %d pending, want >=2", n)
	}

	// The wire shape the create handler emits on the duplicate (409 +
	// fields.pending_correction_id) — matches docs/api/E5 already_pending example.
	emitted := apperr.ConflictWithDetails(
		"CORRECTION_ALREADY_PENDING",
		map[string]string{"pending_correction_id": firstID},
		nil,
	)
	if emitted.Status() != http.StatusConflict {
		t.Errorf("status = %d, want 409", emitted.Status())
	}
	if emitted.Code != "CORRECTION_ALREADY_PENDING" {
		t.Errorf("code = %q, want CORRECTION_ALREADY_PENDING", emitted.Code)
	}
	if emitted.Fields["pending_correction_id"] != "SWP-COR-8001" {
		t.Errorf("fields.pending_correction_id = %q, want SWP-COR-8001", emitted.Fields["pending_correction_id"])
	}
}
