# Phase 7: E5 Attendance - Context

**Gathered:** 2026-06-05 (autonomous — recommended decisions auto-accepted per user's overnight directive)
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the FE-used E5 "attendance" endpoints against the real BE and wire the screens off
MSW, proven with exhaustive full-stack Playwright E2E. The web surface is **exceptions-only
shift-leader verification + corrections review** — NOT clock-in/out (that is mobile/agent-only,
out of scope). Delivers: attendance list (cursor/virtualized) + detail, single verify/reject,
bulk verify/reject (partial success + idempotency), and the corrections queue (list/detail/
approve/reject). Attendance records link to the Phase-6 schedule entry + Phase-5 placement.
Since clock-in/out (mobile) is not built, attendance + correction records are **seeded directly**
so the web verification flows have real exception records to act on. Overtime (E7), payroll (E8),
and reporting (E10) consume verified attendance later.
</domain>

<decisions>
## Implementation Decisions

### Scope = the 10 FE-used hooks ONLY (fe-endpoint-inventory.md E5)
- Attendance: `GET /attendance`, `GET /attendance/{id}`, `POST /attendance/{id}:verify`,
  `:reject`, `POST /attendance:bulk-verify`, `POST /attendance:bulk-reject`.
- Corrections: `GET /corrections`, `GET /corrections/{id}`, `POST /corrections/{id}:approve`,
  `:reject`.
- **OUT of scope (not FE-web):** clock-in/out (`POST /attendance:clock-in`/`:clock-out`),
  agent self-service, geofence evaluation at clock time, auto-close job. Geofence/lateness are
  represented as **stored properties on seeded records** (in_geofence_in/out, is_late,
  auto_closed) so the verification UI has real exceptions; we do NOT implement the mobile clock
  pipeline.

### State machines & rules (per E5 contract + FEATURE INV-1..4)
- **attendance.verification_status:** AutoApproved | Pending | Verified | Rejected. Only
  `Pending` (exception) records are verifiable. Verify → Verified; reject (reason required) →
  Rejected. Acting on a terminal record (Verified/Rejected/Corrected) → 409 (ALREADY VERIFIED/
  REJECTED/CORRECTED per the contract codes).
- **INV-3 exceptions-only:** a record is `Pending` iff is_late OR out-of-geofence OR auto_closed
  OR missing clock-in/out OR attendance-code needs_verification; clean records are `AutoApproved`
  and NOT in the verification queue. (We seed both kinds; the queue lists Pending only.)
- **Scope (INV / RBAC §17):** shift_leader verifies only their **own company's** records
  (`rbac.GuardCompany` via the record's placement→company) → 403 `OUT_OF_SCOPE` cross-company.
  `VERIFY_OWN_RECORD` → 422/409 (per contract) when a leader tries to verify an attendance
  record that is their own. HR/super_admin any company.
- **Bulk verify/reject (`POST /attendance:bulk-verify` / `:bulk-reject`):** take a list of ids,
  return **per-id partial success** (verified / skipped-with-code) per the openapi bulk envelope;
  **idempotent** via the platform Idempotency store (Idempotency-Key header) — replay returns the
  same result; `IDEMPOTENCY_KEY_REUSED` on key reused with a different payload (CONVENTIONS §
  idempotency). Each successful verify/reject writes audit + notify stub.
- **Corrections.status:** Pending | Approved | Rejected | Applied. Approve a Pending correction →
  applies the proposed change to the target attendance record (or marks Applied) + sets status;
  reject (reason) → Rejected. `CORRECTION_ALREADY_PENDING` (one open correction per record),
  `OUTSIDE_CORRECTION_WINDOW` (422 — correction filed/approved outside the allowed window),
  rule violations → 422 with the contract code. Approving applies whitelisted fields only.
- **Audit + notify (success criterion 3):** every verify/reject/approve/reject writes an
  audit_log row in-tx and fires a notification **stub** (TODO Phase-11), per the Phase-4/5/6
  pattern.

### Build approach (mirror Phase-5/6 slice EXACTLY)
- migration → sqlc (`make gen`) → repository → service (apperr codes, audit, GuardCompany scope,
  Idempotency wrap on bulk) → hand-written chi handlers → routes in server.go under RequireRole →
  Go contract tests → FE wiring (MSW off) + live Playwright E2E. Match
  `docs/api/E5-attendance/openapi.yaml` byte-for-byte. Cursor pagination + filters (§11).
- New migrations: `attendance` (records) + `attendance_corrections`. FKs to schedule_entries
  (E4), placements (E3), employees. SWP IDs: check ids.go for ATT/COR (or per CONVENTIONS entity
  table); add prefix only if missing. New query dir `backend/db/queries/attendance/`.
- action-suffix routes (`:verify`, `:reject`, `:bulk-verify`, `:bulk-reject`, `:approve`).

### Seed (in 07-02)
- Several attendance records for the seeded placements (Phase-5 SWP-PL-5001..5004) on known
  dates: at least one clean `AutoApproved`, several `Pending` exceptions (is_late,
  out-of-geofence, auto_closed) so the verification queue + detail + single + bulk flows have
  real targets; one record owned by the shift-leader persona to exercise `VERIFY_OWN_RECORD`;
  records at SWP-CMP-0022 to exercise cross-company `OUT_OF_SCOPE`.
- A couple of `Pending` corrections (incl. one within and one outside the correction window) so
  the corrections queue + approve/reject + OUTSIDE_CORRECTION_WINDOW are testable.
- **TZ note:** date fixtures use clearly-in-range Asia/Jakarta dates (Phase-5/6 TZ-boundary finding).

### Plan split (4 plans, mirrors ROADMAP)
- **07-01** Migrations + sqlc + domain (`attendance`, `attendance_corrections`).
- **07-02** Services + handlers: verify/reject, bulk (partial success + idempotency), corrections
  approve/reject, scope guards, audit, notify stub, seed.
- **07-03** Go contract tests vs E5 openapi (verify/reject, bulk partial-success + idempotency
  replay, OUT_OF_SCOPE, VERIFY_OWN_RECORD, OUTSIDE_GEOFENCE/OUTSIDE_CORRECTION_WINDOW 422, terminal-state 409).
- **07-04** Full-stack Playwright E2E under NEW frontend/e2e/tests/e5/ (per Gherkin AC: list/detail,
  single verify/reject, bulk partial success, corrections approve/reject, scope 403, 422 codes).
  Selectors derived from the REAL e5-attendance components.

### Claude's Discretion
- Whether geofence/lateness flags are plain stored columns vs a small evaluation helper at seed
  time — pick the simplest that yields honest Pending exceptions.
- Correction "apply on approve" mechanics (mutate target now vs mark Applied) — keep state
  correct; match the contract; note any stub.
- Bulk envelope exact grouping/shape — match the openapi example.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel (httpx cursor/PageResponse, rbac roles + `GuardCompany`, audit, apperr +
  `error.details` envelope, ids, **Idempotency store** — already used elsewhere; reuse for bulk,
  db.TxManager, i18n, Asia/Jakarta TZ).
- **Reference slices = Phase-6 scheduling (bulk partial-success envelope, conflict codes, scope)
  and Phase-5 placement (lifecycle state machine, GuardCompany, ConflictWithDetails, audit-in-tx,
  seed).** Bulk-apply partial success in `internal/service/scheduling` is the closest analog for
  bulk-verify/reject.
- E2E harness: real stack + resetDb (add attendance + corrections tables to TRUNCATE) + loginAs
  PERSONAS.* + `window.__swp_get_token__` + waitForToken + e3/e4-helpers (apiAs, pickCombobox).
  Existing E2E layout `frontend/e2e/tests/{e1,e2,e3,e4,smoke}/` → add `e5/`.

### Established Patterns
- Bulk partial success → per-item result list (Phase-6 BulkApplyResult). Idempotency-Key wrap.
  apperr.Conflict()/ConflictWithDetails for 409+details; apperr struct literal for non-default
  status (e.g. 422). Notification dispatch stubbed (TODO Phase-11). FE conflict/error details read
  from `error.details` (NOT `conflict_details` — recurring FE bug; fix toward contract). DataTable
  rows `div.border-b`; toggles `role=switch`; `.js` E2E imports; PERSONAS.* objects.

### Integration Points
- New `backend/db/queries/attendance/` (sqlc glob). Routes in server.go authenticated group under
  RequireRole (verify/reject: shift_leader scoped + hr/super; corrections similar). Seed extension.
  FE screens exist (e5-attendance/*, built from .pen) calling `@swp/api-client` e5 hooks via MSW —
  wire to real BE. E2E under new frontend/e2e/tests/e5/. resetDb must TRUNCATE the new tables.
</code_context>

<specifics>
## Specific Ideas
- The verification screen (attendance-verification-screen.tsx) + dashboard + detail are the
  primary surfaces — E2E drives REAL selectors/overlays, not invented ones.
- Bulk E2E must assert REAL partial success: select multiple records where some verify and at
  least one is skipped with a code (e.g. already-terminal or out-of-scope), + idempotency replay.
- VERIFY_OWN_RECORD E2E: shift-leader cannot verify their own seeded record.
- OUT_OF_SCOPE E2E: shift-leader cannot verify/list another company's records (403).
- Corrections E2E: approve a Pending correction (target updates), reject another; one outside the
  correction window → 422 OUTSIDE_CORRECTION_WINDOW.
</specifics>

<deferred>
## Deferred Ideas
- Clock-in/out (mobile), geofence evaluation at clock time, auto-close job, agent self-service.
- Notification dispatch implementation (stubbed; Phase-11).
- Overtime/payroll/reporting consumption of verified attendance — later phases.
</deferred>

---

*Phase: 07-e5-attendance*
*Context gathered: 2026-06-05 (autonomous)*
