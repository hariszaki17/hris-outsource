# PRD · F5.2 — Attendance Evaluation & Auto-Close

> **Epic:** E5 Attendance · **Feature:** F5.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Raw clock events become meaningful only when judged against the **scheduled shift**: was the agent late? did they forget to clock out? did they no-show? This feature is the **system logic** that computes lateness, assigns status, **auto-closes** open shifts, and routes records to either auto-approval or the verification queue (exceptions-only).

## 2. Goals & non-goals

**Goals**
- Compute `is_late` / `late_minutes` vs scheduled shift start (+ grace).
- Assign `status` (Present / Late / Incomplete / Absent).
- Auto-clock-out open records at scheduled shift end.
- Route to `AutoApproved` vs `Pending` per the exceptions rule.

**Non-goals**
- Capturing clocks (F5.1). Leader decisions (F5.3). OT computation (E7).

## 3. Actors

System (scheduled jobs + on-event logic). Outputs consumed by F5.3/F5.5, E7.

## 4. Platform / clients

System/background — no direct UI; results surface in F5.3 (queue) and F5.5 (records). Agents/leaders see resulting status on their apps.

## 5. Business rules

| Ref | Rule |
|-----|------|
| EV-1 | **Lateness:** if `check_in_at` > scheduled shift `start_at` + **grace period**, set `is_late=true`, `late_minutes`, `status=Late`; else `status=Present`. |
| EV-2 | **Grace period** is a single configurable value (proposed default 15 min; see §10). |
| EV-3 | **Auto-clock-out:** a scheduled job at each shift's end closes any still-open record at `check_out_at = shift end`, sets `auto_closed=true`, `status=Incomplete`. |
| EV-4 | **Absent:** a scheduled shift with **no clock-in** by shift end (+ small buffer) is recorded `status=Absent`. |
| EV-5 | **Exceptions routing (INV-3):** `verification_status=Pending` if `is_late` OR `in_geofence_in/out=false` OR `auto_closed` OR `status∈{Incomplete,Absent}` OR the attendance code `needs_verification`; **otherwise `AutoApproved`**. |
| EV-6 | **Unscheduled** records (no `schedule_id`) skip lateness (nothing to compare) but are always `Pending` (flagged). |
| EV-7 | Re-evaluation runs when a **correction** (F5.4) is applied; status/flags recomputed and routing re-derived. |
| EV-8 | Jobs are **catch-up safe** (evaluate by due time, not "now only"); all derived changes audited as `system`. |

## 6. Data model

Updates `Attendance`: `is_late, late_minutes, auto_closed, status, verification_status`. No new entities (grace + buffer are settings/consts).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Attendance evaluation

  Scenario: On-time clean clock-in auto-approves
    Given a "Morning" shift starts 07:00 with a 15-minute grace
    When "Budi" clocks in at 07:05 inside the geofence and clocks out at shift end
    Then status is Present and verification is AutoApproved

  Scenario: Late clock-in is flagged
    When "Budi" clocks in at 07:30 (grace is 15 min)
    Then is_late is true, late_minutes is 30, status is Late
    And verification is Pending

  Scenario: Out-of-geofence clean time still needs verification
    Given "Budi" clocked in on time but outside the geofence
    Then status is Present but verification is Pending

  Scenario: Auto-clock-out for a forgotten clock-out
    Given "Budi" clocked in but never clocked out
    And his shift ended at 15:00
    When the shift-end job runs
    Then check_out_at is set to 15:00, auto_closed is true, status Incomplete, verification Pending

  Scenario: No-show is marked absent
    Given "Budi" had a scheduled shift and never clocked in
    When the shift-end job runs
    Then status is Absent and verification Pending

  Scenario: Re-evaluation after a correction
    Given a Late record is corrected to an on-time clock-in
    When the correction is applied
    Then is_late becomes false and the record re-routes (may AutoApprove)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Clock-in exactly at grace boundary | Define boundary inclusive/exclusive (proposed: late if strictly after start+grace). |
| C-2 | Cross-midnight shift end | Auto-close uses the shift's end on the following day. |
| C-3 | Unscheduled clock-in | No lateness computed; always Pending (EV-6). |
| C-4 | Early clock-out (before shift end) | Recorded; flagged early/Incomplete for verification (confirm threshold). |
| C-5 | Job downtime/missed run | Catch-up evaluates all overdue shifts on next run (EV-8). |
| C-6 | Agent on approved leave but a shift exists | Reconcile with E6 — leave should suppress "Absent"; confirm precedence. |

## 9. Dependencies

F5.1 (clock data), E4 (scheduled shift times), E2 (codes, grace), E6 (leave vs absent), E7 (consumes worked time), E1 (audit), scheduled job runner.

## 10. Decisions & open questions

- ✅ Auto-clock-out at shift end + flag; exceptions-only routing.
- **Open:** grace-period value (proposed 15 min) and whether it varies by service line.
- **Open:** early-clock-out threshold before it's flagged Incomplete.
- **Open (C-6):** approved leave must suppress Absent — confirm E6↔E5 precedence.
