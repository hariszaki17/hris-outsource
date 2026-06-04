// Package scheduling_test — shift-master (F4.1 / SM-*) contract tests.
//
// Asserts the GET list envelope ({data,next_cursor,has_more}), cross_midnight
// derivation, DUPLICATE_NAME (409), BREAK_OUTSIDE_WINDOW (422), the
// deactivate/reactivate cycle + ALREADY_INACTIVE (409), and the leader-write 403
// match docs/api/E4-shift-scheduling/openapi.yaml EXACTLY.
package scheduling_test

import (
	"net/http"
	"testing"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

func TestListShiftMasters_Envelope(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
	h.seedMaster("SWP-SHF-002", "Malam", "23:00", "07:00", strp("SWP-SVC-003"), true)

	rr := h.do("GET", "/shift-masters", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	for _, k := range []string{"data", "next_cursor", "has_more"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing envelope key: %s", k)
		}
	}
	data, ok := body["data"].([]any)
	if !ok || len(data) == 0 {
		t.Fatalf("data is not a non-empty array: %T %v", body["data"], body["data"])
	}
	first := data[0].(map[string]any)
	for _, k := range []string{"id", "name", "start_time", "end_time", "status", "break_minutes", "in_use_count", "cross_midnight"} {
		if _, ok := first[k]; !ok {
			t.Errorf("data[0] missing key: %s", k)
		}
	}
	if s := strOf(first["status"]); s != "ACTIVE" && s != "INACTIVE" {
		t.Errorf("status = %v, want ACTIVE|INACTIVE", first["status"])
	}
}

func TestCreateShiftMaster_CrossMidnightDerived(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")

	// end_time <= start_time → server derives cross_midnight=true.
	rr := h.do("POST", "/shift-masters", map[string]any{
		"name":       "Malam",
		"start_time": "23:00",
		"end_time":   "07:00",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["cross_midnight"] != true {
		t.Errorf("cross_midnight = %v, want true (end<=start)", body["cross_midnight"])
	}
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE (default)", body["status"])
	}
}

func TestCreateShiftMaster_DuplicateName(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")

	first := h.do("POST", "/shift-masters", map[string]any{
		"name": "Pagi", "start_time": "07:00", "end_time": "15:00",
	})
	if first.Code != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d: %s", first.Code, first.Body.String())
	}

	second := h.do("POST", "/shift-masters", map[string]any{
		"name": "Pagi", "start_time": "08:00", "end_time": "16:00",
	})
	if second.Code != http.StatusConflict {
		t.Fatalf("duplicate name: expected 409, got %d: %s", second.Code, second.Body.String())
	}
	if code := errCode(t, second); code != "DUPLICATE_NAME" {
		t.Errorf("error.code = %q, want DUPLICATE_NAME", code)
	}
}

func TestCreateShiftMaster_BreakOutsideWindow(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")

	// Break 16:00-17:00 lies outside the 07:00-15:00 same-day window.
	rr := h.do("POST", "/shift-masters", map[string]any{
		"name":        "Pagi",
		"start_time":  "07:00",
		"end_time":    "15:00",
		"break_start": "16:00",
		"break_end":   "17:00",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "BREAK_OUTSIDE_WINDOW" {
		t.Errorf("error.code = %v, want BREAK_OUTSIDE_WINDOW", e["code"])
	}
	fields, _ := e["fields"].(map[string]any)
	if _, ok := fields["break_start"]; !ok {
		t.Errorf("error.fields.break_start missing on BREAK_OUTSIDE_WINDOW")
	}
}

func TestDeactivateReactivate(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)

	// Deactivate active → 200 INACTIVE.
	off := h.do("POST", "/shift-masters/SWP-SHF-001:deactivate", nil)
	if off.Code != http.StatusOK {
		t.Fatalf("deactivate: expected 200, got %d: %s", off.Code, off.Body.String())
	}
	if b := decodeBody(t, off); b["status"] != "INACTIVE" || b["is_active"] != false {
		t.Errorf("after deactivate: status=%v is_active=%v, want INACTIVE/false", b["status"], b["is_active"])
	}

	// Deactivate again → 409 ALREADY_INACTIVE.
	again := h.do("POST", "/shift-masters/SWP-SHF-001:deactivate", nil)
	if again.Code != http.StatusConflict {
		t.Fatalf("re-deactivate: expected 409, got %d: %s", again.Code, again.Body.String())
	}
	if code := errCode(t, again); code != "ALREADY_INACTIVE" {
		t.Errorf("error.code = %q, want ALREADY_INACTIVE", code)
	}

	// Reactivate → 200 ACTIVE.
	on := h.do("POST", "/shift-masters/SWP-SHF-001:reactivate", nil)
	if on.Code != http.StatusOK {
		t.Fatalf("reactivate: expected 200, got %d: %s", on.Code, on.Body.String())
	}
	if b := decodeBody(t, on); b["status"] != "ACTIVE" || b["is_active"] != true {
		t.Errorf("after reactivate: status=%v is_active=%v, want ACTIVE/true", b["status"], b["is_active"])
	}
}

func TestShiftMasterWrites_RBAC(t *testing.T) {
	// shift_leader is NOT in the shift-master write group (super/hr only).
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-0021")

	rr := h.do("POST", "/shift-masters", map[string]any{
		"name": "Sore", "start_time": "15:00", "end_time": "23:00",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("leader POST /shift-masters: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}
