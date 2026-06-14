# PRD · F6.5 — Leave Calendar & Balance Views

> **Epic:** E6 Leave Management · **Feature:** F6.5 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

People need visibility into leave: an agent wants to see their **balance and request history**; a shift leader needs a **team leave calendar** to plan coverage (who's off when); HR needs the cross-company picture. Because leave directly affects site staffing, the team calendar is a planning tool, not just a record.

## 2. Goals & non-goals

**Goals**
- Agent: balance (total/used/remaining) + request history + statuses (mobile).
- Leader/HR: team leave calendar with pending + approved leave; coverage view.
- Exports for HR.

**Non-goals**
- Requesting/approving (F6.2/F6.3). Quota mechanics (F6.1).

## 3. Actors

Agent (self), Shift Leader (own company), HR/Super Admin (all), System (query, scope).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | Balance + own request history & statuses. |
| **Web / mobile** | Shift Leader | Team leave calendar (own company), pending queue. |
| **Web console** | HR / Super Admin | Cross-company leave calendar, balances, exports. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| LV-1 | **Scope:** agent sees only own; leader sees own company; HR/Super Admin see all. |
| LV-2 | Agent view shows current balance(s) by leave type/period + history with statuses. |
| LV-3 | Team calendar shows **approved** (and optionally **pending**) leave by date, per agent, for coverage planning. |
| LV-5 | Filters: date range, leave type, status, company. |
| LV-6 | Exports (Excel/PDF) reflect filters and are audited. |
| LV-7 | Read-only; deep-links to the request (F6.2) / approval (F6.3). |
| LV-8 | **Uncovered-post flag** (resolved 2026-05-31): for approved leave, the team calendar + the E4 schedule surface the resulting **open/uncovered shift slots** ("perlu pengganti") so the shift leader can backfill (re-roster an already-placed same-company agent). The agent's named **delegate** is shown as a **non-binding suggested** backfill. No auto-substitution and no cross-company borrowing in v1 — the leader decides. Coverage lives in **E4 scheduling**, not in leave. *(Coverage-clash highlight dropped 2026-06-12.)* |

## 6. Data model

Read-only projection over `LeaveRequest` + `LeaveQuota` + `LeaveType` + `Employee` + `Placement`. No new entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Leave calendar & balance views

  Scenario: Agent views balance and history
    Given I am the agent "Budi"
    When I open "My leave"
    Then I see my remaining annual balance and my past/pending requests

  Scenario: Leader views team leave calendar
    Given I am the shift leader of "Plaza Senayan"
    When I open the team leave calendar for June
    Then I see which agents are on approved leave each day
    And pending requests are indicated

  Scenario: Scope enforced
    When a leader opens leave for a company they don't lead
    Then access is denied

  Scenario: HR exports leave for a period
    Given I am HR and filtered by company and June
    When I export
    Then the file matches the filters and the export is audited
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent with no leave history | Empty state + current balance. |
| C-3 | Cross-period balances | Show current period; allow viewing prior periods. |
| C-4 | Large org export | Paginated/queued export. |

## 9. Dependencies

F6.1–F6.3 (data), E3 (placement/company scope), E2 (leave types), E10 (export/notifications), E1 (scope/audit).

## 10. Decisions & open questions

- ✅ Scoped views; team coverage calendar (who's off when); audited exports. *(Coverage-clash highlight + service-line filter dropped 2026-06-12 — service line removed project-wide.)*
- **Open:** show **pending** leave on the team calendar by default, or approved-only with a toggle?
