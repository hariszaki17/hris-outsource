// Package reporting (handler) — GET /reports/attendance-billable. Parses the query
// params (company_id, position [free-text], period_start/end required, group_by
// default employee), calls the billable service (which enforces leader scope + the
// period cap), and returns the report in a {data} envelope (FE unwraps query.data.data).
package reporting

import (
	"net/http"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
)

// GetBillableReport handles GET /reports/attendance-billable.
func (h *Handler) GetBillableReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	params := svc.BillableParams{
		PeriodStart: q.Get("period_start"),
		PeriodEnd:   q.Get("period_end"),
		GroupBy:     dom.BillableGroupBy(q.Get("group_by")),
	}
	if v := q.Get("company_id"); v != "" {
		params.CompanyID = &v
	}
	if v := q.Get("position"); v != "" {
		params.Position = &v
	}

	report, err := h.billable.GetBillableReport(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[billableReportResp]{Data: toBillableReport(report)})
}
