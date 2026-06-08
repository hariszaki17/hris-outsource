// Package attendance_test — POST /corrections (F5.4) contract tests: the agent
// CREATE path plus scope, the 7-day OUTSIDE_CORRECTION_WINDOW guard, the single-
// active-PENDING dedupe (409 CORRECTION_ALREADY_PENDING), and per-type validation.
package attendance_test

import (
	"net/http"
	"testing"
	"time"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// withinWindow is a shift date inside the 7-day window relative to fixedNow.
func withinWindow() time.Time { return fixedNow.Add(-24 * time.Hour) }

func TestCreateCorrection_AgentOwnRecord_201(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", "SWP-EMP-1042")
	h.seedAttendance("SWP-ATT-1", "SWP-CMP-1", "SWP-EMP-1042", att.VerificationPending, withinWindow())

	rr := h.do(http.MethodPost, "/corrections", map[string]any{
		"attendance_id":         "SWP-ATT-1",
		"type":                  "CHECK_OUT",
		"proposed_check_out_at": withinWindow().Add(8 * time.Hour).UTC().Format(time.RFC3339),
		"reason":                "Lupa clock-out, sudah pulang.",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, n := h.correction.countPending("SWP-ATT-1"); n != 1 {
		t.Fatalf("want 1 pending correction, got %d", n)
	}
}

func TestCreateCorrection_AgentOtherRecord_404(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", "SWP-EMP-1042")
	// Record belongs to a different employee → hidden as 404 (no existence leak).
	h.seedAttendance("SWP-ATT-2", "SWP-CMP-1", "SWP-EMP-9999", att.VerificationPending, withinWindow())

	rr := h.do(http.MethodPost, "/corrections", map[string]any{
		"attendance_id":         "SWP-ATT-2",
		"type":                  "CHECK_OUT",
		"proposed_check_out_at": withinWindow().Add(8 * time.Hour).UTC().Format(time.RFC3339),
		"reason":                "Bukan record saya.",
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateCorrection_OutsideWindow_422(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", "SWP-EMP-1042")
	old := fixedNow.AddDate(0, 0, -10) // older than the 7-day window
	h.seedAttendance("SWP-ATT-3", "SWP-CMP-1", "SWP-EMP-1042", att.VerificationPending, old)

	rr := h.do(http.MethodPost, "/corrections", map[string]any{
		"attendance_id":         "SWP-ATT-3",
		"type":                  "CHECK_OUT",
		"proposed_check_out_at": old.Add(8 * time.Hour).UTC().Format(time.RFC3339),
		"reason":                "Telat lapor.",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateCorrection_AlreadyPending_409(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", "SWP-EMP-1042")
	h.seedAttendance("SWP-ATT-4", "SWP-CMP-1", "SWP-EMP-1042", att.VerificationPending, withinWindow())
	h.seedCorrection("SWP-COR-PRE", "SWP-ATT-4", "SWP-CMP-1", att.CorrectionStatusPending, withinWindow(), att.CorrectionTypeCheckOut)

	rr := h.do(http.MethodPost, "/corrections", map[string]any{
		"attendance_id":         "SWP-ATT-4",
		"type":                  "CHECK_OUT",
		"proposed_check_out_at": withinWindow().Add(8 * time.Hour).UTC().Format(time.RFC3339),
		"reason":                "Koreksi kedua.",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "CORRECTION_ALREADY_PENDING" {
		t.Fatalf("want code CORRECTION_ALREADY_PENDING, got %q (%s)", got, rr.Body.String())
	}
}

func TestCreateCorrection_MissingTypeField_400(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", "SWP-EMP-1042")
	h.seedAttendance("SWP-ATT-5", "SWP-CMP-1", "SWP-EMP-1042", att.VerificationPending, withinWindow())

	// CHECK_IN requires proposed_check_in_at — omitted → 400 INVALID_REQUEST.
	rr := h.do(http.MethodPost, "/corrections", map[string]any{
		"attendance_id": "SWP-ATT-5",
		"type":          "CHECK_IN",
		"reason":        "Lupa absen masuk.",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateCorrection_HRExemptFromWindow_201(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR01")
	old := fixedNow.AddDate(0, 0, -30) // far outside the window
	h.seedAttendance("SWP-ATT-6", "SWP-CMP-1", "SWP-EMP-1042", att.VerificationPending, old)

	rr := h.do(http.MethodPost, "/corrections", map[string]any{
		"attendance_id":               "SWP-ATT-6",
		"type":                        "CODE",
		"proposed_attendance_code_id": "SWP-AC-001",
		"reason":                      "Koreksi retroaktif oleh HR.",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201 (HR window-exempt), got %d: %s", rr.Code, rr.Body.String())
	}
}
