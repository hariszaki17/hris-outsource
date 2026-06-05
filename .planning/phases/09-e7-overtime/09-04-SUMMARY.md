---
phase: 09-e7-overtime
plan: 04
subsystem: e2e
tags: [playwright, e2e, overtime, holidays, fe-wiring, state-machine, rbac, bulk, full-stack]

requires:
  - phase: 09-e7-overtime
    plan: 02
    provides: "OvertimeService/HolidayService + handlers + routes + seed (the real BE the E2E drives); SWP-OT-3000x + SWP-HOL-900x fixtures for every scenario"
  - phase: 09-e7-overtime
    plan: 03
    provides: "the contract-test drift gate that pins the {data}/PageResponse/BulkResult shapes + OT_BELOW_MIN exported seam this E2E mirrors"
  - phase: 08-e6-leave
    plan: 04
    provides: "the Phase-8 E6 full-stack E2E plan (08-04) this mirrors EXACTLY — real FE↔Go↔ephemeral Postgres, one test() per Gherkin AC, e6-helpers shape, the recurring detail-GET {data} unwrap fix"
provides:
  - "frontend/e2e/tests/e7/ — 5 specs / 25 tests green headless: workflow (confirm→L1→final, reject, withdraw, terminal-409), approvals (HR/SL scope, detail render, source filter), bulk (partial success + all-fail 422 + UI), holidays (CRUD + clash + in-use), scope-negatives (OUT_OF_SCOPE + SELF_APPROVAL_FORBIDDEN + below-min)"
  - "frontend/e2e/lib/e7-helpers.ts — OT + HOL fixture-id maps, real-DOM row/holiday locators, persisted-status + bulk-envelope assertion helpers, exact Bahasa button labels"
  - "the recurring detail-GET {data}-envelope fix applied to overtime-detail-screen.tsx (the screen was rendering blank)"
  - "reset-db.ts truncating overtime_approvals + overtime + holidays in FK-safe order"
affects: []

tech-stack:
  added: []
  patterns:
    - "Deep-route auth-restore retry: openRules() retries goto('/overtime/aturan') + re-login when the refresh-token rotation race bounces a fresh direct navigation to /login"
    - "Honest below-min E2E: OT_BELOW_MIN has no web HTTP trigger (create/auto-detect is mobile/system), so the E2E asserts the seeded below-min record's calculation (skipped_too_short + counted 0 + threshold) through the real GET; the 422 wire shape stays owned by the 09-03 contract test"
    - "Unique-duration row anchor: when several PENDING_L1 rows share an employee name, anchor the queue row on the OT's unique counted-minutes label (\"3j 30m\") to hit the exact fixture"

key-files:
  created:
    - frontend/e2e/lib/e7-helpers.ts
    - frontend/e2e/tests/e7/workflow.spec.ts
    - frontend/e2e/tests/e7/approvals.spec.ts
    - frontend/e2e/tests/e7/bulk.spec.ts
    - frontend/e2e/tests/e7/holidays.spec.ts
    - frontend/e2e/tests/e7/scope-negatives.spec.ts
  modified:
    - frontend/apps/web/src/features/e7-overtime/overtime-detail-screen.tsx
    - frontend/e2e/lib/reset-db.ts

key-decisions:
  - "[09-04]: overtime-detail-screen.tsx unwraps the {data:<Overtime>} GET envelope with a bare fallback (the recurring Phase-8 detail-GET fix) — the screen rendered BLANK because raw was {data:{...}} not the bare object; the list + rules screens already double-unwrapped and needed no change"
  - "[09-04]: web confirm + withdraw are agent-self in the UI (out of web scope); the :confirm/:withdraw route guard lets HR/leader staff drive them, so those transitions are exercised via apiAs against the REAL state machine (not the agent-only detail CTA)"
  - "[09-04]: OT_BELOW_MIN is asserted honestly via the seeded SWP-OT-30006 record's calculation (no web endpoint triggers EnforceMinMinutes — the create/auto-detect path is mobile/system; the 422 wire shape is pinned by 09-03)"
  - "[09-04]: bulk partial-success + cross-company/self/terminal failures are driven via apiAs (terminal + cross-company rows are not all selectable through the queue UI), with one additional UI-driven happy bulk-approve via the \"Setujui Massal\" header button"
  - "[09-04]: openRules() retries the deep-route auth-restore race (a fresh goto('/overtime/aturan') can land on /login when refresh-token rotation leaves tryRestoreSession unauthenticated) by re-login + retry"

patterns-established:
  - "Pattern: a deep-authed-route E2E opener that tolerates the refresh-rotation redirect race (retry goto + re-login) — reusable for any spec that lands directly on a nested authed route as its first post-login page"

requirements-completed: [OVT-01, OVT-02]

duration: 51min
completed: 2026-06-05
---

# Phase 9 Plan 04: E7 Overtime Full-Stack E2E Summary

**The e7-overtime screens drive the real Go BE green: confirm→L1→final, reject, withdraw, bulk partial-success (terminal CONFLICT + cross-company OUT_OF_SCOPE) + all-fail 422, holiday CRUD + HOLIDAY_DATE_CLASH/HOLIDAY_IN_USE, OUT_OF_SCOPE + SELF_APPROVAL_FORBIDDEN 403, and the seeded below-min OT_BELOW_MIN effect — all asserted via the REAL component selectors / apiAs against an ephemeral Postgres, headless. 5 specs / 25 tests; the full e1–e7 suite is 209 passed / 6 skipped / 0 failed. Closes OVT-01 / OVT-02 and Phase 9.**

## Performance

- **Duration:** ~51 min
- **Completed:** 2026-06-05
- **Tasks:** 3
- **Files modified:** 6 created + 2 modified

## Accomplishments

- **e7-helpers.ts:** OT (`SWP-OT-3000x`) + HOL (`SWP-HOL-900x`) seeded fixture-id maps, employee/company/name maps, `mondayPlus`/`inUseHolidayDate` TZ helpers (mirroring the seed's UTC-Monday math), real-DOM locators (`otRow` via `div.border-b`, `holidayRow` via `li`), persisted-state probes (`overtimeStatus`/`expectOvertimeStatus`/`overtimeApprovals`, `getHoliday`), the `bulk`/`bulkFailedCode` envelope helpers, and exact Bahasa button labels (`OT_BTN`). Re-exports `apiAs`/`errorCode`/`errorDetails`/`waitForToken` from e5-helpers.
- **reset-db.ts:** added `overtime_approvals` + `overtime` + `holidays` to `TRUNCATE_TABLES` in FK-safe order (before placements/employees; overtime before holidays since `overtime.holiday_id` FKs `holidays`).
- **workflow.spec.ts (7 tests):** `:confirm` 30001→PENDING_L1; leader L1 via the queue "Setujui" → PENDING_HR + L1 entry (anchored on the unique "3j 30m" duration); HR final → APPROVED + L2; L1→final chain end-to-end; HR reject via the detail RejectOvertimeModal (+ disabled-confirm on a short reason); withdraw 200/204 → WITHDRAWN + terminal-409; terminal-409 on an already-APPROVED approve-final.
- **approvals.spec.ts (4 tests):** HR PENDING_HR queue (one row); leader own-company PENDING_L1 queue (cross-company hidden); the detail screen renders the tier-breakdown card + approvals timeline from the real `{data}` envelope (SWP-OT-30009 holiday OT w/ L1+HR trail); source filter narrows the queue.
- **bulk.spec.ts (4 tests):** HR bulk-approve `[30003, 30007]` → succeeded `[30003]` + failed `[30007 CONFLICT]` (200); leader bulk-reject `[30002, 30005]` → succeeded `[30002]` + failed `[30005 OUT_OF_SCOPE]`; all-terminal → 422; UI-driven "Setujui Massal" happy path.
- **holidays.spec.ts (5 tests):** create via HolidayFormModal → toast + list; clash 409 `HOLIDAY_DATE_CLASH`; update 200; delete-free 204; delete-in-use → disabled confirm + in-use Banner + 409 `HOLIDAY_IN_USE`.
- **scope-negatives.spec.ts (5 tests):** leader `:approve-l1` cross-company → 403 `OUT_OF_SCOPE`; own OT → 403 `SELF_APPROVAL_FORBIDDEN`; queue-hidden cross-company; leader list `?company_id=CMP-0022` → 403; the seeded below-min record surfaces the OT_BELOW_MIN effect (skipped_too_short + counted 0 + threshold>0).
- **FE fix:** `overtime-detail-screen.tsx` now unwraps the `{data:<Overtime>}` detail-GET envelope with a bare fallback — the screen rendered blank because `query.data?.data` was `{data:{...}}` not the bare object.

## Task Commits

1. **Task 1: e7-helpers + reset-db TRUNCATE** - `06cfe60` (test)
2. **Task 2: workflow + approvals + bulk specs + detail {data} fix** - `061fb29` (test)
3. **Task 3: holiday + scope/self/below-min specs** - `cd1ed5c` (test)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] overtime-detail-screen.tsx rendered blank (detail {data} envelope not unwrapped)**
- **Found during:** Task 2 (the DETAIL approvals test + WF-reject both failed — the detail route rendered only the app shell).
- **Issue:** `useGetOvertime` returns the HTTP body `{data:<Overtime>}`; the screen read `query.data?.data` (= the `{data}` wrapper) and its `'id' in raw && 'status' in raw` guard then set `ot = undefined` → `if (!ot) return null` → blank page. The GET itself returned 200 with correct data.
- **Fix:** unwrap with a bare fallback (the recurring Phase-8 detail-GET pattern) — prefer the nested `.data` when present, else treat the body as the bare `Overtime`.
- **Files modified:** `frontend/apps/web/src/features/e7-overtime/overtime-detail-screen.tsx`.
- **Commit:** `061fb29`.

**2. [Rule 3 - Blocking] deep-route auth-restore redirect race (holidays specs bounced to /login)**
- **Found during:** Task 3 (all 5 holidays tests timed out — the screenshot showed the login page).
- **Issue:** a fresh `page.goto('/overtime/aturan')` immediately after login intermittently lands on `/login`: the refresh-token rotation race leaves `tryRestoreSession` unauthenticated (the cookie was consumed by a prior refresh) so `authedRoute.beforeLoad` redirects.
- **Fix:** `openRules()` retries the goto and re-logs-in if bounced (up to 3 attempts) before asserting the rules header + token.
- **Files modified:** `frontend/e2e/tests/e7/holidays.spec.ts`.
- **Commit:** `cd1ed5c`.

**3. [Rule 1 - Bug] WF-reject mis-asserted the detail reject-modal min-length block**
- **Found during:** Task 2.
- **Issue:** the detail RejectOvertimeModal's confirm button is `disabled` while `reason.length < 5` (it never fires `handleConfirm`, so no min-length toast appears — unlike the queue modal).
- **Fix:** assert the confirm button is `toBeDisabled()` on a short reason instead of expecting an error toast.
- **Files modified:** `frontend/e2e/tests/e7/workflow.spec.ts`.
- **Commit:** `061fb29`.

Otherwise the plan executed as written. No deviation was needed on the approvals/records/rules screens — they already double-unwrap the list `{data}` envelope.

## Issues Encountered

None blocking. Docker + ephemeral Postgres were available; no fake-green. OT_BELOW_MIN has no web HTTP trigger (create/auto-detect is mobile/system, out of web scope) — asserted honestly via the seeded below-min record's calculation, with the 422 wire shape owned by the 09-03 contract test.

## User Setup Required

None.

---

## Reference (handoff)

- E2E entry: `cd frontend && pnpm --filter @swp/e2e exec playwright test tests/e7 --reporter=line` → 25 green; full suite → **209 passed / 6 skipped / 0 failed**.
- The 13 FE-used E7 endpoints are all exercised (list/detail/confirm/approve-l1/approve-final/reject/withdraw/bulk-approve/bulk-reject + holidays list/create/update/delete); `useListOvertimeRules` is reused from E2 (the rules pane renders, not re-implemented); no out-of-scope endpoint (no `:auto-detect`, no OT create).
- Fixture anchors: SWP-OT-30002 = "3j 30m" (unique queue-row label), SWP-HOL-9001 date = `mondayPlus(-14)` (clash target), `inUseHolidayDate()` returns it.

## Self-Check: PASSED

All 6 created files + 2 modified present; the 3 task commits (`06cfe60`, `061fb29`, `cd1ed5c`) exist in git. The e7 suite is 25 green; the full e1–e7 headless run is 209 passed / 6 skipped / 0 failed (no regressions).

---
*Phase: 09-e7-overtime*
*Completed: 2026-06-05*
