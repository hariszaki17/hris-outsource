---
phase: 04-e2-people
verified: 2026-06-04T00:00:00Z
status: passed
score: 3/3 requirements verified
gaps: []
human_verification:
  - test: "Run full Playwright E2E suite against live stack"
    expected: "26 people tests pass; 88 total suite passes; 0 failures"
    why_human: "E2E suite requires Docker stack (Postgres :5433 + Go API :8081 + Vite :4173). Verified by prior live run per SUMMARY (88 passed / 6 skipped / 0 failed). Cannot re-run without booting containers."
---

# Phase 4: E2 People Verification Report

**Phase Goal:** Employees (CRUD + deactivate/reactivate), employment agreements (CRUD + renew/close + multipart attachment upload), and the change-request approval queue (list/detail/approve/reject) work against the real BE — FE-used endpoints only — with exhaustive Playwright E2E green against the real stack.

**Verified:** 2026-06-04
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | GET/POST/PATCH /employees, :deactivate, :reactivate exist and are RBAC-gated in the real server | VERIFIED | All 6 routes mounted in `server.go` lines 220–233 under correct role guards; handlers are substantive (cursor pagination, NIK guard, audit in tx) |
| 2 | DUPLICATE_NIK (409) fires on duplicate NIK at create/update | VERIFIED | `employees_service.go` lines 174–185 and 225–236 return `apperr.Error{Code:"DUPLICATE_NIK", HTTPStatus:409}` with `nik` field message; unit test `TestCreateEmployee_409_DuplicateNIK` passes |
| 3 | GET/POST /agreements, :renew, :close all mounted and substantive | VERIFIED | Routes mounted at `server.go` lines 246–262; handler methods `ListAgreements`, `GetAgreement`, `CreateAgreement`, `RenewAgreement`, `CloseAgreement` are all fully implemented (non-stub) |
| 4 | Multipart POST /agreements/{id}/attachments (§15 FileRef) works; PKWT_PERIOD_EXCEEDS_MAX (422) and ACTIVE_AGREEMENT_EXISTS (409) enforced | VERIFIED | `UploadAttachment` handler reads multipart, validates MIME + size; `validateAgreementDates` returns `apperr.Rule("PKWT_PERIOD_EXCEEDS_MAX",...)` (HTTP 422); `CreateAgreement` calls `apperr.Conflict("ACTIVE_AGREEMENT_EXISTS")` (HTTP 409) |
| 5 | Authenticated GET /files/{id} returns file bytes; unauthenticated returns 401 | VERIFIED | Route mounted under `d.Authn.Require` group at `server.go` line 250; `DownloadFile` handler returns `Content-Type` + blob; E2E test `AG-download-auth` exercises both paths |
| 6 | GET/GET-detail /change-requests, :approve (applies change in tx), :reject (requires reason) all mounted | VERIFIED | Routes at `server.go` lines 273–278; `ApproveChangeRequest` service calls `UpdateEmployee` + `ResolveChangeRequest` in one `txm.InTx`; `RejectChangeRequest` validates reason len 3–500 |
| 7 | `go build ./...` and `go vet ./...` exit 0 | VERIFIED | Both commands exit 0 (no output = clean) |
| 8 | `go test ./internal/handler/people/...` passes (41 unit tests across 3 files) | VERIFIED | `ok github.com/hariszaki17/hris-outsource/backend/internal/handler/people 0.374s` — 13 employee + 17 agreement + 11 change-request handler tests all pass |
| 9 | SQLC-generated files present and clean (employees, agreements, agreement_attachments, change_requests) | VERIFIED | All 4 generated files present; `git status internal/repository/sqlc/` shows no uncommitted drift; `go build` succeeds confirming interface compatibility |
| 10 | Migrations 00016–00019 present | VERIFIED | `00016_employees.sql`, `00017_employment_agreements.sql`, `00018_agreement_attachments.sql`, `00019_change_requests.sql` all present in `db/migrations/` |
| 11 | E2E spec files exist with one test() per Gherkin scenario | VERIFIED | `employees.spec.ts` (9 tests), `employment-agreements.spec.ts` (10 tests), `change-requests.spec.ts` (7 tests) = 26 total; all DB helper functions (getEmployeeStatus, getEmployeePhone, getAgreementStatus, countAttachmentsForAgreement, getChangeRequestStatus) exist in `e2e/lib/db.ts`; `sample.pdf` fixture present |
| 12 | Seed seeds SWP-EMP-1042/1108/2891, SWP-AG-7001 + attachment SWP-FILE-9001, SWP-CHG-2117/2118 | VERIFIED | `seedEmployees`, `seedAgreements`, `seedChangeRequests` functions present in `cmd/seed/seed.go`; deterministic IDs match E2E test references |

**Score:** 12/12 truths verified

---

### Required Artifacts

| Artifact | Role | Status | Details |
|----------|------|--------|---------|
| `backend/internal/handler/people/employees_handler.go` | PPL-01 HTTP boundary | VERIFIED | 260 lines; List/Get/Create/Update/Deactivate/Reactivate all implemented; not a stub |
| `backend/internal/handler/people/agreements_handler.go` | PPL-02 HTTP boundary | VERIFIED | 323 lines; List/Get/Create/Renew/Close/UploadAttachment/DownloadFile all implemented |
| `backend/internal/handler/people/change_requests_handler.go` | PPL-03 HTTP boundary | VERIFIED | 134 lines; List/Get/Approve/Reject all implemented |
| `backend/internal/service/people/employees_service.go` | PPL-01 business logic | VERIFIED | 336 lines; DUPLICATE_NIK guard, audit in tx, status transitions |
| `backend/internal/service/people/agreements_service.go` | PPL-02 business logic | VERIFIED | 452 lines; PKWT_PERIOD_EXCEEDS_MAX (422), ACTIVE_AGREEMENT_EXISTS (409), renew tx order, close reason enum |
| `backend/internal/service/people/change_requests_service.go` | PPL-03 business logic | VERIFIED | 397 lines; diff builder, approve applies whitelist fields in tx, reject reason validation |
| `backend/internal/server/server.go` | Route mounting | VERIFIED | All 14 people routes mounted with correct RBAC groups (lines 209–279) |
| `backend/cmd/api/main.go` | DI wiring | VERIFIED | `peopleHandler`, `agreementsHandler`, `crHandler` constructed and passed to `Deps` |
| `backend/db/migrations/00016_employees.sql` | Schema | VERIFIED | `CREATE TABLE employees` + `CREATE UNIQUE INDEX employees_nik_uq` |
| `backend/db/migrations/00017_employment_agreements.sql` | Schema | VERIFIED | `CREATE TABLE employment_agreements` + partial unique index `employment_agreements_active_employee_uq` |
| `backend/db/migrations/00018_agreement_attachments.sql` | Schema | VERIFIED | `CREATE TABLE agreement_attachments` |
| `backend/db/migrations/00019_change_requests.sql` | Schema | VERIFIED | `CREATE TABLE change_requests` + status index |
| `backend/internal/repository/sqlc/employees.sql.go` | Generated repo | VERIFIED | 573 lines; List/Get/GetByNIK/Create/Update/SetStatus generated |
| `backend/internal/repository/sqlc/agreements.sql.go` | Generated repo | VERIFIED | 403 lines; List/Get/GetActive/Create/SetStatus generated |
| `backend/internal/repository/sqlc/agreement_attachments.sql.go` | Generated repo | VERIFIED | 105 lines; Create/Get generated |
| `backend/internal/repository/sqlc/change_requests.sql.go` | Generated repo | VERIFIED | 190 lines; List/Get/Create/Resolve generated |
| `backend/cmd/seed/seed.go` | Test data | VERIFIED | seedEmployees/seedAgreements/seedChangeRequests all present with correct IDs |
| `frontend/e2e/tests/e2/employees.spec.ts` | PPL-01 E2E | VERIFIED | 9 tests covering EP-list/create-data-only/create-with-login/reject-dup-NIK/detail/update/deactivate/reactivate/RBAC |
| `frontend/e2e/tests/e2/employment-agreements.spec.ts` | PPL-02 E2E | VERIFIED | 10 tests covering AG-list/create-PKWT/create-PKWTT/PKWT-no-end/PKWT-exceeds-max/only-one-active/renew/close/upload/download-auth |
| `frontend/e2e/tests/e2/change-requests.spec.ts` | PPL-03 E2E | VERIFIED | 7 tests covering CR-queue/detail-diff/approve/reject-needs-reason/reject/already-resolved/RBAC |
| `frontend/e2e/fixtures/sample.pdf` | Upload fixture | VERIFIED | File exists; used by AG-upload-attachment test |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `employees_handler.go` | `employees_service.go` | `svc.Service` injected via `NewHandler(s *svc.Service)` | WIRED | Called on every endpoint; not optional |
| `agreements_handler.go` | `agreements_service.go` | `svc.AgreementService` injected via `NewAgreementHandler` | WIRED | All 7 methods called |
| `change_requests_handler.go` | `change_requests_service.go` | `svc.ChangeRequestService` injected via `NewChangeRequestHandler` | WIRED | All 4 methods called |
| `server.go` | `people` handler package | `d.People`, `d.PeopleAgreements`, `d.PeopleChangeRequests` in Deps | WIRED | All three handler types in `Deps` struct and used in route registration |
| `cmd/api/main.go` | `server.go Deps` | Constructs all 3 handlers and assigns to `Deps` | WIRED | Lines 112–138 in main.go |
| `ApproveChangeRequest` | `UpdateEmployee` (in tx) | `s.repo.UpdateEmployee(ctx, tx, params)` then `s.repo.ResolveChangeRequest` | WIRED | Single `txm.InTx` call; employee mutation happens atomically with CR resolution |
| `UploadAttachment` handler | `AgreementService.UploadAttachment` | multipart parse → `svc.UploadAttachment(ctx, agreementID, params)` | WIRED | Blob, MIME, category, caption all extracted from form and forwarded |
| `DownloadFile` handler | `AgreementService.GetAttachment` | `svc.GetAttachment(ctx, fileID)` → writes `att.Blob` to response | WIRED | Content-Type and Content-Disposition set correctly |
| E2E `employees.spec.ts` | Go API `:8081` | Real HTTP via Playwright `page.goto` / `page.request` | WIRED | DB helpers connect to Postgres `:5433` directly to verify server-side effects |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| PPL-01 | 04-02, 04-05, 04-06 | Employees — list/detail/create/update/deactivate/reactivate | SATISFIED | All 6 employee endpoints mounted + substantive + DUPLICATE_NIK 409 + RBAC; 13 unit tests + 9 E2E tests green |
| PPL-02 | 04-03, 04-05, 04-06 | Employment agreements — list/detail/create/renew/close + attachment upload | SATISFIED | All agreement endpoints including multipart upload + authenticated download; PKWT_PERIOD_EXCEEDS_MAX 422 + ACTIVE_AGREEMENT_EXISTS 409; 17 unit tests + 10 E2E tests green |
| PPL-03 | 04-04, 04-05, 04-06 | Change requests — list/detail/approve/reject (HR approval queue) | SATISFIED | All 4 CR endpoints; approve applies change in tx (whitelist: phone/address/bank_account only); reject validates reason len; 11 unit tests + 7 E2E tests green |

---

### Intentional Skips (not gaps)

| Item | Documented In | Rationale |
|------|--------------|-----------|
| EP-3 login provisioning: `provision_login`/`login_email` accepted but `UserID` stays NULL | 04-02-SUMMARY decisions; comment in `employees_handler.go:87`; E2E test `EP-create-with-login` asserts stub behavior | E1 User creation in CreateEmployee is deferred; E2E test explicitly validates the stub path |
| Session revocation on deactivate: employee status set but linked E1 login not revoked | 04-02-SUMMARY decisions; `employees_service.go:265` comment | Matches Phase-2 "Session revocation on deactivate ... deferred" decision |
| Notification dispatch on CR approve/reject: deferred | `change_requests_service.go:203–205` and `change_requests_service.go:271–272` STUB comments | No notification epic in scope for Phase 4; integration point clearly marked |
| No `internal/service/people/*_test.go` service-layer unit tests | — | Handler tests use real service wired to fake repos, giving equivalent coverage of service business rules. Service logic is not untested — it runs through handler tests which exercise PKWT_PERIOD_EXCEEDS_MAX, ACTIVE_AGREEMENT_EXISTS, DUPLICATE_NIK, approve-applies-to-employee, reject-reason-validation paths all at the service layer. |
| 6 pre-existing E2E skips in full suite | 04-06-SUMMARY | Rate-limit deferred, bulk-delete N/A, company-has-active-placements placeholder — all from prior phases, not Phase 4 |

---

### Anti-Patterns Found

No blockers or warnings found.

- No stub implementations (`return nil`, `return {}`, empty handlers) in any of the 7 people source files.
- No unimplemented TODO/FIXME in handler or service files (deferred items are documented with explicit STUB/DEFERRED comments explaining the rationale and the integration point for when they will be wired).
- No dead-wiring: every handler method calls the real service, every service method queries the real repo.

---

### Build and Test Status

| Check | Command | Result |
|-------|---------|--------|
| Compile | `go build ./...` | Exit 0 — no output |
| Vet | `go vet ./...` | Exit 0 — no output |
| Handler unit tests | `go test -count=1 ./internal/handler/people/...` | `ok ... 0.374s` — 41 tests passed |
| SQLC drift | `git status internal/repository/sqlc/` | No uncommitted changes |
| E2E (prior live run) | `pnpm exec playwright test` | 88 passed / 6 skipped / 0 failed (documented in 04-06-SUMMARY.md) |

---

### Human Verification Required

#### 1. Full E2E Suite Against Live Stack

**Test:** `cd frontend/e2e && pnpm exec playwright test tests/e2/employees.spec.ts tests/e2/employment-agreements.spec.ts tests/e2/change-requests.spec.ts`

**Expected:** 26 tests pass; AG-download-auth verifies unauthenticated → 401 and authenticated → 200 with `content-type: application/pdf`; AG-upload-attachment verifies `countAttachmentsForAgreement` increments by 1 in the DB.

**Why human:** Requires Docker stack (Postgres :5433, Go API :8081, Vite dev server :4173). Cannot be run in this verification session. Verified by prior live run per 04-06-SUMMARY: 26/26 people tests PASSED, 88 total passing, 0 failures.

---

## Gaps Summary

No gaps. All three requirements (PPL-01, PPL-02, PPL-03) are fully satisfied:

- All backend routes are mounted, RBAC-gated, and backed by substantive service and repository implementations.
- All required error codes (DUPLICATE_NIK 409, PKWT_PERIOD_EXCEEDS_MAX 422, ACTIVE_AGREEMENT_EXISTS 409) are enforced at the service layer and covered by unit tests.
- The multipart attachment upload and authenticated file download pipeline is wired end-to-end.
- The change-request approval applies whitelisted employee fields atomically in a transaction.
- Migrations 00016–00019 create the required tables with the correct constraints.
- SQLC-generated files are clean (no drift from `make gen`).
- All three E2E spec files exist with exhaustive per-scenario coverage (26 tests); the sample.pdf fixture and all DB helper functions are present.
- Seed provides all deterministic IDs referenced by E2E tests.

The only outstanding item is re-running the live Playwright suite, which requires Docker — that is classified as human verification, not a gap.

---

_Verified: 2026-06-04_
_Verifier: Claude (gsd-verifier)_
