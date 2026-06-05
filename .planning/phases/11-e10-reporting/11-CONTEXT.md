# Phase 11: E10 Reporting & Notifications - Context

**Gathered:** 2026-06-05 (autonomous — recommended decisions auto-accepted per user's overnight directive)
**Status:** Ready for planning — FINAL milestone phase

<domain>
## Phase Boundary

Implement the FE-used E10 endpoints against the real BE and wire the screens off MSW, proven
with exhaustive full-stack Playwright E2E. This is the **final phase** and the milestone
loop-closer. Delivers: the role-aware **dashboard** (`/dashboards/me` — pending-action counts +
today's team status aggregated from E3..E8), the **billable attendance report**
(`/reports/attendance-billable` — aggregation over verified E5 attendance), the **notifications**
surface (list / mark-read / mark-all-read) **including realizing the notification-dispatch stubs
that every earlier phase left as `TODO(Phase-11)`** so auto-dispatched notifications actually
appear, and the **generalized export framework** (`POST /exports`, `GET /exports/{id}`,
`:cancel`) async via the River worker (over the Phase-10 `export_jobs` table). After this phase
the whole web console works end-to-end against the real backend.
</domain>

<decisions>
## Implementation Decisions

### Scope = the 8 FE-used hooks ONLY (fe-endpoint-inventory.md E10)
- Dashboard: `GET /dashboards/me`. Report: `GET /reports/attendance-billable`.
- Notifications: `GET /notifications`, `POST /notifications/{id}:mark-read`,
  `POST /notifications:mark-all-read`.
- Exports: `POST /exports`, `GET /exports/{id}`, `POST /exports/{id}:cancel`.

### Notifications + realizing the prior-phase dispatch stubs (success criterion 2 — the loop-closer)
- Create the **`notifications`** table (recipient, kind, title/body, link path, read_at, created_at,
  + the dedup/source refs the contract models). Endpoints: list (cursor, unread filter), mark-read
  (single), mark-all-read.
- **Un-stub the NotificationWorker:** `internal/platform/jobs/notify.go`'s `NotificationWorker.Work`
  is currently a no-op — make it INSERT a `notifications` row (real persistence).
- **Add a reusable dispatch helper** (e.g. `notify.Dispatch(ctx, tx, recipient, kind, payload)`
  that `EnqueueTx`'s a `NotificationArgs` in the caller's tx — transactional outbox).
- **Retro-wire the prior-phase `TODO(Phase-11)` notification points to call the dispatch helper**
  so auto-dispatched notifications are REAL: at minimum leave approval (l1/final/reject), OT
  approval (l1/final/reject), attendance verify/reject, change-request resolve, placement lifecycle.
  This is mechanical (replace each stub comment with a `notify.Dispatch(...)` enqueue in the
  existing tx). Each prior service already has the recipient context. Keep it additive — do not
  change existing behavior/tests beyond adding the enqueue.
- **E2E proves it end-to-end:** seed a few notifications so the list renders; AND perform a fresh
  action (e.g. HR approves a seeded leave) and assert a notification appears for the recipient and
  mark-read / mark-all-read flip the unread state. Do NOT fake — drive the real dispatch.

### Dashboard (`GET /dashboards/me`) — role-aware aggregation
- Returns the role-aware payload the contract models: pending-action counts by kind
  (ATTENDANCE_VERIFY, LEAVE_APPROVE, OT_APPROVE, PLACEMENT_EXPIRING, COVERAGE_GAP) each with a deep
  link path, plus today's team status (clock-ins / late count from E5). Read-only aggregation over
  the existing E3..E8 tables, scoped (leader sees own company; HR/super global). Match the openapi
  response shape exactly (what dashboard-screen.tsx + approval-inbox-panel.tsx render).

### Billable attendance report (`GET /reports/attendance-billable`)
- Aggregation over **verified** E5 attendance for a date range / company filter — billable
  day/hour counts per the contract. Cursor or full-set per the openapi. Scope-aware. Match the
  shape billable-report-screen.tsx renders.

### Export framework (`POST /exports` / `GET /exports/{id}` / `:cancel`) — generalize Phase-10
- Reuse the Phase-10 **`export_jobs`** table + River worker pattern. `POST /exports` (kind +
  filters + format) → insert export_jobs QUEUED + `EnqueueTx` a generic export worker in the same
  tx → **202 + { id, status }**. `GET /exports/{id}` → status (QUEUED/RUNNING/DONE/FAILED/CANCELLED)
  + result/download ref. `:cancel` → cancel a QUEUED/RUNNING job (→ CANCELLED). Codes:
  `EXPORT_FORMAT_UNSUPPORTED` (PDF — Excel only v1), `EXPORT_TOO_LARGE`, `RATE_LIMITED_EXPORTS`,
  `EXPORT_EXPIRED`, `EXPORT_FAILED`. The worker builds the artifact + marks DONE.
- The E8 payslip export (Phase-10) is the precedent; generalize so report kinds (attendance-billable
  etc.) flow through the same `/exports` framework. Whether to refactor PayslipExportWorker into a
  generic worker or add a sibling generic ExportWorker is Claude's discretion — keep both working.

### Build approach (mirror Phase-10 slice EXACTLY)
- migration → sqlc (`make gen`) → repository → service (apperr codes, RBAC scope, River enqueue,
  aggregation queries) → hand-written chi handlers → routes in server.go under RequireRole → Go
  contract tests → FE wiring (MSW off) + live Playwright E2E. Match
  `docs/api/E10-reporting/openapi.yaml` byte-for-byte. Cursor pagination (§11). New migration(s):
  `notifications` (+ any report/export-framework support). SWP IDs: check ids.go for NOTIF/EXP
  (EXP already exists from Phase-10); add only if missing. New query dir
  `backend/db/queries/reporting/` (or notifications/reporting split). The harness already spawns
  cmd/worker (Phase-10) — reuse for export + notification jobs.

### Seed (in 11-02)
- A handful of `notifications` for the seeded personas (mix read/unread, across kinds) so the list
  + mark-read flows render. Existing seeded E3..E8 data already feeds the dashboard counts +
  billable report; add anything needed so `/dashboards/me` and the report return non-empty for the
  personas. **TZ note:** clearly-in-range Asia/Jakarta dates.

### Plan split (4 plans, mirrors ROADMAP)
- **11-01** Migrations + sqlc + domain (`notifications`; report/export-framework support; reuse
  export_jobs).
- **11-02** Services + handlers: dashboard aggregation, billable report, notifications
  (list/mark-read/mark-all-read) + un-stub the NotificationWorker + the dispatch helper +
  **retro-wire the prior-phase dispatch points**, export framework (create/get/cancel async),
  RBAC scope, audit, seed. Edits server.go/main.go/seed.go/jobs.go + the prior service files (for
  the dispatch retro-wire). **This is the heaviest plan of the milestone** — if the retro-wire
  makes it too large, the planner may split it (e.g. 11-02a notifications+retrowire / 11-02b
  dashboard+report+exports) into an extra wave; otherwise keep 4 plans.
- **11-03** Go contract tests vs E10 openapi (dashboard shape, billable report, notifications
  list/mark-read/mark-all-read, export create 202 + get + cancel, EXPORT_FORMAT_UNSUPPORTED/
  TOO_LARGE/RATE_LIMITED, RBAC, cursor shapes).
- **11-04** Full-stack Playwright E2E under NEW frontend/e2e/tests/e10/ (per Gherkin AC: dashboard
  renders role-aware counts, billable report renders, notifications list + mark-read +
  mark-all-read + an auto-dispatched notification appears after a real action, export create →
  worker completes (DONE) + cancel). Selectors from the REAL e10-reporting + dashboard components.

### Claude's Discretion
- Notification table dedup/source-ref columns — match the contract; keep dispatch idempotent enough.
- Dashboard aggregation: live queries vs a lightweight rollup — pick live (read-only) for honesty.
- Generic export worker vs generalize PayslipExportWorker — keep both export paths working.
- Which/how many prior-phase dispatch points to retro-wire — cover enough that "auto-dispatched
  from earlier phases appear" is honestly demonstrated by E2E (at least leave + OT + attendance);
  wire the rest if cheap. Document any dispatch point left as a stub.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel (httpx cursor, rbac roles+scope, audit, apperr + error.details, ids, db.TxManager,
  i18n, TZ). **River client + worker already wired into BOTH the API (insert-only, Phase-10) and
  cmd/worker; the harness spawns cmd/worker (Phase-10).** `internal/platform/jobs/notify.go` has
  `NotificationArgs` + `NotificationWorker` (currently a no-op stub to un-stub). `export_jobs` table
  + the export job lifecycle (Phase-10) to generalize.
- **Reference slices = Phase-10 payroll (async export via River, the export_jobs lifecycle, the
  worker-completes-job E2E) and Phase-2 foundations (read/list aggregation).** Every prior epic's
  service has a `TODO(Phase-11)` notify point to retro-wire.
- E2E harness (real stack + cmd/worker + resetDb + loginAs PERSONAS.* + db poll helpers from
  Phase-10's pollExportJob). Layout `frontend/e2e/tests/{e1..e8,smoke}/` → add `e10/`.

### Established Patterns
- Transactional outbox: `jobs.EnqueueTx` inside the write tx. Async export → export_jobs DONE via
  the worker, asserted by DB poll (Phase-10). Cursor list; {data} detail envelope (unwrap on FE —
  recurring finding); apperr struct literals for non-default status; RBAC via RequireRole; audit-in-
  tx. FE errors via classifyError/error.details. DataTable rows div.border-b; .js E2E imports;
  PERSONAS.*. reset-db TRUNCATE in FK order, leaving River internal tables intact.

### Integration Points
- New query dir(s) under backend/db/queries/. New routes in server.go authenticated group (append
  after the payroll group). un-stub notify.go worker + add dispatch helper. Retro-wire prior service
  files. Seed extension. FE screens exist (e10-reporting/* + dashboard/*, built from .pen) calling
  `@swp/api-client` e10 hooks via MSW — wire to real BE. E2E under new frontend/e2e/tests/e10/.
  reset-db must TRUNCATE notifications (+ leave export_jobs handling consistent with Phase-10).
</code_context>

<specifics>
## Specific Ideas
- The dashboard (dashboard-screen.tsx) + inbox (inbox-screen.tsx, approval-inbox-panel.tsx) +
  notifications (notifications-screen.tsx) + billable report + export flow (use-export-flow.ts) are
  the primary surfaces — E2E drives REAL selectors/overlays, not invented ones.
- Notification loop-closer E2E (the milestone capstone): HR approves a seeded leave (or OT) →
  assert a notification row appears for the recipient via the notifications list; mark-read /
  mark-all-read flip unread→read. REAL dispatch, not seeded-only.
- Export E2E: POST /exports (attendance-billable, Excel) → 202 + id → poll export_jobs/GET /exports/{id}
  until DONE (real worker); PDF → EXPORT_FORMAT_UNSUPPORTED; cancel a job → CANCELLED.
- Dashboard E2E: role-aware — leader sees own-company counts, HR sees global; deep-link paths present.

</specifics>

<deferred>
## Deferred Ideas
- PDF export (v1.1 — EXPORT_FORMAT_UNSUPPORTED until then).
- Real notification delivery channels (email/push) — the worker persists in-app notification rows;
  external delivery is out of scope.
- Any prior-phase dispatch point not retro-wired (document which, if any).
- Non-FE E10 endpoints not in the 8-hook inventory.
</deferred>

---

*Phase: 11-e10-reporting*
*Context gathered: 2026-06-05 (autonomous) — FINAL milestone phase*
