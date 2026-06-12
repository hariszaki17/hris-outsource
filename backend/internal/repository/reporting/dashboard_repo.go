// Package reporting (repository) — DashboardRepo implements svc.DashboardRepository
// over the 11-01 dashboard aggregation queries (live counts, no rollup). Scope is a
// nullable companyID (nil = global HR/super, set = the leader's company). Dates are
// converted time.Time → pgtype.Date at the boundary (mirrors Phase-5/9).
package reporting

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
)

// DashboardRepo is the sqlc-backed implementation of svc.DashboardRepository.
type DashboardRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.DashboardRepository = (*DashboardRepo)(nil)

// NewDashboardRepo returns a DashboardRepo backed by pool.
func NewDashboardRepo(pool *db.Pool) *DashboardRepo {
	return &DashboardRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

// HrCounts runs the HR/leader KPI counts. companyID nil = global.
func (r *DashboardRepo) HrCounts(ctx context.Context, today time.Time, companyID *string) (svc.DashboardCounts, error) {
	var c svc.DashboardCounts
	d := pgDate(today)

	v, err := r.q.CountPendingAttendanceVerify(ctx, companyID)
	if err != nil {
		return c, err
	}
	c.PendingAttendanceVerify = int(v)

	la, err := r.q.CountPendingLeaveApprove(ctx, companyID)
	if err != nil {
		return c, err
	}
	c.PendingLeaveApprove = int(la)

	lhr, err := r.q.CountPendingLeaveApproveHR(ctx, companyID)
	if err != nil {
		return c, err
	}
	c.PendingLeaveApproveHR = int(lhr)

	ot, err := r.q.CountPendingOtApprove(ctx, companyID)
	if err != nil {
		return c, err
	}
	c.PendingOTApprove = int(ot)

	ep, err := r.q.CountExpiringPlacements30d(ctx, sqlcgen.CountExpiringPlacements30dParams{Today: d, CompanyID: companyID})
	if err != nil {
		return c, err
	}
	c.ExpiringPlacements30d = int(ep)

	ea, err := r.q.CountExpiringAgreements30d(ctx, d)
	if err != nil {
		return c, err
	}
	c.ExpiringAgreements30d = int(ea)

	ap, err := r.q.CountActivePlacements(ctx)
	if err != nil {
		return c, err
	}
	c.ActivePlacements = int(ap)

	ac, err := r.q.CountActiveCompanies(ctx)
	if err != nil {
		return c, err
	}
	c.ActiveCompanies = int(ac)

	return c, nil
}

// LeaderToday runs the today team-status roll-up scoped to one company.
func (r *DashboardRepo) LeaderToday(ctx context.Context, today time.Time, companyID string) (svc.LeaderTodayRow, error) {
	cid := &companyID
	row, err := r.q.LeaderTodayStatus(ctx, sqlcgen.LeaderTodayStatusParams{Today: pgDate(today), CompanyID: cid})
	if err != nil {
		return svc.LeaderTodayRow{}, err
	}
	return svc.LeaderTodayRow{
		ShiftsTotal:          int(row.ShiftsTotal),
		ClockedIn:            int(row.ClockedIn),
		LateCount:            int(row.LateCount),
		AbsentCount:          int(row.AbsentCount),
		PendingVerifications: int(row.PendingVerifications),
	}, nil
}

// LeaderPending runs the three leader pending counts scoped to one company.
func (r *DashboardRepo) LeaderPending(ctx context.Context, companyID string) (int, int, int, error) {
	cid := &companyID
	av, err := r.q.CountPendingAttendanceVerify(ctx, cid)
	if err != nil {
		return 0, 0, 0, err
	}
	la, err := r.q.CountPendingLeaveApprove(ctx, cid)
	if err != nil {
		return 0, 0, 0, err
	}
	ot, err := r.q.CountPendingOtApprove(ctx, cid)
	if err != nil {
		return 0, 0, 0, err
	}
	return int(av), int(la), int(ot), nil
}

// CompanyName resolves the display name for the leader's company (best-effort).
func (r *DashboardRepo) CompanyName(ctx context.Context, companyID string) (string, error) {
	var name string
	err := r.pool.Pool.QueryRow(ctx,
		`SELECT name FROM client_companies WHERE id = $1`, companyID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// AgentRecent runs the agent's last-7-day attendance roll-up.
func (r *DashboardRepo) AgentRecent(ctx context.Context, employeeID string, today time.Time) (svc.AgentRecentRow, error) {
	row, err := r.q.AgentRecentAttendance(ctx, sqlcgen.AgentRecentAttendanceParams{EmployeeID: employeeID, Today: pgDate(today)})
	if err != nil {
		return svc.AgentRecentRow{}, err
	}
	return svc.AgentRecentRow{Present: int(row.Last7dPresent), Late: int(row.Last7dLate), Absent: int(row.Last7dAbsent)}, nil
}

// AgentPending runs the agent's own pending leave + OT counts.
func (r *DashboardRepo) AgentPending(ctx context.Context, employeeID string) (svc.AgentPendingRow, error) {
	row, err := r.q.CountPendingRequestsForEmployee(ctx, employeeID)
	if err != nil {
		return svc.AgentPendingRow{}, err
	}
	return svc.AgentPendingRow{Leave: int(row.LeavePending), OT: int(row.OtPending)}, nil
}

// agentTodayShiftSQL resolves the agent's single live schedule entry for `today`
// (joined to its shift master for the name, the placement's company for the name,
// and the latest attendance for that entry to derive clock_in_status). A raw query
// (not sqlc) — this read is local to the dashboard and avoids a codegen round-trip.
const agentTodayShiftSQL = `
SELECT se.id,
       COALESCE(sm.name, ''),
       COALESCE(se.start_time, ''),
       COALESCE(se.end_time, ''),
       COALESCE(cc.name, ''),
       se.is_day_off,
       se.shift_master_id,
       a.check_in_at,
       a.check_out_at
FROM schedule_entries se
JOIN placements pl ON pl.id = se.placement_id
JOIN client_companies cc ON cc.id = pl.client_company_id
LEFT JOIN shift_masters sm ON sm.id = se.shift_master_id
LEFT JOIN LATERAL (
    SELECT check_in_at, check_out_at
    FROM attendance
    WHERE schedule_id = se.id AND deleted_at IS NULL
    ORDER BY check_in_at DESC NULLS LAST
    LIMIT 1
) a ON true
WHERE se.employee_id = $1
  AND se.work_date = $2
  AND se.deleted_at IS NULL
  AND se.status <> 'CANCELLED_BY_LEAVE'
ORDER BY se.created_at DESC
LIMIT 1`

// AgentTodayShift returns the agent's live shift for `today` (Asia/Jakarta), or
// nil when off-duty (no entry, an explicit day-off, or a leave-cancelled entry).
func (r *DashboardRepo) AgentTodayShift(ctx context.Context, employeeID string, today time.Time) (*dom.AgentTodayShift, error) {
	var (
		id            string
		shiftName     string
		startTime     string
		endTime       string
		companyName   string
		isDayOff      bool
		shiftMasterID *string
		checkInAt     *time.Time
		checkOutAt    *time.Time
	)
	err := r.pool.Pool.QueryRow(ctx, agentTodayShiftSQL, employeeID, pgDate(today)).Scan(
		&id, &shiftName, &startTime, &endTime, &companyName, &isDayOff, &shiftMasterID, &checkInAt, &checkOutAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Off-duty: an explicit day-off, or an entry with no shift template, is not a
	// clockable shift.
	if isDayOff || shiftMasterID == nil {
		return nil, nil
	}

	status := "NOT_CLOCKED_IN"
	if checkInAt != nil {
		if checkOutAt != nil {
			status = "CLOCKED_OUT"
		} else {
			status = "CLOCKED_IN"
		}
	}

	return &dom.AgentTodayShift{
		ScheduleID:    id,
		ShiftName:     shiftName,
		StartTime:     startTime,
		EndTime:       endTime,
		CompanyName:   companyName,
		ClockInStatus: status,
	}, nil
}

// SuperAdminWidgets runs the four global admin-block aggregations (DB-7). The
// service maps the raw rows to the domain shape (service-line name → enum, audit
// columns → labels).
func (r *DashboardRepo) SuperAdminWidgets(ctx context.Context, now time.Time, auditLimit int) (svc.SuperAdminWidgetsData, error) {
	var d svc.SuperAdminWidgetsData

	active, err := r.q.CountActiveUsers(ctx)
	if err != nil {
		return d, err
	}
	d.ActiveUsers = int(active)

	off, err := r.q.CountOffboardedUsers30d(ctx, now)
	if err != nil {
		return d, err
	}
	d.OffboardedUsers30d = int(off)

	bank, err := r.q.CountBankApprovalsPending(ctx)
	if err != nil {
		return d, err
	}
	d.BankApprovalsPending = int(bank)

	rollups, err := r.q.OrgRollupsByServiceLine(ctx)
	if err != nil {
		return d, err
	}
	d.OrgRollups = make([]svc.OrgRollupRow, 0, len(rollups))
	for _, row := range rollups {
		d.OrgRollups = append(d.OrgRollups, svc.OrgRollupRow{
			ServiceLineName:  row.ServiceLineName,
			Headcount:        int(row.Headcount),
			ActivePlacements: int(row.ActivePlacements),
		})
	}

	audit, err := r.q.RecentAuditEntries(ctx, int32(auditLimit))
	if err != nil {
		return d, err
	}
	d.RecentAudit = make([]svc.AuditRow, 0, len(audit))
	for _, row := range audit {
		d.RecentAudit = append(d.RecentAudit, svc.AuditRow{
			ID:          row.ID,
			ActorUserID: row.ActorUserID,
			ActorRole:   row.ActorRole,
			Action:      row.Action,
			EntityType:  row.EntityType,
			EntityID:    row.EntityID,
			CreatedAt:   row.CreatedAt,
		})
	}

	return d, nil
}

// CountUnread sums unread notifications across the principal's recipient ids.
func (r *DashboardRepo) CountUnread(ctx context.Context, recipientIDs []string) (int, error) {
	total := 0
	for _, rid := range recipientIDs {
		n, err := r.q.CountUnreadNotifications(ctx, rid)
		if err != nil {
			return 0, err
		}
		total += int(n)
	}
	return total, nil
}
