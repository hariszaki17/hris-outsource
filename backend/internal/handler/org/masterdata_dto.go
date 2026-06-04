package org

import (
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// =============================================================================
// Leave Type DTOs
// =============================================================================

// createLeaveTypeRequest is the POST /leave-types body (LeaveTypeWriteRequest).
type createLeaveTypeRequest struct {
	Name               *string `json:"name"`
	Code               *string `json:"code"`
	Description        *string `json:"description"`
	DefaultAnnualQuota *int    `json:"default_annual_quota"`
	IsAnnual           *bool   `json:"is_annual"`
	RequiresDocument   *bool   `json:"requires_document"`
	Color              *string `json:"color"`
}

// updateLeaveTypeRequest is the PATCH /leave-types/{id} body.
type updateLeaveTypeRequest struct {
	Name               *string `json:"name"`
	Code               *string `json:"code"`
	Description        *string `json:"description"`
	DefaultAnnualQuota *int    `json:"default_annual_quota"`
	IsAnnual           *bool   `json:"is_annual"`
	RequiresDocument   *bool   `json:"requires_document"`
	Color              *string `json:"color"`
}

// leaveTypeResponse matches the LeaveType schema in E2 openapi.yaml (line 3299+).
// Status is uppercased (ACTIVE/INACTIVE) at this boundary.
type leaveTypeResponse struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Code               string `json:"code"`
	Description        string `json:"description"`
	DefaultAnnualQuota int    `json:"default_annual_quota"`
	IsAnnual           bool   `json:"is_annual"`
	RequiresDocument   bool   `json:"requires_document"`
	Color              string `json:"color"`
	Status             string `json:"status"` // ACTIVE | INACTIVE
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

func toLeaveTypeResponse(lt domain.LeaveType) leaveTypeResponse {
	return leaveTypeResponse{
		ID:                 lt.ID,
		Name:               lt.Name,
		Code:               lt.Code,
		Description:        lt.Description,
		DefaultAnnualQuota: lt.DefaultAnnualQuota,
		IsAnnual:           lt.IsAnnual,
		RequiresDocument:   lt.RequiresDocument,
		Color:              lt.Color,
		Status:             strings.ToUpper(lt.Status),
		CreatedAt:          lt.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          lt.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// =============================================================================
// Attendance Code DTOs
// =============================================================================

// createAttendanceCodeRequest is the POST /attendance-codes body.
type createAttendanceCodeRequest struct {
	Code              *string `json:"code"`
	Label             *string `json:"label"`
	Description       *string `json:"description"`
	Color             *string `json:"color"`
	IsWorkday         *bool   `json:"is_workday"`
	IsPaid            *bool   `json:"is_paid"`
	IsBillable        *bool   `json:"is_billable"`
	NeedsVerification *bool   `json:"needs_verification"`
}

// updateAttendanceCodeRequest is the PATCH /attendance-codes/{id} body.
type updateAttendanceCodeRequest struct {
	Code              *string `json:"code"`
	Label             *string `json:"label"`
	Description       *string `json:"description"`
	Color             *string `json:"color"`
	IsWorkday         *bool   `json:"is_workday"`
	IsPaid            *bool   `json:"is_paid"`
	IsBillable        *bool   `json:"is_billable"`
	NeedsVerification *bool   `json:"needs_verification"`
}

// attendanceCodeResponse matches the AttendanceCode schema (openapi line 3342+).
// Status is uppercased at this boundary.
type attendanceCodeResponse struct {
	ID                string `json:"id"`
	Code              string `json:"code"`
	Label             string `json:"label"`
	Description       string `json:"description"`
	Color             string `json:"color"`
	IsWorkday         bool   `json:"is_workday"`
	IsPaid            bool   `json:"is_paid"`
	IsBillable        bool   `json:"is_billable"`
	NeedsVerification bool   `json:"needs_verification"`
	Status            string `json:"status"` // ACTIVE | INACTIVE
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

func toAttendanceCodeResponse(ac domain.AttendanceCode) attendanceCodeResponse {
	return attendanceCodeResponse{
		ID:                ac.ID,
		Code:              ac.Code,
		Label:             ac.Label,
		Description:       ac.Description,
		Color:             ac.Color,
		IsWorkday:         ac.IsWorkday,
		IsPaid:            ac.IsPaid,
		IsBillable:        ac.IsBillable,
		NeedsVerification: ac.NeedsVerification,
		Status:            strings.ToUpper(ac.Status),
		CreatedAt:         ac.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         ac.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// =============================================================================
// Overtime Rule DTOs
// =============================================================================

// createOvertimeRuleRequest is the POST /overtime-rules body (OvertimeRuleWriteRequest).
// Rates and min_minutes are optional (service applies defaults).
type createOvertimeRuleRequest struct {
	Name                *string  `json:"name"`
	ServiceLineID       *string  `json:"service_line_id"`
	WeekdayRate         *float64 `json:"weekday_rate"`
	RestdayRate         *float64 `json:"restday_rate"`
	HolidayRate         *float64 `json:"holiday_rate"`
	MinMinutes          *int     `json:"min_minutes"`
	MaxMinutesPerDay    *int     `json:"max_minutes_per_day"`
	PreApprovalRequired *bool    `json:"pre_approval_required"`
}

// updateOvertimeRuleRequest is the PATCH /overtime-rules/{id} body.
type updateOvertimeRuleRequest struct {
	Name                *string  `json:"name"`
	ServiceLineID       *string  `json:"service_line_id"`
	WeekdayRate         *float64 `json:"weekday_rate"`
	RestdayRate         *float64 `json:"restday_rate"`
	HolidayRate         *float64 `json:"holiday_rate"`
	MinMinutes          *int     `json:"min_minutes"`
	MaxMinutesPerDay    *int     `json:"max_minutes_per_day"`
	PreApprovalRequired *bool    `json:"pre_approval_required"`
}

// overtimeRuleResponse matches the OvertimeRule schema (openapi line 3391+).
// Rates are float64 so JSON round-trips cleanly (avoids float32 noise).
// service_line_id is nullable (omitempty would hide it — use explicit *string).
// Status is uppercased at this boundary.
type overtimeRuleResponse struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	ServiceLineID       *string `json:"service_line_id"` // nullable
	WeekdayRate         float64 `json:"weekday_rate"`
	RestdayRate         float64 `json:"restday_rate"`
	HolidayRate         float64 `json:"holiday_rate"`
	MinMinutes          int     `json:"min_minutes"`
	MaxMinutesPerDay    int     `json:"max_minutes_per_day"`
	PreApprovalRequired bool    `json:"pre_approval_required"`
	Status              string  `json:"status"` // ACTIVE | INACTIVE
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func toOvertimeRuleResponse(or domain.OvertimeRule) overtimeRuleResponse {
	return overtimeRuleResponse{
		ID:                  or.ID,
		Name:                or.Name,
		ServiceLineID:       or.ServiceLineID,
		WeekdayRate:         or.WeekdayRate,
		RestdayRate:         or.RestdayRate,
		HolidayRate:         or.HolidayRate,
		MinMinutes:          or.MinMinutes,
		MaxMinutesPerDay:    or.MaxMinutesPerDay,
		PreApprovalRequired: or.PreApprovalRequired,
		Status:              strings.ToUpper(or.Status),
		CreatedAt:           or.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           or.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
