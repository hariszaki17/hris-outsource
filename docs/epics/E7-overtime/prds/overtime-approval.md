# PRD · F7.3 — Two-Level Approval Workflow

> **Epic:** E7 Overtime Tracking · **Feature:** F7.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Overtime costs money and is billable, so it needs sign-off: the **shift leader** (who knows whether the extra work was real and needed) approves first, then **HR** confirms. Only approved OT counts in reporting. Same two-level pattern as leave (E6), applied to both pre-requested and auto-detected OT.

## 2. Goals & non-goals

**Goals**
- Two-level approval: shift leader → HR; reject at either level ends it.
- Count approved OT hours by day-type tier (F7.4) on final approval.
- Escalate to HR-only when a company has no leader.

**Non-goals**
- Capture (F7.2). Rules (F7.1). Pay calc (out of scope).

## 3. Actors

Shift Leader (level 1), HR/Super Admin (level 2), System (route, count, notify), Agent (notified).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / mobile** | Shift Leader | Level-1 approve/reject for their company. |
| **Web console** | HR / Super Admin | Level-2 approve/reject; no-leader escalations; overrides. |
| **Web console** | Lead | Level-2 (final) approve/reject for agents in their assigned companies. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| OA-1 | Flow: `Pending → (leader) LeaderApproved → (HR) Approved`; reject at either level → `Rejected` (reason required). |
| OA-2 | Level-1 = the agent's **company shift leader**; escalates to HR if none (F3.4 SL-7). |
| OA-3 | Level-2 (final) = **HR/Super Admin**, OR the **assigned Lead** scoped to the agent's company. |
| OA-4 | On **final Approved**, the OT counts toward reporting (F7.4) classified by `day_type`. |
| OA-5 | An approver **cannot approve their own** OT (separation). |
| OA-6 | Auto-detected candidates follow the same flow (after any required agent confirmation, F7.2 OC-7). |
| OA-7 | Each decision creates an `OvertimeApproval` record (level, approver, decision, reason, time); audited; agent notified. |
| OA-8 | HR/Super Admin may **override** with a reason. |
| OA-9 | Bulk approval of multiple pending OT records is allowed. |

## 6. Data model

`OvertimeApproval`: `id, overtime_record_id (FK), level (1|2), approver_id, decision, reason, decided_at`. Updates `OvertimeRecord.status`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Two-level overtime approval

  Background:
    Given Budi has a Pending 2-hour OT record at "Plaza Senayan"
    And his company shift leader is "Sari"

  Scenario: Leader then HR approve
    When "Sari" approves it
    Then status becomes LeaderApproved
    When HR approves it
    Then status becomes Approved and the hours count by day_type

  Scenario: Leader rejects
    When "Sari" rejects with a reason
    Then status becomes Rejected and Budi is notified

  Scenario: HR rejects after leader approval
    Given the OT is LeaderApproved
    When HR rejects with a reason
    Then status becomes Rejected and nothing counts

  Scenario: Escalate when no leader
    Given Budi's company has no shift leader
    Then his OT routes directly to HR

  Scenario: Cannot self-approve
    Given a shift leader has their own OT record
    Then they cannot approve it; it goes to HR

  Scenario: Bulk approve
    Given five pending OT records for my company
    When I bulk approve them
    Then all become LeaderApproved (then to HR)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Auto-detected candidate rejected | Status Rejected; the attendance record itself is unaffected. |
| C-2 | Underlying attendance corrected after OT approval | OT re-derived/flagged for re-review (E5 correction propagation). |
| C-3 | Agent withdraws a pending request | Allowed before final → Cancelled. |
| C-4 | Approve OT for a now-ended placement | Allowed (work already happened); audited. |

## 9. Dependencies

F7.2 (records), F7.4 (counting), F3.4 (leader scope/escalation), E5 (correction propagation), E10 (notifications), E1 (audit).

## 10. Decisions & open questions

- ✅ Two-level leader → HR; reject ends; count on final; HR escalation/override.
- **Open:** SLA/auto-escalation if a level is un-actioned for N days?
- **Open (C-2):** how attendance corrections propagate to already-approved OT (recompute vs flag).
