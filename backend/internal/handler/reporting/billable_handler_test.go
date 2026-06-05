// Package reporting_test — E10 billable-report contract tests (F10.3), asserted
// against docs/api/E10-reporting/openapi.yaml BillableReport examples:
//
//	GET /reports/attendance-billable (hr) → 200 BillableReport with summary +
//	    pending_summary{pending_records, pending_hours_estimate, note} + rows[]
//	    matching BillableReportRow; verification_rate_pct null when zero records
//	    (emptyAfterFilters).
//	leader requesting company != own → 403 OUT_OF_SCOPE.
//	period range > 1 year → 422 REPORT_PERIOD_TOO_WIDE with fields.period_end.
package reporting_test

import (
	"net/http"
	"testing"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

func TestBillableReport_HrSummaryPendingRows(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	h.billable.summary = dom.BillableSummary{
		TotalBillableHours:   4128.5,
		TotalWorkedHours:     4310.0,
		TotalPayableHours:    4232.0,
		TotalVerifiedRecords: 612,
	}
	h.billable.pending = dom.BillablePendingSummary{
		PendingRecords:       8,
		PendingHoursEstimate: 64.0,
	}
	h.billable.rows = []dom.BillableReportRow{
		{
			GroupKey:            "SWP-EMP-3104",
			GroupLabel:          "Budi Santoso",
			CompanyID:           strp("SWP-CMP-0021"),
			CompanyName:         strp("Plaza Senayan"),
			ServiceLineID:       strp("SWP-SVC-001"),
			ServiceLineName:     strp("Facility Services"),
			WorkedHours:         176.0,
			BillableHours:       168.0,
			PayableHours:        172.0,
			VerifiedRecordCount: 22,
		},
	}

	rr := h.do("GET", "/reports/attendance-billable?company_id=SWP-CMP-0021&period_start=2026-06-01&period_end=2026-06-30&group_by=employee", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)

	filters, ok := d["filters"].(map[string]any)
	if !ok {
		t.Fatalf("filters missing/not object: %v", d["filters"])
	}
	if filters["period_start"] != "2026-06-01" || filters["period_end"] != "2026-06-30" {
		t.Errorf("filters period = %v..%v, want 2026-06-01..2026-06-30", filters["period_start"], filters["period_end"])
	}
	if filters["group_by"] != "employee" {
		t.Errorf("filters.group_by = %v, want employee", filters["group_by"])
	}

	summary, ok := d["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary missing/not object: %v", d["summary"])
	}
	if summary["total_billable_hours"] != 4128.5 {
		t.Errorf("total_billable_hours = %v, want 4128.5", summary["total_billable_hours"])
	}
	if summary["total_verified_records"] != float64(612) {
		t.Errorf("total_verified_records = %v, want 612", summary["total_verified_records"])
	}
	// verification_rate_pct = 612 / (612 + 8) * 100 = 98.7...; present (non-null).
	vr, ok := summary["verification_rate_pct"].(float64)
	if !ok {
		t.Fatalf("verification_rate_pct missing/not a number: %v", summary["verification_rate_pct"])
	}
	if vr < 98.0 || vr > 99.0 {
		t.Errorf("verification_rate_pct = %v, want ~98.7", vr)
	}

	pending, ok := d["pending_summary"].(map[string]any)
	if !ok {
		t.Fatalf("pending_summary missing/not object: %v", d["pending_summary"])
	}
	if pending["pending_records"] != float64(8) {
		t.Errorf("pending_records = %v, want 8", pending["pending_records"])
	}
	if pending["pending_hours_estimate"] != 64.0 {
		t.Errorf("pending_hours_estimate = %v, want 64.0", pending["pending_hours_estimate"])
	}
	if pending["note"] != "Belum dapat ditagih hingga diverifikasi." {
		t.Errorf("pending note = %v, want the BR-6 callout copy", pending["note"])
	}

	rows, ok := d["rows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("rows = %v, want 1 row", d["rows"])
	}
	row := rows[0].(map[string]any)
	if row["group_key"] != "SWP-EMP-3104" || row["group_label"] != "Budi Santoso" {
		t.Errorf("row group = %v / %v, want SWP-EMP-3104 / Budi Santoso", row["group_key"], row["group_label"])
	}
	if row["worked_hours"] != 176.0 || row["billable_hours"] != 168.0 || row["payable_hours"] != 172.0 {
		t.Errorf("row hours = %v/%v/%v, want 176/168/172", row["worked_hours"], row["billable_hours"], row["payable_hours"])
	}
	if row["verified_record_count"] != float64(22) {
		t.Errorf("verified_record_count = %v, want 22", row["verified_record_count"])
	}
	if _, ok := row["unverified_record_count"]; !ok {
		t.Errorf("unverified_record_count missing (required field)")
	}
}

func TestBillableReport_EmptyVerificationRateNull(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	// Zero records → verification_rate_pct null (emptyAfterFilters example).
	h.billable.summary = dom.BillableSummary{}
	h.billable.pending = dom.BillablePendingSummary{}
	h.billable.rows = nil

	rr := h.do("GET", "/reports/attendance-billable?company_id=SWP-CMP-0099&period_start=2026-06-01&period_end=2026-06-30", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	summary := d["summary"].(map[string]any)
	if v, exists := summary["verification_rate_pct"]; !exists {
		t.Errorf("verification_rate_pct key missing (required, nullable)")
	} else if v != nil {
		t.Errorf("verification_rate_pct = %v, want null when zero records", v)
	}
	// rows is an empty array (present, not null).
	rows, ok := d["rows"].([]any)
	if !ok {
		t.Fatalf("rows not an array: %v", d["rows"])
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0", len(rows))
	}
	// empty pending note "".
	pending := d["pending_summary"].(map[string]any)
	if pending["note"] != "" {
		t.Errorf("empty pending note = %v, want \"\"", pending["note"])
	}
}

func TestBillableReport_LeaderCrossCompanyOutOfScope(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-0021", "SWP-EMP-7001")

	// Leader requests a company that is NOT their own → 403 OUT_OF_SCOPE.
	rr := h.do("GET", "/reports/attendance-billable?company_id=SWP-CMP-0099&period_start=2026-06-01&period_end=2026-06-30", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "OUT_OF_SCOPE" {
		t.Errorf("code = %s, want OUT_OF_SCOPE", got)
	}
}

func TestBillableReport_PeriodTooWide422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")

	// > 1 year range → 422 REPORT_PERIOD_TOO_WIDE with fields.period_end.
	rr := h.do("GET", "/reports/attendance-billable?period_start=2025-01-01&period_end=2026-06-30", nil)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "REPORT_PERIOD_TOO_WIDE" {
		t.Errorf("code = %s, want REPORT_PERIOD_TOO_WIDE", got)
	}
	if f := errFields(t, rr); f["period_end"] == nil {
		t.Errorf("fields.period_end missing on REPORT_PERIOD_TOO_WIDE: %v", f)
	}
}
