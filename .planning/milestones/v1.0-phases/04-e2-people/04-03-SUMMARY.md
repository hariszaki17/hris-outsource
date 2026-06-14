---
phase: 04-e2-people
plan: "03"
subsystem: backend/agreements-slice
tags: [agreements, attachments, file-download, pkwt, pkwtt, multipart, rbac, audit, seed]
dependency_graph:
  requires: [04-02-employees-slice]
  provides: [agreements-api, attachments-api, file-download-route, agreement-seed-rows, people-agreements-coordination-markers]
  affects: [backend/internal/server/server.go, backend/cmd/api/main.go, backend/cmd/seed/seed.go]
tech_stack:
  added: []
  patterns: [pkwt-5yr-cross-field-validation, expiring-virtual-status-dto-boundary, bytea-blob-download, 413-struct-literal, multipart-memory-read, agreement-renew-supersede-chain]
key_files:
  created:
    - backend/internal/repository/people/agreements_repo.go
    - backend/internal/service/people/agreements_service.go
    - backend/internal/handler/people/agreements_dto.go
    - backend/internal/handler/people/agreements_handler.go
  modified:
    - backend/internal/domain/people.go
    - backend/internal/platform/i18n/i18n.go
    - backend/internal/server/server.go
    - backend/cmd/api/main.go
    - backend/cmd/seed/seed.go
decisions:
  - "Bytea blob storage confirmed for attachments: AgreementRepo.CreateAttachment passes blob []byte to sqlc CreateAttachment; GetAttachmentByID returns blob for DownloadFile handler"
  - "EXPIRING virtual status computed at DTO boundary (toAgreementResponse): status='active' AND type='PKWT' AND end_date < now+30d → emit 'EXPIRING'; persisted DB status stays 'active'"
  - "Compensation fields stored plaintext this milestone: base_salary_idr, bpjs_terms jsonb, tax_profile, comp_effective_date; encryption at rest deferred (EA-4)"
  - "FILE_TOO_LARGE uses apperr.Error{Code:'FILE_TOO_LARGE', HTTPStatus:413} struct literal — bypasses statusForCode which has no 413 mapping (same technique as GEOFENCE_RADIUS_INVALID in Phase 3)"
  - "ACTIVE_AGREEMENT_EXISTS uses apperr.Conflict() constructor (HTTPStatus:409, NOT apperr.Rule) — Conflict() forces 409; apperr.Rule() would default to 422"
  - "Multipart upload (UploadAttachment): r.ParseMultipartForm(10<<20), io.ReadAll into memory blob, MIME from part Content-Type header (fallback http.DetectContentType); no idempotency on upload route (binary body)"
  - "AgreementRepo reuses employees_repo.go mapEmployeeFromGetByID via same package; GetEmployeeByID added to AgreementRepository interface for pre-create employee existence check"
  - "PKWTT end_date: if sent, service returns apperr.Invalid (400) — spec requires PKWTT to have no end_date"
metrics:
  duration_seconds: 385
  completed_date: "2026-06-04"
  tasks_completed: 3
  files_created: 4
  files_modified: 5
---

# Phase 4 Plan 03: Agreements + Attachments + File Download Summary

End-to-end employment-agreements slice: domain types, sqlc-backed repository with BPJS JSON marshal/unmarshal, business-logic service with PKWT/PKWTT cross-field rules + EA-2 one-active guard + EA-3 renew-as-successor + EA-5 close, multipart upload handler, authenticated file-download handler, RBAC route groups in server.go, cmd/api wiring, i18n error codes, and seeded agreement + attachment.

## What Was Built

### Domain Extension (backend/internal/domain/people.go)

Three new types added to the existing people domain:

**Agreement** struct:
- All OpenAPI Agreement schema fields: id, employee_id, type (PKWT/PKWTT), agreement_no, start_date, end_date (nil for PKWTT), status, predecessor_id, successor_id, closed_reason, closed_at, compensation, created_by, created_at, updated_at
- `CompensationTerms` nested struct: BaseSalaryIDR *float64, BpjsTerms (four *float64 pct fields), TaxProfile *string, EffectiveDate *time.Time
- `BpjsTerms` struct with all four BPJS percentage fields matching the JSONB column

**AgreementFilter** struct: EmployeeID, Status, Type, EndDateLTE, Limit, cursor fields

**Attachment** struct: ID, AgreementID, Category, Caption, FileName, MIME, SizeBytes, Blob, UploadedBy, CreatedAt

### Repository (backend/internal/repository/people/agreements_repo.go)

`AgreementRepo` implements the `AgreementRepository` interface:
- `ListAgreements`, `GetAgreementByID`, `GetActiveAgreementForEmployee` — reads on pool
- `GetEmployeeByID` — reuses `mapEmployeeFromGetByID` from employees_repo.go (same package) for the pre-create employee existence check
- `CreateAgreement` (tx) — marshals BpjsTerms → JSON before insert; pgtype.Numeric for BaseSalaryIDR
- `SetAgreementStatus` (tx) — drives :close + supersede-on-renew; sets closed_reason/closed_at/successor_id atomically
- `CreateAttachment` (tx) — passes blob bytea, returns metadata-only row (matches sqlc RETURNING clause)
- `GetAttachmentByID` — returns metadata + blob for the download handler

**BPJS JSON marshal/unmarshal**: `unmarshalComp()` handles pgtype.Numeric → *float64 conversion (including exponent scaling) and `json.Unmarshal` of the JSONB bytes into `domain.BpjsTerms`.

### Service (backend/internal/service/people/agreements_service.go)

`AgreementService` with consumer-defined `AgreementRepository` interface; reuses `TxRunner` and `Clock` from employees_service.go (same package):

**ListAgreements**: EXPIRING virtual status translated to `status=active + end_date__lte=now+30d` before querying; limit+1 fetch + cursor encode.

**CreateAgreement**: Five-step validation:
1. Employee must exist → 404
2. PKWT requires end_date; PKWTT must not have end_date → 400 INVALID_REQUEST
3. end_date < start_date → 400 INVALID_REQUEST
4. PKWT period > 5 years → 422 PKWT_PERIOD_EXCEEDS_MAX
5. Active agreement exists → 409 ACTIVE_AGREEMENT_EXISTS (Conflict constructor, NOT Rule)

**RenewAgreement (EA-3)**: Loads predecessor (404 guard); predecessor.status != "active" → 409 CONFLICT; validates successor PKWT dates; InTx: CreateAgreement(successor) + SetAgreementStatus(predecessor→superseded, successor_id=new.id) + audit("agreement.renew"). Returns 201.

**CloseAgreement (EA-5)**: Loads agreement (404); status != "active" → 409 CONFLICT; validates reason enum + effective_date; InTx: SetAgreementStatus(closed) + audit("agreement.close"). Returns 200.

**UploadAttachment**: Loads agreement (404); validates size ≤ 10MB (FILE_TOO_LARGE struct literal → 413); validates MIME ∈ {application/pdf, image/jpeg, image/png} (→ 400); InTx: CreateAttachment + audit("agreement.attach"). Returns domain.Attachment.

**GetAttachment**: repo.GetAttachmentByID → 404 guard. Returns metadata + blob.

### Handler + DTOs (backend/internal/handler/people/)

**agreements_dto.go:**
- `agreementResponse` — all snake_case fields; EXPIRING virtual status in `toAgreementResponse(ag, now)` — computed at DTO boundary from `status="active" AND type="PKWT" AND end_date < now+30d`
- `agreementWriteRequest`, `renewRequest`, `closeRequest` structs
- `compensationTermsReq` / `compensationTermsResp` with nested `bpjsTermsReq` / `bpjsTermsResp`
- `fileRefResponse` — `{id, url, name, size_bytes, mime, uploaded_at}` per §15; url = `/api/v1/files/{id}`
- `toCompensationParams()` helper to parse compensation req → service params

**agreements_handler.go:**
- `AgreementHandler{svc *svc.AgreementService}`, `NewAgreementHandler(s)`
- `ListAgreements` — parses employee_id/status/type/end_date__lte/limit/cursor
- `GetAgreement` — URL param `agreement_id`
- `CreateAgreement` — decode JSON → 201 + Location
- `RenewAgreement` — decode JSON → 201 + Location (new id)
- `CloseAgreement` — decode JSON → 200
- `UploadAttachment` — `r.ParseMultipartForm(10<<20)` → `r.FormFile("file")` → `io.ReadAll` → MIME from part header → `svc.UploadAttachment` → 201 fileRefResponse
- `DownloadFile` — `svc.GetAttachment` → `Content-Type=att.MIME` + `Content-Disposition: inline` → `w.Write(blob)` → 200

### server.go Changes

PEOPLE agreements slice (04-03) added after the "PEOPLE slice end (04-02)" marker:

```
// Reads: super_admin, hr_admin
r.Get("/agreements", d.PeopleAgreements.ListAgreements)
r.Get("/agreements/{agreement_id}", d.PeopleAgreements.GetAgreement)
r.Get("/files/{file_id}", d.PeopleAgreements.DownloadFile)

// Writes: super_admin, hr_admin
r.With(d.Idempotency.Handler).Post("/agreements", ...)                          // + Idempotency
r.With(d.Idempotency.Handler).Post("/agreements/{agreement_id}:renew", ...)    // + Idempotency
r.With(d.Idempotency.Handler).Post("/agreements/{agreement_id}:close", ...)    // + Idempotency
r.Post("/agreements/{agreement_id}/attachments", ...)                           // NO Idempotency
// PEOPLE agreements slice end (04-03). 04-04 change-requests: append here.
```

`PeopleAgreements *peoplehttp.AgreementHandler` added to Deps.

### cmd/api/main.go Changes

```go
// People agreements slice (04-03): employment agreements + attachments + file download (PPL-02).
agreementsRepo := peoplerepo.NewAgreementRepo(pool)
agreementsSvc := peoplesvc.NewAgreementService(agreementsRepo, txm)
agreementsHandler := peoplehttp.NewAgreementHandler(agreementsSvc)
```

`PeopleAgreements: agreementsHandler` in `server.Deps` literal.

### i18n Changes

Three new error codes added to both `id` and `en` language blocks:
- `PKWT_PERIOD_EXCEEDS_MAX`: "Periode PKWT melebihi batas 5 tahun..."
- `ACTIVE_AGREEMENT_EXISTS`: "Karyawan sudah memiliki perjanjian aktif. Gunakan endpoint :renew..."
- `FILE_TOO_LARGE`: "Ukuran file melebihi batas maksimum 10 MB."

### Seed Changes (backend/cmd/seed/seed.go)

`seedAgreements()` inserts two fixtures with `ON CONFLICT (id) DO NOTHING`:

| Entity | ID | Description |
|--------|-----|-------------|
| employment_agreements | SWP-AG-7001 | ACTIVE PKWT for Budi Santoso (SWP-EMP-2891); PKWT/SWP/2026/0142; 2026-06-01 → 2027-05-31; salary 5,200,000; bpjs_terms jsonb; PTKP_K0 |
| agreement_attachments | SWP-FILE-9001 | signed_agreement for SWP-AG-7001; pkwt-budi.pdf; application/pdf; minimal valid PDF blob |

`seedAgreements` called after `seedMasterData` in `Seed()`. Marker `// 04-04 change-requests: append seedChangeRequests call here.` added for the next plan.

## Key Design Decisions

### Bytea Blob Storage for Attachments

`UploadAttachment` reads the entire file into memory with `io.ReadAll` and passes it to `repo.CreateAttachment` as `[]byte`. `DownloadFile` calls `repo.GetAttachmentByID` which returns the blob, then writes it directly with `w.Write(blob)`. The DB is the single source of truth — no filesystem writes, no S3 dependency. Documented trade-offs in 04-01-SUMMARY.md carry forward.

### EXPIRING Virtual Status — DTO Boundary Computation

The DB has no "expiring" status value. `toAgreementResponse` checks:
- `ag.Status == "active"` AND `ag.Type == "PKWT"` AND `ag.EndDate != nil` AND `ag.EndDate < now + 30 days`

If all true, emits `"EXPIRING"` in the response. The `ListAgreements` service method also translates `status=EXPIRING` filter to `status=active + end_date__lte=now+30d` so the query returns the right rows.

### Compensation Plaintext This Milestone

`base_salary_idr`, `bpjs_terms` JSONB, `tax_profile`, and `comp_effective_date` are stored and returned plaintext. EA-4 (encryption at rest) is out of scope in Phase 4. A comment in the service notes this explicitly. The DTO always includes the compensation object (never masked in this phase — EA-4 role-gated masking is deferred).

### FILE_TOO_LARGE 413 Struct Literal

`apperr.Error{Code: "FILE_TOO_LARGE", HTTPStatus: http.StatusRequestEntityTooLarge}` — struct literal is required because `statusForCode` in apperr.go has no 413 case (all unknown codes default to 422 via the switch-default). Same technique used for GEOFENCE_RADIUS_INVALID in Phase 3.

### ACTIVE_AGREEMENT_EXISTS vs Rule()

Used `apperr.Conflict("ACTIVE_AGREEMENT_EXISTS")` (409) not `apperr.Rule()`. The conflict is a state-of-record constraint (EA-2: INV-style invariant), not a semantic rule like PKWT_PERIOD_EXCEEDS_MAX. `apperr.Conflict()` forces 409 unconditionally; `apperr.Rule()` would default to 422 via statusForCode.

## Coordination Contract for 04-04

### server.go

Append new `r.Group{}` blocks **after**:
```
// PEOPLE agreements slice end (04-03). 04-04 change-requests: append here.
```

### cmd/api/main.go

Append new repo/svc/handler wiring blocks after the agreements slice block (which ends with `agreementsHandler := peoplehttp.NewAgreementHandler(agreementsSvc)`). Add new Deps fields after `PeopleAgreements: agreementsHandler`.

### cmd/seed/seed.go

`seedAgreements` is called after `seedMasterData`. 04-04 seed should be appended **after** the `seedAgreements` call (look for the `// 04-04 change-requests: append seedChangeRequests call here.` marker).

### Seed ordering summary:
1. `seedEmployees` (04-02) — MUST be first
2. persona user loop (Phase 1)
3. `seedAuditLog` (Phase 1)
4. `seedClientCompanies` (03-02)
5. `seedServiceLines` (03-03)
6. `seedMasterData` (03-04)
7. `seedAgreements` (04-03) — this plan
8. `seedChangeRequests` (04-04) — append after agreements

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

### Created files exist:
- backend/internal/repository/people/agreements_repo.go: FOUND
- backend/internal/service/people/agreements_service.go: FOUND
- backend/internal/handler/people/agreements_dto.go: FOUND
- backend/internal/handler/people/agreements_handler.go: FOUND

### Modified files exist:
- backend/internal/domain/people.go: FOUND
- backend/internal/platform/i18n/i18n.go: FOUND
- backend/internal/server/server.go: FOUND
- backend/cmd/api/main.go: FOUND
- backend/cmd/seed/seed.go: FOUND

### Commits exist:
- 38067be: feat(04-03): agreements domain + repo + service (PKWT rules, renew, close)
- cd5d047: feat(04-03): agreement handlers (multipart upload + authed download) + routes + wiring
- 71bacd7: feat(04-03): seed employment agreement SWP-AG-7001 + attachment SWP-FILE-9001

### Build:
- `go build ./...` exits 0
- `go vet ./...` exits 0
- Agreement routes mounted in server.go: GET/POST /agreements, :renew, :close, /attachments, GET /files/{file_id}
- agreements_handler.go contains ParseMultipartForm and DownloadFile
- cmd/api/main.go has PeopleAgreements field

## Self-Check: PASSED
