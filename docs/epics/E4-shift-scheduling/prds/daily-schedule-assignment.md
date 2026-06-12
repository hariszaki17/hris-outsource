# PRD · F4.2 — Daily Schedule Assignment

> **Epic:** E4 Shift Configuration & Scheduling · **Feature:** F4.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

The shift leader's daily job: decide who works which shift on which day at their site. hris-outsource does this **day-by-day, manually** — the leader picks a shift from the master (all active templates) for each placed agent on each date. Saving is **immediately live** to the agent (auto-publish), so the schedule the agent sees on mobile is always current. This schedule is what E5 attendance is judged against.

## 2. Goals & non-goals

**Goals**
- Assign a shift to each placed agent per date, via a calendar/grid.
- Enforce placement + conflict rules; auto-publish + notify on save.
- Support marking a day OFF.

**Non-goals**
- Rotations/patterns, coverage targets, approval gates (all excluded by decision). Swaps/edits → F4.4.

## 3. Actors

Shift Leader (own company), HR/Super Admin (any company), System (validate, publish, notify, audit), Agent (recipient).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / tablet** | Shift Leader | Build the schedule grid for their company. |
| **Web console** | HR / Super Admin | Schedule/oversee any company. |
| **Mobile app** | Shift Leader | Quick assign/adjust today's roster. |
| **Mobile app** | Agent | Receives the assignment instantly (read). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| SA-1 | An agent can be scheduled on a date only if they have an **active placement** on that date (E3 INV); the schedule links that `placement_id`. |
| SA-2 | **One shift per agent per date** (INV-1). Assigning a second replaces the first (with a warning). |
| SA-3 | A **shift leader may schedule only agents at their own company** (F3.4 scope). HR/Super Admin may schedule any. |
| SA-5 | Cannot schedule **before placement start** or **after placement end**; ending a placement cancels its future schedule entries. |
| SA-6 | Saving a cell **auto-publishes** (no draft) and **notifies the agent** on mobile (INV-4). |
| SA-7 | A day can be explicitly marked **OFF** (status `Off`) — distinct from "no entry". |
| SA-8 | Cross-midnight shifts are attributed to their **start date** (FEATURE §7). |
| SA-9 | All writes audited. |
| SA-10 | **Shift-master time edits propagate to unrealized schedule entries** (INV-5). A master `start_at` / `end_at` edit updates all matching `Schedule` rows where `work_date >= today`, `status != Off`, and not leave-cancelled — but only the **not-yet-realized** portion: `start_time` is frozen once the agent has checked in; `end_time`/`cross_midnight` is frozen once the agent has checked out. For entries where the agent is checked-in-but-not-out, the open attendance record's shift-end window is also updated to the new master end so that lateness / early / auto-close evaluation uses the live end until checkout. Break times are **not** propagated (master-only; no consumer on `Schedule`). |

## 6. Data model

`Schedule`: `id, employee_id (FK), shift_master_id (FK), placement_id (FK), work_date, status (Scheduled|Off|Changed), created_by`. Unique `(employee_id, work_date)`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Daily schedule assignment

  Background:
    Given I am the shift leader of "Plaza Senayan"
    And "Budi" has an active placement at "Plaza Senayan"

  Scenario: Assign a shift and auto-publish
    When I assign "Budi" the "Parking Night" shift on 2026-06-10
    Then a schedule entry is created with status "Scheduled"
    And it links Budi's active placement
    And "Budi" is notified on mobile immediately

  Scenario: Block scheduling an agent not placed that day
    Given "Andi" has no active placement at "Plaza Senayan" on 2026-06-10
    When I try to schedule "Andi" that day
    Then it is blocked with "Agent is not placed here on this date"

  Scenario: Replacing an existing shift for the day
    Given "Budi" already has "Morning" on 2026-06-10
    When I assign him "Night" on 2026-06-10
    Then I am warned and the entry is replaced with "Night"

  Scenario: Shift picker shows all active shifts
    When I open the shift picker for "Budi"
    Then all active shift templates from the master are available

  Scenario: Cannot schedule beyond placement end
    Given Budi's placement ends 2026-06-30
    When I try to schedule him on 2026-07-05
    Then it is blocked

  Scenario: Leader scope is enforced
    Given "Citra" is placed at a company I do not lead
    When I try to schedule "Citra"
    Then it is blocked by scope

  Scenario: Mark a day off
    When I mark 2026-06-12 as OFF for "Budi"
    Then his schedule shows OFF for that day
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Bulk-fill a shift across a date range | Applies per day; each day still validated (placement active, one/day). |
| C-2 | Agent on approved leave that day (E6) | Warn/block — scheduling over approved leave flagged. |
| C-3 | Placement is `Scheduled` (future start) | Cannot schedule before the start date (SA-5). |
| C-4 | Cross-midnight shift on the last placement day | Allowed; overnight portion handled by E5. |
| C-5 | HR admin schedules a company with no leader | Allowed (HR scope); agent still notified. |
| C-6 | Concurrent edits to the same cell | Last write wins on `(employee, date)` unique; both audited. |
| C-7 | Master `end_at` edited after agent has checked in but not yet checked out | Entry's `start_time` stays frozen; `end_time`/`cross_midnight` updates to the new master value. The open attendance record's stored shift-end window is also updated so lateness/early/auto-close evaluation uses the new end. |
| C-8 | Master times edited after agent has already checked out | Entry's `start_time` and `end_time` are both frozen (fully realized); the edit does **not** affect this entry. |

## 9. Dependencies

F4.1 (shift master), E3 (active placement + leader scope), E1 (audit), E10 (notifications), E5 (consumes schedule), E6 (leave conflicts).

## 10. Decisions & open questions

- ✅ Day-by-day manual; auto-publish; leader-scoped; one shift/agent/day.
- ✅ Scheduling over **approved leave is blocked** (2026-05-29) — the day is protected (E6 integration).
- ✅ Bulk "apply shift to date range" helper **included** in v1 (still validated per-day).
