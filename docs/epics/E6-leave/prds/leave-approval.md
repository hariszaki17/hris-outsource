# PRD · F6.3 — Two-Level Approval Workflow

> **Epic:** E6 Leave Management · **Feature:** F6.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Leave needs sign-off from the agent's **shift leader** (who knows site coverage) and then **HR** (who owns policy and balances). The workflow must move a request through both levels, **commit the reservation** (pending → used on the leave type's window quota, F6.1) only on final approval, and trigger the schedule/attendance integration — while keeping a clear approval trail.

## 2. Goals & non-goals

**Goals**
- Two-level approval: shift leader → HR; reject at either level ends it.
- Commit the reservation (F6.1: `pending_days → used_days` on the type's window quota) and trigger integration (F6.4) on final approval.
- Escalate to HR-only when a company has no leader.

**Non-goals**
- Creating requests (F6.2). Quota/cap mechanics (F6.1). Schedule changes (F6.4 does the work).

## 3. Actors

Shift Leader (level 1), HR/Super Admin (level 2), System (route, commit quota reservation, integrate, notify), Agent (notified).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / mobile** | Shift Leader | Level-1 approve/reject for their company. |
| **Web console** | HR / Super Admin | Level-2 approve/reject; handle no-leader escalations. |
| **Web console** | Lead | Level-2 (final) approve/reject for agents in their assigned companies. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| LA-1 | Flow: `Pending → (leader) LeaderApproved → (HR) Approved`. A reject at either level → `Rejected` (reason required). |
| LA-2 | Level-1 approver is the agent's **company shift leader**; if none, the request **escalates straight to HR** (HR becomes the sole approver — confirm in §10). |
| LA-3 | Level-2 (final) approver is **HR/Super Admin**, OR the **assigned Lead** scoped to the agent's company. |
| LA-4 | On **final Approved**: **commit** the reservation (F6.1 LQ-2 — `pending_days → used_days` on the type's window quota for quota-bearing types; `PER_EVENT`/`UNCAPPED` have no quota to commit), then trigger schedule/attendance integration (F6.4). |
| LA-5 | **The type's remaining is re-checked at final approval** (the window may have been consumed by other approvals, or rolled over, since submission — F6.1 C-5); if now insufficient, approval is **blocked/flagged** (HR may adjust the quota, F6.1 LQ-6, or override). |
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
    Then status becomes Approved, the 3-day reservation is committed (pending → used on the annual quota), and the schedule integration fires

  Scenario: Leader rejects
    When "Sari" rejects with a reason
    Then status becomes Rejected and Budi is notified

  Scenario: HR rejects after leader approval
    Given the request is LeaderApproved
    When HR rejects it with a reason
    Then status becomes Rejected and the reservation is released (no quota consumed)

  Scenario: Escalate when no shift leader
    Given Budi's company has no shift leader
    When Budi submits a request
    Then it routes directly to HR for approval

  Scenario: Balance re-check at final approval
    Given Budi's annual remaining dropped to 2 (another approval consumed it) since submitting a 3-day request
    When HR tries to give final approval
    Then approval is blocked/flagged for insufficient balance (HR may adjust the quota or override)

  Scenario: Cannot self-approve
    Given a shift leader files their own leave
    Then they cannot approve it; it routes to HR
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent withdraws while LeaderApproved | Allowed before final approval → Cancelled; reservation released (no quota consumed). |
| C-2 | Leader is the agent's delegate too | Allowed to approve (not self) unless policy says otherwise. |
| C-3 | HR override without leader step | Allowed (LA-8), audited with reason. |
| C-4 | `PER_EVENT`/`UNCAPPED` request (e.g. bereavement, sick) | Same flow; nothing to commit to a quota — only the per-occurrence cap / document gate applied at submit (F6.1 LQ-13). |
| C-5 | Window rolled over between submit and approval | Re-check at final approval (LA-5); if the new window lacks remaining, block/flag for HR. |

## 9. Dependencies

F6.2 (request), F6.1 (quota commit/re-check), F6.4 (integration), F3.4 (leader scope/escalation), E10 (notifications), E1 (audit).

## 10. Decisions & open questions

- ✅ Two-level leader → HR; reject ends; commit the type's quota reservation + integrate on final; re-check the type's remaining (F6.1 per-type ledger, 2026-06-12).
- **Open (LA-2):** when there's no leader, is HR the **sole** approver, or does someone act as level-1 stand-in?
- **Open:** SLA/auto-escalation if a level sits un-actioned for N days?
