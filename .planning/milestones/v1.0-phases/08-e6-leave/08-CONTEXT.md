# Phase 8: E6 Leave - Context

**Gathered:** 2026-06-05 (autonomous — recommended decisions auto-accepted per user's overnight directive)
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the FE-used E6 "leave" endpoints against the real BE and wire the screens off MSW,
proven with exhaustive full-stack Playwright E2E. The web surface is **HR/leader approval +
quota management + calendar** — NOT agent leave-request creation (that is mobile/agent-only;
requests are seeded as Pending so the web approval flows have real targets). Delivers: leave
request list/detail + the two-level approval state machine (L1 → final, plus HR override and
reject), leave quotas (list/adjust/bulk-grant) with the soft-reservation balance model, and the
leave calendar. **Closes the loop Phase 6 predicted:** on final approval, INV-3 cancels
overlapping E4 schedule entries and populates the E4-owned `approved_leave_days` table (created
in Phase 6 as a fixture-seeded source with the note "E6 wires the production source later").
</domain>

<decisions>
## Implementation Decisions

### Scope = the 10 FE-used hooks ONLY (fe-endpoint-inventory.md E6)
- Leave requests: `GET /leave-requests`, `GET /leave-requests/{id}`,
  `POST /leave-requests/{id}:approve-l1`, `:approve-final`, `:approve-override`, `:reject`.
- Quotas: `GET /leave-quotas`, `POST /leave-quotas/{id}:adjust`, `POST /leave-quotas:bulk-grant`.
- Calendar: `GET /leave-calendar`.
- **OUT of scope (not FE-web):** agent leave-request CREATE + document upload + delegate pick
  (mobile/agent-only). Requests are seeded directly in Pending states so the web approval flows
  have real targets. The annual grant job + period-end expiry job are not built (we seed quotas).

### Approval state machine (per E6 contract + FEATURE INV-2)
- States: `PENDING_L1` → `PENDING_HR` (LeaderApproved) → `APPROVED`; reject at either level →
  `REJECTED`. (FEATURE prose uses Pending/LeaderApproved/Approved; **the openapi enum is
  authoritative** — match its exact status strings.)
- `:approve-l1` (shift_leader for own company, or HR) moves PENDING_L1 → PENDING_HR.
- `:approve-final` (HR/super) moves PENDING_HR → APPROVED.
- `:approve-override` (HR/super) approves even when balance is exceeded (LA-5/LA-8) and/or skips
  L1; performs the INV-3 side effects.
- `:reject` (reason required) at either level → REJECTED.
- **No-leader routing (LA-2):** if the agent's company has no active ShiftLeaderAssignment, a
  seeded request is in `PENDING_HR` directly with `routing.no_leader: true` in the response.
- Acting on a terminal request (APPROVED/REJECTED/CANCELLED) or wrong-state transition → 409
  (per contract code). Cross-company L1 by a leader → 403 `OUT_OF_SCOPE`.

### Quota balance model (per E6 contract + INV-1/4)
- `LeaveQuota`: total, used, pending, `remaining = total − used − pending`; period = calendar
  year (`period_start=YYYY-01-01`, `period_end=YYYY-12-31`).
- `pending` = soft-reservation held by PENDING_L1 + PENDING_HR annual requests against the quota.
- **INV-1 quota guard:** an annual (`is_tahunan`) request cannot exceed `remaining` →
  **422 `QUOTA_EXCEEDED`** with field errors (the FE renders field-level). Override bypasses this
  (HR can approve over-balance).
- `:adjust` (HR) sets/adjusts a quota's total, audited; refuses to set total below current used.
- `:bulk-grant` (HR) grants annual/per-type quotas for a period; `pro_rate: true` →
  `entitlement × remaining_months / 12` for probation joiners; does NOT overwrite `used`;
  refuses to set total below used for any employee (those rows reported as skipped — partial
  success per the contract envelope).

### INV-3 integration side effects (the loop-closer) — on `:approve-final` / `:approve-override`
- Cancel/мark overlapping E4 `schedule_entries` for the leave dates (status → LEAVE/cancelled).
- INSERT the approved leave days into the **E4-owned `approved_leave_days`** table (employee_id,
  leave_date, leave_request_id, leave_type) so the Phase-6 over-leave conflict engine + attendance
  "suppress Absent" read a REAL production source — replacing the Phase-6 seeded fixture mechanism.
- All within the same tx as the approval; audit + notify stub. This is exactly the "E6 wires the
  production source" hand-off the Phase-6 CONTEXT/SUMMARY documented — verify against it.

### Audit + notify (success criterion 1)
- Every approve-l1/final/override/reject + quota adjust/bulk-grant writes an audit_log row in-tx
  and fires a notification **stub** (TODO Phase-11), per the Phase-4..7 pattern. Each approval also
  writes a `leave_approvals` decision row (the approval audit trail per the FEATURE ER diagram).

### Calendar (`GET /leave-calendar`)
- Returns leave entries for a requested date range; `show_pending` toggle (default false) includes
  PENDING_L1 + PENDING_HR; approved entries always shown. Scope-aware (leader sees own company).
  Match the openapi calendar response shape exactly (what leave-calendar-screen.tsx renders).

### Build approach (mirror Phase-5/6/7 slice EXACTLY)
- migration → sqlc (`make gen`) → repository → service (apperr codes, audit, GuardCompany scope) →
  hand-written chi handlers → routes in server.go under RequireRole → Go contract tests → FE
  wiring (MSW off) + live Playwright E2E. Match `docs/api/E6-leave/openapi.yaml` byte-for-byte.
  Cursor pagination + filters (§11). New migrations: `leave_requests`, `leave_quotas`,
  `leave_approvals`. FKs to employees/placements/companies/leave_types (E2). SWP IDs: check ids.go
  for LR (leave request — note Phase-6 used SWP-LR-44210 in approved_leave_days as a DISPLAY id;
  reconcile: the leave-request entity id prefix per CONVENTIONS) + LQ (quota); add prefixes only if
  missing. New query dir `backend/db/queries/leave/`. action-suffix routes.

### Seed (in 08-02)
- Several leave_requests for the seeded agents (Phase-5 SWP-PL-5001..5004 placements): at least
  one PENDING_L1 at SWP-CMP-0021 (Rudi's company → leader L1 target), one PENDING_HR (no-leader or
  leader-approved → final target), one over-balance annual request (QUOTA_EXCEEDED / override
  target), one at SWP-CMP-0022 (OUT_OF_SCOPE target for the leader), and approved/rejected
  terminal ones for the list filters + calendar.
- leave_quotas for those agents (annual, calendar-year, with used/pending so remaining math is
  exercised; one near-exhausted to drive QUOTA_EXCEEDED).
- A request whose dates overlap a seeded E4 schedule entry so the INV-3 cancellation +
  approved_leave_days population is E2E-observable. **TZ note:** clearly-in-range Asia/Jakarta dates.

### Plan split (4 plans, mirrors ROADMAP)
- **08-01** Migrations + sqlc + domain (`leave_requests`, `leave_quotas`, `leave_approvals`).
- **08-02** Services + handlers: approval state machine (l1/final/override/reject), quota checks +
  adjust + bulk-grant, INV-3 side effects (cancel schedule + populate approved_leave_days),
  calendar, scope guards, audit, notify stub, seed. Edits server.go/main.go/seed.go.
- **08-03** Go contract tests vs E6 openapi (state transitions + 409s, QUOTA_EXCEEDED 422 +
  field errors, OVERLAPPING_LEAVE 409, OUT_OF_SCOPE 403, bulk-grant partial success, calendar shape).
- **08-04** Full-stack Playwright E2E under NEW frontend/e2e/tests/e6/ (per Gherkin AC: approvals
  L1/final/override/reject, quota list/adjust/bulk-grant, QUOTA_EXCEEDED, calendar render, scope
  403). Selectors derived from the REAL e6-leave components. + INV-3 side-effect assertion.

### Claude's Discretion
- Whether `leave_approvals` is a separate table or denormalized columns — pick what cleanly
  supports the decision trail + the FE timeline; match the contract response.
- Exact soft-reservation recompute (trigger vs computed-on-read) — pick the simplest correct one.
- How to reconcile the SWP-LR display id used by Phase-6 approved_leave_days with the leave-request
  entity id — keep ids consistent; document.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel (httpx cursor, rbac GuardCompany, audit, apperr + error.details envelope, ids,
  idempotency, db.TxManager, i18n, Asia/Jakarta TZ).
- **Reference slices = Phase-7 attendance (verify/reject state machine, bulk partial-success,
  scope, audit-in-tx, seed) and Phase-6 scheduling (bulk-grant partial success analog; the
  `approved_leave_days` table + its FindApprovedLeaveForAgentDate query + the schedule_entries
  cancel path this phase must write into).** Phase-5 placement for lifecycle/scope/ConflictWithDetails.
- E2E harness (now hardened: detached API process group + freePort(8081) from Phase-7 — reliable
  boot): real stack + resetDb + loginAs PERSONAS.* + window.__swp_get_token__ + waitForToken +
  e3/e4/e5-helpers. Existing E2E layout `frontend/e2e/tests/{e1..e5,smoke}/` → add `e6/`.

### Established Patterns
- State-machine guards via *ForUpdate + RETURNING-rows-or-409. Bulk partial success → {succeeded,
  failed} envelope. apperr.Rule()/struct-literal for 422; Conflict()/ConflictWithDetails for 409.
  Notification dispatch stubbed (TODO Phase-11). FE errors via classifyError/error.details (NOT
  conflict_details). DataTable rows div.border-b; toggles role=switch; .js E2E imports; PERSONAS.*.
- **Cross-epic write-through precedent:** Phase-6 created approved_leave_days expecting E6 to
  populate it — this phase fulfills that. Read backend/internal/service/scheduling for the
  schedule_entries cancel/update query + the approved_leave_days insert target.

### Integration Points
- New `backend/db/queries/leave/` (sqlc glob). Routes in server.go authenticated group under
  RequireRole (l1: shift_leader scoped + hr/super; final/override/quotas: hr/super). Seed
  extension. FE screens exist (e6-leave/*, built from .pen) calling `@swp/api-client` e6 hooks via
  MSW — wire to real BE. E2E under new frontend/e2e/tests/e6/. resetDb must TRUNCATE leave tables
  (and reset approved_leave_days/schedule_entries state between tests given the INV-3 write-through).
</code_context>

<specifics>
## Specific Ideas
- The approvals screen (leave-approvals-screen.tsx) + detail + quotas + calendar are the primary
  surfaces — E2E drives REAL selectors/overlays, not invented ones.
- Approval E2E must drive the REAL state machine: a PENDING_L1 request → leader :approve-l1 →
  PENDING_HR → HR :approve-final → APPROVED; a reject path; an override path (over-balance).
- QUOTA_EXCEEDED E2E: an over-balance annual request blocked at 422 with field errors; override
  succeeds.
- INV-3 side-effect E2E (the loop-closer): approve a leave overlapping a seeded E4 schedule entry,
  then assert the schedule entry is cancelled/LEAVE AND a subsequent E4 schedule attempt on that
  day hits SHIFT_OVER_LEAVE from the now-real approved_leave_days row (production source proven).
- OUT_OF_SCOPE E2E: a leader cannot :approve-l1 another company's request (403).
- Calendar E2E: renders entries for a date range; show_pending toggles pending visibility.

</specifics>

<deferred>
## Deferred Ideas
- Agent leave-request CREATE + document upload + delegate (mobile/agent-only).
- Annual grant job + period-end expiry job (we seed quotas).
- Notification dispatch implementation (stubbed; Phase-11).
- Overtime/payroll/reporting consumption of leave — later phases.
</deferred>

---

*Phase: 08-e6-leave*
*Context gathered: 2026-06-05 (autonomous)*
