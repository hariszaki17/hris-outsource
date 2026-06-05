---
phase: 07-e5-attendance
plan: 01
subsystem: database
tags: [postgres, goose, sqlc, attendance, corrections, pgx, geofence]

# Dependency graph
requires:
  - phase: 06-e4-schedule-shifts
    provides: schedule_entries table (E4 FK target) + sqlc cursor-list/keyset pattern + column-DEFAULT swp_next_id allocation
  - phase: 05-e3-placement
    provides: placements table (scope source FK) + partial-unique-index INV backstop pattern + internal/domain pointer-for-nullable convention
provides:
  - "attendance table (migration 00026): records w/ stored geofence+lateness+auto_closed columns, status + verification_status enums, flags text[], FKs to schedule_entries/placements/client_companies/employees"
  - "attendance_corrections table (migration 00027): status enum, proposed_* fields, original_snapshot jsonb, attendance_shift_date; one-pending-per-attendance partial unique index"
  - "sqlc queries (db/queries/attendance/*.sql): ListAttendance/GetAttendance/GetAttendanceForUpdate/VerifyAttendance/RejectAttendance/ApplyCorrectionToAttendance + ListCorrections/GetCorrection/GetCorrectionForUpdate/ApproveCorrection/RejectCorrection"
  - "generated sqlcgen Querier (11 new methods) + Attendance/AttendanceCorrection models"
  - "internal/domain/attendance package: Attendance, Correction, GeofenceCheck, DiffRow + all enums"
affects: [07-02, 07-03, 07-04, E7-overtime, E8-payroll, E10-reporting]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Stored-column geofence/lateness (no runtime Haversine): in_geofence/in_distance_m/out_geofence/out_distance_m/geofence_radius_m columns seeded directly so the verification UI has honest exceptions without the mobile clock pipeline"
    - "State-guarded RETURNING update queries (WHERE ... AND verification_status IN ('PENDING','ESCALATED') RETURNING *): zero rows ⇒ terminal-state 409 in the 07-02 service (mirrors the Phase-5 lifecycle pattern)"
    - "Apply-on-approve via COALESCE(narg, existing) whitelist + array_remove/append for the CORRECTED flag (de-duped)"

key-files:
  created:
    - backend/db/migrations/00026_attendance.sql
    - backend/db/migrations/00027_attendance_corrections.sql
    - backend/db/queries/attendance/attendance.sql
    - backend/db/queries/attendance/corrections.sql
    - backend/internal/domain/attendance/attendance.go
    - backend/internal/domain/attendance/correction.go
  modified:
    - backend/internal/repository/sqlc/querier.go
    - backend/internal/repository/sqlc/models.go

key-decisions:
  - "Geofence/lateness/auto-close are plain STORED columns (Claude's-discretion call per CONTEXT): in_geofence/in_distance_m/out_geofence/out_distance_m/geofence_radius_m + is_late/late_minutes/auto_closed — simplest path to honest PENDING exceptions for the seed in 07-02; no clock pipeline."
  - "Correction apply-on-approve = mark status APPLIED (not a separate APPROVED→APPLIED two-step): ApproveCorrection sets status='APPLIED' and 07-02 calls ApplyCorrectionToAttendance in the same tx. APPROVED stays in the CHECK enum for forward-compat but is unused by the FE-scope flow."
  - "company_id denormalized onto attendance_corrections (FK to client_companies) so leader-scope queue queries + OUTSIDE_CORRECTION_WINDOW need no JOIN; attendance_shift_date denormalized as the window-check basis."
  - "New internal/domain/attendance/ SUBPACKAGE (not a flat domain/attendance.go) per the plan's interfaces block — the openapi shapes are rich enough to warrant it; diverges from the flat package domain used by placement/scheduling but is deliberate."
  - "ids.go untouched — ATT + COR prefixes already present (confirmed)."

patterns-established:
  - "Cursor keyset on (check_in_at DESC, id) for attendance and (created_at DESC, id) for corrections, with nullable-narg filters (company_id, employee_id, service_line, *_in text[], date_from/to, exceptions bool)"
  - "*ForUpdate row-lock queries (omit JOINs; service re-reads for DTO) for verify/reject/bulk + correction approve/reject — mirrors GetScheduleEntryForUpdate"

requirements-completed: [ATT-01, ATT-02]

# Metrics
duration: 5min
completed: 2026-06-05
---

# Phase 7 Plan 01: E5 Attendance Data Layer Summary

**Two goose migrations (attendance + attendance_corrections) with FKs to the Phase-6 schedule_entries / Phase-5 placements / employees, stored-column geofence+lateness exceptions, 11 sqlc cursor/get/state-transition queries, and an internal/domain/attendance package — `make gen` / build / vet / gofmt all clean.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-06-04T18:01:09Z
- **Completed:** 2026-06-04T18:05:00Z
- **Tasks:** 2
- **Files modified:** 8 (6 created, 2 regenerated)

## Accomplishments
- `attendance` table (00026): all openapi Attendance columns, FKs to schedule_entries (E4, nullable=unscheduled), placements (E3 scope source), client_companies, employees, attendance_codes; geofence/lateness/auto-close as STORED columns; status + verification_status CHECK enums; flags text[]; two partial indexes (company+check_in_at desc, verification_status).
- `attendance_corrections` table (00027): status CHECK enum, proposed_check_in_at/check_out_at/attendance_code_id, original_snapshot jsonb, denormalized company_id + attendance_shift_date; `corrections_one_pending_per_attendance_uq` partial unique index (CORRECTION_ALREADY_PENDING backstop).
- sqlc query set for both tables (list/get/forUpdate/state-transitions) + `make gen` producing 11 Querier methods; build + vet + gofmt green, no sqlc drift on re-gen.
- `internal/domain/attendance` package: Attendance/Correction structs (nullable fields as pointers), GeofenceCheck, DiffRow, and all five enums (AttendanceStatus, VerificationStatus, Flag, CorrectionType, CorrectionStatus).

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrations 00026 (attendance) + 00027 (attendance_corrections)** - `aa85841` (feat)
2. **Task 2: sqlc queries + make gen + domain types** - `06f2179` (feat)

**Plan metadata:** (see final docs commit)

## Files Created/Modified
- `backend/db/migrations/00026_attendance.sql` - attendance records table + 2 partial indexes
- `backend/db/migrations/00027_attendance_corrections.sql` - corrections table + one-pending partial unique index + company/created index
- `backend/db/queries/attendance/attendance.sql` - 6 sqlc queries (list/get/forUpdate/verify/reject/apply-correction)
- `backend/db/queries/attendance/corrections.sql` - 5 sqlc queries (list/get/forUpdate/approve/reject)
- `backend/internal/domain/attendance/attendance.go` - Attendance + AttendanceStatus/VerificationStatus/Flag + GeofenceCheck + service-line consts
- `backend/internal/domain/attendance/correction.go` - Correction + CorrectionType/CorrectionStatus + DiffRow
- `backend/internal/repository/sqlc/{querier,models,attendance.sql,corrections.sql}.go` - generated (do NOT hand-edit)

## Decisions Made
See `key-decisions` frontmatter. Summary: stored-column geofence/lateness; ApproveCorrection marks APPLIED directly; company_id + attendance_shift_date denormalized onto corrections; new domain SUBPACKAGE per plan; ids.go untouched.

## Deviations from Plan

None - plan executed exactly as written. (gofmt -w was applied to the two new domain files to satisfy the `gofmt -l` clean gate — formatting only, no behavior change; part of the Task 2 commit.)

## Issues Encountered
- `goose` and `sqlc` CLIs are not on PATH locally; `make gen` invokes the project's pinned sqlc and ran clean (it parses the migrations as its schema, so it also validates 00026/00027). goose `validate` was skipped (non-blocking — sqlc would fail on malformed DDL).

## Reference for 07-02 (handoff)

**Tables / columns / indexes**

- `attendance` — PK `id text` (`DEFAULT 'SWP-ATT-' || swp_next_id('ATT')`). FKs: `employee_id→employees`, `placement_id→placements` (scope source), `schedule_id→schedule_entries` (NULL=unscheduled), `company_id→client_companies`, `attendance_code_id→attendance_codes`. Seeded-exception/flag columns: **`is_late`** (bool), **`late_minutes`** (int), **`auto_closed`** (bool), geofence **`in_geofence`/`in_distance_m`/`out_geofence`/`out_distance_m`/`geofence_radius_m`** (radius default 100), **`wfo`** (bool default true). Enums-as-text: `status` (PRESENT|LATE|INCOMPLETE|ABSENT|ON_LEAVE), `verification_status` (AUTO_APPROVED|PENDING|VERIFIED|REJECTED|ESCALATED), `flags text[]`. Indexes: `attendance_company_checkin_idx (company_id, check_in_at DESC) WHERE deleted_at IS NULL`, `attendance_vstatus_idx (verification_status) WHERE deleted_at IS NULL`.
- `attendance_corrections` — PK `id text` (`DEFAULT 'SWP-COR-' || swp_next_id('COR')`). FKs: `attendance_id→attendance`, `requester_id→employees`, `company_id→client_companies` (denormalized), `proposed_attendance_code_id→attendance_codes`. `status` enum (PENDING|APPROVED|APPLIED|REJECTED|CANCELLED), `original_snapshot jsonb` default `'{}'`, `attendance_shift_date date` (OUTSIDE_CORRECTION_WINDOW basis). Indexes: `corrections_one_pending_per_attendance_uq (attendance_id) WHERE status='PENDING' AND deleted_at IS NULL` (CORRECTION_ALREADY_PENDING backstop), `corrections_company_created_idx (company_id, created_at DESC) WHERE deleted_at IS NULL`.

**Querier methods (package `sqlcgen`, `internal/repository/sqlc`)**

- Attendance: `ListAttendance(ctx, ListAttendanceParams) ([]ListAttendanceRow, error)`, `GetAttendance(ctx, id string) (GetAttendanceRow, error)`, `GetAttendanceForUpdate(ctx, id string) (GetAttendanceForUpdateRow, error)`, `VerifyAttendance(ctx, VerifyAttendanceParams) (VerifyAttendanceRow, error)`, `RejectAttendance(ctx, RejectAttendanceParams) (RejectAttendanceRow, error)`, `ApplyCorrectionToAttendance(ctx, ApplyCorrectionToAttendanceParams) (ApplyCorrectionToAttendanceRow, error)`.
- Corrections: `ListCorrections(ctx, ListCorrectionsParams) ([]ListCorrectionsRow, error)`, `GetCorrection(ctx, id string) (GetCorrectionRow, error)`, `GetCorrectionForUpdate(ctx, id string) (GetCorrectionForUpdateRow, error)`, `ApproveCorrection(ctx, ApproveCorrectionParams) (ApproveCorrectionRow, error)`, `RejectCorrection(ctx, RejectCorrectionParams) (RejectCorrectionRow, error)`.

**sqlc type quirks (07-02 repository mapping must handle these)**

- **`original_snapshot jsonb` → `[]byte`** on the generated model/rows — the repo must `json.Marshal`/`Unmarshal` to/from `map[string]any` (domain `Correction.OriginalSnapshot`).
- **`attendance_shift_date date` → `pgtype.Date`** (NOT `time.Time`) — convert `<-> time.Time` at the repo boundary, same as Phase-5/6 date columns.
- **Integer columns → `int32`**: `late_minutes int32`, `worked_minutes *int32`, `in_distance_m/out_distance_m *int32`, `geofence_radius_m int32`. Domain uses plain `int`/`*int` — convert at the boundary.
- **`wfo` → field name `Wfo`** (sqlc lower-cases the all-caps; not `WFO`).
- **`flags text[]` → `[]string`** (domain is `[]Flag` — cast on map).
- List params: nullable nargs map to `*string` / `[]string` (for `*_in text[]`) / `pgtype.Date` (date_from/date_to) / `*bool` (exceptions); `page_limit` is `int32`. Keyset cursor params: `CursorCheckInAt *time.Time` + `CursorID *string` (attendance), `CursorCreatedAt *time.Time` + `CursorID *string` (corrections) — both NULL on first page.
- `ApplyCorrectionToAttendanceParams`: `CheckInAt *time.Time`, `CheckOutAt *time.Time`, `AttendanceCodeID *string`, `LastCorrectionID *string`, `ID string` (COALESCE keeps existing when narg is nil).
- List queries `LEFT JOIN employees` (→ `employee_name`/`requester_name`) and `client_companies` (→ `company_name`); those Row fields are `*string` (nullable from the LEFT JOIN).

**State-transition contract (for the 07-02 service)**

- `VerifyAttendance`/`RejectAttendance` guard `WHERE verification_status IN ('PENDING','ESCALATED')` and RETURN the row — **zero rows ⇒ terminal-state 409** (ALREADY_VERIFIED/REJECTED).
- `ApproveCorrection`/`RejectCorrection` guard `WHERE status='PENDING'` — **zero rows ⇒ 409**. Approve sets `status='APPLIED'`; call `ApplyCorrectionToAttendance` in the same tx, then set the attendance `last_correction_id` (the apply query already does this) and append the `CORRECTED` flag (handled in-query, de-duped).
- Scope: load `placement_id`→`company_id` from `GetAttendanceForUpdate` (or `co.company_id` for corrections) then `rbac.GuardCompany`. `VERIFY_OWN_RECORD` needs the record's `employee_id` (present on every Row) vs the actor's employee id.

## Next Phase Readiness
- E5 data layer compiles and is ready for the 07-02 service/handler slice: tables, FKs, indexes, sqlc Querier, and domain types all exist; geofence/lateness are stored columns (no clock pipeline).
- No endpoints, services, repository, or seed yet — those are 07-02. The bulk-verify/reject idempotency wrap reuses the platform Idempotency store (per CONTEXT); `:bulk-*` iterates the single VerifyAttendance/RejectAttendance per id for partial success (Phase-6 BulkApplyResult analog).
- Reminder for 07-02: extend the E2E `resetDb` TRUNCATE list with `attendance_corrections` (before `attendance`, before `placements`) and seed honest PENDING exceptions + cross-company (SWP-CMP-0022) + leader-own records.

## Self-Check: PASSED

All 9 created/generated files present on disk; both task commits (aa85841, 06f2179) found in git log. `make gen` clean (no sqlc drift on re-gen), `go build ./...` + `go vet ./...` exit 0, `gofmt -l` clean.

---
*Phase: 07-e5-attendance*
*Completed: 2026-06-05*
