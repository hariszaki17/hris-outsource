// Package overtime — E7 overtime + holiday services (F7.1/F7.3/F7.4 / OVT-01/02).
// The web surface is HR/leader OT APPROVAL (the two-level state machine) + the
// HR-maintained public-holiday calendar. Agent OT capture/auto-detect is mobile/
// system and OUT of web scope — OT records are seeded (incl. PENDING_AGENT_CONFIRM
// candidates) so the confirm/approval flows have real targets.
//
// This package owns the two-level approval state machine (PENDING_AGENT_CONFIRM →
// PENDING_L1 → PENDING_HR → APPROVED; reject → REJECTED; withdraw → WITHDRAWN), the
// bulk approve/reject partial-success engine, OT_BELOW_MIN enforcement against the
// EXISTING E2 overtime_rules, day_type classification (schedule + holiday calendar),
// holiday CRUD (HOLIDAY_DATE_CLASH / HOLIDAY_IN_USE), GuardCompany scope +
// SELF_APPROVAL_FORBIDDEN, audit-in-tx + notify stub.
//
// Mirrors the Phase-8 leave slice (two-level approval) and Phase-7 attendance slice
// (bulk partial-success) EXACTLY.
package overtime

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	schedulingsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// TxRunner runs a closure inside a DB transaction (db.TxManager satisfies it).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock supplies the current time (overridable in tests).
type Clock func() time.Time

// --- filters ---

// OvertimeFilter is the decoded GET /overtime query (cursor-paged).
type OvertimeFilter struct {
	EmployeeID           *string
	CompanyID            *string
	Status               *string
	StatusIn             []string
	WorkFrom             *time.Time
	WorkTo               *time.Time
	Tier                 *string
	Source               *string
	FlaggedNoPreapproval *bool
	Limit                int
	CursorCreated        *time.Time
	CursorID             *string
}

// HolidayFilter is the decoded GET /holidays query (cursor-paged, ascending by date).
type HolidayFilter struct {
	Category      *string
	ServiceLineID *string
	Year          *int
	Limit         int
	CursorDate    *time.Time
	CursorID      *string
}

// --- overtime repository port ---

// OvertimeRepository is the data dependency for the overtime service.
type OvertimeRepository interface {
	ListOvertime(ctx context.Context, f OvertimeFilter) ([]dom.Overtime, error)
	GetOvertime(ctx context.Context, id string) (dom.Overtime, error)
	GetOvertimeForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.Overtime, error)
	UpdateOvertimeStatus(ctx context.Context, tx pgx.Tx, id string, status dom.OvertimeStatus) (dom.Overtime, error)
	// InsertOvertime persists a new OT record (F7.2 agent/leader request path).
	InsertOvertime(ctx context.Context, tx pgx.Tx, p OvertimeInsertParams) (dom.Overtime, error)

	InsertOvertimeApproval(ctx context.Context, tx pgx.Tx, p ApprovalRow) (dom.OvertimeApproval, error)
	ListOvertimeApprovals(ctx context.Context, overtimeID string) ([]dom.OvertimeApproval, error)
}

// OvertimeInsertParams carries one overtime INSERT for the F7.2 request path. The
// service denormalizes placement/company/service_line from the resolved active
// placement; id is allocated by the column DEFAULT (nil).
type OvertimeInsertParams struct {
	EmployeeID       string
	CompanyID        *string
	PlacementID      string
	ServiceLineID    *string
	WorkDate         time.Time
	PlannedStartTime *string
	PlannedEndTime   *string
	CrossMidnight    bool
	Source           dom.OvertimeSource
	Status           dom.OvertimeStatus
	DayType          dom.OvertimeTier
	HolidayID        *string
	Reason           *string
	CreatedBy        *string
}

// ApprovalRow carries one overtime_approvals decision-trail insert.
type ApprovalRow struct {
	OvertimeID   string
	Level        int    // 1 = leader/L1, 2 = HR final
	Decision     string // APPROVED | REJECTED | OVERRIDE_APPROVED
	ApproverID   *string
	ApproverName *string
	Reason       *string
}

// --- holiday repository port ---

// HolidayRepository is the data dependency for the holiday service + classification.
type HolidayRepository interface {
	ListHolidays(ctx context.Context, f HolidayFilter) ([]dom.Holiday, error)
	GetHoliday(ctx context.Context, id string) (dom.Holiday, error)
	GetHolidayByDateCategory(ctx context.Context, date time.Time, category string) (dom.Holiday, error)
	GetHolidayForDate(ctx context.Context, date time.Time) (dom.Holiday, error)
	InsertHoliday(ctx context.Context, tx pgx.Tx, p HolidayWriteParams) (dom.Holiday, error)
	UpdateHoliday(ctx context.Context, tx pgx.Tx, id string, p HolidayUpdateParams) (dom.Holiday, error)
	SoftDeleteHoliday(ctx context.Context, tx pgx.Tx, id string) (string, error)
	CountOvertimeUsingHoliday(ctx context.Context, holidayID string) (int64, error)
}

// HolidayWriteParams carries one holiday create.
type HolidayWriteParams struct {
	ID                     *string // nil → DEFAULT sequence id; explicit for seed/E2E
	Name                   string
	Date                   time.Time
	Category               string
	Recurring              bool
	ApplicableServiceLines []string
}

// HolidayUpdateParams carries a partial holiday update (nil = keep current).
type HolidayUpdateParams struct {
	Name                   *string
	Date                   *time.Time
	Category               *string
	Recurring              *bool
	ApplicableServiceLines []string // non-nil replaces the whole array
}

// --- overtime-rule read port (the EXISTING E2/Phase-3 overtime_rules) ---

// OvertimeRule is the subset of the E2 overtime_rules master E7 consumes at
// calculation time: min_minutes (OT_BELOW_MIN) + the per-tier reference multiplier
// (INV-2: stored, never applied).
type OvertimeRule struct {
	ID            string
	ServiceLineID *string
	WeekdayRate   float64
	RestdayRate   float64
	HolidayRate   float64
	MinMinutes    int
}

// RuleRepository is the read-through to the E2 overtime_rules master. The
// line-scoped rule wins over the global rule (OR-2); the global default is the
// fallback. domain.ErrNotFound when no rule exists.
type RuleRepository interface {
	FindOvertimeRule(ctx context.Context, serviceLineID *string) (OvertimeRule, error)
}

// --- schedule read port (day_type WORKDAY/RESTDAY classification) ---

// SchedulePort reads the agent's live schedule entry on a work_date to classify
// WORKDAY (scheduled shift exists) vs RESTDAY (none). Satisfied by the EXISTING
// scheduling repo (its FindLiveEntryForAgentDate). The HOLIDAY tier comes from the
// holiday calendar; TierPrecedence (HOLIDAY>RESTDAY>WORKDAY) resolves overlaps.
type SchedulePort interface {
	FindLiveEntryForAgentDate(ctx context.Context, employeeID string, date time.Time) (schedulingsvc.LiveEntry, error)
	// FindActivePlacementForAgentDate resolves the ACTIVE placement covering
	// work_date (OC-6 placement resolution for the F7.2 request path).
	// domain.ErrNotFound when none.
	FindActivePlacementForAgentDate(ctx context.Context, employeeID string, date time.Time) (schedulingsvc.PlacementCover, error)
	// FindApprovedLeaveForAgentDate returns the approved-leave row covering
	// work_date (OT_OVERLAPS_LEAVE). domain.ErrNotFound when none.
	FindApprovedLeaveForAgentDate(ctx context.Context, employeeID string, date time.Time) (schedulingsvc.ApprovedLeave, error)
}
