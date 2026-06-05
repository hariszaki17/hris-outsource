// Package reporting — DashboardService: the role-aware F10.2 landing dashboard
// (GET /dashboards/me). The response is POV-shaped (DB-1): HR/super_admin →
// HrDashboard (cross-company KPIs + "Perlu Tindakan" approval-inbox panel),
// shift_leader → LeaderDashboard (today's team status + pending counts, scoped to
// the leader's own company E3 INV-3), agent → AgentDashboard. Read-only LIVE
// aggregation over the existing E2..E8 tables (CONTEXT: live for honesty, no
// rollup). Deep-link paths match the openapi examples byte-for-byte.
package reporting

import (
	"context"
	"fmt"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// DashboardService implements GET /dashboards/me.
type DashboardService struct {
	repo DashboardRepository
}

// NewDashboardService wires the dashboard service.
func NewDashboardService(repo DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

// jakartaMonths is the Bahasa month names for period_label.
var jakartaMonths = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// jakartaNow returns the current time in Asia/Jakarta (falls back to UTC+7 if the
// tzdata isn't present in the image).
func jakartaNow() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return time.Now().In(loc)
}

// GetMyDashboard returns the role-shaped dashboard for the caller. The concrete
// type (HrDashboard | LeaderDashboard | AgentDashboard) is returned as `any` and
// the handler wraps it in {data}. The FE discriminates on `role`.
func (s *DashboardService) GetMyDashboard(ctx context.Context) (any, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}
	now := jakartaNow()
	switch p.Role {
	case auth.RoleHRAdmin, auth.RoleSuperAdmin:
		return s.hrDashboard(ctx, p, now)
	case auth.RoleShiftLeader:
		return s.leaderDashboard(ctx, p, now)
	case auth.RoleAgent:
		return s.agentDashboard(ctx, p, now)
	default:
		return nil, apperr.Forbidden()
	}
}

func periodLabel(now time.Time) string {
	return fmt.Sprintf("%s %d", jakartaMonths[int(now.Month())], now.Year())
}

// --- HR / Super Admin ---

func (s *DashboardService) hrDashboard(ctx context.Context, p auth.Principal, now time.Time) (any, error) {
	counts, err := s.repo.HrCounts(ctx, now, nil)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	roleLabel := "HR Admin"
	role := "hr_admin"
	if p.Role == auth.RoleSuperAdmin {
		roleLabel = "Super Admin"
		role = "super_admin"
	}

	// pending_approvals_panel — global deep links (openapi examples).
	panel := make([]dom.ApprovalInboxRow, 0, 4)
	if counts.PendingAttendanceVerify > 0 {
		panel = append(panel, dom.ApprovalInboxRow{
			Kind: dom.InboxAttendanceVerify, Label: "Verifikasi kehadiran", Count: counts.PendingAttendanceVerify,
			DeepLink: dom.DeepLink{Epic: "E5", Path: "/attendance?status=PENDING_VERIFY"},
		})
	}
	if counts.PendingLeaveApprove > 0 {
		panel = append(panel, dom.ApprovalInboxRow{
			Kind: dom.InboxLeaveApprove, Label: "Persetujuan cuti", Count: counts.PendingLeaveApprove,
			DeepLink: dom.DeepLink{Epic: "E6", Path: "/leave-requests?status=PENDING_L1,PENDING_L2"},
		})
	}
	if counts.PendingOTApprove > 0 {
		panel = append(panel, dom.ApprovalInboxRow{
			Kind: dom.InboxOTApprove, Label: "Persetujuan lembur", Count: counts.PendingOTApprove,
			DeepLink: dom.DeepLink{Epic: "E7", Path: "/overtime?status=PENDING"},
		})
	}
	if counts.ExpiringPlacements30d > 0 {
		panel = append(panel, dom.ApprovalInboxRow{
			Kind: dom.InboxPlacementExpiring, Label: "Penempatan akan berakhir", Count: counts.ExpiringPlacements30d,
			DeepLink: dom.DeepLink{Epic: "E3", Path: "/placements?expiring_within=30d"},
		})
	}

	return dom.HrDashboard{
		Role:        role,
		RoleLabel:   roleLabel,
		GeneratedAt: now.UTC(),
		PeriodLabel: periodLabel(now),
		KPIs: dom.HrKPIs{
			ActivePlacements: counts.ActivePlacements,
			ActiveCompanies:  counts.ActiveCompanies,
			// attendance_rate_pct / billable_hours_mtd / ot_hours_mtd: no dedicated
			// rollup query in 11-01; emitted as 0 (honest absence, not a fake non-zero
			// constant). The required fields are present per openapi.
			AttendanceRatePct: 0,
			BillableHoursMTD:  0,
			OTHoursMTD:        0,
			LeavePending:      counts.PendingLeaveApproveHR,
		},
		ExpiringPlacements30d:    counts.ExpiringPlacements30d,
		ExpiringAgreements30d:    counts.ExpiringAgreements30d,
		AttendanceAnomaliesToday: counts.PendingAttendanceVerify,
		BillableTrend:            dom.BillableTrend{Granularity: "day", Points: []dom.BillableTrendPoint{}},
		PendingApprovalsPanel:    panel,
	}, nil
}

// --- Shift Leader ---

func (s *DashboardService) leaderDashboard(ctx context.Context, p auth.Principal, now time.Time) (any, error) {
	companyID := p.CompanyID
	if companyID == "" {
		return nil, apperr.OutOfScope()
	}

	today, err := s.repo.LeaderToday(ctx, now, companyID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	av, la, ot, err := s.repo.LeaderPending(ctx, companyID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	name, err := s.repo.CompanyName(ctx, companyID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	// pending_approvals_panel — company-scoped deep links (openapi examples).
	panel := make([]dom.ApprovalInboxRow, 0, 3)
	if av > 0 {
		panel = append(panel, dom.ApprovalInboxRow{
			Kind: dom.InboxAttendanceVerify, Label: "Verifikasi kehadiran", Count: av,
			DeepLink: dom.DeepLink{Epic: "E5", Path: "/attendance?company_id=" + companyID + "&status=PENDING_VERIFY"},
		})
	}
	if la > 0 {
		panel = append(panel, dom.ApprovalInboxRow{
			Kind: dom.InboxLeaveApprove, Label: "Persetujuan cuti", Count: la,
			DeepLink: dom.DeepLink{Epic: "E6", Path: "/leave-requests?company_id=" + companyID + "&status=PENDING_L1"},
		})
	}
	if ot > 0 {
		panel = append(panel, dom.ApprovalInboxRow{
			Kind: dom.InboxOTApprove, Label: "Persetujuan lembur", Count: ot,
			DeepLink: dom.DeepLink{Epic: "E7", Path: "/overtime?company_id=" + companyID + "&status=PENDING"},
		})
	}

	return dom.LeaderDashboard{
		Role:        "shift_leader",
		RoleLabel:   "Shift Leader",
		Company:     dom.LeaderCompany{ID: companyID, Name: name},
		GeneratedAt: now.UTC(),
		Today: dom.LeaderToday{
			Date:                 now.Format("2006-01-02"),
			ShiftsTotal:          today.ShiftsTotal,
			ClockedIn:            today.ClockedIn,
			LateCount:            today.LateCount,
			AbsentCount:          today.AbsentCount,
			PendingVerifications: today.PendingVerifications,
		},
		PendingCounts: dom.LeaderPendingCounts{
			AttendanceVerify: av,
			LeaveApprove:     la,
			OTApprove:        ot,
		},
		ScheduleAlerts:        []dom.ScheduleAlert{}, // no coverage-gap rollup in 11-01; required field present (empty)
		PendingApprovalsPanel: panel,
	}, nil
}

// --- Agent ---

func (s *DashboardService) agentDashboard(ctx context.Context, p auth.Principal, now time.Time) (any, error) {
	empID := p.EmployeeID
	recent := dom.AgentRecentAttendance{}
	pending := dom.AgentPendingRequests{}
	if empID != "" {
		r, err := s.repo.AgentRecent(ctx, empID, now)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		recent = dom.AgentRecentAttendance{Last7dPresent: r.Present, Last7dLate: r.Late, Last7dAbsent: r.Absent}

		pr, err := s.repo.AgentPending(ctx, empID)
		if err != nil {
			return nil, apperr.Internal(err)
		}
		pending = dom.AgentPendingRequests{Leave: pr.Leave, OT: pr.OT}
	}

	unread, err := s.repo.CountUnread(ctx, selfRecipientsList(p))
	if err != nil {
		return nil, apperr.Internal(err)
	}

	return dom.AgentDashboard{
		Role:             "agent",
		GeneratedAt:      now.UTC(),
		TodayShift:       nil, // no today-shift query in 11-01; nullable per openapi (off-duty state)
		RecentAttendance: recent,
		// leave_balance / ot_this_month_hours: no dedicated query in 11-01 → honest
		// zero/empty (required fields present per openapi).
		LeaveBalance:              dom.AgentLeaveBalance{AnnualRemainingDays: 0, AnnualQuotaDays: 0, PeriodLabel: fmt.Sprintf("%d", now.Year())},
		OTThisMonthHours:          0,
		PendingRequests:           pending,
		RecentNotificationsUnread: unread,
	}, nil
}

// selfRecipientsList builds the unread-count recipient set from a principal.
func selfRecipientsList(p auth.Principal) []string {
	ids := make([]string, 0, 2)
	if p.UserID != "" {
		ids = append(ids, p.UserID)
	}
	if p.EmployeeID != "" {
		ids = append(ids, p.EmployeeID)
	}
	return ids
}
