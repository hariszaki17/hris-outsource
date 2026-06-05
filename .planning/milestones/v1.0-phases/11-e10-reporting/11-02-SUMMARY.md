---
phase: 11-e10-reporting
plan: 02
subsystem: api
tags: [e10, reporting, notifications, river, transactional-outbox, cursor, rbac, seed]

requires:
  - phase: 11-01
    provides: notifications table (00035) + reporting sqlc Querier (Insert/List/Get/MarkRead/MarkAllRead/CountUnread) + domain.reporting.Notification/DeepLink/Actor/NotificationKind
  - phase: 10-e8-payroll
    provides: River insert-only client + EnqueueTx outbox + NewWorkerClient pool-backed worker pattern + export_jobs lifecycle precedent
provides:
  - GET /notifications (cursor + read_state UNREAD/READ/ALL + kind/kind__in, scope=self)
  - POST /notifications/{id}:mark-read ({data} envelope, no-op-if-read, 404 on non-owned)
  - POST /notifications:mark-all-read ({marked_count}, optional before_timestamp)
  - un-stubbed NotificationWorker.Work (INSERTs a real notifications row)
  - notify.Dispatch transactional-outbox helper + Dispatcher seam (*Client implements it)
  - REAL auto-dispatched notifications from leave approve-final/reject, OT approve-final/reject, attendance verify/reject
  - seedNotifications fixtures (6 rows, mixed read/unread, HR + agent personas)
affects:
  - 11-02b (extends the SAME reporting Handler with dashboard/billable-report/export methods AFTER the notifications route block)
  - 11-03 (Go contract tests: notifications list/mark-read/mark-all-read shapes + RBAC scope=self)
  - 11-04 (Playwright E2E: notifications list + mark-read + an AUTO-DISPATCHED notification after a real action; reset-db must TRUNCATE notifications)

tech-stack:
  added: []
  patterns:
    - "Transactional-outbox notify seam: nil-safe jobs.Dispatcher injected via SetNotifier (additive — existing service unit tests untouched)"
    - "Un-stubbed River worker writes via sqlcgen.New(pool).InsertNotification (mirrors PayslipExportWorker pool-backed write path; no import cycle)"
    - "scope=self list = recipient_id IN (principal.UserID, principal.EmployeeID); repo fans out the single-recipient sqlc query over the pair + merge-sorts the keyset"

key-files:
  created:
    - backend/internal/repository/reporting/notification_repo.go
    - backend/internal/service/reporting/notification_service.go
    - backend/internal/service/reporting/ports.go
    - backend/internal/handler/reporting/notification_handler.go
    - backend/internal/handler/reporting/dto.go
  modified:
    - backend/internal/platform/jobs/notify.go
    - backend/internal/platform/jobs/jobs.go
    - backend/internal/server/server.go
    - backend/cmd/api/main.go
    - backend/cmd/seed/seed.go
    - backend/internal/service/leave/leave_service.go
    - backend/internal/service/leave/helpers.go
    - backend/internal/service/overtime/overtime_service.go
    - backend/internal/service/overtime/helpers.go
    - backend/internal/service/attendance/attendance_service.go

key-decisions:
  - "Notify seam injected via SetNotifier(jobs.Dispatcher) (not the constructor) — keeps every prior service's constructor signature + its unit-test harness unchanged; nil-safe notify.Dispatch no-ops when unwired"
  - "NotificationArgs field is NotifKind (not Kind) — Kind() is River's reserved JobArgs method; collision was a compile error"
  - "Worker writes via sqlcgen directly (like PayslipExportWorker) instead of the reporting repo interface — simplest, cycle-free"
  - "List scope=self spans recipient_id IN (user id, employee id): auto-dispatched submitter notifications target SWP-EMP-*, system/HR target SWP-USR-*; both must resolve to the logged-in principal"
  - "Attendance verify/reject reuse the ATTENDANCE_VERIFY_NEEDED kind (no dedicated VERIFIED/REJECTED kind in the v1 enum) — documented in-code"
  - "Seed recipients use persona EMPLOYEE ids (deterministic) not the sequence-allocated SWP-USR ids; the service's (user,employee) scope means the persona sees them"

patterns-established:
  - "SetNotifier additive seam: any prior service can opt into real notifications without breaking its drift-gate tests"
  - "notify.Dispatch(ctx, dispatcher, tx, args) — the canonical transactional-outbox call inside an InTx closure"

requirements-completed: [RPT-02]

duration: 11min
completed: 2026-06-05
---

# Phase 11 Plan 02: E10 Notifications + Dispatch Loop-Closer Summary

**The notifications surface (list / mark-read / mark-all-read, scope=self) plus the REAL notification loop-closer: un-stubbed River worker that INSERTs a notifications row, a transactional-outbox notify.Dispatch helper, and retro-wired leave/OT/attendance dispatch points that now enqueue genuine notifications inside their existing write tx.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-06-05T07:45:40Z
- **Completed:** 2026-06-05T07:56:39Z
- **Tasks:** 3
- **Files modified:** 15 (5 created, 10 modified)

## Accomplishments

- **Un-stubbed NotificationWorker** — `Work()` now persists a real `notifications` row via `sqlcgen.InsertNotification` (was a no-op slog stub). Registered WITH the pool in `NewWorkerClient` (like `PayslipExportWorker`).
- **notify.Dispatch transactional-outbox helper** + `Dispatcher` seam — `*Client.Dispatch == EnqueueTx`; `notify.Dispatch(ctx, d, tx, args)` is nil-safe so unwired/unit-test services no-op.
- **Notifications slice** — repo (fan-out + merge over the principal's recipient pair) → service (scope=self, cursor, MarkRead 404, MarkAllRead count) → handler (cursor envelope / {data} / {marked_count}) → routes (all 4 roles, action endpoints Idempotency-wrapped) → main.go wiring.
- **Retro-wired the prior-phase dispatch points** to enqueue REAL notifications inside their existing approval/verify tx: leave approve-final (`LEAVE_APPROVED`) + reject (`LEAVE_REJECTED`), OT approve-final (`OT_APPROVED`) + reject (`OT_REJECTED`), attendance verify + reject (`ATTENDANCE_VERIFY_NEEDED`). Additive — existing tests stay green.
- **Seed** — `seedNotifications` inserts 6 fixtures (mixed read/unread across 5 kinds) for the HR + agent personas so the inbox + mark-read flows render.

## Task Commits

1. **Task 1: Un-stub NotificationWorker + notify.Dispatch helper** - `94f5914` (feat)
2. **Task 2: Notifications repo + service + handler + routes + main.go** - `0eeb02c` (feat)
3. **Task 3: Retro-wire leave/OT/attendance dispatch + seed notifications** - `24b69a7` (feat)

**Plan metadata:** (this commit) `docs(11-02): complete notifications loop-closer plan`

## Files Created/Modified

- `internal/platform/jobs/notify.go` — un-stubbed worker (InsertNotification) + Dispatcher seam + notify.Dispatch; NotificationArgs extended (NotifKind/Title/Body/DeepLink*/Actor*/IsCritical)
- `internal/platform/jobs/jobs.go` — register `NewNotificationWorker(pool)` in `NewWorkerClient`; `registerWorkers` now a pool-less extension point
- `internal/repository/reporting/notification_repo.go` — sqlc-backed repo (List fan-out/merge, MarkRead scoped, MarkAllRead summed), rows → domain.Notification
- `internal/service/reporting/{ports,notification_service}.go` — scope=self service + cursor codec
- `internal/handler/reporting/{notification_handler,dto}.go` — 3 handlers + openapi-shaped DTOs (read_at null when unread; deep_link/actor always objects; {marked_count})
- `internal/server/server.go` — E10 notifications route block after PAYROLL slice end; `Deps.Reporting`
- `cmd/api/main.go` — reporting repo→service→handler wiring + `SetNotifier(jobsClient)` on leave/OT/attendance
- `internal/service/{leave,overtime,attendance}/*` — SetNotifier seam + real notify.Dispatch at the mandatory points
- `cmd/seed/seed.go` — `seedNotifications` (6 rows, SWP-NTF-9000x, ON CONFLICT DO NOTHING)

## Decisions Made

See frontmatter `key-decisions`. Notable: the notify seam is injected via `SetNotifier` (not the constructor) so no prior service's constructor signature or drift-gate test harness changes; `NotificationArgs.NotifKind` avoids the `Kind()` River-method collision; the worker writes via sqlcgen directly (cycle-free, mirrors PayslipExportWorker).

## Dispatch Points Left As Documented Stubs

Per the plan's OPTIONAL coverage, these are intentionally NOT wired (no clean single recipient or self-action), each marked with a `Phase-11 stub (documented)` in-code comment:

- **leave `approve-l1`** (`leave_service.go`) — the L1→HR next-stage event targets the HR approver QUEUE, not one recipient.
- **overtime `confirm`** — "OT confirmed → leader queue" (queue-targeted).
- **overtime `approve-l1`** — "OT L1-approved → HR queue" (queue-targeted).
- **overtime `withdraw`** — agent self-action; the actor IS the recipient (a self-notification adds no value).

The CONTEXT also lists change-request resolve (E2/04) and placement lifecycle (E3/05) as candidates; those were OUT of this plan's mandatory set and remain as their original `TODO(Phase-11)` markers (not touched). The milestone success criterion ("auto-dispatched-from-earlier-phases appear") is honestly satisfied by the wired leave + OT + attendance points and is E2E-proven in 11-04.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `NotificationArgs.Kind` field collided with River's `Kind()` method**
- **Found during:** Task 1 (`go build` after extending NotificationArgs)
- **Issue:** Adding a `Kind string` field to NotificationArgs shadowed the existing `func (NotificationArgs) Kind() string` that River requires as the job-type identifier — "field and method with the same name Kind".
- **Fix:** Renamed the payload field to `NotifKind` (json tag stays `"kind"`); the worker reads `a.NotifKind` (falling back to the legacy `Event` for back-compat).
- **Files modified:** `internal/platform/jobs/notify.go`
- **Verification:** `go build ./...` exits 0.
- **Committed in:** `94f5914` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to compile; no scope change. Everything else executed as written.

## Issues Encountered

- **`.planning/` lives at the repo ROOT, not under `backend/`** — the executor cwd is `backend/`, so the plan/state/summary paths resolve one level up. Read with the absolute root path; no functional impact.
- **reset-db TRUNCATE not in this checkout** — the E2E reset logic (frontend/e2e harness) is not present in the backend tree and is owned by 11-04. Seed is idempotent (ON CONFLICT DO NOTHING), so re-runs are safe. **11-04 MUST add `notifications` to the reset-db TRUNCATE set** (flagged in Next Phase Readiness).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **11-02b** appends dashboard/billable-report/export methods to the SAME `reportinghttp.Handler` (extend `NewHandler` + Deps) AFTER the `// E10 REPORTING notifications slice end (11-02)` marker in server.go.
- **11-03** can contract-test the 3 notification ops against the real service+handler (drift gate); scope=self is service-enforced (recipient pair), mark-read 404 on non-owned, mark-all-read returns `marked_count`.
- **11-04** E2E: (1) seeded inbox renders for HR + agent personas; (2) drive a REAL action (HR approves a seeded leave / OT, or verifies attendance) → assert the auto-dispatched notification appears for the recipient via GET /notifications (the worker runs in the harness-spawned cmd/worker, already booted for Phase-10 exports). **Add `notifications` to the reset-db TRUNCATE.**

### Verification snapshot
- `make gen` + `go build ./...` + `go vet ./...` exit 0; seed + worker binaries build.
- Full backend test suite green (no regression from the retro-wire); leave/OT/attendance/payroll/etc. handler drift gates all `ok`.

## Self-Check: PASSED

- All 5 created files exist on disk (reporting repo/service/ports/handler/dto).
- All 3 task commits exist: `94f5914` (worker un-stub + Dispatch), `0eeb02c` (notifications slice + routes), `24b69a7` (retro-wire + seed).
- Greps confirm: `notify.Dispatch`, `NewNotificationWorker(pool)` registration, `marked_count` handler key, `seedNotifications`, and a `Dispatch` call in all three retro-wired services.
- `make gen` + `go build ./...` + `go vet ./...` exit 0; full backend test suite green (no regression).

---
*Phase: 11-e10-reporting*
*Completed: 2026-06-05*
