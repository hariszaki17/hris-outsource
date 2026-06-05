// Package payroll (handler) — POST /payslips:export (202 + PayslipExportJob stub).
package payroll

import (
	"net/http"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// ExportPayslips handles POST /payslips:export. Decodes the request, queues the
// async export (insert export_jobs QUEUED + River EnqueueTx in one tx), and
// responds 202 + Location + the PayslipExportJob stub. confidential is accepted
// but server-forced true. EXPORT_TOO_LARGE / RULE_VIOLATION surface as 422.
func (h *Handler) ExportPayslips(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	job, err := h.export.Export(r.Context(), svc.ExportRequest{
		Period:      req.Period,
		Year:        req.Year,
		EmployeeIDs: req.EmployeeIDs,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/exports/"+job.ID)
	httpx.WriteJSON(w, http.StatusAccepted, toExportJob(job))
}
