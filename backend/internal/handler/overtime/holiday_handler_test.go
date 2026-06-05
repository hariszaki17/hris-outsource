// Package overtime_test — E7 holiday-calendar (F7.3 / OVT-02) contract tests.
//
// The drift gate for the 4 holiday endpoints, asserted byte-for-shape against
// docs/api/E7-overtime/openapi.yaml:
//
//	GET    /holidays        → 200 {data,next_cursor,has_more}; in_use_by_overtime computed
//	POST   /holidays        → 201 Holiday (in_use_by_overtime false); dup date+category
//	                          → 409 HOLIDAY_DATE_CLASH
//	PATCH  /holidays/{id}    → 200 Holiday
//	DELETE /holidays/{id}    → 204; referenced by APPROVED OT → 409 HOLIDAY_IN_USE
package overtime_test

import (
	"net/http"
	"testing"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

var (
	holDate     = ymd(2026, time.August, 17) // Independence Day — in range
	holDateFree = ymd(2026, time.December, 25)
)

// ---------------------------------------------------------------------------
// GET /holidays — cursor envelope + in_use_by_overtime computed per row
// ---------------------------------------------------------------------------

func TestListHolidays_Envelope(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedHoliday("SWP-HOL-9001", "Hari Kemerdekaan", holDate, dom.HolidayCategoryNational, 2) // in use
	h.seedHoliday("SWP-HOL-9002", "Hari Natal", holDateFree, dom.HolidayCategoryNational, 0)   // free

	rr := h.do("GET", "/holidays?year=2026", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data = %v, want 2 holiday rows", body["data"])
	}
	if _, present := body["next_cursor"]; !present {
		t.Errorf("next_cursor key missing from envelope")
	}
	if _, ok := body["has_more"].(bool); !ok {
		t.Errorf("has_more missing/not a bool: %T", body["has_more"])
	}
	// keyset is ASC by date: Aug 17 precedes Dec 25.
	first := data[0].(map[string]any)
	for _, k := range []string{"id", "name", "date", "category", "recurring", "applicable_service_lines", "in_use_by_overtime"} {
		if _, ok := first[k]; !ok {
			t.Errorf("holiday row missing key: %s", k)
		}
	}
	if first["id"] != "SWP-HOL-9001" {
		t.Errorf("first row id = %v, want SWP-HOL-9001 (ASC by date)", first["id"])
	}
	// in_use_by_overtime computed from CountOvertimeUsingHoliday.
	if iu, _ := first["in_use_by_overtime"].(bool); !iu {
		t.Errorf("SWP-HOL-9001 in_use_by_overtime = false, want true (2 OT references)")
	}
	if iu, _ := data[1].(map[string]any)["in_use_by_overtime"].(bool); iu {
		t.Errorf("SWP-HOL-9002 in_use_by_overtime = true, want false (free holiday)")
	}
}

func TestListHolidays_HasMoreCursor(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedHoliday("SWP-HOL-9001", "A", ymd(2026, time.March, 1), dom.HolidayCategoryNational, 0)
	h.seedHoliday("SWP-HOL-9002", "B", ymd(2026, time.June, 1), dom.HolidayCategoryNational, 0)
	h.seedHoliday("SWP-HOL-9003", "C", ymd(2026, time.September, 1), dom.HolidayCategoryNational, 0)

	rr := h.do("GET", "/holidays?limit=2", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if hm, _ := body["has_more"].(bool); !hm {
		t.Errorf("has_more = false, want true (3 rows, limit 2)")
	}
	cur, ok := body["next_cursor"].(string)
	if !ok || cur == "" {
		t.Fatalf("next_cursor missing/empty when has_more: %v", body["next_cursor"])
	}
	rr2 := h.do("GET", "/holidays?limit=2&cursor="+cur, nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("page 2: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	if data2 := decodeBody(t, rr2)["data"].([]any); len(data2) != 1 {
		t.Errorf("page 2 data length = %d, want 1", len(data2))
	}
}

// ---------------------------------------------------------------------------
// POST /holidays — 201 happy; dup date+category → 409 HOLIDAY_DATE_CLASH
// ---------------------------------------------------------------------------

func TestCreateHoliday_Happy201(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	rr := h.do("POST", "/holidays", map[string]any{
		"name": "Hari Kemerdekaan", "date": "2026-08-17", "category": "NATIONAL",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["name"] != "Hari Kemerdekaan" {
		t.Errorf("name = %v, want Hari Kemerdekaan", body["name"])
	}
	if body["date"] != "2026-08-17" {
		t.Errorf("date = %v, want 2026-08-17", body["date"])
	}
	if iu, _ := body["in_use_by_overtime"].(bool); iu {
		t.Errorf("new holiday in_use_by_overtime = true, want false")
	}
	if loc := rr.Header().Get("Location"); loc == "" {
		t.Errorf("Location header missing on 201")
	}
}

func TestCreateHoliday_DateClash409(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedHoliday("SWP-HOL-9001", "Hari Kemerdekaan", holDate, dom.HolidayCategoryNational, 0)
	rr := h.do("POST", "/holidays", map[string]any{
		"name": "Duplikat", "date": "2026-08-17", "category": "NATIONAL",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "HOLIDAY_DATE_CLASH" {
		t.Errorf("code = %s, want HOLIDAY_DATE_CLASH", got)
	}
}

// ---------------------------------------------------------------------------
// PATCH /holidays/{id} — 200 happy update
// ---------------------------------------------------------------------------

func TestUpdateHoliday_Happy200(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedHoliday("SWP-HOL-9001", "Hari Kemerdekaan", holDate, dom.HolidayCategoryNational, 0)
	rr := h.do("PATCH", "/holidays/SWP-HOL-9001", map[string]any{"name": "HUT RI ke-81"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if body := decodeBody(t, rr); body["name"] != "HUT RI ke-81" {
		t.Errorf("name = %v, want HUT RI ke-81", body["name"])
	}
}

// ---------------------------------------------------------------------------
// DELETE /holidays/{id} — 204 free; HOLIDAY_IN_USE 409 when referenced
// ---------------------------------------------------------------------------

func TestDeleteHoliday_Free204(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedHoliday("SWP-HOL-9002", "Hari Natal", holDateFree, dom.HolidayCategoryNational, 0)
	rr := h.do("DELETE", "/holidays/SWP-HOL-9002", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, ok := h.holiday.byID["SWP-HOL-9002"]; ok {
		t.Errorf("holiday not soft-deleted")
	}
}

func TestDeleteHoliday_InUse409(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	// CountOvertimeUsingHoliday = 1 (an APPROVED OT references it) → blocked.
	h.seedHoliday("SWP-HOL-9001", "Hari Kemerdekaan", holDate, dom.HolidayCategoryNational, 1)
	rr := h.do("DELETE", "/holidays/SWP-HOL-9001", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "HOLIDAY_IN_USE" {
		t.Errorf("code = %s, want HOLIDAY_IN_USE", got)
	}
	// still present (not deleted).
	if _, ok := h.holiday.byID["SWP-HOL-9001"]; !ok {
		t.Errorf("in-use holiday was deleted despite 409")
	}
}
