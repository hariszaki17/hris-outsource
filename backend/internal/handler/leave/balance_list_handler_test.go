// Package leave_test — GET /leave-balances aggregate (one row per employee) contract
// tests, asserted byte-for-shape against docs/api/E6-leave/openapi.yaml:
//
//	GET /leave-balances → 200 {data,next_cursor,has_more}; each row = EmployeeLeaveBalance:
//	  pool_total/consumed/pending/remaining (unearmarked Σ), earmarked_remaining,
//	  next_expiry (soonest active lot w/ remaining), lot_count. q filters by name/nik/nip.
//	  Cursor keyset on (full_name, employee_id); limit+1 → has_more. Expired-only excluded.
package leave_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

func TestListLeaveBalances_AggregatesPerEmployee(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	// Andi: two unearmarked pool lots + one earmarked lot; soonest active expiry 2026-09-30.
	h.seedEmp("SWP-EMP-1001", "Andi Wijaya", "NIK-1001", "NIP-1001")
	h.seedGrant("SWP-LG-A1", "SWP-EMP-1001", 12, 4, 0, "", ymd(2026, time.December, 31)) // pool: rem 8
	h.seedGrant("SWP-LG-A2", "SWP-EMP-1001", 5, 1, 1, "", ymd(2026, time.September, 30)) // pool: rem 3
	h.seedGrant("SWP-LG-A3", "SWP-EMP-1001", 90, 0, 0, "MATERNITY", ymd(2027, time.March, 31))
	// Budi: single pool lot.
	h.seedEmp("SWP-EMP-1002", "Budi Santoso", "NIK-1002", "NIP-1002")
	h.seedGrant("SWP-LG-B1", "SWP-EMP-1002", 6, 0, 0, "", ymd(2026, time.December, 31))

	rr := h.do("GET", "/leave-balances", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data = %v, want 2 employee rows", body["data"])
	}
	if _, present := body["next_cursor"]; !present {
		t.Errorf("next_cursor key missing from envelope")
	}
	if hm, _ := body["has_more"].(bool); hm {
		t.Errorf("has_more should be false for the full page")
	}
	// Ordered by full_name: Andi first.
	andi := data[0].(map[string]any)
	for _, k := range []string{"id", "employee_id", "full_name", "nik", "nip", "pool_total",
		"pool_consumed", "pool_pending", "pool_remaining", "earmarked_remaining", "next_expiry", "lot_count"} {
		if _, ok := andi[k]; !ok {
			t.Errorf("balance row missing key: %s", k)
		}
	}
	if andi["id"] != andi["employee_id"] {
		t.Errorf("id %v != employee_id %v", andi["id"], andi["employee_id"])
	}
	if got := int(andi["pool_total"].(float64)); got != 17 { // 12 + 5
		t.Errorf("pool_total = %d, want 17", got)
	}
	if got := int(andi["pool_consumed"].(float64)); got != 5 { // 4 + 1
		t.Errorf("pool_consumed = %d, want 5", got)
	}
	if got := int(andi["pool_pending"].(float64)); got != 1 {
		t.Errorf("pool_pending = %d, want 1", got)
	}
	if got := int(andi["pool_remaining"].(float64)); got != 11 { // 8 + 3
		t.Errorf("pool_remaining = %d, want 11", got)
	}
	if got := int(andi["earmarked_remaining"].(float64)); got != 90 {
		t.Errorf("earmarked_remaining = %d, want 90", got)
	}
	if got := int(andi["lot_count"].(float64)); got != 3 {
		t.Errorf("lot_count = %d, want 3", got)
	}
	// next_expiry = soonest active lot with remaining > 0 → 2026-09-30.
	if got, _ := andi["next_expiry"].(string); got != "2026-09-30" {
		t.Errorf("next_expiry = %q, want 2026-09-30", got)
	}
}

func TestListLeaveBalances_QFiltersByNameNikNip(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedEmp("SWP-EMP-2001", "Citra Dewi", "3201999", "NIP-CD")
	h.seedGrant("SWP-LG-C1", "SWP-EMP-2001", 10, 0, 0, "", ymd(2026, time.December, 31))
	h.seedEmp("SWP-EMP-2002", "Dewa Putra", "3202888", "NIP-DP")
	h.seedGrant("SWP-LG-D1", "SWP-EMP-2002", 10, 0, 0, "", ymd(2026, time.December, 31))

	cases := []struct {
		name, q, wantEmp string
	}{
		{"by name", "Citra", "SWP-EMP-2001"},
		{"by nik", "3202888", "SWP-EMP-2002"},
		{"by nip", "NIP-DP", "SWP-EMP-2002"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := h.do("GET", "/leave-balances?q="+c.q, nil)
			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
			}
			data := decodeBody(t, rr)["data"].([]any)
			if len(data) != 1 {
				t.Fatalf("q=%q → %d rows, want 1", c.q, len(data))
			}
			if got := data[0].(map[string]any)["employee_id"]; got != c.wantEmp {
				t.Errorf("q=%q → employee_id %v, want %s", c.q, got, c.wantEmp)
			}
		})
	}
}

func TestListLeaveBalances_CursorPagination(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	// Three employees, ordered by full_name: Ana, Bima, Citra.
	for _, e := range []struct{ id, name string }{
		{"SWP-EMP-3001", "Ana"}, {"SWP-EMP-3002", "Bima"}, {"SWP-EMP-3003", "Citra"},
	} {
		h.seedEmp(e.id, e.name, "NIK-"+e.id, "NIP-"+e.id)
		h.seedGrant("SWP-LG-"+e.id, e.id, 5, 0, 0, "", ymd(2026, time.December, 31))
	}

	rr := h.do("GET", "/leave-balances?limit=2", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("page 1 = %d rows, want 2", len(data))
	}
	if hm, _ := body["has_more"].(bool); !hm {
		t.Errorf("has_more should be true (3 rows, limit 2)")
	}
	cursor, _ := body["next_cursor"].(string)
	if cursor == "" {
		t.Fatalf("next_cursor empty on a has_more page")
	}
	if data[0].(map[string]any)["full_name"] != "Ana" || data[1].(map[string]any)["full_name"] != "Bima" {
		t.Errorf("page 1 order wrong: %v, %v", data[0].(map[string]any)["full_name"], data[1].(map[string]any)["full_name"])
	}

	rr2 := h.do("GET", "/leave-balances?limit=2&cursor="+cursor, nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("page 2 expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	body2 := decodeBody(t, rr2)
	data2 := body2["data"].([]any)
	if len(data2) != 1 {
		t.Fatalf("page 2 = %d rows, want 1", len(data2))
	}
	if data2[0].(map[string]any)["full_name"] != "Citra" {
		t.Errorf("page 2 row = %v, want Citra", data2[0].(map[string]any)["full_name"])
	}
	if hm, _ := body2["has_more"].(bool); hm {
		t.Errorf("page 2 has_more should be false")
	}
}

func TestListLeaveBalances_ExpiredOnlyEmployeeExcluded(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	// Active employee.
	h.seedEmp("SWP-EMP-4001", "Eka Sari", "NIK-4001", "NIP-4001")
	h.seedGrant("SWP-LG-E1", "SWP-EMP-4001", 10, 0, 0, "", ymd(2026, time.December, 31))
	// Employee with ONLY an expired lot (expires before fixedNow 2026-06-04) — excluded.
	h.seedEmp("SWP-EMP-4002", "Farel Adi", "NIK-4002", "NIP-4002")
	h.seedGrant("SWP-LG-F1", "SWP-EMP-4002", 10, 0, 0, "", ymd(2026, time.January, 31))

	rr := h.do("GET", "/leave-balances", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data = %d rows, want 1 (expired-only employee excluded)", len(data))
	}
	if got := data[0].(map[string]any)["employee_id"]; got != "SWP-EMP-4001" {
		t.Errorf("listed employee = %v, want SWP-EMP-4001", got)
	}
}
