// Package domain contains types for the E2 operational master-data entities:
// leave types, attendance codes, and overtime rules. These feed E5 attendance,
// E6 leave/quotas, and E7 overtime rule evaluation respectively.
package domain

import "time"

// --- Leave Types ---

// LeaveType represents a company-wide leave category (e.g. annual, sick).
type LeaveType struct {
	ID                 string
	Name               string
	Code               string
	Description        string
	DefaultAnnualQuota int
	IsAnnual           bool
	RequiresDocument   bool
	Color              string
	Status             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// LeaveTypeFilter carries the cursor-pagination and filter parameters for
// GET /leave-types.
type LeaveTypeFilter struct {
	Status          *string
	IsAnnual        *bool
	Limit           int
	CursorCreatedAt *time.Time
	CursorID        *string
}

// --- Attendance Codes ---

// AttendanceCode represents a clock-in status code (PRESENT, LATE, etc.).
type AttendanceCode struct {
	ID                string
	Code              string
	Label             string
	Description       string
	Color             string
	IsWorkday         bool
	IsPaid            bool
	IsBillable        bool
	NeedsVerification bool
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// AttendanceCodeFilter carries the cursor-pagination and filter parameters for
// GET /attendance-codes.
type AttendanceCodeFilter struct {
	Status          *string
	IsBillable      *bool
	Limit           int
	CursorCreatedAt *time.Time
	CursorID        *string
}

// --- Overtime Rules ---

// OvertimeRule represents a pay-rate rule for overtime work. ServiceLineID is
// nil for the global default rule.
type OvertimeRule struct {
	ID                  string
	Name                string
	ServiceLineID       *string
	WeekdayRate         float64
	RestdayRate         float64
	HolidayRate         float64
	MinMinutes          int
	MaxMinutesPerDay    int
	PreApprovalRequired bool
	Status              string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// OvertimeRuleFilter carries the cursor-pagination and filter parameters for
// GET /overtime-rules.
type OvertimeRuleFilter struct {
	Status          *string
	ServiceLine     *string
	Limit           int
	CursorCreatedAt *time.Time
	CursorID        *string
}
