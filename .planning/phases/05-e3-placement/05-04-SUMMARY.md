---
phase: 05-e3-placement
plan: 04
subsystem: testing
tags: [playwright, e2e, full-stack, placement, lifecycle, shift-leader, roster, invariants, INV-1, INV-2, INV-3, INV-4, rbac, react, msw-off]

# Dependency graph
requires:
  - phase: 05-e3-placement
    plan: 02
    provides: 13 FE-used E3 handlers + services + error.details envelope + INV-1..4 enforcement + seeded placements/SLA
  - phase: 05-e3-placement
    plan: 03
    provides: Go contract-test drift gate + coverage map (company-scope E2E targets; site-scope contract-only)
  - phase: 04-e2-people
    provides: Playwright full-stack harness (resetDb, loginAs, personas, window.__swp_get_token__)
provides:
  - 5 E3 Playwright spec files (30 tests) proving the E3 screens drive the real BE green: list/detail/create, lifecycle (renew/transfer/end/resign/terminate), roster, shift-leader assign/replace/end
  - real-409 invariant assertions (INV-1 + details.current_placement, INV-2, INV-4, TERMINAL_STATE_IMMUTABLE, PLACEMENT_PERIOD_OVERLAP, COMPANY_INACTIVE, ALREADY_ENDED, RULE_VIOLATION, OUT_OF_SCOPE)
  - reusable e3-helpers (apiAs token-fetch, Combobox picker driver, error-envelope extractors)
affects: [E4 scheduling E2E, E5 attendance E2E — reuse e3-helpers + placement seed]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "apiAs(page, method, path, body): authenticated browser-context fetch via window.__swp_get_token__ + auto Idempotency-Key — asserts real 409 envelopes the UI can't always surface"
    - "pickCombobox(page, fieldScope, optionText, search): drives the @swp/ui Combobox (button[aria-haspopup=listbox] → search input → option <button>) — the canonical FK-picker interaction for E2E"
    - "comboFieldById(page, htmlFor): xpath //label[@for=id]/.. resolves the FormField wrapper holding a Combobox-backed FK field"
    - "ACTIVE-on-create asserted via a BACKDATED start (UTC-midnight vs Asia/Jakarta boundary derives PENDING_START for a same-day start) — mirrors the 05-03 contract-test decision"

key-files:
  created:
    - frontend/e2e/tests/e3/agent-placement.spec.ts
    - frontend/e2e/tests/e3/placement-lifecycle.spec.ts
    - frontend/e2e/tests/e3/replacement-transfer.spec.ts
    - frontend/e2e/tests/e3/shift-leader-assignment.spec.ts
    - frontend/e2e/tests/e3/company-roster.spec.ts
    - frontend/e2e/lib/e3-helpers.ts
  modified:
    - frontend/packages/api-client/src/errors.ts
    - frontend/apps/web/src/features/e3-placement/company-roster-screen.tsx
    - frontend/apps/web/src/app/router.tsx
    - frontend/e2e/lib/reset-db.ts
    - frontend/e2e/lib/db.ts
    - frontend/e2e/tests/e2/employment-agreements.spec.ts

key-decisions:
  - "INV-3 is structurally unreachable via the company-scope FE path (INV-4 eligibility is checked first, and INV-1 forbids a 2nd placement) — the spec asserts the real reachable 409 (INV_4 precedence); the pure INV_3 envelope is exhaustively contract-tested in 05-03"
  - "Negative invariant conflicts are asserted via apiAs (real 409 envelope) because the UI overlays only surface a generic message; the INV-1 path is also driven through the create-form Banner end-to-end"
  - "Cross-company transfer keeps the Parking service line (the only line with seeded positions) — the company change alone makes it a valid (non-no-op) transfer; service-line-only transfers can't be exercised with the current position seed"

patterns-established:
  - "e3-helpers.apiAs/pickCombobox/comboFieldById — reusable across E4+ E2E for token API calls and FK-picker driving"
  - "Status filters match the PERSISTED lifecycle_status; DTO-derived statuses (EXPIRING) are not filterable server-side — E2E filters use persisted statuses (ENDED) for narrowing assertions"

requirements-completed: [PLC-01, PLC-02, PLC-03, PLC-04]

# Metrics
duration: ~75min
completed: 2026-06-04
---

# Phase 5 Plan 04: E3 Placement Full-Stack E2E Summary

**The E3 placement screens now drive the real Go API off MSW, proven by 30 full-stack Playwright tests (5 specs) that exercise create/list/detail, the full lifecycle state machine (renew/transfer/end/resign/terminate), the company roster, and shift-leader assign/replace/end — including real-409 invariant negatives (INV-1 with details.current_placement, INV-2, INV-4, TERMINAL_STATE_IMMUTABLE, PLACEMENT_PERIOD_OVERLAP, COMPANY_INACTIVE, ALREADY_ENDED, RULE_VIOLATION) and RBAC negatives (agent create 403, shift_leader cross-company roster OUT_OF_SCOPE 403). `pnpm e2e` is green: 116 passed across e1/e2/e3/smoke (all 30 E3 green, 0 regressions) after fixing two real FE bugs and one Phase-5 seed regression surfaced by the run.**

## Performance

- **Duration:** ~75 min
- **Completed:** 2026-06-04
- **Tasks:** 3 (wiring verification + 2 E2E spec tasks) + 3 auto-fixes
- **Files modified/created:** 11

## Accomplishments

- **30 E3 E2E tests** (agent-placement 9, placement-lifecycle 6, replacement-transfer 4, shift-leader-assignment 6, company-roster 5), one `test()` per Gherkin scenario / C-#, all green headless against real FE ↔ real Go API ↔ ephemeral Postgres.
- **Real invariant 409s asserted** (not mocked): INV-1 (+ `details.current_placement`, surfaced through the create-form Banner AND via API), INV-2 (+ replace=true REASSIGNED swap), INV-4, TERMINAL_STATE_IMMUTABLE, PLACEMENT_PERIOD_OVERLAP, COMPANY_INACTIVE, ALREADY_ENDED, RULE_VIOLATION (no-op transfer), and RBAC OUT_OF_SCOPE/agent-403.
- **Transfer/renew atomicity E2E-observable**: predecessor TRANSFERRED/SUPERSEDED + successor ACTIVE, leader auto-vacate (SL-6), both visible via roster + DB assertions.
- **3 real bugs auto-fixed** during the run (see Deviations): a dropped `error.details` on `ApiError`, a roster route-navigation bug, and a Phase-5 seed regression in the e2 agreements suite.
- **Reusable harness**: `e3-helpers.ts` (apiAs token fetch, Combobox picker driver, envelope extractors); reset-db now truncates placement tables; db.ts gains placement/SLA verification helpers.

## Task Commits

1. **Task 1: FE wiring (verification) + ApiError.details fix** — `0641a6d` (fix) — the 6 E3 screens already imported the real `@swp/api-client/e3` hooks (typecheck clean, MSW-off harness default); the only wiring change needed was capturing `error.details` so the INV-1 Banner renders.
2. **Task 1 (cont.): roster route navigation fix** — `fb09744` (fix)
3. **Task 2: agent-placement + lifecycle + transfer specs + harness helpers** — `82bbf8c` (test)
4. **Task 3: shift-leader-assignment + company-roster specs** — `9516f6d` (test)
5. **Regression fix: retarget AG-create e2 tests** — `453f5f0` (fix)

## Files Created/Modified

- `frontend/e2e/tests/e3/*.spec.ts` (5 files) — the E3 E2E suite.
- `frontend/e2e/lib/e3-helpers.ts` — apiAs / pickCombobox / comboFieldById / error extractors.
- `frontend/e2e/lib/reset-db.ts` — truncate placement_history / shift_leader_assignments / placements (FK order).
- `frontend/e2e/lib/db.ts` — getPlacementLifecycleStatus, setCompanyStatus, getActiveLeaderEmployeeForCompany, getShiftLeaderAssignment, getPlacementIdForEmployeeAtCompany.
- `frontend/packages/api-client/src/errors.ts` — ApiError now carries `error.details`.
- `frontend/apps/web/src/features/e3-placement/company-roster-screen.tsx` + `app/router.tsx` — roster filters stay on the roster route; route gains validateSearch.
- `frontend/e2e/tests/e2/employment-agreements.spec.ts` — AG-create targets agreement-less agents.

## Decisions Made

- Negative invariants asserted via the real API envelope (`apiAs`) since the UI overlays surface only a generic message; the INV-1 path is additionally driven through the create-form Banner end-to-end.
- INV-3 is unreachable in company-scope (INV-4 precedence + INV-1) — assert the reachable 409 and lean on 05-03's contract test for the pure INV_3 envelope.
- Cross-company transfer keeps the Parking line (only line with seeded positions); the company change makes it a valid non-no-op transfer.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ApiError dropped `error.details` → INV-1 conflict Banner never rendered**
- **Found during:** Task 2 (AP-inv1-block UI assertion).
- **Issue:** `placement-form.tsx` reads `error.details.current_placement` to render the INV-1 conflict Banner, but `ApiError`/`parseErrorEnvelope` only captured `code/message/fields/request_id` — `details` was discarded, so `extractINVDetails` returned null and the Banner never showed on a real 409 INV_1_VIOLATION.
- **Fix:** Added `details` to `ErrorEnvelope` + `ApiError` (hand-authored file, not generated). Phase 1-4 errors unaffected (details optional/omitempty).
- **Files modified:** frontend/packages/api-client/src/errors.ts.
- **Verification:** AP-inv1-block UI Banner assertion + full E3 suite green.
- **Committed in:** `0641a6d`.

**2. [Rule 1 - Bug] Roster filters navigated to the company DETAIL route, not the roster route**
- **Found during:** Task 3 (RO-filter-status / RO-include-history).
- **Issue:** `company-roster-screen.tsx` `setSearch` + pagination called `navigate({ to: '/client-companies/$clientCompanyId' })` — the company detail route — so every search/status filter, include_history toggle, and page change bounced the user off the roster (no filter row, no table).
- **Fix:** Point all three navigate calls at `/client-companies/$clientCompanyId/roster` and add a `validateSearch` to that route so q/status/include_history/cursor are typed + preserved.
- **Files modified:** company-roster-screen.tsx, app/router.tsx.
- **Verification:** company-roster spec 5/5 green; full E3 suite green.
- **Committed in:** `fb09744`.

**3. [Rule 1 - Bug] Phase-5 seed regression broke two e2 AG-create tests**
- **Found during:** full-suite regression run.
- **Issue:** 05-02's `seedPlacements` gave Rudi (SWP-AG-7003) and Dewi (SWP-AG-7004) active agreements (placements reference them). The e2 AG-create success tests assumed those agents had NO active agreement; EA-2 (one active agreement per employee) now 409s their create, so the success toast never appeared.
- **Fix:** Retargeted AG-create-PKWT → Agus Pratama (SWP-EMP-3002) and AG-create-PKWTT → Bambang Sutrisno (SWP-EMP-3003) — the seeded agreement-less unplaced agents. Rejection/validation AG tests (Rudi, Budi) unaffected.
- **Files modified:** frontend/e2e/tests/e2/employment-agreements.spec.ts.
- **Verification:** employment-agreements spec re-run + full suite green.
- **Committed in:** `453f5f0`.

---

**Total deviations:** 3 auto-fixed (3 bugs). Two were real FE/route bugs the differentiator depended on (INV-1 banner, roster filtering); one was a cross-plan seed regression. **Impact:** all necessary for correctness; no scope creep — the FE-wiring task's intent was exactly to surface and fix such contract/wiring drift against the real BE.

## Issues Encountered

- A transient Docker Desktop daemon outage mid-run aborted one full run (globalSetup `docker compose up` failed); recovered by restarting Docker and re-running. Not a test/code issue.
- First-run Vite cold compile + per-test `resetDb` (TRUNCATE + `go run ./cmd/seed`) make the full suite ~5 min; expected, documented in the harness.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- E3 (Placement) is functionally complete and proven end-to-end; the FE differentiator works against the real BE.
- `e3-helpers.ts` (apiAs, pickCombobox) and the seeded placement/SLA fixtures are ready for E4 (scheduling) E2E, which hangs off the placement record.
- No blockers. Docker availability remains a harness prerequisite (existing concern).

## Self-Check: PASSED

- All 5 E3 spec files + `e3-helpers.ts` + `05-04-SUMMARY.md` present on disk.
- Commits `0641a6d` / `fb09744` / `82bbf8c` / `9516f6d` / `453f5f0` all in git history.
- E3 suite: 30/30 green headless. Full `pnpm e2e`: all green (e1/e2/e3/smoke) after the 3 auto-fixes — the only full-run failures were the 2 e2 AG-create tests, now fixed (employment-agreements re-run 10/10 green).

---
*Phase: 05-e3-placement*
*Completed: 2026-06-04*
