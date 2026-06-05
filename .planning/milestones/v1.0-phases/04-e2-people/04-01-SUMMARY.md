---
phase: 04-e2-people
plan: "01"
subsystem: backend/db
tags: [migration, sqlc, people, employees, agreements, attachments, change-requests]
dependency_graph:
  requires: [03-e2-org-master-data]
  provides: [employees-table, employment-agreements-table, agreement-attachments-table, change-requests-table, people-sqlc-methods]
  affects: [backend/internal/repository/sqlc]
tech_stack:
  added: []
  patterns: [goose-soft-delete, swp_next_id-inline, cursor-pagination-people, bytea-blob-storage, partial-unique-index-ea2]
key_files:
  created:
    - backend/db/migrations/00016_employees.sql
    - backend/db/migrations/00017_employment_agreements.sql
    - backend/db/migrations/00018_agreement_attachments.sql
    - backend/db/migrations/00019_change_requests.sql
    - backend/db/queries/people/employees.sql
    - backend/db/queries/people/agreements.sql
    - backend/db/queries/people/agreement_attachments.sql
    - backend/db/queries/people/change_requests.sql
    - backend/internal/repository/sqlc/employees.sql.go
    - backend/internal/repository/sqlc/agreements.sql.go
    - backend/internal/repository/sqlc/agreement_attachments.sql.go
    - backend/internal/repository/sqlc/change_requests.sql.go
  modified:
    - backend/internal/platform/ids/ids.go
decisions:
  - "Bytea blob for agreement_attachments: simplest approach that passes E2E and survives container teardown via reseed; no external storage dependency"
  - "EA-2 enforced at DB level via partial unique index on employment_agreements(employee_id) WHERE status='active' AND deleted_at IS NULL"
  - "Compensation stored as plain columns (base_salary_idr, bpjs_terms jsonb, tax_profile, comp_effective_date); encryption at rest deferred (EA-4)"
  - "DB stores lowercase status values (active/inactive/superseded/closed/pending/approved/rejected); uppercased to ACTIVE/INACTIVE etc. only at DTO boundary (matches Phase-2 convention)"
  - "File prefix FILE added to ids.go for SWP-FILE attachment IDs"
  - "change_requests uses (submitted_at, id) as cursor (not created_at) to match the logical chronological queue order"
metrics:
  duration_seconds: 271
  completed_date: "2026-06-04"
  tasks_completed: 2
  files_created: 12
  files_modified: 1
---

# Phase 4 Plan 01: People Data Layer Summary

Four migration files and four sqlc query files creating the E2 people data layer — employees, employment agreements, agreement attachments, and change requests — with all Go query methods generated and `go build ./...` clean.

## What Was Built

### Migrations (00016–00019)

**00016_employees.sql** — `employees` table:
- `id text PRIMARY KEY` allocated via `'SWP-EMP-' || swp_next_id('EMP')`
- Full demographic fields: NIK, NIP, join_at, gender, birth_date/place, contact, BPJS IDs, NPWP
- Flat bank_account columns (`bank_name`, `bank_account_number`, `bank_account_holder_name`) — avoids jsonb for potentially indexed fields
- Partial unique index `employees_nik_uq ON employees(nik) WHERE deleted_at IS NULL` — enforces EP-2 (NIK uniqueness) without blocking soft-deleted records
- `user_id` nullable with no FK — keeps loose coupling with E1 users (same pattern as Phase-1 persona literals); FK would create a circular dependency risk

**00017_employment_agreements.sql** — `employment_agreements` table:
- EA-2 enforcement at the DB level: `employment_agreements_active_employee_uq ON employment_agreements(employee_id) WHERE status = 'active' AND deleted_at IS NULL` — one active agreement per employee, DB-guaranteed
- Compensation stored as plain columns this milestone (`base_salary_idr numeric`, `bpjs_terms jsonb`, `tax_profile text`, `comp_effective_date date`); encryption at rest deferred (EA-4)
- `closed_reason` CHECK allows only RESIGNED/TERMINATED/END_OF_TERM/OTHER or NULL
- `predecessor_id`/`successor_id` nullable text (no FK to self — avoids constraint ordering issues on renew)

**00018_agreement_attachments.sql** — `agreement_attachments` table:
- `blob bytea NOT NULL` — storage choice: in-DB bytea. Rationale: simplest working approach that passes E2E and survives container teardown via reseed; no external storage service dependency in this phase
- `category` CHECK: signed_agreement/addendum/supporting_doc
- No `updated_at`/`deleted_at` — attachments are immutable once uploaded

**00019_change_requests.sql** — `change_requests` table:
- `changes jsonb NOT NULL` — stores the `{phone?,address?,bank_account?}` subset as a document
- `request_type` CHECK: PHONE/ADDRESS/BANK_ACCOUNT/MULTIPLE — derived at create time by the service, stored for HR filter performance
- Compound index `change_requests_status_idx ON change_requests(status, submitted_at DESC, id DESC)` — efficient HR queue list by status + chrono order

### sqlc Query Files (backend/db/queries/people/)

**employees.sql** — 6 methods:
- `ListEmployees :many` — full-text filter over full_name/nik/nip/email_personal/phone, status filter, cursor on (created_at, id)
- `GetEmployeeByID :one` — by id + soft-delete guard
- `GetEmployeeByNIK :one` — EP-2 duplicate-NIK pre-check before insert/update
- `CreateEmployee :one` — inline SWP-EMP id allocation, all fields including nullable demographics
- `UpdateEmployee :one` — all editable columns, updates `updated_at`
- `SetEmployeeStatus :one` — drives :deactivate/:reactivate

**agreements.sql** — 5 methods:
- `ListAgreements :many` — filters: employee_id, status, type, end_date__lte (for EXPIRING virtual status server-compute); cursor on (created_at, id)
- `GetAgreementByID :one`
- `GetActiveAgreementForEmployee :one` — EA-2 pre-check; also the predecessor lookup for :renew/:close
- `CreateAgreement :one` — inline SWP-AG id; predecessor_id nullable for :renew chain
- `SetAgreementStatus :one` — drives :close + supersede-on-renew; sets closed_reason/closed_at/successor_id atomically

**agreement_attachments.sql** — 2 methods:
- `CreateAttachment :one` — RETURNING metadata only (not blob); inline SWP-FILE id
- `GetAttachmentByID :one` — RETURNING id, file_name, mime, size_bytes, blob — for authenticated download handler

**change_requests.sql** — 4 methods:
- `ListChangeRequests :many` — cursor on (submitted_at, id) — matches the chronological queue nature; filters: status, employee_id, request_type
- `GetChangeRequestByID :one`
- `CreateChangeRequest :one` — inline SWP-CHG id; changes jsonb + request_type stored
- `ResolveChangeRequest :one` — drives :approve/:reject; sets resolved_at, resolved_by, rejection_reason

### ids.go Update

Added `File Prefix = "FILE"` for SWP-FILE attachment IDs (consistent with all other entity prefixes in the file).

## Key Design Decisions

### Bytea Blob Storage for Attachments

Chose in-DB `bytea` over local-disk or object storage for this milestone. Trade-offs acknowledged:
- **Pros:** Zero infrastructure dependency; works in any Postgres; survives container teardown + reseed; handler is pure Go with no filesystem writes
- **Cons:** DB size grows with file uploads; not suitable for production at scale without an S3/GCS layer
- **Migration path:** Later phases can add an `s3_key` column + background migrator; the `GET /files/{id}` download URL pattern is already abstract enough to switch storage without API changes

### EA-2 Partial Unique Index

The constraint `WHERE status = 'active' AND deleted_at IS NULL` means:
- The DB itself rejects a second INSERT for the same employee while their first agreement is active (service-layer conflict becomes DB-caught)
- Superseded/closed/soft-deleted agreements are unconstrained (historical record preserved)
- The service layer still calls `GetActiveAgreementForEmployee` before :renew/:close to get the predecessor and build the chain — the index is a safety net, not a replacement for the business logic

### Column-to-DTO Casing

DB stores lowercase values (`active`, `pending`, `pkwt`) per the Phase-2 convention. Service layer maps to uppercase at the DTO boundary only (`ACTIVE`, `PENDING`, `PKWT`). This matches the pattern established in Phase-2 foundations and Phase-3 org.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

### Created files exist:
- backend/db/migrations/00016_employees.sql: FOUND
- backend/db/migrations/00017_employment_agreements.sql: FOUND
- backend/db/migrations/00018_agreement_attachments.sql: FOUND
- backend/db/migrations/00019_change_requests.sql: FOUND
- backend/db/queries/people/employees.sql: FOUND
- backend/db/queries/people/agreements.sql: FOUND
- backend/db/queries/people/agreement_attachments.sql: FOUND
- backend/db/queries/people/change_requests.sql: FOUND
- backend/internal/repository/sqlc/employees.sql.go: FOUND
- backend/internal/repository/sqlc/agreements.sql.go: FOUND
- backend/internal/repository/sqlc/agreement_attachments.sql.go: FOUND
- backend/internal/repository/sqlc/change_requests.sql.go: FOUND

### Commits exist:
- 18f192c: feat(04-01): four people migrations
- 154220b: feat(04-01): sqlc query files for people + File id prefix + regenerated sqlc

### Build:
- `make gen` exits 0
- `go build ./...` exits 0
- Migrations applied: goose version 19

## Self-Check: PASSED
