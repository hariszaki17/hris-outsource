---
phase: 11-e10-reporting
plan: 04
subsystem: testing
tags: [e10, reporting, dashboard, notifications, exports, playwright, e2e, full-stack, capstone, milestone]

requires:
  - phase: 11-02
    provides: notifications endpoints (list/mark-read/mark-all-read marked_count) + the un-stubbed NotificationWorker + retro-wired leave/OT/attendance dispatch + seeded SWP-NTF-9000x fixtures
  - phase: 11-02b
    provides: GET /dashboards/me (role-aware) + GET /reports/attendance-billable (verified-only) + generic export framework (POST /exports → ReportExportWorker → DONE; :cancel; DB↔wire PROCESSING/COMPLETED) + seeded VERIFIED billable rows SWP-ATT-9007/9008
  - phase: 11-03
    provides: the E10 Go contract drift gate (the wire shapes the FE client is generated from)
  - phase: 10-e8-payroll
    provides: the Phase-10 E2E harness (cmd/api + cmd/worker spawn + pollExportJob DB poll + reset-db TRUNCATE + loginAs PERSONAS.*)
provides:
  - the E10 FE screens wired off MSW to the real Go BE (the recurring {data}-envelope double-unwrap fixed in dashboard/report/export-flow; marked_count fix in notifications)
  - 5 Playwright E2E specs under frontend/e2e/tests/e10/ (14 tests): dashboard role-aware, billable report, notifications list/mark-read/mark-all, the auto-dispatch CAPSTONE, exports worker-DONE/cancel/PDF-unsupported
  - frontend/e2e/lib/e10-helpers.ts (gotoReady, pollNotification, pollExportJobUntil, listNotificationsVia, DEWI/LR/NTF fixtures)
  - reset-db.ts TRUNCATEs notifications
  - the MILESTONE: the whole web console works end-to-end against the real Go backend (e1..e10 green)
affects: []

tech-stack:
  added: []
  patterns:
    - "gotoReady(route) = goto → waitForToken → networkidle: lets the screen's data query settle (main.tsx awaits tryRestoreSession before createRoot, so the token is present at mount) before asserting rendered data"
    - "pollNotification(predicate): proves a REAL auto-dispatched notification landed via the un-stubbed worker (worker-driven, not seeded) by polling GET /notifications until a row matches kind+deep-link-entity"
    - "Export cancel E2E retries create+cancel across the cancel-vs-worker race (jobs complete in ~0s) until the cancel wins (status CANCELLED), then DB-confirms the terminal state"

key-files:
  created:
    - frontend/e2e/lib/e10-helpers.ts
    - frontend/e2e/tests/e10/dashboard.spec.ts
    - frontend/e2e/tests/e10/billable-report.spec.ts
    - frontend/e2e/tests/e10/notifications.spec.ts
    - frontend/e2e/tests/e10/notifications-autodispatch.spec.ts
    - frontend/e2e/tests/e10/exports.spec.ts
  modified:
    - frontend/apps/web/src/features/e10-reporting/notifications-screen.tsx
    - frontend/apps/web/src/features/e10-reporting/dashboard-screen.tsx
    - frontend/apps/web/src/features/e10-reporting/billable-report-screen.tsx
    - frontend/apps/web/src/features/e10-reporting/use-export-flow.ts
    - frontend/e2e/lib/reset-db.ts

key-decisions:
  - "The dashboard + billable-report + export-flow screens needed a DOUBLE {data}-unwrap (query.data.data.data), not single: Orval's customFetch wraps the HTTP body in {data,status,headers} AND the BE handler wraps the payload in {data:<T>} (dataResponse, every epic) even though the E10 openapi declares the bare schema. With only one unwrap the dashboard rendered AgentFallback and the report/export rendered empty against the real BE. Fixed toward what the BE returns (recurring finding; cf. [08-04]/[10-04]). The notifications LIST is single-wrapped (the cursor envelope {data:rows,next_cursor,has_more} IS the HTTP body) so notifications-screen needed no unwrap change — only the marked_count fix."
  - "mark-all-read onSuccess reads res.data.marked_count (the BE/openapi MarkAllNotificationsRead200 key), was res.data.marked — the documented 11-02 bug."
  - "The capstone drives the REAL dispatch via apiAs POST /leave-requests/SWP-LR-8002:approve-final (HR, PENDING_HR→APPROVED) for determinism; the asserted notification references SWP-LR-8002 (the just-approved entity), distinct from the seeded LEAVE_APPROVED fixture (SWP-LR-8005) — proving it is worker-dispatched, not seeded. Recipient = Dewi Lestari (dewi.lestari@swp.test, SWP-EMP-3001), the submitter."
  - "Export cancel asserts CANCELLED by retrying create+cancel across the cancel-vs-worker race (4/5 cancels win empirically); the QUEUED→CANCELLED transition is also pinned by the 11-03 contract test."
  - "gotoReady waits for networkidle (not a reload): main.tsx awaits tryRestoreSession() before createRoot().render(), so the token is in memory when the screen mounts and fires its query — the brief context-canceled 500s in the logs are the StrictMode/auth-restore double-mount discards, recovered by the retryable-500 query retry."

patterns-established:
  - "gotoReady(page, route): the E10 navigate-and-settle helper for data screens"
  - "pollNotification: the worker-driven auto-dispatch E2E proof (real dispatch, not seeded)"

requirements-completed: [RPT-01, RPT-02, RPT-03, RPT-04]

duration: 53min
completed: 2026-06-05
---

# Phase 11 Plan 04: E10 FE Wiring + Full-Stack Playwright (Milestone Capstone) Summary

**The final milestone plan: the E10 reporting/dashboard/notifications/export screens wired off MSW to the real Go BE (fixing the recurring {data}-envelope double-unwrap in dashboard/report/export-flow + the notifications marked_count bug), proven by 14 exhaustive full-stack Playwright specs under frontend/e2e/tests/e10/ — including the loop-closer CAPSTONE (a real HR approve-final auto-dispatches a notification the recipient genuinely sees, via the un-stubbed worker) and the export worker-completes/cancel/PDF-unsupported flow. Full e1..e10 suite: 239 passed / 6 skipped / 0 failed. The whole web console now works end-to-end against the real backend — the milestone is closed.**

## Performance

- **Duration:** ~53 min
- **Started:** 2026-06-05T08:28:36Z
- **Completed:** 2026-06-05T09:21:45Z
- **Tasks:** 3
- **Files modified:** 11 (6 created, 5 modified)

## Accomplishments

- **FE wiring off MSW (fixed toward the contract):**
  - `notifications-screen.tsx` — mark-all-read onSuccess reads `res.data.marked_count` (was `marked`).
  - `dashboard-screen.tsx` + `billable-report-screen.tsx` + `use-export-flow.ts` — the recurring **double {data}-unwrap** (`query.data.data.data`): Orval's customFetch wraps the HTTP body in `{data,status,headers}` AND the BE handler wraps the payload in `{data:<T>}` though the E10 openapi declares the bare schema. With only one unwrap the dashboard rendered the agent fallback and the report/export rendered empty against the real BE; with the fix all three render. A bare fallback keeps each robust if the envelope ever flattens. (The notifications LIST is single-wrapped — the cursor envelope IS the body — so its screen needed no unwrap change.)
- **reset-db.ts** — added `notifications` to `TRUNCATE_TABLES` (no FK to kept tables; seed re-applies SWP-NTF-9000x).
- **e10-helpers.ts** — reuses `apiAs`/`PERSONAS`/`pollExportJob`; adds `gotoReady` (navigate-and-settle), `pollNotification` (worker-driven auto-dispatch proof), `pollExportJobUntil` (CANCELLED-terminal), `listNotificationsVia`, and the `DEWI`/`LR`/`NTF` fixtures.
- **5 Playwright specs (14 tests), all green headless against the real stack:**
  - `dashboard.spec.ts` — HR global HrDashboard (role_label, KPI cards, inbox deep-link paths), super_admin label, shift_leader own-company LeaderDashboard (company subtitle + today cards) NOT the global KPI shape.
  - `billable-report.spec.ts` — HR report renders summary StatCards + non-empty billable rows (seeded VERIFIED rows), leader server-scoped (cross-company → 403 OUT_OF_SCOPE), >1yr → 422 REPORT_PERIOD_TOO_WIDE.
  - `notifications.spec.ts` — seeded list renders, UNREAD pill filters, single mark-read flips unread→read (BE read_at), mark-all-read clears unread (marked_count toast + button hides), filtered-empty kind state.
  - `notifications-autodispatch.spec.ts` — **THE CAPSTONE**: HR approve-final on SWP-LR-8002 → the un-stubbed NotificationWorker INSERTs a LEAVE_APPROVED row for the submitter (Dewi) → it appears in her GET /notifications (worker-driven, references the just-approved entity, NOT seeded) and in the UI, where clicking flips unread→read.
  - `exports.spec.ts` — POST /exports → 202 + SWP-EXP id THEN the real ReportExportWorker flips export_jobs → DONE (DB poll) + the report Ekspor modal reaches its Unduh success step; :cancel → CANCELLED (retried across the race); PDF → 422 EXPORT_FORMAT_UNSUPPORTED.

## Task Commits

1. **Task 1: FE wiring (marked_count) + reset-db notifications + e10-helpers** — `65bbc73` (fix)
2. **Task 1 (cont.): dashboard/report/export {data}-envelope double-unwrap** — `92a5a3d` (fix)
3. **Task 2: dashboard + billable-report + notifications specs** — `a611b8f` (test)
4. **Task 3: auto-dispatch capstone + export framework specs** — `0ec682b` (test)

**Plan metadata:** (this commit) `docs(11-04): complete E10 FE-wiring + full-stack E2E milestone plan`

## Final Regression — the milestone capstone

`cd frontend && pnpm e2e` (full suite, headless, real Go BE + River worker + ephemeral Postgres):

- **239 passed / 6 skipped / 0 failed** (10.3m). Exit 0.
- 14 new e10 tests across 5 specs (3 dashboard + 3 billable-report + 4 notifications + 1 capstone + 3 exports), zero regressions in e1..e9 (225 prior e1..e8 + e9 + the new e10).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] dashboard + billable-report + export-flow rendered empty/agent-fallback against the real BE (single {data}-unwrap)**
- **Found during:** Task 2 (first headless run — dashboard/report UI assertions failed; the screens showed the agent fallback / "Tidak ada hasil" while the apiAs assertions on the same endpoints returned 200 with data)
- **Issue:** The plan flagged a possible unwrap fix but expected the existing `query.data.data` to match. Against the real BE the screens needed a SECOND unwrap: the BE wraps the payload in `{data:<T>}` (dataResponse) though the E10 openapi declares the bare schema, and Orval's customFetch already wraps the HTTP body in `{data}` — so the real payload is at `query.data.data.data`.
- **Fix:** Peel both envelopes (bare fallback) in `dashboard-screen.tsx`, `billable-report-screen.tsx`, and `use-export-flow.ts` (an `unwrapExportJob` helper for createExport + poll). The notifications LIST was already correct (single-wrapped cursor envelope).
- **Files modified:** `dashboard-screen.tsx`, `billable-report-screen.tsx`, `use-export-flow.ts`
- **Commit:** `92a5a3d`

**2. [Rule 3 - Blocking] initial-mount auth-restore race surfaced canceled 500s on the data screens**
- **Found during:** Task 2 (the first test in each file failed on the screen-rendered heading/data)
- **Issue:** The screen's data query could fire during the StrictMode/auth-restore double-mount and get context-canceled (logged as a 500). The retryable-500 query retry recovers it, but the first UI assertion could land before recovery.
- **Fix:** `gotoReady(page, route)` = goto → waitForToken → `waitForLoadState('networkidle')`, so the query has settled before assertions. (No reload needed — main.tsx awaits `tryRestoreSession()` before `createRoot().render()`, so the token is present at mount.)
- **Files modified:** `frontend/e2e/lib/e10-helpers.ts`
- **Committed in:** `65bbc73` / `92a5a3d`

### Environment note (not a deviation)

- A reproducible-looking `405 Method Not Allowed` on `POST /exports/{id}:cancel` during manual curl validation was traced to a SHELL-quoting artifact (a trailing `\r` from a `psql`/`python` id extraction collapsed `:c` so the URL became `…:cancel` → `…ancel`, missing the `:cancel` literal → chi fell to the GET node). The cancel route is correct; the FE/E2E send clean ids and `:cancel` returns 200. No BE/FE change.

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking). No architectural changes; no auth gates.

## Verification snapshot

- `pnpm -C apps/web exec tsc --noEmit` exits 0 (web app typechecks with the unwrap fixes).
- `cd frontend/e2e && npx tsc --noEmit` exits 0 (E2E specs + helpers typecheck).
- Manual BE validation against the booted stack: HR approve-final SWP-LR-8002 → Dewi sees `LEAVE_APPROVED | Cuti disetujui | SWP-LR-8002`; POST /exports → SWP-EXP DONE; PDF → 422 EXPORT_FORMAT_UNSUPPORTED; :cancel (clean id) → 200/CANCELLED.
- `cd frontend && pnpm e2e` — **239 passed / 6 skipped / 0 failed** (the full e1..e10 milestone capstone, no regressions).

## Self-Check: PASSED

(Appended below after file + commit verification.)

## Next Phase Readiness

This is the FINAL plan of the milestone. After 11-04 the whole web console works end-to-end against the real Go backend (e1..e10 green) — the milestone is closed. No further phases in v1.0.

---
*Phase: 11-e10-reporting*
*Completed: 2026-06-05*
