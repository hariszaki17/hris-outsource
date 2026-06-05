---
phase: 02-e1-foundations
plan: "01"
subsystem: database
tags: [sqlc, postgres, goose, migrations, platform_settings, audit_log, users]

# Dependency graph
requires:
  - phase: 01-test-harness-auth
    provides: "users + audit_log tables, sqlc config, ids package, existing identity queries"
provides:
  - "platform_settings migration 00008 with 7 locked v1 seed rows"
  - "ListUsers, UpdateUserEmail, ChangeUserRole, SetUserStatus sqlc query methods (users management)"
  - "ListAuditLog, GetAuditLogByID sqlc query methods (audit-log read)"
  - "ListPlatformSettings sqlc query method (platform settings read)"
  - "foundations query package at backend/db/queries/foundations/"
affects: [02-e1-foundations wave-2 services+handlers, 02-e1-foundations wave-3 contract tests]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Foundations query package separate from identity: db/queries/foundations/ picked up automatically by sqlc glob db/queries/*"
    - "Cursor pagination pattern for list queries: (cursor_created_at, cursor_id) IS NULL OR (col < cursor) idiom"
    - "Optional filter pattern: (sqlc.narg(x)::type IS NULL OR col = sqlc.narg(x)) for all nullable filters"

key-files:
  created:
    - backend/db/migrations/00008_platform_settings.sql
    - backend/db/queries/foundations/audit_log.sql
    - backend/db/queries/foundations/platform_settings.sql
    - backend/internal/repository/sqlc/audit_log.sql.go
    - backend/internal/repository/sqlc/platform_settings.sql.go
  modified:
    - backend/db/queries/identity/users.sql
    - backend/internal/repository/sqlc/users.sql.go
    - backend/internal/repository/sqlc/querier.go

key-decisions:
  - "ids.go unchanged — platform_settings keys are plain text, not SWP-prefixed; USR and AL prefixes already existed"
  - "foundations/ query package created under db/queries/ — sqlc glob db/queries/* picks up subdirectories automatically"
  - "platform_settings stored as flat key/value table (key PK, value, label, locked, sort) matching the openapi response shape"
  - "ListAuditLog returns []AuditLog (full row type, all columns) since audit_log has no nullable column conflicts"

patterns-established:
  - "New epic query packages: db/queries/<epic>/ — sqlc picks them up without sqlc.yaml changes"
  - "Cursor list queries always fetch limit+1 and use (col, id) < (cursor_col, cursor_id) for stable pagination"

requirements-completed: [FND-01, FND-02, FND-03]

# Metrics
duration: 2min
completed: "2026-06-04"
---

# Phase 02 Plan 01: E1 Data Layer — platform_settings + E1 sqlc Query Methods

**Migration 00008 + sqlc foundations package delivering ListUsers/ListAuditLog/ListPlatformSettings and 4 user-management query methods for wave-2 service layer consumption**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-06-04T00:34:48Z
- **Completed:** 2026-06-04T00:36:49Z
- **Tasks:** 2
- **Files modified:** 7 (2 new migration/query, 3 new generated, 2 extended)

## Accomplishments

- Created `backend/db/migrations/00008_platform_settings.sql` (goose Up/Down) with 7 locked v1 rows seeded: locale, timezone, date_format, currency, version, stack, legacy_data_source
- Appended 4 user-management queries to `identity/users.sql` (ListUsers with cursor+filters, UpdateUserEmail, ChangeUserRole, SetUserStatus — all RETURNING the full management row)
- Created `foundations/` query package: ListAuditLog (7 optional filters + cursor pagination) + GetAuditLogByID; ListPlatformSettings (sorted by `sort ASC`)
- `make gen` + `go build ./... && go vet ./...` all clean; original 5 identity queries intact

## Generated Method Signatures (wave-2 consumes verbatim)

### ListUsers

```go
type ListUsersParams struct {
    Role            *string
    Status          *string
    CompanyID       *string
    Q               *string
    CursorCreatedAt *time.Time
    CursorID        *string
    RowLimit        int32
}

type ListUsersRow struct {
    ID          string
    Email       string
    Role        string
    EmployeeID  *string
    CompanyID   *string
    Status      string
    FullName    string
    LastLoginAt *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

func (q *Queries) ListUsers(ctx context.Context, arg ListUsersParams) ([]ListUsersRow, error)
```

### UpdateUserEmail / ChangeUserRole / SetUserStatus

All three return the same row type (no password_hash, no deleted_at — management view):

```go
type UpdateUserEmailParams struct { Email string; ID string }
type ChangeUserRoleParams  struct { Role  string; ID string }
type SetUserStatusParams   struct { Status string; ID string }

// Common return type (name varies: UpdateUserEmailRow / ChangeUserRoleRow / SetUserStatusRow)
struct {
    ID, Email, Role string
    EmployeeID, CompanyID *string
    Status   string
    FullName string
    LastLoginAt *time.Time
    CreatedAt, UpdatedAt time.Time
}
```

### ListAuditLog

```go
type ListAuditLogParams struct {
    ActorUserID     *string
    Action          *string
    EntityType      *string
    EntityID        *string
    CreatedGte      *time.Time
    CreatedLte      *time.Time
    Q               *string
    CursorCreatedAt *time.Time
    CursorID        *string
    RowLimit        int32
}

func (q *Queries) ListAuditLog(ctx context.Context, arg ListAuditLogParams) ([]AuditLog, error)
// Returns the full AuditLog row type (all 10 columns per the audit_log table)
```

### GetAuditLogByID

```go
func (q *Queries) GetAuditLogByID(ctx context.Context, id string) (AuditLog, error)
```

### ListPlatformSettings

```go
type ListPlatformSettingsRow struct {
    Key    string
    Value  string
    Label  string
    Locked bool
}

func (q *Queries) ListPlatformSettings(ctx context.Context) ([]ListPlatformSettingsRow, error)
// Sorted by sort ASC; wave-2 maps rows to the PlatformSettings response object (key→{value,label,locked})
```

## Task Commits

1. **Task 1: platform_settings migration + ids prefix** - `a67e04b` (chore)
2. **Task 2: users management queries + foundations query package** - `23d8622` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified

- `backend/db/migrations/00008_platform_settings.sql` — goose migration; CREATE TABLE + 7 INSERT rows; DROP TABLE in Down
- `backend/db/queries/identity/users.sql` — appended ListUsers, UpdateUserEmail, ChangeUserRole, SetUserStatus
- `backend/db/queries/foundations/audit_log.sql` — ListAuditLog + GetAuditLogByID
- `backend/db/queries/foundations/platform_settings.sql` — ListPlatformSettings
- `backend/internal/repository/sqlc/users.sql.go` — regenerated (4 new methods)
- `backend/internal/repository/sqlc/audit_log.sql.go` — new generated file
- `backend/internal/repository/sqlc/platform_settings.sql.go` — new generated file

## Decisions Made

- **ids.go unchanged** — platform_settings rows are addressed by plain text key (not SWP-prefixed); no new ID prefix needed. USR and AL already in place.
- **foundations/ query package** — created at `db/queries/foundations/` as a sibling to `identity/`. The sqlc glob `db/queries/*` picks up new subdirectories automatically without modifying sqlc.yaml.
- **platform_settings shape** — flat key/value table with `sort int` column for ordered response. Wave-2 handler maps the slice into a `map[string]SettingEntry` matching the openapi PlatformSettings object.
- **ListAuditLog returns `[]AuditLog`** — the full row type (no projected subset needed); all 10 columns returned.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Wave-2 (services + handlers) can now consume all 7 new Querier methods directly. The generated param structs match the OpenAPI filter parameters exactly. The platform_settings migration will be applied during the wave-2 E2E seeding step via `go run ./cmd/migrate up`.

---
*Phase: 02-e1-foundations*
*Completed: 2026-06-04*
