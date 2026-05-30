# PRD · F4.4 — Schedule Changes & Shift Swaps

> **Epic:** E4 Shift Configuration & Scheduling · **Feature:** F4.4 · **Status:** Draft v1 — leader edits in v1; **agent swap/day-off requests deferred to post-v1** (2026-05-29)
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Schedules change: a leader adjusts or clears a cell, or an agent can't make a shift and needs to swap or take the day off. This PRD covers **leader-driven edits** (always available) and an **optional agent-initiated swap / day-off request** approved by the leader (mobile self-service). All changes re-publish instantly and notify affected agents.

## 2. Goals & non-goals

**Goals**
- Edit/clear schedule entries with the same conflict rules as assignment.
- (Optional) Agent requests a **day-off** or a **shift swap** with a counterpart; leader approves/rejects.
- Re-publish + notify on every applied change; full audit trail.

**Non-goals**
- Initial assignment (F4.2). Attendance consequences of a no-show (E5). Leave requests (E6 — a formal leave is different from a one-off day-off swap; see §10).

## 3. Actors

Shift Leader (approver, editor), Agent (requester, mobile), HR/Super Admin (any company), System (validate, apply, notify, audit).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / mobile** | Shift Leader | Edit/clear cells; approve/reject swap & day-off requests. |
| **Mobile app** | Agent | Request a swap (with a named counterpart) or a day-off; see status. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| CH-1 | Editing a cell re-runs assignment rules (F4.2: placement active, one/day, scope); clearing removes the entry (distinct from OFF). |
| CH-2 | A change sets `status = Changed`, **auto-publishes**, and notifies the affected agent(s). |
| CH-3 | **Swap request:** an agent proposes swapping their shift on date D with a **counterpart agent** (also placed at the same company). On approval, the two agents' shifts on D (and/or the counterpart's date) are exchanged atomically. |
| CH-4 | **Day-off request:** an agent requests their shift on date D be removed. On approval, the entry is cleared/marked OFF. |
| CH-5 | Both agents in a swap must be **placed at the same company** (the leader's). Cross-company swaps are not allowed (different leaders). |
| CH-6 | Requests have status `Pending → Approved | Rejected | Withdrawn`; rejection requires a reason. |
| CH-7 | Only the **company's shift leader** (or HR/Super Admin) can approve; requests route to them. If the company has no leader, requests escalate to HR (F3.4 SL-7). |
| CH-8 | All requests and applied changes are audited. |

## 6. Data model

`Schedule` (update/clear). Optional `ScheduleChangeRequest`: `id, type (Swap|DayOff), requester_employee_id, counterpart_employee_id (null for DayOff), work_date, counterpart_date (null), status, reason, decided_by, decided_at, created_at`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Schedule changes & shift swaps

  Background:
    Given I am the shift leader of "Plaza Senayan"
    And "Budi" and "Citra" are placed there and scheduled on 2026-06-10

  Scenario: Leader edits a scheduled shift
    When I change "Budi"'s 2026-06-10 shift from "Morning" to "Night"
    Then the entry updates to "Night" with status Changed
    And "Budi" is notified immediately

  Scenario: Leader clears a shift
    When I clear "Budi"'s 2026-06-10 shift
    Then the entry is removed (not OFF)
    And "Budi" is notified

  Scenario: Agent requests a day-off, leader approves
    Given I am the agent "Budi"
    When I request a day-off for 2026-06-10 with a reason
    And the shift leader approves it
    Then my 2026-06-10 shift is cleared/OFF
    And I am notified of the approval

  Scenario: Agent requests a swap with a counterpart, approved
    Given "Budi" (Morning) wants to swap 2026-06-10 with "Citra" (Night)
    When "Budi" submits a swap request naming "Citra"
    And the shift leader approves it
    Then Budi gets "Night" and Citra gets "Morning" on 2026-06-10 atomically
    And both are notified

  Scenario: Reject a request with a reason
    When the shift leader rejects Budi's request with a reason
    Then the request is Rejected and Budi sees the reason

  Scenario: Cross-company swap is blocked
    Given the counterpart is placed at a different company
    When "Budi" submits a swap with that counterpart
    Then it is blocked because both must be at the same company

  Scenario: Request escalates when there is no shift leader
    Given "Plaza Senayan" has no shift leader
    When "Budi" submits a day-off request
    Then it routes to an HR admin for approval
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Swap that would violate one-shift-per-day for either agent | Blocked by conflict rules (CH-1). |
| C-2 | Counterpart not scheduled on the swap date | Swap becomes a give-away (counterpart gains a shift) — confirm whether allowed or require mutual shifts. |
| C-3 | Agent withdraws a pending request | Allowed before decision; status Withdrawn. |
| C-4 | Day-off vs formal leave | A one-off day-off swap is operational; multi-day/entitled absence should go through E6 Leave (see §10). |
| C-5 | Change to a past date | Blocked/limited (past schedules tie to attendance E5); only HR correction with reason. |
| C-6 | Counterpart's placement ends before the date | Blocked (placement inactive). |

## 9. Dependencies

F4.2 (assignment rules), E3 (placement/scope/leader), E6 (distinguish from formal leave), E10 (notifications), E1 (audit).

## 10. Decisions & open questions

- ✅ Leader edits/clears always available; changes auto-publish + notify.
- ✅ **Agent-initiated swap / day-off requests are DEFERRED to post-v1** (2026-05-29). **v1 = leader-driven edits/clears only**; the swap-request flow in this PRD documents the intended post-v1 design.
- **Deferred (post-v1):** day-off-swap vs formal-leave (E6) boundary; mutual-exchange vs one-way give-away.
