// Package attendance (handler) — agent clock-in/out endpoints (F5.1):
// POST /attendance:clock-in (201) · POST /attendance:clock-out (200). Decode →
// service → httpx.WriteJSON; apperr envelopes flow through httpx.WriteError. Both are
// idempotent at the router (Idempotency-Key). The agent is always self — employee_id
// in the body is ignored (server fills from the token). Mirrors the RejectAttendance
// handler shape; reuses toAttendanceResponse + dataResponse from this package.
package attendance

import (
	"net/http"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// ClockHandler holds the agent clock-in/out service.
type ClockHandler struct {
	clock *svc.ClockService
}

// NewClockHandler wires the handler to the clock service.
func NewClockHandler(c *svc.ClockService) *ClockHandler {
	return &ClockHandler{clock: c}
}

// ClockIn handles POST /attendance:clock-in (201, ClockInResponse).
func (h *ClockHandler) ClockIn(w http.ResponseWriter, r *http.Request) {
	var req clockInRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec, autoClosedPrevious, err := h.clock.ClockIn(r.Context(), svc.ClockInParams{
		Lat:                  req.Lat,
		Lng:                  req.Lng,
		GPSAvailable:         req.GPSAvailable,
		WFO:                  req.wfoOrDefault(),
		PhotoID:              req.PhotoID,
		ForceOutsideGeofence: req.ForceOutsideGeofence,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, clockInResponse{
		Data:               toAttendanceResponse(rec),
		AutoClosedPrevious: autoClosedPrevious,
		Message:            clockInMessage(autoClosedPrevious),
	})
}

// ClockOut handles POST /attendance:clock-out (200, ClockOutResponse).
func (h *ClockHandler) ClockOut(w http.ResponseWriter, r *http.Request) {
	var req clockOutRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec, err := h.clock.ClockOut(r.Context(), svc.ClockOutParams{
		Lat:          req.Lat,
		Lng:          req.Lng,
		GPSAvailable: req.GPSAvailable,
		PhotoID:      req.PhotoID,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[attendanceResponse]{Data: toAttendanceResponse(rec)})
}
