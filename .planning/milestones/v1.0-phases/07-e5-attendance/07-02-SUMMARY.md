---
phase: 07-e5-attendance
plan: 02
subsystem: backend
tags: [go, chi, sqlc, attendance, corrections, verification, bulk, idempotency, rbac, audit, seed]

# Dependency graph
requires:
  - phase: 07-e5-attendance
    provides: "attendance + attendance_corrections tables, 11 sqlc Querier methods (list/get/forUpdate + verify/reject/apply + correction approve/reject), internal/domain/attendance package"
  - phase: 06-e4-schedule-shifts
    provides: "scheduling slice = bulk partial-success + scope analog (mirrored); server.go SCHEDULING marker; main.go wiring pattern"
  - phase: 05-e3-placement
    provides: "rbac.GuardCompany scope source; placements (SWP-PL-5001..5004) as attendance FK anchors; shift-leader persona Rudi EMP-1108 @ CMP-0021"
provides:
  - "10 FE-used E5 endpoints live against the real BE: GET /attendance, GET /attendance/{id}, POST /attendance/{id}:verify|:reject, POST /attendance:bulk-verify|:bulk-reject, GET /corrections, GET /corrections/{id}, POST /corrections/{id}:approve|:reject"
  - "repository/attendance (sqlc-backed attendance + correction repos + mapping)"
  - "service/attendance (AttendanceService verify/reject + bulk partial-success; CorrectionService approve-applies + reject + exported CheckCorrectionWindow)"
  - "handler/attendance (Handler{att,cor} + 10 hand-written chi handlers + openapi-exact DTOs)"
  - "E5 route group in server.go under RequireRole(super_admin,hr_admin,shift_leader) + idempotency-wrapped actions; main.go wiring; Attendance Deps field"
  - "seed fixtures: 6 attendance rows (SWP-ATT-9001..9006) + 2 corrections (SWP-COR-8001/8002); reset-db.ts TRUNCATE list extended"
affects: [07-03, 07-04, E7-overtime, E8-payroll, E10-reporting]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Bulk partial-success = loop ids → single Verify/Reject per item → apperr.As maps failures to {id,error{code,message}}; handler picks 200 (>=1 succeeded) vs 422 (all failed). Idempotency owned by the router middleware, NOT the service (BulkActionResponse shape, not the scheduling CellResult shape)."
    - "Apply-on-approve in one tx: CorrectionService.Approve loads-for-update → scope/terminal/window guards → attRepo.ApplyCorrectionToAttendance (COALESCE whitelist + CORRECTED flag + last_correction_id) → repo.ApproveCorrection (status→APPLIED) → audit x2 (correction + attendance). Returns {data, attendance}."
    - "OUTSIDE_CORRECTION_WINDOW exposed as the package-level exported func CheckCorrectionWindow(shiftDate, isHR, now) so the 07-03 contract test can drive the 422 directly (HR-exempt; the correction-CREATE endpoint is out of web scope)."
    - "Cross-scope reads (Get attendance/correction) return 404 (hide existence) per openapi; write-path scope returns 403 OUT_OF_SCOPE."

key-files:
  created:
    - backend/internal/repository/attendance/mapping.go
    - backend/internal/repository/attendance/attendance_repo.go
    - backend/internal/repository/attendance/correction_repo.go
    - backend/internal/service/attendance/attendance_service.go
    - backend/internal/service/attendance/correction_service.go
    - backend/internal/service/attendance/correction_port.go
    - backend/internal/service/attendance/cursor.go
    - backend/internal/handler/attendance/handler.go
    - backend/internal/handler/attendance/attendance_handler.go
    - backend/internal/handler/attendance/attendance_dto.go
    - backend/internal/handler/attendance/correction_handler.go
    - backend/internal/handler/attendance/correction_dto.go
  modified:
    - backend/internal/server/server.go
    - backend/cmd/api/main.go
    - backend/cmd/seed/seed.go
    - frontend/e2e/lib/reset-db.ts

key-decisions:
  - "VERIFY_OWN_RECORD = 403 (struct literal &apperr.Error{HTTPStatus:403}); terminal-state verify/reject = 409 CONFLICT with fields.verification_status; terminal correction = 409 CONFLICT with fields.status. Matched openapi over the loosely-worded CONTEXT (plan-checker confirmed; the FE Error envelope reads error.code)."
  - "Leader own-record detection uses the principal's EmployeeID (== record.EmployeeID) only for shift_leader; HR/super never hit VERIFY_OWN_RECORD. Resolved from the JWT `emp` claim (auth.Principal.EmployeeID) — Rudi = EMP-1108."
  - "CorrectionService takes BOTH the correction repo AND the attendance repo so approve applies the proposed change to the target attendance in the SAME tx (ApplyCorrectionToAttendance), then ApproveCorrection flips APPLIED — single InTx, no two-phase."
  - "Bulk envelope writer: 200 when len(succeeded) >= 1 else 422; both carry the same {succeeded,failed} body (openapi BulkActionResponse). Idempotency is the router wrap (r.With(d.Idempotency.Handler)) — the service never re-implements replay."
  - "DTO nullable strategy: required-nullable openapi fields (check_out_at, schedule_id, geofence_out, verified_by, lat_out, ...) are pointers WITHOUT omitempty so they serialize as JSON null; denormalized display names (employee_name/company_name/requester_name) use omitempty (present on list/get JOINs, absent on write re-reads where harmless)."
  - "Correction cursor keys on (created_at DESC, id); attendance on (check_in_at DESC, id). has_more via fetch-limit+1 then trim; opaque base64(JSON) cursor via httpx.EncodeCursor/DecodeCursor (Decode* exported for the handler)."
  - "Seed flags bound as Postgres array-literal strings ('{LATE}', '{LATE,ESCALATED}') to the text[] column — Postgres input parser casts; matches the 07-01 flags text[] column."

patterns-established:
  - "Generic single-object envelope dataResponse[T]{Data T} and the page envelope httpx.PageResponse[T] reused across both attendance + correction handlers."
  - "attendanceCols/correctionCols normalization struct in mapping.go collapses the 6 identical attendance Row shapes (list/get/forUpdate/verify/reject/apply) and 5 correction Row shapes into one mapper each, avoiding per-Row field duplication."

requirements-completed: [ATT-01, ATT-02]

# Metrics
duration: 12min
completed: 2026-06-04
---

# Phase 7 Plan 02: E5 Attendance Verification + Corrections Service/Handler Summary

**The 10 FE-used E5 endpoints now run against the real backend: repository over the 07-01 sqlc, services enforcing leader scope (OUT_OF_SCOPE 403), own-record (VERIFY_OWN_RECORD 403), terminal-state (409 CONFLICT), bulk partial-success ({succeeded,failed} 200/422) idempotent at the router, corrections approve-applies + reject with the OUTSIDE_CORRECTION_WINDOW (422) seam, audit-in-tx + notify stub, byte-for-shape openapi DTOs, routes mounted under RequireRole, main.go wired, and seed planting honest exception + correction fixtures — `make gen` / `go build` / `go vet` / `gofmt` all clean and the full Go test suite green (no regressions).**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-06-04T18:12:05Z
- **Completed:** 2026-06-04T18:24:10Z
- **Tasks:** 3
- **Files modified:** 16 (12 created, 4 modified)

## Accomplishments
- **Repository** (`internal/repository/attendance/`): `AttendanceRepo` + `CorrectionRepo` over the 07-01 Querier; `mapping.go` handles the sqlc quirks (jsonb→[]byte→map, pgtype.Date↔time.Time, int32→int, Wfo casing, flags text[]→[]Flag, geofence stored cols→*GeofenceCheck). Verify/Reject/Approve/Reject return an affected-row count (0 ⇒ terminal-state → service 409). pgx.ErrNoRows → domain.ErrNotFound.
- **Attendance service**: List (leader-scope forced + OUT_OF_SCOPE on cross-company filter), Get (404 cross-scope), Verify/Reject (scope + own-record + terminal guards, audit-in-tx, notify stub), BulkVerify/BulkReject (per-item partial success via the single path + apperr.As). Cursor list with has_more.
- **Correction service**: List/Get (+server-rendered diff[]), Approve (apply-on-approve in one tx → APPLIED, audit x2), Reject (→ REJECTED), exported `CheckCorrectionWindow` for the 07-03 OUTSIDE_CORRECTION_WINDOW test seam.
- **Handlers** (`internal/handler/attendance/`): `Handler{attendance, corrections}` + `NewHandler`; the 6 attendance + 4 correction handlers; openapi-exact DTOs (required-nullable as JSON null, GeofenceCheck object, BulkActionResponse {succeeded,failed}, approve {data,attendance}); bulk writer 200/422 selection.
- **Wiring**: server.go E5 `r.Group` (4 reads + 6 idempotency-wrapped actions under `RequireRole(super_admin,hr_admin,shift_leader)`) after the SCHEDULING marker; main.go constructs repos/svcs/handler and adds `Attendance:` to Deps.
- **Seed**: `seedAttendance` (SWP-ATT-9001 AUTO_APPROVED clean, 9002 PENDING LATE, 9003 PENDING OUTSIDE_GEOFENCE, 9004 PENDING AUTO_CLOSED no-clockout, 9005 CMP-0022 cross-company OUT_OF_SCOPE target, 9006 Rudi-own ESCALATED VERIFY_OWN_RECORD target) + `seedCorrections` (SWP-COR-8001 PENDING CHECK_OUT on 9004 approve target, SWP-COR-8002 PENDING CHECK_IN on 9002 reject target), both idempotent. reset-db.ts TRUNCATE list extended (attendance_corrections + attendance before placements).

## Task Commits

Each task was committed atomically:

1. **Task 1: Repository + attendance verification service** - `a741032` (feat)
2. **Task 2: Correction service + all 10 chi handlers + DTOs** - `80c53e0` (feat)
3. **Task 3: Routes + main.go wiring + seed fixtures** - `be1913d` (feat)

**Plan metadata:** (see final docs commit)

## Files Created/Modified
See `key-files` frontmatter. 12 created (repo×3, service×4, handler×5), 4 modified (server.go, main.go, seed.go, reset-db.ts).

## Decisions Made
See `key-decisions` frontmatter. Headline: VERIFY_OWN_RECORD=403 + terminal=409 CONFLICT (matched openapi over CONTEXT); correction approve applies-in-tx via the attendance repo; bulk 200/422 with router-owned idempotency; required-nullable DTO fields emit `null`; OUTSIDE_CORRECTION_WINDOW exposed as an exported seam for 07-03.

## Deviations from Plan

None - plan executed exactly as written. Two within-plan structural choices worth noting (both anticipated by the plan's interfaces block):
- The correction repository **port** lives in `correction_port.go` (its own file) so the Task-1 repository layer and the Task-2 service layer share one definition without a forward-reference; the cursor helpers live in `cursor.go`. (The plan listed `correction_repo.go`/`correction_service.go` as the homes; splitting the shared port/cursor out keeps each commit independently buildable — behaviour-identical.)
- A package-level `dataResponse[T]` generic envelope + `httpx.PageResponse[T]` are reused instead of bespoke per-endpoint list structs (matches the openapi `{data}` / `{data,next_cursor,has_more}` shapes exactly).

## Issues Encountered
- No ephemeral Postgres was available in this session (only the legacy MySQL container), so the seed was **not executed live**. Mitigation: the seed compiles, and the INSERT column-lists / placeholder counts / Exec-arg counts were statically verified against migrations 00026/00027 (attendance: 27 cols, 24 binds, 24 args; corrections: 13 cols, 11 binds, 11 args — exact). Live seed execution + the 6 fixtures' reachability are exercised by 07-03 (contract tests) and 07-04 (E2E) against the real harness DB.
- IDE/linter sqlc lag is moot here — `make gen` produced **no drift** (07-01 sqlc unchanged; this plan only consumes it).

## Next Phase Readiness
- **07-03 (contract tests):** all error codes are reachable — OUT_OF_SCOPE (403), VERIFY_OWN_RECORD (403, struct literal), terminal CONFLICT (409 with fields), OUTSIDE_CORRECTION_WINDOW (422 via exported `CheckCorrectionWindow`), CORRECTION_ALREADY_PENDING (409, 07-01 partial-unique backstop), bulk 200/422 partial-success, idempotency replay (router middleware). Seed targets: 9005 (cross-company), 9006 (leader-own), 8001/8002 (correction approve/reject).
- **07-04 (E2E):** FE e5-attendance components wire to the real BE (MSW off); reset-db truncates the new tables; selectors derive from the real components. FE conflict-details should read `error.details`/`error.fields` (recurring `conflict_details` bug — fix toward contract if hit).
- **Notify** is stubbed everywhere (`// TODO(Phase-11)`); downstream E7/E10 recompute on correction-approve is a noted TODO (PRD F5.4 C-4).

## Self-Check: PASSED

All 12 created + 4 modified files present on disk; all three task commits (a741032, 80c53e0, be1913d) found in git log. `make gen` clean (no sqlc drift), `go build ./...` + `go vet ./...` exit 0, `gofmt -l` clean for all changed Go files, full `go test ./internal/...` suite green (no regressions).

---
*Phase: 07-e5-attendance*
*Completed: 2026-06-04*
