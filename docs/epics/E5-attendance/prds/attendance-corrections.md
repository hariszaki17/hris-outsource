# PRD · F5.4 — Attendance Corrections

> **Epic:** E5 Attendance · **Feature:** F5.4 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Clock data is imperfect — a forgotten clock-out (auto-closed), a missed clock-in, a wrong attendance code. Agents and leaders need a controlled way to **correct** a record, with approval and an audit trail that preserves the original. Mirrors legacy `attendance_corrections` (typed, statused, with approval bookkeeping).

## 2. Goals & non-goals

**Goals**
- File a correction (missed/wrong clock-in/out, or code) with a proposed value + reason.
- Approve via the shift leader (escalate HR); on approval, apply + **re-evaluate** (F5.2) and keep the original snapshot.

**Non-goals**
- Normal clock-in/out (F5.1). Verification of clean exceptions (F5.3, though a reject there often spawns a correction).

## 3. Actors

Agent (requester, mobile), Shift Leader / HR (approver), System (apply, re-evaluate, audit), Agent (notified).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | File a correction for own record; track status. |
| **Web / mobile** | Shift Leader / HR | Review & decide corrections; may also file on behalf. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| CR-1 | Correction `type ∈ {check_in, check_out, code, other}` with a `corrected_time` (or code) and a **required reason**. |
| CR-2 | Approval routes to the **company shift leader**; escalates to **HR** if none (F3.4 SL-7). |
| CR-3 | Statuses: `Pending → Approved → Applied` or `Pending → Rejected` (reason required on reject). |
| CR-4 | On **Applied**, the correction updates the `Attendance` record and triggers **re-evaluation** (F5.2: lateness/status/routing recomputed). |
| CR-5 | The **original values are preserved** (snapshot) for audit; corrections never erase history. |
| CR-6 | A record may have **multiple** corrections over time; each is independently audited. |
| CR-7 | Corrections to **migrated/historical** records are allowed by HR only (data integrity), within policy. |
| CR-8 | All actions audited; requester notified of the decision. |
| CR-9 | **A `check_in` correction re-evaluates status.** Approving (applying) a `check_in` correction that resolves an **`Absent`** record — or corrects a wrong clock-in time — re-runs F5.2 over the new `check_in_at`: `status` recomputes `Absent → Present` or `Late` against `shift_start_at` + the 15-min grace, and `is_late` / `late_minutes` are recomputed. Specializes CR-4 for the absence-resolution case (an `Absent` record carries `check_in_at = null`; the correction populates it). |

## 6. Data model

`AttendanceCorrection`: `id, attendance_id (FK), requester_id (FK), type, corrected_time, status, notes, decided_by, decided_at, original_snapshot (json), created_at`. (Single-level approval; legacy multi-level `current_level` collapsed — DATA-MAPPING G-7.)

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Attendance corrections

  Background:
    Given I am the agent "Budi"
    And my 2026-06-10 record was auto-closed because I forgot to clock out

  Scenario: File and approve a missed clock-out correction
    When I file a check_out correction with the real time 15:10 and a reason
    And the shift leader approves it
    Then my record's check_out_at becomes 15:10 and auto_closed is cleared
    And the record is re-evaluated and re-routed
    And the original auto-closed values are retained as a snapshot

  Scenario: Reject a correction
    When the shift leader rejects my correction with a reason
    Then the correction is Rejected and I see the reason
    And my attendance record is unchanged

  Scenario: Correction changes lateness
    Given my record was marked Late due to a wrong clock-in time
    When a check_in correction to an on-time value is approved
    Then is_late becomes false after re-evaluation

  Scenario: Correction escalates without a leader
    Given my company has no shift leader
    When I file a correction
    Then it routes to HR for approval

  Scenario: History preserved
    Given a correction is applied
    Then the original pre-correction values remain queryable
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Correction on an Absent record (agent actually worked) | Allowed; on approval creates/updates the worked record and re-evaluates. |
| C-2 | Multiple corrections on one record | Each tracked; latest applied value wins; all audited. |
| C-3 | Correction window | Configurable limit on how far back agents can self-correct (HR exempt) — see §10. |
| C-4 | Correction after the record fed E7 overtime / E10 billing | Downstream recomputation/flagging required — confirm propagation. |
| C-5 | Correcting a migrated historical record | HR-only (CR-7). |

## 9. Dependencies

F5.1/F5.2 (records + re-evaluation), F3.4 (approver scope/escalation), E7/E10 (downstream recompute), E1 (audit), E10 (notifications).

## 10. Decisions & open questions

- ✅ Typed corrections, leader-approved (HR escalation), apply→re-evaluate, original snapshot kept.
- **Open:** self-correction window (how many days back an agent may correct before it's HR-only).
- **Open (C-4):** how corrections propagate to already-computed OT (E7) / billing (E10) — recompute vs flag.
