// Package leave (repository) — implements the E6 leave + quota service ports over
// the 08-01 sqlc queries. Reads on the pool; locked re-checks + writes via
// q.WithTx(tx). Date columns convert pgtype.Date ↔ time.Time (Phase-5/6 pattern);
// jsonb last_adjustment/last_override marshal ↔ []byte; pgx.ErrNoRows →
// domain.ErrNotFound. int32 ↔ int cast at the boundary.
package leave

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
)

// --- pg type helpers ---

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func timeToPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func pgDateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

func i32(n int) int32 { return int32(n) }

func i32ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

// --- jsonb embedded objects ---

func marshalAdjustment(a dom.LeaveQuotaAdjustment) ([]byte, error) { return json.Marshal(a) }

func unmarshalAdjustment(b []byte) *dom.LeaveQuotaAdjustment {
	if len(b) == 0 {
		return nil
	}
	var a dom.LeaveQuotaAdjustment
	if err := json.Unmarshal(b, &a); err != nil {
		return nil
	}
	return &a
}

func marshalOverride(o dom.LeaveQuotaOverride) ([]byte, error) { return json.Marshal(o) }

func unmarshalOverride(b []byte) *dom.LeaveQuotaOverride {
	if len(b) == 0 {
		return nil
	}
	var o dom.LeaveQuotaOverride
	if err := json.Unmarshal(b, &o); err != nil {
		return nil
	}
	return &o
}

// --- leave-request mappers ---

// requestCore is the shared scalar column set across the Get/ForUpdate/List/Update
// rows (all carry the same leave_requests columns). We map field-by-field per row
// type to avoid reflection.

func mapRequestFromList(r sqlcgen.ListLeaveRequestsRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
		ServiceLineID:   r.ServiceLineID,
		LeaveTypeID:     r.LeaveTypeID,
		StartDate:       pgDateToTime(r.StartDate),
		EndDate:         pgDateToTime(r.EndDate),
		DurationDays:    int(r.DurationDays),
		Reason:          r.Reason,
		Notes:           r.Notes,
		Status:          dom.LeaveStatus(r.Status),
		DelegateID:      r.DelegateID,
		DocumentFileID:  r.DocumentFileID,
		Backdated:       r.Backdated,
		ClockInConflict: r.ClockInConflict,
		CreatedBy:       r.CreatedBy,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		EmployeeName:    r.EmployeeName,
		CompanyName:     r.CompanyName,
		LeaveTypeName:   r.LeaveTypeName,
		LeaveTypeCode:   r.LeaveTypeCode,
	}
	lr.Routing = dom.LeaveRouting{NoLeader: r.NoLeader, AssignedLeaderID: r.AssignedLeaderID}
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride, r.BalanceEarmark, r.BalanceAllocation)
	return lr
}

func mapRequestFromGet(r sqlcgen.GetLeaveRequestRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
		ServiceLineID:   r.ServiceLineID,
		LeaveTypeID:     r.LeaveTypeID,
		StartDate:       pgDateToTime(r.StartDate),
		EndDate:         pgDateToTime(r.EndDate),
		DurationDays:    int(r.DurationDays),
		Reason:          r.Reason,
		Notes:           r.Notes,
		Status:          dom.LeaveStatus(r.Status),
		DelegateID:      r.DelegateID,
		DocumentFileID:  r.DocumentFileID,
		Backdated:       r.Backdated,
		ClockInConflict: r.ClockInConflict,
		CreatedBy:       r.CreatedBy,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		EmployeeName:    r.EmployeeName,
		CompanyName:     r.CompanyName,
		LeaveTypeName:   r.LeaveTypeName,
		LeaveTypeCode:   r.LeaveTypeCode,
	}
	lr.Routing = dom.LeaveRouting{NoLeader: r.NoLeader, AssignedLeaderID: r.AssignedLeaderID}
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride, r.BalanceEarmark, r.BalanceAllocation)
	return lr
}

func mapRequestFromForUpdate(r sqlcgen.GetLeaveRequestForUpdateRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
		ServiceLineID:   r.ServiceLineID,
		LeaveTypeID:     r.LeaveTypeID,
		StartDate:       pgDateToTime(r.StartDate),
		EndDate:         pgDateToTime(r.EndDate),
		DurationDays:    int(r.DurationDays),
		Reason:          r.Reason,
		Notes:           r.Notes,
		Status:          dom.LeaveStatus(r.Status),
		DelegateID:      r.DelegateID,
		DocumentFileID:  r.DocumentFileID,
		Backdated:       r.Backdated,
		ClockInConflict: r.ClockInConflict,
		CreatedBy:       r.CreatedBy,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
	lr.Routing = dom.LeaveRouting{NoLeader: r.NoLeader, AssignedLeaderID: r.AssignedLeaderID}
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride, r.BalanceEarmark, r.BalanceAllocation)
	return lr
}

func mapRequestFromUpdate(r sqlcgen.UpdateLeaveRequestStatusRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
		ServiceLineID:   r.ServiceLineID,
		LeaveTypeID:     r.LeaveTypeID,
		StartDate:       pgDateToTime(r.StartDate),
		EndDate:         pgDateToTime(r.EndDate),
		DurationDays:    int(r.DurationDays),
		Reason:          r.Reason,
		Notes:           r.Notes,
		Status:          dom.LeaveStatus(r.Status),
		DelegateID:      r.DelegateID,
		DocumentFileID:  r.DocumentFileID,
		Backdated:       r.Backdated,
		ClockInConflict: r.ClockInConflict,
		CreatedBy:       r.CreatedBy,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
	lr.Routing = dom.LeaveRouting{NoLeader: r.NoLeader, AssignedLeaderID: r.AssignedLeaderID}
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride, r.BalanceEarmark, r.BalanceAllocation)
	return lr
}

func mapRequestFromDates(r sqlcgen.UpdateLeaveRequestDatesRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
		ServiceLineID:   r.ServiceLineID,
		LeaveTypeID:     r.LeaveTypeID,
		StartDate:       pgDateToTime(r.StartDate),
		EndDate:         pgDateToTime(r.EndDate),
		DurationDays:    int(r.DurationDays),
		Reason:          r.Reason,
		Notes:           r.Notes,
		Status:          dom.LeaveStatus(r.Status),
		DelegateID:      r.DelegateID,
		DocumentFileID:  r.DocumentFileID,
		Backdated:       r.Backdated,
		ClockInConflict: r.ClockInConflict,
		CreatedBy:       r.CreatedBy,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
	lr.Routing = dom.LeaveRouting{NoLeader: r.NoLeader, AssignedLeaderID: r.AssignedLeaderID}
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride, r.BalanceEarmark, r.BalanceAllocation)
	return lr
}

func mapBalanceCheck(requested, remaining *int32, requiresOverride *bool, earmark *string, allocation []byte) dom.BalanceCheck {
	bc := dom.BalanceCheck{Earmark: earmark}
	if requested != nil {
		v := int(*requested)
		bc.RequestedDays = &v
	}
	if remaining != nil {
		v := int(*remaining)
		bc.RemainingDaysAtCheck = &v
	}
	if requiresOverride != nil {
		bc.RequiresOverride = *requiresOverride
	}
	bc.Allocation = unmarshalAllocation(allocation)
	return bc
}

// allocLine is the jsonb shape persisted in leave_requests.balance_allocation.
type allocLine struct {
	GrantID   string `json:"grant_id"`
	Days      int    `json:"days"`
	ExpiresAt string `json:"expires_at"` // YYYY-MM-DD
}

// MarshalAllocation serializes the FIFO split for the balance_allocation column.
func MarshalAllocation(alloc []dom.AllocationLine) ([]byte, error) {
	if len(alloc) == 0 {
		return nil, nil
	}
	lines := make([]allocLine, 0, len(alloc))
	for _, a := range alloc {
		lines = append(lines, allocLine{GrantID: a.GrantID, Days: a.Days, ExpiresAt: a.ExpiresAt.Format("2006-01-02")})
	}
	return json.Marshal(lines)
}

func unmarshalAllocation(b []byte) []dom.AllocationLine {
	if len(b) == 0 {
		return nil
	}
	var lines []allocLine
	if err := json.Unmarshal(b, &lines); err != nil {
		return nil
	}
	out := make([]dom.AllocationLine, 0, len(lines))
	for _, l := range lines {
		t, _ := time.Parse("2006-01-02", l.ExpiresAt)
		out = append(out, dom.AllocationLine{GrantID: l.GrantID, Days: l.Days, ExpiresAt: t})
	}
	return out
}

// --- leave-approval mappers ---

func mapApproval(r sqlcgen.LeaveApproval) dom.LeaveApproval {
	return dom.LeaveApproval{
		ID:             r.ID,
		LeaveRequestID: r.LeaveRequestID,
		Stage:          dom.LeaveStage(r.Stage),
		Decision:       dom.LeaveDecision(r.Decision),
		ActorID:        r.ActorID,
		ActorRole:      r.ActorRole,
		DecisionNote:   r.DecisionNote,
		RejectReason:   r.RejectReason,
		IsOverride:     r.IsOverride,
		OverrideReason: r.OverrideReason,
		OccurredAt:     r.OccurredAt,
	}
}

// --- leave-quota mappers ---

func mapQuotaFromList(r sqlcgen.ListLeaveQuotasRow) dom.LeaveQuota {
	return dom.LeaveQuota{
		ID:             r.ID,
		EmployeeID:     r.EmployeeID,
		LeaveTypeID:    r.LeaveTypeID,
		Period:         int(r.Period),
		PeriodStart:    pgDateToTime(r.PeriodStart),
		PeriodEnd:      pgDateToTime(r.PeriodEnd),
		Total:          int(r.Total),
		Used:           int(r.Used),
		Pending:        int(r.Pending),
		IsProrated:     r.IsProrated,
		ProrateMonths:  int(r.ProrateMonths),
		Closed:         r.Closed,
		LastAdjustment: unmarshalAdjustment(r.LastAdjustment),
		LastOverride:   unmarshalOverride(r.LastOverride),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		EmployeeName:   r.EmployeeName,
		LeaveTypeName:  r.LeaveTypeName,
		LeaveTypeCode:  r.LeaveTypeCode,
	}
}

func mapQuotaFromGet(r sqlcgen.GetLeaveQuotaRow) dom.LeaveQuota {
	return dom.LeaveQuota{
		ID:             r.ID,
		EmployeeID:     r.EmployeeID,
		LeaveTypeID:    r.LeaveTypeID,
		Period:         int(r.Period),
		PeriodStart:    pgDateToTime(r.PeriodStart),
		PeriodEnd:      pgDateToTime(r.PeriodEnd),
		Total:          int(r.Total),
		Used:           int(r.Used),
		Pending:        int(r.Pending),
		IsProrated:     r.IsProrated,
		ProrateMonths:  int(r.ProrateMonths),
		Closed:         r.Closed,
		LastAdjustment: unmarshalAdjustment(r.LastAdjustment),
		LastOverride:   unmarshalOverride(r.LastOverride),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		EmployeeName:   r.EmployeeName,
		LeaveTypeName:  r.LeaveTypeName,
		LeaveTypeCode:  r.LeaveTypeCode,
	}
}

// mapQuotaFromModel maps the bare LeaveQuota model (no denormalized names) returned
// by FindQuotaForEmployeeTypePeriod / *ForUpdate / the mutation queries.
func mapQuotaFromModel(r sqlcgen.LeaveQuota) dom.LeaveQuota {
	return dom.LeaveQuota{
		ID:             r.ID,
		EmployeeID:     r.EmployeeID,
		LeaveTypeID:    r.LeaveTypeID,
		Period:         int(r.Period),
		PeriodStart:    pgDateToTime(r.PeriodStart),
		PeriodEnd:      pgDateToTime(r.PeriodEnd),
		Total:          int(r.Total),
		Used:           int(r.Used),
		Pending:        int(r.Pending),
		IsProrated:     r.IsProrated,
		ProrateMonths:  int(r.ProrateMonths),
		Closed:         r.Closed,
		LastAdjustment: unmarshalAdjustment(r.LastAdjustment),
		LastOverride:   unmarshalOverride(r.LastOverride),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// --- leave-grant / consumption mappers (F6.1 grant-lot ledger) ---

func grantFromCreate(r sqlcgen.CreateLeaveGrantRow) dom.LeaveGrant {
	return dom.LeaveGrant{
		ID: r.ID, EmployeeID: r.EmployeeID, Amount: int(r.AmountDays),
		Source: dom.LeaveGrantSource(r.Source), Earmark: r.Earmark, Remark: r.Remark,
		GrantedAt: r.GrantedAt, EffectiveFrom: pgDateToTime(r.EffectiveFrom), ExpiresAt: pgDateToTime(r.ExpiresAt),
		Consumed: int(r.ConsumedDays), Pending: int(r.PendingDays), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func grantFromPatch(r sqlcgen.PatchLeaveGrantRow) dom.LeaveGrant {
	return dom.LeaveGrant{
		ID: r.ID, EmployeeID: r.EmployeeID, Amount: int(r.AmountDays),
		Source: dom.LeaveGrantSource(r.Source), Earmark: r.Earmark, Remark: r.Remark,
		GrantedAt: r.GrantedAt, EffectiveFrom: pgDateToTime(r.EffectiveFrom), ExpiresAt: pgDateToTime(r.ExpiresAt),
		Consumed: int(r.ConsumedDays), Pending: int(r.PendingDays), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func grantFromGet(r sqlcgen.GetLeaveGrantRow) dom.LeaveGrant {
	return dom.LeaveGrant{
		ID: r.ID, EmployeeID: r.EmployeeID, Amount: int(r.AmountDays),
		Source: dom.LeaveGrantSource(r.Source), Earmark: r.Earmark, Remark: r.Remark,
		GrantedAt: r.GrantedAt, EffectiveFrom: pgDateToTime(r.EffectiveFrom), ExpiresAt: pgDateToTime(r.ExpiresAt),
		Consumed: int(r.ConsumedDays), Pending: int(r.PendingDays), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, EmployeeName: r.EmployeeName,
	}
}

func grantFromForUpdate(r sqlcgen.GetLeaveGrantForUpdateRow) dom.LeaveGrant {
	return dom.LeaveGrant{
		ID: r.ID, EmployeeID: r.EmployeeID, Amount: int(r.AmountDays),
		Source: dom.LeaveGrantSource(r.Source), Earmark: r.Earmark, Remark: r.Remark,
		GrantedAt: r.GrantedAt, EffectiveFrom: pgDateToTime(r.EffectiveFrom), ExpiresAt: pgDateToTime(r.ExpiresAt),
		Consumed: int(r.ConsumedDays), Pending: int(r.PendingDays), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func grantFromList(r sqlcgen.ListLeaveGrantsRow) dom.LeaveGrant {
	return dom.LeaveGrant{
		ID: r.ID, EmployeeID: r.EmployeeID, Amount: int(r.AmountDays),
		Source: dom.LeaveGrantSource(r.Source), Earmark: r.Earmark, Remark: r.Remark,
		GrantedAt: r.GrantedAt, EffectiveFrom: pgDateToTime(r.EffectiveFrom), ExpiresAt: pgDateToTime(r.ExpiresAt),
		Consumed: int(r.ConsumedDays), Pending: int(r.PendingDays), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, EmployeeName: r.EmployeeName,
	}
}

func grantFromAlloc(r sqlcgen.GetActiveLotsForAllocationRow) dom.LeaveGrant {
	return dom.LeaveGrant{
		ID: r.ID, EmployeeID: r.EmployeeID, Amount: int(r.AmountDays),
		Source: dom.LeaveGrantSource(r.Source), Earmark: r.Earmark, Remark: r.Remark,
		GrantedAt: r.GrantedAt, EffectiveFrom: pgDateToTime(r.EffectiveFrom), ExpiresAt: pgDateToTime(r.ExpiresAt),
		Consumed: int(r.ConsumedDays), Pending: int(r.PendingDays), CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func consumption(r sqlcgen.LeaveConsumption) dom.LeaveConsumption {
	return dom.LeaveConsumption{
		ID: r.ID, LeaveRequestID: r.LeaveRequestID, GrantID: r.GrantID,
		Days: int(r.Days), CreatedAt: r.CreatedAt,
	}
}

func consumptions(rows []sqlcgen.LeaveConsumption) []dom.LeaveConsumption {
	out := make([]dom.LeaveConsumption, 0, len(rows))
	for _, r := range rows {
		out = append(out, consumption(r))
	}
	return out
}

// --- calendar mappers ---

func mapCalendarEntry(r sqlcgen.ListCalendarEntriesRow) dom.LeaveCalendarEntry {
	return dom.LeaveCalendarEntry{
		LeaveRequestID: r.LeaveRequestID,
		EmployeeID:     r.EmployeeID,
		EmployeeName:   r.EmployeeName,
		CompanyID:      r.CompanyID,
		CompanyName:    r.CompanyName,
		ServiceLine:    r.ServiceLineID,
		LeaveTypeID:    r.LeaveTypeID,
		LeaveTypeCode:  r.LeaveTypeCode,
		LeaveTypeName:  r.LeaveTypeName,
		StartDate:      pgDateToTime(r.StartDate),
		EndDate:        pgDateToTime(r.EndDate),
		Status:         dom.LeaveStatus(r.Status),
		DelegateID:     r.DelegateID,
		DelegateName:   r.DelegateName,
	}
}
