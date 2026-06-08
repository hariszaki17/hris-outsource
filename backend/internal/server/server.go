// Package server assembles the chi router: the global middleware chain, health
// and metrics endpoints, and the /api/v1 routes (public vs authenticated). New
// epics mount their (oapi-codegen) routers under the authenticated group.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	attendancehttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/attendance"
	foundationshttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/foundations"
	identityhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/identity"
	leavehttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/leave"
	orghttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/org"
	overtimehttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/overtime"
	payrollhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/payroll"
	peoplehttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/people"
	placementhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/placement"
	reportinghttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/reporting"
	schedulinghttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/scheduling"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/idempotency"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/obs"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// Deps are everything the router needs, built in cmd/api.
type Deps struct {
	AllowedOrigins []string
	RatePerMinute  int
	RateBurst      int

	Auth        *identityhttp.Handler
	Foundations *foundationshttp.Handler
	// ORG slice (03-02): client companies + sites.
	// Siblings 03-03 (service lines + positions) and 03-04 (master data) should
	// add their own Deps fields here (OrgServiceLines, OrgMasterData) — or reuse
	// OrgCompanies if they share the same handler package (they don't in this plan;
	// they use separate packages). See 03-02-SUMMARY.md for the coordination contract.
	OrgCompanies    *orghttp.Handler
	OrgServiceLines *orghttp.ServiceLineHandler // 03-03: service lines + positions
	OrgMasterData   *orghttp.MasterDataHandler  // 03-04: leave types, attendance codes, overtime rules
	// PEOPLE slice (04-02): employees (E2 F2.1 / PPL-01).
	// Siblings 04-03 (agreements) and 04-04 (change-requests) append their own
	// Deps fields here — see 04-02-SUMMARY.md for the coordination contract.
	People               *peoplehttp.Handler
	PeopleAgreements     *peoplehttp.AgreementHandler     // 04-03: agreements + attachments + file download
	PeopleChangeRequests *peoplehttp.ChangeRequestHandler // 04-04: change-request HR approval queue
	// PLACEMENT slice (05-02): placements + lifecycle + shift-leader + roster (E3).
	Placement *placementhttp.Handler
	// SCHEDULING slice (06-02): shift masters + schedule grid + conflict engine (E4).
	Scheduling *schedulinghttp.Handler
	// ATTENDANCE slice (07-02): verify/reject (+bulk) + corrections (E5).
	Attendance *attendancehttp.Handler
	// ATTENDANCE clock slice (F5.1): agent mobile clock-in/out (E5).
	Clock *attendancehttp.ClockHandler
	// LEAVE slice (08-02): approval state machine + quotas + calendar (E6).
	Leave *leavehttp.Handler
	// OVERTIME slice (09-02): OT two-level approval + holiday calendar (E7).
	Overtime *overtimehttp.Handler
	// PAYROLL slice (10-02): historical payslip archive + audit notes + async export (E8).
	Payroll *payrollhttp.Handler
	// REPORTING slice (11-02): E10 notifications (list/mark-read/mark-all-read).
	// 11-02b extends the SAME handler with dashboard/billable-report/export methods.
	Reporting   *reportinghttp.Handler
	Authn       *auth.Authenticator
	Idempotency *idempotency.Middleware
	Obs         *obs.Providers
}

// New builds the root HTTP handler.
func New(d Deps) http.Handler {
	r := chi.NewRouter()

	// Global chain (order matters): request id -> access log -> panic recovery
	// -> secure headers -> CORS. otelhttp wraps the whole handler at the end.
	r.Use(httpx.RequestIDMiddleware)
	r.Use(httpx.AccessLog)
	r.Use(httpx.Recover)
	r.Use(httpx.SecureHeaders)
	r.Use(httpx.CORS(d.AllowedOrigins))

	// Ops endpoints (no auth, not under /api/v1).
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Handle("/metrics", d.Obs.MetricsHandler())

	rl := httpx.NewRateLimiter(d.RatePerMinute, d.RateBurst, auth.RateLimitKey)

	r.Route("/api/v1", func(r chi.Router) {
		// --- Public (CONVENTIONS §1: only login + forgot-password are public;
		//     refresh/logout carry their own refresh credential, not an access token).
		r.Group(func(r chi.Router) {
			r.Use(rl.Middleware) // keyed by IP for unauthenticated calls
			r.Post("/auth/login", d.Auth.Login)
			r.Post("/auth/refresh", d.Auth.Refresh)
			r.Post("/auth/logout", d.Auth.Logout)
			r.Post("/auth/forgot-password", d.Auth.ForgotPassword)
			r.Post("/auth/reset-password", d.Auth.ResetPassword)
		})

		// --- Authenticated: access token required, then per-user rate limit.
		r.Group(func(r chi.Router) {
			r.Use(d.Authn.Require)
			r.Use(rl.Middleware)
			r.Get("/auth/me", d.Auth.Me)
			// Authenticated self-service password change (EP-3 forced rotation + voluntary).
			r.Post("/auth/change-password", d.Auth.ChangePassword)

			// E1 Foundations: user management, audit-log, platform settings.
			// All endpoints require super_admin or hr_admin (CONVENTIONS §17 x-rbac).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))

				// Users management
				r.Get("/users", d.Foundations.ListUsers)
				// D1 (2026-06-07): standalone user-create removed — every user is
				// auto-provisioned at employee create (POST /employees). Admin role is
				// then set via :change-role. Listing/role/status management stay here.
				r.Patch("/users/{user_id}", d.Foundations.UpdateUser)
				// Action endpoints: chi matches the literal ':' suffix on the path param route.
				r.With(d.Idempotency.Handler).Post("/users/{user_id}:change-role", d.Foundations.ChangeUserRole)
				r.With(d.Idempotency.Handler).Post("/users/{user_id}:deactivate", d.Foundations.DeactivateUser)
				r.With(d.Idempotency.Handler).Post("/users/{user_id}:reactivate", d.Foundations.ReactivateUser)
				r.Post("/users/{user_id}:send-password-reset", d.Foundations.SendUserPasswordReset)

				// Audit log
				r.Get("/audit-log", d.Foundations.ListAuditLog)
				r.Get("/audit-log/{audit_log_id}", d.Foundations.GetAuditLogEntry)

				// Platform settings (read-only)
				r.Get("/platform/settings", d.Foundations.GetPlatformSettings)
			})

			// ---------------------------------------------------------------
			// ORG slice (03-02): client companies + sites (E2 F2.3 + F2.6).
			// COORDINATION POINT: 03-03 and 03-04 append THEIR OWN Group
			// blocks immediately after this closing brace. Do NOT modify the
			// foundations group above. Each sibling owns its own r.Group{}.
			// ---------------------------------------------------------------

			// Reads: hr_admin, super_admin, shift_leader (company_or_global scope).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.Get("/client-companies", d.OrgCompanies.ListClientCompanies)
				r.Get("/client-companies/{client_company_id}", d.OrgCompanies.GetClientCompany)
				r.Get("/client-companies/{client_company_id}/sites", d.OrgCompanies.ListSites)
				r.Get("/sites/{site_id}", d.OrgCompanies.GetSite)
			})

			// Writes: hr_admin, super_admin (global scope).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.With(d.Idempotency.Handler).Post("/client-companies", d.OrgCompanies.CreateClientCompany)
				r.Patch("/client-companies/{client_company_id}", d.OrgCompanies.UpdateClientCompany)
				r.With(d.Idempotency.Handler).Post("/client-companies/{client_company_id}:deactivate", d.OrgCompanies.DeactivateClientCompany)
				r.With(d.Idempotency.Handler).Post("/client-companies/{client_company_id}:reactivate", d.OrgCompanies.ReactivateClientCompany)
				r.With(d.Idempotency.Handler).Post("/client-companies/{client_company_id}/sites", d.OrgCompanies.CreateSite)
				r.Patch("/sites/{site_id}", d.OrgCompanies.UpdateSite)
				r.With(d.Idempotency.Handler).Post("/sites/{site_id}:deactivate", d.OrgCompanies.DeactivateSite)
			})
			// ORG slice end (03-02). 03-03 sibling: append r.Group{} here.

			// ---------------------------------------------------------------
			// ORG slice (03-03): service lines + positions (E2 F2.4).
			// COORDINATION POINT: 03-04 appends its OWN Group block after this
			// closing brace. Do NOT modify 03-02 or 03-03 groups.
			// ---------------------------------------------------------------

			// Service-line + position reads: all authenticated roles.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
				r.Get("/service-lines", d.OrgServiceLines.ListServiceLines)
				r.Get("/service-lines/{service_line_id}", d.OrgServiceLines.GetServiceLine)
				r.Get("/service-lines/{service_line_id}/positions", d.OrgServiceLines.ListPositionsInServiceLine)
			})

			// Service-line writes: super_admin only.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin))
				r.With(d.Idempotency.Handler).Post("/service-lines", d.OrgServiceLines.CreateServiceLine)
				r.Patch("/service-lines/{service_line_id}", d.OrgServiceLines.UpdateServiceLine)
				r.With(d.Idempotency.Handler).Post("/service-lines/{service_line_id}:discontinue", d.OrgServiceLines.DiscontinueServiceLine)
			})

			// Position writes: super_admin + hr_admin.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.With(d.Idempotency.Handler).Post("/service-lines/{service_line_id}/positions", d.OrgServiceLines.CreatePosition)
				r.Patch("/positions/{position_id}", d.OrgServiceLines.UpdatePosition)
				r.Delete("/positions/{position_id}", d.OrgServiceLines.SoftDeletePosition)
			})
			// ORG slice end (03-03). 03-04 sibling: append r.Group{} here.

			// ---------------------------------------------------------------
			// ORG slice (03-04): operational master data — leave types,
			// attendance codes, overtime rules (E2 F2.5 / ORG-04).
			// ---------------------------------------------------------------

			// Leave-type + attendance-code reads: all roles (including agent).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
				r.Get("/leave-types", d.OrgMasterData.ListLeaveTypes)
				r.Get("/attendance-codes", d.OrgMasterData.ListAttendanceCodes)
			})

			// Overtime-rule reads: super_admin, hr_admin, shift_leader (agent excluded per spec x-rbac).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.Get("/overtime-rules", d.OrgMasterData.ListOvertimeRules)
			})

			// All master-data writes: super_admin, hr_admin.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				// Leave types
				r.With(d.Idempotency.Handler).Post("/leave-types", d.OrgMasterData.CreateLeaveType)
				r.Patch("/leave-types/{leave_type_id}", d.OrgMasterData.UpdateLeaveType)
				r.Delete("/leave-types/{leave_type_id}", d.OrgMasterData.SoftDeleteLeaveType)
				// Attendance codes
				r.With(d.Idempotency.Handler).Post("/attendance-codes", d.OrgMasterData.CreateAttendanceCode)
				r.Patch("/attendance-codes/{attendance_code_id}", d.OrgMasterData.UpdateAttendanceCode)
				r.Delete("/attendance-codes/{attendance_code_id}", d.OrgMasterData.SoftDeleteAttendanceCode)
				// Overtime rules
				r.With(d.Idempotency.Handler).Post("/overtime-rules", d.OrgMasterData.CreateOvertimeRule)
				r.Patch("/overtime-rules/{overtime_rule_id}", d.OrgMasterData.UpdateOvertimeRule)
				r.Delete("/overtime-rules/{overtime_rule_id}", d.OrgMasterData.SoftDeleteOvertimeRule)
			})
			// ORG slice end (03-04). Phase 4+ appends after this line.

			// ---------------------------------------------------------------
			// PEOPLE slice (04-02): employees (E2 F2.1 / PPL-01).
			// COORDINATION POINT: 04-03 (agreements) and 04-04 (change-requests)
			// append THEIR OWN Group blocks after the "PEOPLE slice end" marker
			// below. Do NOT modify this group.
			// ---------------------------------------------------------------

			// Employee list: super_admin, hr_admin, shift_leader (no agent — agents
			// have no roster view). The detail endpoint is split out below so agents
			// can self-read.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.Get("/employees", d.People.ListEmployees)
			})

			// Employee detail: super_admin, hr_admin, shift_leader AND agent.
			// OpenAPI x-rbac for GET /employees/{id} lists agent with scope:self;
			// the service hides anyone but the agent's own record as 404 (no leak).
			// Split from the list group so the agent role applies to the detail only.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
				r.Get("/employees/{employee_id}", d.People.GetEmployee)
			})

			// Employee writes: super_admin, hr_admin.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.With(d.Idempotency.Handler).Post("/employees", d.People.CreateEmployee)
				r.Patch("/employees/{employee_id}", d.People.UpdateEmployee)
				r.With(d.Idempotency.Handler).Post("/employees/{employee_id}:deactivate", d.People.DeactivateEmployee)
				r.With(d.Idempotency.Handler).Post("/employees/{employee_id}:reactivate", d.People.ReactivateEmployee)
				// Login is auto-provisioned at create (D1); admins may re-issue the
				// show-once temp password here.
				r.With(d.Idempotency.Handler).Post("/employees/{employee_id}:regenerate-password", d.People.RegenerateTempPassword)
			})
			// PEOPLE slice end (04-02). 04-03 agreements: append r.Group{} here.

			// ---------------------------------------------------------------
			// PEOPLE agreements slice (04-03): employment agreements + attachments
			// + authenticated file download (E2 F2.2 / PPL-02).
			// COORDINATION POINT: 04-04 (change-requests) appends its OWN Group
			// block after the "PEOPLE agreements slice end" marker below.
			// ---------------------------------------------------------------

			// Agreement reads: hr_admin, super_admin (global scope).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.Get("/agreements", d.PeopleAgreements.ListAgreements)
				r.Get("/agreements/{agreement_id}", d.PeopleAgreements.GetAgreement)
				// Authenticated file download — served in same read group so
				// shift_leader or agent could be added later without refactor.
				r.Get("/files/{file_id}", d.PeopleAgreements.DownloadFile)
			})

			// Agreement writes: hr_admin, super_admin.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.With(d.Idempotency.Handler).Post("/agreements", d.PeopleAgreements.CreateAgreement)
				r.With(d.Idempotency.Handler).Post("/agreements/{agreement_id}:renew", d.PeopleAgreements.RenewAgreement)
				r.With(d.Idempotency.Handler).Post("/agreements/{agreement_id}:close", d.PeopleAgreements.CloseAgreement)
				// Multipart upload — NO idempotency (binary body, not JSON; idempotency
				// middleware expects JSON or no-body; spec does not flag this op).
				r.Post("/agreements/{agreement_id}/attachments", d.PeopleAgreements.UploadAttachment)
			})
			// PEOPLE agreements slice end (04-03). 04-04 change-requests: append here.

			// ---------------------------------------------------------------
			// PEOPLE change-requests slice (04-04): HR approval queue for
			// agent-submitted profile-change requests (E2 F2.1 EP-5 / PPL-03).
			// x-rbac: hr_admin, super_admin — no shift_leader or agent access.
			// ---------------------------------------------------------------
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				// List + detail reads.
				r.Get("/change-requests", d.PeopleChangeRequests.ListPendingChangeRequests)
				r.Get("/change-requests/{change_request_id}", d.PeopleChangeRequests.GetChangeRequest)
				// Approve / reject actions.
				r.With(d.Idempotency.Handler).Post("/change-requests/{change_request_id}:approve", d.PeopleChangeRequests.ApproveChangeRequest)
				r.With(d.Idempotency.Handler).Post("/change-requests/{change_request_id}:reject", d.PeopleChangeRequests.RejectChangeRequest)
			})

			// Agent self-service: file a profile-change request (E2 F2.1 EP-5,
			// createChangeRequest). x-rbac scope:self — the service enforces that an
			// agent files only for their own employee_id (else 404). hr_admin/
			// super_admin admitted so staff can file on behalf. Idempotency-wrapped.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleHRAdmin, auth.RoleSuperAdmin))
				r.With(d.Idempotency.Handler).Post("/employees/{employee_id}/change-requests", d.PeopleChangeRequests.CreateChangeRequest)
			})
			// PEOPLE change-requests slice end (04-04). Phase 5+ appends after this line.

			// ---------------------------------------------------------------
			// PLACEMENT slice (05-02): E3 placement CRUD + lifecycle actions,
			// shift-leader assignment (INV-2/3/4), and the company roster.
			// COORDINATION POINT: future Phase-5 slices append AFTER this block.
			// ---------------------------------------------------------------

			// Placement reads: super_admin, hr_admin, shift_leader (company_or_global).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.Get("/placements", d.Placement.ListPlacements)
				// DEDICATED expiring endpoint — the FE useListExpiringPlacements toggle hits
				// GET /placements/expiring?within_days=N. In chi a static segment wins over a
				// {param} at the same position regardless of order; register it BEFORE
				// "/placements/{id}" for clarity so it is never shadowed.
				r.Get("/placements/expiring", d.Placement.ListExpiringPlacements)
				// Dashboard stat cards (F3.1 / C2SSLA). Static segment — register
				// BEFORE "/placements/{id}" so it is never shadowed by the param route.
				r.Get("/placements/stats", d.Placement.GetPlacementStats)
				r.Get("/placements/{id}", d.Placement.GetPlacement)
				r.Get("/client-companies/{company_id}/roster", d.Placement.GetCompanyRoster)
			})

			// Placement + shift-leader writes: super_admin, hr_admin (global).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.With(d.Idempotency.Handler).Post("/placements", d.Placement.CreatePlacement)
				r.Patch("/placements/{id}", d.Placement.UpdatePlacement)
				r.With(d.Idempotency.Handler).Post("/placements/{id}:renew", d.Placement.RenewPlacement)
				r.With(d.Idempotency.Handler).Post("/placements/{id}:transfer", d.Placement.TransferPlacement)
				r.With(d.Idempotency.Handler).Post("/placements/{id}:end", d.Placement.EndPlacement)
				r.With(d.Idempotency.Handler).Post("/placements/{id}:resign", d.Placement.ResignPlacement)
				r.With(d.Idempotency.Handler).Post("/placements/{id}:terminate", d.Placement.TerminatePlacement)
				r.With(d.Idempotency.Handler).Post("/shift-leader-assignments", d.Placement.CreateShiftLeaderAssignment)
				r.With(d.Idempotency.Handler).Post("/shift-leader-assignments/{id}:replace", d.Placement.ReplaceShiftLeaderAssignment)
				r.With(d.Idempotency.Handler).Post("/shift-leader-assignments/{id}:end", d.Placement.EndShiftLeaderAssignment)
			})
			// PLACEMENT slice end (05-02). Phase 5+ appends after this line.

			// ---------------------------------------------------------------
			// SCHEDULING slice (06-02): E4 shift masters + schedule grid +
			// conflict engine + bulk-apply (F4.1/F4.2/F4.3 / SA-* / SM-*).
			// COORDINATION POINT: future Phase-6 slices append AFTER this block.
			// ---------------------------------------------------------------

			// Shift-master reads + ALL schedule ops: super_admin, hr_admin,
			// shift_leader (leader scope enforced in the service via GuardCompany).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.Get("/shift-masters", d.Scheduling.ListShiftMasters)
				r.Get("/shift-masters/{id}", d.Scheduling.GetShiftMaster)
				r.Get("/schedule", d.Scheduling.ListSchedule)
				r.With(d.Idempotency.Handler).Post("/schedule", d.Scheduling.CreateScheduleEntry)
				r.With(d.Idempotency.Handler).Patch("/schedule/{id}", d.Scheduling.UpdateScheduleEntry)
				r.With(d.Idempotency.Handler).Delete("/schedule/{id}", d.Scheduling.DeleteScheduleEntry)
				// :check is side-effect-free → NO idempotency wrapper.
				r.Post("/schedule:check", d.Scheduling.CheckScheduleConflicts)
				r.With(d.Idempotency.Handler).Post("/schedule:bulk-apply", d.Scheduling.BulkApplySchedule)
			})

			// Agent self-schedule (F4.3 "Jadwal Saya"): adds RoleAgent for this
			// ONE read; per-row scope (agent self-only / leader-company / staff
			// any) is enforced in the service (SV-1).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
				r.Get("/schedule/by-agent/{employee_id}", d.Scheduling.GetScheduleByAgent)
			})

			// Shift-master WRITES: super_admin, hr_admin (global scope).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.With(d.Idempotency.Handler).Post("/shift-masters", d.Scheduling.CreateShiftMaster)
				r.Patch("/shift-masters/{id}", d.Scheduling.UpdateShiftMaster)
				r.With(d.Idempotency.Handler).Post("/shift-masters/{id}:deactivate", d.Scheduling.DeactivateShiftMaster)
				r.With(d.Idempotency.Handler).Post("/shift-masters/{id}:reactivate", d.Scheduling.ReactivateShiftMaster)
			})
			// SCHEDULING slice end (06-02). Phase 6+ appends after this line.

			// ---------------------------------------------------------------
			// ATTENDANCE slice (07-02): E5 verify/reject (+bulk) + corrections
			// (F5.3/F5.4 / ATT-01/ATT-02). All web ops (reads + verify/reject/
			// bulk + corrections approve/reject) per openapi x-rbac:
			// super_admin, hr_admin, shift_leader (agent is mobile/self, not web).
			// Leader scope is enforced in the service via rbac.GuardCompany.
			// COORDINATION POINT: future Phase-7 slices append AFTER this block.
			// ---------------------------------------------------------------
			// Attendance READS: super_admin, hr_admin, shift_leader, AND agent
			// (agent self-scope is forced in the service: employee_id → caller).
			// Corrections reads stay admin/leader-only (web surface).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
				r.Get("/attendance", d.Attendance.ListAttendance)
				r.Get("/attendance/{id}", d.Attendance.GetAttendance)
			})

			// Agent CLOCK-IN/OUT (F5.1, mobile, scope:self). Idempotent per openapi.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleAgent))
				r.With(d.Idempotency.Handler).Post("/attendance:clock-in", d.Clock.ClockIn)
				r.With(d.Idempotency.Handler).Post("/attendance:clock-out", d.Clock.ClockOut)
			})

			// Correction CREATE (F5.4): an agent files their own correction;
			// shift_leader/HR/super may file too (scope enforced in the service).
			// Own group so agents are admitted; POST is a distinct method from the
			// admin/leader-only GET /corrections below, so no chi route conflict.
			// Idempotency-Key required per openapi.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleShiftLeader, auth.RoleHRAdmin, auth.RoleSuperAdmin))
				r.With(d.Idempotency.Handler).Post("/corrections", d.Attendance.CreateCorrection)
			})

			// Attendance/corrections WRITES + corrections reads: super_admin,
			// hr_admin, shift_leader (the web verify/reject/corrections surface).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				// corrections reads
				r.Get("/corrections", d.Attendance.ListCorrections)
				r.Get("/corrections/{id}", d.Attendance.GetCorrection)
				// actions (idempotent per openapi — Idempotency-Key required)
				r.With(d.Idempotency.Handler).Post("/attendance/{id}:verify", d.Attendance.VerifyAttendance)
				r.With(d.Idempotency.Handler).Post("/attendance/{id}:reject", d.Attendance.RejectAttendance)
				r.With(d.Idempotency.Handler).Post("/attendance:bulk-verify", d.Attendance.BulkVerify)
				r.With(d.Idempotency.Handler).Post("/attendance:bulk-reject", d.Attendance.BulkReject)
				r.With(d.Idempotency.Handler).Post("/corrections/{id}:approve", d.Attendance.ApproveCorrection)
				r.With(d.Idempotency.Handler).Post("/corrections/{id}:reject", d.Attendance.RejectCorrection)
			})
			// ATTENDANCE slice end (07-02 + F5.1 clock). Phase 7+ appends after this line.

			// ---------------------------------------------------------------
			// LEAVE slice (08-02): E6 two-level approval + quotas + calendar
			// (F6.1/F6.2/F6.3 / LVE-01..03). Leader scope (own-company) is
			// enforced in the service via rbac.GuardCompany. Action endpoints
			// are idempotent (Idempotency-Key required) per openapi.
			// COORDINATION POINT: future Phase-8 slices append AFTER this block.
			// ---------------------------------------------------------------

			// Reads: super_admin, hr_admin, shift_leader, AGENT. Agent reads are
			// SELF-scoped in the service (List forces employee_id; Get → 404 on
			// another employee; balance-by-employee → 403 on another employee).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
				r.Get("/leave-requests", d.Leave.ListLeaveRequests)
				r.Get("/leave-requests/{id}", d.Leave.GetLeaveRequest)
				r.Get("/leave-balances/by-employee/{employee_id}", d.Leave.GetLeaveBalanceByEmployee)
			})

			// Staff-only reads (calendar / quota / grant ledger / aggregate balance):
			// super_admin, hr_admin, shift_leader (company_or_global). Agent excluded.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.With(d.Idempotency.Handler).Post("/leave-requests/{id}:approve-l1", d.Leave.ApproveLeaveRequestL1)
				r.With(d.Idempotency.Handler).Post("/leave-requests/{id}:reject", d.Leave.RejectLeaveRequest)
				r.Get("/leave-quotas", d.Leave.ListLeaveQuotas) // DEPRECATED 2026-06-08
				r.Get("/leave-calendar", d.Leave.GetLeaveCalendar)
				// F6.1 grant-lot ledger + aggregate balance reads (company_or_global).
				r.Get("/leave-grants", d.Leave.ListLeaveGrants)
				r.Get("/leave-grants/{id}", d.Leave.GetLeaveGrant)
				r.Get("/leave-balances", d.Leave.ListLeaveBalances)
				// HR cancel-approved of an APPROVED leave (reverses the grant lots).
				r.With(d.Idempotency.Handler).Post("/leave-requests/{id}:cancel-approved", d.Leave.CancelApprovedLeaveRequest)
			})

			// Agent file-a-request + own-request actions (F6.2): agent, hr_admin,
			// super_admin. Create / submit / cancel are SELF-guarded in the service
			// (Cancel guards own request; an agent withdraws only their own). All
			// action routes require Idempotency-Key.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleHRAdmin, auth.RoleSuperAdmin))
				r.With(d.Idempotency.Handler).Post("/leave-requests", d.Leave.CreateLeaveRequest)
				r.With(d.Idempotency.Handler).Post("/leave-requests/{id}:submit", d.Leave.SubmitLeaveRequest)
				r.With(d.Idempotency.Handler).Post("/leave-requests/{id}:cancel", d.Leave.CancelLeaveRequest)
			})

			// Final/override approval + grant/quota writes: super_admin, hr_admin (global).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.With(d.Idempotency.Handler).Post("/leave-requests/{id}:approve-final", d.Leave.ApproveLeaveRequestFinal)
				r.With(d.Idempotency.Handler).Post("/leave-requests/{id}:approve-override", d.Leave.ApproveLeaveRequestOverride)
				r.With(d.Idempotency.Handler).Post("/leave-requests/{id}:shorten", d.Leave.ShortenLeaveRequest)
				// F6.1 grant-lot writes (global).
				r.With(d.Idempotency.Handler).Post("/leave-grants", d.Leave.CreateLeaveGrant)
				r.With(d.Idempotency.Handler).Patch("/leave-grants/{id}", d.Leave.PatchLeaveGrant)
				// DEPRECATED 2026-06-08 — migration-only.
				r.With(d.Idempotency.Handler).Post("/leave-quotas/{id}:adjust", d.Leave.AdjustLeaveQuota)
				r.With(d.Idempotency.Handler).Post("/leave-quotas:bulk-grant", d.Leave.BulkGrantLeaveQuotas)
			})
			// LEAVE slice end (08-02). Phase 8+ appends after this line.

			// ---------------------------------------------------------------
			// OVERTIME slice (09-02): E7 two-level OT approval + holiday
			// calendar (F7.1/F7.3/F7.4 / OVT-01/OVT-02). Leader scope
			// (own-company) + SELF_APPROVAL_FORBIDDEN are enforced in the
			// service via rbac.GuardCompany / guardSelf. Action endpoints are
			// idempotent (Idempotency-Key required) per openapi.
			// COORDINATION POINT: future Phase-9 slices append AFTER this block.
			// ---------------------------------------------------------------

			// Reads: super_admin, hr_admin, shift_leader, agent. Agent reads are
			// self-scoped in the service (cross-employee → 404); staff use
			// GuardCompany. Holiday reads stay staff-only.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
				r.Get("/overtime", d.Overtime.ListOvertime)
				r.Get("/overtime/{id}", d.Overtime.GetOvertime)
			})

			// Holiday reads: super_admin, hr_admin, shift_leader.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.Get("/holidays", d.Overtime.ListHolidays)
			})

			// Agent-write group: POST /overtime (create / F7.2), :confirm, :withdraw.
			// Roles agent, shift_leader, hr_admin, super_admin (x-rbac scope:self);
			// the agent self-check + leader company scope are enforced in-service.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleShiftLeader, auth.RoleHRAdmin, auth.RoleSuperAdmin))
				r.With(d.Idempotency.Handler).Post("/overtime", d.Overtime.CreateOvertime)
				r.With(d.Idempotency.Handler).Post("/overtime/{id}:confirm", d.Overtime.Confirm)
				r.With(d.Idempotency.Handler).Post("/overtime/{id}:withdraw", d.Overtime.Withdraw)
			})

			// Approval group: L1 approve + reject + bulk — super_admin, hr_admin,
			// shift_leader (company_or_global). SELF_APPROVAL_FORBIDDEN + scope in-service.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.With(d.Idempotency.Handler).Post("/overtime/{id}:approve-l1", d.Overtime.ApproveL1)
				r.With(d.Idempotency.Handler).Post("/overtime/{id}:reject", d.Overtime.Reject)
				r.With(d.Idempotency.Handler).Post("/overtime:bulk-approve", d.Overtime.BulkApprove)
				r.With(d.Idempotency.Handler).Post("/overtime:bulk-reject", d.Overtime.BulkReject)
			})

			// Final approval + holiday writes: super_admin, hr_admin (global).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.With(d.Idempotency.Handler).Post("/overtime/{id}:approve-final", d.Overtime.ApproveFinal)
				r.With(d.Idempotency.Handler).Post("/holidays", d.Overtime.CreateHoliday)
				r.With(d.Idempotency.Handler).Patch("/holidays/{id}", d.Overtime.UpdateHoliday)
				r.Delete("/holidays/{id}", d.Overtime.DeleteHoliday)
			})
			// OVERTIME slice end (09-02). Phase 9+ appends after this line.

			// ---------------------------------------------------------------
			// PAYROLL slice (10-02): E8 historical, read-only payslip archive
			// (F8.1/F8.2 / PAY-01/PAY-02). The web surface is HR/Super-Admin
			// ONLY (INV-3/4 — agent self-summary is mobile, shift_leader has no
			// payroll access; both → 403). The 5 FE-used ops: list, detail,
			// audit-notes list/create, async export. No 405-immutable / PDF /
			// forward-export handlers (out of scope). chi matches the `:export`
			// action suffix natively (decision [02-01]).
			// COORDINATION POINT: future Phase-10 slices append AFTER this block.
			// ---------------------------------------------------------------
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleHRAdmin, auth.RoleSuperAdmin))
				// Payslip READS (list + detail) are self-or-global (PAY-01): the
				// agent reads ONLY their own (employee_id forced to the caller in
				// the service; another employee's id → 404, no existence leak).
				// hr/super keep the global archive.
				r.Get("/payslips", d.Payroll.ListPayslips)
				r.Get("/payslips/{id}", d.Payroll.GetPayslip)
			})
			// Audit-notes (read + append) and async export stay HR/Super-Admin
			// ONLY — the agent self-summary surface is reads, not these.
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
				r.Get("/payslips/{id}/audit-notes", d.Payroll.ListAuditNotes)
				r.With(d.Idempotency.Handler).Post("/payslips/{id}/audit-notes", d.Payroll.CreateAuditNote)
				r.With(d.Idempotency.Handler).Post("/payslips:export", d.Payroll.ExportPayslips)
			})
			// PAYROLL slice end (10-02). Phase 10+ appends after this line.

			// ---------------------------------------------------------------
			// E10 REPORTING slice — NOTIFICATIONS (11-02). The caller's in-app
			// inbox: list (cursor + read_state/kind), single mark-read, bulk
			// mark-all-read. scope=self is enforced in the service (recipient
			// set = the principal's user id + employee id); all four roles may
			// read their OWN notifications. Action endpoints are idempotent
			// (Idempotency-Key required) per openapi.
			// COORDINATION POINT: 11-02b appends dashboard/report/exports AFTER
			// this block (extending d.Reporting with new methods).
			// ---------------------------------------------------------------
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
				r.Get("/notifications", d.Reporting.ListNotifications)
				r.With(d.Idempotency.Handler).Post("/notifications/{notification_id}:mark-read", d.Reporting.MarkNotificationRead)
				r.With(d.Idempotency.Handler).Post("/notifications:mark-all-read", d.Reporting.MarkAllNotificationsRead)
			})
			// E10 REPORTING notifications slice end (11-02). 11-02b appends after this line.

			// ---------------------------------------------------------------
			// E10 REPORTING slice — DASHBOARD + REPORT (11-02b). The role-aware
			// landing dashboard (all 4 roles, scope=self) + the billable
			// attendance report (HR/super/leader; leader is server-scoped to
			// their own company, else 403 OUT_OF_SCOPE). Both return the
			// {data:<body>} envelope the FE unwraps (query.data.data).
			// ---------------------------------------------------------------
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
				r.Get("/dashboards/me", d.Reporting.GetMyDashboard)
			})
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.Get("/reports/attendance-billable", d.Reporting.GetBillableReport)
			})

			// ---------------------------------------------------------------
			// E10 REPORTING slice — EXPORTS (11-02b). The generic export
			// framework: POST /exports (202 + ExportJob, QUEUED + EnqueueTx the
			// ReportExportWorker in one tx), GET /exports/{id} (status poll,
			// scope=self, DB→wire status mapping), POST /exports/{id}:cancel.
			// Create + cancel are Idempotency-wrapped per openapi. chi matches
			// the `:cancel` action suffix natively.
			// ---------------------------------------------------------------
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
				r.With(d.Idempotency.Handler).Post("/exports", d.Reporting.CreateExport)
				r.Get("/exports/{export_id}", d.Reporting.GetExport)
				r.With(d.Idempotency.Handler).Post("/exports/{export_id}:cancel", d.Reporting.CancelExport)
			})
			// E10 REPORTING exports slice end (11-02b).
		})
	})

	return otelhttp.NewHandler(r, "http.server")
}
