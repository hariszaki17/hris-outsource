// Package leave (repository) — EntitlementRepo implements svc.EntitlementRepository
// over the employee_leave_entitlements sqlc queries (migration 00063). HR-assigned
// per-employee leave entitlements (PRD leave-entitlement-assignment, 2026-06-15):
// reads on the pool; mutations via q.WithTx(tx) so the audit row commits atomically.
package leave

import (
	"context"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// EntitlementRepo is the sqlc-backed implementation of svc.EntitlementRepository.
type EntitlementRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.EntitlementRepository = (*EntitlementRepo)(nil)

// NewEntitlementRepo returns an EntitlementRepo backed by pool.
func NewEntitlementRepo(pool *db.Pool) *EntitlementRepo {
	return &EntitlementRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// ListEntitlements returns an employee's active assignments + type display metadata.
func (r *EntitlementRepo) ListEntitlements(ctx context.Context, employeeID string) ([]dom.LeaveEntitlement, error) {
	rows, err := r.q.ListEmployeeEntitlements(ctx, employeeID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]dom.LeaveEntitlement, 0, len(rows))
	for _, row := range rows {
		out = append(out, dom.LeaveEntitlement{
			ID:               row.ID,
			EmployeeID:       row.EmployeeID,
			LeaveTypeID:      row.LeaveTypeID,
			EntitledDays:     i32ptrToIntPtr(row.EntitledDays),
			Active:           row.Active,
			Note:             row.Note,
			AssignedBy:       row.AssignedBy,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
			Code:             row.Code,
			Name:             row.Name,
			CapBasis:         dom.LeaveTypeCapBasis(row.CapBasis),
			CapValue:         i32ptrToIntPtr(row.CapValue),
			CapUnit:          row.CapUnit,
			Paid:             row.Paid,
			RequiresDocument: row.RequiresDocument,
			Color:            row.Color,
			AppliesTo:        row.AppliesTo,
			Common:           row.Common,
		})
	}
	return out, nil
}

// UpsertEntitlement assigns a type (insert) or reactivates+re-quotas an existing one.
func (r *EntitlementRepo) UpsertEntitlement(ctx context.Context, tx pgx.Tx, p svc.EntitlementWrite) (dom.LeaveEntitlement, error) {
	row, err := r.q.WithTx(tx).UpsertEmployeeEntitlement(ctx, sqlcgen.UpsertEmployeeEntitlementParams{
		EmployeeID:   p.EmployeeID,
		LeaveTypeID:  p.LeaveTypeID,
		EntitledDays: i32ptr(p.EntitledDays),
		Note:         p.Note,
		AssignedBy:   p.AssignedBy,
	})
	if err != nil {
		return dom.LeaveEntitlement{}, mapErr(err)
	}
	return mapEntitlementCore(row.ID, row.EmployeeID, row.LeaveTypeID, row.EntitledDays, row.Active, row.Note, row.AssignedBy), nil
}

// UpdateEntitlement edits the base entitled_days / reactivates (PATCH).
func (r *EntitlementRepo) UpdateEntitlement(ctx context.Context, tx pgx.Tx, p svc.EntitlementWrite) (dom.LeaveEntitlement, error) {
	row, err := r.q.WithTx(tx).UpdateEmployeeEntitlement(ctx, sqlcgen.UpdateEmployeeEntitlementParams{
		EntitledDays: i32ptr(p.EntitledDays),
		Note:         p.NotePtr,
		AssignedBy:   p.AssignedBy,
		EmployeeID:   p.EmployeeID,
		LeaveTypeID:  p.LeaveTypeID,
	})
	if err != nil {
		return dom.LeaveEntitlement{}, mapErr(err)
	}
	return mapEntitlementCore(row.ID, row.EmployeeID, row.LeaveTypeID, row.EntitledDays, row.Active, row.Note, row.AssignedBy), nil
}

// DeactivateEntitlement removes a type from an employee (soft on/off).
func (r *EntitlementRepo) DeactivateEntitlement(ctx context.Context, tx pgx.Tx, employeeID, leaveTypeID string, assignedBy *string) (dom.LeaveEntitlement, error) {
	row, err := r.q.WithTx(tx).DeactivateEmployeeEntitlement(ctx, sqlcgen.DeactivateEmployeeEntitlementParams{
		AssignedBy: assignedBy, EmployeeID: employeeID, LeaveTypeID: leaveTypeID,
	})
	if err != nil {
		return dom.LeaveEntitlement{}, mapErr(err)
	}
	return mapEntitlementCore(row.ID, row.EmployeeID, row.LeaveTypeID, row.EntitledDays, row.Active, row.Note, row.AssignedBy), nil
}

func mapEntitlementCore(id, employeeID, leaveTypeID string, entitledDays *int32, active bool, note string, assignedBy *string) dom.LeaveEntitlement {
	return dom.LeaveEntitlement{
		ID:           id,
		EmployeeID:   employeeID,
		LeaveTypeID:  leaveTypeID,
		EntitledDays: i32ptrToIntPtr(entitledDays),
		Active:       active,
		Note:         note,
		AssignedBy:   assignedBy,
	}
}
