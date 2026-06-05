---
phase: 04-e2-people
plan: "04"
subsystem: backend/change-requests-slice
tags: [change-requests, approval-queue, approve-applies-change, reject-with-reason, rbac, audit, seed, people]
dependency_graph:
  requires: [04-03-agreements-slice, 04-02-employees-slice, 04-01-data-layer]
  provides: [change-requests-api, change-request-seed-rows, people-change-requests-coordination-markers]
  affects: [backend/internal/server/server.go, backend/cmd/api/main.go, backend/cmd/seed/seed.go]
tech_stack:
  added: []
  patterns: [approve-applies-change-in-same-tx, whitelisted-fields-only-constraint, jsonb-changes-unmarshal, old-new-diff-at-service-boundary, deferred-notification-stub, cursor-on-submitted-at-id]
key_files:
  created:
    - backend/internal/repository/people/change_requests_repo.go
    - backend/internal/service/people/change_requests_service.go
    - backend/internal/handler/people/change_requests_dto.go
    - backend/internal/handler/people/change_requests_handler.go
  modified:
    - backend/internal/domain/people.go
    - backend/internal/server/server.go
    - backend/cmd/api/main.go
    - backend/cmd/seed/seed.go
decisions:
  - "Approve applies whitelisted fields only (phone/address/bank_account) — statutory fields (NIK/NIP/join_at/gender) are never touched in the approve tx; this is enforced in buildApproveParams by starting from all current emp fields and only overlaying the CR.Changes keys that are non-nil"
  - "Notification dispatch on CR resolution (approve or reject) is DEFERRED — stub comment in ApproveChangeRequest + RejectChangeRequest marks the enqueue point for Phase N (notifications epic)"
  - "Diff (old→new) is computed at the service layer in GetChangeRequestDetail — not stored — avoids snapshot drift; old values are the live employee fields at query time"
  - "All three backend people slices (employees 04-02, agreements 04-03, change-requests 04-04) are now wired in server.go; the coordination markers form a clear append chain for future phases"
  - "crPageCursor uses json keys s/i (SubmittedAt/ID) to match the change_requests index order (submitted_at DESC, id DESC) — different from pageCursor (c/i for created_at/id) used by employees and agreements"
metrics:
  duration_seconds: 329
  completed_date: "2026-06-04"
  tasks_completed: 3
  files_created: 4
  files_modified: 4
---

# Phase 4 Plan 04: Change-Request HR Approval Queue Summary

Complete change-request approval queue: ChangeRequest/ChangeRequestDetail/ChangeRequestChanges domain types, sqlc-backed repo with jsonb unmarshal, service with approve-applies-whitelisted-change-in-same-tx + reject-requires-reason, detail-with-diff view, four handler endpoints under hr_admin/super_admin RBAC, coordination markers in server.go, cmd/api wiring, and two seeded PENDING change-requests for the HR queue.

## What Was Built

### Domain Extension (backend/internal/domain/people.go)

Five new types added:

**ChangeRequestChanges** — the whitelisted `{phone?, address?, bank_account?}` jsonb subset as a Go struct with omitempty JSON tags (so nil fields are not emitted in audit snapshots).

**ChangeRequest** — domain entity for a `change_requests` row; status is DB-lowercase (`pending`/`approved`/`rejected`), uppercased only at the DTO boundary. `Changes` is the deserialized `ChangeRequestChanges`; `RequestType` is stored uppercase per the DB CHECK.

**ChangeRequestDetail** — value object for the GET detail endpoint: embeds `ChangeRequest`, adds an `EmployeeRef` (`{id, full_name, nip}`) and a `Diff map[string]ChangeRequestFieldDiff`. The diff is computed at service time (not stored).

**EmployeeRef** — compact 3-field ref for the detail response employee object.

**ChangeRequestFieldDiff** — `{Old any, New any}` for one changed field; serialises cleanly to JSON for the wire diff map.

**ChangeRequestFilter** — cursor on `(submitted_at, id)` per the change_requests index; status/employee_id/request_type/Q filters.

### Repository (backend/internal/repository/people/change_requests_repo.go)

`ChangeRequestRepo` implements `svc.ChangeRequestRepository`:

- `ListChangeRequests` — maps `ChangeRequestFilter` → `sqlcgen.ListChangeRequestsParams`, unmarshals changes jsonb per row.
- `GetChangeRequestByID` — single CR fetch with `mapErr` for ErrNoRows → domain.ErrNotFound.
- `GetEmployeeByID` — reuses `mapEmployeeFromGetByID` from `employees_repo.go` (same package).
- `UpdateEmployee` — reuses `dateToPgtype`/`nullStr`/`mapEmployeeFromUpdate` helpers from `employees_repo.go` (same package); the approve flow overlays whitelisted fields before calling this.
- `ResolveChangeRequest` — drives `:approve` and `:reject` via `sqlcgen.ResolveChangeRequest`; takes a `ResolveChangeRequestParams` struct.

**jsonb changes unmarshal:** `mapChangeRequest()` calls `json.Unmarshal(row.Changes, &changes)` to populate `domain.ChangeRequestChanges`. The `BankAccount` field in changes jsonb uses snake_case keys `bank_name`/`account_number`/`account_holder_name` matching the `domain.BankAccount` struct tags.

### Service (backend/internal/service/people/change_requests_service.go)

`ChangeRequestService` with consumer-defined `ChangeRequestRepository` interface; reuses `TxRunner` and `Clock` from `employees_service.go` (same package).

**ListChangeRequests:** Lowercases status filter for the DB query; cursor encodes `{s: submitted_at, i: id}` (json keys `s`/`i`).

**GetChangeRequestDetail:** Loads CR (404) + employee (404 guard); calls `buildDiff()` to produce the per-field old→new map. Old = current employee value at query time; new = requested value in `CR.Changes`.

**ApproveChangeRequest:**
1. Loads CR → 404 if missing; 409 CONFLICT if status != "pending".
2. Loads employee to get all current fields.
3. `buildApproveParams()` starts from all current employee fields and overlays only the non-nil keys in `CR.Changes` (phone / address / bank_account). Statutory fields (NIK, NIP, join_at, gender, etc.) are copied verbatim and never modified.
4. InTx: `repo.UpdateEmployee` → `repo.ResolveChangeRequest(status=approved)` → `audit.Record(action=change_request.approve)`.
5. STUB comment marks the notification enqueue point.

**RejectChangeRequest:**
1. Validates reason: trim → len 3..500 → 400 INVALID_REQUEST.
2. Loads CR → 404 / 409 CONFLICT if already resolved.
3. InTx: `repo.ResolveChangeRequest(status=rejected, rejection_reason=reason)` → `audit.Record(action=change_request.reject)`.
4. Employee NOT modified.
5. STUB comment marks the notification enqueue point.

### Handler + DTOs (backend/internal/handler/people/)

**change_requests_dto.go:**
- `changeRequestResponse` — all snake_case; status uppercase; changes `{phone?,address?,bank_account?}`; `request_type`; `submitted_at`/`resolved_at` as RFC3339.
- `changeRequestDetailResponse` — adds `employee{id,full_name,nip}` and `diff{<field>:{old,new}}`.
- `rejectRequest {reason string}` — decoded from JSON body.
- `toChangeRequestResponse`, `toChangeRequestDetailResponse`, `toChangesResp`, `toDiffResp` mappers.

**change_requests_handler.go:**
- `ChangeRequestHandler{svc *svc.ChangeRequestService}` + `NewChangeRequestHandler`.
- `ListPendingChangeRequests` — parses all 4 filters + limit/cursor; returns `PageResponse[changeRequestResponse]`.
- `GetChangeRequest` — URL param `change_request_id`; returns detail with diff.
- `ApproveChangeRequest` — no body; actor from `auth.PrincipalFrom`; returns 200 with CR.
- `RejectChangeRequest` — `json.NewDecoder(r.Body).Decode(&req)`; returns 200 with CR.
- `crHandlerCursor{SubmittedAt time.Time json:"s"; ID string json:"i"}` — matches service's `crPageCursor`.

### server.go Changes

PEOPLE change-requests slice (04-04) added after `// PEOPLE agreements slice end (04-03)`:

```
r.Group(func(r chi.Router) {
    r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
    r.Get("/change-requests", ...)
    r.Get("/change-requests/{change_request_id}", ...)
    r.With(d.Idempotency.Handler).Post("/change-requests/{change_request_id}:approve", ...)
    r.With(d.Idempotency.Handler).Post("/change-requests/{change_request_id}:reject", ...)
})
// PEOPLE change-requests slice end (04-04). Phase 5+ appends after this line.
```

`PeopleChangeRequests *peoplehttp.ChangeRequestHandler` added to Deps.

### cmd/api/main.go Changes

```go
// People change-requests slice (04-04)
crRepo := peoplerepo.NewChangeRequestRepo(pool)
crSvc := peoplesvc.NewChangeRequestService(crRepo, txm)
crHandler := peoplehttp.NewChangeRequestHandler(crSvc)
```

`PeopleChangeRequests: crHandler` in `server.Deps` literal.

### Seed Changes (backend/cmd/seed/seed.go)

`seedChangeRequests()` inserts two fixtures with `ON CONFLICT (id) DO NOTHING`:

| Entity | ID | Description |
|--------|----|-------------|
| change_requests | SWP-CHG-2117 | PENDING MULTIPLE for Budi (phone + bank change); submitted_at 2026-06-03T08:00:00Z |
| change_requests | SWP-CHG-2118 | PENDING ADDRESS for Budi; submitted_at 2026-06-03T09:30:00Z |

The "old" phone `+62-812-3344-5566` and BCA account `1234567890` come from `seedEmployees` (04-02), so the detail diff renders a meaningful old→new transition when the HR user reviews the request.

`seedChangeRequests` called after `seedAgreements` in `Seed()`.

## Key Design Decisions

### Approve-Applies-Change in the Same Tx

`ApproveChangeRequest` calls `repo.UpdateEmployee` and `repo.ResolveChangeRequest` in the same `InTx` closure. This guarantees atomicity: either both succeed (employee updated + CR marked approved) or neither (rollback). The service never calls `ApproveChangeRequest` from outside a tx, and the repo implementations share the `tx pgx.Tx` parameter throughout.

### Whitelisted Fields Only — Statutory Fields Never Touched

`buildApproveParams` starts by copying all current employee fields (NIK, NIP, join_at, gender, address, phone, email, NPWP, BPJS, bank) verbatim from the loaded `domain.Employee`. Then it overlays only the fields present in `CR.Changes`:
- `changes.Phone != nil` → override `p.Phone`
- `changes.Address != nil` → override `p.Address`
- `changes.BankAccount != nil` → override `p.BankName`, `p.BankAccountNumber`, `p.BankAccountHolderName`

NIK/NIP/join_at/gender are never overridden — the change-request schema in the DB enforces this at insert time (only `phone`/`address`/`bank_account` keys are accepted), and the service enforces it at apply time.

### Old→New Diff at Service Boundary

The diff is computed live in `GetChangeRequestDetail` — not stored as a snapshot. This means:
- The "old" value is the employee's current value at query time (which may have changed if a prior CR was already approved).
- This is intentional: the HR user sees what would change relative to the current state, not an outdated snapshot. For audit history, `audit_log` captures the exact before/after at approve time.

### Deferred Notification Stub

Both `ApproveChangeRequest` and `RejectChangeRequest` contain a `// STUB: notification dispatch on CR resolution is DEFERRED.` comment at the `EnqueueTx` integration point. When E11 Notifications is implemented, each method gets a single `jobs.Client.EnqueueTx(tx, ...)` call inserted at that comment.

### All Three People Slices Now Wired

After this plan, `server.go` has the complete E2 people route tree:
1. 04-02: employees (CRUD + deactivate/reactivate)
2. 04-03: agreements (CRUD + renew/close + attachments + file download)
3. 04-04: change-requests (list/detail + approve/reject)

The append-chain of coordination markers makes the sequential slice ordering explicit and avoids merge conflicts.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

### Created files exist:
- backend/internal/repository/people/change_requests_repo.go: FOUND
- backend/internal/service/people/change_requests_service.go: FOUND
- backend/internal/handler/people/change_requests_dto.go: FOUND
- backend/internal/handler/people/change_requests_handler.go: FOUND

### Modified files exist:
- backend/internal/domain/people.go: FOUND
- backend/internal/server/server.go: FOUND
- backend/cmd/api/main.go: FOUND
- backend/cmd/seed/seed.go: FOUND

### Commits exist:
- e3e2752: feat(04-04): change-request domain + repo + service (approve applies, reject)
- 4f7f3e1: feat(04-04): change-request handlers + DTOs + routes + wiring
- 3a0fb7e: feat(04-04): seed pending change-requests SWP-CHG-2117 + SWP-CHG-2118

### Build:
- `go build ./...` exits 0
- `go vet ./...` exits 0
- Change-request routes mounted: GET+GET /change-requests, POST :approve, POST :reject
- server.go has PeopleChangeRequests Deps field
- cmd/api/main.go has PeopleChangeRequests: crHandler
- "PEOPLE change-requests slice end (04-04)" marker present in server.go

## Self-Check: PASSED
