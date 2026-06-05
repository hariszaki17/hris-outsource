---
phase: 08-e6-leave
plan: 04
subsystem: frontend
tags: [leave, e6, e2e, playwright, inv-3, full-stack, fe-wiring, drift]
requires:
  - 08-02 LeaveService/QuotaService/CalendarService + 10 E6 routes + seedLeave (SWP-LR-8001..8007, SWP-LQ-8001/8002)
  - 08-03 Go contract tests (the drift gate these E2E specs are the live-stack twin of)
  - Phase-7 hardened E2E harness (detached API process-group boot + freePort(8081) + waitForToken)
  - the real e6-leave FE screens (approvals/detail/quotas/calendar/overlays) + @swp/api-client e6 hooks
provides:
  - the E6 full-stack Playwright suite (21 tests, 5 specs) green headless vs real FE↔Go↔ephemeral Postgres
  - the INV-3 loop-closer PROOF: approval inserts the real approved_leave_days row + cancels the schedule entry
  - e6-helpers.ts (LR/LQ/EMP/SCH ids, leaveRow/openLeaveDetail/expectLeaveStatus/quotaRemaining/
    scheduleCheckOverLeave/scheduleEntryStatus/mondayPlus) — the reusable E6 E2E lib
  - reset-db.ts leave-table truncation (FK-ordered) so the INV-3 write-through never leaks between specs
  - FE fix: leave-detail unwraps the BE {data} envelope + opens the override modal off ApiError.code
affects:
  - Phase-6 over-leave conflict engine + Phase-7 Absent-suppression now read a REAL E6-populated approved_leave_days
  - closes LVE-01/LVE-02/LVE-03 — every E6 web surface works against the real backend
tech-stack:
  added: []
  patterns: [full-stack-playwright, apiAs-real-409-probe, div-border-b-row-locator, role-switch-toggle,
    waitForToken-post-goto, real-fixture-driven-INV-assertion, DB-vs-DTO-status-boundary-assertion,
    action-response-only-schedule_impact, envelope-unwrap-toward-contract]
key-files:
  created:
    - frontend/e2e/lib/e6-helpers.ts
    - frontend/e2e/tests/e6/approvals.spec.ts
    - frontend/e2e/tests/e6/quotas.spec.ts
    - frontend/e2e/tests/e6/calendar.spec.ts
    - frontend/e2e/tests/e6/scope-negatives.spec.ts
    - frontend/e2e/tests/e6/inv3-loop-closer.spec.ts
  modified:
    - frontend/apps/web/src/features/e6-leave/leave-detail-screen.tsx
    - frontend/e2e/lib/reset-db.ts
decisions:
  - "FE detail unwraps the BE {data:<LeaveRequest>} envelope: the E6 openapi declares the bare LeaveRequest (so Orval narrows query.data.data to LeaveRequest) but the Go handler wraps in dataResponse like every other epic — the screen now unwraps the extra layer (fallback to bare) so detail/approve/reject render. Fixed toward what the BE returns + how every other detail screen consumes it."
  - "Override modal opens off ApiError.code BALANCE_RECHECK_FAILED, not the message text: the 422 carries error.fields so classifyError returns 'validation' (not 'rule'), and the Bahasa message lacks 'BALANCE' — the original message-substring check never fired. The detail GET also never pre-flags balance_check.requires_override (the BE only re-checks at approve-final), so the error path is the REAL override trigger (proven by the OVERRIDE-happy/BALANCE-override tests clicking plain Setujui → 422 → modal)."
  - "schedule_impact[].new_status='LEAVE' is asserted on the approve-final ACTION RESPONSE, not GET /leave-requests/{id}: LeaveService.Get only re-derives the timeline, NOT schedule_impact (it is built from the cancel RETURNING rows at action time). So the UI never shows a schedule_impact section on a fresh GET; the 'LEAVE' DTO mapping is asserted where the contract actually delivers it (the real Go approve-final response)."
  - "INV-3 pre-condition probes monday+2 and asserts DOUBLE_SHIFT (not 'not-found'): the engine checks SHIFT_OVER_LEAVE (step 5) BEFORE DOUBLE_SHIFT (step 6); pre-approval there is no approved_leave_days row so the live SWP-SCH-6002 entry yields DOUBLE_SHIFT — a clean before/after vs the post-approval SHIFT_OVER_LEAVE."
  - "Queue/quota row assertions anchor on employee_name / employee_id (what the DataTable renders), NOT the LR id (the approvals queue has no id column); detail navigation is by direct /leave/$id goto (the id is the URL)."
  - "Seeded remaining is total−used−PENDING: the soft-reservation pending count makes Dewi remaining 5 (12−4−3) and Budi remaining −3 (12−11−4) — the 08-02 seed comment's '8'/'1' were total−used only; the E2E asserts the real derived remaining."
metrics:
  duration_min: 64
  tasks: 3
  files: 8
  completed: "2026-06-05"
---

# Phase 8 Plan 04: E6 Leave Full-Stack Playwright E2E + INV-3 Loop-Closer Summary

Wired the four E6 leave screens off MSW to the real Go backend and proved the whole
epic with **21 full-stack Playwright tests** (5 specs) running green **headless**
against the real FE ↔ Go API ↔ ephemeral Postgres — including the **INV-3
loop-closer**: approving a leave that overlaps a seeded E4 schedule entry now cancels
that entry (`CANCELLED_BY_LEAVE`) **and** populates the real `approved_leave_days`
table, so the Phase-6 over-leave conflict (`SHIFT_OVER_LEAVE`) fires from the
production leave source instead of the Phase-6 seeded fixture. Full suite: **184
passed / 6 skipped / 0 failed** across e1–e6 — no regressions. Closes LVE-01/02/03.

## What was built

**Task 1 — FE wiring (MSW off) + e6-helpers + reset-db** (commit `d75cc18`)
- `leave-detail-screen.tsx`: two surgical fixes (below) — unwrap the `{data}` envelope
  + open the override modal off `ApiError.code`.
- `e6-helpers.ts`: seeded `LR`/`LQ`/`EMP`/`SCH` id constants (incl. `SWP-LR-8007`),
  `leaveRow`/`expectLeaveRow`/`expectNoLeaveRow`/`openLeaveDetail`/`expectLeaveStatus`/
  `quotaRemaining`/`scheduleCheckOverLeave`/`scheduleEntryStatus`/`mondayPlus`, plus
  re-exports of `apiAs`/`waitForToken`/`errorCode`/`errorDetails` from e5-helpers.
- `reset-db.ts`: `leave_approvals` → `leave_requests` → `leave_quotas` added in FK order
  (before placements/employees); `approved_leave_days` + `schedule_entries` already
  present so the INV-3 write-through resets between specs.

**Task 2 — approvals + quotas + calendar + scope specs** (commit `98dd434`, 20 tests)
- `approvals.spec.ts` (9): L1-forward, HR-final (+L1/HR timeline), L1→final end-to-end,
  reject (+min-length negative), override (+min-length negative, via the 422 error path),
  PENDING_HR list + APPROVED filter, no-leader badge.
- `quotas.spec.ts` (5): list remaining math, adjust happy (+2→7), adjust refuse
  (total<used 422 field error), bulk-grant preview→apply, balance-recheck→override.
- `calendar.spec.ts` (2): empty-default (show_pending OFF) + toggle-ON renders grid + cell.
- `scope-negatives.spec.ts` (4): leader cross-company `:approve-l1` 403 OUT_OF_SCOPE,
  list 403, queue-hidden, HR global 200.

**Task 3 — INV-3 loop-closer (the proof)** (commit `43da715`, 1 test)
- `inv3-loop-closer.spec.ts`: pre-condition (monday+2 schedule create → DOUBLE_SHIFT, no
  leave source yet) → approve `SWP-LR-8007` (real detail screen loads it; approve-final
  action response carries `schedule_impact[].new_status === 'LEAVE'`) → post-condition
  (same create → 409 `SHIFT_OVER_LEAVE`, `details.leave_request_id === 'SWP-LR-8007'`,
  from the REAL approved_leave_days row) → `GET /schedule` shows `SWP-SCH-6002`
  `status === 'CANCELLED_BY_LEAVE'` (the E4 DB value; `LEAVE` lives only at the E6 DTO).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] FE detail screen never rendered (wrong envelope layer)**
- **Found during:** Task 2 (every detail-driven test rendered an empty body).
- **Issue:** `leave-detail-screen` read `query.data?.data` and narrowed it to a
  `LeaveRequest`. The E6 openapi declares the GET 200 body as the **bare** `LeaveRequest`
  (so Orval's mutator wrap makes `query.data.data` = body), but the Go handler returns the
  standard `dataResponse{data:<LeaveRequest>}` envelope (like every other epic). So
  `query.data.data` was `{data:{...}}`, the `'id' in raw` narrow failed, and `lr`
  was `undefined` → `return null` (blank screen).
- **Fix:** Unwrap the BE's extra `data` layer when present, falling back to the bare shape
  (both contracts work). Mirrors how `attendance-detail-screen` reads `query.data.data.data`.
- **Files modified:** leave-detail-screen.tsx
- **Commit:** `d75cc18`

**2. [Rule 1 - Bug] Over-balance override modal never opened**
- **Found during:** Task 1 audit + Task 2 (the override CTA wasn't present and the error
  path didn't trigger the modal).
- **Issue:** `approveFinal.onError` opened the override modal only when
  `kind === 'rule' && message.includes('BALANCE')`. But the BE 422
  `BALANCE_RECHECK_FAILED` carries `error.fields` → `classifyError` returns `'validation'`
  (the `error.fields` branch precedes `isRuleViolation`), and its Bahasa message is
  *"Saldo cuti tidak mencukupi…"* (no "BALANCE"). The proactive
  `balance_check.requires_override` flag is also absent on the detail GET (the BE only
  re-checks at approve-final). So neither path opened the modal.
- **Fix:** Open the modal off `ApiError.code === 'BALANCE_RECHECK_FAILED'` (mirrors the
  Phase-6 error.details precedent). The OVERRIDE-happy / BALANCE-override tests prove it:
  click plain "Setujui" → real 422 → modal opens → override succeeds.
- **Files modified:** leave-detail-screen.tsx
- **Commit:** `d75cc18`

**3. [Rule 3 - Blocking] schedule_impact not on GET — asserted on the action response**
- **Found during:** Task 3 (the detail had no "Dampak Jadwal" section post-approval).
- **Issue:** `LeaveService.Get` re-derives only the timeline; `schedule_impact[]` is built
  from the cancel `RETURNING` rows at approve-final/override time and is **not** persisted
  or recomputed on GET. So the UI never shows a schedule_impact section on a fresh GET.
- **Fix:** Assert `new_status === 'LEAVE'` on the **approve-final action response**
  (`apiAs` POST) — the real Go contract surface that carries it — while the real detail
  screen still loads `SWP-LR-8007` from the live API. Documented as an action-response-only
  surface (not a defect; the contract delivers schedule_impact on the action, not the read).
- **Files modified:** inv3-loop-closer.spec.ts (no FE/BE code change)
- **Commit:** `43da715`

### Adjusted test expectations (not code fixes)
- Queue/quota rows anchor on `employee_name`/`employee_id` (the approvals queue has no LR-id
  column); detail navigation by direct `/leave/$id` goto.
- Seeded `remaining` is `total − used − pending` → Dewi 5 (not 8), Budi −3 (not 1); adjust
  +2 → 7. (The 08-02 seed comment quoted total−used.)
- INV-3 pre-condition asserts DOUBLE_SHIFT (engine order: SHIFT_OVER_LEAVE precedes
  DOUBLE_SHIFT; no leave row pre-approval → the live entry yields DOUBLE_SHIFT).

## Verification

- `frontend/e2e/tests/e6/*` green headless: 21 E6 tests (approvals 9, quotas 5, calendar 2,
  scope 4, inv3 1) pass against the real FE+Go+ephemeral Postgres.
- Full suite `npx playwright test --workers=1`: **184 passed / 6 skipped / 0 failed** —
  no e1/e2/e3/e4/e5 regressions.
- `pnpm --filter @swp/web build` clean; `tsc --noEmit -p e2e/tsconfig.json` clean.
- INV-3 HTTP trace confirms the loop: pre `POST /schedule` 409 (DOUBLE_SHIFT) → approve-final
  200 → post `POST /schedule` 409 (SHIFT_OVER_LEAVE) → `GET /schedule` 200 (CANCELLED_BY_LEAVE).

## Deferred Issues

None new. (The pre-existing `.golangci.yml` schema mismatch from 08-02/08-03 is unrelated to
this FE/E2E plan and already tracked in `deferred-items.md`.)

## Self-Check: PASSED

- FOUND: all 6 created E2E files (e6-helpers + 5 specs) + the 2 modified files (leave-detail-screen, reset-db).
- FOUND: commits d75cc18 (FE wiring + helpers + reset-db), 98dd434 (approvals/quotas/calendar/scope specs), 43da715 (INV-3 loop-closer).
