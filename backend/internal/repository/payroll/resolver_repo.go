// Package payroll (repository) — ClarificationTargetResolver implementation
// (PC-13 / C-PC-6). For a source record (ATTENDANCE/OVERTIME/LEAVE) it reads the
// owning agent + company, then the company's active shift-leader. The service picks
// the leader if present, else the agent.
package payroll

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	pdom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// TargetResolver is the sqlc-backed ClarificationTargetResolver.
type TargetResolver struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ClarificationTargetResolver = (*TargetResolver)(nil)

// NewTargetResolver returns a TargetResolver backed by pool.
func NewTargetResolver(pool *db.Pool) *TargetResolver {
	return &TargetResolver{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// ResolveTarget reads the record's owning agent + company and the company's active
// shift-leader. A missing record surfaces as domain.ErrNotFound (the service maps to
// 404). A company with no active leader leaves LeaderEmployeeID empty (C-PC-6 falls
// back to the agent).
func (r *TargetResolver) ResolveTarget(ctx context.Context, sourceType, sourceID string) (svc.ClarificationTarget, error) {
	var (
		employeeID string
		companyID  string
	)
	switch pdom.ClarificationSourceType(sourceType) {
	case pdom.ClarificationSourceAttendance:
		row, err := r.q.GetAttendanceOwner(ctx, sourceID)
		if err != nil {
			return svc.ClarificationTarget{}, notFoundOr(err)
		}
		employeeID, companyID = row.EmployeeID, row.CompanyID
	case pdom.ClarificationSourceOvertime:
		row, err := r.q.GetOvertimeOwner(ctx, sourceID)
		if err != nil {
			return svc.ClarificationTarget{}, notFoundOr(err)
		}
		employeeID = row.EmployeeID
		companyID = derefStr(row.CompanyID)
	case pdom.ClarificationSourceLeave:
		row, err := r.q.GetLeaveRequestOwner(ctx, sourceID)
		if err != nil {
			return svc.ClarificationTarget{}, notFoundOr(err)
		}
		employeeID = row.EmployeeID
		companyID = derefStr(row.CompanyID)
	default:
		return svc.ClarificationTarget{}, domain.ErrNotFound
	}

	out := svc.ClarificationTarget{AgentEmployeeID: employeeID, CompanyID: companyID}
	if companyID != "" {
		leader, err := r.q.GetActiveCompanyLeaderEmployee(ctx, companyID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return svc.ClarificationTarget{}, err
		}
		if err == nil {
			out.LeaderEmployeeID = leader
		}
	}
	return out, nil
}

func notFoundOr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
