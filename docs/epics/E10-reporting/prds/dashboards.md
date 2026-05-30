# PRD · F10.2 — Role-Based Dashboards

> **Epic:** E10 Reporting & Notifications · **Feature:** F10.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Each role needs a useful landing view: an agent wants their next shift and request statuses; a shift leader wants today's roster, pending approvals, and exceptions for their site; HR wants cross-company KPIs. Dashboards turn the data the other epics produce into an at-a-glance operational picture.

## 2. Goals & non-goals

**Goals**
- Role-tailored dashboards (agent / shift leader / HR), correctly scoped.
- Surface the most actionable items per role (next shift, approvals, exceptions, KPIs).

**Non-goals**
- Detailed reports/exports (F10.3/F10.4). Notification delivery (F10.1).

## 3. Actors

Agent, Shift Leader, HR/Super Admin, System (compute, scope).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent / Leader | Personal / team dashboard. |
| **Web console** | Leader / HR | Team / cross-company dashboards. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| DB-1 | **Scope:** agent = own; shift leader = their company; HR/Super Admin = all (INV-3). |
| DB-2 | **Agent dashboard:** next/upcoming shift, today's clock status, leave balance, pending requests (leave/OT), recent notifications. |
| DB-3 | **Shift-leader dashboard:** today's roster, who's clocked in, pending approvals (attendance/leave/OT), open exceptions, coverage gaps for their company. |
| DB-4 | **HR dashboard:** cross-company KPIs — attendance rate, billable hours trend, OT totals, leave usage, active placements/headcount. |
| DB-5 | Dashboard widgets **deep-link** to the underlying feature (approve, verify, schedule). |
| DB-6 | Data reflects auto-published/near-live state (E4/E5). |

## 6. Data model

Read projections across E3–E8. No new entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Role-based dashboards

  Scenario: Agent dashboard
    Given I am the agent "Budi"
    When I open my dashboard
    Then I see my next shift, clock status, leave balance, and pending requests

  Scenario: Shift-leader dashboard scoped to their company
    Given I am the shift leader of "Plaza Senayan"
    When I open my dashboard
    Then I see today's roster, pending approvals, and exceptions for "Plaza Senayan" only

  Scenario: HR cross-company KPIs
    Given I am HR
    When I open my dashboard
    Then I see attendance, billable-hours, OT, leave, and headcount KPIs across companies

  Scenario: Deep link to action
    Given a pending approval on my dashboard
    When I tap it
    Then I'm taken to the approval screen

  Scenario: Scope enforced
    When a leader opens a dashboard
    Then it shows only their company's data
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | New user with no data | Friendly empty/getting-started state. |
| C-2 | HR with many companies | KPI aggregation performant; drill-down by company. |
| C-3 | Leader of a company with no agents | Prompt to place agents (E3). |
| C-4 | Near-real-time freshness | Acceptable small delay; note caching strategy. |

## 9. Dependencies

E3–E8 (data), F10.1 (notifications widget), E1 (scope).

## 10. Decisions & open questions

- ✅ Role-tailored, scoped dashboards with deep links.
- **Open:** exact KPI set + freshness/caching targets for the HR dashboard.
