---
phase: 06-e4-schedule-shifts
plan: 04
subsystem: frontend
tags: [e2e, playwright, e4, scheduling, conflict-engine, bulk-apply, rbac-scope, fe-wiring, full-stack]

# Dependency graph
requires:
  - phase: 06-e4-schedule-shifts
    plan: 02
    provides: real Go E4 endpoints (shift-master CRUD, schedule CRUD, conflict engine, bulk-apply, :check) + seed (SWP-SHF-001/002, SWP-SCH-6001/6002, approved_leave_days SWP-LR-44210)
  - phase: 06-e4-schedule-shifts
    plan: 03
    provides: E4 contract-test drift gate (response shapes locked to openapi) + fixture-id anchors
  - phase: 05-e3-placement
    plan: 04
    provides: E2E harness patterns (e3-helpers apiAs/pickCombobox, window.__swp_get_token__, resetDb, loginAs personas, error.details envelope)
provides:
  - frontend/e2e/lib/e4-helpers.ts (E4 DOM/selectors + apiAs re-export + seed-anchored dates + waitForToken)
  - 5 E4 spec files (shift-masters, schedule-grid, conflict-negatives, bulk-apply, leader-scope) — 27 tests
  - FE wiring fix: schedule-overlays reads conflict details via error.details (not conflict_details)
  - reset-db extended to truncate E4 tables (schedule_entries / approved_leave_days / shift_masters)
  - green full-stack proof of all 4 ROADMAP Phase-6 success criteria
affects: [07-attendance (schedule entries are the attendance reference), 08-e6-leave (over-leave production source)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "waitForToken(page): poll window.__swp_get_token__ after a full page.goto() before the first apiAs — the in-memory access token is re-hydrated ASYNCHRONOUSLY by tryRestoreSession (refresh-cookie → /auth/refresh), so an immediate apiAs races to 401"
    - "Grid cell selector anchored on the real i18n aria-label 'cell.ariaLabel'='{{agent}} — {{date}}' via [aria-label*=agent][aria-label*=date]; popover rooted on 'picker.title'='Pilih shift untuk {name}'"
    - "C-5 leader past-date DELETE guard worked around in CH-1 by creating a FUTURE cell first, then clearing it — keeps the test a genuine leader-scoped delete independent of which weekday 'today' is"
    - "Conflict negatives asserted two ways: direct apiAs single-POST (top-level error envelope) AND, for over-leave/double-shift, the real popover :check block toast"

key-files:
  created:
    - frontend/e2e/lib/e4-helpers.ts
    - frontend/e2e/tests/e4/shift-masters.spec.ts
    - frontend/e2e/tests/e4/schedule-grid.spec.ts
    - frontend/e2e/tests/e4/conflict-negatives.spec.ts
    - frontend/e2e/tests/e4/bulk-apply.spec.ts
    - frontend/e2e/tests/e4/leader-scope.spec.ts
  modified:
    - frontend/apps/web/src/features/e4-scheduling/schedule-overlays.tsx
    - frontend/e2e/lib/reset-db.ts

key-decisions:
  - "FE conflict-details fix: ShiftPickerPopover read failed[].conflict_details (undefined against the real BE); changed to failed[].error.details — the real envelope nests details under error (mirrors the Phase-5 error.details precedent). Block messages now render."
  - "reset-db.ts (blocking, Rule 3): added schedule_entries / approved_leave_days / shift_masters to TRUNCATE_TABLES (before placements/employees for FK order). Without it, test-created schedule entries survived across specs (seed is ON CONFLICT DO NOTHING) and would pollute later runs."
  - "SHIFT_OVER_LEAVE delivered HONESTLY: driven by the seeded approved_leave_days row (SWP-EMP-3001 / monday+3 / SWP-LR-44210). Asserted via real 409 details.leave_request_id==='SWP-LR-44210' (apiAs) AND the real popover :check block toast — never mocked. E6 (Phase 8) wires the production leave source."
  - "Date-sensitivity: the seed places entries on monday+1/+2 and leave on monday+3 of the CURRENT week. On the run day (Fri Jun 5) those are PAST dates. PATCH has no past guard (CH-2/SA-2 edit seeded past cells fine); only DELETE does (C-5) → CH-1 creates a future cell first."

# Metrics
duration: ~69min
completed: 2026-06-04
---

# Phase 6 Plan 04: E4 Schedule & Shifts — Full-Stack E2E + FE Wiring Summary

**Wired the E4 screens off MSW to the real Go API and proved them with 27 full-stack Playwright tests across 5 new `frontend/e2e/tests/e4/` specs: shift-master CRUD, schedule-grid cell CRUD via the real ShiftPickerPopover, every reachable conflict code (DOUBLE_SHIFT, OUTSIDE_PLACEMENT_PERIOD, the honestly-seeded SHIFT_OVER_LEAVE, SHIFT_DEACTIVATED, SHIFT_NOT_FOR_SERVICE_LINE), bulk-apply partial success, and leader-scope 403. Two real FE↔BE / harness fixes were found and corrected (conflict_details→error.details; reset-db E4 truncation). `pnpm e2e` is GREEN headless — 145 passed, 0 failed — with zero e1/e2/e3 regressions.**

## Performance

- **Duration:** ~69 min
- **Tasks:** 2
- **Files:** 6 created + 2 modified

## Task Commits

1. **Task 1: FE wiring + e4-helpers + shift-masters + schedule-grid specs** — `66b11f1` (feat) — 15 tests green
2. **Task 2: conflict-negative + bulk-apply + leader-scope specs** — `0db2ea4` (test) — 12 tests green

**Plan metadata:** _(final docs commit — this SUMMARY + STATE + ROADMAP + REQUIREMENTS)_

## Spec → scenario coverage map (27 tests)

**shift-masters.spec.ts (8)** — `SM-list` (Pagi/Malam + cross-midnight indicator) · `SM-create` · `SM-create-cross-midnight` (note + server-derived chip) · `SM-duplicate-name` (real 409 DUPLICATE_NAME, apiAs + UI) · `SM-break-outside-window` (real 422 BREAK_OUTSIDE_WINDOW) · `SM-deactivate-reactivate` · `SM-filter-status` · `SM-rbac-leader-readonly` (leader reads list, POST → 403).

**schedule-grid.spec.ts (7)** — `SA-1-assign-autopublish` (empty cell → Pagi → published toast → chip) · `SA-2-replace` (existing cell → Malam) · `SA-4-picker-filtered-by-service-line` (SVC-003 agent → Malam + untagged Pagi, SM-3) · `SA-7-mark-day-off` (Libur) · `CH-1-clear-cell` (future cell create-then-clear, real DELETE 204) · `CH-2-edit-swap` (PATCH → status MODIFIED, apiAs-verified) · `grid-empty`.

**conflict-negatives.spec.ts (5)** — `CONF-double-shift` (409 + existing_entry_id) · `CONF-outside-placement-period` (422) · `CONF-shift-over-leave` (409 + details.leave_request_id===SWP-LR-44210, apiAs + popover toast) · `CONF-shift-deactivated` (422) · `CONF-shift-not-for-service-line` (422 + placement/shift service_line_id details).

**bulk-apply.spec.ts (4)** — `BULK-partial` (UI preview succeeded>0 AND failed>0 spanning the leave Thu + apply; apiAs :bulk-apply 200 with failed[].error.code SHIFT_OVER_LEAVE) · `BULK-all-failed` (422, failed non-empty, succeeded empty) · `BULK-weekdays-mask` (`[1..5]` over Mon–Sun → exactly 5 cells) · `CHECK-dry-run` (:check 200 + no entry persisted).

**leader-scope.spec.ts (3)** — `SCOPE-403-create` (Rudi → EMP-2891 @ CMP-0022 → 403 OUT_OF_SCOPE) · `SCOPE-403-list` (Rudi GET company 0022 → 403) · `SCOPE-hr-global` (hr_admin GET 0022 → 200).

## SHIFT_OVER_LEAVE delivery (honest, not mocked)

Driven by the **real** seeded `approved_leave_days` row (06-01/06-02): `SWP-EMP-3001` / `leave_date=monday+3` / `leave_request_id=SWP-LR-44210` / `ANNUAL`. The conflict engine reads it via `FindApprovedLeaveForAgentDate` (no fake path). E2E asserts both the real **409 envelope** (`details.leave_request_id === 'SWP-LR-44210'`) and the real **popover :check block toast**, plus a bulk-apply cell that fails over-leave. **E6 production source note:** Phase 8 (E6 Leave) later populates/supersedes `approved_leave_days` from the production `leave_requests` source; the `SWP-LR-*` id carries no FK so namespaces never collide. No deferral — the success criterion is met honestly.

## FE / harness fixes (Phase-5 style)

**1. [Rule 1 — Bug] schedule-overlays read conflict details from the wrong key.**
- **Found:** Task 1, against the real BE.
- **Issue:** `ShiftPickerPopover.handleAssignShift` read the `:check` failed item as `first?.conflict_details` (always `undefined`), so DOUBLE_SHIFT/over-leave block messages never rendered.
- **Fix:** read `first?.error?.details` — the real envelope nests conflict details under `error` (the same location the FE already reads `error.code`, mirroring the Phase-5 `error.details` precedent). Typed the failed item accordingly. Behaviour otherwise identical (DOUBLE_SHIFT still routes to the replace flow).
- **Files:** `schedule-overlays.tsx` · **Commit:** `66b11f1`

**2. [Rule 3 — Blocking] reset-db did not truncate the E4 tables.**
- **Found:** Task 1 (designing resetDb-per-test isolation).
- **Issue:** `TRUNCATE_TABLES` omitted `schedule_entries` / `approved_leave_days` / `shift_masters`. The seed is `ON CONFLICT DO NOTHING`, so test-created schedule entries survived resetDb and would pollute later specs.
- **Fix:** added the three tables to `TRUNCATE_TABLES` ahead of `placements`/`employees` (FK order). Reseeded by `go run ./cmd/seed`.
- **Files:** `reset-db.ts` · **Commit:** `66b11f1`

**3. [Rule 1 — Bug, test harness] post-goto 401 race.**
- **Found:** Task 2 (first conflict-negatives run — all 11 apiAs calls got 401).
- **Issue:** after a full `page.goto()`, JS module memory resets and the in-memory access token is re-hydrated **asynchronously** by `tryRestoreSession`. An immediate `apiAs` had no Bearer token → 401.
- **Fix:** added `waitForToken(page)` (polls `window.__swp_get_token__`) and call it after each `goto` that precedes an `apiAs`. (Task-1 specs were unaffected because they awaited an authed page element first.)
- **Files:** `e4-helpers.ts` + Task-2 specs · **Commit:** `0db2ea4`

## Deviations from plan

- **CH-1 clears a freshly-created FUTURE cell instead of the seeded Tue cell.** The seeded entries land on monday+1/+2 (past dates on the run day); C-5 forbids a *leader* clearing a past-dated entry (403). Creating an in-window future cell first keeps CH-1 a genuine leader-scoped DELETE that is immune to the current weekday. Functionally equivalent, more robust. (CH-2/SA-2 edit the seeded past cells via PATCH, which has no past-date guard, so they were left as-is.)
- **`status__in` not `status` on list params** (noted from the generated `ListScheduleParams`) — the FE grid never sends a status filter, so no change needed; tests use the required `company_id/start_date/end_date` only.
- No openapi-vs-impl mismatch beyond the FE-side `conflict_details` read; the 06-03 contract tests already proved the BE shapes.

## Final result

`pnpm e2e` (headless, real FE + real Go API + ephemeral Postgres): **145 passed, 0 failed** (`.last-run.json` status `passed`). The 27 new E4 tests are included; **no e1/e2/e3 regressions**. All 4 ROADMAP Phase-6 success criteria are TRUE: (1) HR/leader manage shift masters + schedule entries; (2) conflict check returns correct codes + bulk-apply partial success; (3) schedule lists company-scoped + leader-scoped 403; (4) exhaustive E4 E2E green.

## User Setup Required

None.

## Self-Check: PASSED

All 6 created files present on disk (e4-helpers.ts + 5 e4 spec files); both modified files carry their fix (`first?.error?.details` in schedule-overlays, `schedule_entries` in reset-db TRUNCATE_TABLES); both task commits (`66b11f1`, `0db2ea4`) present in git history. `pnpm e2e` exits 0 — 145 passed, 0 failed, `.last-run.json` status `passed`.

---
*Phase: 06-e4-schedule-shifts*
*Completed: 2026-06-04*
