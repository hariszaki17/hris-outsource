# PRD · F8.5 — Payroll Period Close (month-end reconciliation gate)

> **Epic:** E8 Payroll · **Feature:** F8.5 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_ · **Added:** 2026-06-17
> **Bridges:** E5 Attendance · E6 Leave · E7 Overtime · E10 Notifications · E11 Approvals

---

## 1. Context & problem

Pay for a month depends on three upstreams — **attendance** (E5, verified + per-day `is_payable`), **overtime** (E7, approved hours), and **leave** (E6, paid vs unpaid days). Today each is reconciled **record-by-record** with no notion of a **period**: there's no view of whether a month is "done," no lock to freeze the inputs, no back-channel for HR to ask a shift-leader/agent about a record, and no clean immutable hand-off to payroll. So HR has no authoritative *"the inputs for June are settled"* signal, and any late change silently re-floats the numbers.

**Payroll Period Close** is the month-end **gate**: one **PayrollPeriod** per month that HR reviews across three tabs (Attendance · Overtime · Leave), resolves every exception (with a new clarification back-channel), then **locks** — snapshotting an immutable per-employee dataset that the payroll run (F8.3) consumes. Post-lock changes never mutate history; they carry forward as `PayrollAdjustment` (the F8.3 PR-13 mechanism). The attendance tab is the largest; it orchestrates the existing E5 resolution actions (F5.3 verify, F5.4 correct, F5.6 manual, set-payable, F5.2 auto-close).

## 2. Goals & non-goals

**Goals**
- A per-month **PayrollPeriod** with a state machine `OPEN → LOCKED` (+ super-admin `REOPENED`), global scope (all companies).
- **Completeness** across all three upstreams per employee: expected (E4 roster) vs recorded (E5) vs leave (E6), plus pending OT (E7) and pending leave approvals — with unresolved-exception counts per tab.
- **Resolve** routing to existing actions (E5 F5.3/F5.4/F5.6/set-payable/F5.2; E6 leave approval; E7 OT approval) + a new thin **clarification request** to SL/lead/agent.
- **Lock** that (a) requires every blocking exception resolved or disposed, (b) snapshots an immutable **PeriodEmployeeSummary** per FIELD employee (attendance + OT + leave figures), (c) audits who/when.
- **Immutable hand-off:** the payroll run for the month requires `LOCKED`; post-lock changes emit a `PayrollAdjustment` to the next open run.

**Non-goals**
- Computing pay — F8.3 owns that; this emits the *inputs*, not money.
- A full message thread — clarification is **one round** (ask → answer → resolve).
- Per-company partial close (v1 = global monthly; §10 open).
- Building the payroll-run compute itself (F8.3, still spec-only) — this feature leaves a clean seam (PC-10) the run plugs into; the `PayrollAdjustment` ledger it writes accumulates ready for that run.

## 3. Actors

HR admin (global) & super admin — own review/lock/reopen, raise clarifications. Shift leader / lead / agent — **recipients** of clarification, continue verification (F5.3) & corrections (F5.4) & their approvals (E6/E7). System — completeness compute, snapshot, adjustment emit, notifications.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web** | HR / super admin | The cockpit: period picker, 3 completeness tabs (Attendance · Overtime · Leave), resolve drill-down, lock/reopen, raise clarification. |
| **Web / Mobile** | SL / lead / agent | Receive clarification notification (E10), answer it, optionally file a correction (F5.4). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| **PC-1** | A **PayrollPeriod** is keyed by `(year, month)` (global, v1). Status `OPEN` (default; upstreams mutable) → `LOCKED` → (`REOPENED` → `OPEN`). Auto-created `OPEN` on first access for any month with attendance/schedule data. |
| **PC-2** | **Attendance completeness (per FIELD employee):** `expected` = E4 workable entries (`is_day_off=false`, `status ∈ {SCHEDULED, MODIFIED}`, not `CANCELLED_BY_LEAVE`); `recorded` = attendance rows in-month; `clean` = `verification_status ∈ {VERIFIED, AUTO_APPROVED}`; `on_leave` = E6 approved leave days; coverage gap = `expected − (clean + on_leave)`. |
| **PC-3** | **Attendance blockers** (resolve or dispose before lock): `verification_status ∈ {PENDING, ESCALATED}`; open records (`check_out_at IS NULL`); `is_payable IS NULL` on a no-shift record; coverage gap with no record and no leave (uncaptured shift → confirm ABSENT or manual entry). |
| **PC-4** | **Overtime blockers:** any E7 overtime in-period with `status = PENDING` (not yet approved/rejected). **Leave blockers:** any E6 leave request overlapping the period with `status = PENDING`. A clean lock needs zero of each. |
| **PC-5** | **Gating scope:** blocking applies to **FIELD employees** (pay is attendance-gated). **INTERNAL** staff are fixed-salary — shown in the completeness view as discipline info, **do not block** lock. |
| **PC-6** | **Disposition:** each blocker clears via an existing action — verify/reject (F5.3), correction (F5.4), manual entry (F5.6), `set-payable`, auto-close (F5.2); OT approve/reject (E7); leave approve/reject (E6). A real no-show is *disposed* by confirming `ABSENT` (terminal, payroll-known), not deleted. |
| **PC-7** | **Lock precondition:** `LOCKED` permitted only when in-scope (FIELD) blockers across all three tabs = 0. A **super-admin force-lock** with a required reason overrides a non-zero count (audited, reason stored) — for genuinely-stuck records (absconded agent). |
| **PC-8** | **On lock:** for each FIELD employee, materialize an **immutable** `PeriodEmployeeSummary`: attendance (payable_days, present, late, absent, no_shift_payable_days, worked_minutes), overtime (approved_ot_minutes by tier), leave (paid_leave_days, unpaid_leave_days). Authoritative payroll input. Stamp `locked_by`, `locked_at`. |
| **PC-9** | **Post-lock immutability:** after lock, records remain legally correctable (F5.4) / approvable, but a change to a locked-month record **does not mutate** the `PeriodEmployeeSummary`. It emits a `PayrollAdjustment` (`source_type ∈ {Attendance, Overtime, Correction}`, `origin_year/month` = the locked period, signed `amount` left for the run to value) carried to the next open run. |
| **PC-10** | **Payroll precondition (seam):** a payroll run (F8.3) for `(year, month)` requires that month `LOCKED` and reads the `PeriodEmployeeSummary`, not live records. *(F8.3 compute is not yet built; this guard + the summary table are the contract it will consume. Until then the summary + adjustment ledger simply accumulate.)* |
| **PC-11** | **F5.7 interaction:** auto-reconcile (F5.7) is a **no-op** for attendance in a `LOCKED` period — a late schedule-create still inserts the shift but must not silently alter locked figures; reconcile runs only against `OPEN`-period records. |
| **PC-12** | **Reopen:** super-admin only, reason required, audited. Allowed **only while no payroll run for the month is `Posted`** (after post, history is fixed — adjustments only). Reopen voids the `PeriodEmployeeSummary` and returns to `OPEN`. |
| **PC-13** | **Clarification request (new, advisory):** HR raises a `ClarificationRequest` on a specific record (attendance / OT / leave) → target auto-resolved (the record's company shift-leader, else the agent). Sends an **E10 notification** (push + in-app, deep-linked). State `OPEN → ANSWERED → RESOLVED` (or `CANCELLED`). Target answers with a note and **may** file a correction (F5.4) which routes normally. **Not** an approval line; blocks nothing on its own, but an `OPEN` clarification surfaces in the cockpit as "awaiting reply." One round; HR may re-ask. |
| **PC-14** | **Audit:** every transition (`PERIOD_LOCKED`, `PERIOD_FORCE_LOCKED`, `PERIOD_REOPENED`) and clarification (`CLARIFICATION_RAISED/ANSWERED/RESOLVED`) is audited (E1) with before/after period status + resolved/forced counts. |
| **PC-15** | **RBAC:** review/lock/reopen/raise-clarification = `hr_admin` + `super_admin`, global scope. Force-lock + reopen = `super_admin` only. SL/lead/agent: no period actions beyond answering clarifications (scope-checked to their company/self). |

## 6. Data model + state machine

**New tables**
- `payroll_periods` — `id`, `year`, `month`, `status` (OPEN|LOCKED|REOPENED), `locked_by`, `locked_at`, `force_locked` bool, `force_lock_reason`, `reopened_by`, `reopened_at`, timestamps. Unique `(year, month)`.
- `period_employee_summaries` — `id`, `period_id`, `employee_id`, `payable_days`, `present_days`, `late_days`, `absent_days`, `no_shift_payable_days`, `worked_minutes`, `approved_ot_minutes`, `paid_leave_days`, `unpaid_leave_days`, `created_at`. **Immutable**; unique `(period_id, employee_id)`.
- `clarification_requests` — `id`, `source_type` (ATTENDANCE|OVERTIME|LEAVE), `source_id`, `period_id`, `raised_by`, `target_employee_id`, `question`, `status` (OPEN|ANSWERED|RESOLVED|CANCELLED), `answer`, `answered_by`, `answered_at`, `created_at`.
- `payroll_adjustments` — the E8 F8.3 carry-forward ledger (create now, consumed later by the run): `id`, `employee_id`, `source_type`, `source_id`, `origin_year`, `origin_month`, `note`, `amount` (signed, nullable until valued), `status` (PENDING|APPLIED), `applied_run_id` nullable.

**Reads:** `attendance`, `schedule_entries`, `approved_leave_days`, E7 overtime, E6 leave. **Reuses:** E10 `notifications`, E1 audit.

```
PayrollPeriod (year, month):
  OPEN ──lock (PC-7: all blockers=0 | super-admin force)──▶ LOCKED ──┐
   ▲                                                                 │
   └──────── reopen (super-admin, no Posted run, PC-12) ◀── REOPENED ┘
LOCKED ──emit──▶ PeriodEmployeeSummary (immutable) ──▶ F8.3 payroll run (PC-10)
post-lock change ──▶ PayrollAdjustment → next open run (PC-9)
```

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Payroll period close

  Scenario: Cannot lock with open exceptions
    Given June has 3 FIELD attendance records PENDING and 1 OT PENDING
    When HR opens the June period cockpit
    Then the Attendance tab shows 3 blockers and the Overtime tab shows 1
    And Lock is disabled until all are resolved or disposed

  Scenario: Lock emits the immutable summary
    Given all June FIELD blockers across the 3 tabs are cleared
    When HR locks June
    Then the period becomes LOCKED with locked_by/locked_at
    And a PeriodEmployeeSummary exists per FIELD employee with attendance+OT+leave figures
    And the figures are immutable

  Scenario: Post-lock change carries forward, never mutates history
    Given June is LOCKED
    When an agent's approved correction adds a payable June day
    Then the June PeriodEmployeeSummary is unchanged
    And a PENDING PayrollAdjustment (origin June) is created for the next run

  Scenario: Auto-reconcile inert on a locked period
    Given June is LOCKED and an UNSCHEDULED June record exists
    When a late shift covering it is created
    Then F5.7 does not re-derive the locked record

  Scenario: Clarification round-trip
    Given a June attendance record is ambiguous
    When HR raises a clarification to the record's shift leader
    Then the shift leader gets a notification deep-linked to the record
    And answers with a note (optionally filing a correction)
    And HR resolves the clarification and verifies the record

  Scenario: Super-admin force-lock with reason
    Given one agent absconded and his record stays ABSENT-pending
    When the super admin force-locks June with a reason
    Then the period locks, the reason is stored, and it is audited

  Scenario: Payroll requires a locked period
    Given June is OPEN
    When a June payroll run is attempted
    Then it is blocked until June is locked

  Scenario: Reopen only before posting
    Given June is LOCKED and its payroll run is not Posted
    When the super admin reopens June with a reason
    Then the period returns to OPEN and the summary is voided
    But once a run is Posted, reopen is rejected
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-PC-1 | INTERNAL staff PENDING record | Shown for discipline; does not block lock (PC-5). |
| C-PC-2 | Cross-midnight shift on month's last day | Belongs to its `work_date` month (shift start date), not clock-out date. |
| C-PC-3 | Approved leave covers a gap | Counts as `on_leave`, not a blocker (PC-2). |
| C-PC-4 | Late roster after lock (F5.7) | Shift inserts; reconcile no-op on locked records (PC-11); HR corrects → adjustment. |
| C-PC-5 | Correction/OT/leave approved **before** lock | Applies in place — no adjustment (only post-lock changes adjust). |
| C-PC-6 | Clarification target = agent without app access | Falls back to the record's shift leader; if none, the placement's HR owner. |
| C-PC-7 | Two runs touch the same month (rerun) | Period stays LOCKED; reruns read the same summary; adjustments apply once. |
| C-PC-8 | Month with no attendance (all leave / no placements) | Locks with an empty summary set. |
| C-PC-9 | Reopen after a Posted run | Rejected (PC-12) — adjustments only. |
| C-PC-10 | Pending leave spanning two months | Blocks **both** periods until decided (counted in each overlapped month). |

## 9. API (contract additions, E8 openapi)

All `x-rbac: { roles: [hr_admin, super_admin], scope: global }` unless noted.

| Method · Path | Purpose |
|---|---|
| `GET /payroll-periods/{year}/{month}` | Period status + per-tab blocker counts (auto-creates OPEN). |
| `GET /payroll-periods/{year}/{month}/completeness` | Per-employee completeness rows (cursor-paged): expected/clean/on_leave/blockers, FIELD/INTERNAL flag. |
| `POST /payroll-periods/{year}/{month}:lock` | Lock; body optional `{ force, reason }` (force = super-admin). 422 `PERIOD_HAS_BLOCKERS` when blockers>0 and not forced. |
| `POST /payroll-periods/{year}/{month}:reopen` | Super-admin; body `{ reason }`. 409 `PERIOD_RUN_POSTED` if a run is Posted. |
| `GET /payroll-periods/{year}/{month}/summary` | The immutable `PeriodEmployeeSummary` set (post-lock). |
| `POST /clarifications` | Raise: `{ source_type, source_id, question }`. |
| `POST /clarifications/{id}:answer` | Target answers `{ answer }` (scope-checked). |
| `POST /clarifications/{id}:resolve` / `:cancel` | HR closes the loop. |

No change to existing endpoints' request/response shapes. Description-only note added to F8.3 payroll-run op (requires LOCKED period).

## 10. Dependencies

E5 F5.2/F5.3/F5.4/F5.6/F5.7 (resolution + lock interaction), E4 (expected roster), E6 (leave feed + approvals), E7 (OT feed + approvals), **E8 F8.3** (run precondition + `PayrollAdjustment` reuse), E10 (clarification notifications), E11 (corrections still route there), E1 (audit, RBAC). **New migrations:** 4 tables.

## 11. Decisions & open questions

- ✅ **Unified PayrollPeriod** gating attendance + OT + leave as one month-end lock (2026-06-17); this attendance cockpit is its primary tab.
- ✅ **Global monthly** period (v1) — per-company partial close deferred (2026-06-17).
- ✅ Immutable `PeriodEmployeeSummary` on lock; post-lock → `PayrollAdjustment` (reuses F8.3 PR-13).
- ✅ **Clarification = thin, notification-backed, one round** (not E11, not chat).
- ✅ **FIELD gates the lock; INTERNAL is informational** (mirrors FIELD-only `is_payable`).
- ✅ **F5.7 inert on locked periods** (PC-11).
- ✅ **Global monthly only in v1 — per-company partial close deferred to v2** (2026-06-17). Lock the whole month at once; per-company independent lock + roll-up revisited only if a slow-client bottleneck shows in practice.
- ✅ **Adjustment valuation is owned by the F8.3 payroll run** (2026-06-17). Period-close emits the `PayrollAdjustment` event with `amount = null` (it does not compute pay); the run values + applies it at next-run time, where wage/proration rules live. Already built this way (`amount` nullable).
- ✅ **OT/Leave tabs are block-and-link, not inline-approve** (2026-06-17). The cockpit surfaces *pending* OT (E7) / leave (E6) as blockers and links to their existing E6/E7 approval screens; HR approves there and returns to lock. No duplicated approval UI / E11 line-membership logic in the cockpit. Inline-approve revisited only on HR round-trip complaints.
