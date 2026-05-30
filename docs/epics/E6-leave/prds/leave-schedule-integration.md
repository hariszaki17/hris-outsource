# PRD · F6.4 — Leave–Schedule/Attendance Integration

> **Epic:** E6 Leave Management · **Feature:** F6.4 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

An approved leave must reconcile with the rest of the system: the agent's **scheduled shifts** (E4) on those days should be cleared/marked as leave, and attendance (E5) must **not** mark those days "Absent". Without this, a person on approved leave would still appear to be a no-show. This feature is the glue that keeps leave, schedule, and attendance consistent.

## 2. Goals & non-goals

**Goals**
- On leave **Approved**, cancel/mark overlapping scheduled shifts as Leave.
- Ensure E5 treats those dates as **Leave, not Absent**.
- On leave **cancelled/shortened**, restore the affected schedule days.

**Non-goals**
- Approval flow (F6.3). Re-creating the schedule (leader does that, E4). Clock-in mechanics (E5).

## 3. Actors

System (applies the integration), Shift Leader (re-schedules restored days), Agent (sees updated schedule).

## 4. Platform / clients

System-driven; results show on the schedule calendar (E4) and attendance (E5) for agents (mobile) and leaders (web).

## 5. Business rules

| Ref | Rule |
|-----|------|
| LI-1 | On a leave **Approved**, find the agent's schedule entries (E4) overlapping `[start_date, end_date]` and mark them **`Leave`** (or clear them). |
| LI-2 | Those leave-covered dates are **tagged so E5 does not produce an `Absent`** record (EV-4 suppression). |
| LI-3 | If the agent **already clocked in/worked** on a day later covered by an approved (e.g., backdated) leave, **flag the conflict** for leader/HR review rather than auto-overwriting. |
| LI-4 | On leave **cancelled/shortened**, the previously-cleared schedule days are **restored to unassigned** (leader re-assigns) and absent-suppression is removed. |
| LI-5 | Leave days are **visible on the schedule calendar** (E4 F4.3) and the leave calendar (F6.5). |
| LI-6 | All integration changes are audited and the agent + leader notified. |

## 6. Data model

Updates `Schedule` (E4: status `Leave`/cleared); writes a leave-day marker consumed by `Attendance` evaluation (E5). No new core entities (may use a lightweight `leave_day` projection).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Leave–schedule/attendance integration

  Scenario: Approval clears overlapping shifts
    Given Budi has shifts on 2026-06-10, 11, 12
    When a leave for 2026-06-10 to 2026-06-12 is approved
    Then those schedule entries are marked Leave
    And attendance does not mark those days Absent

  Scenario: Leave day is not an absence
    Given 2026-06-10 is covered by approved leave
    When the attendance end-of-day job runs
    Then no Absent record is created for Budi that day

  Scenario: Conflict when the day was already worked
    Given Budi clocked in on 2026-06-10
    And a backdated leave covering 2026-06-10 is approved
    Then the conflict is flagged for leader/HR review (not auto-overwritten)

  Scenario: Shortening leave restores the schedule
    Given an approved leave 2026-06-10 to 2026-06-12 is shortened to end 2026-06-11
    Then 2026-06-12 is restored to unassigned for re-scheduling
    And absent-suppression for 2026-06-12 is removed

  Scenario: Leave shows on calendars
    Then the leave days appear on both the schedule calendar and the leave calendar
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Leave overlaps days with no schedule yet | Mark the dates as leave so future scheduling/absent logic respects them. |
| C-2 | Partial-day leave (if supported) | Day kept but flagged partial — depends on F6.2 half-day decision. |
| C-3 | Leave cancelled after the dates passed | Historical attendance for those days reconciled/flagged (no silent change). |
| C-4 | Agent placed at a different company mid-leave (transfer) | Leave follows the agent; new company's schedule respects it. |

## 9. Dependencies

F6.3 (approval trigger), E4 (schedule), E5 (absent suppression / attendance), E10 (notifications), E1 (audit).

## 10. Decisions & open questions

- ✅ Approved leave cancels shifts + suppresses absent; cancel/shorten restores.
- **Open (C-3):** policy when leave covering already-worked/attended days is approved (flag vs adjust).
- **Open:** partial-day leave interaction with shift scheduling (tied to F6.2 half-day decision).
