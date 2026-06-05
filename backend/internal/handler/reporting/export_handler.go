// Package reporting (handler) — the generic export framework endpoints:
// POST /exports (202 + bare ExportJob under {data}), GET /exports/{export_id}, and
// POST /exports/{export_id}:cancel. The DB status (RUNNING/DONE) is mapped to the
// wire enum (PROCESSING/COMPLETED) in the DTO so the built FE (use-export-flow.ts)
// drives it unchanged. Idempotency-Key is router-enforced on the action endpoints.
package reporting

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
)

// CreateExport handles POST /exports — queues a generic export (202 + ExportJob).
// The FE reads res.data.id, so the body is the job under the {data} envelope.
func (h *Handler) CreateExport(w http.ResponseWriter, r *http.Request) {
	var body exportRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"body": "Body tidak valid."}))
		return
	}
	if body.ReportType == "" || body.Format == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{
			"report_type": "report_type & format wajib.",
		}))
		return
	}

	job, err := h.exports.CreateExport(r.Context(), svc.ExportRequest{
		ReportType:   body.ReportType,
		Format:       body.Format,
		Confidential: body.Confidential,
		Filters:      body.Filters,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, dataResponse[exportJobResponse]{Data: toExportJob(job)})
}

// GetExport handles GET /exports/{export_id} — status poll (scope=self). DB status
// is wire-mapped (DONE→COMPLETED, RUNNING→PROCESSING).
func (h *Handler) GetExport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "export_id")
	job, err := h.exports.GetExport(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[exportJobResponse]{Data: toExportJob(job)})
}

// CancelExport handles POST /exports/{export_id}:cancel — cancels a QUEUED/RUNNING
// job (no-op 200 if terminal). scope=self.
func (h *Handler) CancelExport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "export_id")
	job, err := h.exports.CancelExport(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[exportJobResponse]{Data: toExportJob(job)})
}
