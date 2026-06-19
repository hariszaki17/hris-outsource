// Package calendar — Agent web calendar service ports (E10 F10.5).
// The calendar aggregates schedule + leave + holidays + attendance per day
// for an agent's month view.
package calendar

import (
	"context"
	"time"
)

// ScheduleCalendarPort provides the agent's schedule for a date range.
type ScheduleCalendarPort interface {
	GetScheduleForAgentDate(ctx context.Context, employeeID string, date time.Time) (*ScheduleCalendarEntry, error)
}

// ScheduleCalendarEntry is one day's schedule info for the agent calendar.
type ScheduleCalendarEntry struct {
	ID            string
	ShiftMasterID *string
	ShiftName     *string
	Status        string
	IsDayOff      bool
	StartTime     *string
	EndTime       *string
}

// LeaveCalendarPort provides approved leave days for a date range.
type LeaveCalendarPort interface {
	GetLeaveForAgentDate(ctx context.Context, employeeID string, date time.Time) (*LeaveCalendarEntry, error)
}

// LeaveCalendarEntry is one day's leave info for the agent calendar.
type LeaveCalendarEntry struct {
	LeaveRequestID string
	LeaveTypeName  string
	LeaveTypeCode  string
	StartDate      time.Time
	EndDate        time.Time
	Status         string
}

// HolidayCalendarPort provides holidays in a date range.
type HolidayCalendarPort interface {
	GetHolidayForDate(ctx context.Context, date time.Time) (*HolidayCalendarEntry, error)
}

// HolidayCalendarEntry is one day's holiday info for the agent calendar.
type HolidayCalendarEntry struct {
	ID   string
	Name string
}

// AttendanceCalendarPort provides attendance records for a date range.
type AttendanceCalendarPort interface {
	GetAttendanceForAgentDate(ctx context.Context, employeeID string, date time.Time) (*AttendanceCalendarEntry, error)
}

// AttendanceCalendarEntry is one day's attendance info for the agent calendar.
type AttendanceCalendarEntry struct {
	ID        string
	Status    string
	ClockIn   *string
	ClockOut  *string
}
