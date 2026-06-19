// Package overtime (repository) — OvertimeRepo implements svc.OvertimeRepository
// and svc.RuleRepository over the 09-01 sqlc overtime/overtime_approvals queries +
// the EXISTING E2/Phase-3 overtime_rules queries (reused, NOT reimplemented).
// Reads on the pool; locked re-checks + writes via q.WithTx(tx).
package overtime

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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

// --- create (F7.2 agent/leader request path) ---

// InsertOvertime persists a new OT record via the 09-01 InsertOvertime query
// (id allocated by the column DEFAULT). worked/counted minutes are 0 at request
// time (the OT is pre-approval; actuals are filled later from attendance).
func (r *OvertimeRepo) InsertOvertime(ctx context.Context, tx pgx.Tx, p svc.OvertimeInsertParams) (dom.Overtime, error) {
	row, err := r.q.WithTx(tx).InsertOvertime(ctx, sqlcgen.InsertOvertimeParams{
		EmployeeID:       p.EmployeeID,
		CompanyID:        p.CompanyID,
		PlacementID:      p.PlacementID,
		WorkDate:         timeToPgDate(p.WorkDate),
		PlannedStartTime: p.PlannedStartTime,
		PlannedEndTime:   p.PlannedEndTime,
		CrossMidnight:    p.CrossMidnight,
		Source:           string(p.Source),
		Status:           string(p.Status),
		DayType:          string(p.DayType),
		HolidayID:        p.HolidayID,
		Reason:           p.Reason,
		CreatedBy:        p.CreatedBy,
	})
	if err != nil {
		return dom.Overtime{}, mapErr(err)
	}
	return mapOvertimeFromInsert(row), nil
}

// --- E11 approval linkage ---

// SetApprovalInstanceID links the freshly-created E11 ApprovalInstance to the OT record
// (called inside the create/confirm tx after engine.CreateInstance).
func (r *OvertimeRepo) SetApprovalInstanceID(ctx context.Context, tx pgx.Tx, id, instanceID string) error {
	return r.q.WithTx(tx).SetOvertimeApprovalInstanceID(ctx, sqlcgen.SetOvertimeApprovalInstanceIDParams{
		ApprovalInstanceID: &instanceID,
		ID:                 id,
	})
}

// --- rule lookup (reused E2/Phase-3 overtime_rules) ---

// FindOvertimeRule resolves the single GLOBAL overtime rule for OT_BELOW_MIN +
// reference-multiplier lookup. Overtime rules are GLOBAL ONLY (decision 2026-06-12 —
// the service_line scope axis + line-vs-global precedence were dropped): the first
// active rule is the effective rule. domain.ErrNotFound when none is configured.
func (r *OvertimeRepo) FindOvertimeRule(ctx context.Context) (svc.OvertimeRule, error) {
	active := "active"
	rows, err := r.q.ListOvertimeRules(ctx, sqlcgen.ListOvertimeRulesParams{
		Status:   &active,
		RowLimit: 1,
	})
	if err != nil {
		return svc.OvertimeRule{}, err
	}
	if len(rows) > 0 {
		return ruleFromList(rows[0]), nil
	}
	return svc.OvertimeRule{}, mapErr(pgx.ErrNoRows)
}

func ruleFromList(r sqlcgen.ListOvertimeRulesRow) svc.OvertimeRule {
	return svc.OvertimeRule{
		ID:          r.ID,
		WeekdayRate: float64(r.WeekdayRate),
		RestdayRate: float64(r.RestdayRate),
		HolidayRate: float64(r.HolidayRate),
		MinMinutes:  int(r.MinMinutes),
	}
}

// --- duplicate detection + aggregation (2026-06-19) ---

// ExistsActiveOvertimeForAgentDate reports whether the agent already has an active
// (not REJECTED, not CANCELLED) OT for work_date.
func (r *OvertimeRepo) ExistsActiveOvertimeForAgentDate(ctx context.Context, employeeID string, workDate time.Time) (bool, error) {
	return r.q.ExistsActiveOvertimeForAgentDate(ctx, employeeID, timeToPgDate(workDate))
}

// AggregateOvertime returns APPROVED OT totals grouped by agent or day_type.
func (r *OvertimeRepo) AggregateOvertime(ctx context.Context, p svc.AggregateParams) ([]svc.OvertimeAggregateRow, error) {
	var dateFrom, dateTo pgtype.Date
	if p.DateFrom != nil {
		dateFrom = timeToPgDate(*p.DateFrom)
	}
	if p.DateTo != nil {
		dateTo = timeToPgDate(*p.DateTo)
	}
	rows, err := r.q.AggregateOvertime(ctx, sqlcgen.AggregateOvertimeParams{
		GroupBy:   p.GroupBy,
		CompanyID: p.CompanyID,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
	})
	if err != nil {
		return nil, err
	}
	out := make([]svc.OvertimeAggregateRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, svc.OvertimeAggregateRow{
			GroupKey:       row.GroupKey,
			EmployeeID:     row.EmployeeID,
			EmployeeName:   row.EmployeeName,
			DayType:        row.DayType,
			TotalMinutes:   int(row.TotalMinutes),
			TotalApproved:  int(row.TotalApprovedCount),
			WorkdayCount:   int(row.WorkdayCount),
			RestdayCount:   int(row.RestdayCount),
			HolidayCount:   int(row.HolidayCount),
			WorkdayMinutes: int(row.WorkdayMinutes),
			RestdayMinutes: int(row.RestdayMinutes),
			HolidayMinutes: int(row.HolidayMinutes),
		})
	}
	return out, nil
}
