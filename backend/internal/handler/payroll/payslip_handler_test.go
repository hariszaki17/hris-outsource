// Package payroll_test — E8 payslip read + audit-note contract tests (PAY-01).
//
// The drift gate for the 4 read/note endpoints, asserted byte-for-shape against
// docs/api/E8-payroll/openapi.yaml:
//
//	GET  /payslips                    → 200 {data,next_cursor,has_more}; FINAL money
//	                                    strings + a DECRYPT_FAIL row (money null,
//	                                    status DECRYPT_FAIL) at 200, NOT a 4xx;
//	                                    empty → meta.code MISSING_PAYROLL_HISTORY
//	GET  /payslips/{id}               → 200 {data:<Payslip>} full breakdown for
//	                                    FINAL; nulled money + [] arrays for
//	                                    DECRYPT_FAIL
//	GET  /payslips/{id}/audit-notes   → 200 {data,...} oldest-first
//	POST /payslips/{id}/audit-notes   → 201 PayslipAuditNote; composite id; blank
//	                                    text 400; missing payslip 404
//
// The DECRYPT_FAIL row status is produced HONESTLY: seedDecryptFail stores random
// garbage bytes that the REAL crypto.Decrypt rejects (ErrDecrypt) — we assert the
// row status, not a stub flag.
package payroll_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// Persona / fixture constants mirror the 10-02 seed.
const (
	empBudi = "SWP-EMP-1042" // Budi — FINAL payslips
	empRudi = "SWP-EMP-1118" // Rudi — the DECRYPT_FAIL payslip (SWP-PS-90119)
	psFinal = "SWP-PS-90121" // Budi 2025-12 FINAL (full breakdown)
	psFail  = "SWP-PS-90119" // Rudi 2025-12 DECRYPT_FAIL (garbage ciphertext)
)

var finalMoney = moneyFields{gross: "8500000.00", deduct: "1175000.00", takeHome: "7325000.00", workDays: 22}

// ---------------------------------------------------------------------------
// GET /payslips — list envelope + FINAL/DECRYPT_FAIL mixed at 200
// ---------------------------------------------------------------------------

func TestListPayslips_MixedFinalAndDecryptFailAt200(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	// FINAL paid later → sorts first under paid_on DESC.
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)
	// DECRYPT_FAIL paid earlier → second row.
	h.seedDecryptFail(psFail, empRudi, "Rudi Hartono", 2025, 11, ymd(2025, time.November, 28))

	rr := h.do("GET", "/payslips", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (decrypt-fail is a row status, NOT a 4xx), got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data missing/not an array: %T", body["data"])
	}
	if len(data) != 2 {
		t.Fatalf("data length = %d, want 2", len(data))
	}
	if _, present := body["next_cursor"]; !present {
		t.Errorf("next_cursor key missing from envelope")
	}
	if _, ok := body["has_more"].(bool); !ok {
		t.Errorf("has_more missing/not a bool: %T", body["has_more"])
	}

	rowByID := map[string]map[string]any{}
	for _, raw := range data {
		row := raw.(map[string]any)
		rowByID[row["id"].(string)] = row
	}

	// FINAL row — money strings present, status FINAL, decrypt_fail false.
	fin := rowByID[psFinal]
	if fin == nil {
		t.Fatalf("FINAL row %s missing from list", psFinal)
	}
	if fin["status"] != "FINAL" {
		t.Errorf("FINAL row status = %v, want FINAL", fin["status"])
	}
	if fin["decrypt_fail"] != false {
		t.Errorf("FINAL row decrypt_fail = %v, want false", fin["decrypt_fail"])
	}
	if fin["gross_earnings"] != "8500000.00" {
		t.Errorf("FINAL gross_earnings = %v, want \"8500000.00\"", fin["gross_earnings"])
	}
	if fin["take_home_pay"] != "7325000.00" {
		t.Errorf("FINAL take_home_pay = %v, want \"7325000.00\"", fin["take_home_pay"])
	}
	if fin["working_days"] != float64(22) {
		t.Errorf("FINAL working_days = %v, want 22", fin["working_days"])
	}
	// list shape omits the breakdown arrays.
	if _, present := fin["earnings"]; present {
		t.Errorf("list row should omit earnings[]; got %v", fin["earnings"])
	}

	// DECRYPT_FAIL row — money null, status DECRYPT_FAIL, locked_reason set, at 200.
	df := rowByID[psFail]
	if df == nil {
		t.Fatalf("DECRYPT_FAIL row %s missing from list (must NOT be filtered out)", psFail)
	}
	if df["status"] != "DECRYPT_FAIL" {
		t.Errorf("DECRYPT_FAIL row status = %v, want DECRYPT_FAIL", df["status"])
	}
	if df["decrypt_fail"] != true {
		t.Errorf("DECRYPT_FAIL row decrypt_fail = %v, want true", df["decrypt_fail"])
	}
	for _, k := range []string{"gross_earnings", "gross_deductions", "take_home_pay", "working_days"} {
		v, present := df[k]
		if !present || v != nil {
			t.Errorf("DECRYPT_FAIL row %s = %v (present=%v), want present-and-null", k, v, present)
		}
	}
	if df["locked_reason"] != "decrypt_fail" {
		t.Errorf("DECRYPT_FAIL row locked_reason = %v, want decrypt_fail", df["locked_reason"])
	}
}

func TestListPayslips_EmptyMetaCode(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	rr := h.do("GET", "/payslips", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, _ := body["data"].([]any)
	if len(data) != 0 {
		t.Errorf("data = %v, want empty array", data)
	}
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing/not an object on empty list: %v", body["meta"])
	}
	if meta["code"] != "MISSING_PAYROLL_HISTORY" {
		t.Errorf("meta.code = %v, want MISSING_PAYROLL_HISTORY", meta["code"])
	}
}

func TestListPayslips_StatusFilterReachesRepo(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)
	h.seedDecryptFail(psFail, empRudi, "Rudi Hartono", 2025, 11, ymd(2025, time.November, 28))

	rr := h.do("GET", "/payslips?status=DECRYPT_FAIL", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("status=DECRYPT_FAIL returned %d rows, want 1", len(data))
	}
	if data[0].(map[string]any)["id"] != psFail {
		t.Errorf("filtered row = %v, want %s", data[0], psFail)
	}
}

func TestListPayslips_PeriodFilterReachesRepo(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)
	h.seedFinal("SWP-PS-90123", empBudi, "Budi Santoso", 2025, 11, ymd(2025, time.November, 28), finalMoney)

	rr := h.do("GET", "/payslips?period=2025-12", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("period=2025-12 returned %d rows, want 1", len(data))
	}
	if data[0].(map[string]any)["id"] != psFinal {
		t.Errorf("filtered row = %v, want %s", data[0], psFinal)
	}
}

func TestListPayslips_HasMoreCursor(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.seedFinal("SWP-PS-90121", empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)
	h.seedFinal("SWP-PS-90123", empBudi, "Budi Santoso", 2025, 11, ymd(2025, time.November, 28), finalMoney)
	h.seedFinal("SWP-PS-90124", empBudi, "Budi Santoso", 2025, 10, ymd(2025, time.October, 28), finalMoney)

	rr := h.do("GET", "/payslips?limit=2", nil)
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
	rr2 := h.do("GET", "/payslips?limit=2&cursor="+cur, nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("page 2: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	data2 := decodeBody(t, rr2)["data"].([]any)
	if len(data2) != 1 {
		t.Errorf("page 2 data length = %d, want 1 (the remaining row)", len(data2))
	}
}

// ---------------------------------------------------------------------------
// GET /payslips/{id} — detail {data} envelope; FINAL breakdown + DECRYPT_FAIL
// ---------------------------------------------------------------------------

func TestGetPayslip_FinalFullBreakdown(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)
	h.seedFinalBreakdown(psFinal)

	rr := h.do("GET", "/payslips/"+psFinal, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "FINAL" {
		t.Errorf("status = %v, want FINAL", d["status"])
	}
	if d["read_only"] != true {
		t.Errorf("read_only = %v, want true", d["read_only"])
	}
	// source block.
	src, ok := d["source"].(map[string]any)
	if !ok {
		t.Fatalf("source missing/not an object: %v", d["source"])
	}
	if src["system"] != "lumen_swp" {
		t.Errorf("source.system = %v, want lumen_swp", src["system"])
	}
	if src["source_id"] != "44218" {
		t.Errorf("source.source_id = %v, want 44218", src["source_id"])
	}
	// earnings populated with decrypted values + for_bpjs.
	earnings, ok := d["earnings"].([]any)
	if !ok || len(earnings) != 2 {
		t.Fatalf("earnings = %v, want 2 entries", d["earnings"])
	}
	e0 := earnings[0].(map[string]any)
	if e0["name"] != "Gaji Pokok" || e0["value"] != "6500000.00" {
		t.Errorf("earnings[0] = %v, want Gaji Pokok / 6500000.00", e0)
	}
	if e0["for_bpjs"] != true {
		t.Errorf("earnings[0].for_bpjs = %v, want true", e0["for_bpjs"])
	}
	// deductions populated.
	deductions, ok := d["deductions"].([]any)
	if !ok || len(deductions) != 2 {
		t.Fatalf("deductions = %v, want 2 entries", d["deductions"])
	}
	// benefits populated (HR view, no for_bpjs field).
	benefits, ok := d["benefits"].([]any)
	if !ok || len(benefits) != 2 {
		t.Fatalf("benefits = %v, want 2 entries", d["benefits"])
	}
	b0 := benefits[0].(map[string]any)
	if b0["value"] != "260000.00" {
		t.Errorf("benefits[0].value = %v, want 260000.00", b0["value"])
	}
}

func TestGetPayslip_DecryptFailNulledEmptyArrays(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.seedDecryptFail(psFail, empRudi, "Rudi Hartono", 2025, 12, ymd(2025, time.December, 28))
	// The summary money ciphertext is garbage, so the whole payslip is DECRYPT_FAIL
	// and the breakdown is surfaced as empty arrays regardless of any component rows.

	rr := h.do("GET", "/payslips/"+psFail, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (decrypt-fail is a row status), got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "DECRYPT_FAIL" {
		t.Errorf("status = %v, want DECRYPT_FAIL", d["status"])
	}
	if d["decrypt_fail"] != true {
		t.Errorf("decrypt_fail = %v, want true", d["decrypt_fail"])
	}
	if d["locked_reason"] != "decrypt_fail" {
		t.Errorf("locked_reason = %v, want decrypt_fail", d["locked_reason"])
	}
	for _, k := range []string{"gross_earnings", "gross_deductions", "take_home_pay", "working_days"} {
		v, present := d[k]
		if !present || v != nil {
			t.Errorf("%s = %v (present=%v), want present-and-null", k, v, present)
		}
	}
	// earnings/deductions/benefits present as EMPTY arrays (not omitted, not null).
	for _, k := range []string{"earnings", "deductions", "benefits"} {
		arr, ok := d[k].([]any)
		if !ok {
			t.Fatalf("%s = %v, want an (empty) array on detail decrypt-fail", k, d[k])
		}
		if len(arr) != 0 {
			t.Errorf("%s = %v, want empty array", k, arr)
		}
	}
}

func TestGetPayslip_NotFound404(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	rr := h.do("GET", "/payslips/SWP-PS-99999", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "NOT_FOUND" {
		t.Errorf("code = %s, want NOT_FOUND", got)
	}
}

// ---------------------------------------------------------------------------
// GET/POST /payslips/{id}/audit-notes
// ---------------------------------------------------------------------------

func TestListAuditNotes_OldestFirstOnDecryptFailPayslip(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.seedDecryptFail(psFail, empRudi, "Rudi Hartono", 2025, 12, ymd(2025, time.December, 28))
	h.seedNote(psFail, "Decrypt failed pada migrasi 2026-05-30.", "SWP-EMP-9001", "Sari Hadi", ymd(2026, time.May, 30))
	h.seedNote(psFail, "Konfirmasi key payroll lama dengan finance team.", "SWP-EMP-9001", "Sari Hadi", ymd(2026, time.May, 31))

	rr := h.do("GET", "/payslips/"+psFail+"/audit-notes", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data = %v, want 2 notes", body["data"])
	}
	if _, present := body["next_cursor"]; !present {
		t.Errorf("next_cursor key missing from notes envelope")
	}
	// oldest-first: NOTE-1 before NOTE-2.
	if data[0].(map[string]any)["id"] != psFail+"-NOTE-1" {
		t.Errorf("notes[0].id = %v, want %s-NOTE-1 (oldest-first)", data[0].(map[string]any)["id"], psFail)
	}
	if data[1].(map[string]any)["id"] != psFail+"-NOTE-2" {
		t.Errorf("notes[1].id = %v, want %s-NOTE-2", data[1].(map[string]any)["id"], psFail)
	}
}

func TestCreateAuditNote_CompositeIDAndAuthor(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)

	rr := h.doWithHeaders("POST", "/payslips/"+psFinal+"/audit-notes",
		map[string]any{"text": "Sudah direview tim payroll, sesuai."},
		map[string]string{"Idempotency-Key": "11111111-1111-1111-1111-111111111111"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	// First note → composite id {payslip_id}-NOTE-1.
	if body["id"] != psFinal+"-NOTE-1" {
		t.Errorf("id = %v, want %s-NOTE-1", body["id"], psFinal)
	}
	if body["payslip_id"] != psFinal {
		t.Errorf("payslip_id = %v, want %s", body["payslip_id"], psFinal)
	}
	// author from the principal employee id.
	if body["author_id"] != "SWP-EMP-9001" {
		t.Errorf("author_id = %v, want SWP-EMP-9001", body["author_id"])
	}
	if loc := rr.Header().Get("Location"); loc == "" {
		t.Errorf("Location header missing on 201")
	}
}

func TestCreateAuditNote_BlankText400(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)

	rr := h.do("POST", "/payslips/"+psFinal+"/audit-notes", map[string]any{"text": "   "})
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 400/422 on blank text, got %d: %s", rr.Code, rr.Body.String())
	}
	if f := errFields(t, rr); f["text"] == nil {
		t.Errorf("fields.text missing on blank-text reject: %v", f)
	}
}

func TestCreateAuditNote_MissingPayslip404(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	rr := h.do("POST", "/payslips/SWP-PS-99999/audit-notes", map[string]any{"text": "Catatan."})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "NOT_FOUND" {
		t.Errorf("code = %s, want NOT_FOUND", got)
	}
}

// ---------------------------------------------------------------------------
// RBAC — shift_leader → 403 on every read/note endpoint (no payroll access).
// The AGENT is admitted to the READS (list/detail, self-scoped — see
// payslip_agent_scope_test.go) but stays 403 on the audit-note endpoints.
// ---------------------------------------------------------------------------

func TestPayslipReadEndpoints_RBACForbidden(t *testing.T) {
	allEndpoints := []struct {
		name, method, path string
		body               any
	}{
		{"list", "GET", "/payslips", nil},
		{"detail", "GET", "/payslips/" + psFinal, nil},
		{"notes-list", "GET", "/payslips/" + psFinal + "/audit-notes", nil},
		{"notes-create", "POST", "/payslips/" + psFinal + "/audit-notes", map[string]any{"text": "x"}},
	}
	// shift_leader: forbidden everywhere (no payroll surface at all).
	for _, ep := range allEndpoints {
		t.Run("shift_leader-"+ep.name, func(t *testing.T) {
			h := newHarness(t, auth.RoleShiftLeader)
			h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)
			rr := h.do(ep.method, ep.path, ep.body)
			if rr.Code != http.StatusForbidden {
				t.Fatalf("%s %s as shift_leader: expected 403, got %d: %s", ep.method, ep.path, rr.Code, rr.Body.String())
			}
		})
	}
	// agent: forbidden on the audit-note endpoints only (reads are self-scoped).
	noteEndpoints := allEndpoints[2:]
	for _, ep := range noteEndpoints {
		t.Run("agent-"+ep.name, func(t *testing.T) {
			h := newHarness(t, auth.RoleAgent)
			h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)
			rr := h.do(ep.method, ep.path, ep.body)
			if rr.Code != http.StatusForbidden {
				t.Fatalf("%s %s as agent: expected 403, got %d: %s", ep.method, ep.path, rr.Code, rr.Body.String())
			}
		})
	}
}
