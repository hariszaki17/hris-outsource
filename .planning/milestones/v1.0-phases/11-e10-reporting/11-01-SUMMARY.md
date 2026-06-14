---
phase: 11-e10-reporting
plan: 01
subsystem: backend-data-layer
tags: [e10, reporting, notifications, exports, dashboard, billable, sqlc, migration]
requires:
  - export_jobs table (Phase-10 / 00034)
  - swp_next_id('NTF') + swp_next_id('EXP') (ids.go prefixes already present)
  - attendance / leave_requests / overtime / placements / employment_agreements / schedule_entries / attendance_codes / shift_masters / client_companies / service_lines / employees tables (E2..E8)
provides:
  - notifications table (00035) + SWP-NTF-* allocation
  - generalized export_jobs (00036: CANCELLED status, EXCEL format, report_type/filters/audit_log_entry_id/progress_percent/expires_at)
  - reporting sqlc Querier methods (notifications, generic exports, dashboard aggregations, billable aggregation)
  - internal/domain/reporting/* domain types (openapi-shaped)
affects:
  - 11-02 (services + handlers: dashboard, notifications + dispatch helper + worker un-stub, retro-wire)
  - 11-02b (billable report + export framework services)
  - internal/repository/payroll/mapping.go (mapExportJob made generic)
tech-stack:
  added: []
  patterns:
    - "ALTER (never recreate) to generalize a prior-phase table"
    - "min(text)::text cast so sqlc emits string not interface{}"
    - "generic mapExportJob over field-identical sqlc Row structs (ALTER split the row types)"
key-files:
  created:
    - backend/db/migrations/00035_notifications.sql
    - backend/db/migrations/00036_export_jobs_generalize.sql
    - backend/db/queries/reporting/notifications.sql
    - backend/db/queries/reporting/exports.sql
    - backend/db/queries/reporting/dashboard.sql
    - backend/db/queries/reporting/billable.sql
    - backend/internal/domain/reporting/notification.go
    - backend/internal/domain/reporting/export.go
    - backend/internal/domain/reporting/dashboard.go
    - backend/internal/domain/reporting/billable.go
  modified:
    - backend/internal/repository/payroll/mapping.go
    - backend/internal/repository/sqlc/* (regenerated)
decisions:
  - "Schema-aligned the aggregation queries to the REAL E5..E9 schema (verification_status PENDING/VERIFIED not PENDING_VERIFY; PENDING_L1/HR not bare PENDING; client_company_id not company_id on placements; no attendance_shift_date — use check_in_at::date) — the plan prose used placeholder enum values"
  - "GetExportJob name collided with Phase-10 (one shared sqlcgen package) → renamed the generic getter GetExportJobGeneric"
  - "00036 ALTER added columns to export_jobs, so sqlc stopped collapsing the Phase-10 explicit-RETURNING queries onto the shared ExportJob model and emitted distinct Row structs → mapExportJob made generic over InsertExportJobRow|GetExportJobRow"
metrics:
  duration_min: 7
  tasks: 3
  files: 12
  completed: 2026-06-05
---

# Phase 11 Plan 01: E10 Reporting Data Foundation Summary

Notifications table + generalized export_jobs + the reporting sqlc queries and domain
types that the wave-2 service plans (11-02 / 11-02b) build on — a data-layer-only slice
(2 migrations, 4 sqlc query files in a new `reporting/` dir, 4 domain files, regenerated
sqlc), mirroring the Phase-10 10-01 pattern. `make gen` + `go build` + `go vet` clean; full
backend test suite 13 packages pass / 0 fail (no Phase-10 regression).

## What was built

- **00035_notifications.sql** — `notifications` table (SWP-NTF-* via `swp_next_id('NTF')`
  column DEFAULT). Columns: recipient_id, kind (no CHECK — forward-compat), title, body,
  flattened `deep_link_epic/_entity_id/_path`, `actor_id/_label`, is_critical, read_at,
  created_at. Indexes: `notifications_recipient_created_idx (recipient_id, created_at DESC,
  id DESC)` for the keyset list + `notifications_unread_idx … WHERE read_at IS NULL` for the
  unread filter/count.
- **00036_export_jobs_generalize.sql** — ALTER (never recreate) of the Phase-10 table:
  status CHECK adds CANCELLED, format CHECK adds EXCEL (keeps XLSX), and adds
  `report_type (DEFAULT 'PAYSLIPS')`, `filters jsonb (DEFAULT '{}')`, `audit_log_entry_id`,
  `progress_percent`, `expires_at` — all nullable/defaulted so the Phase-10 PAYSLIP_EXPORT
  insert + worker path is unchanged.
- **reporting sqlc queries** (4 files) + **reporting domain types** (4 files) — see handoff.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] mapExportJob no longer compiled after the export_jobs ALTER**
- **Found during:** Task 3 (`go build` after `make gen`)
- **Issue:** Adding columns to `export_jobs` (00036) made sqlc emit distinct `InsertExportJobRow`/`GetExportJobRow` structs for the Phase-10 explicit-RETURNING queries (previously collapsed onto the shared `ExportJob` model). `mapExportJob(sqlcgen.ExportJob)` then rejected those rows.
- **Fix:** Made `mapExportJob` generic over `InsertExportJobRow | GetExportJobRow` (field-identical) in `internal/repository/payroll/mapping.go`. Phase-10 behaviour unchanged.
- **Commit:** 82a6e11

**2. [Rule 3 - Blocking] GetExportJob query-name collision**
- **Found during:** Task 3 (`make gen`)
- **Issue:** sqlc shares one `sqlcgen` package across all `db/queries/*` dirs → global query-name uniqueness. The plan's `GetExportJob` collided with Phase-10's.
- **Fix:** Renamed the new generic getter `GetExportJobGeneric`.
- **Commit:** 82a6e11

**3. [Rule 1 - Correctness] Schema-aligned the aggregation queries**
- **Found during:** Task 3 (reading the real E5..E9 migrations)
- **Issue:** The plan `<action>` prose used placeholder enum/column names that don't exist: `verification_status='PENDING_VERIFY'`, a bare overtime `'PENDING'`, `attendance_shift_date`, `is_verified`, placements `company_id`.
- **Fix:** Aligned to reality — `verification_status IN ('PENDING','ESCALATED')` for the leader queue and `='VERIFIED'` for billable; `status IN ('PENDING_L1','PENDING_HR')` for leave + OT; `placements.client_company_id` + `service_line_id` + `lifecycle_status` + `end_date`; `check_in_at::date` as the shift date; `attendance_codes.is_billable`. Documented in each query-file header.
- **Commit:** 82a6e11

**4. [Rule 1 - Quality] min(text) → interface{}**
- **Issue:** `min(p.client_company_id)` etc. emitted `interface{}` (sqlc can't infer min-over-text).
- **Fix:** Cast to `::text` → sqlc emits `string`. Cleaner repo mapping in 11-02.
- **Commit:** 352ca5e

No other deviations. No auth gates. ids.go NOT touched. No service/handler/route/seed edits.

## Self-Check: PASSED

- All 10 created files exist on disk (2 migrations, 4 sqlc query files, 4 domain files).
- All 4 task commits exist: 0e9fe23 (notifications migration), 40eef74 (export_jobs generalize),
  82a6e11 (queries + domain + generic mapExportJob), 352ca5e (billable ::text refinement).
- `make gen` exits 0; `go build ./...` + `go vet ./...` exit 0; `gofmt -l` clean on new files.
- Full backend test suite: 13 packages pass / 0 fail (no Phase-10 regression).

---

## Reference for 11-02 / 11-02b (handoff)

Everything wave-2 needs to build services with zero hand-rolled SQL. All methods are on the
generated `sqlcgen.Querier` (mockable). Repos live under `internal/repository/reporting/`
(11-02 creates), mapping rows → `internal/domain/reporting` types.

### Tables / columns

**notifications** (00035): `id, recipient_id, kind, title, body, deep_link_epic,
deep_link_entity_id, deep_link_path, actor_id, actor_label, is_critical, read_at, created_at`.
`recipient_id` is SWP-USR-* or SWP-EMP-* (the dispatch helper decides which — be consistent
with what `GET /notifications` scopes on; the principal's user id is the natural key). id via
DEFAULT — INSERT omits it.

**export_jobs** (00034 + 00036): base Phase-10 columns + `report_type, filters jsonb,
audit_log_entry_id, progress_percent, expires_at`. DB status enum =
`QUEUED/RUNNING/DONE/FAILED/CANCELLED`; DB format = `XLSX` (payslip) or `EXCEL` (generic).

### sqlc Querier methods (notifications)

| Method | Signature shape | Notes |
|---|---|---|
| `ListNotifications(ctx, ListNotificationsParams)` → `[]Notification` | Params: `RecipientID string`, `ReadState *string` ('ALL'/'UNREAD'/'READ'), `Kind *string`, `CursorCreatedAt *time.Time`, `CursorID *string`, `RowLimit int64` | keyset `(created_at,id) < cursor` DESC; fetch `limit+1` for the cursor envelope like ListAuditLog |
| `GetNotification(ctx, GetNotificationParams{ID, RecipientID})` → `Notification` | scope=self | `pgx.ErrNoRows → domain.ErrNotFound` |
| `InsertNotification(ctx, InsertNotificationParams)` → `Notification` | Params: RecipientID, Kind, Title, Body, DeepLinkEpic *string, DeepLinkEntityID *string, DeepLinkPath *string (COALESCE→''), ActorID *string, ActorLabel *string (COALESCE→'system'), IsCritical bool | **the dispatch-helper / NotificationWorker write path** (un-stub `internal/platform/jobs/notify.go` to call this) |
| `MarkNotificationRead(ctx, MarkNotificationReadParams{ID, RecipientID})` → `Notification` | `read_at = COALESCE(read_at, now())` (idempotent) | returns the row |
| `MarkAllNotificationsRead(ctx, MarkAllNotificationsReadParams{RecipientID, Before *time.Time})` → `int64` (`:execrows`) | optional `Before` cutoff | returns affected count |
| `CountUnreadNotifications(ctx, recipientID string)` → `int64` | unread badge + `AgentDashboard.recent_notifications_unread` | |

`Notification` (model) fields: `ID, RecipientID, Kind, Title, Body, DeepLinkEpic *string,
DeepLinkEntityID *string, DeepLinkPath string, ActorID *string, ActorLabel string,
IsCritical bool, ReadAt *time.Time, CreatedAt time.Time`. Re-nest into the domain
`Notification{DeepLink{Epic,EntityID,Path}, Actor{ID,Label}}` at the repo boundary.

### sqlc Querier methods (generic exports)

| Method | Returns | Notes |
|---|---|---|
| `InsertExportJobGeneric(ctx, InsertExportJobGenericParams)` → `InsertExportJobGenericRow` | Params: `ReportType string, Format string, Confidential bool, Filters []byte, RequestedByID string, RequestedByName *string, AuditLogEntryID *string, ExpiresAt *time.Time` | status defaults QUEUED; `kind` is set = report_type. **`Filters` is `[]byte`** — `json.Marshal(map[string]any)` in, `json.Unmarshal` out |
| `GetExportJobGeneric(ctx, id string)` → `GetExportJobGenericRow` | requester scope in service | |
| `UpdateExportJobStatusGeneric(ctx, UpdateExportJobStatusGenericParams)` → `UpdateExportJobStatusGenericRow` | Params: `Status string, ProgressPercent *int32, RowCount *int32, ArtifactRef *string, ErrorMessage *string, ExpiresAt *time.Time, ID string` | stamps started_at on RUNNING (once), completed_at on DONE/FAILED/CANCELLED — **the generic export worker's lifecycle writer** |
| `CancelExportJob(ctx, id string)` → `CancelExportJobRow` | `QUEUED/RUNNING → CANCELLED` only; **0 rows if already terminal → re-read via GetExportJobGeneric** (no-op-safe) | |

Generic export Row fields: `ID, ReportType, Status, Format string, Confidential bool,
Filters []byte, ProgressPercent *int32, RowCount *int32, ArtifactRef *string,
ErrorMessage *string, AuditLogEntryID *string, ExpiresAt *time.Time, RequestedByID string,
RequestedByName *string, RequestedAt time.Time, StartedAt *time.Time, CompletedAt *time.Time`.

**DTO status mapping (11-02b owns this):** DB `RUNNING → wire PROCESSING`,
`DB DONE → wire COMPLETED`; QUEUED/FAILED/CANCELLED pass through. `filename` on the wire =
`artifact_ref`. `file_url` = `/api/v1/exports/{id}/download` once COMPLETED. `error{code,message}`
populated only when status=FAILED. The Phase-10 `InsertExportJob`/`GetExportJob`/
`UpdateExportJobStatus` (payroll dir) are UNCHANGED — keep using them for PAYSLIPS.

### sqlc Querier methods (dashboard aggregations)

All return `int64` unless noted. Scope param `CompanyID *string` (nil = global; set = leader's
company). `Today` params are `pgtype.Date` (convert `time.Time` ↔ `pgtype.Date` like Phase-5/9).

| Method | Param | Feeds |
|---|---|---|
| `CountPendingAttendanceVerify(ctx, companyID *string)` | | leader attendance_verify + HrDashboard anomalies seed |
| `CountPendingLeaveApprove(ctx, companyID *string)` | PENDING_L1+HR | leader leave_approve |
| `CountPendingLeaveApproveHR(ctx, companyID *string)` | PENDING_HR only | `HrDashboard.kpis.leave_pending` |
| `CountPendingOtApprove(ctx, companyID *string)` | PENDING_L1+HR | leader/HR ot_approve |
| `CountExpiringPlacements30d(ctx, CountExpiringPlacements30dParams{Today pgtype.Date, CompanyID *string})` | | expiring_placements_30d |
| `CountExpiringAgreements30d(ctx, today pgtype.Date)` | | expiring_agreements_30d |
| `CountActivePlacements(ctx)` | | kpis.active_placements |
| `CountActiveCompanies(ctx)` | DISTINCT client_company_id | kpis.active_companies |
| `LeaderTodayStatus(ctx, LeaderTodayStatusParams{Today pgtype.Date, CompanyID *string})` → `LeaderTodayStatusRow{ShiftsTotal, ClockedIn, LateCount, AbsentCount, PendingVerifications int64}` | | `LeaderDashboard.today` |
| `AgentRecentAttendance(ctx, AgentRecentAttendanceParams{EmployeeID string, Today pgtype.Date})` → `AgentRecentAttendanceRow{Last7dPresent, Last7dLate, Last7dAbsent int64}` | | `AgentDashboard.recent_attendance` |
| `CountPendingRequestsForEmployee(ctx, employeeID string)` → `CountPendingRequestsForEmployeeRow{LeavePending, OtPending int64}` | | `AgentDashboard.pending_requests` |

**Not provided here (11-02 computes from existing E5/E7/E6 queries or adds as needed):**
`attendance_rate_pct`, `billable_hours_mtd` (reuse BillableSummary below), `ot_hours_mtd`,
`billable_trend.points`, leave_balance, today_shift, schedule_alerts. The CONTEXT allows live
aggregation — wire these in the service using the above + the epic repos.

### sqlc Querier methods (billable report)

Shared params on every billable query: `PeriodStart pgtype.Date, PeriodEnd pgtype.Date,
CompanyID *string, ServiceLineID *string`.

| Method | Returns | group_by |
|---|---|---|
| `BillableAggregateByEmployee` | `[]BillableAggregateByEmployeeRow` | employee |
| `BillableAggregateByDay` | `[]BillableAggregateByDayRow` | day |
| `BillableAggregateByShiftMaster` | `[]BillableAggregateByShiftMasterRow` | shift_master |
| `BillableSummary` | `BillableSummaryRow{TotalBillableMinutes, TotalWorkedMinutes, TotalVerifiedRecords int64}` | totals |
| `BillablePendingSummary` | `BillablePendingSummaryRow{PendingRecords, PendingMinutesEstimate int64}` | not-yet-verified |

Aggregate Row fields: `GroupKey string, GroupLabel string, CompanyID string, CompanyName
string, ServiceLineID string, ServiceLineName string, WorkedMinutes int64, BillableMinutes
int64, VerifiedRecordCount int64`.

**Hours conversion (service):** all `*Minutes` are int64 → divide by 60.0 for the wire
`*_hours` floats. `payable_hours = worked_hours` (v1 has no separate payable column —
faithful stand-in derived from real `worked_minutes`). `verification_rate_pct` = the service
computes `verified / (verified + pending)` from BillableSummary.TotalVerifiedRecords +
BillablePendingSummary.PendingRecords (null when both 0). `unverified_record_count` per row is
NOT in the per-group rows (the per-group pending split was out of scope — emit 0 per row or
add a sibling pending-by-group query if the FE needs it; the report-level pending_summary IS
provided).

### sqlc type quirks (recurring)

- `jsonb` (export filters) → `[]byte` — marshal/unmarshal `map[string]any`.
- `pgtype.Date` for all date params (`Today`, `PeriodStart/End`) — convert from `time.Time`.
- `count(*)::bigint` and `sum(...)::bigint` → `int64`.
- nullable text params → `*string` (sqlc.narg); the `emit_pointers_for_null_types` config is on.
- `min(text)::text` was cast deliberately so company/service-line names come back as `string`
  (not `interface{}`). They are `""` only if the LEFT JOIN missed (won't happen via the
  placements FK chain).
- timestamptz → `time.Time`, nullable timestamptz → `*time.Time` (sqlc.yaml override).

### 11-02 loop-closer reminders (from CONTEXT)

- Un-stub `internal/platform/jobs/notify.go` `NotificationWorker.Work` → call
  `InsertNotification`. Add `notify.Dispatch(ctx, tx, recipient, kind, payload)` that
  `EnqueueTx`'s a `NotificationArgs` in the caller's tx (transactional outbox).
- Retro-wire the prior-phase `TODO(Phase-11)` points: leave approve/reject (08-*), OT
  approve/reject (09-*), attendance verify/reject (07-*), change-request resolve (04-*),
  placement lifecycle (05-*). Additive enqueue only — don't change existing behavior/tests.
- Seed a handful of `notifications` (mixed read/unread, across kinds) in the 11-02 seed; add
  `notifications` to reset-db TRUNCATE (keep export_jobs handling consistent with Phase-10).
