// Package payroll (repository) — F8.5 sqlc-row -> domain mappers. The period FSM
// queries share an identical column set, so each row type maps via the same shape.
package payroll

import (
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
)

func mapGetPeriod(r sqlcgen.GetPayrollPeriodRow) dom.PayrollPeriod {
	return dom.PayrollPeriod{
		ID: r.ID, Year: int(r.Year), Month: int(r.Month), Status: dom.PeriodStatus(r.Status),
		LockedBy: r.LockedBy, LockedAt: r.LockedAt, ForceLocked: r.ForceLocked,
		ForceLockReason: r.ForceLockReason, ReopenedBy: r.ReopenedBy, ReopenedAt: r.ReopenedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapGetPeriodForUpdate(r sqlcgen.GetPayrollPeriodForUpdateRow) dom.PayrollPeriod {
	return dom.PayrollPeriod{
		ID: r.ID, Year: int(r.Year), Month: int(r.Month), Status: dom.PeriodStatus(r.Status),
		LockedBy: r.LockedBy, LockedAt: r.LockedAt, ForceLocked: r.ForceLocked,
		ForceLockReason: r.ForceLockReason, ReopenedBy: r.ReopenedBy, ReopenedAt: r.ReopenedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapCreatePeriod(r sqlcgen.CreatePayrollPeriodRow) dom.PayrollPeriod {
	return dom.PayrollPeriod{
		ID: r.ID, Year: int(r.Year), Month: int(r.Month), Status: dom.PeriodStatus(r.Status),
		LockedBy: r.LockedBy, LockedAt: r.LockedAt, ForceLocked: r.ForceLocked,
		ForceLockReason: r.ForceLockReason, ReopenedBy: r.ReopenedBy, ReopenedAt: r.ReopenedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapLockPeriod(r sqlcgen.LockPayrollPeriodRow) dom.PayrollPeriod {
	return dom.PayrollPeriod{
		ID: r.ID, Year: int(r.Year), Month: int(r.Month), Status: dom.PeriodStatus(r.Status),
		LockedBy: r.LockedBy, LockedAt: r.LockedAt, ForceLocked: r.ForceLocked,
		ForceLockReason: r.ForceLockReason, ReopenedBy: r.ReopenedBy, ReopenedAt: r.ReopenedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapReopenPeriod(r sqlcgen.ReopenPayrollPeriodRow) dom.PayrollPeriod {
	return dom.PayrollPeriod{
		ID: r.ID, Year: int(r.Year), Month: int(r.Month), Status: dom.PeriodStatus(r.Status),
		LockedBy: r.LockedBy, LockedAt: r.LockedAt, ForceLocked: r.ForceLocked,
		ForceLockReason: r.ForceLockReason, ReopenedBy: r.ReopenedBy, ReopenedAt: r.ReopenedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// --- clarification mappers (identical column set across the queries) ---

func mapCreateClarification(r sqlcgen.CreateClarificationRequestRow) dom.ClarificationRequest {
	return dom.ClarificationRequest{
		ID: r.ID, SourceType: dom.ClarificationSourceType(r.SourceType), SourceID: r.SourceID,
		PeriodID: r.PeriodID, RaisedBy: r.RaisedBy, TargetEmployeeID: r.TargetEmployeeID,
		Question: r.Question, Status: dom.ClarificationStatus(r.Status), Answer: r.Answer,
		AnsweredBy: r.AnsweredBy, AnsweredAt: r.AnsweredAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapGetClarification(r sqlcgen.GetClarificationRequestRow) dom.ClarificationRequest {
	return dom.ClarificationRequest{
		ID: r.ID, SourceType: dom.ClarificationSourceType(r.SourceType), SourceID: r.SourceID,
		PeriodID: r.PeriodID, RaisedBy: r.RaisedBy, TargetEmployeeID: r.TargetEmployeeID,
		Question: r.Question, Status: dom.ClarificationStatus(r.Status), Answer: r.Answer,
		AnsweredBy: r.AnsweredBy, AnsweredAt: r.AnsweredAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapGetClarificationForUpdate(r sqlcgen.GetClarificationRequestForUpdateRow) dom.ClarificationRequest {
	return dom.ClarificationRequest{
		ID: r.ID, SourceType: dom.ClarificationSourceType(r.SourceType), SourceID: r.SourceID,
		PeriodID: r.PeriodID, RaisedBy: r.RaisedBy, TargetEmployeeID: r.TargetEmployeeID,
		Question: r.Question, Status: dom.ClarificationStatus(r.Status), Answer: r.Answer,
		AnsweredBy: r.AnsweredBy, AnsweredAt: r.AnsweredAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapAnswerClarification(r sqlcgen.AnswerClarificationRequestRow) dom.ClarificationRequest {
	return dom.ClarificationRequest{
		ID: r.ID, SourceType: dom.ClarificationSourceType(r.SourceType), SourceID: r.SourceID,
		PeriodID: r.PeriodID, RaisedBy: r.RaisedBy, TargetEmployeeID: r.TargetEmployeeID,
		Question: r.Question, Status: dom.ClarificationStatus(r.Status), Answer: r.Answer,
		AnsweredBy: r.AnsweredBy, AnsweredAt: r.AnsweredAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapSetClarificationStatus(r sqlcgen.SetClarificationStatusRow) dom.ClarificationRequest {
	return dom.ClarificationRequest{
		ID: r.ID, SourceType: dom.ClarificationSourceType(r.SourceType), SourceID: r.SourceID,
		PeriodID: r.PeriodID, RaisedBy: r.RaisedBy, TargetEmployeeID: r.TargetEmployeeID,
		Question: r.Question, Status: dom.ClarificationStatus(r.Status), Answer: r.Answer,
		AnsweredBy: r.AnsweredBy, AnsweredAt: r.AnsweredAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapCreateAdjustment(r sqlcgen.CreatePayrollAdjustmentRow) dom.PayrollAdjustment {
	return dom.PayrollAdjustment{
		ID: r.ID, EmployeeID: r.EmployeeID, SourceType: dom.AdjustmentSourceType(r.SourceType),
		SourceID: r.SourceID, OriginYear: int(r.OriginYear), OriginMonth: int(r.OriginMonth),
		Note: r.Note, Amount: r.Amount, Status: dom.AdjustmentStatus(r.Status),
		AppliedRunID: r.AppliedRunID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
