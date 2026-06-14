---
phase: 04-e2-people
plan: "06"
subsystem: e2e-testing
tags: [playwright, e2e, employees, employment-agreements, change-requests, people, upload, rbac, pkwt]
dependency_graph:
  requires:
    - phase: 04-02-employees-slice
      provides: [employees-api, employee-seed-rows]
    - phase: 04-03-agreements-slice
      provides: [agreements-api, attachments-api, seed-SWP-AG-7001]
    - phase: 04-04-change-requests-slice
      provides: [change-requests-api, seed-SWP-CHG-2117/2118]
  provides:
    - exhaustive-people-e2e-suite (26 tests green)
    - real-upload-e2e-coverage (setInputFiles + countAttachmentsForAgreement)
    - pkwt-422-e2e-coverage
    - change-request-approve-applies-e2e-coverage
  affects: [phase-05-placements, future-phases-reusing-people-e2e-patterns]
tech_stack:
  added: []
  patterns:
    - section-scoped-row-locators (section[aria-label=...] to avoid FilterSelect collision)
    - window-token-helper-for-authenticated-api-calls (window.__swp_get_token__ in E2E mode)
    - json-tags-on-domain-structs-for-diff-serialization (BankAccount)
    - renew-supersede-before-insert (release partial unique index before new active row)
key_files:
  created: []
  modified:
    - frontend/e2e/tests/e2/employees.spec.ts
    - frontend/e2e/tests/e2/employment-agreements.spec.ts
    - frontend/e2e/tests/e2/change-requests.spec.ts
    - frontend/apps/web/src/features/e2-identity/employee-form.tsx
    - frontend/apps/web/src/lib/auth.ts
    - frontend/packages/api-client/src/mutator.ts
    - backend/internal/domain/people.go
    - backend/internal/service/people/agreements_service.go
    - backend/internal/service/people/employees_service.go
key_decisions:
  - "BankAccount json tags: Added json:\"bank_name\"/\"account_number\"/\"account_holder_name\" to domain.BankAccount so that diff serialization uses snake_case keys matching FE formatDiffValue(); and so jsonb unmarshal from DB (where seed stores snake_case) correctly populates struct fields"
  - "RenewAgreement tx order: supersede predecessor BEFORE inserting successor — releases the partial unique index (employment_agreements_active_employee_uq) so the insert doesn't hit a constraint error"
  - "window.__swp_get_token__ E2E helper: exposes in-memory access token on window object only when VITE_ENABLE_MSW=false; allows page.evaluate() to make authenticated fetch() calls without cookie-only strategies that miss the Bearer token"
  - "FormData boundary fix: skip Content-Type override in customFetch when body is FormData — fetch sets multipart/form-data with correct boundary automatically; overriding breaks uploads"
  - "gender z.preprocess in employee-form: coerce '' → undefined before nativeEnum check since FilterSelect emits empty string for placeholder; prevents Zod parse error on submit"
  - "section[aria-label] row scoping for change-requests: the DataTable is wrapped in section[aria-label=Antrian Persetujuan]; scoping div.border-b locators to that section avoids collisions with FilterSelect option rows which also have border-b and contain type label text"
requirements_completed: [PPL-01, PPL-02, PPL-03]
duration: ~90min
completed: "2026-06-04"
---

# Phase 4 Plan 06: People E2E (employees, agreements, change-requests) Summary

**26 Playwright E2E tests covering every Gherkin scenario for E2 people (employees CRUD/deactivate/reactivate/dup-NIK, agreement create/renew/close/PKWT-max/real-upload/download-auth, change-request queue/diff/approve/reject) green headless — 88 total suite passing with 0 regressions.**

## Performance

- **Duration:** ~90 min
- **Started:** 2026-06-04T18:00:00Z (continuation from interrupted prior run)
- **Completed:** 2026-06-04T19:57:00Z
- **Tasks:** 4 tasks (T1-T4 from plan; T1-T3 committed by prior run, T4 + fixes in this run)
- **Files modified:** 9

## Accomplishments

- Fixed 4 BE/FE bugs discovered during E2E green run; all 26 new people tests pass headless
- Full attachment upload E2E: `setInputFiles(sample.pdf)` → POST /agreements/{id}/attachments → attachment name rendered + DB count +1
- PKWT-period-exceeds-5-years 422 and ACTIVE_AGREEMENT_EXISTS 409 covered end-to-end
- Change-request approve-applies: CR approve updates employee phone in same tx (DB assertion passes)
- File download auth gate: unauthenticated → 401; authenticated (via `window.__swp_get_token__`) → 200

## Test Results (Final Run)

| Spec file | Tests | Result |
|-----------|-------|--------|
| employees.spec.ts | 9 | PASSED |
| employment-agreements.spec.ts | 10 | PASSED |
| change-requests.spec.ts | 7 | PASSED |
| **People total** | **26** | **PASSED** |
| Full suite (`pnpm e2e`) | 88 passed + 6 skipped | **GREEN** |

The 6 skipped tests are intentional from prior phases (rate-limit deferred, bulk-delete N/A, company-has-active-placements placeholder).

## Task Commits

1. **Task 1-3 (committed by prior session):**
   - `279c58b` test(04-06): employees.spec.ts — 9 E2E tests
   - `fa54e48` test(04-06): employment-agreements.spec.ts + change-requests.spec.ts

2. **Task 4 (this session):**
   - `d007215` fix(04-06): BE bugs (BankAccount json tags, renew tx order, actorID removal)
   - `00ba0c3` fix(04-06): FE wiring + E2E spec locator fixes

## Files Created/Modified

- `frontend/e2e/tests/e2/employees.spec.ts` — 9 tests: EP-list/create/dup-NIK/detail/update/deactivate/reactivate/RBAC; viewport 1600x900
- `frontend/e2e/tests/e2/employment-agreements.spec.ts` — 10 tests: AG-list/create-PKWT/create-PKWTT/reject-no-end/exceeds-max/only-one-active/renew/close/upload/download-auth; fixed locators
- `frontend/e2e/tests/e2/change-requests.spec.ts` — 7 tests: CR-queue/detail-diff/approve/reject-needs-reason/reject/already-resolved/RBAC; section-scoped row helpers
- `frontend/apps/web/src/features/e2-identity/employee-form.tsx` — gender z.preprocess coerce '' → undefined
- `frontend/apps/web/src/lib/auth.ts` — window.__swp_get_token__ E2E helper (VITE_ENABLE_MSW=false only)
- `frontend/packages/api-client/src/mutator.ts` — skip Content-Type for FormData bodies
- `backend/internal/domain/people.go` — json tags on BankAccount struct
- `backend/internal/service/people/agreements_service.go` — renew tx order fix (supersede first)
- `backend/internal/service/people/employees_service.go` — remove unused actorID param

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] BankAccount json tags missing — diff shows raw JSON with uppercase keys**
- **Found during:** Task 4 (E2E green run, CR-detail-diff test)
- **Issue:** `domain.BankAccount` had no JSON struct tags. When embedded as `any` in the change-request diff response, Go's encoding/json serialized field names as `BankName`/`AccountNumber`/`AccountHolderName` (uppercase). The FE `formatDiffValue()` checks `obj.account_number` (snake_case), so it fell back to `JSON.stringify()` displaying raw JSON. The same uppercase-key mismatch caused jsonb unmarshal from DB (which stores snake_case keys) to fail, leaving new bank values empty.
- **Fix:** Added `json:"bank_name"`, `json:"account_number"`, `json:"account_holder_name"` tags to `domain.BankAccount`
- **Files modified:** `backend/internal/domain/people.go`
- **Committed in:** d007215

**2. [Rule 1 - Bug] RenewAgreement constraint violation — insert before supersede**
- **Found during:** Task 4 (AG-renew test)
- **Issue:** `RenewAgreement` tried to insert a new ACTIVE agreement for the employee BEFORE marking the predecessor as SUPERSEDED. The partial unique index `employment_agreements_active_employee_uq` (WHERE status='active' AND deleted_at IS NULL) blocked the insert because the predecessor was still active.
- **Fix:** Reordered the tx: SetAgreementStatus(predecessor, 'superseded') FIRST, then CreateAgreement (new active), then backfill successor_id on predecessor.
- **Files modified:** `backend/internal/service/people/agreements_service.go`
- **Committed in:** d007215

**3. [Rule 1 - Bug] employees_service.CreateEmployee/UpdateEmployee actorID parameter — latent signature mismatch**
- **Found during:** Task 4 (E2E compile check)
- **Issue:** Service functions had `actorID string` parameter but callers (handlers) didn't pass it; actor is retrieved from context via `auth.PrincipalFrom`. Build was passing only by coincidence; parameter was unused.
- **Fix:** Removed `actorID string` from both function signatures.
- **Files modified:** `backend/internal/service/people/employees_service.go`
- **Committed in:** d007215

**4. [Rule 1 - Bug] FormData upload broken — Content-Type override removed multipart boundary**
- **Found during:** Task 4 (AG-upload-attachment test)
- **Issue:** `customFetch` in mutator.ts unconditionally set `Content-Type: application/json` on any body that was not already set. When the upload mutation sent a FormData body, this overrode the browser-set `multipart/form-data; boundary=...` header, causing the server to fail parsing the multipart request.
- **Fix:** Added `!(options.body instanceof FormData)` guard before the Content-Type assignment.
- **Files modified:** `frontend/packages/api-client/src/mutator.ts`
- **Committed in:** 00ba0c3

**5. [Rule 1 - Bug] employee-form gender field Zod validation error on submit**
- **Found during:** Task 4 (EP-create tests)
- **Issue:** `FilterSelect` emits `''` (empty string) when the placeholder is selected. Zod's `z.nativeEnum(Gender).optional()` does not accept `''` as a valid value, causing a Zod parse error on form submit even when gender is not required.
- **Fix:** Wrapped with `z.preprocess((v) => v === '' ? undefined : v, z.nativeEnum(Gender).optional())`.
- **Files modified:** `frontend/apps/web/src/features/e2-identity/employee-form.tsx`
- **Committed in:** 00ba0c3

**6. [Rule 2 - Missing Critical] E2E auth helper for authenticated API calls from page.evaluate()**
- **Found during:** Task 4 (AG-download-auth test)
- **Issue:** The AG-download-auth test needed to make an authenticated request directly to the API from the browser context. `page.request.get()` uses Playwright's network layer (no in-memory access token). `credentials:'include'` sends the refresh cookie but not the Bearer token (which lives in JS memory via auth.ts). No mechanism existed to access the in-memory token from `page.evaluate()`.
- **Fix:** Added `window.__swp_get_token__` property to expose the access token in E2E mode (`VITE_ENABLE_MSW=false`) only. Test uses `page.evaluate()` to call `fetch()` with the Bearer header.
- **Files modified:** `frontend/apps/web/src/lib/auth.ts`
- **Committed in:** 00ba0c3

---

**Total deviations:** 6 auto-fixed (5 Rule 1 bugs, 1 Rule 2 missing critical)
**Impact on plan:** All auto-fixes necessary for correctness. No scope creep.

## Issues Encountered

**Stale API process on port 8081**: The test harness uses `reuseExistingServer: true` for the Vite dev server (non-CI). A previous interrupted test run left the Go API process running on port 8081 with the OLD binary (before the BankAccount json tags fix). When new test runs started, `globalSetup` tried to start a new `go run ./cmd/api` which failed to bind port 8081 and exited silently while the stale process continued serving. The stale process was identified via `lsof -i :8081`, killed, and subsequent test runs compiled the new binary correctly.

**Root cause mitigation**: The `globalSetup` in `backend.ts` should kill any existing process on port 8081 before starting the new one (defensive). This is a known gap in the harness — logged for future improvement.

## Self-Check

### Commits exist:
- 279c58b: FOUND (test/employees.spec.ts — 9 tests)
- fa54e48: FOUND (test/employment-agreements + change-requests specs)
- d007215: FOUND (fix/BE bugs)
- 00ba0c3: FOUND (fix/FE wiring + spec locators)

### Test results:
- 26/26 people tests PASSED headless
- 88 passed + 6 skipped in full suite (0 failures, 0 regressions)

## Self-Check: PASSED

---

*Phase: 04-e2-people*
*Completed: 2026-06-04*
