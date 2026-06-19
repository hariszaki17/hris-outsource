// Package payroll (handler) — F8.3 Payroll Run handlers (§9). Hand-written chi
// handlers: decode -> service -> httpx.WriteJSON; apperr envelopes flow through
// httpx.WriteError. Single objects wrap in {data}; lists write the page envelope
// at the top level. Mirrors the existing period_handler.go + payslip_handler.go.
package payroll

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// RunHandler holds the F8.3 payroll-run service.
type RunHandler struct {
	run *svc.RunService
}

// NewRunHandler wires the F8.3 handler.
func NewRunHandler(r *svc.RunService) *RunHandler {
	return &RunHandler{run: r}
}

// OpenRun handles POST /payroll-runs — creates a new DRAFT payroll run.
// Idempotency-Key required.
func (h *RunHandler) OpenRun(w http.ResponseWriter, r *http.Request) {
	var req openRunRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	cutoff, err := time.Parse("2006-01-02", req.CutoffDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"cutoff_date": "Format tanggal harus YYYY-MM-DD."}))
		return
	}

	run, err := h.run.OpenRun(r.Context(), req.Year, req.Month, cutoff)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/payroll-runs/"+run.ID)
	httpx.WriteJSON(w, http.StatusCreated, dataResponse[runResponse]{Data: toRunResponse(run)})
}

// ListRuns handles GET /payroll-runs — offset-paginated list, newest first.
func (h *RunHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := intParam(q.Get("limit"))
	offset := intParam(q.Get("offset"))

	runs, err := h.run.ListRuns(r.Context(), limit, offset)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]runResponse, 0, len(runs))
	for _, run := range runs {
		items = append(items, toRunResponse(run))
	}
	if items == nil {
		items = []runResponse{}
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[[]runResponse]{Data: items})
}

// GetRun handles GET /payroll-runs/{id} — single run detail.
func (h *RunHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	run, err := h.run.GetRun(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[runResponse]{Data: toRunResponse(run)})
}

// AssembleRun handles POST /payroll-runs/{id}:assemble — triggers assembly of draft
// payslip lines from E2 data. Returns the in-memory lines for HR review.
func (h *RunHandler) AssembleRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := h.run.Assemble(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	lines := make([]employeeRunLineResponse, 0, len(result.Lines))
	for _, l := range result.Lines {
		lines = append(lines, toEmployeeRunLineResponse(l))
	}

	httpx.WriteJSON(w, http.StatusOK, dataResponse[assembleRunResponse]{
		Data: assembleRunResponse{
			Run:             toRunResponse(result.Run),
			Lines:           lines,
			EligibleCount:   result.EligibleCount,
			WithAdjustments: result.WithAdjustments,
		},
	})
}

// PostRun handles POST /payroll-runs/{id}:post — generates immutable payslips from
// the reviewed assembly lines, marks adjustments applied, flips run to POSTED.
func (h *RunHandler) PostRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var body postRunBody
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	lines := make([]dom.EmployeeRunLine, 0, len(body.Lines))
	for _, l := range body.Lines {
		lines = append(lines, dom.EmployeeRunLine{
			EmployeeID:      l.EmployeeID,
			EmployeeName:    l.EmployeeName,
			EmployeeType:    l.EmployeeType,
			BaseSalaryIDR:   l.BaseSalaryIDR,
			WorkingDays:     l.WorkingDays,
			GrossEarnings:   l.GrossEarnings,
			GrossDeductions: l.GrossDeductions,
			TakeHomePay:     l.TakeHomePay,
			AdjustmentIDs:   l.AdjustmentIDs,
		})
	}

	result, err := h.run.PostRun(r.Context(), id, lines)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, dataResponse[postRunResponse]{
		Data: postRunResponse{
			Run:                toRunResponse(result.Run),
			PayslipCount:       result.PayslipCount,
			AdjustmentsApplied: result.AdjustmentsApplied,
		},
	})
}

// ListRunPayslips handles GET /payroll-runs/{id}/payslips — the payslips belonging
// to a run (decrypted, summary).
func (h *RunHandler) ListRunPayslips(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit := intParam(r.URL.Query().Get("limit"))

	rows, err := h.run.ListRunPayslips(r.Context(), id, limit)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]runPayslipResponse, 0, len(rows))
	for _, v := range rows {
		items = append(items, toRunPayslipResponse(v))
	}
	if items == nil {
		items = []runPayslipResponse{}
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[[]runPayslipResponse]{Data: items})
}

