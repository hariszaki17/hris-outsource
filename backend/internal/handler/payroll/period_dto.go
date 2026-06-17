// Package payroll (handler) — F8.5 Payroll Period Close DTOs. Wire shapes for the
// §9 endpoints. snake_case JSON, *string for nullable. Mirrors the existing payroll
// dto.go conventions (dataResponse[T] for single objects, httpx.PageResponse[T] for
// lists).
package payroll

import (
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
)

// --- period ---

type blockerCountsResponse struct {
	Attendance int `json:"attendance"`
	Overtime   int `json:"overtime"`
	Leave      int `json:"leave"`
	Total      int `json:"total"`
}

type periodResponse struct {
	ID              string     `json:"id"`
	Year            int        `json:"year"`
	Month           int        `json:"month"`
	Period          string     `json:"period"` // YYYY-MM
	Status          string     `json:"status"`
	LockedBy        *string    `json:"locked_by"`
	LockedAt        *time.Time `json:"locked_at"`
	ForceLocked     bool       `json:"force_locked"`
	ForceLockReason *string    `json:"force_lock_reason"`
	ReopenedBy      *string    `json:"reopened_by"`
	ReopenedAt      *time.Time `json:"reopened_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Blockers is the live FIELD per-tab tally (nil on the lock/reopen action
	// responses, which echo the period only).
	Blockers *blockerCountsResponse `json:"blockers,omitempty"`

	// Lockable is true when the period is OPEN and all blocker counts are zero
	// (i.e. an ordinary HR user can lock without a force flag). The field is
	// omitempty so it is absent on lock/reopen action responses where Blockers
	// is also nil. Matches the OpenAPI spec PayrollPeriod.lockable field.
	Lockable *bool `json:"lockable,omitempty"`

	// OpenClarifications is the count of OPEN clarifications on this period
	// (the "awaiting reply" badge, PC-13 / F8.5). Omitted on lock/reopen action
	// responses where Blockers is also nil.
	OpenClarifications *int `json:"open_clarifications,omitempty"`
}

func toPeriod(p dom.PayrollPeriod) periodResponse {
	return periodResponse{
		ID: p.ID, Year: p.Year, Month: p.Month,
		Period:          periodStr(p.Year, p.Month),
		Status:          string(p.Status),
		LockedBy:        p.LockedBy,
		LockedAt:        p.LockedAt,
		ForceLocked:     p.ForceLocked,
		ForceLockReason: p.ForceLockReason,
		ReopenedBy:      p.ReopenedBy,
		ReopenedAt:      p.ReopenedAt,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
	}
}

func toBlockerCounts(b dom.PeriodBlockerCounts) blockerCountsResponse {
	return blockerCountsResponse{
		Attendance: b.Attendance, Overtime: b.Overtime, Leave: b.Leave, Total: b.Total(),
	}
}

// --- completeness ---

type completenessResponse struct {
	EmployeeID         string  `json:"employee_id"`
	EmployeeName       *string `json:"employee_name"`
	EmployeeType       string  `json:"employee_type"`
	Gates              bool    `json:"gates"` // FIELD gates the lock; INTERNAL is informational (PC-5)
	Expected           int     `json:"expected"`
	Recorded           int     `json:"recorded"`
	Clean              int     `json:"clean"`
	OnLeave            int     `json:"on_leave"`
	CoverageGap        int     `json:"coverage_gap"`
	AttendanceBlockers int     `json:"attendance_blockers"`
	PendingOvertime    int     `json:"pending_overtime"`
	PendingLeave       int     `json:"pending_leave"`
	Blockers           int     `json:"blockers"` // in-scope total (0 for INTERNAL, PC-5)
}

func toCompleteness(c dom.EmployeeCompleteness) completenessResponse {
	return completenessResponse{
		EmployeeID:         c.EmployeeID,
		EmployeeName:       c.EmployeeName,
		EmployeeType:       c.EmployeeType,
		Gates:              c.EmployeeType == "FIELD",
		Expected:           c.Expected,
		Recorded:           c.Recorded,
		Clean:              c.Clean,
		OnLeave:            c.OnLeave,
		CoverageGap:        c.CoverageGap,
		AttendanceBlockers: c.AttendanceBlockers,
		PendingOvertime:    c.PendingOT,
		PendingLeave:       c.PendingLeave,
		Blockers:           c.InScopeBlockerCount(),
	}
}

// --- summary ---

type summaryResponse struct {
	ID                 string    `json:"id"`
	EmployeeID         string    `json:"employee_id"`
	EmployeeName       *string   `json:"employee_name"`
	PayableDays        int       `json:"payable_days"`
	PresentDays        int       `json:"present_days"`
	LateDays           int       `json:"late_days"`
	AbsentDays         int       `json:"absent_days"`
	NoShiftPayableDays int       `json:"no_shift_payable_days"`
	WorkedMinutes      int       `json:"worked_minutes"`
	ApprovedOTMinutes  int       `json:"approved_ot_minutes"`
	PaidLeaveDays      int       `json:"paid_leave_days"`
	UnpaidLeaveDays    int       `json:"unpaid_leave_days"`
	CreatedAt          time.Time `json:"created_at"`
}

func toSummary(s dom.PeriodEmployeeSummary) summaryResponse {
	return summaryResponse{
		ID: s.ID, EmployeeID: s.EmployeeID, EmployeeName: s.EmployeeName,
		PayableDays: s.PayableDays, PresentDays: s.PresentDays, LateDays: s.LateDays,
		AbsentDays: s.AbsentDays, NoShiftPayableDays: s.NoShiftPayableDays,
		WorkedMinutes: s.WorkedMinutes, ApprovedOTMinutes: s.ApprovedOTMinutes,
		PaidLeaveDays: s.PaidLeaveDays, UnpaidLeaveDays: s.UnpaidLeaveDays,
		CreatedAt: s.CreatedAt,
	}
}

// --- lock / reopen request bodies ---

type lockRequest struct {
	Force  bool   `json:"force"`
	Reason string `json:"reason"`
}

type reopenRequest struct {
	Reason string `json:"reason"`
}

// --- clarification ---

type clarificationResponse struct {
	ID               string     `json:"id"`
	SourceType       string     `json:"source_type"`
	SourceID         string     `json:"source_id"`
	PeriodID         *string    `json:"period_id"`
	RaisedBy         *string    `json:"raised_by"`
	TargetEmployeeID string     `json:"target_employee_id"`
	Question         string     `json:"question"`
	Status           string     `json:"status"`
	Answer           *string    `json:"answer"`
	AnsweredBy       *string    `json:"answered_by"`
	AnsweredAt       *time.Time `json:"answered_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func toClarification(c dom.ClarificationRequest) clarificationResponse {
	return clarificationResponse{
		ID: c.ID, SourceType: string(c.SourceType), SourceID: c.SourceID,
		PeriodID: c.PeriodID, RaisedBy: c.RaisedBy, TargetEmployeeID: c.TargetEmployeeID,
		Question: c.Question, Status: string(c.Status), Answer: c.Answer,
		AnsweredBy: c.AnsweredBy, AnsweredAt: c.AnsweredAt,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

type raiseClarificationRequest struct {
	SourceType string  `json:"source_type"`
	SourceID   string  `json:"source_id"`
	Question   string  `json:"question"`
	Period     *string `json:"period"` // optional YYYY-MM the record surfaced in
}

type answerClarificationRequest struct {
	Answer string `json:"answer"`
}

// periodStr builds YYYY-MM (handler-local; mirrors the service periodString).
func periodStr(year, month int) string {
	b := []byte("0000-00")
	b[0] = byte('0' + (year/1000)%10)
	b[1] = byte('0' + (year/100)%10)
	b[2] = byte('0' + (year/10)%10)
	b[3] = byte('0' + year%10)
	b[5] = byte('0' + (month/10)%10)
	b[6] = byte('0' + month%10)
	return string(b)
}
