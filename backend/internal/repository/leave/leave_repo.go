// Package leave (repository) — LeaveRepo implements svc.LeaveRepository over the
// 08-01 sqlc leave_requests / leave_approvals / leave_calendar / leave_types
// queries. Reads on the pool; locked re-checks + writes via q.WithTx(tx).
package leave

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// LeaveRepo is the sqlc-backed implementation of svc.LeaveRepository.
type LeaveRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.LeaveRepository = (*LeaveRepo)(nil)

// NewLeaveRepo returns a LeaveRepo backed by pool.
func NewLeaveRepo(pool *db.Pool) *LeaveRepo {
	return &LeaveRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func strptr(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

// --- list / get ---

func (r *LeaveRepo) ListLeaveRequests(ctx context.Context, f svc.RequestFilter) ([]dom.LeaveRequest, error) {
	p := sqlcgen.ListLeaveRequestsParams{
		CompanyID:       strptr(f.CompanyID),
		Status:          strptr(f.Status),
		StatusIn:        f.StatusIn,
		EmployeeID:      strptr(f.EmployeeID),
		LeaveTypeID:     strptr(f.LeaveTypeID),
		Q:               strptr(f.Q),
		CursorCreatedAt: f.CursorCreated,
		CursorID:        f.CursorID,
		Lim:             i32(f.Limit),
	}
	if f.StartFrom != nil {
		p.StartFrom = timeToPgDate(*f.StartFrom)
	}
	if f.StartTo != nil {
		p.StartTo = timeToPgDate(*f.StartTo)
	}
	rows, err := r.q.ListLeaveRequests(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]dom.LeaveRequest, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRequestFromList(row))
	}
	return out, nil
}

func (r *LeaveRepo) GetLeaveRequest(ctx context.Context, id string) (dom.LeaveRequest, error) {
	row, err := r.q.GetLeaveRequest(ctx, id)
	if err != nil {
		return dom.LeaveRequest{}, mapErr(err)
	}
	return mapRequestFromGet(row), nil
}

func (r *LeaveRepo) GetLeaveRequestForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.LeaveRequest, error) {
	row, err := r.q.WithTx(tx).GetLeaveRequestForUpdate(ctx, id)
	if err != nil {
		return dom.LeaveRequest{}, mapErr(err)
	}
	return mapRequestFromForUpdate(row), nil
}

// --- transitions ---

func (r *LeaveRepo) UpdateLeaveRequestStatus(ctx context.Context, tx pgx.Tx, p svc.UpdateStatusParams) (dom.LeaveRequest, error) {
	row, err := r.q.WithTx(tx).UpdateLeaveRequestStatus(ctx, sqlcgen.UpdateLeaveRequestStatusParams{
		Status:                  string(p.Status),
		NoLeader:                p.NoLeader,
		AssignedLeaderID:        p.AssignedLeaderID,
		ClockInConflict:         p.ClockInConflict,
		BalanceQuotaID:          p.BalanceQuotaID,
		BalanceRequestedDays:    i32ptr(p.BalanceRequestedDays),
		BalanceRemainingAtCheck: i32ptr(p.BalanceRemainingAtCheck),
		BalanceRequiresOverride: p.BalanceRequiresOverride,
		ID:                      p.ID,
	})
	if err != nil {
		return dom.LeaveRequest{}, mapErr(err)
	}
	return mapRequestFromUpdate(row), nil
}

// --- approvals (decision trail) ---

func (r *LeaveRepo) InsertLeaveApproval(ctx context.Context, tx pgx.Tx, p svc.ApprovalRow) (dom.LeaveApproval, error) {
	row, err := r.q.WithTx(tx).InsertLeaveApproval(ctx, sqlcgen.InsertLeaveApprovalParams{
		LeaveRequestID: p.LeaveRequestID,
		Stage:          string(p.Stage),
		Decision:       string(p.Decision),
		ActorID:        p.ActorID,
		ActorRole:      p.ActorRole,
		DecisionNote:   p.DecisionNote,
		RejectReason:   p.RejectReason,
		IsOverride:     p.IsOverride,
		OverrideReason: p.OverrideReason,
	})
	if err != nil {
		return dom.LeaveApproval{}, mapErr(err)
	}
	return mapApproval(row), nil
}

func (r *LeaveRepo) ListLeaveApprovalsForRequest(ctx context.Context, id string) ([]dom.LeaveApproval, error) {
	rows, err := r.q.ListLeaveApprovalsForRequest(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]dom.LeaveApproval, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapApproval(row))
	}
	return out, nil
}

// --- leave-type read-through (is_annual gate) ---

func (r *LeaveRepo) GetLeaveType(ctx context.Context, id string) (svc.LeaveTypeInfo, error) {
	row, err := r.q.GetLeaveTypeByID(ctx, id)
	if err != nil {
		return svc.LeaveTypeInfo{}, mapErr(err)
	}
	return svc.LeaveTypeInfo{
		ID:       row.ID,
		Code:     row.Code,
		Name:     row.Name,
		IsAnnual: row.IsAnnual,
	}, nil
}

// --- calendar ---

func (r *LeaveRepo) ListCalendarEntries(ctx context.Context, f svc.CalendarFilter, statusIn []string, from, to time.Time) ([]dom.LeaveCalendarEntry, error) {
	rows, err := r.q.ListCalendarEntries(ctx, sqlcgen.ListCalendarEntriesParams{
		RangeTo:     timeToPgDate(to),
		RangeFrom:   timeToPgDate(from),
		StatusIn:    statusIn,
		CompanyID:   strptr(f.CompanyID),
		ServiceLine: strptr(f.ServiceLine),
		LeaveTypeID: strptr(f.LeaveTypeID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]dom.LeaveCalendarEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapCalendarEntry(row))
	}
	return out, nil
}
