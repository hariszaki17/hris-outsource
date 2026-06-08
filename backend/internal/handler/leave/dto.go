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

type adjustRequest struct {
	Delta  int    `json:"delta"`
	Reason string `json:"reason"`
}

type bulkGrantRequest struct {
	LeaveTypeID            string   `json:"leave_type_id"`
	Period                 int      `json:"period"`
	DefaultEntitlementDays *int     `json:"default_entitlement_days"`
	EmployeeIDs            []string `json:"employee_ids"`
	ProRate                bool     `json:"pro_rate"`
	Preview                bool     `json:"preview"`
}

// --- response: LeaveRequest ---

type routingResponse struct {
	NoLeader           bool    `json:"no_leader"`
	AssignedLeaderID   *string `json:"assigned_leader_id"`
	AssignedLeaderName *string `json:"assigned_leader_name"`
}

type allocationLineResponse struct {
	GrantID   string `json:"grant_id"`
	Days      int    `json:"days"`
	ExpiresAt string `json:"expires_at"`
}

type balanceCheckResponse struct {
	RequestedDays        *int                     `json:"requested_days,omitempty"`
	RemainingDaysAtCheck *int                     `json:"remaining_days_at_check,omitempty"`
	Earmark              *string                  `json:"earmark"`
	Allocation           []allocationLineResponse `json:"allocation,omitempty"`
	CheckedAt            *string                  `json:"checked_at,omitempty"`
	RequiresOverride     bool                     `json:"requires_override"`
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
	EmployeeServiceLine *string `json:"employee_service_line,omitempty"`
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

// --- grant-lot ledger (F6.1) ---

type grantWriteRequest struct {
	EmployeeID    string  `json:"employee_id"`
	AmountDays    int     `json:"amount_days"`
	ExpiresAt     string  `json:"expires_at"`
	Source        string  `json:"source"`
	Earmark       *string `json:"earmark"`
	Remark        string  `json:"remark"`
	EffectiveFrom *string `json:"effective_from"`
}

type grantPatchRequest struct {
	AmountDays *int    `json:"amount_days"`
	ExpiresAt  *string `json:"expires_at"`
	Earmark    *string `json:"earmark"`
	SetEarmark bool    `json:"-"`
	Remark     string  `json:"remark"`
}

type leaveConsumptionResponse struct {
	ID             string `json:"id"`
	LeaveRequestID string `json:"leave_request_id"`
	GrantID        string `json:"grant_id"`
	Days           int    `json:"days"`
	CreatedAt      string `json:"created_at"`
}

type leaveGrantResponse struct {
	ID            string                     `json:"id"`
	EmployeeID    string                     `json:"employee_id"`
	EmployeeName  *string                    `json:"employee_name,omitempty"`
	AmountDays    int                        `json:"amount_days"`
	Source        string                     `json:"source"`
	Earmark       *string                    `json:"earmark"`
	Remark        *string                    `json:"remark,omitempty"`
	GrantedAt     string                     `json:"granted_at"`
	EffectiveFrom string                     `json:"effective_from"`
	ExpiresAt     string                     `json:"expires_at"`
	ConsumedDays  int                        `json:"consumed_days"`
	PendingDays   int                        `json:"pending_days"`
	RemainingDays int                        `json:"remaining_days"`
	IsActive      bool                       `json:"is_active"`
	Consumptions  []leaveConsumptionResponse `json:"consumptions,omitempty"`
	CreatedBy     *string                    `json:"created_by,omitempty"`
	CreatedAt     string                     `json:"created_at"`
	UpdatedAt     string                     `json:"updated_at"`
}

type earmarkLineResponse struct {
	GrantID       string `json:"grant_id"`
	Earmark       string `json:"earmark"`
	Source        string `json:"source,omitempty"`
	RemainingDays int    `json:"remaining_days"`
	ExpiresAt     string `json:"expires_at"`
}

type leaveBalanceResponse struct {
	EmployeeID    string                `json:"employee_id"`
	EmployeeName  *string               `json:"employee_name,omitempty"`
	PoolRemaining int                   `json:"pool_remaining"`
	PendingTotal  int                   `json:"pending_total"`
	NextExpiry    *string               `json:"next_expiry"`
	Earmarked     []earmarkLineResponse `json:"earmarked"`
	AllLots       []leaveGrantResponse  `json:"all_lots,omitempty"`
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

type leaveQuotaResponse struct {
	ID             string                   `json:"id"`
	EmployeeID     string                   `json:"employee_id"`
	EmployeeName   *string                  `json:"employee_name,omitempty"`
	LeaveTypeID    string                   `json:"leave_type_id"`
	LeaveTypeName  *string                  `json:"leave_type_name,omitempty"`
	Period         int                      `json:"period"`
	PeriodStart    string                   `json:"period_start"`
	PeriodEnd      string                   `json:"period_end"`
	Total          int                      `json:"total"`
	Used           int                      `json:"used"`
	Pending        int                      `json:"pending"`
	Remaining      int                      `json:"remaining"`
	IsProrated     bool                     `json:"is_prorated"`
	ProrateMonths  int                      `json:"prorate_months"`
	Closed         bool                     `json:"closed"`
	LastAdjustment *quotaAdjustmentResponse `json:"last_adjustment"`
	LastOverride   *quotaOverrideResponse   `json:"last_override"`
	CreatedAt      string                   `json:"created_at"`
	UpdatedAt      string                   `json:"updated_at"`
}

// --- response: bulk-grant ---

type bulkGrantSucceededRow struct {
	EmployeeID    string  `json:"employee_id"`
	EmployeeName  *string `json:"employee_name,omitempty"`
	QuotaID       *string `json:"quota_id"`
	Total         int     `json:"total"`
	IsProrated    bool    `json:"is_prorated"`
	ProrateMonths *int    `json:"prorate_months"`
}

type bulkGrantFailedRow struct {
	EmployeeID string         `json:"employee_id"`
	Error      bulkGrantError `json:"error"`
}

type bulkGrantError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type bulkGrantResponse struct {
	Preview       bool                    `json:"preview"`
	TotalAffected int                     `json:"total_affected"`
	Succeeded     []bulkGrantSucceededRow `json:"succeeded"`
	Failed        []bulkGrantFailedRow    `json:"failed"`
}

// --- response: calendar ---

type calendarEntryResponse struct {
	LeaveRequestID string  `json:"leave_request_id"`
	EmployeeID     string  `json:"employee_id"`
	EmployeeName   *string `json:"employee_name,omitempty"`
	CompanyID      *string `json:"company_id,omitempty"`
	CompanyName    *string `json:"company_name,omitempty"`
	ServiceLine    *string `json:"service_line,omitempty"`
	LeaveTypeID    string  `json:"leave_type_id"`
	LeaveTypeCode  *string `json:"leave_type_code,omitempty"`
	StartDate      string  `json:"start_date"`
	EndDate        string  `json:"end_date"`
	Status         string  `json:"status"`
	DelegateID     *string `json:"delegate_id"`
	DelegateName   *string `json:"delegate_name"`
}

type calendarClashResponse struct {
	Date            string   `json:"date"`
	CompanyID       string   `json:"company_id"`
	CompanyName     *string  `json:"company_name,omitempty"`
	ServiceLine     string   `json:"service_line"`
	AgentCount      int      `json:"agent_count"`
	LeaveRequestIDs []string `json:"leave_request_ids"`
}

type calendarResponse struct {
	Period      int                     `json:"period"`
	Month       *int                    `json:"month"`
	ShowPending bool                    `json:"show_pending"`
	Entries     []calendarEntryResponse `json:"entries"`
	Clashes     []calendarClashResponse `json:"clashes"`
}

// --- generic envelopes ---

type dataResponse[T any] struct {
	Data T `json:"data"`
}

// --- mappers ---

func dateStr(t time.Time) string { return t.UTC().Format("2006-01-02") }
func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func toLeaveGrantResponse(g dom.LeaveGrant, now time.Time) leaveGrantResponse {
	out := leaveGrantResponse{
		ID:            g.ID,
		EmployeeID:    g.EmployeeID,
		EmployeeName:  g.EmployeeName,
		AmountDays:    g.Amount,
		Source:        string(g.Source),
		Earmark:       g.Earmark,
		Remark:        g.Remark,
		GrantedAt:     dateStr(g.GrantedAt),
		EffectiveFrom: dateStr(g.EffectiveFrom),
		ExpiresAt:     dateStr(g.ExpiresAt),
		ConsumedDays:  g.Consumed,
		PendingDays:   g.Pending,
		RemainingDays: g.Remaining(),
		IsActive:      g.IsActive(now),
		CreatedBy:     g.CreatedBy,
		CreatedAt:     rfc3339(g.CreatedAt),
		UpdatedAt:     rfc3339(g.UpdatedAt),
	}
	for _, c := range g.Consumptions {
		out.Consumptions = append(out.Consumptions, leaveConsumptionResponse{
			ID: c.ID, LeaveRequestID: c.LeaveRequestID, GrantID: c.GrantID,
			Days: c.Days, CreatedAt: rfc3339(c.CreatedAt),
		})
	}
	return out
}

func toLeaveBalanceResponse(b dom.LeaveBalance, now time.Time) leaveBalanceResponse {
	out := leaveBalanceResponse{
		EmployeeID:    b.EmployeeID,
		EmployeeName:  b.EmployeeName,
		PoolRemaining: b.PoolRemaining,
		PendingTotal:  b.PendingTotal,
		Earmarked:     make([]earmarkLineResponse, 0, len(b.Earmarked)),
	}
	if b.NextExpiry != nil {
		s := dateStr(*b.NextExpiry)
		out.NextExpiry = &s
	}
	for _, e := range b.Earmarked {
		out.Earmarked = append(out.Earmarked, earmarkLineResponse{
			GrantID: e.GrantID, Earmark: e.Earmark, Source: string(e.Source),
			RemainingDays: e.Remaining, ExpiresAt: dateStr(e.ExpiresAt),
		})
	}
	for _, lot := range b.AllLots {
		out.AllLots = append(out.AllLots, toLeaveGrantResponse(lot, now))
	}
	return out
}

func toBalanceCheckResponse(bc dom.BalanceCheck) balanceCheckResponse {
	out := balanceCheckResponse{
		RequestedDays:        bc.RequestedDays,
		RemainingDaysAtCheck: bc.RemainingDaysAtCheck,
		Earmark:              bc.Earmark,
		RequiresOverride:     bc.RequiresOverride,
	}
	for _, a := range bc.Allocation {
		out.Allocation = append(out.Allocation, allocationLineResponse{
			GrantID:   a.GrantID,
			Days:      a.Days,
			ExpiresAt: a.ExpiresAt.Format("2006-01-02"),
		})
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
		EmployeeServiceLine: r.ServiceLineID,
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
		Period:        q.Period,
		PeriodStart:   dateStr(q.PeriodStart),
		PeriodEnd:     dateStr(q.PeriodEnd),
		Total:         q.Total,
		Used:          q.Used,
		Pending:       q.Pending,
		Remaining:     q.Remaining(),
		IsProrated:    q.IsProrated,
		ProrateMonths: q.ProrateMonths,
		Closed:        q.Closed,
		CreatedAt:     rfc3339(q.CreatedAt),
		UpdatedAt:     rfc3339(q.UpdatedAt),
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

func toBulkGrantResponse(r svc.BulkGrantResult) bulkGrantResponse {
	out := bulkGrantResponse{
		Preview:       r.Preview,
		TotalAffected: r.TotalAffected,
		Succeeded:     make([]bulkGrantSucceededRow, 0, len(r.Succeeded)),
		Failed:        make([]bulkGrantFailedRow, 0, len(r.Failed)),
	}
	for _, s := range r.Succeeded {
		out.Succeeded = append(out.Succeeded, bulkGrantSucceededRow{
			EmployeeID:    s.EmployeeID,
			EmployeeName:  s.EmployeeName,
			QuotaID:       s.QuotaID,
			Total:         s.Total,
			IsProrated:    s.IsProrated,
			ProrateMonths: s.ProrateMonths,
		})
	}
	for _, f := range r.Failed {
		out.Failed = append(out.Failed, bulkGrantFailedRow{
			EmployeeID: f.EmployeeID,
			Error:      bulkGrantError{Code: f.Code, Message: f.Message},
		})
	}
	return out
}

func toCalendarResponse(r svc.CalendarResult) calendarResponse {
	out := calendarResponse{
		Period:      r.Period,
		Month:       r.Month,
		ShowPending: r.ShowPending,
		Entries:     make([]calendarEntryResponse, 0, len(r.Entries)),
		Clashes:     make([]calendarClashResponse, 0, len(r.Clashes)),
	}
	for _, e := range r.Entries {
		out.Entries = append(out.Entries, calendarEntryResponse{
			LeaveRequestID: e.LeaveRequestID,
			EmployeeID:     e.EmployeeID,
			EmployeeName:   e.EmployeeName,
			CompanyID:      e.CompanyID,
			CompanyName:    e.CompanyName,
			ServiceLine:    e.ServiceLine,
			LeaveTypeID:    e.LeaveTypeID,
			LeaveTypeCode:  e.LeaveTypeCode,
			StartDate:      dateStr(e.StartDate),
			EndDate:        dateStr(e.EndDate),
			Status:         string(e.Status),
			DelegateID:     e.DelegateID,
			DelegateName:   e.DelegateName,
		})
	}
	for _, c := range r.Clashes {
		out.Clashes = append(out.Clashes, calendarClashResponse{
			Date:            c.Date,
			CompanyID:       c.CompanyID,
			CompanyName:     c.CompanyName,
			ServiceLine:     c.ServiceLine,
			AgentCount:      c.AgentCount,
			LeaveRequestIDs: c.LeaveRequestIDs,
		})
	}
	return out
}
