---
phase: 02-e1-foundations
plan: "03"
subsystem: backend-api
tags: [go, testing, contract-tests, foundations, users, audit-log, platform-settings, rbac, cursor-pagination, httptest, fakeTx]

# Dependency graph
requires:
  - phase: 02-e1-foundations
    plan: "02"
    provides: "foundations handlers, service, repository — what we are testing"
  - phase: 01-test-harness-auth
    provides: "httptest pattern from handler/identity tests"
provides:
  - "backend/internal/handler/foundations/handler_test.go: 15 contract tests"
  - "fakeTx (pgx.Tx stub) + fakeFoundationsRepo + fakeTxRunner pattern for foundations slice"
  - "drift gate: go test ./internal/handler/foundations/... must pass to merge"
affects: [wave-4 E2E, any future handler changes in foundations slice]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "fakeTx: minimal pgx.Tx stub — only Exec is wired (returns success), all others panic; satisfies audit.Record without a DB"
    - "fakeTxRunner passes *fakeTx (not nil) so audit.Record can exec without panicking"
    - "fakeFoundationsRepo: in-memory map-backed repository with cursor filtering matching the SQL ORDER BY (created_at asc, id asc)"
    - "Dynamic principal injection: harness.principal mutable per-test; closure in middleware reads it at request time"
    - "Table-driven RBAC subtests: for _, role := range []auth.Role{agent, shift_leader} { for _, ep := range endpoints }"

key-files:
  created:
    - backend/internal/handler/foundations/handler_test.go
  modified: []

key-decisions:
  - "fakeTx instead of nil pgx.Tx: identity tests pass nil because identity service does NOT call audit.Record inside InTx; foundations service DOES, so nil panics. Solution: implement pgx.Tx interface with only Exec as no-op."
  - "fakeFoundationsRepo cursor filtering: the fake mirrors the SQL 'WHERE (created_at, id) > (cursor_created_at, cursor_id)' semantics so cursor tests are meaningful without a real DB."
  - "Dynamic principal via closure: a single middleware closure reads &fh.principal per request, allowing tests to set h.principal = <role> before calling h.do() without re-building the router."
  - "Principal injection does not require the auth JWT middleware: tests inject the principal directly via auth.WithPrincipal, bypassing token parsing; RBAC middleware reads from context and works normally."

# Metrics
duration: 15min
completed: "2026-06-04"
---

# Phase 02 Plan 03: E1 Foundations Contract Tests Summary

**Go contract tests for all 10 E1 handler routes — cursor envelope, UPPER status, 7 settings keys, RBAC 403 for agent/shift_leader, audit detail before/after/request_id**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-06-04
- **Tasks:** 2
- **Files created:** 1 (handler_test.go)

## Accomplishments

- Created `backend/internal/handler/foundations/handler_test.go` (package `foundations_test`):
  - 15 test functions covering all acceptance criteria
  - `fakeTx`: full `pgx.Tx` interface implementation — only `Exec` is no-op; others panic (safe because tests don't call them)
  - `fakeTxRunner.InTx` passes `*fakeTx` so `audit.Record` inside the service can `Exec` without panicking
  - `fakeFoundationsRepo`: in-memory repository implementing all 11 methods of `foundationssvc.Repository`; cursor filtering matches the SQL ordering semantics
  - Dynamic principal injection via mutable `harness.principal` field + closure middleware — no router rebuild needed per test

### Task 1 tests (users + RBAC)

| Test | What it asserts |
|------|----------------|
| `TestListUsers_ShapeAndEnvelope` | 200 + envelope keys (data/next_cursor/has_more) + all 11 User fields + status=ACTIVE (uppercase) |
| `TestListUsers_CursorAdvances` | limit=2 with 5 users: has_more=true, next_cursor set, page 2 has different ids |
| `TestCreateUser_201` | 201 + Location header + User shape (id/email/role/status/created_at/updated_at) |
| `TestCreateUser_409_EmailTaken` | 409 + error.code=CONFLICT on duplicate email |
| `TestChangeUserRole_422_RoleNotAllowed` | 422 + error.code=ROLE_NOT_ALLOWED for invalid new_role |
| `TestDeactivateReactivate` | deactivate→200 DISABLED; deactivate again→409; reactivate→200 ACTIVE; reactivate again→409 |
| `TestSendPasswordReset_202` | 202 + message field + reset token inserted in fake repo |
| `TestRBAC_NonAdmin_403` | 8 subtests: agent + shift_leader × 4 endpoints → 403 FORBIDDEN |

### Task 2 tests (audit-log + platform settings)

| Test | What it asserts |
|------|----------------|
| `TestListAuditLog_ShapeAndFilters` | 200 + envelope + 9 AuditLogEntrySummary keys + entity_type filter returns only matching rows |
| `TestListAuditLog_CursorAdvances` | cursor paginates audit entries (same pattern as user list) |
| `TestGetAuditLogEntry_200` | before/after/request_id present; all summary keys also present in detail |
| `TestGetAuditLogEntry_404` | error.code=NOT_FOUND for unknown id |
| `TestGetPlatformSettings_200` | all 7 keys (locale/timezone/date_format/currency/version/stack/legacy_data_source); each has value/label/locked; locale.value=id-ID + locked=true |
| `TestAuditLog_RBAC_403` | agent → 403 FORBIDDEN on /audit-log |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] nil pgx.Tx panics in audit.Record**
- **Found during:** Task 1 (first test run)
- **Issue:** The identity test pattern passes `nil` as `pgx.Tx` to `InTx` because the identity service never calls `audit.Record` inside transactions. The foundations service calls `audit.Record` on every write, which calls `tx.Exec(ctx, ...)` — nil dereference panic.
- **Fix:** Implemented `fakeTx` satisfying the full `pgx.Tx` interface. Only `Exec` is wired as a no-op returning success. `fakeTxRunner.InTx` passes `&fakeTx{}` instead of `nil`. `audit.Record` runs without panicking and silently discards the INSERT.
- **Files modified:** `backend/internal/handler/foundations/handler_test.go`
- **Commit:** `4ced476` (fixed in same commit)

## Self-Check

---
## Self-Check: PASSED

Files exist:
- `backend/internal/handler/foundations/handler_test.go` — FOUND

Commits:
- `4ced476` — test(02-03): contract tests for E1 foundations handlers — FOUND

Test gate: `go test ./internal/handler/foundations/... -count=1` — PASSED (15/15)
Regression gate: `go test ./... -count=1` — PASSED (no regressions in identity/auth tests)
Vet gate: `go vet ./internal/handler/foundations/...` — PASSED

Acceptance criteria:
- [x] handler_test.go exists, package foundations_test
- [x] Contains all 8 Task-1 test functions (TestListUsers_ShapeAndEnvelope, TestListUsers_CursorAdvances, TestCreateUser_201, TestCreateUser_409_EmailTaken, TestChangeUserRole_422_RoleNotAllowed, TestDeactivateReactivate, TestSendPasswordReset_202, TestRBAC_NonAdmin_403)
- [x] grep "FORBIDDEN" returns >= 1 (8 assertions)
- [x] grep "next_cursor" returns a match
- [x] Task-1 subset test run exits 0
- [x] Contains all 5 Task-2 functions (TestListAuditLog_ShapeAndFilters, TestListAuditLog_CursorAdvances, TestGetAuditLogEntry_200, TestGetAuditLogEntry_404, TestGetPlatformSettings_200)
- [x] grep "legacy_data_source" returns a match
- [x] Full `go test ./internal/handler/foundations/... -count=1` exits 0
- [x] `go vet ./internal/handler/foundations/...` exits 0
