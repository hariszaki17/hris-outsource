# Phase 9: E7 Overtime - Context

**Gathered:** 2026-06-05 (autonomous — recommended decisions auto-accepted per user's overnight directive)
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the FE-used E7 "overtime" endpoints against the real BE and wire the screens off MSW,
proven with exhaustive full-stack Playwright E2E. The web surface is **HR/leader OT approval +
holiday-calendar management** — NOT agent OT capture/confirm-from-mobile (that is mobile/agent;
OT records are seeded incl. auto-detected candidates so the web confirm/approval flows have real
targets). Delivers: overtime list/detail + the workflow state machine (confirm → L1 → final, plus
reject and withdraw), bulk approve/reject (partial success), business-rule enforcement (e.g. OT
below `min_minutes` → 422), and the public-holiday calendar CRUD (which feeds OT day_type
classification). OT is recorded as **hours/minutes only** — multipliers are stored as reference,
**no monetary calc in v1** (INV-2). `overtime_rules` already exist (E2/Phase-3).
</domain>

<decisions>
## Implementation Decisions

### Scope = the 13 FE-used hooks ONLY (fe-endpoint-inventory.md E7)
- Overtime: `GET /overtime`, `GET /overtime/{id}`, `POST /overtime/{id}:confirm`, `:approve-l1`,
  `:approve-final`, `:reject`, `:withdraw`, `POST /overtime:bulk-approve`, `:bulk-reject`.
- Holidays: `GET /holidays`, `POST /holidays`, `PATCH /holidays/{id}`, `DELETE /holidays/{id}`.
- `useListOvertimeRules` is REUSED from E2 (already implemented Phase-3 — do NOT reimplement).
- **OUT of scope (not FE-web):** agent OT request/capture + the auto-detection job from verified
  attendance (mobile/system). OT records (incl. PENDING_AGENT_CONFIRM auto-detected candidates)
  are seeded directly so the web confirm/approval flows have real targets.

### Overtime workflow state machine (per E7 contract + FEATURE INV-3/4)
- States (the openapi enum is authoritative — match exactly): a candidate may start
  `PENDING_AGENT_CONFIRM` → (`:confirm`) → `PENDING_L1`/Pending → (`:approve-l1`) → `PENDING_HR`/
  LeaderApproved → (`:approve-final`) → `APPROVED`. `:reject` (reason) at either level → REJECTED.
  `:withdraw` → WITHDRAWN. Acting on a terminal record (APPROVED/REJECTED/WITHDRAWN/CANCELLED) or
  a wrong-state transition → 409 (per contract code). INV-4: a candidate never counts until APPROVED.
- **Scope (RBAC §17):** shift_leader is L1 approver for their **own company** only → 403
  `OUT_OF_SCOPE` cross-company; `SELF_APPROVAL_FORBIDDEN` when an approver acts on their own OT.
  HR/super = final + bulk + holidays. Each transition writes an approval/decision trail row.
- **Bulk approve/reject (`POST /overtime:bulk-approve` / `:bulk-reject`):** per-id partial success
  ({succeeded, failed}) per the openapi envelope; each success audited + notify stub. (Idempotency
  via the platform store if the contract specifies an Idempotency-Key; mirror Phase-7 bulk.)

### Business rules (per E7 contract + INV-1/5)
- **OT_BELOW_MIN (422):** OT minutes below the applicable rule's `min_minutes` → not counted /
  blocked with the contract code + field errors (INV-5). Rule lookup uses the existing
  `overtime_rules` (E2), matched by day_type (+ optional service_line).
- **day_type classification (INV-1):** each OT record is Workday | RestDay | Holiday, derived from
  the schedule (E4) + the **public-holiday calendar** (this phase). Creating/deleting a holiday
  changes classification — model the dependency; `HOLIDAY_DATE_CLASH` (duplicate holiday date) and
  `HOLIDAY_IN_USE` (delete a holiday referenced by OT/schedule) per the contract.
- `OT_NO_SCHEDULED_SHIFT`, `OT_OVERLAPS_LEAVE` per the contract where the spec models them.
- **Multipliers stored as reference, NOT applied (INV-2):** record hours/minutes only.

### Holiday calendar (CRUD)
- `GET /holidays` (list, cursor + year/range filter), `POST` (create — `HOLIDAY_DATE_CLASH` on dup
  date), `PATCH` (update name/date), `DELETE` (`HOLIDAY_IN_USE` guard). A small master table.
  HR/super manage. Match the openapi shape exactly (what holiday-overlays.tsx renders).

### Audit + notify (success criterion 3)
- Every confirm/approve-l1/approve-final/reject/withdraw + bulk + holiday create/update/delete
  writes an audit_log row in-tx and fires a notification **stub** (TODO Phase-11), per Phase-4..8.

### Build approach (mirror Phase-7/8 slice EXACTLY)
- migration → sqlc (`make gen`) → repository → service (apperr codes, audit, GuardCompany scope,
  *ForUpdate guards, bulk partial-success) → hand-written chi handlers → routes in server.go under
  RequireRole → Go contract tests → FE wiring (MSW off) + live Playwright E2E. Match
  `docs/api/E7-overtime/openapi.yaml` byte-for-byte. Cursor pagination + filters (§11). New
  migrations: `overtime` (records) + `holidays`. FKs to attendance/schedule/placements/employees/
  overtime_rules. SWP IDs: check ids.go for OT/HOL (or per CONVENTIONS); add prefixes only if
  missing. New query dir `backend/db/queries/overtime/`. action-suffix routes.

### Seed (in 09-02)
- Overtime records for the seeded placements (Phase-5 SWP-PL-5001..5004): a PENDING_AGENT_CONFIRM
  auto-detected candidate (confirm target), a PENDING_L1 at SWP-CMP-0021 (Rudi leader L1 target),
  a PENDING_HR (final target), an over-an approver's own record (SELF_APPROVAL_FORBIDDEN), a
  CMP-0022 record (OUT_OF_SCOPE target), a below-min record (OT_BELOW_MIN), terminal ones for list
  filters, and at least one classified as Holiday (using a seeded holiday) and one RestDay.
- A couple of public holidays (one used by an OT record for HOLIDAY_IN_USE, one free for delete).
- **TZ note:** clearly-in-range Asia/Jakarta dates (Phase-5..8 TZ-boundary finding).

### Plan split (4 plans, mirrors ROADMAP)
- **09-01** Migrations + sqlc + domain (`overtime`, `holidays`).
- **09-02** Services + handlers: OT workflow state machine (confirm/l1/final/reject/withdraw),
  bulk approve/reject (partial success), OT_BELOW_MIN + day_type classification + holiday CRUD
  (clash/in-use guards), scope + SELF_APPROVAL_FORBIDDEN, audit, notify stub, seed. Edits
  server.go/main.go/seed.go.
- **09-03** Go contract tests vs E7 openapi (state transitions + 409s, OT_BELOW_MIN 422, holiday
  clash/in-use, OUT_OF_SCOPE/SELF_APPROVAL_FORBIDDEN 403, bulk partial success, cursor shapes).
- **09-04** Full-stack Playwright E2E under NEW frontend/e2e/tests/e7/ (per Gherkin AC: confirm,
  L1→final, reject, withdraw, bulk partial, OT_BELOW_MIN, holiday CRUD + clash/in-use, scope 403).
  Selectors derived from the REAL e7-overtime components.

### Claude's Discretion
- Whether day_type is computed at seed/record time vs on-read — pick the simplest correct.
- Whether the approval/decision trail is a separate table or columns — match the contract response.
- Exact bulk envelope grouping — match the openapi example.
- How `:confirm` maps (agent-confirm semantics surfaced on the web) — follow the contract + what
  the e7 component calls.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel (httpx cursor, rbac GuardCompany, audit, apperr + error.details envelope, ids,
  idempotency, db.TxManager, i18n, Asia/Jakarta TZ).
- **Reference slices = Phase-8 leave (two-level approval state machine, scope, bulk, audit-in-tx,
  seed, the calendar read) and Phase-7 attendance (bulk partial-success + idempotency).** Phase-5
  placement for lifecycle/scope/ConflictWithDetails. The existing `overtime_rules` (E2/Phase-3) +
  its queries are the rule source for OT_BELOW_MIN/day_type.
- E2E harness (hardened detached-API boot + freePort from Phase-7): real stack + resetDb + loginAs
  PERSONAS.* + window.__swp_get_token__ + waitForToken + e3..e6-helpers. Existing E2E layout
  `frontend/e2e/tests/{e1..e6,smoke}/` → add `e7/`.

### Established Patterns
- Two-level approval via *ForUpdate + RETURNING-or-409. Bulk partial success → {succeeded,failed}.
  apperr.Rule()/struct-literal for 422; Conflict()/ConflictWithDetails for 409; OUT_OF_SCOPE 403.
  Notification dispatch stubbed (TODO Phase-11). FE errors via classifyError/error.details (NOT
  conflict_details). **Recurring FE finding:** detail GET may be wrapped `{data}` by the handler
  even when the openapi declares the bare object — unwrap with a bare fallback (Phase-8 fix).
  DataTable rows div.border-b; toggles role=switch; .js E2E imports; PERSONAS.*.

### Integration Points
- New `backend/db/queries/overtime/` (sqlc glob). Routes in server.go authenticated group under
  RequireRole (l1: shift_leader scoped + hr/super; final/bulk/holidays: hr/super). Seed extension.
  FE screens exist (e7-overtime/*, built from .pen) calling `@swp/api-client` e7 hooks via MSW —
  wire to real BE. E2E under new frontend/e2e/tests/e7/. resetDb must TRUNCATE overtime + holidays.
</code_context>

<specifics>
## Specific Ideas
- The approvals screen (overtime-approvals-screen.tsx) + records + detail + rules + holiday
  overlays are the primary surfaces — E2E drives REAL selectors/overlays, not invented ones.
- Workflow E2E drives the REAL state machine: PENDING_AGENT_CONFIRM → :confirm → :approve-l1 (leader)
  → :approve-final (HR) → APPROVED; reject + withdraw paths.
- OT_BELOW_MIN E2E: an OT shorter than the rule's min_minutes blocked at 422 with field errors.
- Holiday E2E: create (clash on dup date 409), update, delete (HOLIDAY_IN_USE 409 when referenced).
- SELF_APPROVAL_FORBIDDEN + OUT_OF_SCOPE E2E: an approver cannot approve their own / another
  company's OT (403).
- Bulk E2E: multiple OT ids → some approved, at least one skipped with a code; partial-success shape.

</specifics>

<deferred>
## Deferred Ideas
- Agent OT request/capture + the auto-detection job from verified attendance (mobile/system).
- Monetary/multiplier application (INV-2 — v1 records hours only; payroll is Phase-10 read-only).
- Notification dispatch implementation (stubbed; Phase-11).
- Reporting consumption of approved OT — Phase-11.
</deferred>

---

*Phase: 09-e7-overtime*
*Context gathered: 2026-06-05 (autonomous)*
