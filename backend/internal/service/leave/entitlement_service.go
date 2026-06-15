// Package leave — EntitlementService: HR-assigned per-employee leave entitlements
// (PRD leave-entitlement-assignment, RATIFIED 2026-06-15). HR enrols each employee
// into the leave types they get, with a per-period quota; only assigned types are
// requestable (ELE-1). List is a plain read; assign/update/remove run in a tx with an
// audit row (CONVENTIONS §16.1). assigned_by is the acting user (audit).
package leave

import (
	"context"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
)

// EntitlementService implements the HR leave-entitlement CRUD.
type EntitlementService struct {
	repo EntitlementRepository
	txm  TxRunner
}

// NewEntitlementService wires the entitlement service.
func NewEntitlementService(repo EntitlementRepository, txm TxRunner) *EntitlementService {
	return &EntitlementService{repo: repo, txm: txm}
}

// List returns an employee's active leave-type assignments (HR "Hak Cuti" grid).
func (s *EntitlementService) List(ctx context.Context, employeeID string) ([]dom.LeaveEntitlement, error) {
	return s.repo.ListEntitlements(ctx, employeeID)
}

// Assign enrols (or reactivates+re-quotas) a leave type for an employee (POST, ELE-6).
func (s *EntitlementService) Assign(ctx context.Context, employeeID, leaveTypeID string, entitledDays *int, note string) (dom.LeaveEntitlement, error) {
	w := EntitlementWrite{
		EmployeeID:   employeeID,
		LeaveTypeID:  leaveTypeID,
		EntitledDays: entitledDays,
		Note:         note,
		AssignedBy:   actorUserIDPtr(ctx),
	}
	var out dom.LeaveEntitlement
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		e, aerr := s.repo.UpsertEntitlement(ctx, tx, w)
		if aerr != nil {
			return aerr
		}
		out = e
		return audit.Record(ctx, tx, entitlementAudit(audit.ActionCreate, e))
	})
	if err != nil {
		return dom.LeaveEntitlement{}, asAppErr(err)
	}
	return out, nil
}

// Update edits the base entitled_days / reactivates a type (PATCH, ELE-4).
func (s *EntitlementService) Update(ctx context.Context, employeeID, leaveTypeID string, entitledDays *int, note *string) (dom.LeaveEntitlement, error) {
	w := EntitlementWrite{
		EmployeeID:   employeeID,
		LeaveTypeID:  leaveTypeID,
		EntitledDays: entitledDays,
		NotePtr:      note,
		AssignedBy:   actorUserIDPtr(ctx),
	}
	var out dom.LeaveEntitlement
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		e, aerr := s.repo.UpdateEntitlement(ctx, tx, w)
		if aerr != nil {
			return aerr
		}
		out = e
		return audit.Record(ctx, tx, entitlementAudit(audit.ActionUpdate, e))
	})
	if err != nil {
		return dom.LeaveEntitlement{}, asAppErr(err)
	}
	return out, nil
}

// Remove deactivates a type for an employee (DELETE — soft on/off, ELE-6).
func (s *EntitlementService) Remove(ctx context.Context, employeeID, leaveTypeID string) (dom.LeaveEntitlement, error) {
	var out dom.LeaveEntitlement
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		e, aerr := s.repo.DeactivateEntitlement(ctx, tx, employeeID, leaveTypeID, actorUserIDPtr(ctx))
		if aerr != nil {
			return aerr
		}
		out = e
		return audit.Record(ctx, tx, entitlementAudit(audit.ActionDelete, e))
	})
	if err != nil {
		return dom.LeaveEntitlement{}, asAppErr(err)
	}
	return out, nil
}

func entitlementAudit(action audit.Action, e dom.LeaveEntitlement) audit.Entry {
	return audit.Entry{
		Action:     action,
		EntityType: "employee_leave_entitlement",
		EntityID:   e.ID,
		After: map[string]any{
			"employee_id":   e.EmployeeID,
			"leave_type_id": e.LeaveTypeID,
			"entitled_days": e.EntitledDays,
			"active":        e.Active,
		},
	}
}
