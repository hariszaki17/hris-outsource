// Package leave_test — E6 leave-quota (F6.2 / LVE-02) contract tests.
//
// The drift gate for the 3 quota endpoints, asserted byte-for-shape against
// docs/api/E6-leave/openapi.yaml:
//
//	GET  /leave-quotas              → 200 {data,next_cursor,has_more}; remaining = total-used-pending
//	                                  (LeaveQuotaListResponseExample)
//	POST /leave-quotas/{id}:adjust  → 200 {data} total adjusted + last_adjustment (QuotaAfterAdjustExample);
//	                                  total<used → 422 RULE_VIOLATION(fields.delta); missing reason → 400
//	POST /leave-quotas:bulk-grant   → 200 {preview,total_affected,succeeded[],failed[]} partial success
//	                                  (BulkGrantApplyResponseExample); preview=true writes nothing
//	                                  (BulkGrantPreviewResponseExample)
package leave_test

import (
	"net/http"
	"testing"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// ---------------------------------------------------------------------------
// GET /leave-quotas — list shape + remaining math
// ---------------------------------------------------------------------------

func TestListLeaveQuotas_RemainingMath(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedQuota("SWP-LQ-8001", empAgent, leaveAnn, 2026, 12, 4, 0)       // remaining 8
	h.seedQuota("SWP-LQ-8002", "SWP-EMP-1188", leaveAnn, 2026, 12, 2, 1) // remaining 9

	rr := h.do("GET", "/leave-quotas?period=2026", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data = %v, want 2 quota rows", body["data"])
	}
	if _, present := body["next_cursor"]; !present {
		t.Errorf("next_cursor key missing from envelope")
	}
	// each row: remaining = total - used - pending; shape carries the openapi keys.
	for _, raw := range data {
		row := raw.(map[string]any)
		for _, k := range []string{"id", "employee_id", "leave_type_id", "period", "total", "used", "pending", "remaining", "last_adjustment", "last_override"} {
			if _, ok := row[k]; !ok {
				t.Errorf("quota row missing key: %s", k)
			}
		}
		total := int(row["total"].(float64))
		used := int(row["used"].(float64))
		pending := int(row["pending"].(float64))
		remaining := int(row["remaining"].(float64))
		if remaining != total-used-pending {
			t.Errorf("remaining = %d, want %d (total-used-pending)", remaining, total-used-pending)
		}
	}
}

func TestListLeaveQuotas_PendingRecomputedOnRead(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	q := h.seedQuota("SWP-LQ-8001", empAgent, leaveAnn, 2026, 12, 4, 0)
	// a PENDING request reserves 2 days → recompute-on-read must surface pending=2.
	h.quota.pending[q.ID] = 2

	rr := h.do("GET", "/leave-quotas?period=2026", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	row := decodeBody(t, rr)["data"].([]any)[0].(map[string]any)
	if p := int(row["pending"].(float64)); p != 2 {
		t.Errorf("pending = %d, want 2 (recomputed-on-read)", p)
	}
	if rem := int(row["remaining"].(float64)); rem != 6 {
		t.Errorf("remaining = %d, want 6 (12-4-2)", rem)
	}
}

// ---------------------------------------------------------------------------
// POST :adjust — total adjusted + last_adjustment; total<used refused 422
// ---------------------------------------------------------------------------

func TestAdjustQuota_Happy(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedQuota("SWP-LQ-8001", empAgent, leaveAnn, 2026, 12, 4, 0)

	rr := h.do("POST", "/leave-quotas/SWP-LQ-8001:adjust", map[string]any{
		"delta": 1, "reason": "Koreksi entitlement sesuai surat HRD.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if total := int(d["total"].(float64)); total != 13 {
		t.Errorf("total = %d, want 13 (12+1) (QuotaAfterAdjustExample)", total)
	}
	if remaining := int(d["remaining"].(float64)); remaining != 9 {
		t.Errorf("remaining = %d, want 9 (13-4-0)", remaining)
	}
	la, ok := d["last_adjustment"].(map[string]any)
	if !ok {
		t.Fatalf("last_adjustment missing/not an object: %v", d["last_adjustment"])
	}
	if int(la["delta"].(float64)) != 1 {
		t.Errorf("last_adjustment.delta = %v, want 1", la["delta"])
	}
	for _, k := range []string{"delta", "reason", "adjusted_by", "adjusted_at"} {
		if _, ok := la[k]; !ok {
			t.Errorf("last_adjustment missing key: %s", k)
		}
	}
}

func TestAdjustQuota_RefuseTotalBelowUsed422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedQuota("SWP-LQ-8001", empAgent, leaveAnn, 2026, 12, 10, 0) // used 10
	// delta -5 → total 7 < used 10 → refused.
	rr := h.do("POST", "/leave-quotas/SWP-LQ-8001:adjust", map[string]any{
		"delta": -5, "reason": "Pengurangan kuota tahunan.",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "RULE_VIOLATION" {
		t.Fatalf("code = %s, want RULE_VIOLATION", got)
	}
	if f := errFields(t, rr); f["delta"] == nil {
		t.Errorf("fields.delta missing: %v", f)
	}
	// no change.
	if q := h.quota.byID["SWP-LQ-8001"]; q.Total != 12 {
		t.Errorf("total = %d, want 12 (unchanged on refuse)", q.Total)
	}
}

func TestAdjustQuota_MissingReason400(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedQuota("SWP-LQ-8001", empAgent, leaveAnn, 2026, 12, 4, 0)
	rr := h.do("POST", "/leave-quotas/SWP-LQ-8001:adjust", map[string]any{"delta": 1, "reason": "no"})
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 400/422, got %d: %s", rr.Code, rr.Body.String())
	}
	if f := errFields(t, rr); f["reason"] == nil {
		t.Errorf("fields.reason missing on a short-reason adjust: %v", f)
	}
}

// ---------------------------------------------------------------------------
// POST :bulk-grant — pro-rate + partial success; preview writes nothing
// ---------------------------------------------------------------------------

func TestBulkGrant_ApplyPartialSuccess(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	// grant set: one full-year joiner, one mid-year (pro-rate), one whose existing
	// used (14) exceeds the new total (12) → lands in failed[].
	h.quota.grantSet = []svc.GrantCandidate{
		{EmployeeID: "SWP-EMP-1042", EmployeeName: strp("Budi Santoso"), PlacementStart: ymd(2025, time.January, 1)},
		{EmployeeID: "SWP-EMP-1188", EmployeeName: strp("Dewi Lestari"), PlacementStart: ymd(2026, time.June, 1)},
		{EmployeeID: "SWP-EMP-1099", EmployeeName: strp("Andika"), PlacementStart: ymd(2025, time.January, 1)},
	}
	// the over-used existing quota for the failed row.
	h.seedQuota("SWP-LQ-OVER", "SWP-EMP-1099", leaveAnn, 2026, 14, 14, 0)

	rr := h.do("POST", "/leave-quotas:bulk-grant", map[string]any{
		"leave_type_id":            leaveAnn,
		"period":                   2026,
		"default_entitlement_days": 12,
		"employee_ids":             []string{"all"},
		"pro_rate":                 true,
		"preview":                  false,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if pv, _ := body["preview"].(bool); pv {
		t.Errorf("preview = true, want false")
	}
	if ta := int(body["total_affected"].(float64)); ta != 3 {
		t.Errorf("total_affected = %d, want 3", ta)
	}
	succeeded, _ := body["succeeded"].([]any)
	failed, _ := body["failed"].([]any)
	if len(succeeded) != 2 {
		t.Errorf("succeeded = %d, want 2", len(succeeded))
	}
	if len(failed) != 1 {
		t.Fatalf("failed = %d, want 1 (the over-used row)", len(failed))
	}
	// the failed row carries RULE_VIOLATION + the over-used employee id.
	fr := failed[0].(map[string]any)
	if fr["employee_id"] != "SWP-EMP-1099" {
		t.Errorf("failed[0].employee_id = %v, want SWP-EMP-1099", fr["employee_id"])
	}
	ferr := fr["error"].(map[string]any)
	if ferr["code"] != "RULE_VIOLATION" {
		t.Errorf("failed[0].error.code = %v, want RULE_VIOLATION", ferr["code"])
	}
	// applied rows carry a non-null quota_id; the mid-year joiner is prorated.
	var sawProrated, sawQuotaID bool
	for _, raw := range succeeded {
		row := raw.(map[string]any)
		if row["quota_id"] != nil {
			sawQuotaID = true
		}
		if row["employee_id"] == "SWP-EMP-1188" {
			if ip, _ := row["is_prorated"].(bool); !ip {
				t.Errorf("mid-year joiner is_prorated = %v, want true", row["is_prorated"])
			}
			if pm := int(row["prorate_months"].(float64)); pm != 7 {
				t.Errorf("prorate_months = %d, want 7 (Jun-Dec inclusive)", pm)
			}
			if total := int(row["total"].(float64)); total != 7 {
				t.Errorf("prorated total = %d, want 7 (round(12*7/12))", total)
			}
			sawProrated = true
		}
	}
	if !sawProrated {
		t.Errorf("no prorated row found in succeeded[]")
	}
	if !sawQuotaID {
		t.Errorf("apply succeeded[] rows missing a written quota_id")
	}
}

func TestBulkGrant_PreviewNoWrite(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.quota.grantSet = []svc.GrantCandidate{
		{EmployeeID: "SWP-EMP-1042", EmployeeName: strp("Budi Santoso"), PlacementStart: ymd(2025, time.January, 1)},
		{EmployeeID: "SWP-EMP-1188", EmployeeName: strp("Dewi Lestari"), PlacementStart: ymd(2026, time.June, 1)},
	}
	before := len(h.quota.byID)

	rr := h.do("POST", "/leave-quotas:bulk-grant", map[string]any{
		"leave_type_id":            leaveAnn,
		"period":                   2026,
		"default_entitlement_days": 12,
		"employee_ids":             []string{"all"},
		"pro_rate":                 true,
		"preview":                  true,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if pv, _ := body["preview"].(bool); !pv {
		t.Errorf("preview = false, want true")
	}
	succeeded := body["succeeded"].([]any)
	if len(succeeded) != 2 {
		t.Errorf("succeeded = %d, want 2 (computed, not written)", len(succeeded))
	}
	// preview projections carry a null quota_id (nothing written).
	for _, raw := range succeeded {
		if raw.(map[string]any)["quota_id"] != nil {
			t.Errorf("preview row has non-null quota_id: %v", raw)
		}
	}
	// no new quota rows persisted.
	if len(h.quota.byID) != before {
		t.Errorf("preview wrote %d new quota rows, want 0", len(h.quota.byID)-before)
	}
}

// silence unused import if the file is trimmed.
var _ = dom.LeaveStatusApproved
