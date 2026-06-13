// Package leave (handler) — request/response DTOs + snake_case mappers for the 10
// FE-used E6 endpoints. Required-nullable openapi fields use pointers WITHOUT
// omitempty so they serialize as JSON `null` (not absent); denormalized display
// names use omitempty. Timestamps are UTC RFC3339; dates are YYYY-MM-DD. The DTOs
// match docs/api/E6-leave/openapi.yaml byte-for-shape.
package leave

import (
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// --- request bodies ---

// leaveRequestWriteRequest is the POST /leave-requests body (openapi
// LeaveRequestWriteRequest). employee_id is optional (an agent omits it → server fills
// from the token); duration_days is NEVER read (server-computed). submit defaults true
// (*bool: nil ⇒ submit) for the create-and-submit single-call path.
type leaveRequestWriteRequest struct {
	LeaveTypeID    string  `json:"leave_type_id"`
	StartDate      string  `json:"start_date"`
	EndDate        string  `json:"end_date"`
	Reason         string  `json:"reason"`
	EmployeeID     *string `json:"employee_id"`
	DelegateID     *string `json:"delegate_id"`
	DocumentFileID *string `json:"document_file_id"`
	Submit         *bool   `json:"submit"`
}

type noteRequest struct {
	Note string `json:"note"`
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

type overrideRequest struct {
	OverrideReason string `json:"override_reason"`
}

type shortenRequest struct {
	NewEndDate string `json:"new_end_date"`
	Reason     string `json:"reason"`
}

// --- response: LeaveRequest ---

type routingResponse struct {
	NoLeader           bool    `json:"no_leader"`
	AssignedLeaderID   *string `json:"assigned_leader_id"`
	AssignedLeaderName *string `json:"assigned_leader_name"`
}

type balanceCheckResponse struct {
	RequestedDays        *int    `json:"requested_days,omitempty"`
	RemainingDaysAtCheck *int    `json:"remaining_days_at_check,omitempty"`
	CheckedAt            *string `json:"checked_at,omitempty"`
	RequiresOverride     bool    `json:"requires_override"`
}

type timelineEntryResponse struct {
	Stage          string  `json:"stage"`
	Status         string  `json:"status"`
	ActorID        *string `json:"actor_id"`
	ActorName      *string `json:"actor_name"`
	ActorRole      *string `json:"actor_role"`
	Decision       *string `json:"decision"`
	DecisionNote   *string `json:"decision_note"`
	RejectReason   *string `json:"reject_reason"`
	Override       bool    `json:"override"`
	OverrideReason *string `json:"override_reason"`
	OccurredAt     string  `json:"occurred_at"`
}

type scheduleImpactResponse struct {
	ScheduleID      string `json:"schedule_id"`
	Date            string `json:"date"`
	PriorStatus     string `json:"prior_status"`
	NewStatus       string `json:"new_status"`
	ClockInConflict bool   `json:"clock_in_conflict"`
}

type leaveRequestResponse struct {
	ID                  string  `json:"id"`
	EmployeeID          string  `json:"employee_id"`
	EmployeeName        *string `json:"employee_name,omitempty"`
	EmployeeCompanyID   *string `json:"employee_company_id,omitempty"`
	EmployeeCompanyName *string `json:"employee_company_name,omitempty"`
	LeaveTypeID         string  `json:"leave_type_id"`
	LeaveTypeName       *string `json:"leave_type_name,omitempty"`
	StartDate           string  `json:"start_date"`
	EndDate             string  `json:"end_date"`
	DurationDays        int     `json:"duration_days"`
	Reason              *string `json:"reason"`
	Notes               *string `json:"notes"`
	Status              string  `json:"status"`
	DelegateID          *string `json:"delegate_id"`
	DocumentFileID      *string `json:"document_file_id"`
	Backdated           bool    `json:"backdated"`
	ClockInConflict     bool    `json:"clock_in_conflict"`

	Routing        routingResponse          `json:"routing"`
	BalanceCheck   balanceCheckResponse     `json:"balance_check"`
	Timeline       []timelineEntryResponse  `json:"timeline"`
	ScheduleImpact []scheduleImpactResponse `json:"schedule_impact"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}


// --- response: LeaveQuota ---

type quotaAdjustmentResponse struct {
	Delta      int    `json:"delta"`
	Reason     string `json:"reason"`
	AdjustedBy string `json:"adjusted_by"`
	AdjustedAt string `json:"adjusted_at"`
}

type quotaOverrideResponse struct {
	LeaveRequestID string `json:"leave_request_id"`
	OverrideReason string `json:"override_reason"`
	OverriddenBy   string `json:"overridden_by"`
	OverriddenAt   string `json:"overridden_at"`
}

// leaveQuotaResponse is the per-type window (openapi LeaveQuota) returned by the HR
// adjust-entitled action. remaining = entitled - used - pending (derived).
type leaveQuotaResponse struct {
	ID             string                   `json:"id"`
	EmployeeID     string                   `json:"employee_id"`
	EmployeeName   *string                  `json:"employee_name,omitempty"`
	LeaveTypeID    string                   `json:"leave_type_id"`
	LeaveTypeName  *string                  `json:"leave_type_name,omitempty"`
	PeriodKey      string                   `json:"period_key"`
	EntitledDays   int                      `json:"entitled_days"`
	UsedDays       int                      `json:"used_days"`
	PendingDays    int                      `json:"pending_days"`
	Remaining      int                      `json:"remaining"`
	Source         string                   `json:"source"`
	Remark         string                   `json:"remark,omitempty"`
	ExpiresAt      *string                  `json:"expires_at,omitempty"`
	LastAdjustment *quotaAdjustmentResponse `json:"last_adjustment"`
	LastOverride   *quotaOverrideResponse   `json:"last_override"`
	CreatedAt      string                   `json:"created_at"`
	UpdatedAt      string                   `json:"updated_at"`
}

// --- response: calendar ---

type calendarEntryResponse struct {
	LeaveRequestID string  `json:"leave_request_id"`
	EmployeeID     string  `json:"employee_id"`
	EmployeeName   *string `json:"employee_name,omitempty"`
	CompanyID      *string `json:"company_id,omitempty"`
	CompanyName    *string `json:"company_name,omitempty"`
	LeaveTypeID    string  `json:"leave_type_id"`
	LeaveTypeCode  *string `json:"leave_type_code,omitempty"`
	StartDate      string  `json:"start_date"`
	EndDate        string  `json:"end_date"`
	Status         string  `json:"status"`
	DelegateID     *string `json:"delegate_id"`
	DelegateName   *string `json:"delegate_name"`
}

type calendarResponse struct {
	Period      int                     `json:"period"`
	Month       *int                    `json:"month"`
	ShowPending bool                    `json:"show_pending"`
	Entries     []calendarEntryResponse `json:"entries"`
}

// --- generic envelopes ---

type dataResponse[T any] struct {
	Data T `json:"data"`
}

// --- mappers ---

func dateStr(t time.Time) string { return t.UTC().Format("2006-01-02") }
func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func toBalanceCheckResponse(bc dom.BalanceCheck) balanceCheckResponse {
	out := balanceCheckResponse{
		RequestedDays:        bc.RequestedDays,
		RemainingDaysAtCheck: bc.RemainingDaysAtCheck,
		RequiresOverride:     bc.RequiresOverride,
	}
	if bc.CheckedAt != nil {
		s := rfc3339(*bc.CheckedAt)
		out.CheckedAt = &s
	}
	return out
}

func toLeaveRequestResponse(r dom.LeaveRequest) leaveRequestResponse {
	out := leaveRequestResponse{
		ID:                  r.ID,
		EmployeeID:          r.EmployeeID,
		EmployeeName:        r.EmployeeName,
		EmployeeCompanyID:   r.CompanyID,
		EmployeeCompanyName: r.CompanyName,
		LeaveTypeID:         r.LeaveTypeID,
		LeaveTypeName:       r.LeaveTypeName,
		StartDate:           dateStr(r.StartDate),
		EndDate:             dateStr(r.EndDate),
		DurationDays:        r.DurationDays,
		Reason:              r.Reason,
		Notes:               r.Notes,
		Status:              string(r.Status),
		DelegateID:          r.DelegateID,
		DocumentFileID:      r.DocumentFileID,
		Backdated:           r.Backdated,
		ClockInConflict:     r.ClockInConflict,
		Routing: routingResponse{
			NoLeader:           r.Routing.NoLeader,
			AssignedLeaderID:   r.Routing.AssignedLeaderID,
			AssignedLeaderName: r.Routing.AssignedLeader,
		},
		BalanceCheck:   toBalanceCheckResponse(r.BalanceCheck),
		Timeline:       make([]timelineEntryResponse, 0, len(r.Timeline)),
		ScheduleImpact: make([]scheduleImpactResponse, 0, len(r.ScheduleImpact)),
		CreatedAt:      rfc3339(r.CreatedAt),
		UpdatedAt:      rfc3339(r.UpdatedAt),
	}
	for _, t := range r.Timeline {
		var dec *string
		if t.Decision != nil {
			d := string(*t.Decision)
			dec = &d
		}
		out.Timeline = append(out.Timeline, timelineEntryResponse{
			Stage:          string(t.Stage),
			Status:         string(t.Status),
			ActorID:        t.ActorID,
			ActorName:      t.ActorName,
			ActorRole:      t.ActorRole,
			Decision:       dec,
			DecisionNote:   t.DecisionNote,
			RejectReason:   t.RejectReason,
			Override:       t.Override,
			OverrideReason: t.OverrideReason,
			OccurredAt:     rfc3339(t.OccurredAt),
		})
	}
	for _, si := range r.ScheduleImpact {
		out.ScheduleImpact = append(out.ScheduleImpact, scheduleImpactResponse{
			ScheduleID:      si.ScheduleID,
			Date:            si.Date,
			PriorStatus:     si.PriorStatus,
			NewStatus:       si.NewStatus,
			ClockInConflict: si.ClockInConflict,
		})
	}
	return out
}

func toLeaveQuotaResponse(q dom.LeaveQuota) leaveQuotaResponse {
	out := leaveQuotaResponse{
		ID:            q.ID,
		EmployeeID:    q.EmployeeID,
		EmployeeName:  q.EmployeeName,
		LeaveTypeID:   q.LeaveTypeID,
		LeaveTypeName: q.LeaveTypeName,
		PeriodKey:     q.PeriodKey,
		EntitledDays:  q.EntitledDays,
		UsedDays:      q.UsedDays,
		PendingDays:   q.PendingDays,
		Remaining:     q.RemainingPerType(),
		Source:        string(q.Source),
		Remark:        q.Remark,
		CreatedAt:     rfc3339(q.CreatedAt),
		UpdatedAt:     rfc3339(q.UpdatedAt),
	}
	if q.ExpiresAt != nil {
		s := dateStr(*q.ExpiresAt)
		out.ExpiresAt = &s
	}
	if q.LastAdjustment != nil {
		out.LastAdjustment = &quotaAdjustmentResponse{
			Delta:      q.LastAdjustment.Delta,
			Reason:     q.LastAdjustment.Reason,
			AdjustedBy: q.LastAdjustment.AdjustedBy,
			AdjustedAt: rfc3339(q.LastAdjustment.AdjustedAt),
		}
	}
	if q.LastOverride != nil {
		out.LastOverride = &quotaOverrideResponse{
			LeaveRequestID: q.LastOverride.LeaveRequestID,
			OverrideReason: q.LastOverride.OverrideReason,
			OverriddenBy:   q.LastOverride.OverriddenBy,
			OverriddenAt:   rfc3339(q.LastOverride.OverriddenAt),
		}
	}
	return out
}

func toCalendarResponse(r svc.CalendarResult) calendarResponse {
	out := calendarResponse{
		Period:      r.Period,
		Month:       r.Month,
		ShowPending: r.ShowPending,
		Entries:     make([]calendarEntryResponse, 0, len(r.Entries)),
	}
	for _, e := range r.Entries {
		out.Entries = append(out.Entries, calendarEntryResponse{
			LeaveRequestID: e.LeaveRequestID,
			EmployeeID:     e.EmployeeID,
			EmployeeName:   e.EmployeeName,
			CompanyID:      e.CompanyID,
			CompanyName:    e.CompanyName,
			LeaveTypeID:    e.LeaveTypeID,
			LeaveTypeCode:  e.LeaveTypeCode,
			StartDate:      dateStr(e.StartDate),
			EndDate:        dateStr(e.EndDate),
			Status:         string(e.Status),
			DelegateID:     e.DelegateID,
			DelegateName:   e.DelegateName,
		})
	}
	return out
}

// typeBalanceResponse is one row in GET /leave-balances/by-employee/{id}/types
// (openapi LeaveTypeBalance) — per-type current-window balance (F6.5, 2026-06-12).
type typeBalanceResponse struct {
	LeaveTypeID      string  `json:"leave_type_id"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	CapBasis         string  `json:"cap_basis"`
	CapValue         *int    `json:"cap_value,omitempty"`
	CapUnit          string  `json:"cap_unit"`
	Paid             bool    `json:"paid"`
	Gender           string  `json:"gender"`
	RequiresDocument bool    `json:"requires_document"`
	Color            string  `json:"color"`
	HasWindow        bool    `json:"has_window"`
	EntitledDays     *int    `json:"entitled_days,omitempty"`
	UsedDays         int     `json:"used_days"`
	PendingDays      int     `json:"pending_days"`
	RemainingDays    *int    `json:"remaining_days,omitempty"`
	ExpiresAt        *string `json:"expires_at,omitempty"`
}

func toTypeBalanceResponse(b dom.TypeBalance) typeBalanceResponse {
	out := typeBalanceResponse{
		LeaveTypeID:      b.LeaveTypeID,
		Code:             b.Code,
		Name:             b.Name,
		CapBasis:         string(b.CapBasis),
		CapValue:         b.CapValue,
		CapUnit:          b.CapUnit,
		Paid:             b.Paid,
		Gender:           b.Gender,
		RequiresDocument: b.RequiresDocument,
		Color:            b.Color,
		HasWindow:        b.HasWindow,
		UsedDays:         b.Used,
		PendingDays:      b.Pending,
		RemainingDays:    b.Remaining(),
	}
	if b.HasWindow {
		e := b.Entitled
		out.EntitledDays = &e
	} else if b.CapValue != nil {
		e := *b.CapValue
		out.EntitledDays = &e
	}
	if b.ExpiresAt != nil {
		s := b.ExpiresAt.Format("2006-01-02")
		out.ExpiresAt = &s
	}
	return out
}

// adjustEntitledRequest is the body of POST /leave-quotas:adjust-entitled (per-type).
type adjustEntitledRequest struct {
	EmployeeID  string `json:"employee_id"`
	LeaveTypeID string `json:"leave_type_id"`
	StartDate   string `json:"start_date"`
	Delta       int    `json:"delta"`
	Reason      string `json:"reason"`
}
