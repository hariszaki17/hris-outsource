// Package reporting (handler) — request/response DTOs + snake_case mappers
// matching docs/api/E10-reporting/openapi.yaml byte-for-shape. The Notification
// wire shape: read_at is a pointer WITHOUT omitempty (serializes JSON null when
// unread); deep_link + actor are ALWAYS objects (the openapi marks both required).
package reporting

import (
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
)

// --- generic envelopes ---

type dataResponse[T any] struct {
	Data T `json:"data"`
}

// notificationPageResponse is the GET /notifications cursor envelope
// (CONVENTIONS §8 / openapi CursorPage allOf).
type notificationPageResponse struct {
	Data       []notificationResponse `json:"data"`
	NextCursor *string                `json:"next_cursor"`
	HasMore    bool                   `json:"has_more"`
}

// --- request bodies ---

type markAllReadRequest struct {
	BeforeTimestamp *string `json:"before_timestamp"`
}

// --- response: Notification (openapi schemas.Notification) ---

type deepLinkResponse struct {
	Epic     string  `json:"epic"`
	EntityID *string `json:"entity_id"` // null when no entity (no omitempty)
	Path     string  `json:"path"`
}

type actorResponse struct {
	ID    *string `json:"id"` // null = system actor (no omitempty)
	Label string  `json:"label"`
}

type notificationResponse struct {
	ID         string           `json:"id"`
	Kind       string           `json:"kind"`
	Title      string           `json:"title"`
	Body       string           `json:"body"`
	ReadAt     *string          `json:"read_at"` // null = unread (no omitempty)
	CreatedAt  string           `json:"created_at"`
	DeepLink   deepLinkResponse `json:"deep_link"`
	Actor      actorResponse    `json:"actor"`
	IsCritical bool             `json:"is_critical"`
}

// --- response: mark-all-read ---

type markAllReadResponse struct {
	MarkedCount int `json:"marked_count"`
}

// --- mappers ---

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func rfc3339Ptr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

// ================================================================
// DASHBOARD (openapi Dashboard oneOf — role-discriminated)
// ================================================================

type deepLinkObj struct {
	Epic     string  `json:"epic"`
	EntityID *string `json:"entity_id,omitempty"`
	Path     string  `json:"path"`
}

type approvalInboxRowResp struct {
	Kind     string      `json:"kind"`
	Label    string      `json:"label"`
	Count    int         `json:"count"`
	DeepLink deepLinkObj `json:"deep_link"`
}

type hrKPIsResp struct {
	ActivePlacements  int     `json:"active_placements"`
	ActiveCompanies   int     `json:"active_companies"`
	AttendanceRatePct float64 `json:"attendance_rate_pct"`
	BillableHoursMTD  float64 `json:"billable_hours_mtd"`
	OTHoursMTD        float64 `json:"ot_hours_mtd"`
	LeavePending      int     `json:"leave_pending"`
}

type billableTrendPointResp struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

type billableTrendResp struct {
	Granularity string                   `json:"granularity"`
	Points      []billableTrendPointResp `json:"points"`
}

type hrDashboardResp struct {
	Role                     string                 `json:"role"`
	RoleLabel                string                 `json:"role_label"`
	GeneratedAt              string                 `json:"generated_at"`
	PeriodLabel              string                 `json:"period_label"`
	KPIs                     hrKPIsResp             `json:"kpis"`
	ExpiringPlacements30d    int                    `json:"expiring_placements_30d"`
	ExpiringAgreements30d    int                    `json:"expiring_agreements_30d"`
	AttendanceAnomaliesToday int                    `json:"attendance_anomalies_today"`
	BillableTrend            billableTrendResp      `json:"billable_trend"`
	PendingApprovalsPanel    []approvalInboxRowResp `json:"pending_approvals_panel"`
	// Admin is present ONLY for super_admin (omitempty → omitted for hr_admin, C-6).
	Admin *superAdminWidgetsResp `json:"admin,omitempty"`
}

// --- SuperAdminWidgets (openapi schemas.SuperAdminWidgets, DB-7) ---

type userAccessResp struct {
	ActiveUsers         int `json:"active_users"`
	PendingProvisioning int `json:"pending_provisioning"`
	Offboarded30d       int `json:"offboarded_30d"`
}

type auditEntryResp struct {
	ID          string `json:"id"`
	ActorLabel  string `json:"actor_label"`
	Action      string `json:"action"`
	TargetLabel string `json:"target_label"`
	At          string `json:"at"`
}

type orgRollupResp struct {
	ServiceLine      string `json:"service_line"`
	Headcount        int    `json:"headcount"`
	ActivePlacements int    `json:"active_placements"`
}

type pendingGrantsResp struct {
	BankApprovals int `json:"bank_approvals"`
	RoleRequests  int `json:"role_requests"`
}

type superAdminWidgetsResp struct {
	UserAccess    userAccessResp    `json:"user_access"`
	RecentAudit   []auditEntryResp  `json:"recent_audit"`
	OrgRollups    []orgRollupResp   `json:"org_rollups"`
	PendingGrants pendingGrantsResp `json:"pending_grants"`
}

type leaderCompanyResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type leaderTodayResp struct {
	Date                 string `json:"date"`
	ShiftsTotal          int    `json:"shifts_total"`
	ClockedIn            int    `json:"clocked_in"`
	LateCount            int    `json:"late_count"`
	AbsentCount          int    `json:"absent_count"`
	PendingVerifications int    `json:"pending_verifications"`
}

type leaderPendingCountsResp struct {
	AttendanceVerify int `json:"attendance_verify"`
	LeaveApprove     int `json:"leave_approve"`
	OTApprove        int `json:"ot_approve"`
}

type scheduleAlertResp struct {
	Kind     string      `json:"kind"`
	Label    string      `json:"label"`
	Date     *string     `json:"date"`
	DeepLink deepLinkObj `json:"deep_link"`
}

type leaderDashboardResp struct {
	Role                  string                  `json:"role"`
	RoleLabel             string                  `json:"role_label"`
	Company               leaderCompanyResp       `json:"company"`
	GeneratedAt           string                  `json:"generated_at"`
	Today                 leaderTodayResp         `json:"today"`
	PendingCounts         leaderPendingCountsResp `json:"pending_counts"`
	ScheduleAlerts        []scheduleAlertResp     `json:"schedule_alerts"`
	PendingApprovalsPanel []approvalInboxRowResp  `json:"pending_approvals_panel"`
}

type agentTodayShiftResp struct {
	ScheduleID    string       `json:"schedule_id"`
	ShiftName     string       `json:"shift_name"`
	StartTime     string       `json:"start_time"`
	EndTime       string       `json:"end_time"`
	CompanyName   string       `json:"company_name"`
	ClockInStatus string       `json:"clock_in_status"`
	DeepLink      *deepLinkObj `json:"deep_link,omitempty"`
}

type agentRecentResp struct {
	Last7dPresent int `json:"last_7d_present"`
	Last7dLate    int `json:"last_7d_late"`
	Last7dAbsent  int `json:"last_7d_absent"`
}

type agentLeaveBalanceResp struct {
	AnnualRemainingDays float64 `json:"annual_remaining_days"`
	AnnualQuotaDays     float64 `json:"annual_quota_days"`
	PeriodLabel         string  `json:"period_label"`
}

type agentPendingResp struct {
	Leave int `json:"leave"`
	OT    int `json:"ot"`
}

type agentDashboardResp struct {
	Role                      string                `json:"role"`
	GeneratedAt               string                `json:"generated_at"`
	TodayShift                *agentTodayShiftResp  `json:"today_shift"`
	RecentAttendance          agentRecentResp       `json:"recent_attendance"`
	LeaveBalance              agentLeaveBalanceResp `json:"leave_balance"`
	OTThisMonthHours          float64               `json:"ot_this_month_hours"`
	PendingRequests           agentPendingResp      `json:"pending_requests"`
	RecentNotificationsUnread int                   `json:"recent_notifications_unread"`
}

func toDeepLink(d dom.DeepLink) deepLinkObj {
	return deepLinkObj{Epic: d.Epic, EntityID: d.EntityID, Path: d.Path}
}

func toApprovalRows(rows []dom.ApprovalInboxRow) []approvalInboxRowResp {
	out := make([]approvalInboxRowResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, approvalInboxRowResp{
			Kind:     string(r.Kind),
			Label:    r.Label,
			Count:    r.Count,
			DeepLink: toDeepLink(r.DeepLink),
		})
	}
	return out
}

// toDashboard maps the role-shaped domain payload to its wire DTO.
func toDashboard(v any) any {
	switch d := v.(type) {
	case dom.HrDashboard:
		return hrDashboardResp{
			Role:        d.Role,
			RoleLabel:   d.RoleLabel,
			GeneratedAt: rfc3339(d.GeneratedAt),
			PeriodLabel: d.PeriodLabel,
			KPIs: hrKPIsResp{
				ActivePlacements:  d.KPIs.ActivePlacements,
				ActiveCompanies:   d.KPIs.ActiveCompanies,
				AttendanceRatePct: d.KPIs.AttendanceRatePct,
				BillableHoursMTD:  d.KPIs.BillableHoursMTD,
				OTHoursMTD:        d.KPIs.OTHoursMTD,
				LeavePending:      d.KPIs.LeavePending,
			},
			ExpiringPlacements30d:    d.ExpiringPlacements30d,
			ExpiringAgreements30d:    d.ExpiringAgreements30d,
			AttendanceAnomaliesToday: d.AttendanceAnomaliesToday,
			BillableTrend: billableTrendResp{
				Granularity: d.BillableTrend.Granularity,
				Points:      toTrendPoints(d.BillableTrend.Points),
			},
			PendingApprovalsPanel: toApprovalRows(d.PendingApprovalsPanel),
			Admin:                 toSuperAdminWidgets(d.Admin),
		}
	case dom.LeaderDashboard:
		alerts := make([]scheduleAlertResp, 0, len(d.ScheduleAlerts))
		for _, a := range d.ScheduleAlerts {
			alerts = append(alerts, scheduleAlertResp{
				Kind: a.Kind, Label: a.Label, Date: a.Date, DeepLink: toDeepLink(a.DeepLink),
			})
		}
		return leaderDashboardResp{
			Role:        d.Role,
			RoleLabel:   d.RoleLabel,
			Company:     leaderCompanyResp{ID: d.Company.ID, Name: d.Company.Name},
			GeneratedAt: rfc3339(d.GeneratedAt),
			Today: leaderTodayResp{
				Date:                 d.Today.Date,
				ShiftsTotal:          d.Today.ShiftsTotal,
				ClockedIn:            d.Today.ClockedIn,
				LateCount:            d.Today.LateCount,
				AbsentCount:          d.Today.AbsentCount,
				PendingVerifications: d.Today.PendingVerifications,
			},
			PendingCounts: leaderPendingCountsResp{
				AttendanceVerify: d.PendingCounts.AttendanceVerify,
				LeaveApprove:     d.PendingCounts.LeaveApprove,
				OTApprove:        d.PendingCounts.OTApprove,
			},
			ScheduleAlerts:        alerts,
			PendingApprovalsPanel: toApprovalRows(d.PendingApprovalsPanel),
		}
	case dom.AgentDashboard:
		var todayShift *agentTodayShiftResp
		if d.TodayShift != nil {
			var dl *deepLinkObj
			if d.TodayShift.DeepLink != nil {
				v := toDeepLink(*d.TodayShift.DeepLink)
				dl = &v
			}
			todayShift = &agentTodayShiftResp{
				ScheduleID:    d.TodayShift.ScheduleID,
				ShiftName:     d.TodayShift.ShiftName,
				StartTime:     d.TodayShift.StartTime,
				EndTime:       d.TodayShift.EndTime,
				CompanyName:   d.TodayShift.CompanyName,
				ClockInStatus: d.TodayShift.ClockInStatus,
				DeepLink:      dl,
			}
		}
		return agentDashboardResp{
			Role:        d.Role,
			GeneratedAt: rfc3339(d.GeneratedAt),
			TodayShift:  todayShift,
			RecentAttendance: agentRecentResp{
				Last7dPresent: d.RecentAttendance.Last7dPresent,
				Last7dLate:    d.RecentAttendance.Last7dLate,
				Last7dAbsent:  d.RecentAttendance.Last7dAbsent,
			},
			LeaveBalance: agentLeaveBalanceResp{
				AnnualRemainingDays: d.LeaveBalance.AnnualRemainingDays,
				AnnualQuotaDays:     d.LeaveBalance.AnnualQuotaDays,
				PeriodLabel:         d.LeaveBalance.PeriodLabel,
			},
			OTThisMonthHours:          d.OTThisMonthHours,
			PendingRequests:           agentPendingResp{Leave: d.PendingRequests.Leave, OT: d.PendingRequests.OT},
			RecentNotificationsUnread: d.RecentNotificationsUnread,
		}
	default:
		return v
	}
}

// toSuperAdminWidgets maps the admin-only block (nil → nil so the `admin` field is
// omitted entirely for hr_admin, C-6). recent_audit / org_rollups are always arrays
// (never null) per openapi (empty → client renders the empty-state, C-5).
func toSuperAdminWidgets(w *dom.SuperAdminWidgets) *superAdminWidgetsResp {
	if w == nil {
		return nil
	}
	audit := make([]auditEntryResp, 0, len(w.RecentAudit))
	for _, a := range w.RecentAudit {
		audit = append(audit, auditEntryResp{
			ID:          a.ID,
			ActorLabel:  a.ActorLabel,
			Action:      a.Action,
			TargetLabel: a.TargetLabel,
			At:          rfc3339(a.At),
		})
	}
	rollups := make([]orgRollupResp, 0, len(w.OrgRollups))
	for _, r := range w.OrgRollups {
		rollups = append(rollups, orgRollupResp{
			ServiceLine:      string(r.ServiceLine),
			Headcount:        r.Headcount,
			ActivePlacements: r.ActivePlacements,
		})
	}
	return &superAdminWidgetsResp{
		UserAccess: userAccessResp{
			ActiveUsers:         w.UserAccess.ActiveUsers,
			PendingProvisioning: w.UserAccess.PendingProvisioning,
			Offboarded30d:       w.UserAccess.Offboarded30d,
		},
		RecentAudit: audit,
		OrgRollups:  rollups,
		PendingGrants: pendingGrantsResp{
			BankApprovals: w.PendingGrants.BankApprovals,
			RoleRequests:  w.PendingGrants.RoleRequests,
		},
	}
}

func toTrendPoints(pts []dom.BillableTrendPoint) []billableTrendPointResp {
	out := make([]billableTrendPointResp, 0, len(pts))
	for _, p := range pts {
		out = append(out, billableTrendPointResp{Date: p.Date, Value: p.Value})
	}
	return out
}

// ================================================================
// BILLABLE REPORT (openapi BillableReport)
// ================================================================

type billableFiltersResp struct {
	CompanyID       *string `json:"company_id"`
	CompanyName     *string `json:"company_name"`
	ServiceLineID   *string `json:"service_line_id"`
	ServiceLineName *string `json:"service_line_name"`
	PeriodStart     string  `json:"period_start"`
	PeriodEnd       string  `json:"period_end"`
	GroupBy         string  `json:"group_by"`
}

type billableSummaryResp struct {
	TotalBillableHours   float64  `json:"total_billable_hours"`
	TotalWorkedHours     float64  `json:"total_worked_hours"`
	TotalPayableHours    float64  `json:"total_payable_hours"`
	TotalVerifiedRecords int      `json:"total_verified_records"`
	VerificationRatePct  *float64 `json:"verification_rate_pct"`
}

type billablePendingResp struct {
	PendingRecords       int     `json:"pending_records"`
	PendingHoursEstimate float64 `json:"pending_hours_estimate"`
	Note                 string  `json:"note"`
}

type billableRowResp struct {
	GroupKey              string  `json:"group_key"`
	GroupLabel            string  `json:"group_label"`
	CompanyID             *string `json:"company_id"`
	CompanyName           *string `json:"company_name"`
	ServiceLineID         *string `json:"service_line_id"`
	ServiceLineName       *string `json:"service_line_name"`
	WorkedHours           float64 `json:"worked_hours"`
	BillableHours         float64 `json:"billable_hours"`
	PayableHours          float64 `json:"payable_hours"`
	VerifiedRecordCount   int     `json:"verified_record_count"`
	UnverifiedRecordCount int     `json:"unverified_record_count"`
}

type billableReportResp struct {
	GeneratedAt    string              `json:"generated_at"`
	Filters        billableFiltersResp `json:"filters"`
	Summary        billableSummaryResp `json:"summary"`
	PendingSummary billablePendingResp `json:"pending_summary"`
	Rows           []billableRowResp   `json:"rows"`
}

func toBillableReport(r dom.BillableReport) billableReportResp {
	rows := make([]billableRowResp, 0, len(r.Rows))
	for _, row := range r.Rows {
		rows = append(rows, billableRowResp{
			GroupKey:              row.GroupKey,
			GroupLabel:            row.GroupLabel,
			CompanyID:             row.CompanyID,
			CompanyName:           row.CompanyName,
			ServiceLineID:         row.ServiceLineID,
			ServiceLineName:       row.ServiceLineName,
			WorkedHours:           row.WorkedHours,
			BillableHours:         row.BillableHours,
			PayableHours:          row.PayableHours,
			VerifiedRecordCount:   row.VerifiedRecordCount,
			UnverifiedRecordCount: row.UnverifiedRecordCount,
		})
	}
	return billableReportResp{
		GeneratedAt: rfc3339(r.GeneratedAt),
		Filters: billableFiltersResp{
			CompanyID:       r.Filters.CompanyID,
			CompanyName:     r.Filters.CompanyName,
			ServiceLineID:   r.Filters.ServiceLineID,
			ServiceLineName: r.Filters.ServiceLineName,
			PeriodStart:     r.Filters.PeriodStart,
			PeriodEnd:       r.Filters.PeriodEnd,
			GroupBy:         string(r.Filters.GroupBy),
		},
		Summary: billableSummaryResp{
			TotalBillableHours:   r.Summary.TotalBillableHours,
			TotalWorkedHours:     r.Summary.TotalWorkedHours,
			TotalPayableHours:    r.Summary.TotalPayableHours,
			TotalVerifiedRecords: r.Summary.TotalVerifiedRecords,
			VerificationRatePct:  r.Summary.VerificationRatePct,
		},
		PendingSummary: billablePendingResp{
			PendingRecords:       r.PendingSummary.PendingRecords,
			PendingHoursEstimate: r.PendingSummary.PendingHoursEstimate,
			Note:                 r.PendingSummary.Note,
		},
		Rows: rows,
	}
}

// ================================================================
// EXPORTS (openapi ExportJob) — DB→WIRE status mapping
// ================================================================

type exportRequestBody struct {
	ReportType   string         `json:"report_type"`
	Format       string         `json:"format"`
	Confidential bool           `json:"confidential"`
	Filters      map[string]any `json:"filters"`
}

type exportErrorObj struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type exportJobResponse struct {
	ID              string          `json:"id"`
	ReportType      string          `json:"report_type"`
	Status          string          `json:"status"`
	Format          string          `json:"format"`
	Confidential    bool            `json:"confidential"`
	ProgressPercent *int            `json:"progress_percent"`
	Filename        *string         `json:"filename"`
	SizeBytes       *int64          `json:"size_bytes"`
	FileURL         *string         `json:"file_url"`
	Error           *exportErrorObj `json:"error"`
	Filters         map[string]any  `json:"filters"`
	AuditLogEntryID string          `json:"audit_log_entry_id"`
	RequesterID     string          `json:"requester_id"`
	RequestedAt     string          `json:"requested_at"`
	CompletedAt     *string         `json:"completed_at"`
	ExpiresAt       *string         `json:"expires_at"`
}

// mapExportStatus maps the DB status to the wire ExportStatus enum (the FE reads
// QUEUED/PROCESSING/COMPLETED/FAILED/CANCELLED): RUNNING→PROCESSING, DONE→COMPLETED.
func mapExportStatus(s dom.ExportStatus) string {
	switch s {
	case dom.StatusRunning:
		return "PROCESSING"
	case dom.StatusDone:
		return "COMPLETED"
	default:
		return string(s) // QUEUED / FAILED / CANCELLED pass through
	}
}

// mapExportFormat normalizes the DB format to the wire enum (XLSX→EXCEL).
func mapExportFormat(f dom.ExportFormat) string {
	if f == dom.FormatXLSX {
		return string(dom.FormatExcel)
	}
	return string(f)
}

func toExportJob(j dom.ExportJob) exportJobResponse {
	resp := exportJobResponse{
		ID:              j.ID,
		ReportType:      string(j.ReportType),
		Status:          mapExportStatus(j.Status),
		Format:          mapExportFormat(j.Format),
		Confidential:    j.Confidential,
		ProgressPercent: j.ProgressPercent,
		SizeBytes:       j.SizeBytes,
		Filters:         j.Filters,
		RequesterID:     j.RequesterID,
		RequestedAt:     rfc3339(j.RequestedAt),
		CompletedAt:     rfc3339Ptr(j.CompletedAt),
		ExpiresAt:       rfc3339Ptr(j.ExpiresAt),
	}
	if j.AuditLogEntryID != nil {
		resp.AuditLogEntryID = *j.AuditLogEntryID
	}
	// filename + file_url present once COMPLETED (DB DONE).
	if j.Status == dom.StatusDone {
		resp.Filename = j.Filename
		url := "/api/v1/exports/" + j.ID + "/download"
		resp.FileURL = &url
	}
	// error{code,message} only when FAILED.
	if j.Status == dom.StatusFailed {
		code := "INTERNAL"
		if j.ErrCode != nil {
			code = *j.ErrCode
		}
		msg := ""
		if j.ErrMessage != nil {
			msg = *j.ErrMessage
		}
		resp.Error = &exportErrorObj{Code: code, Message: msg}
	}
	if resp.Filters == nil {
		resp.Filters = map[string]any{}
	}
	return resp
}

func toNotification(n dom.Notification) notificationResponse {
	return notificationResponse{
		ID:        n.ID,
		Kind:      string(n.Kind),
		Title:     n.Title,
		Body:      n.Body,
		ReadAt:    rfc3339Ptr(n.ReadAt),
		CreatedAt: rfc3339(n.CreatedAt),
		DeepLink: deepLinkResponse{
			Epic:     n.DeepLink.Epic,
			EntityID: n.DeepLink.EntityID,
			Path:     n.DeepLink.Path,
		},
		Actor: actorResponse{
			ID:    n.Actor.ID,
			Label: n.Actor.Label,
		},
		IsCritical: n.IsCritical,
	}
}
