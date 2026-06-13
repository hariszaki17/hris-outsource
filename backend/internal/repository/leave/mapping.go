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
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride)
	return lr
}

func mapRequestFromGet(r sqlcgen.GetLeaveRequestRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
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
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride)
	return lr
}

func mapRequestFromForUpdate(r sqlcgen.GetLeaveRequestForUpdateRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
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
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride)
	return lr
}

func mapRequestFromCreate(r sqlcgen.CreateLeaveRequestRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
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
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride)
	return lr
}

func mapRequestFromUpdate(r sqlcgen.UpdateLeaveRequestStatusRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
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
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride)
	return lr
}

func mapRequestFromDates(r sqlcgen.UpdateLeaveRequestDatesRow) dom.LeaveRequest {
	lr := dom.LeaveRequest{
		ID:              r.ID,
		EmployeeID:      r.EmployeeID,
		PlacementID:     r.PlacementID,
		CompanyID:       r.CompanyID,
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
	lr.BalanceCheck = mapBalanceCheck(r.BalanceRequestedDays, r.BalanceRemainingAtCheck, r.BalanceRequiresOverride)
	return lr
}

func mapBalanceCheck(requested, remaining *int32, requiresOverride *bool) dom.BalanceCheck {
	bc := dom.BalanceCheck{}
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
	return bc
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

// mapQuotaFromModel maps the bare per-type LeaveQuota window model returned by the
// QuotaMeter store queries (ResolveQuotaWindow / OpenQuotaWindow / the mutations).
func mapQuotaFromModel(r sqlcgen.LeaveQuota) dom.LeaveQuota {
	return dom.LeaveQuota{
		ID:             r.ID,
		EmployeeID:     r.EmployeeID,
		LeaveTypeID:    r.LeaveTypeID,
		PeriodKey:      derefStr(r.PeriodKey),
		EntitledDays:   int(r.EntitledDays),
		UsedDays:       int(r.UsedDays),
		PendingDays:    int(r.PendingDays),
		Source:         dom.QuotaSource(r.Source),
		Remark:         r.Remark,
		ExpiresAt:      pgDatePtr(r.ExpiresAt),
		CreatedBy:      r.CreatedBy,
		LastAdjustment: unmarshalAdjustment(r.LastAdjustment),
		LastOverride:   unmarshalOverride(r.LastOverride),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// derefStr returns the pointed-to string or "" when nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// pgDatePtr converts a nullable pgtype.Date to *time.Time (nil when not valid).
func pgDatePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

// --- calendar mappers ---

func mapCalendarEntry(r sqlcgen.ListCalendarEntriesRow) dom.LeaveCalendarEntry {
	return dom.LeaveCalendarEntry{
		LeaveRequestID: r.LeaveRequestID,
		EmployeeID:     r.EmployeeID,
		EmployeeName:   r.EmployeeName,
		CompanyID:      r.CompanyID,
		CompanyName:    r.CompanyName,
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
