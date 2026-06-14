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
- Give Super Admin an oversight **superset**: the HR cockpit **plus** admin-only widgets (users & access, audit feed, org rollups, pending grants) — DB-7.
- Deliver the shift-leader dashboard on **both web and mobile** from one payload — DB-8.

**Non-goals**
- Detailed reports/exports (F10.3/F10.4). Notification delivery (F10.1).

## 3. Actors

Agent, Shift Leader, HR/Super Admin, System (compute, scope).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent / **Shift Leader** | Personal dashboard (agent) · team **Beranda** (leader — DB-8). |
| **Web console** | Shift Leader / HR Admin / **Super Admin** | Team (leader) · cross-company cockpit (HR) · cockpit **+ admin widgets** (super admin — DB-7). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| DB-1 | **Scope:** agent = own; shift leader = their company; HR/Super Admin = all (INV-3). |
| DB-2 | **Agent dashboard:** next/upcoming shift, today's clock status, leave balance, pending requests (leave/OT), recent notifications. |
| DB-3 | **Shift-leader dashboard:** today's roster, who's clocked in, pending approvals (attendance/leave/OT), open exceptions, coverage gaps for their company. |
| DB-4 | **HR dashboard:** cross-company KPIs — attendance rate, billable hours trend, OT totals, leave usage, active placements/headcount. |
| DB-5 | Dashboard widgets **deep-link** to the underlying feature (approve, verify, schedule). |
| DB-6 | Data reflects auto-published/near-live state (E4/E5). |
| DB-7 | **Super Admin superset:** the `super_admin` dashboard returns the `HrDashboard` payload **plus** an `admin` block — (a) **user & access** (active users, accounts pending provisioning, offboarded/disabled ≤30d — E2/F2.7); (b) **recent audit feed** (last sensitive actions — E1 audit); (c) **org rollups by position** (headcount + active placements grouped by free-text position — E3); (d) **pending grants** (role-change requests awaiting super-admin action). *(Bank-account approval escalations removed 2026-06-14 — profile edits are instant, no bank approval queue.)* `admin` is present **only** for `super_admin`; the `hr_admin` payload is unchanged. Each widget deep-links (DB-5). |
| DB-8 | **Leader dual-surface:** the `LeaderDashboard` payload backs **both** the web team dashboard and the mobile Beranda. No separate endpoint — `GET /dashboards/me` returns it for `shift_leader` on either client. |

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

  Scenario: Super Admin sees admin widgets (DB-7)
    Given I am a super admin
    When I open my dashboard
    Then I see the HR cross-company KPIs
    And I also see the admin widgets: users & access, recent audit, org rollups by position, and pending grants

  Scenario: HR Admin does not see admin widgets (DB-7)
    Given I am an HR admin
    When I open my dashboard
    Then I see the HR cross-company KPIs
    And the response carries no admin block, so no admin widgets render

  Scenario: Shift-leader mobile Beranda (DB-8)
    Given I am a shift leader on the mobile app
    When I open Beranda
    Then I see today's roster status, pending approvals, and schedule alerts for my company
    And it is the same data as my web team dashboard
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | New user with no data | Friendly empty/getting-started state. |
| C-2 | HR with many companies | KPI aggregation performant; drill-down by company. |
| C-3 | Leader of a company with no agents | Prompt to place agents (E3). |
| C-4 | Near-real-time freshness | Acceptable small delay; note caching strategy. |
| C-5 | Super Admin admin widgets with zero data (fresh tenant) | Each admin widget renders its empty state; no errors (DB-7, C-1). |
| C-6 | HR Admin payload must omit `admin` block | `admin` is absent (not null-filled); FE never renders admin widgets for `hr_admin` (DB-7). |
| C-7 | Audit feed volume | Recent audit feed is capped (last ~8); a "Lihat semua" link deep-links to the full audit log (DB-5). |

## 9. Dependencies

E3–E8 (data), F10.1 (notifications widget), E1 (scope).

## 10. Decisions & open questions

- ✅ Role-tailored, scoped dashboards with deep links.
- ✅ **Super Admin = HR cockpit superset** (DB-7, resolved 2026-06-11, [EPICS §8](../../../EPICS.md)) — admin-only widget block on `HrDashboard.admin`, super-admin only. Widget set: users & access · recent audit feed · org rollups by position · pending grants. *(Rollup axis changed from service line → free-text position 2026-06-12.)*
- ✅ **Shift-leader dashboard is dual-surface** (DB-8, resolved 2026-06-11) — one `LeaderDashboard` payload powers web + mobile Beranda; no new endpoint. Mobile frame `.pen` `UMzuO`.
- **Open:** exact KPI set + freshness/caching targets for the HR dashboard.
