// Package overtime (repository) — OvertimeRepo implements svc.OvertimeRepository
// and svc.RuleRepository over the 09-01 sqlc overtime/overtime_approvals queries +
// the EXISTING E2/Phase-3 overtime_rules queries (reused, NOT reimplemented).
// Reads on the pool; locked re-checks + writes via q.WithTx(tx).
package overtime

import (
	"context"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/overtime"
)

// OvertimeRepo is the sqlc-backed implementation of svc.OvertimeRepository +
// svc.RuleRepository.
type OvertimeRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var (
	_ svc.OvertimeRepository = (*OvertimeRepo)(nil)
	_ svc.RuleRepository     = (*OvertimeRepo)(nil)
)

// NewOvertimeRepo returns an OvertimeRepo backed by pool.
func NewOvertimeRepo(pool *db.Pool) *OvertimeRepo {
	return &OvertimeRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- list / get ---

func (r *OvertimeRepo) ListOvertime(ctx context.Context, f svc.OvertimeFilter) ([]dom.Overtime, error) {
	p := sqlcgen.ListOvertimeParams{
		EmployeeID:           strptr(f.EmployeeID),
		CompanyID:            strptr(f.CompanyID),
		Status:               strptr(f.Status),
		StatusIn:             f.StatusIn,
		DayType:              strptr(f.Tier),
		Source:               strptr(f.Source),
		FlaggedNoPreapproval: f.FlaggedNoPreapproval,
		CursorCreatedAt:      f.CursorCreated,
		CursorID:             f.CursorID,
		Lim:                  i32(f.Limit),
	}
	if f.WorkFrom != nil {
		p.WorkFrom = timeToPgDate(*f.WorkFrom)
	}
	if f.WorkTo != nil {
		p.WorkTo = timeToPgDate(*f.WorkTo)
	}
	rows, err := r.q.ListOvertime(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]dom.Overtime, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOvertimeFromList(row))
	}
	return out, nil
}

func (r *OvertimeRepo) GetOvertime(ctx context.Context, id string) (dom.Overtime, error) {
	row, err := r.q.GetOvertime(ctx, id)
	if err != nil {
		return dom.Overtime{}, mapErr(err)
	}
	return mapOvertimeFromGet(row), nil
}

func (r *OvertimeRepo) GetOvertimeForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.Overtime, error) {
	row, err := r.q.WithTx(tx).GetOvertimeForUpdate(ctx, id)
	if err != nil {
		return dom.Overtime{}, mapErr(err)
	}
	return mapOvertimeFromForUpdate(row), nil
}

// --- transition ---

func (r *OvertimeRepo) UpdateOvertimeStatus(ctx context.Context, tx pgx.Tx, id string, status dom.OvertimeStatus) (dom.Overtime, error) {
	row, err := r.q.WithTx(tx).UpdateOvertimeStatus(ctx, sqlcgen.UpdateOvertimeStatusParams{
		Status: string(status),
		ID:     id,
	})
	if err != nil {
		return dom.Overtime{}, mapErr(err)
	}
	return mapOvertimeFromUpdate(row), nil
}

// --- approvals (decision trail) ---

func (r *OvertimeRepo) InsertOvertimeApproval(ctx context.Context, tx pgx.Tx, p svc.ApprovalRow) (dom.OvertimeApproval, error) {
	row, err := r.q.WithTx(tx).InsertOvertimeApproval(ctx, sqlcgen.InsertOvertimeApprovalParams{
		OvertimeID:   p.OvertimeID,
		Level:        i32(p.Level),
		Decision:     p.Decision,
		ApproverID:   p.ApproverID,
		ApproverName: p.ApproverName,
		Reason:       p.Reason,
	})
	if err != nil {
		return dom.OvertimeApproval{}, mapErr(err)
	}
	return mapApproval(row), nil
}

func (r *OvertimeRepo) ListOvertimeApprovals(ctx context.Context, overtimeID string) ([]dom.OvertimeApproval, error) {
	rows, err := r.q.ListOvertimeApprovals(ctx, overtimeID)
	if err != nil {
		return nil, err
	}
	out := make([]dom.OvertimeApproval, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapApproval(row))
	}
	return out, nil
}

// --- rule lookup (reused E2/Phase-3 overtime_rules) ---

// FindOvertimeRule resolves the applicable overtime rule for OT_BELOW_MIN +
// reference-multiplier lookup. A line-scoped active rule wins over the global
// default (OR-2); the global default (service_line_id NULL) is the fallback.
// domain.ErrNotFound when no rule is configured.
func (r *OvertimeRepo) FindOvertimeRule(ctx context.Context, serviceLineID *string) (svc.OvertimeRule, error) {
	active := "active"
	// 1. Try a line-scoped active rule first (when a line is given).
	if serviceLineID != nil && *serviceLineID != "" {
		rows, err := r.q.ListOvertimeRules(ctx, sqlcgen.ListOvertimeRulesParams{
			Status:        &active,
			ServiceLineID: serviceLineID,
			RowLimit:      1,
		})
		if err != nil {
			return svc.OvertimeRule{}, err
		}
		if len(rows) > 0 {
			return ruleFromList(rows[0]), nil
		}
	}
	// 2. Fall back to the global default (service_line_id IS NULL). The list query
	//    has no "global only" filter, so scan the active rules and pick a NULL-line
	//    one.
	rows, err := r.q.ListOvertimeRules(ctx, sqlcgen.ListOvertimeRulesParams{
		Status:   &active,
		RowLimit: 200,
	})
	if err != nil {
		return svc.OvertimeRule{}, err
	}
	for _, row := range rows {
		if row.ServiceLineID == nil {
			return ruleFromList(row), nil
		}
	}
	// 3. As a last resort, return any active rule (keeps OT_BELOW_MIN enforceable).
	if len(rows) > 0 {
		return ruleFromList(rows[0]), nil
	}
	return svc.OvertimeRule{}, mapErr(pgx.ErrNoRows)
}

func ruleFromList(r sqlcgen.ListOvertimeRulesRow) svc.OvertimeRule {
	return svc.OvertimeRule{
		ID:            r.ID,
		ServiceLineID: r.ServiceLineID,
		WeekdayRate:   float64(r.WeekdayRate),
		RestdayRate:   float64(r.RestdayRate),
		HolidayRate:   float64(r.HolidayRate),
		MinMinutes:    int(r.MinMinutes),
	}
}
