# PRD · F7.2 — Overtime Capture (request + auto-detect)

> **Epic:** E7 Overtime Tracking · **Feature:** F7.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

OT enters the system two ways: an agent/leader **pre-requests** it, or the system **auto-detects** it from **verified attendance** (E5) that runs past the scheduled shift end. Both produce an OT record, classified by day type, that then needs approval (F7.3). Auto-detect ensures real extra work isn't missed; pre-request supports planned OT.

## 2. Goals & non-goals

**Goals**
- Agent/leader pre-request OT (date, hours, reason).
- Auto-detect candidate OT from verified attendance beyond the scheduled shift end, above `min_minutes`.
- Classify each record's `day_type` (workday / rest day / holiday).

**Non-goals**
- Approval (F7.3). Rules definition (F7.1). Pay calc (out of scope v1).

## 3. Actors

Agent / Shift Leader (request), System (auto-detect, classify), HR (oversight).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | Request OT; confirm an auto-detected candidate. |
| **Web / mobile** | Shift Leader | Request OT on behalf; review candidates. |
| System | — | Auto-detect from verified attendance; classify day type. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| OC-1 | A **request** captures: `work_date`, `start_at`/`end_at` (or hours), reason; `source = Requested`. |
| OC-2 | **Auto-detect:** when verified attendance (E5) shows `check_out_at` beyond the scheduled shift end by **≥ `min_minutes`** (F7.1), create a candidate OT (`source = AutoDetected`, linked `attendance_id`). |
| OC-3 | Each record is classified into a **`day_type`** using the schedule (E4) + holiday calendar (F7.1 OR-5). |
| OC-4 | OT below `min_minutes` is **not created** (ignored). |
| OC-5 | A record's `duration_minutes` is computed from start/end (or attendance excess); cross-midnight handled. |
| OC-6 | Requested OT for a date with **no active placement** is blocked; auto-detected OT always has a placement (it came from attendance). |
| OC-7 | New records enter `Pending` (F7.3). Auto-detected candidates may require **agent confirmation** before/at level-1 (see §10). |
| OC-8 | Duplicate protection: auto-detect must not double-create OT for a record already requested/created for the same shift. |
| OC-9 | All captures audited; relevant parties notified.

## 6. Data model

Creates `OvertimeRecord` (FEATURE §4): `employee_id, placement_id, attendance_id (null if request), work_date, start_at, end_at, duration_minutes, day_type, source, status, notes`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Overtime capture

  Scenario: Agent pre-requests OT
    Given I am the agent "Budi" placed at "Plaza Senayan"
    When I request OT for 2026-06-10, 2 hours, with a reason
    Then a Pending OT record is created with source Requested and day_type classified

  Scenario: Auto-detect OT from verified attendance
    Given Budi's shift ended 15:00 and his verified clock-out was 16:30
    And min_minutes is 60
    When the OT detection runs
    Then a candidate OT of 90 minutes is created with source AutoDetected linked to the attendance

  Scenario: Below threshold is ignored
    Given Budi worked 20 minutes past his shift and min_minutes is 60
    Then no OT record is created

  Scenario: Rest-day OT classification
    Given Budi has no scheduled shift on 2026-06-14 but worked
    When OT is captured
    Then it is classified day_type RestDay

  Scenario: No double-counting
    Given Budi already requested OT for his 2026-06-10 shift
    When auto-detect runs for the same shift
    Then it does not create a duplicate

  Scenario: Request without placement blocked
    Given Budi has no active placement on 2026-07-01
    When he requests OT that day
    Then it is blocked
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Cross-midnight OT | Attributed per the day-type/start-day rule (F7.1 C-1). |
| C-2 | Auto-detected OT later corrected (E5 correction) | Re-derive duration/day_type; re-evaluate the candidate. |
| C-3 | OT request overlapping an existing OT | Blocked/merged — confirm. |
| C-4 | Pre-approval-required rule but worked without request | See F7.1/§10 — reject vs allow late approval. |
| C-5 | Holiday + rest day same date | Per F7.1 C-2 precedence. |

## 9. Dependencies

F7.1 (rules/thresholds/day-type), E5 (verified attendance source), E4 (shift end/rest day), E3 (placement), E1 (audit), F7.3 (approval).

## 10. Decisions & open questions

- ✅ Both request + auto-detect; classify day type; ignore < min_minutes.
- **Open (OC-7):** must the agent confirm an auto-detected candidate before the leader sees it, or route straight to the leader?
- **Open (C-4):** if a rule requires pre-approval, is unrequested-but-worked OT rejected or still approvable after the fact?
