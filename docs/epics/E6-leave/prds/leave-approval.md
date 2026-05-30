# PRD · F6.3 — Two-Level Approval Workflow

> **Epic:** E6 Leave Management · **Feature:** F6.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Leave needs sign-off from the agent's **shift leader** (who knows site coverage) and then **HR** (who owns policy and balances). The workflow must move a request through both levels, deduct the quota only on final approval, and trigger the schedule/attendance integration — while keeping a clear approval trail.

## 2. Goals & non-goals

**Goals**
- Two-level approval: shift leader → HR; reject at either level ends it.
- Deduct quota (F6.1) and trigger integration (F6.4) on final approval.
- Escalate to HR-only when a company has no leader.

**Non-goals**
- Creating requests (F6.2). Quota mechanics (F6.1). Schedule changes (F6.4 does the work).

## 3. Actors

Shift Leader (level 1), HR/Super Admin (level 2), System (route, deduct, integrate, notify), Agent (notified).

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
| LA-4 | On **final Approved**: deduct the quota (F6.1, annual types), then trigger schedule/attendance integration (F6.4). |
| LA-5 | Balance is **re-checked at final approval** (it may have changed since submission); if now insufficient, approval is blocked/flagged. |
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
    Then status becomes Approved, 3 days are deducted, and the schedule integration fires

  Scenario: Leader rejects
    When "Sari" rejects with a reason
    Then status becomes Rejected and Budi is notified

  Scenario: HR rejects after leader approval
    Given the request is LeaderApproved
    When HR rejects it with a reason
    Then status becomes Rejected and no quota is deducted

  Scenario: Escalate when no shift leader
    Given Budi's company has no shift leader
    When Budi submits a request
    Then it routes directly to HR for approval

  Scenario: Balance re-check at final approval
    Given Budi's remaining dropped to 2 since submitting a 3-day request
    When HR tries to give final approval
    Then approval is blocked/flagged for insufficient balance

  Scenario: Cannot self-approve
    Given a shift leader files their own leave
    Then they cannot approve it; it routes to HR
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent withdraws while LeaderApproved | Allowed before final approval → Cancelled; no deduction. |
| C-2 | Leader is the agent's delegate too | Allowed to approve (not self) unless policy says otherwise. |
| C-3 | HR override without leader step | Allowed (LA-8), audited with reason. |
| C-4 | Non-annual type (no quota) | Same flow; no deduction step. |
| C-5 | Request spanning a period boundary | Balance check per period (F6.2 C-3). |

## 9. Dependencies

F6.2 (request), F6.1 (quota deduction/re-check), F6.4 (integration), F3.4 (leader scope/escalation), E10 (notifications), E1 (audit).

## 10. Decisions & open questions

- ✅ Two-level leader → HR; reject ends; deduct + integrate on final; re-check balance.
- **Open (LA-2):** when there's no leader, is HR the **sole** approver, or does someone act as level-1 stand-in?
- **Open:** SLA/auto-escalation if a level sits un-actioned for N days?
