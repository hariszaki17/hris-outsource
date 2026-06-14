// Package org — MasterDataService implements list/create/update/soft-delete for
// the three operational master-data entities: leave types, attendance codes, and
// overtime rules. Follows the same pattern as Service (companies) and
// ServiceLineService in this package — a separate struct, reuses package-level
// helpers (pageCursor, ClampLimit, isUniqueViolation, TxRunner, Clock).
package org

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// MasterDataRepository is the data port consumed by MasterDataService.
// Implemented by repository/org.MasterDataRepository over sqlc.
type MasterDataRepository interface {
	// Leave types
	ListLeaveTypes(ctx context.Context, f domain.LeaveTypeFilter) ([]domain.LeaveType, error)
	GetLeaveTypeByID(ctx context.Context, id string) (domain.LeaveType, error)
	CreateLeaveType(ctx context.Context, tx pgx.Tx, p CreateLeaveTypeParams) (domain.LeaveType, error)
	UpdateLeaveType(ctx context.Context, tx pgx.Tx, p UpdateLeaveTypeParams) (domain.LeaveType, error)
	SoftDeleteLeaveType(ctx context.Context, tx pgx.Tx, id string) error
	// Attendance codes
	ListAttendanceCodes(ctx context.Context, f domain.AttendanceCodeFilter) ([]domain.AttendanceCode, error)
	GetAttendanceCodeByID(ctx context.Context, id string) (domain.AttendanceCode, error)
	CreateAttendanceCode(ctx context.Context, tx pgx.Tx, p CreateAttendanceCodeParams) (domain.AttendanceCode, error)
	UpdateAttendanceCode(ctx context.Context, tx pgx.Tx, p UpdateAttendanceCodeParams) (domain.AttendanceCode, error)
	SoftDeleteAttendanceCode(ctx context.Context, tx pgx.Tx, id string) error
	// Overtime rules
	ListOvertimeRules(ctx context.Context, f domain.OvertimeRuleFilter) ([]domain.OvertimeRule, error)
	GetOvertimeRuleByID(ctx context.Context, id string) (domain.OvertimeRule, error)
	CreateOvertimeRule(ctx context.Context, tx pgx.Tx, p CreateOvertimeRuleParams) (domain.OvertimeRule, error)
	UpdateOvertimeRule(ctx context.Context, tx pgx.Tx, p UpdateOvertimeRuleParams) (domain.OvertimeRule, error)
	SoftDeleteOvertimeRule(ctx context.Context, tx pgx.Tx, id string) error
}

// --- Leave Type params ---

// CreateLeaveTypeParams carries the fields for inserting a new leave type.
type CreateLeaveTypeParams struct {
	Name               string
	Code               string
	Description        string
	DefaultAnnualQuota int
	IsAnnual           bool
	RequiresDocument   bool
	Color              string
}

// UpdateLeaveTypeParams carries the fields for updating a leave type.
type UpdateLeaveTypeParams struct {
	ID                 string
	Name               string
	Code               string
	Description        string
	DefaultAnnualQuota int
	IsAnnual           bool
	RequiresDocument   bool
	Color              string
}

// --- Attendance Code params ---

// CreateAttendanceCodeParams carries the fields for inserting a new attendance code.
type CreateAttendanceCodeParams struct {
	Code              string
	Label             string
	Description       string
	Color             string
	IsWorkday         bool
	IsPaid            bool
	IsBillable        bool
	NeedsVerification bool
}

// UpdateAttendanceCodeParams carries the fields for updating an attendance code.
type UpdateAttendanceCodeParams struct {
	ID                string
	Code              string
	Label             string
	Description       string
	Color             string
	IsWorkday         bool
	IsPaid            bool
	IsBillable        bool
	NeedsVerification bool
}

// --- Overtime Rule params ---

// CreateOvertimeRuleParams carries the fields for inserting a new overtime rule.
type CreateOvertimeRuleParams struct {
	Name                string
	WeekdayRate         float64
	RestdayRate         float64
	HolidayRate         float64
	MinMinutes          int
	MaxMinutesPerDay    int
	PreApprovalRequired bool
}

// UpdateOvertimeRuleParams carries the fields for updating an overtime rule.
type UpdateOvertimeRuleParams struct {
	ID                  string
	Name                string
	WeekdayRate         float64
	RestdayRate         float64
	HolidayRate         float64
	MinMinutes          int
	MaxMinutesPerDay    int
	PreApprovalRequired bool
}

// --- Service ---

// MasterDataService implements the E2 operational master-data business logic.
// Separate from Service (companies) and ServiceLineService — parallel-merge clean.
type MasterDataService struct {
	repo MasterDataRepository
	txm  TxRunner // reuses TxRunner from companies_service.go
}

// NewMasterDataService wires the service with its dependencies.
func NewMasterDataService(repo MasterDataRepository, txm TxRunner) *MasterDataService {
	return &MasterDataService{repo: repo, txm: txm}
}

// minMinutesDefault is the default value for overtime rule min_minutes (spec default=30).
const minMinutesDefault = 30

// validateMinMinutes returns RULE_VIOLATION (422) if min_minutes < 30.
func validateMinMinutes(m int) error {
	if m < 30 {
		return apperr.Rule("RULE_VIOLATION", map[string]string{"min_minutes": "Minimal 30 menit."})
	}
	return nil
}

// mapMDConflict translates a unique-index violation into apperr.Conflict("CONFLICT").
// Passes through any apperr.Error (e.g. RULE_VIOLATION) unchanged.
func mapMDConflict(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	if isUniqueViolation(err) { // reuses isUniqueViolation from companies_service.go
		return apperr.Conflict("CONFLICT")
	}
	return apperr.Internal(err)
}

// =============================================================================
// Leave Types
// =============================================================================

// GetLeaveType returns a single leave type by id.
func (s *MasterDataService) GetLeaveType(ctx context.Context, id string) (domain.LeaveType, error) {
	lt, err := s.repo.GetLeaveTypeByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.LeaveType{}, apperr.NotFound()
	}
	if err != nil {
		return domain.LeaveType{}, apperr.Internal(err)
	}
	return lt, nil
}

// ListLeaveTypes returns a cursor-paginated page of leave types.
func (s *MasterDataService) ListLeaveTypes(ctx context.Context, f domain.LeaveTypeFilter) ([]domain.LeaveType, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1

	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListLeaveTypes(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// CreateLeaveType inserts a new leave type with audit.
func (s *MasterDataService) CreateLeaveType(ctx context.Context, p CreateLeaveTypeParams) (domain.LeaveType, error) {
	var created domain.LeaveType
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateLeaveType(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "leave_type",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"name":      created.Name,
				"code":      created.Code,
				"is_annual": created.IsAnnual,
			},
		})
	}); err != nil {
		return domain.LeaveType{}, mapMDConflict(err)
	}
	return created, nil
}

// UpdateLeaveType patches a leave type with audit.
func (s *MasterDataService) UpdateLeaveType(ctx context.Context, p UpdateLeaveTypeParams) (domain.LeaveType, error) {
	current, err := s.repo.GetLeaveTypeByID(ctx, p.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.LeaveType{}, apperr.NotFound()
	}
	if err != nil {
		return domain.LeaveType{}, apperr.Internal(err)
	}

	var updated domain.LeaveType
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdateLeaveType(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "leave_type",
			EntityID:   p.ID,
			Before:     map[string]any{"name": current.Name, "code": current.Code},
			After:      map[string]any{"name": updated.Name, "code": updated.Code},
		})
	}); err != nil {
		return domain.LeaveType{}, mapMDConflict(err)
	}
	return updated, nil
}

// SoftDeleteLeaveType sets deleted_at on the leave type (status→INACTIVE, 204 at handler).
// TODO(Phase 7/8): guard against LT_IN_USE when referenced by leave requests.
func (s *MasterDataService) SoftDeleteLeaveType(ctx context.Context, id string) error {
	_, err := s.repo.GetLeaveTypeByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if err != nil {
		return apperr.Internal(err)
	}

	// TODO(Phase 7/8): check leave_requests references → return apperr.Conflict("LT_IN_USE")

	return s.txm.InTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.SoftDeleteLeaveType(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("leave_type.delete"),
			EntityType: "leave_type",
			EntityID:   id,
			Before:     map[string]any{"status": "active"},
			After:      map[string]any{"status": "inactive"},
		})
	})
}

// =============================================================================
// Attendance Codes
// =============================================================================

// GetAttendanceCode returns a single attendance code by id.
func (s *MasterDataService) GetAttendanceCode(ctx context.Context, id string) (domain.AttendanceCode, error) {
	ac, err := s.repo.GetAttendanceCodeByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.AttendanceCode{}, apperr.NotFound()
	}
	if err != nil {
		return domain.AttendanceCode{}, apperr.Internal(err)
	}
	return ac, nil
}

// ListAttendanceCodes returns a cursor-paginated page of attendance codes.
func (s *MasterDataService) ListAttendanceCodes(ctx context.Context, f domain.AttendanceCodeFilter) ([]domain.AttendanceCode, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1

	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListAttendanceCodes(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// CreateAttendanceCode inserts a new attendance code with audit.
func (s *MasterDataService) CreateAttendanceCode(ctx context.Context, p CreateAttendanceCodeParams) (domain.AttendanceCode, error) {
	var created domain.AttendanceCode
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateAttendanceCode(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "attendance_code",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"code":       created.Code,
				"label":      created.Label,
				"is_workday": created.IsWorkday,
			},
		})
	}); err != nil {
		return domain.AttendanceCode{}, mapMDConflict(err)
	}
	return created, nil
}

// UpdateAttendanceCode patches an attendance code with audit.
func (s *MasterDataService) UpdateAttendanceCode(ctx context.Context, p UpdateAttendanceCodeParams) (domain.AttendanceCode, error) {
	current, err := s.repo.GetAttendanceCodeByID(ctx, p.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.AttendanceCode{}, apperr.NotFound()
	}
	if err != nil {
		return domain.AttendanceCode{}, apperr.Internal(err)
	}

	var updated domain.AttendanceCode
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdateAttendanceCode(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "attendance_code",
			EntityID:   p.ID,
			Before:     map[string]any{"code": current.Code, "label": current.Label},
			After:      map[string]any{"code": updated.Code, "label": updated.Label},
		})
	}); err != nil {
		return domain.AttendanceCode{}, mapMDConflict(err)
	}
	return updated, nil
}

// SoftDeleteAttendanceCode sets deleted_at on the attendance code (204 at handler).
// TODO(Phase 7/8): guard against AC_IN_USE when referenced by attendance records.
func (s *MasterDataService) SoftDeleteAttendanceCode(ctx context.Context, id string) error {
	_, err := s.repo.GetAttendanceCodeByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if err != nil {
		return apperr.Internal(err)
	}

	// TODO(Phase 7/8): check attendance_records references → return apperr.Conflict("AC_IN_USE")

	return s.txm.InTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.SoftDeleteAttendanceCode(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("attendance_code.delete"),
			EntityType: "attendance_code",
			EntityID:   id,
			Before:     map[string]any{"status": "active"},
			After:      map[string]any{"status": "inactive"},
		})
	})
}

// =============================================================================
// Overtime Rules
// =============================================================================

// ListOvertimeRules returns a cursor-paginated page of overtime rules.
func (s *MasterDataService) ListOvertimeRules(ctx context.Context, f domain.OvertimeRuleFilter) ([]domain.OvertimeRule, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1

	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListOvertimeRules(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// CreateOvertimeRule inserts a new overtime rule with audit.
// OR-1: min_minutes < 30 → RULE_VIOLATION (422). Default min_minutes to 30 when 0.
func (s *MasterDataService) CreateOvertimeRule(ctx context.Context, p CreateOvertimeRuleParams) (domain.OvertimeRule, error) {
	if p.MinMinutes == 0 {
		p.MinMinutes = minMinutesDefault
	}
	if err := validateMinMinutes(p.MinMinutes); err != nil {
		return domain.OvertimeRule{}, err
	}

	var created domain.OvertimeRule
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateOvertimeRule(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "overtime_rule",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"name":        created.Name,
				"min_minutes": created.MinMinutes,
			},
		})
	}); err != nil {
		return domain.OvertimeRule{}, mapMDConflict(err)
	}
	return created, nil
}

// UpdateOvertimeRule patches an overtime rule with audit.
// OR-1: min_minutes < 30 → RULE_VIOLATION (422). Carries forward min_minutes when 0.
func (s *MasterDataService) UpdateOvertimeRule(ctx context.Context, p UpdateOvertimeRuleParams) (domain.OvertimeRule, error) {
	current, err := s.repo.GetOvertimeRuleByID(ctx, p.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.OvertimeRule{}, apperr.NotFound()
	}
	if err != nil {
		return domain.OvertimeRule{}, apperr.Internal(err)
	}

	// Carry forward current fields when request fields are zero/nil.
	if p.Name == "" {
		p.Name = current.Name
	}
	if p.MinMinutes == 0 {
		p.MinMinutes = current.MinMinutes
	}
	if p.MaxMinutesPerDay == 0 {
		p.MaxMinutesPerDay = current.MaxMinutesPerDay
	}
	if p.WeekdayRate == 0 {
		p.WeekdayRate = current.WeekdayRate
	}
	if p.RestdayRate == 0 {
		p.RestdayRate = current.RestdayRate
	}
	if p.HolidayRate == 0 {
		p.HolidayRate = current.HolidayRate
	}
	if err := validateMinMinutes(p.MinMinutes); err != nil {
		return domain.OvertimeRule{}, err
	}

	var updated domain.OvertimeRule
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdateOvertimeRule(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "overtime_rule",
			EntityID:   p.ID,
			Before:     map[string]any{"name": current.Name, "min_minutes": current.MinMinutes},
			After:      map[string]any{"name": updated.Name, "min_minutes": updated.MinMinutes},
		})
	}); err != nil {
		return domain.OvertimeRule{}, mapMDConflict(err)
	}
	return updated, nil
}

// SoftDeleteOvertimeRule sets deleted_at on the overtime rule (204 at handler).
// TODO(Phase 7/8): guard against OTR_IN_USE when referenced by schedules.
func (s *MasterDataService) SoftDeleteOvertimeRule(ctx context.Context, id string) error {
	_, err := s.repo.GetOvertimeRuleByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if err != nil {
		return apperr.Internal(err)
	}

	// TODO(Phase 7/8): check schedule references → return apperr.Conflict("OTR_IN_USE")

	return s.txm.InTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.SoftDeleteOvertimeRule(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("overtime_rule.delete"),
			EntityType: "overtime_rule",
			EntityID:   id,
			Before:     map[string]any{"status": "active"},
			After:      map[string]any{"status": "inactive"},
		})
	})
}
