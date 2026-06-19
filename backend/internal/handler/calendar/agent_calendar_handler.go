// Package calendar — Agent calendar handler (E10 F10.5): GET /me/calendar
// returns the agent's own month-view calendar aggregating schedule, leave,
// holidays, and attendance per day.
package calendar

import (
	"net/http"
	"strconv"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/calendar"
)

// AgentCalendarHandler handles the agent's own calendar endpoint.
type AgentCalendarHandler struct {
	svc *svc.AgentCalendarService
}

// NewAgentCalendarHandler wires the agent calendar handler.
func NewAgentCalendarHandler(s *svc.AgentCalendarService) *AgentCalendarHandler {
	return &AgentCalendarHandler{svc: s}
}

// GetAgentCalendar handles GET /me/calendar?month=&year=. Reads employeeID from
// the auth principal (scope:self), calls the service, and returns the per-day
// merged calendar entries as JSON.
func (h *AgentCalendarHandler) GetAgentCalendar(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.Unauthenticated())
		return
	}
	if p.EmployeeID == "" {
		httpx.WriteError(w, r, apperr.Forbidden())
		return
	}

	q := r.URL.Query()
	year, month := parseYearMonth(q.Get("year"), q.Get("month"))
	days, err := h.svc.GetMonth(r.Context(), p.EmployeeID, year, month)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]agentCalendarDayResponse, 0, len(days))
	for _, d := range days {
		items = append(items, toAgentCalendarDayResponse(d))
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[[]agentCalendarDayResponse]{Data: items})
}

// --- DTOs ---

type dataResponse[T any] struct {
	Data T `json:"data"`
}

type agentCalendarDayResponse struct {
	Date       string                       `json:"date"`
	Schedule   *scheduleCalendarResponse    `json:"schedule,omitempty"`
	Leave      *leaveCalendarResponse       `json:"leave,omitempty"`
	Holiday    *holidayCalendarResponse     `json:"holiday,omitempty"`
	Attendance *attendanceCalendarResponse  `json:"attendance,omitempty"`
}

type scheduleCalendarResponse struct {
	ID            string  `json:"id"`
	ShiftMasterID *string `json:"shift_master_id,omitempty"`
	ShiftName     *string `json:"shift_name,omitempty"`
	Status        string  `json:"status"`
	IsDayOff      bool    `json:"is_day_off"`
	StartTime     *string `json:"start_time,omitempty"`
	EndTime       *string `json:"end_time,omitempty"`
}

type leaveCalendarResponse struct {
	LeaveRequestID string `json:"leave_request_id"`
	LeaveTypeName  string `json:"leave_type_name"`
	LeaveTypeCode  string `json:"leave_type_code"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	Status         string `json:"status"`
}

type holidayCalendarResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type attendanceCalendarResponse struct {
	ID       string  `json:"id"`
	Status   string  `json:"status"`
	ClockIn  *string `json:"clock_in,omitempty"`
	ClockOut *string `json:"clock_out,omitempty"`
}

func toAgentCalendarDayResponse(d svc.AgentCalendarDay) agentCalendarDayResponse {
	out := agentCalendarDayResponse{Date: d.Date}
	if d.Schedule != nil {
		out.Schedule = &scheduleCalendarResponse{
			ID:            d.Schedule.ID,
			ShiftMasterID: d.Schedule.ShiftMasterID,
			ShiftName:     d.Schedule.ShiftName,
			Status:        d.Schedule.Status,
			IsDayOff:      d.Schedule.IsDayOff,
			StartTime:     d.Schedule.StartTime,
			EndTime:       d.Schedule.EndTime,
		}
	}
	if d.Leave != nil {
		out.Leave = &leaveCalendarResponse{
			LeaveRequestID: d.Leave.LeaveRequestID,
			LeaveTypeName:  d.Leave.LeaveTypeName,
			LeaveTypeCode:  d.Leave.LeaveTypeCode,
			StartDate:      d.Leave.StartDate.Format("2006-01-02"),
			EndDate:        d.Leave.EndDate.Format("2006-01-02"),
			Status:         d.Leave.Status,
		}
	}
	if d.Holiday != nil {
		out.Holiday = &holidayCalendarResponse{
			ID:   d.Holiday.ID,
			Name: d.Holiday.Name,
		}
	}
	if d.Attendance != nil {
		out.Attendance = &attendanceCalendarResponse{
			ID:       d.Attendance.ID,
			Status:   d.Attendance.Status,
			ClockIn:  d.Attendance.ClockIn,
			ClockOut: d.Attendance.ClockOut,
		}
	}
	return out
}

// --- helpers ---

func parseYearMonth(yearRaw, monthRaw string) (int, int) {
	now := time.Now()
	y := now.Year()
	m := int(now.Month())
	if v, err := strconv.Atoi(yearRaw); err == nil && v >= 2000 && v <= 2100 {
		y = v
	}
	if v, err := strconv.Atoi(monthRaw); err == nil && v >= 1 && v <= 12 {
		m = v
	}
	return y, m
}
