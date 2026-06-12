// Package reporting — E10 services (F10.1 notifications + 11-02b dashboard /
// billable report / export framework). 11-02 owns the notification surface: the
// caller's in-app inbox (cursor list + read-state/kind filter), single mark-read,
// and bulk mark-all-read — all scope=self (a user only ever sees rows addressed
// to their SWP-USR-* or SWP-EMP-* id; the auto-dispatch in leave/OT/attendance
// targets whichever the principal carries).
//
// Mirrors the Phase-2 foundations audit-log list (keyset cursor on created_at,id)
// + the Phase-10 payroll slice shape (service → handler → routes → seed).
package reporting

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
)

// TxRunner runs a closure inside a DB transaction (db.TxManager satisfies it).
// Used by the export service for the transactional-outbox insert+enqueue.
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Jobs is the River enqueue seam (the real *jobs.Client satisfies it). EnqueueTx
// inserts the ReportExportArgs in the SAME tx as the export_jobs QUEUED insert
// (transactional outbox). An interface so 11-03 can fake it.
type Jobs interface {
	EnqueueTx(ctx context.Context, tx pgx.Tx, args river.JobArgs) error
}

// --- filters ---

// NotificationFilter is the decoded GET /notifications query (cursor-paged,
// newest-first). ReadState is one of UNREAD/READ/ALL (default ALL); Kinds is the
// optional kind / kind__in set. RecipientIDs is filled by the service from the
// principal (scope=self) — never from the client.
type NotificationFilter struct {
	RecipientIDs  []string // [actorUserID, actorEmployeeID] — scope=self
	ReadState     *string  // UNREAD | READ | ALL (nil → ALL)
	Kinds         []string // kind or kind__in (empty → no kind filter)
	Limit         int
	CursorCreated *time.Time
	CursorID      *string
}

// --- repository port ---

// NotificationRepository is the data dependency for the notification service.
// It wraps the 11-01 sqlc notifications queries. List/Get/MarkRead/MarkAllRead
// are all recipient-scoped at the SQL level (scope=self defense-in-depth).
type NotificationRepository interface {
	// List returns up to limit rows for ANY of recipientIDs, newest-first. The
	// service passes limit+1 for the cursor probe. (sqlc's ListNotifications is
	// single-recipient; the repo fans out + merges for the user/employee pair.)
	List(ctx context.Context, f NotificationFilter, limit int) ([]dom.Notification, error)
	// MarkRead flips read_at null→now for (id ∈ recipientIDs); returns the row.
	// domain.ErrNotFound when no owned row matches (404 / scope=self).
	MarkRead(ctx context.Context, id string, recipientIDs []string) (dom.Notification, error)
	// MarkAllRead marks every unread row for recipientIDs (optional before cutoff)
	// and returns the affected count.
	MarkAllRead(ctx context.Context, recipientIDs []string, before *time.Time) (int, error)
}

// --- dashboard port ---

// DashboardCounts is the bundle of live dashboard aggregations the service needs
// to assemble any role's payload. companyID nil = global (HR/super); set = the
// leader's own company. Only the fields a given role needs are read.
type DashboardCounts struct {
	PendingAttendanceVerify int
	PendingLeaveApprove     int
	PendingLeaveApproveHR   int
	PendingOTApprove        int
	ExpiringPlacements30d   int
	ExpiringAgreements30d   int
	ActivePlacements        int
	ActiveCompanies         int
}

// LeaderToday mirrors the sqlc LeaderTodayStatus row (today's team roll-up).
type LeaderTodayRow struct {
	ShiftsTotal          int
	ClockedIn            int
	LateCount            int
	AbsentCount          int
	PendingVerifications int
}

// AgentRecentRow mirrors the sqlc AgentRecentAttendance row.
type AgentRecentRow struct {
	Present int
	Late    int
	Absent  int
}

// AgentPendingRow mirrors the sqlc CountPendingRequestsForEmployee row.
type AgentPendingRow struct {
	Leave int
	OT    int
}

// SuperAdminWidgetsData is the raw bundle backing the admin-only widget block
// (DB-7). The repo runs the four global aggregations; the service maps it to the
// domain shape (org rollups grouped by free-text position, audit columns → labels).
// now is the Asia/Jakarta-resolved instant used for the offboarded-30d window.
type SuperAdminWidgetsData struct {
	ActiveUsers          int
	OffboardedUsers30d   int
	BankApprovalsPending int
	OrgRollups           []OrgRollupRow
	RecentAudit          []AuditRow
}

// OrgRollupRow mirrors the sqlc OrgRollupsByPosition row (free-text position).
type OrgRollupRow struct {
	Position         string
	Headcount        int
	ActivePlacements int
}

// AuditRow mirrors the sqlc RecentAuditEntries row (raw audit columns).
type AuditRow struct {
	ID          string
	ActorUserID *string
	ActorRole   *string
	Action      string
	EntityType  string
	EntityID    string
	CreatedAt   time.Time
}

// DashboardRepository wraps the 11-01 dashboard aggregation queries. today is the
// Asia/Jakarta calendar date the service resolves once. companyID nil = global.
type DashboardRepository interface {
	HrCounts(ctx context.Context, today time.Time, companyID *string) (DashboardCounts, error)
	LeaderToday(ctx context.Context, today time.Time, companyID string) (LeaderTodayRow, error)
	LeaderPending(ctx context.Context, companyID string) (attendanceVerify, leaveApprove, otApprove int, err error)
	CompanyName(ctx context.Context, companyID string) (string, error)
	AgentRecent(ctx context.Context, employeeID string, today time.Time) (AgentRecentRow, error)
	AgentPending(ctx context.Context, employeeID string) (AgentPendingRow, error)
	// AgentTodayShift returns the agent's live shift for `today` (Asia/Jakarta), or
	// nil when off-duty (no entry, an explicit day-off, or a leave-cancelled entry).
	// clock_in_status is derived from today's attendance for that schedule entry.
	AgentTodayShift(ctx context.Context, employeeID string, today time.Time) (*dom.AgentTodayShift, error)
	CountUnread(ctx context.Context, recipientIDs []string) (int, error)
	// SuperAdminWidgets runs the four global admin-block aggregations (DB-7). now is
	// used for the offboarded-30d window; auditLimit caps recent_audit (~8).
	SuperAdminWidgets(ctx context.Context, now time.Time, auditLimit int) (SuperAdminWidgetsData, error)
}

// --- billable port ---

// BillableQuery is the decoded /reports/attendance-billable filter set (after
// scope coercion). CompanyID/Position nil = unfiltered. Position is the free-text
// placement position filter (exact match).
type BillableQuery struct {
	CompanyID   *string
	Position    *string
	PeriodStart string // ISO date
	PeriodEnd   string // ISO date
	GroupBy     dom.BillableGroupBy
}

// BillableRepository wraps the 11-01 billable aggregation queries. The aggregate
// rows + the two summary rows feed BillableReport; CountInScope backs the export
// size guard.
type BillableRepository interface {
	Aggregate(ctx context.Context, q BillableQuery) ([]dom.BillableReportRow, error)
	Summary(ctx context.Context, q BillableQuery) (dom.BillableSummary, error)
	PendingSummary(ctx context.Context, q BillableQuery) (dom.BillablePendingSummary, error)
	// CountInScope returns verified+pending record count for the export size guard.
	CountInScope(ctx context.Context, q BillableQuery) (int, error)
}

// --- export port ---

// ExportInsert carries one generic export_jobs QUEUED insert (transactional outbox).
type ExportInsert struct {
	ReportType      string
	Format          string
	Confidential    bool
	Filters         []byte // json.Marshal(map[string]any)
	RequestedByID   string
	RequestedByName *string
	AuditLogEntryID *string
	ExpiresAt       *time.Time
}

// ExportRepository wraps the 11-01 generic export queries. Insert runs in-tx
// (WithTx); Get/Cancel run on the pool. CountRecent backs the per-user throttle.
type ExportRepository interface {
	InsertExportJob(ctx context.Context, tx pgx.Tx, p ExportInsert) (dom.ExportJob, error)
	GetExportJob(ctx context.Context, id string) (dom.ExportJob, error)
	CancelExportJob(ctx context.Context, id string) (dom.ExportJob, error)
	// CountRecentExports counts this requester's export_jobs created within window.
	CountRecentExports(ctx context.Context, requesterID string, since time.Time) (int, error)
}
