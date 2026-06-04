package placement

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/placement"
)

// CreateShiftLeaderAssignment handles POST /shift-leader-assignments (201).
func (h *Handler) CreateShiftLeaderAssignment(w http.ResponseWriter, r *http.Request) {
	var req shiftLeaderAssignRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	startDate, err := parseDate(req.StartDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	res, err := h.leaders.CreateAssignment(r.Context(), svc.AssignParams{
		ClientCompanyID: req.ClientCompanyID,
		EmployeeID:      req.EmployeeID,
		StartDate:       startDate,
		Replace:         req.Replace,
		ReplaceReason:   req.ReplaceReason,
		Notes:           req.Notes,
		ActorUserID:     actorPtr(r),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resp := shiftLeaderCreateResponse{Assignment: toShiftLeaderAssignmentResponse(res.Assignment)}
	if res.Replaced != nil {
		rep := toShiftLeaderAssignmentResponse(*res.Replaced)
		resp.ReplacedAssignment = &rep
	}
	w.Header().Set("Location", "/api/v1/shift-leader-assignments/"+res.Assignment.ID)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// ReplaceShiftLeaderAssignment handles POST /shift-leader-assignments/{id}:replace (201).
func (h *Handler) ReplaceShiftLeaderAssignment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req shiftLeaderReplaceRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	startDate, err := parseDate(req.StartDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	res, err := h.leaders.ReplaceAssignment(r.Context(), svc.ReplaceParams{
		AssignmentID:  id,
		NewEmployeeID: req.NewEmployeeID,
		StartDate:     startDate,
		ReplaceReason: req.ReplaceReason,
		Notes:         req.Notes,
		ActorUserID:   actorPtr(r),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resp := shiftLeaderCreateResponse{Assignment: toShiftLeaderAssignmentResponse(res.Assignment)}
	if res.Replaced != nil {
		rep := toShiftLeaderAssignmentResponse(*res.Replaced)
		resp.ReplacedAssignment = &rep
	}
	w.Header().Set("Location", "/api/v1/shift-leader-assignments/"+res.Assignment.ID)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// EndShiftLeaderAssignment handles POST /shift-leader-assignments/{id}:end (200).
func (h *Handler) EndShiftLeaderAssignment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req shiftLeaderEndRequest
	// Body is optional for this endpoint.
	_ = decodeJSON(r, &req)
	ended, err := h.leaders.EndAssignment(r.Context(), svc.EndAssignmentParams{
		AssignmentID: id,
		Reason:       req.Reason,
		ActorUserID:  actorPtr(r),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toShiftLeaderAssignmentResponse(ended))
}
