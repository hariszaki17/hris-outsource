# cmd/seed

Deterministic seed command for the SWP HRIS dev/test stack.

## What it does

Inserts the four demo personas into the `users` table with known argon2id-hashed
passwords. The seed is **idempotent**: re-running against a populated DB skips any
user whose email already exists — no error, no duplicate.

## Quick start

```sh
# 1. Run migrations first (creates the users table + id allocator)
DATABASE_URL="postgres://..." go run ./cmd/migrate up

# 2. Seed personas
DATABASE_URL="postgres://..." go run ./cmd/seed
```

## Flags

| Flag       | Description |
|------------|-------------|
| `-genkeys` | Print a fresh Ed25519 keypair (base64 std) and exit. No DB connection needed. |

### `-genkeys` output format

```
<private-key-base64>   ← AUTH_JWT_PRIVATE_KEY (64 raw bytes, base64 std)
<public-key-base64>    ← AUTH_JWT_PUBLIC_KEY  (32 raw bytes, base64 std)
```

The E2E harness (`lib/backend.ts` in the Playwright project) reads these two lines
and exports them as `AUTH_JWT_PRIVATE_KEY` / `AUTH_JWT_PUBLIC_KEY`.

## Personas

| Email                  | Role          | Password (constant)                   | Employee ID   | Company ID   |
|------------------------|---------------|---------------------------------------|---------------|--------------|
| `sari.hadi@swp.test`   | `hr_admin`    | `PasswordHRAdmin` = `Pass1ng-Garuda!` | SWP-EMP-1042  | —            |
| `rudi.wijaya@swp.test` | `shift_leader`| `PasswordShiftLeader` = `Lead3r-Senayan!` | SWP-EMP-1108 | SWP-CMP-0021 |
| `super.admin@swp.test` | `super_admin` | `PasswordSuperAdmin` = `Sup3r-Admin-2026!` | —        | —            |
| `agent.budi@swp.test`  | `agent`       | `PasswordAgent` = `Ag3nt-Budi-2026!` | SWP-EMP-2891  | —            |

Password constants are exported from `seed.go`:

```go
seed.PasswordHRAdmin      // "Pass1ng-Garuda!"
seed.PasswordShiftLeader  // "Lead3r-Senayan!"
seed.PasswordSuperAdmin   // "Sup3r-Admin-2026!"
seed.PasswordAgent        // "Ag3nt-Budi-2026!"
```

### Shift leader scope note

`rudi.wijaya@swp.test` carries `company_id = SWP-CMP-0021` (literal). This is the
deterministic company ID for "Plaza Senayan" per the harness spec. The `companies`
table lands in Phase 3; the FK is not enforced until that migration runs.

## Idempotency

For each persona, `Seed` calls `GetUserByEmail` before `CreateUser`. If a
non-deleted row exists with the same email it is skipped. This means:

- Safe to run after `make migrate-up` on an empty DB.
- Safe to run again after a partial failure — only missing rows are inserted.
- Does NOT update passwords or roles of existing rows. To reset, truncate `users`
  and re-run migrations.

## Extending the seed (per-phase rule)

Each new phase that adds fixtures for its screens appends to the `Seed` function
in `seed.go` (or imports from a phase-specific file). The convention:

```go
// Phase markers:
// Phase 1: four core personas (above)
// Phase N: add <describe fixture> — company "Plaza Senayan" row, employee
//           records, placements, etc.
```

Keep the extension block below the persona loop so the base personas are always
seeded first.
