---
phase: 11-e10-reporting
plan: 02b
type: execute
wave: 3
depends_on: ["11-01", "11-02"]
files_modified:
  - backend/internal/repository/reporting/dashboard_repo.go
  - backend/internal/repository/reporting/billable_repo.go
  - backend/internal/repository/reporting/export_repo.go
  - backend/internal/service/reporting/dashboard_service.go
  - backend/internal/service/reporting/billable_service.go
  - backend/internal/service/reporting/export_service.go
  - backend/internal/service/reporting/ports.go
  - backend/internal/handler/reporting/dashboard_handler.go
  - backend/internal/handler/reporting/billable_handler.go
  - backend/internal/handler/reporting/export_handler.go
  - backend/internal/handler/reporting/dto.go
  - backend/internal/platform/jobs/report_export.go
  - backend/internal/platform/jobs/jobs.go
  - backend/internal/server/server.go
  - backend/cmd/api/main.go
autonomous: true
requirements: [RPT-01, RPT-03]
user_setup: []

must_haves:
  truths:
    - "GET /dashboards/me returns a role-shaped payload: HR/super_admin → HrDashboard (kpis + expiring + pending_approvals_panel rows with deep links); shift_leader → LeaderDashboard (today clock-in/late/absent + pending_counts + schedule_alerts, scoped to own company); agent → AgentDashboard"
    - "GET /reports/attendance-billable aggregates VERIFIED attendance on billable codes over a date range/company/service-line filter, scope-aware (leader → own company only, else 403 OUT_OF_SCOPE), returns summary + pending_summary + rows; REPORT_PERIOD_TOO_WIDE (422) over 1 year"
    - "POST /exports inserts an export_jobs QUEUED row (report_type + filters) + EnqueueTx a ReportExportWorker in the SAME tx → 202 + {id,status}; the worker builds the artifact + marks the job DONE (wire) → COMPLETED; the Phase-10 payslip export still works"
    - "GET /exports/{id} returns the job status mapped to the WIRE enum (RUNNING→PROCESSING, DONE→COMPLETED), scope=self; POST /exports/{id}:cancel cancels a QUEUED/RUNNING job → CANCELLED (no-op 200 if terminal)"
    - "POST /exports with format=PDF/CSV → 422 EXPORT_FORMAT_UNSUPPORTED; oversized scope → 422 EXPORT_TOO_LARGE; per-user throttle → 429 RATE_LIMITED_EXPORTS"
  artifacts:
    - path: "backend/internal/service/reporting/dashboard_service.go"
      provides: "role-aware aggregation: HR/Leader/Agent payloads scoped (leader → own company)"
      contains: "LeaderDashboard"
    - path: "backend/internal/service/reporting/billable_service.go"
      provides: "verified-attendance billable aggregation + pending_summary + scope guard + period cap"
      contains: "REPORT_PERIOD_TOO_WIDE"
    - path: "backend/internal/service/reporting/export_service.go"
      provides: "generic export: format guard + size guard + throttle + insert QUEUED + EnqueueTx in one tx + get + cancel"
      contains: "EXPORT_FORMAT_UNSUPPORTED"
    - path: "backend/internal/platform/jobs/report_export.go"
      provides: "ReportExportWorker that builds the artifact + marks export_jobs DONE (generic report kinds)"
      contains: "ReportExportWorker"
    - path: "backend/internal/handler/reporting/export_handler.go"
      provides: "POST /exports (202) + GET /exports/{id} + :cancel handlers with DB→wire status mapping"
      contains: "PROCESSING"
    - path: "backend/internal/server/server.go"
      provides: "dashboard + report + exports routes appended after the NOTIFICATIONS block"
      contains: "/exports"
  key_links:
    - from: "backend/internal/service/reporting/export_service.go"
      to: "backend/internal/platform/jobs (River)"
      via: "EnqueueTx(ReportExportArgs) in the same tx as InsertExportJobGeneric(QUEUED)"
      pattern: "EnqueueTx"
    - from: "backend/internal/platform/jobs/report_export.go"
      to: "export_jobs table"
      via: "UpdateExportJobStatusGeneric → RUNNING then DONE"
      pattern: "DONE"
    - from: "backend/internal/handler/reporting/export_handler.go"
      to: "ExportJob wire DTO"
      via: "DB status DONE→COMPLETED, RUNNING→PROCESSING mapping"
      pattern: "COMPLETED"
    - from: "backend/internal/service/reporting/billable_service.go"
      to: "verified attendance + billable codes + placements"
      via: "BillableAggregate sqlc query (verification_status='VERIFIED', is_billable)"
      pattern: "Billable"
---

<objective>
Implement the read-aggregation + export half of E10: the role-aware dashboard (/dashboards/me), the
billable attendance report (/reports/attendance-billable), and the GENERALIZED export framework
(POST /exports, GET /exports/{id}, :cancel) async via a ReportExportWorker over the Phase-10
export_jobs table. The export status enum is mapped at the DTO boundary (DB RUNNING/DONE → wire
PROCESSING/COMPLETED) so the built FE (use-export-flow.ts + e10-shared.tsx) drives it unchanged.

This plan APPENDS its routes to server.go AFTER the NOTIFICATIONS block that 11-02 added, and EXTENDS
the same reporting Handler + main.go wiring. Mirror Phase-10's payroll export precedent (insert QUEUED
+ EnqueueTx in one tx → 202; worker flips to DONE) — keep the payslip export path working. Match
docs/api/E10-reporting/openapi.yaml byte-for-byte for the 5 ops + error codes.

Purpose: success-criteria 1 (dashboard + billable) and 3 (export framework async).
Output: dashboard/billable/export repo+service+handler, ReportExportWorker, routes, main.go wiring.
</objective>

<execution_context>
@/Users/diaz/.claude/get-shit-done/workflows/execute-plan.md
@/Users/diaz/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/11-e10-reporting/11-CONTEXT.md
@.planning/phases/11-e10-reporting/11-01-PLAN.md
@.planning/phases/11-e10-reporting/11-02-PLAN.md
@.planning/reference/backend-build-conventions.md

<interfaces>
<!-- Phase-10 export precedent to GENERALIZE (keep working): -->
From backend/internal/service/payroll/export_service.go:
  Export(ctx, req) → dom.ExportJob   // insert export_jobs QUEUED + jobs.EnqueueTx(PayslipExportArgs) in one tx → 202; EXPORT_TOO_LARGE guard
From backend/internal/platform/jobs/jobs.go:
  NewWorkerClient(pool) registers NewPayslipExportWorker(pool) — ADD NewReportExportWorker(pool) alongside it
From backend/internal/platform/jobs/payslip_export.go:
  PayslipExportWorker.Work → UpdateExportJobStatus RUNNING→DONE  (mirror for ReportExportWorker over the generic columns)

<!-- WIRE status enum the FE uses (e10-shared.tsx + use-export-flow.ts):
     QUEUED / PROCESSING / COMPLETED / FAILED / CANCELLED.  DB keeps RUNNING/DONE.
     Service/handler MUST map: DB RUNNING→wire PROCESSING, DB DONE→wire COMPLETED. -->

<!-- FE anchors (do NOT invent shapes — match these): -->
  dashboard-screen.tsx       → body = query.data.data (bare Dashboard, role-discriminated)
  billable-report-screen.tsx → report = query.data.data (bare BillableReport); reads summary, pending_summary{pending_records,pending_hours_estimate}, rows[]{group_key,group_label,company_name,service_line_name,worked_hours,billable_hours,payable_hours,verified_record_count,unverified_record_count}
  use-export-flow.ts         → createExport → res.data.id (202 body = bare ExportJob); polls useGetExport(id) → res.data.status (COMPLETED/FAILED/CANCELLED terminal); reads file_url, filename, size_bytes, progress_percent, error.code/message
  ExportRequest (FE sends)   → { report_type:"ATTENDANCE_BILLABLE", format:"EXCEL", filters:{period_start,period_end,company_id?,service_line_id?,group_by?} }

<!-- apperr: Rule(code,fields)=422; OutOfScope()=403; struct-literal apperr.Error{HTTPStatus:429} for RATE_LIMITED_EXPORTS (mirror Phase-3 GEOFENCE_RADIUS_INVALID technique for non-default status). -->
<!-- rbac.GuardCompany(ctx, companyID) → OUT_OF_SCOPE 403 for leader scope (used by attendance/leave). -->
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Dashboard + Billable report (repo + service + handler + routes)</name>
  <read_first>
    - backend/db/queries/reporting/dashboard.sql + billable.sql (the 11-01 aggregation queries)
    - backend/internal/domain/reporting/dashboard.go + billable.go (the structs to fill)
    - backend/internal/service/foundations/* (Phase-2 read/list aggregation analog — service struct + actor/role helpers)
    - backend/internal/platform/rbac (actorRole(ctx), GuardCompany, the shift_leader company resolution — how the leader's own company_id is obtained from the session/scope)
    - frontend/apps/web/src/features/e10-reporting/dashboard-screen.tsx (HrDashboardView/LeaderDashboardView fields consumed) + billable-report-screen.tsx (summary/pending/rows fields + ApprovalInboxPanel rows kind/label/count/deep_link)
    - docs/api/E10-reporting/openapi.yaml HrDashboard/LeaderDashboard/AgentDashboard required fields + BillableReport/BillableReportRow + the pending_approvals_panel ApprovalInboxRow (kind enum + deep_link path examples) + REPORT_PERIOD_TOO_WIDE/OUT_OF_SCOPE
  </read_first>
  <action>
    Dashboard service (dashboard_service.go): GetMyDashboard(ctx) switches on actorRole(ctx):
      - hr_admin / super_admin → HrDashboard: role + role_label ("HR Admin"/"Super Admin"), generated_at=now, period_label (current month Bahasa, Asia/Jakarta), kpis{active_placements, active_companies, attendance_rate_pct, billable_hours_mtd, ot_hours_mtd, leave_pending} from the dashboard.sql counts (live read; honest values from seeded data; attendance_rate_pct/billable MTD may be a documented derived value if the column isn't present — derive from real rows, never a constant), expiring_placements_30d, expiring_agreements_30d, attendance_anomalies_today, billable_trend{granularity:"day", points:[]} (empty array acceptable if no trend rollup — required field present), pending_approvals_panel = []ApprovalInboxRow built from the pending counts with deep_link paths EXACTLY as the openapi examples (ATTENDANCE_VERIFY→"/attendance?status=PENDING_VERIFY", LEAVE_APPROVE→"/leave-requests?status=PENDING_L1,PENDING_L2", OT_APPROVE→"/overtime?status=PENDING", PLACEMENT_EXPIRING→"/placements?expiring_within=30d"); omit rows whose count is 0 OR include them — match the FE (panel renders all rows; include the non-zero ones at minimum). super_admin = same shape, role_label "Super Admin" (D1).
      - shift_leader → LeaderDashboard scoped to the leader's own company: company{id,name}, today{date, shifts_total, clocked_in, late_count, absent_count, pending_verifications} from LeaderTodayStatus, pending_counts{attendance_verify, leave_approve, ot_approve}, schedule_alerts[] (may be empty), pending_approvals_panel with company-scoped deep_link paths (?company_id=<id>&status=...).
      - agent → AgentDashboard (today_shift nullable, recent_attendance, leave_balance, ot_this_month_hours, pending_requests{leave,ot}, recent_notifications_unread=CountUnread). Honest values from seeded data; today_shift may be null.
    Handler (dashboard_handler.go): GET /dashboards/me → return the role-shaped object in a {data: <Dashboard>} envelope (FE unwraps query.data.data). Set Cache-Control: private, max-age=30.

    Billable service (billable_service.go): GetBillableReport(ctx, params{company_id?, service_line_id?, period_start, period_end, group_by}):
      - validate period_end >= period_start (else 400 INVALID_REQUEST) and (period_end - period_start) <= 1 year (else 422 REPORT_PERIOD_TOO_WIDE with fields.period_end).
      - scope: if leader, force company_id = leader's own company; if the request's company_id != leader company → 403 OUT_OF_SCOPE (apperr.OutOfScope). HR/super → company_id optional.
      - run BillableAggregate + BillablePendingSummary; build BillableReport{generated_at, filters{company_id,company_name,service_line_id,service_line_name,period_start,period_end,group_by}, summary{total_billable_hours,total_worked_hours,total_payable_hours,total_verified_records,verification_rate_pct(nullable when 0 records)}, pending_summary{pending_records,pending_hours_estimate,note}, rows[]}.
    Handler (billable_handler.go): GET /reports/attendance-billable → {data: BillableReport} envelope (FE unwraps query.data.data). Parse query params; group_by default employee.

    Routes (server.go) — append AFTER the NOTIFICATIONS block (11-02):
      // E10 REPORTING slice — DASHBOARD + REPORT (11-02b).
      r.Group(func(r chi.Router) { r.Use(rbac.RequireRole(all 4 roles)); r.Get("/dashboards/me", d.Reporting.GetMyDashboard) })
      r.Group(func(r chi.Router) { r.Use(rbac.RequireRole(SuperAdmin, HRAdmin, ShiftLeader)); r.Get("/reports/attendance-billable", d.Reporting.GetBillableReport) })
    Extend the reporting Handler + main.go wiring (same Handler struct, new methods).

    `cd backend && go build ./...`.
  </action>
  <verify>
    <automated>cd backend && go build ./... && grep -q 'r.Get("/dashboards/me"' internal/server/server.go && grep -q "attendance-billable" internal/server/server.go && grep -q "REPORT_PERIOD_TOO_WIDE" internal/service/reporting/billable_service.go && grep -q "LeaderDashboard" internal/service/reporting/dashboard_service.go && echo OK</automated>
  </verify>
  <acceptance_criteria>
    - GET /dashboards/me returns HrDashboard for hr_admin/super_admin (role_label differs), LeaderDashboard scoped to the leader's company, AgentDashboard for agent — all openapi-required fields present.
    - pending_approvals_panel rows carry the exact deep_link paths from the openapi examples.
    - GET /reports/attendance-billable aggregates verified attendance on billable codes; returns summary + pending_summary + rows; leader requesting a non-owned company → 403 OUT_OF_SCOPE; >1yr range → 422 REPORT_PERIOD_TOO_WIDE.
    - Both endpoints return the {data: <body>} envelope the FE unwraps (query.data.data).
    - Routes appended after the NOTIFICATIONS block; `go build ./...` exits 0.
  </acceptance_criteria>
</task>

<task type="auto">
  <name>Task 2: Generic export framework (service + handler + routes) + status DTO mapping</name>
  <read_first>
    - backend/internal/service/payroll/export_service.go (the EXACT precedent: EXPORT_TOO_LARGE guard + InsertExportJob + EnqueueTx in one tx + 202 stub + audit.Record in-tx)
    - backend/db/queries/reporting/exports.sql (InsertExportJobGeneric/GetExportJob/UpdateExportJobStatusGeneric/CancelExportJob from 11-01)
    - backend/internal/domain/reporting/export.go (ExportJob + status consts)
    - backend/internal/platform/apperr (Rule for 422; the struct-literal apperr.Error{HTTPStatus:429} technique for RATE_LIMITED_EXPORTS — mirror Phase-3 GEOFENCE_RADIUS_INVALID / Phase-4 FILE_TOO_LARGE)
    - backend/internal/platform/audit (audit.Record in-tx, returns the AL id for audit_log_entry_id)
    - frontend/apps/web/src/features/e10-reporting/use-export-flow.ts (202 body = bare ExportJob → res.data.id; poll res.data.status; reads file_url/filename/size_bytes/progress_percent/error) + e10-shared.tsx (ExportStatus enum QUEUED/PROCESSING/COMPLETED/FAILED/CANCELLED)
    - docs/api/E10-reporting/openapi.yaml /exports POST (ExportRequest, 202 ExportJob, 422 EXPORT_FORMAT_UNSUPPORTED/EXPORT_TOO_LARGE, 429 RATE_LIMITED_EXPORTS), /exports/{id} GET, :cancel
  </read_first>
  <action>
    Export service (export_service.go) — generalize the Phase-10 path:
      - CreateExport(ctx, req{ReportType, Format, Confidential, Filters map[string]any}):
          * format guard: if Format != EXCEL → 422 EXPORT_FORMAT_UNSUPPORTED with fields.format ("PDF akan tersedia di v1.1.").
          * size guard: estimate row count from the report's filters (for ATTENDANCE_BILLABLE reuse a count over the billable query; cap e.g. 250000) → 422 EXPORT_TOO_LARGE with fields.period_end.
          * throttle: per-user recent-export count (simple: count this requester's export_jobs in the last N seconds; if over the cap → apperr.Error{Code:"RATE_LIMITED_EXPORTS", HTTPStatus:429}). Keep the threshold lenient so the happy-path E2E never trips it; document the value.
          * confidential: force true when ReportType==PAYSLIPS (EX-5); else use req.Confidential (default false).
          * scope: leader inherits own company into filters.company_id.
          * in ONE tx: audit.Record(CREATE, entity "export", ...) capturing the AL id → InsertExportJobGeneric(QUEUED, report_type, format=EXCEL, filters jsonb, requested_by, audit_log_entry_id) → jobs.EnqueueTx(ReportExportArgs{JobID, ReportType, Filters}). Return the job stub (status QUEUED) for the 202.
      - GetExport(ctx, id): GetExportJob; scope=self (requester_id == caller) else 404. Return domain job.
      - CancelExport(ctx, id): CancelExportJob (sets CANCELLED if QUEUED/RUNNING); re-read; no-op 200 if already terminal. scope=self.

    ReportExportWorker (backend/internal/platform/jobs/report_export.go): NewReportExportWorker(pool) mirroring PayslipExportWorker; Work() → UpdateExportJobStatusGeneric RUNNING (started_at) → build the artifact (CSV/row-count faithful stand-in like the payslip worker; set artifact_ref/filename, size_bytes, row_count, expires_at=now+7d) → DONE (completed_at). On panic/error → FAILED + error_message. Register NewReportExportWorker(pool) in jobs.go NewWorkerClient alongside the payslip + notification workers.

    Handler (export_handler.go + dto.go) — DTO status mapping DB→WIRE:
      - mapStatus: QUEUED→QUEUED, RUNNING→PROCESSING, DONE→COMPLETED, FAILED→FAILED, CANCELLED→CANCELLED. mapFormat: XLSX→EXCEL, EXCEL→EXCEL.
      - POST /exports → 202 with bare ExportJob (FE reads res.data.id; Orval wraps in {data} — return the job; httpx writes {data:<job>} or bare per the established pattern; match what the FE unwraps: res.data.id means the body IS the job under the Orval {data} wrapper → return the job object, handler wraps in dataResponse).
      - GET /exports/{id} → ExportJob (mapped status). file_url = "/api/v1/exports/"+id+"/download" once COMPLETED (the download endpoint itself is OUT of FE scope — not routed).
      - POST /exports/{id}:cancel → ExportJob (CANCELLED).
      - Idempotency-Key required on POST /exports + :cancel.

    Routes (server.go) — append AFTER the DASHBOARD+REPORT block:
      // E10 REPORTING slice — EXPORTS (11-02b).
      r.Group(func(r chi.Router) {
        r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
        r.With(d.Idempotency.Handler).Post("/exports", d.Reporting.CreateExport)
        r.Get("/exports/{export_id}", d.Reporting.GetExport)
        r.With(d.Idempotency.Handler).Post("/exports/{export_id}:cancel", d.Reporting.CancelExport)
      })
    Extend the reporting Handler + main.go wiring (construct the generic ExportService with the jobs.Client + audit + the billable count for the size guard).

    `cd backend && go build ./...` and confirm the Phase-10 payslip export still compiles/works (its repo + worker untouched).
  </action>
  <verify>
    <automated>cd backend && go build ./... && grep -q 'r.Post("/exports"' internal/server/server.go && grep -q ":cancel" internal/server/server.go && grep -q "ReportExportWorker" internal/platform/jobs/report_export.go && grep -q "NewReportExportWorker(pool)" internal/platform/jobs/jobs.go && grep -q "EXPORT_FORMAT_UNSUPPORTED" internal/service/reporting/export_service.go && grep -q "PROCESSING" internal/handler/reporting/dto.go && echo OK</automated>
  </verify>
  <acceptance_criteria>
    - POST /exports → 202 + bare ExportJob (status QUEUED); inserts export_jobs (report_type + filters) + EnqueueTx ReportExportArgs in ONE tx.
    - ReportExportWorker flips export_jobs RUNNING→DONE (started_at/completed_at/artifact_ref/row_count/expires_at set); registered in NewWorkerClient.
    - GET /exports/{id} returns the WIRE status (DONE→COMPLETED, RUNNING→PROCESSING), scope=self (non-owner → 404).
    - :cancel sets CANCELLED for QUEUED/RUNNING (no-op 200 if terminal).
    - format=PDF/CSV → 422 EXPORT_FORMAT_UNSUPPORTED; oversized → 422 EXPORT_TOO_LARGE; throttle → 429 RATE_LIMITED_EXPORTS.
    - The Phase-10 payslip export path still builds and is registered (both export workers coexist).
    - Exports routes appended after the dashboard/report block; `go build ./...` exits 0.
  </acceptance_criteria>
</task>

</tasks>

<verification>
- `cd backend && go build ./...` exits 0; existing `go test ./internal/...` stays green.
- /dashboards/me, /reports/attendance-billable, /exports (POST/GET/:cancel) all mounted after the 11-02 NOTIFICATIONS block; RBAC per openapi x-rbac.
- Export status mapped DB↔wire so the built FE drives it unchanged; both export workers (payslip + report) registered.
- Aggregations read live from real seeded E3..E8 rows (no constants).
</verification>

<success_criteria>
- Role-aware dashboard + billable report render against the real BE; export framework create→worker-completes→DONE + cancel + format/size/throttle errors work; payslip export unaffected.
</success_criteria>

<output>
After completion, create `.planning/phases/11-e10-reporting/11-02b-SUMMARY.md`.
</output>
