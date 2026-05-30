# PRD · F4.3 — Schedule Calendar & Agent View

> **Epic:** E4 Shift Configuration & Scheduling · **Feature:** F4.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Once shifts are assigned, people need to *see* them: the shift leader needs a company calendar to manage coverage at a glance, and each agent needs their own upcoming shifts on mobile with enough detail (time + site/location) to show up correctly. Because scheduling auto-publishes, these views are always live.

## 2. Goals & non-goals

**Goals**
- Leader: company schedule calendar (day / week / by-agent).
- Agent: own upcoming schedule on mobile, with shift times + site location and reminders.
- Correct scoping and live updates.

**Non-goals**
- Editing (F4.2/F4.4). Attendance status overlay (E5 may extend this view later).

## 3. Actors

Shift Leader (own company), HR/Super Admin (any), Agent (self), System (query, scope, notify).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / tablet** | Shift Leader, HR | Company calendar: day/week, by-agent matrix. |
| **Mobile app** | Agent | "My schedule": upcoming shifts, times, site + map, shift reminders. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| SV-1 | A shift leader sees only **their company's** schedule; HR/Super Admin see any; an agent sees **only their own** (SV scope). |
| SV-2 | Views: day, week, and by-agent matrix for the company; agent mobile shows a forward-looking list (+ today highlighted). |
| SV-3 | Each entry shows shift title, **start/end times**, break, OFF marker, and the **site (client company) + location/geo**. |
| SV-4 | Views reflect auto-published changes **without a refresh step** (live or near-live). |
| SV-5 | Agents receive a **shift reminder** notification ahead of each shift (lead time TBD, default the evening before / X hours prior). |
| SV-6 | Times render in the org timezone (Asia/Jakarta); cross-midnight shifts display spanning two days. |

## 6. Data model

Read-only projection over `Schedule` + `ShiftMaster` + `Placement` + `ClientCompany`. No new entities (reminder lead-time is a setting/const).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Schedule views

  Scenario: Leader views the weekly company calendar
    Given I am the shift leader of "Plaza Senayan"
    When I open the week view
    Then I see each placed agent's shift per day for that week

  Scenario: Agent views own upcoming shifts on mobile
    Given I am the agent "Budi"
    When I open "My schedule"
    Then I see my upcoming shifts with times and the site location
    And today's shift is highlighted

  Scenario: Agent cannot see other agents' schedules
    When "Budi" opens his schedule
    Then he sees only his own shifts

  Scenario: Live update after a change
    Given "Budi" is viewing his schedule
    When the leader changes his shift
    Then Budi's view reflects the new shift without manual refresh
    And he receives a change notification

  Scenario: Shift reminder
    Given "Budi" has a shift tomorrow at 07:00
    Then he receives a reminder ahead of the shift

  Scenario: Cross-midnight display
    Given "Budi" has a 23:00–07:00 shift on 2026-06-10
    Then it displays spanning 2026-06-10 into 2026-06-11
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent with no upcoming shifts | Empty state ("no shifts scheduled"). |
| C-2 | Agent placed at a company with no geo | Location shows address only; map disabled. |
| C-3 | Leader of a company with no placed agents | Empty calendar + prompt to place agents (E3). |
| C-4 | Large company calendar | Paginated/virtualized by agent; week scoped by default. |

## 9. Dependencies

F4.2 (schedule data), E3 (placement/site + leader scope), E2 (ClientCompany location), E10 (reminders/notifications), E1 (scope).

## 10. Decisions & open questions

- ✅ Leader company calendar + agent mobile self-view; live.
- **Open:** shift-reminder lead time — evening-before, X hours prior, or both? (default: evening-before + 1h prior.)
