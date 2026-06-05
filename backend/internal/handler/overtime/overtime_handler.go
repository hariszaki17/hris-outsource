// Package overtime (handler) — the 9 overtime endpoints:
// GET /overtime · GET /overtime/{id} · POST /overtime/{id}:confirm · :approve-l1 ·
// :approve-final · :reject · :withdraw · POST /overtime:bulk-approve · :bulk-reject.
package overtime

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/overtime"
)

// ListOvertime handles GET /overtime (cursor-paged, filtered, scoped). Writes the
// PageResponse directly ({data, next_cursor, has_more}) — the FE reads query.data.data.
func (h *Handler) ListOvertime(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := svc.OvertimeFilter{
		EmployeeID:           strPtrParam(q.Get("employee_id")),
		CompanyID:            strPtrParam(q.Get("company_id")),
		Status:               strPtrParam(q.Get("status")),
		StatusIn:             csvParam(q.Get("status__in")),
		WorkFrom:             parseDateParam(q.Get("work_date__gte")),
		WorkTo:               parseDateParam(q.Get("work_date__lte")),
		Tier:                 strPtrParam(q.Get("tier")),
		Source:               strPtrParam(q.Get("source")),
		FlaggedNoPreapproval: boolPtrParam(q.Get("flagged_no_preapproval")),
		Limit:                intParam(q.Get("limit")),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		ca, id, err := svc.DecodeOvertimeCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		f.CursorCreated = ca
		f.CursorID = id
	}
	rows, calcs, next, hasMore, err := h.overtime.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]overtimeResponse, 0, len(rows))
	for i, rec := range rows {
		items = append(items, toOvertimeResponse(rec, calcs[i], false))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[overtimeResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// GetOvertime handles GET /overtime/{id} — wraps the single object in {data} (the FE
// detail unwraps {data}); includes the approval timeline + recomputed calculation.
func (h *Handler) GetOvertime(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, calc, err := h.overtime.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[overtimeResponse]{Data: toOvertimeResponse(rec, calc, true)})
}

// Confirm handles POST /overtime/{id}:confirm (optional note).
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req noteRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec, calc, err := h.overtime.Confirm(r.Context(), id, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[overtimeResponse]{Data: toOvertimeResponse(rec, calc, true)})
}

// ApproveL1 handles POST /overtime/{id}:approve-l1 (optional note).
func (h *Handler) ApproveL1(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req noteRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec, calc, err := h.overtime.ApproveL1(r.Context(), id, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[overtimeResponse]{Data: toOvertimeResponse(rec, calc, true)})
}

// ApproveFinal handles POST /overtime/{id}:approve-final (optional {note, is_override}).
func (h *Handler) ApproveFinal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req approveFinalRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec, calc, err := h.overtime.ApproveFinal(r.Context(), id, req.Note, req.IsOverride)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[overtimeResponse]{Data: toOvertimeResponse(rec, calc, true)})
}

// Reject handles POST /overtime/{id}:reject (reason required, minLen 5).
func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req rejectRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len([]rune(req.Reason)) < 5 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."}))
		return
	}
	rec, calc, err := h.overtime.Reject(r.Context(), id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[overtimeResponse]{Data: toOvertimeResponse(rec, calc, true)})
}

// Withdraw handles POST /overtime/{id}:withdraw (no body, 204).
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.overtime.Withdraw(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BulkApprove handles POST /overtime:bulk-approve ({ids, note?}). 200 if ≥1 succeeded
// else 422 (Phase-7 bulk semantics).
func (h *Handler) BulkApprove(w http.ResponseWriter, r *http.Request) {
	var req bulkApproveRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(req.IDs) == 0 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"ids": "Minimal satu lembur."}))
		return
	}
	result, err := h.overtime.BulkApprove(r.Context(), req.IDs, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	writeBulk(w, result)
}

// BulkReject handles POST /overtime:bulk-reject ({ids, reason}). Same partial-success.
func (h *Handler) BulkReject(w http.ResponseWriter, r *http.Request) {
	var req bulkRejectRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(req.IDs) == 0 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"ids": "Minimal satu lembur."}))
		return
	}
	if len([]rune(req.Reason)) < 5 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."}))
		return
	}
	result, err := h.overtime.BulkReject(r.Context(), req.IDs, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	writeBulk(w, result)
}

// writeBulk writes the BulkResult with 200 (≥1 succeeded) or 422 (all failed).
func writeBulk(w http.ResponseWriter, result svc.BulkResult) {
	status := http.StatusOK
	if len(result.Succeeded) == 0 {
		status = http.StatusUnprocessableEntity
	}
	httpx.WriteJSON(w, status, toBulkResultResponse(result))
}
