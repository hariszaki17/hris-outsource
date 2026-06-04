---
phase: 07-e5-attendance
plan: 04
subsystem: e2e
tags: [playwright, e2e, attendance, corrections, verification, bulk, idempotency, rbac, harness]

# Dependency graph
requires:
  - phase: 07-e5-attendance
    provides: "10 FE-used E5 endpoints live against the real BE (verify/reject + bulk + corrections approve/reject); seed fixtures SWP-ATT-9001..9006 + SWP-COR-8001/8002; reset-db TRUNCATE list"
  - phase: 07-e5-attendance
    provides: "E5 contract tests (drift gate) — codes OUT_OF_SCOPE/VERIFY_OWN_RECORD/CONFLICT/IDEMPOTENCY_KEY_REUSED/OUTSIDE_CORRECTION_WINDOW"
  - phase: 06-e4-schedule-shifts
    provides: "E2E harness pattern (loginAs, PERSONAS, resetDb, apiAs, waitForToken, e4-helpers re-exports, div.border-b rows)"
provides:
  - "E5 full-stack Playwright suite (5 specs, 18 tests) GREEN headless against real FE + real Go API + ephemeral Postgres"
  - "e5-helpers.ts (apiAs/apiAsWithKey/errorCode/waitForToken re-exports + ATT/COR seed-id constants + queueRow helpers)"
  - "harness orphan-API reaping fix (freePort + detached process-group kill) — go run no longer leaves a stale :8081 binary serving old routes"
affects: [E7-overtime, E8-payroll, E10-reporting]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "apiAsWithKey(page, method, path, body, key) sends a FIXED Idempotency-Key so two calls replay identically against the REAL Postgres idempotency store (vs apiAs which sends a fresh random key per call)."
    - "Bulk partial-success E2E driven via apiAs for determinism: verify/reject one id first to make it terminal, then bulk over [terminal, pending] → 200 {succeeded:[pending], failed:[{terminal, CONFLICT}]}; two terminal ids → 422 all-fail."
    - "Drawer overlay text collision: list-row text (dimmed behind the Drawer) shadows the drawer's copy under getByText().first() → anchor drawer assertions on content UNIQUE to the drawer (diff-table field key / correction id)."
    - "Harness API lifecycle: `go run ./cmd/api` execs a child binary that does NOT receive the parent's SIGTERM; spawn detached + kill the process-group on teardown AND freePort(8081) before boot so the new binary actually serves (was the 404 root cause)."

key-files:
  created:
    - frontend/e2e/lib/e5-helpers.ts
    - frontend/e2e/tests/e5/attendance-list-detail.spec.ts
    - frontend/e2e/tests/e5/verify-reject.spec.ts
    - frontend/e2e/tests/e5/corrections.spec.ts
    - frontend/e2e/tests/e5/bulk-idempotency.spec.ts
    - frontend/e2e/tests/e5/scope-negatives.spec.ts
  modified:
    - frontend/e2e/lib/backend.ts

key-decisions:
  - "reset-db.ts + correction-overlays.tsx required NO change: 07-02 already added attendance_corrections+attendance to TRUNCATE_TABLES in correct FK order, and no `conflict_details` literal exists anywhere under apps/web/src/features/e5-attendance/ (all screens already read errors via classifyError(err).message → error.details). Task-1's FE audit was a verified no-op."
  - "OUTSIDE_CORRECTION_WINDOW (422) is NOT asserted in web E2E: the correction-CREATE path is mobile/agent-only (out of web scope) and 07-02 seeds only in-window corrections, so a leader-approvable out-of-window correction is unreachable from the browser. It is fully contract-tested in 07-03 via the exported CheckCorrectionWindow seam (documented, not faked)."
  - "Bulk determinism via apiAs not the BulkBar UI: the BulkBar verifies ALL selected rows and every seeded PENDING row is actionable, so a real partial-failure cannot be forced through pure UI selection. Driving bulk-verify/reject through apiAs over [terminal, pending] yields a deterministic {succeeded,failed} envelope (the BulkBar UI itself is exercised indirectly by the same hooks the screen calls)."
  - "Single verify/reject + detail navigation use direct /attendance/$id routing for determinism (9002 and 9004 both belong to EMP-3001, so a queue-row text match on employee_id is ambiguous); Sari's EMP-1042 row (9003) is the only unambiguous single-row UI target."

patterns-established:
  - "queueRow/expectQueueRow/expectNoQueueRow: DataTable (div.border-b) row helpers filtered by the employee_id rendered mono — the E5 analog of the e3/e4 div.border-b row pattern."

requirements-completed: [ATT-01, ATT-02]

# Metrics
duration: ~75min
completed: 2026-06-05
---

# Phase 7 Plan 04: E5 Attendance FE Wiring + Full-Stack E2E Summary

**The built E5 screens (verification queue, detail, corrections drawer + reject modal) now work end-to-end against the real Go backend with MSW off, proven by an exhaustive 5-spec / 18-test Playwright suite that is GREEN headless against the real FE + real Go API + ephemeral Postgres — list/scope/detail, single verify/reject (+validation) + inline-queue verify, corrections approve (target attendance gains CORRECTED) + reject, bulk partial-success with a real CONFLICT failure, Postgres-backed idempotency replay + IDEMPOTENCY_KEY_REUSED 409, and the leader OUT_OF_SCOPE + VERIFY_OWN_RECORD 403 negatives against the seeded fixtures. The full `pnpm e2e` is green (163 passed, 6 skipped, 0 failures) with zero e1/e2/e3/e4 regressions. A latent harness defect (orphaned `go run ./cmd/api` child holding :8081 and serving a STALE binary → 404 on the new E5 routes) was found and fixed.**

## Performance
- **Duration:** ~75 min (incl. harness-defect diagnosis + full-suite regression run)
- **Tasks:** 3
- **Files:** 6 created, 1 modified

## Accomplishments
- **Task 1 — helpers + audit:** verified the interrupted-run `e5-helpers.ts` artifact is correct and reused it (re-exports apiAs/errorCode/errorDetails/waitForToken; adds apiAsWithKey with a fixed Idempotency-Key; ATT/COR seed-id constants; queueRow DataTable helpers). Confirmed reset-db already truncates attendance_corrections+attendance (07-02) and that NO `conflict_details` bug exists in e5-attendance (FE audit = no-op).
- **Task 2 — core E2E (3 specs, 10 tests):** `attendance-list-detail` (HR queue lists PENDING exceptions / AUTO_APPROVED 9001 excluded; SL scope banner + hidden company filter + CMP-0022 absent; row-click → detail header + ids); `verify-reject` (detail verify → VERIFIED via apiAs cross-check; detail reject + reason → REJECTED + reason persisted; <5-char reason disables the ConfirmDialog confirm; inline-queue Verifikasi → row leaves the PENDING queue on refetch); `corrections` (PENDING list 8001/8002; approve 8001 → drawer diff renders → Setujui → correction APPLIED **and** target attendance 9004 gains the CORRECTED flag; reject 8002 via the reject modal → REJECTED).
- **Task 3 — bulk + idempotency + scope (2 specs, 8 tests):** `bulk-idempotency` (partial verify [terminal 9002, pending 9003] → 200 succeeded=[9003]+failed[9002 CONFLICT]; fixed-key replay returns the byte-identical 200 body, then a key-reuse with a different body → 409 IDEMPOTENCY_KEY_REUSED against the REAL Postgres store; bulk-reject partial; 422 all-fail); `scope-negatives` (leader Rudi → 403 OUT_OF_SCOPE on cross-company verify + list, 403 VERIFY_OWN_RECORD on his own escalated 9006, HR global 200) — all triggering the REAL codes against the seeded fixtures.

## Task Commits
1. **Task 1: e5-helpers (reuse interrupted artifact) + reset-db/FE-audit verification** — `875d114` (test)
2. **Task 2: list/detail + verify/reject + corrections E2E + harness orphan-API fix** — `6299559` (test)
3. **Task 3: bulk partial-success + idempotency + scope-negative E2E** — `8a99d53` (test)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Harness left a stale API on :8081 serving old routes (→ 404 on E5 endpoints)**
- **Found during:** Task 2 (first E2E run — every E5 screen showed "Gagal memuat data"; direct curl of `GET /api/v1/attendance` returned chi's bare `404 page not found`).
- **Root cause:** `frontend/e2e/lib/backend.ts` spawns `go run ./cmd/api`, which compiles and execs a CHILD binary (`exe/api`). `stopBackend` sent SIGTERM only to the `go run` parent, which does NOT forward the signal — leaving an ORPHANED `exe/api` bound to :8081. A subsequent run's fresh API then failed to bind ("address already in use") and exited, while the orphan kept serving a STALE binary compiled BEFORE the 07-02 E5 routes existed → 404 on `/attendance` + `/corrections` (while `/schedule`, `/attendance-codes` from older binaries still answered).
- **Fix:** added `freePort(8081)` (lsof + SIGKILL) before boot, spawned the API `detached` (own process group), and on teardown killed the whole group via `process.kill(-pid, 'SIGTERM')` with a `freePort` fallback. After the fix `/attendance` served 200 and 9/10 (then 10/10) Task-2 tests passed.
- **Files modified:** frontend/e2e/lib/backend.ts
- **Commit:** 6299559

### No-op audits (planned work that required no change)
- **reset-db.ts:** already contained `attendance_corrections` + `attendance` in correct FK order (added by 07-02) — Task-1 acceptance satisfied without an edit.
- **correction-overlays.tsx / e5-attendance/*:** no `conflict_details` literal anywhere; all screens already surface BE errors via `classifyError(err).message` (the `error.details` contract). Documented as "no conflict_details bug in E5" per the plan.

## Issues Encountered
- **Drawer overlay text collision (Task 2, COR-approve):** `getByText('Koreksi jam keluar').first()` matched the dimmed LIST row behind the Drawer (reported "hidden") rather than the drawer copy. Re-anchored the assertion on drawer-unique content (correction id + the `check_out_at` diff-table field key). Green thereafter.

## Authentication Gates
None.

## Next Phase Readiness
- **ATT-01 + ATT-02 fully closed** — every E5 web surface works end-to-end against the real BE with MSW off, exhaustively asserted (list/detail, single + bulk verify/reject with partial success + real Postgres idempotency, corrections approve/reject, OUT_OF_SCOPE, VERIFY_OWN_RECORD).
- **Harness hardening benefits all later phases:** the orphan-API reaping fix removes a flaky-404 trap that would otherwise have bitten E6/E7/E8 E2E runs.
- **OUTSIDE_CORRECTION_WINDOW** remains covered only at the contract layer (07-03) until the mobile correction-CREATE path lands — noted for the mobile epic.

## Self-Check: PASSED
- All 6 created + 1 modified files present on disk.
- All three task commits (875d114, 6299559, 8a99d53) found in git log.
- Full `pnpm e2e` green: 163 passed, 6 skipped, 0 failed; e5 suite = 18 passed; no e1/e2/e3/e4 regressions.

---
*Phase: 07-e5-attendance*
*Completed: 2026-06-05*
