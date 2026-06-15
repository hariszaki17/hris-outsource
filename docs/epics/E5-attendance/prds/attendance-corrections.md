# PRD · F5.4 — Attendance Corrections

> **Epic:** E5 Attendance · **Feature:** F5.4 · **Status:** Draft v1 (rev 2026-06-15 — E11 routing, NEW_ENTRY, payable)
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Clock data is imperfect — a forgotten clock-out (auto-closed), a missed clock-in, a wrong attendance code. Agents and leaders need a controlled way to **correct** a record, with approval and an audit trail that preserves the original. Mirrors legacy `attendance_corrections` (typed, statused, with approval bookkeeping).

## 2. Goals & non-goals

**Goals**
- File a correction (missed/wrong clock-in/out, or code) with a proposed value + reason.
- **Search** one's own attendance to find the problematic record (agent self-service, on the **Pengajuan** tab — a 3rd tab beside Cuti/Lembur).
- **Create a record for a day that has none** (`NEW_ENTRY`) — forgot to clock in/out entirely, or a day with **no configured shift**.
- Route every correction through the **E11 approval engine** (same per-company configurable template as leave/overtime); on final approval, apply + **re-evaluate** (F5.2), keep the original snapshot, and set **payability**.
- An approved correction can make the day **payable** for payroll (mirrors the leave per-day payable model).

**Non-goals**
- Normal clock-in/out (F5.1). Verification of clean exceptions (F5.3, though a reject there often spawns a correction).
- A correction-native approval queue — approvals live in the **E11 Inbox** now (the HR `/corrections` screen becomes a read-only list).

## 3. Actors

Agent (requester, mobile), Shift Leader / HR (approver), System (apply, re-evaluate, audit), Agent (notified).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web — Pengajuan tab (Koreksi)** | Agent | Search own attendance, file a correction or a `new_entry` for a no-shift day, track status. |
| **Mobile app** | Agent | Same (Koreksi Form / Tracker / Detail). |
| **Web / mobile — E11 Inbox** | Shift Leader / HR | Review & decide corrections in the approval Inbox (line-membership gated); may also file on behalf. |
| **Web — `/corrections`** | Shift Leader / HR | Read-only list/history of corrections (decisions moved to the Inbox). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| CR-1 | Correction `type ∈ {check_in, check_out, code, other, **new_entry**}` with a proposed value (or code) and a **required reason**. `new_entry` carries a `work_date` instead of an `attendance_id`. |
| CR-2 | **Approval routes through the E11 engine** (request_type `CORRECTION`): the per-company configurable approval template decides the line(s); no self-approval (E11 INV-3); no template → super-admin fallback line (E11 INV-7). *(Supersedes the prior hardcoded shift-leader → HR routing; 2026-06-15.)* |
| CR-3 | Statuses: `PENDING → APPROVED → APPLIED` (engine final-approve fires the apply hook) or `PENDING → REJECTED` (engine reject, reason on the approval action) or `PENDING → CANCELLED` (requester withdraws before a decision). |
| CR-10 | **NEW_ENTRY create.** A `new_entry` correction creates an `Attendance` row on approval for `work_date` using the requester's **active placement** (company/site/position); `schedule_id = null` (no shift), flagged `UNSCHEDULED` + `CORRECTED`. At least `proposed_check_in_at` is required; it must fall on `work_date`. |
| CR-11 | **No duplicate day.** A `new_entry` for a `work_date` that already has a record → `409 ATTENDANCE_ALREADY_EXISTS` (correct the existing record instead). No active placement on `work_date` → `422 NO_ACTIVE_PLACEMENT`. |
| CR-12 | **Payability on apply.** When applied, the affected day's `Attendance.is_payable` is set: **had a scheduled shift → `true`** (auto-payable); **no shift → `null`** (pending). Mirrors the leave per-day payable model (E6). |
| CR-13 | **Manual payable flag.** For a no-shift applied day (`is_payable = null`), SL/HR/super may flag it via `POST /attendance/{id}:set-payable`. A shift-backed day is auto-payable and is rejected there → `422 ATTENDANCE_HAS_SHIFT_AUTO_PAYABLE`. Payroll treats `null` as non-payable. |
| CR-14 | **7-day window vs work_date.** The agent self-correction window is measured against `attendance.shift_date` for existing records and against `work_date` for `new_entry`; older → `422 OUTSIDE_CORRECTION_WINDOW` (HR exempt). |
| CR-4 | On **Applied**, the correction updates the `Attendance` record and triggers **re-evaluation** (F5.2: lateness/status/routing recomputed). |
| CR-5 | The **original values are preserved** (snapshot) for audit; corrections never erase history. |
| CR-6 | A record may have **multiple** corrections over time; each is independently audited. |
| CR-7 | Corrections to **migrated/historical** records are allowed by HR only (data integrity), within policy. |
| CR-8 | All actions audited; requester notified of the decision. |
| CR-9 | **A `check_in` correction re-evaluates status.** Approving (applying) a `check_in` correction that resolves an **`Absent`** record — or corrects a wrong clock-in time — re-runs F5.2 over the new `check_in_at`: `status` recomputes `Absent → Present` or `Late` against `shift_start_at` + the 15-min grace, and `is_late` / `late_minutes` are recomputed. Specializes CR-4 for the absence-resolution case (an `Absent` record carries `check_in_at = null`; the correction populates it). |

## 6. Data model

`AttendanceCorrection`: `id, attendance_id (FK, **nullable** — null for a pending new_entry), work_date (**nullable**, set for new_entry), requester_id (FK), type, proposed_check_in_at, proposed_check_out_at, proposed_attendance_code_id, reason, status, approval_instance_id (FK → E11 approval_instances), decided_by, decided_at, reject_reason, original_snapshot (json), created_at, updated_at`. Approval bookkeeping now lives in **E11** (`approval_instances`/`approval_actions`); the legacy multi-level `current_level` stays collapsed (DATA-MAPPING G-7).

`Attendance` gains **`is_payable` (nullable boolean)** — `true` payable · `false` not payable · `null` pending an SL/HR/super flag (no-shift day). Payroll consumes it like `approved_leave_days.is_payable` (E6/E8).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Attendance corrections

  Background:
    Given I am the agent "Budi"
    And my 2026-06-10 record was auto-closed because I forgot to clock out

  Scenario: File and approve a missed clock-out correction (via E11)
    When I file a check_out correction with the real time 15:10 and a reason
    Then an E11 approval_instance is opened and the correction is PENDING with approval_instance_id
    When the approver approves it in the Inbox (final line)
    Then my record's check_out_at becomes 15:10 and auto_closed is cleared
    And the record is re-evaluated and re-routed
    And the original auto-closed values are retained as a snapshot
    And the correction status is APPLIED

  Scenario: Reject a correction
    When the approver rejects my correction in the Inbox with a reason
    Then the correction is REJECTED and I see the reason
    And my attendance record is unchanged

  Scenario: Correction changes lateness
    Given my record was marked Late due to a wrong clock-in time
    When a check_in correction to an on-time value is approved
    Then is_late becomes false after re-evaluation

  Scenario: Search and create a record for a no-shift day (NEW_ENTRY), payable pending
    Given I worked 2026-06-10 but there is no attendance record and no shift that day
    When I search my attendance, find the day missing, and file a new_entry with check-in 07:00 and check-out 15:00
    And the approver approves it
    Then a new attendance record is created for 2026-06-10 flagged UNSCHEDULED and CORRECTED
    And its is_payable is null (pending) because the day had no scheduled shift
    When HR flags the day payable via :set-payable
    Then is_payable becomes true and payroll counts the day

  Scenario: Approved correction on a shift day is auto-payable
    Given my 2026-06-11 record had a scheduled shift
    When a correction on it is approved and applied
    Then is_payable is true automatically and :set-payable is rejected with ATTENDANCE_HAS_SHIFT_AUTO_PAYABLE

  Scenario: New_entry for a day that already has a record is blocked
    Given a record already exists for 2026-06-10
    When I file a new_entry for 2026-06-10
    Then I get ATTENDANCE_ALREADY_EXISTS

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
| C-6 | NEW_ENTRY for a day that already has a record | `409 ATTENDANCE_ALREADY_EXISTS` — correct the existing record instead (CR-11). |
| C-7 | Approved correction changes worked time / creates a payable day | `Attendance.is_payable` drives payroll: auto `true` for shift days, `null`→manual flag for no-shift days (CR-12/CR-13). Feeds E8 like `approved_leave_days`. |
| C-8 | Requester is on the correction's E11 approval line | No self-approval (E11 INV-3) — the engine excludes the requester; another line member (or fallback) decides. |

## 9. Dependencies

F5.1/F5.2 (records + re-evaluation), **E11 (approval engine — routing, Inbox, hooks)**, F3.4 (placement → company scope), E7/E10 (downstream recompute), E8 (payroll consumes `is_payable`), E1 (audit), E10 (notifications). Surface: agent **Pengajuan** tab (web + mobile).

## 10. Decisions & open questions

- ✅ Typed corrections; apply→re-evaluate; original snapshot kept.
- ✅ **(2026-06-15) Approval routes through the E11 engine** (request_type `CORRECTION`), replacing the hardcoded shift-leader → HR routing. HR/SL act in the E11 Inbox; `/corrections` becomes a read-only list.
- ✅ **(2026-06-15) `NEW_ENTRY`** lets an agent create a record for a day with no attendance / no shift, via the searchable picker on the Pengajuan tab.
- ✅ **(2026-06-15) Payability** introduced on `Attendance` (`is_payable`), mirroring the leave per-day model: auto for shift days, manual SL/HR/super flag for no-shift days.
- ✅ **Self-correction window = 7 days** (agents), measured vs `work_date` for new_entry; HR exempt (EPICS §8).
- **Open (C-4):** synchronous propagation to already-computed OT (E7) / billing (E10) — v1 flags/notifies; recompute deferred.
