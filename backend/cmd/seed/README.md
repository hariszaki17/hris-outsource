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
| `-demo`    | After the default seed, layer a **production-like demo dataset** on top (additive, idempotent). See [Demo profile](#demo-profile--demo) below. **Never set by the E2E harness.** |

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

---

## Demo profile (`-demo`)

`go run ./cmd/seed -demo` first runs the **default `Seed`** (personas, auth,
global master-data, the thin E2E fixtures) and **then** runs `SeedDemo`
(`demo.go`) to layer a rich, production-LIKE dataset on top — so a developer who
logs into the web console sees a populated system instead of the thin E2E rows.

The default `go run ./cmd/seed` (no flags) — the path the Playwright E2E harness
(`frontend/e2e/lib/backend.ts`) invokes — is **completely unaffected**: `SeedDemo`
is only reachable behind `-demo`. No existing function in `seed.go` was changed;
the only edit to existing code is the `-demo` flag + the conditional call in
`main.go`.

### How to run

```sh
# 1. Bring up Postgres + run migrations
docker compose up -d postgres
DATABASE_URL="postgres://hris:hris@localhost:5432/hris?sslmode=disable" \
  go run ./cmd/migrate up

# 2. Seed default + demo (payroll money is encrypted with this key)
PAYROLL_ENCRYPTION_KEY="$(head -c 32 /dev/urandom | base64)" \
DATABASE_URL="postgres://hris:hris@localhost:5432/hris?sslmode=disable" \
  go run ./cmd/seed -demo
```

`PAYROLL_ENCRYPTION_KEY` is a base64 std-encoded 32-byte AES-256 key — the SAME
key the API decrypts payslips with. If it is unset, the demo payslip step is
skipped (logged) and everything else still seeds. Use the same key for the API.

### Determinism + idempotency

- All name/variation choices come from a **fixed-seed** `math/rand` source
  (`rand.NewSource(20260605)`) → every fresh run produces identical data.
- Every INSERT is `ON CONFLICT (id) DO NOTHING` (or a `NOT EXISTS` guard for
  bigserial-PK child tables). A **sentinel short-circuit** (`SWP-CMP-1001`
  present → log + return) makes re-runs a no-op fast path.
- All real DB constraints are honored, never disabled: **INV-1** (≤1 active
  placement per agent), **EA-2** (≤1 active agreement per employee), **INV-2**
  (≤1 active leader per company), **INV-4** (leader is actively placed at the
  company), **INV-5** (site belongs to the placement's company), the
  schedule_entries partial-unique on `(employee_id, work_date)` (agents are
  NEVER double-booked), and every CHECK enum + FK.

### ID bands (DISJOINT from all `seed.go` E2E literals)

| Entity                     | Demo band                         |
|----------------------------|-----------------------------------|
| `client_companies`         | `SWP-CMP-1001` … `SWP-CMP-1008`   |
| `client_sites`             | `SWP-SITE-2001` …                 |
| `service_lines`            | reuses global `SWP-SVC-001/002/003` |
| `positions`                | `SWP-POS-201` … `SWP-POS-206`     |
| `employees` (agents)       | `SWP-EMP-20001` … `SWP-EMP-20120` |
| `employment_agreements`    | `SWP-AG-20001` … `SWP-AG-20120`   |
| `placements`               | `SWP-PL-20001` …                  |
| `shift_leader_assignments` | `SWP-SLA-2001` … `SWP-SLA-2008`   |
| `shift_masters`            | `SWP-SHF-201` … `SWP-SHF-203`     |
| `schedule_entries`         | `SWP-SCH-2xxxxx`                  |
| `attendance`               | `SWP-ATT-2xxxxx`                  |
| `attendance_corrections`   | `SWP-COR-2xxx`                    |
| `leave_requests`           | `SWP-LR-21xxx`                    |
| `leave_quotas`             | `SWP-LQ-2xxx`                     |
| `overtime`                 | `SWP-OT-22xxx`                    |
| `holidays`                 | `SWP-HOL-201` … `SWP-HOL-204`     |
| `payslips`                 | `SWP-PS-23xxx`                    |
| `notifications`            | `SWP-NTF-24xxx`                   |

These literal high-band ids never advance the `swp_next_id` cursor (the demo
supplies explicit ids), so runtime-allocated ids stay well below them — exactly
the pattern `seed.go` already uses (e.g. `SWP-LR-44210`).

### Volumes (verified against a live Postgres — these are the DEMO additions)

| Table                      | Demo rows | Notes |
|----------------------------|-----------|-------|
| `client_companies`         | 8         | Facility / Building / Parking lines |
| `client_sites`             | 24        | 2–4 per company, Jakarta-area geofences |
| `employees`                | 120       | Indonesian names/NIKs; one active agreement each (PKWT + PKWTT) |
| `employment_agreements`    | 120       | ~35% PKWTT indefinite, rest PKWT fixed-term |
| `placements`               | 120       | 104 ACTIVE (13 expiring ≤30d), 16 terminal/history |
| `shift_leader_assignments` | 8         | exactly one active leader per company, placed there |
| `schedule_entries`         | 3 120     | 30 days × ~104 active agents, 3-shift 24/7 rotation, no double-book |
| `attendance`               | 1 211     | 14 days back; ~70% AUTO_APPROVED, PENDING exceptions, some VERIFIED |
| `attendance_corrections`   | 28        | PENDING CHECK_OUT corrections on auto-closed records |
| `leave_quotas`             | 60        | realistic total/used/pending |
| `leave_requests`           | 40        | PENDING / APPROVED / REJECTED (E11 single-level engine status) |
| `holidays`                 | 4         | NATIONAL + CUSTOM |
| `overtime`                 | 50        | all states (E11 single-level PENDING) × WORKDAY/RESTDAY/HOLIDAY tiers |
| `payslips`                 | 80        | 2 periods (2026-04/05) × 40 agents, FINAL, money AES-256-GCM encrypted |
| `payslip_components`       | 480       | 6 lines per payslip |
| `payslip_benefits`         | 160       | 2 employer benefits per payslip |
| `notifications`            | 80        | 6 kinds, 50/50 read/unread, persona + demo-leader recipients |
| `audit_log`                | 15        | placement/leave/marker rows for populated dashboards |

(Totals in the DB are these plus the small E2E fixture set already created by the
default `Seed`.) Verified live: all of INV-1/EA-2/INV-2/INV-4/INV-5 and the
schedule no-double-book check returned **0 violations**, and the encrypted
payslip money decrypts cleanly under `PAYROLL_ENCRYPTION_KEY`.
