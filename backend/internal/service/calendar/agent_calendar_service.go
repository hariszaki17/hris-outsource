// Package calendar — AgentCalendarService: aggregates schedule + leave + holidays
// + attendance into per-day entries for the agent's month-view calendar (E10 F10.5).
package calendar

import (
	"context"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// AgentCalendarService aggregates agent-facing calendar data.
type AgentCalendarService struct {
	schedule   ScheduleCalendarPort
	leave      LeaveCalendarPort
	holiday    HolidayCalendarPort
	attendance AttendanceCalendarPort
}

// NewAgentCalendarService wires the agent calendar service.
func NewAgentCalendarService(
	schedule ScheduleCalendarPort,
	leave LeaveCalendarPort,
	holiday HolidayCalendarPort,
	attendance AttendanceCalendarPort,
) *AgentCalendarService {
	return &AgentCalendarService{
		schedule:   schedule,
		leave:      leave,
		holiday:    holiday,
		attendance: attendance,
	}
}

// AgentCalendarDay is one day in the agent's month calendar.
type AgentCalendarDay struct {
	Date       string                  `json:"date"`
	Schedule   *ScheduleCalendarEntry  `json:"schedule,omitempty"`
	Leave      *LeaveCalendarEntry     `json:"leave,omitempty"`
	Holiday    *HolidayCalendarEntry   `json:"holiday,omitempty"`
	Attendance *AttendanceCalendarEntry `json:"attendance,omitempty"`
}

// GetMonth returns the agent's calendar for the given year/month. The agent is
// resolved from the auth principal (scope:self). Each day in the month carries
// merged schedule, leave, holiday, and attendance data.
func (s *AgentCalendarService) GetMonth(ctx context.Context, employeeID string, year, month int) ([]AgentCalendarDay, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}
	// Agent scope:self: may only read own calendar.
	if p.Role == auth.RoleAgent {
		if p.EmployeeID == "" || p.EmployeeID != employeeID {
			return nil, apperr.Forbidden()
		}
	}

	firstOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	daysInMonth := lastOfMonth.Day()

	days := make([]AgentCalendarDay, 0, daysInMonth)
	for d := 1; d <= daysInMonth; d++ {
		date := firstOfMonth.AddDate(0, 0, d-1)
		day := AgentCalendarDay{
			Date: date.Format("2006-01-02"),
		}
		day.Schedule, _ = s.schedule.GetScheduleForAgentDate(ctx, employeeID, date)
		day.Leave, _ = s.leave.GetLeaveForAgentDate(ctx, employeeID, date)
		day.Holiday, _ = s.holiday.GetHolidayForDate(ctx, date)
		day.Attendance, _ = s.attendance.GetAttendanceForAgentDate(ctx, employeeID, date)
		days = append(days, day)
	}
	return days, nil
}


