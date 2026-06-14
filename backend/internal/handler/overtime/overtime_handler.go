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

// CreateOvertime handles POST /overtime (F7.2 / createOvertimeRequest). Decodes the
// OvertimeWriteRequest, resolves placement/leave/day-type in the service, and writes
// 201 {data: Overtime} with a Location header. Agent callers omit employee_id (filled
// from the token); a mismatched employee_id → 403 in the service.
func (h *Handler) CreateOvertime(w http.ResponseWriter, r *http.Request) {
	var req overtimeWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	workDate := parseDateParam(req.WorkDate)
	if workDate == nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"work_date": "Wajib diisi (format YYYY-MM-DD)."}))
		return
	}
	rec, calc, err := h.overtime.Create(r.Context(), svc.CreateOvertimeInput{
		EmployeeID:       req.EmployeeID,
		WorkDate:         *workDate,
		PlannedStartTime: req.PlannedStartTime,
		PlannedEndTime:   req.PlannedEndTime,
		Reason:           req.Reason,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/overtime/"+rec.ID)
	httpx.WriteJSON(w, http.StatusCreated, dataResponse[overtimeResponse]{Data: toOvertimeResponse(rec, calc, true)})
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

// Withdraw handles POST /overtime/{id}:withdraw (no body, 204).
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.overtime.Withdraw(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

