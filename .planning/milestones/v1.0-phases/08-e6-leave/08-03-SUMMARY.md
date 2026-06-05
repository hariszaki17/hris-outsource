---
phase: 08-e6-leave
plan: 03
subsystem: backend
tags: [leave, e6, contract-tests, drift-gate, state-machine, quota, calendar, inv-3]
requires:
  - 08-02 LeaveService/QuotaService/CalendarService + leavehandler.Handler (the real services under test)
  - 08-02 svc.LeaveRepository / svc.QuotaRepository / svc.SchedulePort ports (faked here)
  - platform kernel (httpx RequestID, rbac RequireRole/GuardCompany, apperr, auth principal, audit-in-tx via fakeTx)
  - Phase-7 attendance contract-test harness (07-03) as the EXACT pattern mirror
provides:
  - the E6 drift gate — a contract test pinning every one of the 10 FE-used endpoints' openapi response shape + status
  - leave_testkit_test.go — reusable fakeTx + in-memory fake leave/quota/schedule repos + newHarness(role,company,employee)
  - state-machine + 409/403/422 + bulk-partial-success + calendar-shape + INV-3 side-effect assertions over the REAL service+handler
affects:
  - 08-04 (Playwright E2E) — the live-stack twin of these contract tests; selectors/flows mirror what is pinned here
  - any future E6 handler/service/DTO edit — drift now fails this gate before it reaches the FE
tech-stack:
  added: []
  patterns: [contract-test-drift-gate, fakeTx-audit-in-tx, in-memory-fake-repos-shared-state, mutable-principal-closure-middleware, stub-idempotency-at-router-boundary, INV-3-side-effect-recording-fake, re-readable-response-body-snapshot]
key-files:
  created:
    - backend/internal/handler/leave/leave_testkit_test.go
    - backend/internal/handler/leave/leave_handler_test.go
    - backend/internal/handler/leave/quota_handler_test.go
    - backend/internal/handler/leave/calendar_handler_test.go
  modified: []
decisions:
  - "fakeScheduleRepo RECORDS the INV-3 calls (cancelCalls + insertedDays) and returns a configurable cancelReturns[employee] so the loop-closer firing AND the schedule_impact[] DB→DTO new_status mapping (CANCELLED_BY_LEAVE → LEAVE) are asserted at the service-contract level without Postgres."
  - "decodeBody reads from bytes.NewReader(rr.Body.Bytes()) (a buffer snapshot) so one response can be decoded more than once (errCode + errFields on the same rr) — the attendance harness only ever decoded once, so this is the one deliberate divergence."
  - "fakeQuotaRepo is keyed by id and resolves FindQuotaForEmployeeTypePeriod by scanning (employee,type,period) so the approve-final balance re-check + bulk-grant over-used detection observe the same mutable rows the deduct/upsert wrote."
  - "Over-balance approve-final blocks BEFORE the INV-3 tx (no deduct, no cancel, status unchanged) — asserted; override deducts even into negative remaining + records last_override + still fires INV-3."
metrics:
  duration_min: 6
  tasks: 2
  files: 4
  completed: "2026-06-05"
---

# Phase 8 Plan 03: E6 Leave Go Contract Tests (Drift Gate) Summary

Built the **Go contract-test drift gate** for all 10 FE-used E6 endpoints — the
mechanism that replaces server codegen and pins the wire contract to
`docs/api/E6-leave/openapi.yaml`. The suite drives the **real** LeaveService /
QuotaService / CalendarService + handler over in-memory fakes through an
httptest+chi harness that mirrors `server.go`'s RequireRole + Idempotency
positions, mirroring the Phase-7 attendance harness (07-03) exactly. 30 contract
tests assert the two-level approval state machine, every contract error
code+status, the bulk-grant partial-success envelope, the calendar shape +
show_pending toggle, and the INV-3 loop-closer side-effects (cancel +
approved_leave_days insert + the `CANCELLED_BY_LEAVE → LEAVE` DTO mapping).
`go test ./... -count=1` exits 0 with no e1..e5 regressions.

## What was built

**Task 1 — Testkit harness + leave approval contract tests** (commit `22fb14c`)
- `leave_testkit_test.go`: `fakeTx` (Exec no-op for audit-in-tx) + `fakeTxRunner`;
  `fakeLeaveRepo` (requests/approvals/leaveTypes/calendar maps with mutating
  `UpdateLeaveRequestStatus` so the `*ForUpdate` re-read + list/get observe the
  transition); `fakeQuotaRepo` (keyed by id + resolves by (emp,type,period));
  `fakeScheduleRepo` implementing `svc.SchedulePort` that **records** the INV-3
  `CancelScheduleEntriesForLeave` + `InsertApprovedLeaveDay` calls and returns a
  configurable `schedule_impact[]`; an in-memory `stubIdempotency` at the same
  router position as `server.go`; `newHarness(role, company, employee)` mounting
  the real services+handler on chi with a mutable-principal closure middleware.
- `leave_handler_test.go`: l1 → PENDING_HR + L1/APPROVED timeline; wrong-state
  409 (fields.status); cross-company 403 OUT_OF_SCOPE; self-approve 403 FORBIDDEN;
  final deduct + INV-3 fired (1 cancel call, 3 approved_leave_days inserts,
  schedule_impact new_status `LEAVE`); over-balance 422 BALANCE_RECHECK_FAILED
  (requires_override, no deduct/no state change/no INV-3); override force-approve
  (OVERRIDE_APPROVED timeline, deduct into negative remaining, last_override set,
  INV-3 fired) + short-reason reject; reject happy + terminal 409 + short-reason
  400; list envelope/cursor/leader-scope/cross-company-403; get full-shape +
  cross-scope 404; LA-2 no-leader routing serialization (collapsed HR-first timeline).

**Task 2 — Quota + calendar contract tests** (commit `2bf65ff`)
- `quota_handler_test.go`: list remaining = total−used−pending + pending
  recompute-on-read; adjust happy (total adjusted + `last_adjustment{delta,reason,
  adjusted_by,adjusted_at}`) + refuse total<used → 422 RULE_VIOLATION(fields.delta,
  no change) + missing-reason 400; bulk-grant apply partial success
  (`{preview,total_affected,succeeded[],failed[]}` — mid-year joiner prorated to
  total 7 / prorate_months 7, an over-used employee in `failed[]` with
  RULE_VIOLATION, applied rows carry a written quota_id) + preview no-write
  (null quota_id, zero new rows).
- `calendar_handler_test.go`: LeaveCalendarResponse shape
  (`{period,month,show_pending,entries[],clashes[]}`); show_pending=false →
  APPROVED-only, =true → +PENDING_L1/PENDING_HR; leader cross-company 403 +
  scope forcing; clash detection (≥2 same-service-line agents off the same
  day/company → a `clashes[]` entry with agent_count ≥ 2).

## Contract coverage (the 10 endpoints × what is pinned)

| Endpoint | Status / shape asserted | Error codes asserted |
|----------|-------------------------|----------------------|
| GET /leave-requests | 200 {data,next_cursor,has_more}; leader-scope | OUT_OF_SCOPE 403 |
| GET /leave-requests/{id} | 200 {data} full + timeline (L1+HR) | NOT_FOUND 404 (cross-scope hide) |
| POST :approve-l1 | 200 PENDING_HR + L1/APPROVED | CONFLICT 409, OUT_OF_SCOPE 403, FORBIDDEN 403 |
| POST :approve-final | 200 APPROVED + schedule_impact LEAVE + deduct + INV-3 | BALANCE_RECHECK_FAILED 422 (requires_override) |
| POST :approve-override | 200 OVERRIDE_APPROVED + last_override + INV-3 | INVALID/RULE 400/422 (short reason) |
| POST :reject | 200 REJECTED | CONFLICT 409 (terminal), 400 (no reason) |
| GET /leave-quotas | 200 list; remaining math; pending recompute | — |
| POST :adjust | 200 {data} total + last_adjustment | RULE_VIOLATION 422 (fields.delta), 400 (reason) |
| POST :bulk-grant | 200 {succeeded[],failed[]} partial + preview no-write | RULE_VIOLATION (failed row) |
| GET /leave-calendar | 200 {period,entries,clashes} + show_pending | OUT_OF_SCOPE 403 |

INV-3 side-effects (cancel + approved_leave_days insert + `CANCELLED_BY_LEAVE → LEAVE`)
are asserted at the service-contract level via the recording `fakeScheduleRepo`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Response body decoded only once**
- **Found during:** Task 1 (TestApproveL1_WrongState409, TestApproveFinal_OverBalance422)
- **Issue:** `decodeBody` used `json.NewDecoder(rr.Body)`, which drains the
  `*httptest.ResponseRecorder` body buffer. Tests that assert both the error code
  AND its fields call `errCode` then `errFields`, so the second decode hit EOF.
- **Fix:** `decodeBody` now decodes from `bytes.NewReader(rr.Body.Bytes())` (a
  buffer snapshot), making a response re-decodable. The attendance harness never
  re-read a body, so this is a deliberate (documented) divergence, not a
  regression of that pattern.
- **Files modified:** leave_testkit_test.go
- **Commit:** `22fb14c`

## Verification

- `go test ./internal/handler/leave/... -count=1` exits 0 — 30 contract tests green.
- `go test ./... -count=1` exits 0 — all e1..e5 contract/service suites still green
  (no regressions in identity, foundations, org, people, placement, scheduling,
  attendance, or the 08-02 leave service unit tests).
- `go build ./...` + `go vet ./internal/handler/leave/...` clean; `gofmt -l` clean.
- Grep gates: `newHarness`, `BALANCE_RECHECK_FAILED`, `OUT_OF_SCOPE`,
  `QUOTA_EXCEEDED`/`RULE_VIOLATION`, `show_pending` all present.

## Deferred Issues

**golangci-lint config version mismatch (pre-existing, out of scope).** Same
`.golangci.yml` schema mismatch logged by 08-02 (`make lint` →
`unsupported version of the configuration: ""`) affects every package equally
and is NOT introduced here. Quality gate satisfied via `go build`, `go vet`,
`gofmt -l` (clean), and `go test ./... -count=1` (green). Already tracked in
`.planning/phases/08-e6-leave/deferred-items.md`.

## Self-Check: PASSED

- FOUND: backend/internal/handler/leave/leave_testkit_test.go
- FOUND: backend/internal/handler/leave/leave_handler_test.go
- FOUND: backend/internal/handler/leave/quota_handler_test.go
- FOUND: backend/internal/handler/leave/calendar_handler_test.go
- FOUND: commits 22fb14c (testkit + approval tests), 2bf65ff (quota + calendar tests)
