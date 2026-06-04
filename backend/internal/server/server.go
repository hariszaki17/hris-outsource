// Package server assembles the chi router: the global middleware chain, health
// and metrics endpoints, and the /api/v1 routes (public vs authenticated). New
// epics mount their (oapi-codegen) routers under the authenticated group.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	foundationshttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/foundations"
	identityhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/identity"
	orghttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/org"
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
	Authn           *auth.Authenticator
	Idempotency     *idempotency.Middleware
	Obs             *obs.Providers
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

			// E1 Foundations: user management, audit-log, platform settings.
			// All endpoints require super_admin or hr_admin (CONVENTIONS §17 x-rbac).
			r.Group(func(r chi.Router) {
				r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))

				// Users management
				r.Get("/users", d.Foundations.ListUsers)
				r.With(d.Idempotency.Handler).Post("/users", d.Foundations.CreateUser)
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
		})
	})

	return otelhttp.NewHandler(r, "http.server")
}
