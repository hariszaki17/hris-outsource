// Package overtime (handler) — request/response DTOs + snake_case mappers for the
// 13 FE-used E7 endpoints. Required-nullable openapi fields use pointers WITHOUT
// omitempty so they serialize as JSON `null` (decision [07-02]); denormalized
// display names use the nested *Ref shape. Timestamps are UTC RFC3339; dates are
// YYYY-MM-DD. The DTOs match docs/api/E7-overtime/openapi.yaml byte-for-shape.
package overtime

import (
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/overtime"
)

// --- request bodies ---

type noteRequest struct {
	Note string `json:"note"`
}

// overtimeWriteRequest is the createOvertimeRequest body (OvertimeWriteRequest).
// employee_id is optional for an agent caller (server fills from token).
type overtimeWriteRequest struct {
	EmployeeID       string `json:"employee_id"`
	WorkDate         string `json:"work_date"`
	PlannedStartTime string `json:"planned_start_time"`
	PlannedEndTime   string `json:"planned_end_time"`
	Reason           string `json:"reason"`
}

type approveFinalRequest struct {
	Note       string `json:"note"`
	IsOverride bool   `json:"is_override"`
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

type bulkApproveRequest struct {
	IDs  []string `json:"ids"`
	Note string   `json:"note"`
}

type bulkRejectRequest struct {
	IDs    []string `json:"ids"`
	Reason string   `json:"reason"`
}

type holidayWriteRequest struct {
	Name      string `json:"name"`
	Date      string `json:"date"`
	Category  string `json:"category"`
	Recurring *bool  `json:"recurring"`
}

// --- generic envelopes ---

type dataResponse[T any] struct {
	Data T `json:"data"`
}

// --- response: Overtime ---

type employeeRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type companyRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type tierBreakdownResponse struct {
	Tier           string  `json:"tier"`
	Minutes        int     `json:"minutes"`
	Multiplier     float64 `json:"multiplier"`
	OvertimeRuleID *string `json:"overtime_rule_id,omitempty"`
	Supersedes     *string `json:"supersedes"`
}

type calculationResponse struct {
	WorkedMinutes       int                     `json:"worked_minutes"`
	CountedMinutes      int                     `json:"counted_minutes"`
	MinMinutesThreshold int                     `json:"min_minutes_threshold"`
	SkippedTooShort     bool                    `json:"skipped_too_short"`
	TierBreakdown       []tierBreakdownResponse `json:"tier_breakdown"`
}

type approvalResponse struct {
	Level     int          `json:"level"`
	Decision  string       `json:"decision"`
	Approver  *employeeRef `json:"approver,omitempty"`
	Reason    *string      `json:"reason"`
	DecidedAt string       `json:"decided_at"`
}

type overtimeResponse struct {
	ID                   string              `json:"id"`
	Employee             employeeRef         `json:"employee"`
	Company              companyRef          `json:"company"`
	PlacementID          string              `json:"placement_id"`
	AttendanceID         *string             `json:"attendance_id"`
	WorkDate             string              `json:"work_date"`
	PlannedStartTime     *string             `json:"planned_start_time"`
	PlannedEndTime       *string             `json:"planned_end_time"`
	ActualStartTime      *string             `json:"actual_start_time"`
	ActualEndTime        *string             `json:"actual_end_time"`
	CrossMidnight        bool                `json:"cross_midnight"`
	Source               string              `json:"source"`
	Status               string              `json:"status"`
	TierIndicator        string              `json:"tier_indicator"`
	FlaggedNoPreapproval bool                `json:"flagged_no_preapproval"`
	Reason               *string             `json:"reason"`
	Calculation          calculationResponse `json:"calculation"`
	Approvals            []approvalResponse  `json:"approvals,omitempty"`
	CreatedAt            string              `json:"created_at"`
	UpdatedAt            string              `json:"updated_at"`
}

// --- response: Holiday ---

type holidayResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Date            string `json:"date"`
	Category        string `json:"category"`
	Recurring       bool   `json:"recurring"`
	InUseByOvertime bool   `json:"in_use_by_overtime"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// --- response: BulkResult ---

type bulkFailedErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type bulkFailedItem struct {
	ID    string        `json:"id"`
	Error bulkFailedErr `json:"error"`
}

type bulkResultResponse struct {
	Succeeded []string         `json:"succeeded"`
	Failed    []bulkFailedItem `json:"failed"`
}

// --- mappers ---

func dateStr(t time.Time) string { return t.UTC().Format("2006-01-02") }
func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// toOvertimeResponse builds the openapi Overtime shape. includeApprovals controls
// the approvals[] array (present on GET detail, omitted on list per openapi).
func toOvertimeResponse(o dom.Overtime, calc svc.Calculation, includeApprovals bool) overtimeResponse {
	out := overtimeResponse{
		ID:                   o.ID,
		Employee:             employeeRef{ID: o.EmployeeID, Name: derefStr(o.EmployeeName)},
		Company:              companyRef{ID: derefStr(o.CompanyID), Name: derefStr(o.CompanyName)},
		PlacementID:          o.PlacementID,
		AttendanceID:         o.AttendanceID,
		WorkDate:             dateStr(o.WorkDate),
		PlannedStartTime:     o.PlannedStartTime,
		PlannedEndTime:       o.PlannedEndTime,
		ActualStartTime:      o.ActualStartTime,
		ActualEndTime:        o.ActualEndTime,
		CrossMidnight:        o.CrossMidnight,
		Source:               string(o.Source),
		Status:               string(o.Status),
		TierIndicator:        string(o.DayType),
		FlaggedNoPreapproval: o.FlaggedNoPreapproval,
		Reason:               o.Reason,
		Calculation:          toCalculationResponse(calc),
		CreatedAt:            rfc3339(o.CreatedAt),
		UpdatedAt:            rfc3339(o.UpdatedAt),
	}
	if includeApprovals {
		out.Approvals = make([]approvalResponse, 0, len(o.Approvals))
		for _, a := range o.Approvals {
			ar := approvalResponse{
				Level:     a.Level,
				Decision:  a.Decision,
				Reason:    a.Reason,
				DecidedAt: rfc3339(a.DecidedAt),
			}
			if a.ApproverID != nil {
				ar.Approver = &employeeRef{ID: *a.ApproverID, Name: derefStr(a.ApproverName)}
			}
			out.Approvals = append(out.Approvals, ar)
		}
	}
	return out
}

func toCalculationResponse(c svc.Calculation) calculationResponse {
	out := calculationResponse{
		WorkedMinutes:       c.WorkedMinutes,
		CountedMinutes:      c.CountedMinutes,
		MinMinutesThreshold: c.MinMinutesThreshold,
		SkippedTooShort:     c.SkippedTooShort,
		TierBreakdown:       make([]tierBreakdownResponse, 0, len(c.TierBreakdown)),
	}
	for _, t := range c.TierBreakdown {
		tr := tierBreakdownResponse{
			Tier:           string(t.Tier),
			Minutes:        t.Minutes,
			Multiplier:     t.Multiplier,
			OvertimeRuleID: t.OvertimeRule,
		}
		if t.Supersedes != nil {
			s := string(*t.Supersedes)
			tr.Supersedes = &s
		}
		out.TierBreakdown = append(out.TierBreakdown, tr)
	}
	return out
}

func toHolidayResponse(h dom.Holiday) holidayResponse {
	return holidayResponse{
		ID:              h.ID,
		Name:            h.Name,
		Date:            dateStr(h.Date),
		Category:        string(h.Category),
		Recurring:       h.Recurring,
		InUseByOvertime: h.InUseByOvertime,
		CreatedAt:       rfc3339(h.CreatedAt),
		UpdatedAt:       rfc3339(h.UpdatedAt),
	}
}

func toBulkResultResponse(r svc.BulkResult) bulkResultResponse {
	out := bulkResultResponse{
		Succeeded: make([]string, 0, len(r.Succeeded)),
		Failed:    make([]bulkFailedItem, 0, len(r.Failed)),
	}
	out.Succeeded = append(out.Succeeded, r.Succeeded...)
	for _, f := range r.Failed {
		out.Failed = append(out.Failed, bulkFailedItem{
			ID:    f.ID,
			Error: bulkFailedErr{Code: f.Code, Message: f.Message},
		})
	}
	return out
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
