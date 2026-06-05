// Package payroll (handler) — the 4 read/note endpoints: GET /payslips,
// GET /payslips/{id}, GET /payslips/{id}/audit-notes, POST /payslips/{id}/audit-notes.
package payroll

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// ListPayslips handles GET /payslips (cursor-paged, paid_on DESC). Writes the page
// envelope at the top level ({data, next_cursor, has_more}); attaches
// meta.code MISSING_PAYROLL_HISTORY when the service flags zero rows.
func (h *Handler) ListPayslips(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := svc.PayslipFilter{
		EmployeeID: strPtrParam(q.Get("employee_id")),
		Year:       intPtrParam(q.Get("year")),
		Status:     strPtrParam(q.Get("status")),
		Limit:      intParam(q.Get("limit")),
	}
	if period := q.Get("period"); period != "" {
		y, m := splitPeriod(period)
		if y != nil {
			f.Year = y
		}
		f.Month = m
	}
	if cursor := q.Get("cursor"); cursor != "" {
		paidOn, id, err := svc.DecodePayslipCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		f.CursorPaidOn = paidOn
		f.CursorID = id
	}

	rows, next, hasMore, missingHistory, err := h.payslip.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]payslipResponse, 0, len(rows))
	for _, p := range rows {
		items = append(items, toPayslipSummary(p))
	}

	resp := payslipPageResponse{Data: items, NextCursor: next, HasMore: hasMore}
	if missingHistory {
		resp.Meta = &pageMeta{Code: "MISSING_PAYROLL_HISTORY"}
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// GetPayslip handles GET /payslips/{id} — wraps the full breakdown in {data}.
func (h *Handler) GetPayslip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.payslip.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[payslipResponse]{Data: toPayslipDetail(p)})
}

// ListAuditNotes handles GET /payslips/{id}/audit-notes (chronological).
func (h *Handler) ListAuditNotes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cursor := r.URL.Query().Get("cursor")

	rows, next, hasMore, err := h.payslip.ListAuditNotes(r.Context(), id, cursor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]auditNoteResponse, 0, len(rows))
	for _, n := range rows {
		items = append(items, toAuditNote(n))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[auditNoteResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// CreateAuditNote handles POST /payslips/{id}/audit-notes (201 + Location).
func (h *Handler) CreateAuditNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req createAuditNoteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	note, err := h.payslip.CreateAuditNote(r.Context(), id, req.Text)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/payslips/"+id+"/audit-notes")
	httpx.WriteJSON(w, http.StatusCreated, toAuditNote(note))
}
