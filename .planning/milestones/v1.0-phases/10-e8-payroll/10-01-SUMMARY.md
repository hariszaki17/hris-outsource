---
phase: 10-e8-payroll
plan: 01
subsystem: backend-data
tags: [payroll, encryption-at-rest, sqlc, migration, crypto, export-jobs]
dependency-graph:
  requires:
    - employees(id) FK target (00016)
    - placements(id) FK target (05-01)
    - swp_next_id('PS') / swp_next_id('EXP') sequences (ids.go PS+EXP prefixes pre-existing)
  provides:
    - payslips + payslip_components + payslip_benefits + payslip_audit_notes tables (encrypted-at-rest)
    - export_jobs table (QUEUED/RUNNING/DONE/FAILED lifecycle)
    - internal/platform/crypto (AES-256-GCM Encrypt/Decrypt + ErrDecrypt)
    - config.Crypto.PayrollKey (PAYROLL_ENCRYPTION_KEY env)
    - sqlc Querier methods (ListPayslips/GetPayslip/.../CountPayslipsInScope/UpdateExportJobStatus/...)
    - internal/domain/payroll types (Payslip/ExportJob/lines/enums)
  affects:
    - 10-02 (services/handlers/seed/worker build on this data layer + crypto)
tech-stack:
  added:
    - crypto/aes + crypto/cipher (AES-256-GCM) stdlib — no new deps
  patterns:
    - column-DEFAULT SWP-id allocation ([05-01])
    - bigserial child tables (no SWP id) for nested line items ([08-01]/[09-01])
    - encrypted-at-rest *_enc bytea columns (NEW; INV-2)
    - keyset cursor with COALESCE(paid_on, sentinel) NULLS-LAST DESC
key-files:
  created:
    - backend/db/migrations/00033_payslips.sql
    - backend/db/migrations/00034_export_jobs.sql
    - backend/db/queries/payroll/payslips.sql
    - backend/db/queries/payroll/audit_notes.sql
    - backend/db/queries/payroll/export_jobs.sql
    - backend/internal/platform/crypto/crypto.go
    - backend/internal/platform/crypto/crypto_test.go
    - backend/internal/domain/payroll/payroll.go
  modified:
    - backend/internal/platform/config/config.go
    - backend/internal/repository/sqlc/ (regenerated — payslips/export_jobs/audit_notes .sql.go, models.go, querier.go)
decisions:
  - "[10-01]: monetary fields stored as *_enc bytea AES-256-GCM ciphertext (INV-2) — NO plaintext money column anywhere; decrypt at the 10-02 service boundary"
  - "[10-01]: export_jobs terminal-success status is DONE (openapi PayslipExportJob.status), NOT COMPLETED (CONTEXT prose) — spec wins byte-for-byte"
  - "[10-01]: crypto.ErrDecrypt is the typed DECRYPT_FAIL source; DecryptPtr distinguishes null(ok,nil)/valid(ok,value)/garbage(fail,nil)"
  - "[10-01]: payslip_audit_notes.id is a service-assigned composite text PK '{payslip_id}-NOTE-{seq}' (NOT swp_next_id); seq via CountPayslipAuditNotes+1; unique (payslip_id,seq) guard"
  - "[10-01]: ListPayslips keyset uses COALESCE(paid_on,DATE '0001-01-01') so NULL paid_on sorts last under DESC; cursor tuple (paid_on,id)"
metrics:
  duration: ~20min
  completed: 2026-06-05
---

# Phase 10 Plan 01: Payroll Data Layer + Crypto Helper Summary

E8 payroll persistence foundation: two goose migrations (encrypted-at-rest payslips + breakdown + append-only audit notes; async export-job tracker), the sqlc query set, the `internal/domain/payroll` types pinned to the E8 openapi enums, and a NEW `internal/platform/crypto` AES-256-GCM helper (key from config) — the gnarly DECRYPT_FAIL source for INV-2.

## What was built

- **00033_payslips.sql** — `payslips` (money stored as `gross_earnings_enc` / `gross_deductions_enc` / `take_home_pay_enc` **bytea ciphertext**, never plaintext; `status` CHECK `FINAL`/`DECRYPT_FAIL`; `source_system`/`source_id`; soft-delete; column-DEFAULT `SWP-PS-` id) + three child tables: `payslip_components` (`kind` EARNING/DEDUCTION + `value_enc` + `for_bpjs`), `payslip_benefits` (`value_enc`), `payslip_audit_notes` (composite text id + `seq` + author, unique `(payslip_id,seq)`).
- **00034_export_jobs.sql** — `export_jobs` with the `QUEUED/RUNNING/DONE/FAILED` lifecycle, scope columns (`scope_period`/`scope_year`/`scope_employee_ids`), `confidential` default true, `row_count`/`artifact_ref`/`error_message`, `started_at`/`completed_at`.
- **internal/platform/crypto** — AES-256-GCM `New`/`NewFromBase64`/`Encrypt`/`Decrypt`/`DecryptPtr`, nonce-prepended, typed `ErrDecrypt`. Round-trip + garbage + wrong-key + too-short + three-case `DecryptPtr` + base64 unit tests all green.
- **config.Crypto.PayrollKey** — loaded from `PAYROLL_ENCRYPTION_KEY` (base64 32-byte key; milestone env constant, not KMS).
- **sqlc queries** (`db/queries/payroll/`) — payslips list/get/components/benefits + seed inserts; audit notes list/count/insert + `PayslipExists`; export jobs insert/get/update-status + `CountPayslipsInScope`. `make gen` clean.
- **internal/domain/payroll** — `Payslip`, `EarningLine`/`DeductionLine`/`BenefitLine`, `PayslipAuditNote`, `ExportJob`, `SourceRef`, and `PayslipStatus`/`ExportJobStatus` enums byte-for-byte to openapi. Money is `*string` with **no arithmetic method** (INV-2).

## Reference for 10-02 (handoff)

The 10-02 service/handler/seed/worker slice builds on this. Everything below is verified against the regenerated `internal/repository/sqlc`.

### Tables & columns
- `payslips(id text PK, employee_id, employee_name?, placement_id?, year int, month int, paid_on date?, working_days int?, gross_earnings_enc bytea?, gross_deductions_enc bytea?, take_home_pay_enc bytea?, status text, source_system text, source_id text, created_at, updated_at, deleted_at?)`
- `payslip_components(id bigserial PK, payslip_id, kind EARNING|DEDUCTION, name, value_enc bytea?, for_bpjs bool, sort_order int)`
- `payslip_benefits(id bigserial PK, payslip_id, name, value_enc bytea?, sort_order int)`
- `payslip_audit_notes(id text PK composite, payslip_id, seq int, text, author_id, author_name?, created_at)` — unique `(payslip_id, seq)`
- `export_jobs(id text PK, kind, status, format, confidential, requested_by_id, requested_by_name?, scope_period?, scope_year?, scope_employee_ids text[], row_count?, artifact_ref?, error_message?, requested_at, started_at?, completed_at?)`

### crypto signatures (`internal/platform/crypto`)
```go
func New(key []byte) (*Cipher, error)            // requires len(key)==32
func NewFromBase64(b64 string) (*Cipher, error)  // config.Crypto.PayrollKey -> here
func (c *Cipher) Encrypt(plaintext string) ([]byte, error)   // nonce-prepended GCM
func (c *Cipher) Decrypt(ciphertext []byte) (string, error)  // -> ErrDecrypt on garbage/short/wrong-key
func (c *Cipher) DecryptPtr(ciphertext []byte) (*string, bool) // nil/empty->(nil,true); valid->(&v,true); garbage->(nil,false)
var ErrDecrypt = errors.New("crypto: decrypt failed")
```
**Boundary rule:** in 10-02, decrypt each `*_enc` with `DecryptPtr`. If ANY present-but-garbage column returns `ok=false`, set the payslip `Status=DECRYPT_FAIL`, `DecryptFail=true`, null ALL money, `LockedReason="decrypt_fail"`, and return `earnings/deductions/benefits` as `[]` (200 OK, not an error). A NULL `*_enc` (`ok=true, *string=nil`) is just an absent value, NOT a decrypt failure. Construct the Cipher once at startup from `cfg.Crypto.PayrollKey` via `NewFromBase64` (wire into the payroll service deps). The seed (10-04 env) MUST encrypt with the SAME key.

### sqlc Querier signatures (exact generated names + type quirks)
**payslips.sql.go**
- `ListPayslips(ctx, ListPayslipsParams) ([]ListPayslipsRow, error)` — `Params{ EmployeeID *string; Year *int32; Month *int32; Status *string; CursorID *string; CursorPaidOn pgtype.Date; Lim int32 }`. Row carries the three `*_enc []byte` + `PaidOn pgtype.Date` + `WorkingDays *int32`. (period filter = split YYYY-MM into Year+Month.)
- `GetPayslip(ctx, id string) (GetPayslipRow, error)`
- `ListPayslipComponents(ctx, payslipID string) ([]PayslipComponent, error)` — `PayslipComponent{ ID int64; PayslipID; Kind; Name; ValueEnc []byte; ForBpjs bool; SortOrder int32 }`
- `ListPayslipBenefits(ctx, payslipID string) ([]PayslipBenefit, error)` — `PayslipBenefit{ ID int64; PayslipID; Name; ValueEnc []byte; SortOrder int32 }`
- `InsertPayslip(ctx, InsertPayslipParams) (InsertPayslipRow, error)` — `Params.ID *string` (nil → DEFAULT fires), `*_enc []byte`, `PaidOn pgtype.Date`, `WorkingDays *int32`, `Status/SourceSystem/SourceID string`. `ON CONFLICT (id) DO NOTHING` → the `:one` returns `pgx.ErrNoRows` when the explicit-id row already exists (seed re-run); handle like the overtime seed.
- `InsertPayslipComponent(ctx, InsertPayslipComponentParams) (PayslipComponent, error)`, `InsertPayslipBenefit(ctx, InsertPayslipBenefitParams) (PayslipBenefit, error)` — `ValueEnc []byte` (narg).

**audit_notes.sql.go**
- `ListPayslipAuditNotes(ctx, ListPayslipAuditNotesParams) ([]PayslipAuditNote, error)` — `Params{ PayslipID string; CursorSeq *int32; CursorCreatedAt *time.Time; Lim int32 }` (oldest-first, keyset `(created_at,seq) ASC`). `PayslipAuditNote{ ID; PayslipID; Seq int32; Text; AuthorID; AuthorName *string; CreatedAt time.Time }`.
- `CountPayslipAuditNotes(ctx, payslipID string) (int64, error)` — next `seq = count+1`; build the composite id `fmt.Sprintf("%s-NOTE-%d", payslipID, seq)`.
- `InsertPayslipAuditNote(ctx, InsertPayslipAuditNoteParams) (PayslipAuditNote, error)` — `Params{ ID; PayslipID; Seq int32; Text; AuthorID; AuthorName *string }`.
- `PayslipExists(ctx, id string) (bool, error)` — the note-create/list 404 guard.

**export_jobs.sql.go**
- `InsertExportJob(ctx, InsertExportJobParams) (ExportJob, error)` — `Params{ ID *string; Kind *string; Format *string; Confidential *bool; RequestedByID string; RequestedByName *string; ScopePeriod *string; ScopeYear *int32; ScopeEmployeeIds []string }` (nil ID/Kind/Format/Confidential → COALESCE defaults: SWP-EXP id, PAYSLIP_EXPORT, XLSX, true). Returns `ExportJob{ ID; Kind; Status; Format; Confidential; RequestedByID; RequestedByName *string; ScopePeriod *string; ScopeYear *int32; ScopeEmployeeIds []string; RowCount *int32; ArtifactRef *string; ErrorMessage *string; RequestedAt; StartedAt *time.Time; CompletedAt *time.Time }`. **Insert inside the same tx as `jobs.Client.EnqueueTx` (transactional outbox).**
- `GetExportJob(ctx, id string) (ExportJob, error)`
- `UpdateExportJobStatus(ctx, UpdateExportJobStatusParams) (ExportJob, error)` — `Params{ Status string; RowCount *int32; ArtifactRef *string; ErrorMessage *string; ID string }`. The SQL stamps `started_at` once on RUNNING and `completed_at` on DONE/FAILED automatically; pass nil for the result fields you aren't setting (COALESCE keeps prior).
- `CountPayslipsInScope(ctx, CountPayslipsInScopeParams) (int64, error)` — `Params{ Year *int32; Month *int32; EmployeeIds []string }` (empty/nil EmployeeIds = no employee restriction). Compare to the EXPORT_TOO_LARGE threshold (default 50,000).

### sqlc type quirks (mirror Phase-5/6/8/9 repo conversions in 10-02)
- `paid_on` → `pgtype.Date` (convert `<->` `*time.Time`; NULL = `Valid:false`).
- `working_days`/`year`/`month`/`seq`/`sort_order`/`row_count`/`scope_year` → `int32` (cast to/from domain `int`).
- `*_enc` / `value_enc` → `[]byte` (nil = NULL column).
- `employee_name`/`placement_id`/`requested_by_name`/`author_name`/`artifact_ref`/`error_message` → `*string`.
- `scope_employee_ids` → `[]string` (sqlc names it `ScopeEmployeeIds`).
- `started_at`/`completed_at` → `*time.Time`; `created_at`/`requested_at`/`updated_at` → `time.Time`.
- `ON CONFLICT DO NOTHING` `:one` inserts return `pgx.ErrNoRows` on conflict — the repo maps that appropriately for the idempotent seed.

### Enums (domain ↔ openapi, byte-for-byte)
- `PayslipStatusFinal="FINAL"`, `PayslipStatusDecryptFail="DECRYPT_FAIL"`.
- `ExportJobStatusQueued/Running/Done/Failed` = `QUEUED/RUNNING/DONE/FAILED` (**DONE** is terminal-success — the worker sets DONE).
- `LockedReasonDecryptFail="decrypt_fail"`, `SourceSystemLumenSwp="lumen_swp"`.

### Not done here (10-02 owns)
Repository, service (decrypt-at-boundary + RBAC + River enqueue + audit + notify stub), hand-written chi handlers, routes under `RequireRole(hr_admin, super_admin)`, `PayslipExportWorker`, the seed (incl. the deliberately-garbage DECRYPT_FAIL ciphertext row), and `cfg.Crypto.PayrollKey` wiring into main/seed env.

## Deviations from Plan

None — plan executed exactly as written. The plan's CONTEXT-vs-openapi `COMPLETED`/`DONE` discrepancy was pre-resolved in the plan (use `DONE`); followed accordingly.

### Out-of-scope discoveries (logged, NOT fixed)
- `gofmt -l internal/` flags ~13 PRE-EXISTING Phase 1-4 identity/people files as unformatted. None touched by 10-01; logged to `.planning/phases/10-e8-payroll/deferred-items.md`. All files created by 10-01 are gofmt-clean.

## Verification

- `make gen` (sqlc) clean.
- `go build ./...` exits 0.
- `go vet ./...` exits 0.
- `go test ./internal/platform/crypto/` PASS (round-trip + ErrDecrypt on garbage/short/wrong-key + DecryptPtr three-case + base64).
- No plaintext money column on any table (only `*_enc bytea`).
- `internal/platform/ids/ids.go` untouched (PS + EXP prefixes pre-existing).

## Self-Check: PASSED

All 8 created files present on disk; all 3 task commits (9cf323e, 65513b5, 5d33f55) in git log.
