// Package attendance (handler) — the 3 attendance-activity endpoints (F5.8 /
// SWP-ACT-*):
//
//	POST   /attendance/{id}/activities             createAttendanceActivity (scope:self)
//	GET    /attendance/{id}/activities             listAttendanceActivities (attendance-read)
//	DELETE /attendance/{id}/activities/{activityId} deleteAttendanceActivity (scope:self)
//
// Decode → service → httpx.WriteJSON; apperr envelopes flow through httpx.WriteError.
// POST/DELETE are idempotent at the router (Idempotency-Key). The agent is always self
// (employee_id is derived from the token in the service, never the body). Mirrors the
// clock + correction handler shapes; reuses decodeJSON + dataResponse from this package.
package attendance

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// ActivityHandler holds the attendance-activity service.
type ActivityHandler struct {
	activities *svc.ActivityService
}

// NewActivityHandler wires the handler to the activity service.
func NewActivityHandler(a *svc.ActivityService) *ActivityHandler {
	return &ActivityHandler{activities: a}
}

// --- request / response DTOs ---

// createActivityRequest is the openapi CreateActivityRequest.
type createActivityRequest struct {
	Note string `json:"note"`
}

// activityResponse is the openapi AttendanceActivity object.
type activityResponse struct {
	ID           string `json:"id"`
	AttendanceID string `json:"attendance_id"`
	EmployeeID   string `json:"employee_id"`
	Note         string `json:"note"`
	RecordedAt   string `json:"recorded_at"`
	CreatedAt    string `json:"created_at"`
}

func toActivityResponse(a att.AttendanceActivity) activityResponse {
	return activityResponse{
		ID:           a.ID,
		AttendanceID: a.AttendanceID,
		EmployeeID:   a.EmployeeID,
		Note:         a.Note,
		RecordedAt:   a.RecordedAt.UTC().Format(time.RFC3339),
		CreatedAt:    a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// --- handlers ---

// CreateActivity handles POST /attendance/{id}/activities (F5.8, scope:self). Returns
// 201 {data}. Validation (note 1..500), ownership, and open-record guards live in the
// service.
func (h *ActivityHandler) CreateActivity(w http.ResponseWriter, r *http.Request) {
	attendanceID := chi.URLParam(r, "id")
	var req createActivityRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	act, err := h.activities.Create(r.Context(), attendanceID, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, dataResponse[activityResponse]{Data: toActivityResponse(act)})
}

// ListActivities handles GET /attendance/{id}/activities (cursor-paged, chronological).
// Readable by anyone who may read the parent attendance (AA-10); scope enforced in the
// service.
func (h *ActivityHandler) ListActivities(w http.ResponseWriter, r *http.Request) {
	attendanceID := chi.URLParam(r, "id")
	q := r.URL.Query()
	rows, next, hasMore, err := h.activities.List(r.Context(), attendanceID, intParam(q.Get("limit")), q.Get("cursor"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]activityResponse, 0, len(rows))
	for _, a := range rows {
		items = append(items, toActivityResponse(a))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[activityResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// DeleteActivity handles DELETE /attendance/{id}/activities/{activityId} (scope:self).
// Returns 204. Open-record + ownership guards live in the service (already-closed →
// 422 ATTENDANCE_CLOSED; not found / not owner → 404).
func (h *ActivityHandler) DeleteActivity(w http.ResponseWriter, r *http.Request) {
	attendanceID := chi.URLParam(r, "id")
	activityID := chi.URLParam(r, "activityId")
	if err := h.activities.Delete(r.Context(), attendanceID, activityID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
