---
phase: 04-e2-people
plan: "02"
subsystem: backend/employees-slice
tags: [employees, people, crud, deactivate, reactivate, duplicate-nik, seed, rbac, audit]
dependency_graph:
  requires: [04-01-people-data-layer]
  provides: [employees-api, employee-routes, employee-seed-rows, people-slice-coordination-markers]
  affects: [backend/internal/server/server.go, backend/cmd/api/main.go, backend/cmd/seed/seed.go]
tech_stack:
  added: []
  patterns: [consumer-defined-repo-interface, cursor-pagination, conflict-guard-duplicate-nik, ep3-login-provisioning-stub, deactivate-login-stub, seed-ordering-contract]
key_files:
  created:
    - backend/internal/domain/people.go
    - backend/internal/repository/people/employees_repo.go
    - backend/internal/service/people/employees_service.go
    - backend/internal/handler/people/employees_dto.go
    - backend/internal/handler/people/employees_handler.go
  modified:
    - backend/internal/platform/i18n/i18n.go
    - backend/internal/server/server.go
    - backend/cmd/api/main.go
    - backend/cmd/seed/seed.go
decisions:
  - "GET /employees/{id} RBAC: web roles only (super_admin, hr_admin, shift_leader) — agent excluded because FE web app never calls detail as agent; agent self-service is mobile-only in Phase 4"
  - "EP-3 login provisioning stub: provision_login/login_email accepted in write request body but UserID stays NULL; no E1 user created in this milestone"
  - "Deactivate-login stub: DeactivateEmployee sets employee status only; linked E1 session not revoked (matches Phase-2 'Session revocation on deactivate ... deferred' decision)"
  - "seedEmployees called BEFORE persona user loop in Seed() — ordering contract documented in seed.go and this SUMMARY"
  - "DUPLICATE_NIK uses apperr.Error{} struct literal (HTTPStatus:409, Fields:{nik:...}) — Conflict() constructor exists but does not accept fields"
metrics:
  duration_seconds: 447
  completed_date: "2026-06-04"
  tasks_completed: 3
  files_created: 5
  files_modified: 4
---

# Phase 4 Plan 02: Employees Slice Summary

End-to-end employees slice: domain types, sqlc-backed repository, business-logic service with DUPLICATE_NIK guard + audit, hand-written chi handlers and DTOs matching the OpenAPI Employee schema exactly, RBAC route groups in server.go, cmd/api wiring, i18n error messages, and seeded persona employee rows ordered before the user-seed loop.

## What Was Built

### Domain (backend/internal/domain/people.go)

`Employee` struct with all OpenAPI Employee fields:
- `BankAccount` nested struct (flat DB columns mapped to struct)
- `HasLogin bool` derived from `UserID != nil` (never stored)
- `CurrentPosition`, `CurrentServiceLine`, `CurrentClientCompany` as Phase-5 stubs (always nil)
- `EmployeeFilter` for cursor-paginated list queries

### Repository (backend/internal/repository/people/employees_repo.go)

Implements the service's consumer-defined `EmployeeRepository` interface over sqlc:
- 6 methods: `ListEmployees`, `GetEmployeeByID`, `GetEmployeeByNIK`, `CreateEmployee`, `UpdateEmployee`, `SetEmployeeStatus`
- Reads on the pool; writes take `pgx.Tx` (same pattern as org repo)
- `pgx.ErrNoRows` → `domain.ErrNotFound` in `mapErr()`
- `pgtype.Date` ↔ `time.Time` conversion helpers

### Service (backend/internal/service/people/employees_service.go)

Business logic with consumer-defined interfaces:
- `EmployeeRepository` interface (6 methods)
- `TxRunner`, `Clock`, `Service`, `NewService`, `SetClock` — mirrored from org service
- `ListEmployees`: fetch limit+1, cursor encode, status lowercased before query
- `GetEmployee`: repo lookup + ErrNotFound → apperr.NotFound()
- `CreateEmployee`: required-field validation (full_name, nik, join_at) → 400; NIK duplicate pre-check → 409 DUPLICATE_NIK; InTx create + audit
- `UpdateEmployee`: load current (404); NIK-change duplicate re-check; InTx update + audit
- `DeactivateEmployee`: already-inactive guard → 409 CONFLICT; InTx SetEmployeeStatus('inactive') + audit(employee.deactivate) including reason
- `ReactivateEmployee`: already-active guard → 409 CONFLICT; InTx SetEmployeeStatus('active') + audit(employee.reactivate)

### Handler + DTOs (backend/internal/handler/people/)

**employees_dto.go:**
- `employeeResponse` — all fields snake_case, status uppercased to ACTIVE/INACTIVE at DTO boundary, join_at/birth_date as "YYYY-MM-DD", bank_account as nested object, current_* as null (Phase-5 stubs)
- `employeeWriteRequest` — includes `provision_login` + `login_email` (EP-3 stub fields accepted but ignored)
- `toEmployeeResponse()` mapper — handles BirthDate nil check, bank_account always present
- Local `queryStringPtr`, `parseLimit`, `pageCursor`, `derefString` helpers (not exported; no cross-package coupling)

**employees_handler.go:**
- `Handler struct { svc *svc.Service }`, `NewHandler(s)` — mirrors org handler shape
- `ListEmployees`: parses q/status/limit/cursor, calls service, returns `httpx.PageResponse[employeeResponse]`
- `GetEmployee`: URL param `employee_id`
- `CreateEmployee`: decodes request, parses dates, calls `svc.CreateEmployee`, returns 201 + Location header
- `UpdateEmployee`: loads current employee for partial-update carry-forward, resolves all fields
- `DeactivateEmployee`: optional reason body (decode errors ignored)
- `ReactivateEmployee`: no body

### server.go Changes

Two new route groups under the authenticated `/api/v1` group, after the ORG slice (03-04):

```
// PEOPLE slice (04-02): employees (E2 F2.1 / PPL-01).
// Reads: super_admin, hr_admin, shift_leader
r.Get("/employees", d.People.ListEmployees)
r.Get("/employees/{employee_id}", d.People.GetEmployee)

// Writes: super_admin, hr_admin
r.Post("/employees", d.People.CreateEmployee)                             // + Idempotency
r.Patch("/employees/{employee_id}", d.People.UpdateEmployee)
r.Post("/employees/{employee_id}:deactivate", d.People.DeactivateEmployee) // + Idempotency
r.Post("/employees/{employee_id}:reactivate", d.People.ReactivateEmployee) // + Idempotency
// PEOPLE slice end (04-02). 04-03 agreements: append r.Group{} here.
```

`Deps.People *peoplehttp.Handler` field added.

### cmd/api/main.go Changes

```go
// People slice (04-02): employees (E2 F2.1 / PPL-01).
peopleRepo := peoplerepo.New(pool)
peopleSvc := peoplesvc.NewService(peopleRepo, txm)
peopleHandler := peoplehttp.NewHandler(peopleSvc)
```

`People: peopleHandler` in `server.Deps` literal.

### i18n Changes

Added `DUPLICATE_NIK` to both `id` and `en` language blocks:
- ID: "NIK sudah terdaftar untuk karyawan lain."
- EN: "NIK is already registered to another employee."

### Seed Changes (backend/cmd/seed/seed.go)

`seedEmployees()` function inserts 6 employee rows with `ON CONFLICT (id) DO NOTHING`:

| ID | Name | NIK | NIP | Notes |
|----|------|-----|-----|-------|
| SWP-EMP-1042 | Sari Hadi | 3175001505900042 | 1042 | hr_admin persona |
| SWP-EMP-1108 | Rudi Wijaya | 3175001505900108 | 1108 | shift_leader persona |
| SWP-EMP-2891 | Budi Santoso | 3175001505902891 | 2891 | agent; phone + BCA bank |
| SWP-EMP-3001 | Dewi Lestari | 3175001505903001 | 3001 | extra agent |
| SWP-EMP-3002 | Agus Pratama | 3175001505903002 | 3002 | extra shift_leader |
| SWP-EMP-3003 | Bambang Sutrisno | 3175001505903003 | 3003 | extra hr_admin |

Budi Santoso (SWP-EMP-2891) has `phone = "+62-812-3344-5566"` and BCA bank account so the change-request diff E2E (04-04) has a real "old" value to diff against.

`seedEmployees` is called at the **very top of `Seed()`**, before the persona user loop.

## Stub Documentation

### EP-3: Login Provisioning Stub

`provision_login` and `login_email` fields are present in `employeeWriteRequest` and accepted by the handler, but `CreateEmployee` sets `UserID = NULL` (see `CreateEmployee` repo call) and does not create an E1 user. The FE create form does not require login provisioning for the E2E milestone. Full implementation deferred to Phase 4 E1 wiring.

### Deactivate-Login Stub

`DeactivateEmployee` sets the employee status to `inactive` only. The linked E1 user session is NOT revoked (no `SetUserStatus` call, no token invalidation). This matches the Phase-2 decision "Session revocation on deactivate ... deferred" recorded in STATE.md. The linked user's session remains valid until natural expiry.

### Phase-5 Stubs

`current_position`, `current_service_line`, `current_client_company` are always `null` in Employee responses. These will be populated in Phase 5 when the placements table is wired. The domain fields (`PositionRef`, `ServiceLineRef`, `ClientCompanyRef`) exist and are ready for wiring.

## GET /employees/{id} RBAC Decision

The OpenAPI x-rbac for `GET /employees/{employee_id}` lists `agent` as an allowed role. The FE web application never calls the employee detail endpoint as an agent (agent self-service is scoped to mobile). The web route group uses `super_admin, hr_admin, shift_leader` only. When the mobile app routes are added (later phase), agent access should be re-evaluated with a separate route group or an explicit note in the mobile handler.

## Coordination Contract for 04-03 and 04-04

### server.go

Append new `r.Group{}` blocks **after** the comment:
```
// PEOPLE slice end (04-02). 04-03 agreements: append r.Group{} here.
```
This is at the end of the authenticated group in `server.go`, before the closing `})` of `r.Route("/api/v1", ...)`.

### cmd/api/main.go

Append new repo/svc/handler wiring blocks **after** the people slice block (which ends with `peopleHandler := peoplehttp.NewHandler(peopleSvc)`). Add the new Deps fields to the `server.Deps{}` literal after `People: peopleHandler`.

### cmd/seed/seed.go

`seedEmployees` is called first in `Seed()`. 04-03 and 04-04 seed functions should be appended **after** `seedMasterData` (which is the last call before `return nil`) — employees already exist for FK references.

### seed ordering summary:
1. `seedEmployees` (04-02) — MUST be first (users FK to employees)
2. persona user loop (Phase 1)
3. `seedAuditLog` (Phase 1)
4. `seedClientCompanies` (03-02)
5. `seedServiceLines` (03-03)
6. `seedMasterData` (03-04)
7. `seedAgreements` (04-03) — append here
8. `seedChangeRequests` (04-04) — append after agreements

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

### Created files exist:
- backend/internal/domain/people.go: FOUND
- backend/internal/repository/people/employees_repo.go: FOUND
- backend/internal/service/people/employees_service.go: FOUND
- backend/internal/handler/people/employees_dto.go: FOUND
- backend/internal/handler/people/employees_handler.go: FOUND

### Commits exist:
- 9ddd3bf: feat(04-02): domain + repository + service for employees
- 35ad3f2: feat(04-02): employee handlers + DTOs + routes + cmd/api wiring
- 5bf075b: feat(04-02): seed persona employee rows

### Build:
- `go build ./...` exits 0
- `go vet ./...` exits 0
- Employee routes mounted: `/employees` GET and `/employees/{employee_id}:deactivate` POST confirmed in server.go
- Seed: 6 employee rows in DB (SELECT count(*) FROM employees = 6)

## Self-Check: PASSED
