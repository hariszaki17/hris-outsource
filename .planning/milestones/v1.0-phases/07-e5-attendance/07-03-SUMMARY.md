---
phase: 07-e5-attendance
plan: 03
subsystem: backend
tags: [go, contract-tests, attendance, corrections, chi, httptest, fakes, idempotency, rbac, drift-gate]

# Dependency graph
requires:
  - phase: 07-e5-attendance
    provides: "07-02 attendance + correction services/handlers (the SUT) + the 10 E5 routes + openapi-exact DTOs + exported CheckCorrectionWindow seam + seed fixtures SWP-ATT-9001..9006 / SWP-COR-8001/8002"
  - phase: 06-e4-schedule-shifts
    provides: "scheduling contract-test harness shape (fakeTx no-op Exec, fakeTxRunner, in-memory fake repos over the real svc ports, newHarness + mutable-principal middleware, decodeBody/errObject helpers)"
provides:
  - "internal/handler/attendance contract-test suite (31 tests) = the E5 drift gate replacing server codegen"
  - "attendance_testkit_test.go: fakeTx/fakeTxRunner, fakeAttendanceRepo + fakeCorrectionRepo (real svc ports), in-memory stubIdempotency middleware mirroring the Postgres replay/reuse contract, newHarness(role,company,employee) over the real attendance.Service + correction.Service + handler"
  - "attendance_handler_test.go: list envelope + cursor/has_more, leader-scope, OUT_OF_SCOPE 403, get/cross-scope 404, verify/reject, VERIFY_OWN_RECORD 403, terminal CONFLICT 409, missing-reason 400, bulk partial-success 200/422, idempotency replay + IDEMPOTENCY_KEY_REUSED 409"
  - "correction_handler_test.go: list/scope, get-with-diff, approve→APPLIED (+ CORRECTED), non-PENDING 409, OUTSIDE_CORRECTION_WINDOW 422 (+ HR exempt), reject→REJECTED, missing-reason 400, CORRECTION_ALREADY_PENDING seam"
affects: [07-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "E5 contract tests mirror the Phase-6 scheduling harness EXACTLY: fakeTx (only Exec no-op so audit.Record works inside InTx), fakeTxRunner, in-memory fake repos implementing the REAL svc.AttendanceRepository + svc.CorrectionRepository ports, newHarness mounting the real services + handler on a chi.Router with a mutable-principal closure middleware (swap role/company/employee per case via h.principal)."
    - "Idempotency replay is asserted at the router boundary with an in-memory stubIdempotency middleware (scoped by principal UserID) that reproduces the production contract — same key+body → replay stored status/body (+ Idempotent-Replayed header); same key+different body → 409 IDEMPOTENCY_KEY_REUSED. The real middleware is Postgres-backed (*db.Pool) and cannot be stood up in a fake harness; the Postgres store is exercised by 07-04 E2E."
    - "Fake repos drive the terminal-state 409 honestly: VerifyAttendance/RejectAttendance return zero rows (n=0) when verification_status is not PENDING/ESCALATED; ApproveCorrection/RejectCorrection return n=0 when status != PENDING — exactly the real RETURNING-guard contract from 07-01/07-02."
    - "VERIFY_OWN_RECORD is driven by setting the harness principal's EmployeeID == the seeded record's EmployeeID for a shift_leader (Rudi EMP-1108 owns SWP-ATT-9006 ESCALATED); OUT_OF_SCOPE by seeding the record at CMP-0022 while the leader leads CMP-0021."

key-files:
  created:
    - backend/internal/handler/attendance/attendance_testkit_test.go
    - backend/internal/handler/attendance/attendance_handler_test.go
    - backend/internal/handler/attendance/correction_handler_test.go
  modified: []

key-decisions:
  - "Idempotency replay seam = in-memory stubIdempotency middleware in the testkit, NOT the real Postgres middleware (which needs *db.Pool). It mirrors the exact wire contract (replay same key+body; 409 IDEMPOTENCY_KEY_REUSED on same key+different body) at the same router position as server.go (r.With(idem.Handler) on the 6 action routes). Documented as a seam; the Postgres-backed store is covered by 07-04 E2E."
  - "CORRECTION_ALREADY_PENDING is asserted as a SEAM, not a live service path: the one-pending guard belongs to the correction-CREATE endpoint (mobile/agent-only, OUT of this phase's web scope) backstopped by the 07-01 partial-unique index. The test asserts (a) the fake repo's pending pre-check (countPending) detects two PENDING corrections on one attendance, and (b) the exact 409 + fields.pending_correction_id wire shape via apperr.ConflictWithDetails — matching the openapi already_pending example without inventing a web endpoint."
  - "OUTSIDE_CORRECTION_WINDOW is driven through the REAL CorrectionService.Approve (not the exported CheckCorrectionWindow directly): a shift_leader (non-HR) approving a correction whose attendance_shift_date is 2026-05-01 (> 7 days before the fixed clock 2026-06-04) yields 422 with fields.attendance_date=2026-05-01 + window_days=\"7\"; a parallel HR test asserts the window exemption — so both branches of the real guard are exercised end-to-end."
  - "Fixed clock 2026-06-04T05:00Z (12:00 WIB) set on both services via SetClock — same anchor as the scheduling harness; recent shift date 2026-06-03 sits inside the 7-day window, stale 2026-05-01 outside it (Asia/Jakarta-safe)."
  - "Error-envelope assertions read error.code + error.fields.* (terminal verify→fields.verification_status, terminal correction→fields.status, window→fields.attendance_date/window_days, missing-reason→fields.reason) per the httpx envelope (fields, not details, for these E5 codes) — matching docs/api/E5-attendance/openapi.yaml byte-for-shape."

patterns-established:
  - "doWithHeaders(method,path,body,headers) extends the scheduling do() helper so idempotency-key headers can be threaded; do() delegates to it with nil headers."
  - "seedAttendance(id,company,employee,vstatus,checkIn,flags...) + seedCorrection(id,attID,company,status,shiftDate,type) + seedCheckOutCorrection (proposed check-out + original_snapshot for diff[]) are the E5 fixture helpers, mirroring the scheduling seedMaster/seedPlacement style."

requirements-completed: [ATT-01, ATT-02]

# Metrics
duration: 5min
completed: 2026-06-04
---

# Phase 7 Plan 03: E5 Attendance + Corrections Contract Tests Summary

**The E5 contract is now locked by 31 Go table-driven tests over the REAL attendance + correction services and handlers (chi router + mutable principal, in-memory fake repos): list/cursor envelopes, leader-scope + OUT_OF_SCOPE 403, cross-scope 404, verify/reject 200, VERIFY_OWN_RECORD 403, terminal CONFLICT 409 (fields.verification_status/status), missing-reason 400, bulk partial-success {succeeded,failed} 200/422, idempotency replay + IDEMPOTENCY_KEY_REUSED 409, correction get-with-diff, approve→APPLIED (+ attendance CORRECTED), OUTSIDE_CORRECTION_WINDOW 422 (+ HR exempt), and the CORRECTION_ALREADY_PENDING seam — all asserted byte-for-shape against docs/api/E5-attendance/openapi.yaml. `go test ./... -count=1` exits 0 with no regressions; `go build`/`go vet`/`gofmt -l` clean.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-06-04T18:29:44Z
- **Completed:** 2026-06-04T18:34:42Z
- **Tasks:** 2
- **Files modified:** 3 (3 created)

## Accomplishments
- **Test harness** (`attendance_testkit_test.go`): fakeTx (Exec no-op for audit-in-tx) + fakeTxRunner; `fakeAttendanceRepo` + `fakeCorrectionRepo` implementing the real `svc.AttendanceRepository` / `svc.CorrectionRepository` ports (maps keyed by id; Verify/Reject/Approve/Reject mutate + return updated row, or zero rows on terminal-state to drive 409; ApplyCorrectionToAttendance applies the COALESCE whitelist + appends CORRECTED); an in-memory `stubIdempotency` middleware reproducing the Postgres replay/reuse contract; `newHarness(role, company, employee)` mounting the real `attendance.Service` + `correction.Service` + `attendance.Handler` on a chi.Router with `RequestIDMiddleware` + a mutable-principal closure middleware + the 10 routes (6 actions wrapped with the stub idempotency, exactly like server.go).
- **Attendance contract tests** (`attendance_handler_test.go`, 18 tests): list envelope `{data,next_cursor,has_more}` + has_more/cursor paging round-trip; leader-scope forced + cross-company `OUT_OF_SCOPE` 403; get 200 `{data}` + cross-scope 404; verify/reject 200; `VERIFY_OWN_RECORD` 403 (leader EMP-1108 + own ESCALATED record); verify `OUT_OF_SCOPE` 403 (CMP-0022 record); terminal `CONFLICT` 409 with `fields.verification_status`; missing/short reason 400 `INVALID_REQUEST` (`fields.reason`); terminal reject 409; bulk-verify/bulk-reject partial success (1 succeeded + 1 `VERIFY_OWN_RECORD` failure) 200 and all-failed 422; idempotency replay (same key+body → identical body + `Idempotent-Replayed`) + `IDEMPOTENCY_KEY_REUSED` 409 (same key, different body).
- **Correction contract tests** (`correction_handler_test.go`, 13 tests): list `{data,next_cursor,has_more}` + leader-scope + `OUT_OF_SCOPE`; get-with-diff (asserts a `check_out_at` diff row `{field,before,after}` with after=`2026-06-03T08:10:00Z`); approve→`APPLIED` returning `{data, attendance}` with the attendance `flags` containing `CORRECTED`; non-PENDING approve 409 (`fields.status=APPLIED`); leader stale-shift `OUTSIDE_CORRECTION_WINDOW` 422 (`fields.attendance_date=2026-05-01` + `window_days="7"`); HR window-exempt 200; reject→`REJECTED` (+ `reject_reason`); missing-reason 400; non-PENDING reject 409; and the `CORRECTION_ALREADY_PENDING` seam (two PENDING corrections detected by `countPending` + the 409 + `fields.pending_correction_id` wire shape via `apperr.ConflictWithDetails`).

## Task Commits

Each task was committed atomically:

1. **Task 1: Harness + attendance verification contract tests** - `6eba35e` (test)
2. **Task 2: Correction contract tests** - `10b7768` (test)

**Plan metadata:** (see final docs commit)

## Files Created/Modified
See `key-files` frontmatter. 3 created (testkit + 2 test files), 0 production files modified — this plan is a pure drift gate over the 07-02 SUT.

## Decisions Made
See `key-decisions` frontmatter. Headlines: idempotency replay asserted via an in-memory stub middleware mirroring the Postgres contract at the same router position (real store covered by 07-04 E2E); `CORRECTION_ALREADY_PENDING` asserted as a documented seam (create endpoint out of web scope + 07-01 partial-unique backstop); `OUTSIDE_CORRECTION_WINDOW` driven through the real `CorrectionService.Approve` for both the leader-422 and HR-exempt branches; error assertions read `error.code` + `error.fields.*` per the httpx envelope.

## Deviations from Plan

None - plan executed exactly as written. Two within-plan clarifications worth noting (both anticipated by the plan's `<assertions_required>` / interfaces block):
- The plan's idempotency note offered "wire d.Idempotency.Handler OR assert with a stub store"; the real `idempotency.Middleware` requires a `*db.Pool` so the **stub-store** path was taken (documented seam) — the replay + `IDEMPOTENCY_KEY_REUSED` contract is asserted at the identical router boundary.
- The plan's `CORRECTION_ALREADY_PENDING` note said "assert via the service pre-check"; there is **no live service pre-check** (the one-pending guard is on the out-of-web-scope correction-CREATE endpoint + the 07-01 partial-unique index), so it is asserted as the documented seam (fake `countPending` + the exact 409 wire shape) rather than fabricating a web endpoint. This follows the autonomous directive to fix toward the contract without weakening assertions.

## Issues Encountered
- None. The 07-02 services/handlers matched the openapi contract exactly — every asserted status + code + envelope key was reachable without touching production code (no Rule 1/2/3 fixes needed).

## Next Phase Readiness
- **07-04 (E2E):** the BE contract is now drift-locked. FE wiring (MSW off) can rely on every E5 status/code/shape; any FE-side drift (e.g. reading `conflict_details` instead of `error.fields`/`error.details`) will be caught against the real BE. The Postgres-backed idempotency store + the live seed fixtures (SWP-ATT-9001..9006, SWP-COR-8001/8002) are exercised end-to-end there; reset-db already truncates the new tables (07-02).

## Self-Check: PASSED

- All 3 created files present on disk (attendance_testkit_test.go, attendance_handler_test.go, correction_handler_test.go).
- Both task commits found in git log (6eba35e, 10b7768).
- `go test ./internal/handler/attendance/... -count=1` exits 0 (31 tests pass); full `go test ./... -count=1` green (no regressions); `go build ./...` + `go vet ./...` exit 0; `gofmt -l` clean for all three files.

---
*Phase: 07-e5-attendance*
*Completed: 2026-06-04*
