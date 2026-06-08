# PRD · F6.3 — Two-Level Approval Workflow

> **Epic:** E6 Leave Management · **Feature:** F6.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Leave needs sign-off from the agent's **shift leader** (who knows site coverage) and then **HR** (who owns policy and balances). The workflow must move a request through both levels, **commit the FIFO reservation** (pending → consumed across grant-lots, F6.1) only on final approval, and trigger the schedule/attendance integration — while keeping a clear approval trail.

## 2. Goals & non-goals

**Goals**
- Two-level approval: shift leader → HR; reject at either level ends it.
- Commit the grant-lot reservation (F6.1: pending → consumed, `LeaveConsumption` rows) and trigger integration (F6.4) on final approval.
- Escalate to HR-only when a company has no leader.

**Non-goals**
- Creating requests (F6.2). Grant-lot mechanics (F6.1). Schedule changes (F6.4 does the work).

## 3. Actors

Shift Leader (level 1), HR/Super Admin (level 2), System (route, commit grant-lot reservation, integrate, notify), Agent (notified).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / mobile** | Shift Leader | Level-1 approve/reject for their company. |
| **Web console** | HR / Super Admin | Level-2 approve/reject; handle no-leader escalations. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| LA-1 | Flow: `Pending → (leader) LeaderApproved → (HR) Approved`. A reject at either level → `Rejected` (reason required). |
| LA-2 | Level-1 approver is the agent's **company shift leader**; if none, the request **escalates straight to HR** (HR becomes the sole approver — confirm in §10). |
| LA-3 | Level-2 approver is **HR/Super Admin**. |
| LA-4 | On **final Approved**: **commit** the FIFO reservation (F6.1 LQ-2 — `pending_days → consumed_days` across the allocated lots, write `LeaveConsumption` rows), then trigger schedule/attendance integration (F6.4). |
| LA-5 | **Available balance is re-checked at final approval** (lots may have been consumed or **expired** since submission — F6.1 C-4); the commit re-allocates FIFO across still-active lots. If now insufficient, approval is **blocked/flagged** (HR may pre-fund a lot, F6.1 LQ-11, or override). |
| LA-6 | An approver **cannot approve their own** leave request (separation) — routes to the next level / HR. |
| LA-7 | Each decision creates a `LeaveApproval` record (level, approver, decision, reason, time); all audited; agent notified at each step. |
| LA-8 | HR/Super Admin may **override** (force approve/reject) with a reason. |

## 6. Data model

`LeaveApproval`: `id, leave_request_id (FK), level (1|2), approver_id, decision, reason, decided_at`. Updates `LeaveRequest.status`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Two-level leave approval

  Background:
    Given Budi has a Pending 3-day annual leave request
    And Budi's company has shift leader "Sari"

  Scenario: Leader approves then HR approves
    When "Sari" approves the request
    Then status becomes LeaderApproved and it moves to HR
    When HR approves it
    Then status becomes Approved, the 3-day reservation is committed (consumed across the FIFO-allocated lots), and the schedule integration fires

  Scenario: Leader rejects
    When "Sari" rejects with a reason
    Then status becomes Rejected and Budi is notified

  Scenario: HR rejects after leader approval
    Given the request is LeaderApproved
    When HR rejects it with a reason
    Then status becomes Rejected and the reservation is released (no lots consumed)

  Scenario: Escalate when no shift leader
    Given Budi's company has no shift leader
    When Budi submits a request
    Then it routes directly to HR for approval

  Scenario: Balance re-check at final approval
    Given Budi's available balance dropped to 2 (a lot expired) since submitting a 3-day request
    When HR tries to give final approval
    Then approval is blocked/flagged for insufficient balance (HR may pre-fund a lot or override)

  Scenario: Cannot self-approve
    Given a shift leader files their own leave
    Then they cannot approve it; it routes to HR
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent withdraws while LeaderApproved | Allowed before final approval → Cancelled; reservation released (no lots consumed). |
| C-2 | Leader is the agent's delegate too | Allowed to approve (not self) unless policy says otherwise. |
| C-3 | HR override without leader step | Allowed (LA-8), audited with reason. |
| C-4 | Earmarked (e.g. maternity) request | Same flow; commit draws **only** the matching earmarked lot (F6.1 LQ-10). |
| C-5 | Request spanning multiple lots' expiries | Commit FIFO-spans lots, writing one `LeaveConsumption` row per lot (F6.1 C-2). |

## 9. Dependencies

F6.2 (request), F6.1 (grant-lot commit/re-check), F6.4 (integration), F3.4 (leader scope/escalation), E10 (notifications), E1 (audit).

## 10. Decisions & open questions

- ✅ Two-level leader → HR; reject ends; commit grant-lot reservation + integrate on final; re-check available balance (F6.1 grant-lot ledger, 2026-06-08).
- **Open (LA-2):** when there's no leader, is HR the **sole** approver, or does someone act as level-1 stand-in?
- **Open:** SLA/auto-escalation if a level sits un-actioned for N days?
