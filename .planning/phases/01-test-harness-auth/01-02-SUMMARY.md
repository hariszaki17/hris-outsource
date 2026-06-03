---
phase: 01-test-harness-auth
plan: 02
subsystem: testing
tags: [go, seed, argon2id, ed25519, jwt, postgres, sqlc]

# Dependency graph
requires:
  - phase: 01-test-harness-auth
    provides: "users table migration (00002_users.sql), sqlcgen.CreateUser + GetUserByEmail, auth.HashPassword + auth.GenerateKeypair"
provides:
  - "backend/cmd/seed: deterministic four-persona seeder with known argon2id passwords"
  - "Exported password constants (PasswordHRAdmin, PasswordShiftLeader, PasswordSuperAdmin, PasswordAgent) for harness reference"
  - "-genkeys flag prints two-line base64 Ed25519 keypair (priv 64B / pub 32B)"
  - "Idempotent Seed() function extendable per phase"
affects:
  - 01-03
  - 01-04
  - 01-05

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "cmd/seed follows run() error wrapper pattern (same as cmd/migrate)"
    - "Idempotent seed: GetUserByEmail check before CreateUser, skip-if-exists"
    - "Exported password constants co-locate source-of-truth with the seeder"
    - "Phase-marker comments signal where later phases extend Seed()"

key-files:
  created:
    - backend/cmd/seed/main.go
    - backend/cmd/seed/seed.go
    - backend/cmd/seed/README.md
  modified: []

key-decisions:
  - "shift_leader company_id = SWP-CMP-0021 literal (FK not enforced until Phase 3 companies migration)"
  - "Sequential inserts (no transaction wrapping) — simplest match for skip-if-exists idempotency loop"
  - "Exported password constants in seed.go rather than a separate constants file so harness imports the seed package"

patterns-established:
  - "Per-phase seed extension: append fixtures below the persona loop with a Phase N comment marker"
  - "-genkeys prints exactly two bare base64 lines (no labels) — harness contract"

requirements-completed: [HARN-02]

# Metrics
duration: 2min
completed: 2026-06-03
---

# Phase 1 Plan 02: cmd/seed — Deterministic Persona Seeder Summary

**`backend/cmd/seed` seeds four named personas (hr_admin Sari Hadi, shift_leader Rudi Wijaya @ SWP-CMP-0021, super_admin, agent) with known argon2id passwords via sqlcgen, idempotently; `-genkeys` prints a parseable two-line base64 Ed25519 keypair for the E2E harness.**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-06-03T23:09:02Z
- **Completed:** 2026-06-03T23:11:03Z
- **Tasks:** 2
- **Files modified:** 3 (created)

## Accomplishments

- `cmd/seed/main.go`: `-genkeys` flag prints private (64 B) + public (32 B) Ed25519 keys as two bare base64 lines; default mode loads DATABASE_URL, opens pgx pool, calls Seed()
- `cmd/seed/seed.go`: four personas with role-correct attributes, exported password constants, idempotent GetUserByEmail check, phase-marker comments for per-phase extension
- `cmd/seed/README.md`: documents persona table (email/password/role/IDs), `-genkeys` output format, idempotency behavior, and the per-phase extend rule
- `go build ./... && go vet ./cmd/seed` both exit 0; `-genkeys` private key decodes to 64 bytes, public key to 32 bytes

## Persona Reference

| Email                  | Role          | Password Constant      | Plaintext Password    | Employee ID   | Company ID   |
|------------------------|---------------|------------------------|-----------------------|---------------|--------------|
| `sari.hadi@swp.test`   | `hr_admin`    | `PasswordHRAdmin`      | `Pass1ng-Garuda!`     | SWP-EMP-1042  | —            |
| `rudi.wijaya@swp.test` | `shift_leader`| `PasswordShiftLeader`  | `Lead3r-Senayan!`     | SWP-EMP-1108  | SWP-CMP-0021 |
| `super.admin@swp.test` | `super_admin` | `PasswordSuperAdmin`   | `Sup3r-Admin-2026!`   | —             | —            |
| `agent.budi@swp.test`  | `agent`       | `PasswordAgent`        | `Ag3nt-Budi-2026!`    | SWP-EMP-2891  | —            |

## `-genkeys` Output Format

```
<private-key-base64>   ← line 1: AUTH_JWT_PRIVATE_KEY (base64 std, 64 raw bytes)
<public-key-base64>    ← line 2: AUTH_JWT_PUBLIC_KEY  (base64 std, 32 raw bytes)
```

No labels, no extra output. Harness parses `line[0]` / `line[1]` directly.

## Idempotency Mechanism

For each persona, `Seed()` calls `q.GetUserByEmail(ctx, email)`:
- `pgx.ErrNoRows` → user does not exist → hash password + `q.CreateUser(...)`
- Any other return → user exists → log skip, continue loop

The shift_leader's `company_id = "SWP-CMP-0021"` is a deterministic literal matching the harness spec. The `companies` table and its FK constraint land in Phase 3; no constraint violation occurs with Phase 1 migrations.

## Task Commits

Each task was committed atomically:

1. **Task 1: cmd/seed entrypoint with -genkeys flag and DB wiring** — `c1bc6d1` (feat)
2. **Task 2: Seed the four personas + minimal fixtures, idempotently** — `7407d26` (feat)

**Plan metadata:** (see final docs commit below)

## Files Created/Modified

- `backend/cmd/seed/main.go` — Entrypoint: `-genkeys` flag + DB wiring + `run() error` wrapper
- `backend/cmd/seed/seed.go` — `Seed()` function + exported password constants + four personas
- `backend/cmd/seed/README.md` — Persona table, `-genkeys` format, idempotency docs, per-phase extension rule

## Decisions Made

- **shift_leader company_id literal**: Used `SWP-CMP-0021` directly (no companies table in Phase 1; FK not enforced). Phase 3 seeds the actual company row.
- **Sequential inserts, no transaction**: Skip-if-exists loop is naturally sequential; wrapping in a transaction would complicate the idempotency check (checking and inserting in the same tx is safe but unnecessary complexity here).
- **Exported constants in seed.go**: Password constants live in the seeder package rather than a separate `testdata` package, keeping the source of truth co-located with the hashing logic.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. Seed runs against any migrated DATABASE_URL.

## Next Phase Readiness

- `cmd/seed` is ready for 01-01 globalSetup to call `go run ./cmd/seed` after migrations
- Password constants are importable by the harness persona registry (01-04 / 01-05)
- The phase-marker comment block in `seed.go` makes the extension point for Phase 3+ fixtures explicit

---
*Phase: 01-test-harness-auth*
*Completed: 2026-06-03*

## Self-Check: PASSED

- `backend/cmd/seed/main.go` exists: FOUND
- `backend/cmd/seed/seed.go` exists: FOUND
- `backend/cmd/seed/README.md` exists: FOUND
- commit `c1bc6d1` exists: FOUND
- commit `7407d26` exists: FOUND
