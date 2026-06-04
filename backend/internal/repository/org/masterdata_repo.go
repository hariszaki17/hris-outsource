// Package org (repository) — master-data repository for leave types, attendance
// codes, and overtime rules. Sits alongside companies_repo.go and
// serviceline_repo.go in the same package; reuses mapErr and nullStr helpers
// declared in companies_repo.go (no redeclaration).
package org

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
)

// MasterDataRepository is the sqlc-backed implementation of svc.MasterDataRepository.
type MasterDataRepository struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check
var _ svc.MasterDataRepository = (*MasterDataRepository)(nil)

// NewMasterDataRepo returns a new MasterDataRepository backed by pool.
func NewMasterDataRepo(pool *db.Pool) *MasterDataRepository {
	return &MasterDataRepository{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// =============================================================================
// Leave Types
// =============================================================================

// ListLeaveTypes returns a cursor page of leave types matching the filter.
func (r *MasterDataRepository) ListLeaveTypes(ctx context.Context, f domain.LeaveTypeFilter) ([]domain.LeaveType, error) {
	rows, err := r.q.ListLeaveTypes(ctx, sqlcgen.ListLeaveTypesParams{
		Status:          f.Status,
		IsAnnual:        f.IsAnnual,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.LeaveType, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.LeaveType{
			ID:                 row.ID,
			Name:               row.Name,
			Code:               row.Code,
			Description:        row.Description,
			DefaultAnnualQuota: int(row.DefaultAnnualQuota),
			IsAnnual:           row.IsAnnual,
			RequiresDocument:   row.RequiresDocument,
			Color:              row.Color,
			Status:             row.Status,
			CreatedAt:          row.CreatedAt,
			UpdatedAt:          row.UpdatedAt,
		})
	}
	return out, nil
}

// GetLeaveTypeByID fetches a single leave type by SWP-LT id.
func (r *MasterDataRepository) GetLeaveTypeByID(ctx context.Context, id string) (domain.LeaveType, error) {
	row, err := r.q.GetLeaveTypeByID(ctx, id)
	if err != nil {
		return domain.LeaveType{}, mapErr(err)
	}
	return domain.LeaveType{
		ID:                 row.ID,
		Name:               row.Name,
		Code:               row.Code,
		Description:        row.Description,
		DefaultAnnualQuota: int(row.DefaultAnnualQuota),
		IsAnnual:           row.IsAnnual,
		RequiresDocument:   row.RequiresDocument,
		Color:              row.Color,
		Status:             row.Status,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

// CreateLeaveType inserts a new leave type in the given transaction.
func (r *MasterDataRepository) CreateLeaveType(ctx context.Context, tx pgx.Tx, p svc.CreateLeaveTypeParams) (domain.LeaveType, error) {
	row, err := r.q.WithTx(tx).CreateLeaveType(ctx, sqlcgen.CreateLeaveTypeParams{
		Name:               p.Name,
		Code:               p.Code,
		Description:        p.Description,
		DefaultAnnualQuota: int32(p.DefaultAnnualQuota),
		IsAnnual:           p.IsAnnual,
		RequiresDocument:   p.RequiresDocument,
		Color:              p.Color,
	})
	if err != nil {
		return domain.LeaveType{}, mapErr(err)
	}
	return domain.LeaveType{
		ID:                 row.ID,
		Name:               row.Name,
		Code:               row.Code,
		Description:        row.Description,
		DefaultAnnualQuota: int(row.DefaultAnnualQuota),
		IsAnnual:           row.IsAnnual,
		RequiresDocument:   row.RequiresDocument,
		Color:              row.Color,
		Status:             row.Status,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

// UpdateLeaveType patches a leave type's mutable fields in the given transaction.
func (r *MasterDataRepository) UpdateLeaveType(ctx context.Context, tx pgx.Tx, p svc.UpdateLeaveTypeParams) (domain.LeaveType, error) {
	row, err := r.q.WithTx(tx).UpdateLeaveType(ctx, sqlcgen.UpdateLeaveTypeParams{
		ID:                 p.ID,
		Name:               p.Name,
		Code:               p.Code,
		Description:        p.Description,
		DefaultAnnualQuota: int32(p.DefaultAnnualQuota),
		IsAnnual:           p.IsAnnual,
		RequiresDocument:   p.RequiresDocument,
		Color:              p.Color,
	})
	if err != nil {
		return domain.LeaveType{}, mapErr(err)
	}
	return domain.LeaveType{
		ID:                 row.ID,
		Name:               row.Name,
		Code:               row.Code,
		Description:        row.Description,
		DefaultAnnualQuota: int(row.DefaultAnnualQuota),
		IsAnnual:           row.IsAnnual,
		RequiresDocument:   row.RequiresDocument,
		Color:              row.Color,
		Status:             row.Status,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

// SoftDeleteLeaveType sets deleted_at on the leave type (hard soft-delete).
func (r *MasterDataRepository) SoftDeleteLeaveType(ctx context.Context, tx pgx.Tx, id string) error {
	return r.q.WithTx(tx).SoftDeleteLeaveType(ctx, id)
}

// =============================================================================
// Attendance Codes
// =============================================================================

// ListAttendanceCodes returns a cursor page of attendance codes matching the filter.
func (r *MasterDataRepository) ListAttendanceCodes(ctx context.Context, f domain.AttendanceCodeFilter) ([]domain.AttendanceCode, error) {
	rows, err := r.q.ListAttendanceCodes(ctx, sqlcgen.ListAttendanceCodesParams{
		Status:          f.Status,
		IsBillable:      f.IsBillable,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AttendanceCode, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.AttendanceCode{
			ID:                row.ID,
			Code:              row.Code,
			Label:             row.Label,
			Description:       row.Description,
			Color:             row.Color,
			IsWorkday:         row.IsWorkday,
			IsPaid:            row.IsPaid,
			IsBillable:        row.IsBillable,
			NeedsVerification: row.NeedsVerification,
			Status:            row.Status,
			CreatedAt:         row.CreatedAt,
			UpdatedAt:         row.UpdatedAt,
		})
	}
	return out, nil
}

// GetAttendanceCodeByID fetches a single attendance code by SWP-AC id.
func (r *MasterDataRepository) GetAttendanceCodeByID(ctx context.Context, id string) (domain.AttendanceCode, error) {
	row, err := r.q.GetAttendanceCodeByID(ctx, id)
	if err != nil {
		return domain.AttendanceCode{}, mapErr(err)
	}
	return domain.AttendanceCode{
		ID:                row.ID,
		Code:              row.Code,
		Label:             row.Label,
		Description:       row.Description,
		Color:             row.Color,
		IsWorkday:         row.IsWorkday,
		IsPaid:            row.IsPaid,
		IsBillable:        row.IsBillable,
		NeedsVerification: row.NeedsVerification,
		Status:            row.Status,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

// CreateAttendanceCode inserts a new attendance code in the given transaction.
func (r *MasterDataRepository) CreateAttendanceCode(ctx context.Context, tx pgx.Tx, p svc.CreateAttendanceCodeParams) (domain.AttendanceCode, error) {
	row, err := r.q.WithTx(tx).CreateAttendanceCode(ctx, sqlcgen.CreateAttendanceCodeParams{
		Code:              p.Code,
		Label:             p.Label,
		Description:       p.Description,
		Color:             p.Color,
		IsWorkday:         p.IsWorkday,
		IsPaid:            p.IsPaid,
		IsBillable:        p.IsBillable,
		NeedsVerification: p.NeedsVerification,
	})
	if err != nil {
		return domain.AttendanceCode{}, mapErr(err)
	}
	return domain.AttendanceCode{
		ID:                row.ID,
		Code:              row.Code,
		Label:             row.Label,
		Description:       row.Description,
		Color:             row.Color,
		IsWorkday:         row.IsWorkday,
		IsPaid:            row.IsPaid,
		IsBillable:        row.IsBillable,
		NeedsVerification: row.NeedsVerification,
		Status:            row.Status,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

// UpdateAttendanceCode patches an attendance code's mutable fields.
func (r *MasterDataRepository) UpdateAttendanceCode(ctx context.Context, tx pgx.Tx, p svc.UpdateAttendanceCodeParams) (domain.AttendanceCode, error) {
	row, err := r.q.WithTx(tx).UpdateAttendanceCode(ctx, sqlcgen.UpdateAttendanceCodeParams{
		ID:                p.ID,
		Code:              p.Code,
		Label:             p.Label,
		Description:       p.Description,
		Color:             p.Color,
		IsWorkday:         p.IsWorkday,
		IsPaid:            p.IsPaid,
		IsBillable:        p.IsBillable,
		NeedsVerification: p.NeedsVerification,
	})
	if err != nil {
		return domain.AttendanceCode{}, mapErr(err)
	}
	return domain.AttendanceCode{
		ID:                row.ID,
		Code:              row.Code,
		Label:             row.Label,
		Description:       row.Description,
		Color:             row.Color,
		IsWorkday:         row.IsWorkday,
		IsPaid:            row.IsPaid,
		IsBillable:        row.IsBillable,
		NeedsVerification: row.NeedsVerification,
		Status:            row.Status,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

// SoftDeleteAttendanceCode sets deleted_at on the attendance code.
func (r *MasterDataRepository) SoftDeleteAttendanceCode(ctx context.Context, tx pgx.Tx, id string) error {
	return r.q.WithTx(tx).SoftDeleteAttendanceCode(ctx, id)
}

// =============================================================================
// Overtime Rules
// =============================================================================

// ListOvertimeRules returns a cursor page of overtime rules matching the filter.
func (r *MasterDataRepository) ListOvertimeRules(ctx context.Context, f domain.OvertimeRuleFilter) ([]domain.OvertimeRule, error) {
	rows, err := r.q.ListOvertimeRules(ctx, sqlcgen.ListOvertimeRulesParams{
		Status:          f.Status,
		ServiceLineID:   f.ServiceLine,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.OvertimeRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOvertimeRuleDomain(row.ID, row.Name, row.ServiceLineID,
			row.WeekdayRate, row.RestdayRate, row.HolidayRate,
			int(row.MinMinutes), int(row.MaxMinutesPerDay),
			row.PreApprovalRequired, row.Status, row.CreatedAt, row.UpdatedAt))
	}
	return out, nil
}

// GetOvertimeRuleByID fetches a single overtime rule by SWP-OTR id.
func (r *MasterDataRepository) GetOvertimeRuleByID(ctx context.Context, id string) (domain.OvertimeRule, error) {
	row, err := r.q.GetOvertimeRuleByID(ctx, id)
	if err != nil {
		return domain.OvertimeRule{}, mapErr(err)
	}
	return toOvertimeRuleDomain(row.ID, row.Name, row.ServiceLineID,
		row.WeekdayRate, row.RestdayRate, row.HolidayRate,
		int(row.MinMinutes), int(row.MaxMinutesPerDay),
		row.PreApprovalRequired, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// CreateOvertimeRule inserts a new overtime rule in the given transaction.
func (r *MasterDataRepository) CreateOvertimeRule(ctx context.Context, tx pgx.Tx, p svc.CreateOvertimeRuleParams) (domain.OvertimeRule, error) {
	row, err := r.q.WithTx(tx).CreateOvertimeRule(ctx, sqlcgen.CreateOvertimeRuleParams{
		Name:                p.Name,
		ServiceLineID:       p.ServiceLineID,
		WeekdayRate:         float32(p.WeekdayRate),
		RestdayRate:         float32(p.RestdayRate),
		HolidayRate:         float32(p.HolidayRate),
		MinMinutes:          int32(p.MinMinutes),
		MaxMinutesPerDay:    int32(p.MaxMinutesPerDay),
		PreApprovalRequired: p.PreApprovalRequired,
	})
	if err != nil {
		return domain.OvertimeRule{}, mapErr(err)
	}
	return toOvertimeRuleDomain(row.ID, row.Name, row.ServiceLineID,
		row.WeekdayRate, row.RestdayRate, row.HolidayRate,
		int(row.MinMinutes), int(row.MaxMinutesPerDay),
		row.PreApprovalRequired, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// UpdateOvertimeRule patches an overtime rule's mutable fields.
func (r *MasterDataRepository) UpdateOvertimeRule(ctx context.Context, tx pgx.Tx, p svc.UpdateOvertimeRuleParams) (domain.OvertimeRule, error) {
	row, err := r.q.WithTx(tx).UpdateOvertimeRule(ctx, sqlcgen.UpdateOvertimeRuleParams{
		ID:                  p.ID,
		Name:                p.Name,
		ServiceLineID:       p.ServiceLineID,
		WeekdayRate:         float32(p.WeekdayRate),
		RestdayRate:         float32(p.RestdayRate),
		HolidayRate:         float32(p.HolidayRate),
		MinMinutes:          int32(p.MinMinutes),
		MaxMinutesPerDay:    int32(p.MaxMinutesPerDay),
		PreApprovalRequired: p.PreApprovalRequired,
	})
	if err != nil {
		return domain.OvertimeRule{}, mapErr(err)
	}
	return toOvertimeRuleDomain(row.ID, row.Name, row.ServiceLineID,
		row.WeekdayRate, row.RestdayRate, row.HolidayRate,
		int(row.MinMinutes), int(row.MaxMinutesPerDay),
		row.PreApprovalRequired, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// SoftDeleteOvertimeRule sets deleted_at on the overtime rule.
func (r *MasterDataRepository) SoftDeleteOvertimeRule(ctx context.Context, tx pgx.Tx, id string) error {
	return r.q.WithTx(tx).SoftDeleteOvertimeRule(ctx, id)
}

// --- mapping helper ---

func toOvertimeRuleDomain(
	id, name string,
	serviceLineID *string,
	weekdayRate, restdayRate, holidayRate float32,
	minMinutes, maxMinutesPerDay int,
	preApprovalRequired bool,
	status string,
	createdAt, updatedAt time.Time,
) domain.OvertimeRule {
	return domain.OvertimeRule{
		ID:                  id,
		Name:                name,
		ServiceLineID:       serviceLineID,
		WeekdayRate:         float64(weekdayRate),
		RestdayRate:         float64(restdayRate),
		HolidayRate:         float64(holidayRate),
		MinMinutes:          minMinutes,
		MaxMinutesPerDay:    maxMinutesPerDay,
		PreApprovalRequired: preApprovalRequired,
		Status:              status,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
}
