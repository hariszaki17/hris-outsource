package reporting

import "time"

// ApprovalInboxKind is one "Perlu Tindakan" panel row kind (openapi
// ApprovalInboxRow.kind), byte-for-byte.
type ApprovalInboxKind string

const (
	InboxAttendanceVerify  ApprovalInboxKind = "ATTENDANCE_VERIFY"
	InboxLeaveApprove      ApprovalInboxKind = "LEAVE_APPROVE"
	InboxOTApprove         ApprovalInboxKind = "OT_APPROVE"
	InboxPlacementExpiring ApprovalInboxKind = "PLACEMENT_EXPIRING"
	InboxAgreementExpiring ApprovalInboxKind = "AGREEMENT_EXPIRING"
	InboxHRChangeRequest   ApprovalInboxKind = "HR_CHANGE_REQUEST"
)

// ApprovalInboxRow is one row of the HR/Leader "Perlu Tindakan" panel (openapi
// schemas.ApprovalInboxRow). Each row deep-links into the owning epic's queue.
type ApprovalInboxRow struct {
	Kind     ApprovalInboxKind
	Label    string
	Count    int
	DeepLink DeepLink
}

// BillableTrendPoint is one bucket of HrDashboard.billable_trend.points.
type BillableTrendPoint struct {
	Date  string // ISO date
	Value float64
}

// HrKPIs is HrDashboard.kpis (openapi). AttendanceRatePct / BillableHoursMTD /
// OTHoursMTD are floats per the contract.
type HrKPIs struct {
	ActivePlacements  int
	ActiveCompanies   int
	AttendanceRatePct float64
	BillableHoursMTD  float64
	OTHoursMTD        float64
	LeavePending      int
}

// HrDashboard is the HR Admin / Super Admin dashboard (openapi schemas.HrDashboard).
// Role is "hr_admin" or "super_admin" (D1: same body, distinct RoleLabel).
type HrDashboard struct {
	Role                     string // hr_admin | super_admin
	RoleLabel                string
	GeneratedAt              time.Time
	PeriodLabel              string
	KPIs                     HrKPIs
	ExpiringPlacements30d    int
	ExpiringAgreements30d    int
	AttendanceAnomaliesToday int
	BillableTrend            BillableTrend
	PendingApprovalsPanel    []ApprovalInboxRow
	// Admin is the admin-only widget block (openapi schemas.SuperAdminWidgets, DB-7).
	// Populated ONLY when Role == super_admin; nil for hr_admin (C-6 → handler omits).
	Admin *SuperAdminWidgets
}

// SuperAdminWidgets is the admin-only dashboard block (openapi schemas.SuperAdminWidgets).
type SuperAdminWidgets struct {
	UserAccess    UserAccessSummary
	RecentAudit   []AuditEntry
	OrgRollups    []OrgRollup
	PendingGrants PendingGrants
}

// UserAccessSummary is SuperAdminWidgets.user_access (E2 / F2.7).
type UserAccessSummary struct {
	ActiveUsers         int
	PendingProvisioning int
	Offboarded30d       int
}

// AuditEntry is one SuperAdminWidgets.recent_audit row (E1 audit, newest first).
type AuditEntry struct {
	ID          string
	ActorLabel  string
	Action      string
	TargetLabel string
	At          time.Time
}

// OrgRollup is one SuperAdminWidgets.org_rollups row. Rollups GROUP BY the
// placement's free-text position (decision 2026-06-12: service_line removed; the
// rollup dimension is the free-text position, no master/enum).
type OrgRollup struct {
	Position         string // free-text placement position
	Headcount        int
	ActivePlacements int
}

// PendingGrants is SuperAdminWidgets.pending_grants (items awaiting super-admin action).
type PendingGrants struct {
	BankApprovals int
	RoleRequests  int
}

// BillableTrend is HrDashboard.billable_trend.
type BillableTrend struct {
	Granularity string // day | week | month
	Points      []BillableTrendPoint
}

// LeaderCompany is LeaderDashboard.company.
type LeaderCompany struct {
	ID   string
	Name string
}

// LeaderToday is LeaderDashboard.today (openapi).
type LeaderToday struct {
	Date                 string // ISO date
	ShiftsTotal          int
	ClockedIn            int
	LateCount            int
	AbsentCount          int
	PendingVerifications int
}

// LeaderPendingCounts is LeaderDashboard.pending_counts.
type LeaderPendingCounts struct {
	AttendanceVerify int
	LeaveApprove     int
	OTApprove        int
}

// ScheduleAlert is one LeaderDashboard.schedule_alerts row.
type ScheduleAlert struct {
	Kind     string // COVERAGE_GAP | UNASSIGNED_SHIFT | PLACEMENT_EXPIRING
	Label    string
	Date     *string // nullable ISO date
	DeepLink DeepLink
}

// LeaderDashboard is the Shift Leader dashboard (openapi schemas.LeaderDashboard),
// scoped to the leader's single company (E3 INV-3).
type LeaderDashboard struct {
	Role                  string // shift_leader
	RoleLabel             string
	Company               LeaderCompany
	GeneratedAt           time.Time
	Today                 LeaderToday
	PendingCounts         LeaderPendingCounts
	ScheduleAlerts        []ScheduleAlert
	PendingApprovalsPanel []ApprovalInboxRow
}

// AgentTodayShift is AgentDashboard.today_shift (nullable when off-duty).
type AgentTodayShift struct {
	ScheduleID    string
	ShiftName     string
	StartTime     string // HH:MM Asia/Jakarta
	EndTime       string
	CompanyName   string
	ClockInStatus string // NOT_CLOCKED_IN | CLOCKED_IN | CLOCKED_OUT | LATE | ABSENT
	DeepLink      *DeepLink
}

// AgentRecentAttendance is AgentDashboard.recent_attendance.
type AgentRecentAttendance struct {
	Last7dPresent int
	Last7dLate    int
	Last7dAbsent  int
}

// AgentLeaveBalance is AgentDashboard.leave_balance.
type AgentLeaveBalance struct {
	AnnualRemainingDays float64
	AnnualQuotaDays     float64
	PeriodLabel         string
}

// AgentPendingRequests is AgentDashboard.pending_requests.
type AgentPendingRequests struct {
	Leave int
	OT    int
}

// AgentDashboard is the Agent (self) dashboard (openapi schemas.AgentDashboard).
type AgentDashboard struct {
	Role                      string // agent
	GeneratedAt               time.Time
	TodayShift                *AgentTodayShift // nil = off-duty
	RecentAttendance          AgentRecentAttendance
	LeaveBalance              AgentLeaveBalance
	OTThisMonthHours          float64
	PendingRequests           AgentPendingRequests
	RecentNotificationsUnread int
}
