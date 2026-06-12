# PRD · F7.4 — Overtime Records & Reporting

> **Epic:** E7 Overtime Tracking · **Feature:** F7.4 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Approved overtime needs to be visible and aggregable: an agent wants to see their approved OT; HR needs OT **hours by day-type tier** per agent / company / period to feed a future payroll run (E8) and client billing (E10). Because v1 is hours-only, this is about accurate **hour accounting per tier**, with multipliers shown as reference.

## 2. Goals & non-goals

**Goals**
- Agent view of own approved OT (mobile).
- Aggregated OT reporting: hours by day-type tier, per agent / company / position / period.
- Exports feeding payroll context (E8) and billing (E10).

**Non-goals**
- Pay computation (v1 hours-only). Capture/approval (F7.2/F7.3).

## 3. Actors

Agent (self), Shift Leader (own company), HR/Super Admin (all), System (query, scope, export).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | Own approved OT hours + status. |
| **Web / mobile** | Shift Leader | Team OT for their company. |
| **Web console** | HR / Super Admin | Cross-company OT reports + exports. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| OR-1 | **Scope:** agent sees own; leader sees own company; HR/Super Admin see all. |
| OR-2 | Reports aggregate **approved** OT `duration` grouped by `day_type` (Workday/RestDay/Holiday), agent, company, position (free-text), and period. |
| OR-3 | The applicable rule `multiplier` is shown as **reference** alongside hours (no money computed in v1). |
| OR-4 | Pending/Rejected OT is **excluded** from approved totals (may show separately). |
| OR-5 | Exports (Excel/CSV/PDF) reflect filters, are **audited**, and are structured for **payroll import (E8)** and **billing (E10)**. |
| OR-6 | Read-only; deep-links to the OT record / approval. |
| OR-7 | Times render in Asia/Jakarta; cross-midnight OT attributed per the day-type rule. |

## 6. Data model

Read-only projection over `OvertimeRecord` + `OvertimeRule` + `Employee` + `Placement`. The `position` axis is the free-text position carried from the placement. No new entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Overtime records & reporting

  Scenario: Agent views own approved OT
    Given I am the agent "Budi"
    When I open "My overtime"
    Then I see my approved OT hours by date and status

  Scenario: HR reports OT by tier for a company
    Given I am HR
    When I run the OT report for "Plaza Senayan" for June
    Then I get approved OT hours grouped by day_type (Workday/RestDay/Holiday) and agent
    And each tier shows its reference multiplier

  Scenario: Only approved OT counts
    Given Budi has approved, pending, and rejected OT in June
    Then the approved totals exclude the pending and rejected records

  Scenario: Export for payroll/billing
    Given I filtered OT by company and June
    When I export
    Then the file is structured for payroll/billing import and the export is audited

  Scenario: Scope enforced
    When a leader opens OT for a company they don't lead
    Then access is denied
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent with no OT | Empty state. |
| C-2 | OT approved after the period closed | Appears in the period it was worked; late-approval flagged. |
| C-3 | Correction changes approved OT hours | Report reflects re-derived hours; change audited. |
| C-4 | Large org export | Paginated/queued. |
| C-5 | Migrated historical OT (day_type defaulted) | Flagged so reports note unclassified/Workday-defaulted history. |

## 9. Dependencies

F7.1–F7.3 (data/tiers), E3 (placement/company), E8 (payroll import), E10 (billing/export), E1 (scope/audit).

## 10. Decisions & open questions

- ✅ Hours by day-type tier; multipliers reference-only; approved-only totals; export to E8/E10.
- **Open:** exact export schema for the future payroll run (align when E8/payroll is built).
