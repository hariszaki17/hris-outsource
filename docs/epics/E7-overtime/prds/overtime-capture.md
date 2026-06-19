# PRD · F7.2 — Overtime Capture (request-only)

> **Epic:** E7 Overtime Tracking · **Feature:** F7.2 · **Status:** Draft v2
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

OT enters the system through **explicit requests** from agents/leaders. The system classifies each request by day type and enters the approval flow (F7.3). **No auto-detection** from attendance records — all overtime must be explicitly requested. *(Resolved 2026-06-19.)*

## 2. Goals & non-goals

**Goals**
- Agent/leader request OT (date, hours, reason).
- Classify each record's `day_type` (workday / rest day / holiday).

**Non-goals**
- Approval (F7.3). Rules definition (F7.1). Pay calc (out of scope v1).
- **Auto-detection from attendance** — all OT must be explicitly requested. *(Resolved 2026-06-19.)*

## 3. Actors

Agent / Shift Leader (request), System (classify), HR (oversight).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | Request OT. |
| **Web / mobile** | Shift Leader | Request OT on behalf. |
| System | — | Classify day type. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| OC-1 | A **request** captures: `work_date`, `start_at`/`end_at` (or hours), reason; `source = Requested`. |
| OC-2 | ~~**Auto-detect**~~ **REMOVED (2026-06-19).** No auto-detection from attendance — all OT must be explicitly requested. |
| OC-3 | Each record is classified into a **`day_type`** using the schedule (E4) + holiday calendar (F7.1 OR-5). |
| OC-4 | ~~`min_minutes`~~ **REMOVED (2026-06-19).** SL/HR determine OT validity, not a system threshold. |
| OC-5 | A record's `duration_minutes` is computed from start/end; cross-midnight handled. |
| OC-6 | Requested OT for a date with **no active placement** is blocked. |
| OC-7 | New records enter `Pending` (F7.3). |
| OC-8 | All captures audited; relevant parties notified. |

## 6. Data model

Creates `OvertimeRecord` (FEATURE §4): `employee_id, placement_id, attendance_id (null — always null in v2), work_date, start_at, end_at, duration_minutes, day_type, source (always Requested), status, notes`.

> `attendance_id` is always null (no auto-detection). `source` is always `Requested`. *(Resolved 2026-06-19.)*

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Overtime capture

  Scenario: Agent requests OT
    Given I am the agent "Budi" placed at "Plaza Senayan"
    When I request OT for 2026-06-10, 2 hours, with a reason
    Then a Pending OT record is created with source Requested and day_type classified

  Scenario: Rest-day OT classification
    Given Budi has no scheduled shift on 2026-06-14
    When OT is requested for that date
    Then it is classified day_type RestDay

  Scenario: Request without placement blocked
    Given Budi has no active placement on 2026-07-01
    When he requests OT that day
    Then it is blocked
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Cross-midnight OT | Attributed per the day-type/start-day rule (F7.1 C-1). |
| C-2 | OT request overlapping an existing OT | Blocked/merged — confirm. |
| C-3 | Holiday + rest day same date | Per F7.1 C-2 precedence. |

## 9. Dependencies

F7.1 (rules/day-type), E4 (shift end/rest day), E3 (placement), E1 (audit), F7.3 (approval).

## 10. Decisions & open questions

- ✅ **Request-only** — no auto-detection from attendance (OC-2 removed, 2026-06-19).
- ✅ **No `min_minutes`** — SL/HR determine OT validity (OC-4 removed, 2026-06-19).
- ✅ Classify day type per schedule + holiday calendar.

_No open questions remain for this feature._
