// Package payroll (handler) — F8.5 Payroll Period Close handlers (§9). Hand-written
// chi handlers: decode -> service -> httpx.WriteJSON; apperr envelopes flow through
// httpx.WriteError. Lists write the page envelope at the top level; single objects
// wrap in {data}. Mirrors the existing payroll handlers + the scheduling slice.
package payroll

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// PeriodHandler holds the F8.5 services (period FSM + clarification back-channel).
type PeriodHandler struct {
	period        *svc.PeriodService
	clarification *svc.ClarificationService
}

// NewPeriodHandler wires the F8.5 handler.
func NewPeriodHandler(p *svc.PeriodService, c *svc.ClarificationService) *PeriodHandler {
	return &PeriodHandler{period: p, clarification: c}
}

// --- period ---

// GetPeriod handles GET /payroll-periods/{year}/{month} — status + FIELD per-tab
// blocker counts (auto-creates OPEN, PC-1).
func (h *PeriodHandler) GetPeriod(w http.ResponseWriter, r *http.Request) {
	year, month, err := pathYearMonth(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	view, err := h.period.GetOrCreatePeriod(r.Context(), year, month)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resp := toPeriod(view.Period)
	bc := toBlockerCounts(view.Blockers)
	resp.Blockers = &bc
	httpx.WriteJSON(w, http.StatusOK, dataResponse[periodResponse]{Data: resp})
}

// GetCompleteness handles GET /payroll-periods/{year}/{month}/completeness — the
// per-employee completeness rows (cursor-paged).
func (h *PeriodHandler) GetCompleteness(w http.ResponseWriter, r *http.Request) {
	year, month, err := pathYearMonth(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := r.URL.Query()
	rows, next, err := h.period.Completeness(r.Context(), year, month, q.Get("cursor"), intParam(q.Get("limit")))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]completenessResponse, 0, len(rows))
	for _, c := range rows {
		items = append(items, toCompleteness(c))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[completenessResponse]{
		Data: items, NextCursor: next, HasMore: next != nil,
	})
}

// LockPeriod handles POST /payroll-periods/{year}/{month}:lock. Body optional
// {force, reason}; force = super-admin (enforced in the service). 422
// PERIOD_HAS_BLOCKERS when blockers>0 and not forced.
func (h *PeriodHandler) LockPeriod(w http.ResponseWriter, r *http.Request) {
	year, month, err := pathYearMonth(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req lockRequest
	// Body is optional — an empty body is a non-forced lock.
	if r.ContentLength != 0 {
		if derr := decodeJSON(r, &req); derr != nil {
			httpx.WriteError(w, r, derr)
			return
		}
	}
	res, err := h.period.Lock(r.Context(), year, month, req.Force, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[periodResponse]{Data: toPeriod(res.Period)})
}

// ReopenPeriod handles POST /payroll-periods/{year}/{month}:reopen — super-admin,
// body {reason}. 409 PERIOD_RUN_POSTED if a run is Posted (no run exists yet).
func (h *PeriodHandler) ReopenPeriod(w http.ResponseWriter, r *http.Request) {
	year, month, err := pathYearMonth(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req reopenRequest
	if r.ContentLength != 0 {
		if derr := decodeJSON(r, &req); derr != nil {
			httpx.WriteError(w, r, derr)
			return
		}
	}
	period, err := h.period.Reopen(r.Context(), year, month, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[periodResponse]{Data: toPeriod(period)})
}

// GetSummary handles GET /payroll-periods/{year}/{month}/summary — the immutable
// per-employee summary set (post-lock; cursor-paged).
func (h *PeriodHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	year, month, err := pathYearMonth(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := r.URL.Query()
	rows, next, err := h.period.Summaries(r.Context(), year, month, q.Get("cursor"), intParam(q.Get("limit")))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]summaryResponse, 0, len(rows))
	for _, s := range rows {
		items = append(items, toSummary(s))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[summaryResponse]{
		Data: items, NextCursor: next, HasMore: next != nil,
	})
}

// --- clarification ---

// RaiseClarification handles POST /clarifications — raise a clarification on a record.
func (h *PeriodHandler) RaiseClarification(w http.ResponseWriter, r *http.Request) {
	var req raiseClarificationRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var periodID *string
	// (period is an optional display hint; the service stores period_id only when a
	// caller resolves it upstream. We keep it nil here — the cockpit passes the id.)
	_ = req.Period

	cr, err := h.clarification.Raise(r.Context(), svc.RaiseParams{
		SourceType: req.SourceType,
		SourceID:   req.SourceID,
		Question:   req.Question,
		PeriodID:   periodID,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/clarifications/"+cr.ID)
	httpx.WriteJSON(w, http.StatusCreated, dataResponse[clarificationResponse]{Data: toClarification(cr)})
}

// AnswerClarification handles POST /clarifications/{id}:answer (scope-checked).
func (h *PeriodHandler) AnswerClarification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req answerClarificationRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	cr, err := h.clarification.Answer(r.Context(), id, req.Answer)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[clarificationResponse]{Data: toClarification(cr)})
}

// ResolveClarification handles POST /clarifications/{id}:resolve (HR closes the loop).
func (h *PeriodHandler) ResolveClarification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cr, err := h.clarification.Resolve(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[clarificationResponse]{Data: toClarification(cr)})
}

// CancelClarification handles POST /clarifications/{id}:cancel (HR closes the loop).
func (h *PeriodHandler) CancelClarification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cr, err := h.clarification.Cancel(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[clarificationResponse]{Data: toClarification(cr)})
}

// --- helpers ---

// pathYearMonth parses the {year}/{month} path params into ints. A malformed value
// is a 400 INVALID_REQUEST.
func pathYearMonth(r *http.Request) (int, int, error) {
	year, yerr := strconv.Atoi(chi.URLParam(r, "year"))
	month, merr := strconv.Atoi(chi.URLParam(r, "month"))
	if yerr != nil || merr != nil {
		return 0, 0, apperr.Invalid(map[string]string{"month": "Tahun/bulan tidak valid."})
	}
	return year, month, nil
}
