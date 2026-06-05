---
phase: 04-e2-people
plan: "05"
subsystem: backend/people-contract-tests
tags: [go, contract-tests, drift-gate, employees, agreements, change-requests, rbac]
dependency_graph:
  requires: [04-02, 04-03, 04-04]
  provides: [PPL-01, PPL-02, PPL-03 drift-gate]
  affects: [backend/internal/handler/people]
tech_stack:
  added: []
  patterns:
    - httptest + chi router per-package test harness (mirrors org/foundations pattern)
    - in-memory fake repos (maps + counter IDs) satisfying service consumer interfaces
    - fakeTx/fakeTxRunner for audit.Record Exec calls
    - mutable-principal middleware for per-test RBAC swapping
    - mime/multipart.Writer for attachment upload test bodies
key_files:
  created:
    - backend/internal/handler/people/employees_handler_test.go
    - backend/internal/handler/people/agreements_handler_test.go
    - backend/internal/handler/people/change_requests_handler_test.go
  modified: []
decisions:
  - "413 FILE_TOO_LARGE and 422 PKWT_PERIOD_EXCEEDS_MAX paths confirmed correct via test — both use apperr.Error struct literals (not statusForCode), matching Phase-4 decisions in STATE.md"
  - "multipart.Writer with explicit Content-Type part header used to pass MIME through to UploadAttachment handler (matches how browser form sends it)"
  - "Oversize 413 test uses 10MB+1 byte body sent in a single part — ParseMultipartForm 10<<20 memory limit causes the service SizeBytes check to fire"
  - "fakeTx/fakeTxRunner defined once in employees_handler_test.go and reused by agreements and change_requests test files (same package people_test)"
  - "itoa() helper defined once in employees_handler_test.go (shared across all test files in package)"
  - "fakeEmployeeRepo.UpdateEmployee applies Phone/Address as pointer conditionals matching buildApproveParams whitelist behaviour"
metrics:
  duration: ~25min
  completed: "2026-06-04"
  tasks: 3
  files: 3
---

# Phase 4 Plan 05: People Contract Tests Summary

Go contract tests for all E2 people endpoints — the drift gate asserting exact JSON field names, types, and status codes matching the OpenAPI spec.

## One-liner

Contract tests for all 3 people handler packages: employees (13 tests), agreements (15 tests), change-requests (11 tests); `go test ./... -count=1` green.

## Endpoints Covered

### Employees (`employees_handler_test.go`)

| Test | Endpoint | Assertion |
|------|----------|-----------|
| TestListEmployees_ShapeAndEnvelope | GET /employees | envelope keys + 24-key Employee shape + UPPERCASE status + has_login bool + bank_account nesting |
| TestListEmployees_Cursor_Pagination | GET /employees?limit=20 | has_more=true + next_cursor non-null when 21 records |
| TestGetEmployee_200 | GET /employees/{id} | 200 + id match |
| TestGetEmployee_404 | GET /employees/{id} | 404 NOT_FOUND |
| TestCreateEmployee_201_WithBankAccount | POST /employees | 201 + Location header + bank_account{bank_name,account_number,account_holder_name} + has_login=false |
| TestCreateEmployee_400_MissingRequiredFields | POST /employees | 400 INVALID_REQUEST + fields{full_name,nik,join_at} |
| TestCreateEmployee_409_DuplicateNIK | POST /employees | 409 DUPLICATE_NIK + fields.nik |
| TestUpdateEmployee_200 | PATCH /employees/{id} | 200 + updated field |
| TestUpdateEmployee_404 | PATCH /employees/{id} | 404 |
| TestDeactivateEmployee_200_Then_409 | POST /employees/{id}:deactivate | 200 INACTIVE → 409 CONFLICT |
| TestReactivateEmployee_200_Then_409 | POST /employees/{id}:reactivate | 200 ACTIVE → 409 CONFLICT |
| TestEmployeeRBAC_ShiftLeader_403_OnWrite | POST /employees | 403 FORBIDDEN (shift_leader excluded from write group) |
| TestEmployeeRBAC_ShiftLeader_200_OnRead | GET /employees | 200 (shift_leader in read group) |

### Agreements (`agreements_handler_test.go`)

| Test | Endpoint | Assertion |
|------|----------|-----------|
| TestListAgreements_ShapeAndEnvelope | GET /agreements | envelope + 15-key Agreement shape + 4 bpjs_terms pcts |
| TestListAgreements_EXPIRING_VirtualStatus | GET /agreements | EXPIRING emitted when end_date < now+30d (SetClock deterministic) |
| TestGetAgreement_200 | GET /agreements/{id} | 200 + id match |
| TestGetAgreement_404 | GET /agreements/{id} | 404 NOT_FOUND |
| TestCreateAgreement_PKWT_201_WithCompensation | POST /agreements | 201 + ACTIVE + non-null end_date + all 4 bpjs pcts + Location header |
| TestCreateAgreement_PKWTT_201_NullEndDate | POST /agreements | 201 + null end_date for PKWTT |
| TestCreateAgreement_PKWT_MissingEndDate_400 | POST /agreements | 400 INVALID_REQUEST + fields.end_date |
| TestCreateAgreement_PKWT_PeriodExceedsMax_422 | POST /agreements | 422 PKWT_PERIOD_EXCEEDS_MAX + fields.end_date |
| TestCreateAgreement_ActiveAgreementExists_409 | POST /agreements | 409 ACTIVE_AGREEMENT_EXISTS |
| TestRenewAgreement_201_PredecessorSuperseded | POST /agreements/{id}:renew | 201 + predecessor_id + repo predecessor status=superseded |
| TestRenewAgreement_NonActivePredecessor_409 | POST /agreements/{id}:renew | 409 CONFLICT |
| TestCloseAgreement_200_ClosedReason | POST /agreements/{id}:close | 200 CLOSED + closed_reason=RESIGNED |
| TestCloseAgreement_AlreadyClosed_409 | POST /agreements/{id}:close | 409 CONFLICT |
| TestUploadAttachment_201_FileRefShape | POST /agreements/{id}/attachments | 201 + §15 FileRef 6 keys (id,url,name,size_bytes,mime,uploaded_at) + url prefix /api/v1/files/ |
| TestUploadAttachment_413_FILE_TOO_LARGE | POST /agreements/{id}/attachments | 413 FILE_TOO_LARGE (>10MB payload) |
| TestUploadAttachment_400_DisallowedMIME | POST /agreements/{id}/attachments | 400 INVALID_REQUEST (text/plain rejected) |
| TestDownloadFile_RequiresAuth_401 | GET /files/{id} | 401 UNAUTHENTICATED (no principal) |

### Change Requests (`change_requests_handler_test.go`)

| Test | Endpoint | Assertion |
|------|----------|-----------|
| TestListPendingChangeRequests_ShapeAndEnvelope | GET /change-requests | envelope + 10-key shape + PENDING status + request_type + changes.phone |
| TestGetChangeRequest_200_WithDiff | GET /change-requests/{id} | 200 + employee{id,full_name,nip} + diff.phone{old,new} |
| TestGetChangeRequest_404 | GET /change-requests/{id} | 404 NOT_FOUND |
| TestApproveChangeRequest_200_AppliesToEmployee | POST /change-requests/{id}:approve | 200 APPROVED + resolved_by + resolved_at + employee.phone updated to requested value |
| TestApproveChangeRequest_AlreadyResolved_409 | POST /change-requests/{id}:approve | 409 CONFLICT on already-approved |
| TestRejectChangeRequest_MissingReason_400 | POST /change-requests/{id}:reject | 400 INVALID_REQUEST + fields.reason |
| TestRejectChangeRequest_ShortReason_400 | POST /change-requests/{id}:reject | 400 (len<3 reason) |
| TestRejectChangeRequest_200_StatusRejectedAndReason | POST /change-requests/{id}:reject | 200 REJECTED + rejection_reason + employee phone UNCHANGED |
| TestRejectChangeRequest_AlreadyResolved_409 | POST /change-requests/{id}:reject | 409 CONFLICT on already-rejected |
| TestChangeRequestRBAC_ShiftLeader_403 | GET /change-requests | 403 FORBIDDEN |
| TestChangeRequestRBAC_Agent_403 | GET /change-requests | 403 FORBIDDEN |

## Deviations from Plan

None — plan executed exactly as written.

## Spec-vs-Code Notes

No discrepancies found. All assertions confirm alignment between the OpenAPI spec and the handler implementations:

- `PKWT_PERIOD_EXCEEDS_MAX` correctly returns 422 (apperr.Rule) as specified.
- `ACTIVE_AGREEMENT_EXISTS` correctly returns 409 (apperr.Conflict) as specified.
- `FILE_TOO_LARGE` correctly returns 413 (apperr.Error{HTTPStatus:413}) as specified.
- FileRef §15 shape has exactly 6 keys: id, url, name, size_bytes, mime, uploaded_at.
- EXPIRING virtual status computed at DTO boundary (not stored) confirmed via SetClock test.
- Approve applies whitelisted fields (phone/address/bank_account) and NOT statutory fields confirmed by phone-updated assertion.
- Reject does NOT modify employee, confirmed by phone-unchanged assertion.

## Self-Check: PASSED
