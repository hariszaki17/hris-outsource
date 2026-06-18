# PRD · F5.8 — Attendance Activity Log (clock-out gate)

> **Epic:** E5 Attendance · **Feature:** F5.8 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Today an attendance record says *when* an agent worked (clock-in/out, lateness) but not *what* they did.
SWP needs a lightweight, per-shift record of an agent's activities so a supervisor reviewing the day can
see the work performed — and so clock-out can't happen on an empty record. F5.8 adds an **Attendance
Activity Log**: while a shift is open the agent logs one or more short **notes** of what they did, each
stamped with a server-set **`recorded_at`** (when it was logged). **Agent clock-out is blocked unless at
least one activity exists** (INV-7). Activities can be logged **anytime the record is open** — from
immediately after clock-in until clock-out — so the agent can journal as the day goes, not only at the end.

## 2. Goals & non-goals

**Goals**
- Let an agent attach ≥0 timestamped activity notes to their **own open** attendance record.
- **Gate agent clock-out on ≥1 activity** (INV-7) — reject with a clear, recoverable error.
- Capture **when** each activity was recorded (server time, `Asia/Jakarta`).
- Allow logging **anytime while open** (right after clock-in through clock-out), and let the agent
  **delete their own** activity while the record is still open (mistake fix).
- Surface the activity list on the agent's clock surface (web + mobile) and let supervisors read it.

**Non-goals**
- Structured task taxonomy / categories / time-tracking per activity (v1 is free-text note only).
- Editing an activity after creation (delete + re-add instead) or editing after clock-out (locked).
- Activities on **system auto-close** (INV-4) or **HR/leader manual entry** (F5.6) — both **exempt**,
  they never require activities (coverage must never be lost to a missing note).
- Approval/verification of activities (they are a log, not a request).

## 3. Actors

Agent (logs/deletes own activities; gated on clock-out), Shift Leader / HR / Super Admin (read activities
within their attendance scope), System (sets `recorded_at`, enforces the clock-out gate, audits).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | Log activity (note) while open; see list; delete own; clock-out gated. |
| **Web console** (`/me` Kehadiran) | Agent | Same as mobile (web clock card). |
| **Web / mobile** | Shift Leader / HR | Read an attendance record's activity list (audit). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| AA-1 | **One attendance → many activities.** An `AttendanceActivity` belongs to exactly one `Attendance`; a record has 0..N activities ordered by `recorded_at`. |
| AA-2 | **Scope: self for writes.** An agent may create/delete activities only on **their own** attendance record (`attendance.employee_id == principal.employee_id`), resolved from the JWT principal — never a body id (scope:self). Cross-employee write → `403`/`404`. |
| AA-3 | **Open-record only for writes.** Activities may be created or deleted only while the record is **open** (`check_in_at` set, `check_out_at == null`). After clock-out the log is **immutable** → `422 ATTENDANCE_CLOSED`. A record with no clock-in (true `ABSENT`) cannot hold activities → `422 ATTENDANCE_NOT_OPEN`. |
| AA-4 | **Anytime while open.** No minimum delay after clock-in — an activity may be logged immediately after clock-in (AA-3 is the only timing gate). |
| AA-5 | **`recorded_at` is server-set.** The server stamps `recorded_at = now()` (`Asia/Jakarta` canonical) at creation; the client never supplies it. It is the "when did the agent do/log this" timestamp. |
| AA-6 | **Note is required free-text**, 1..500 chars after trimming; empty/whitespace → `400 INVALID_REQUEST`; over-length → `400 INVALID_REQUEST` (`fields.note`). |
| AA-7 | **Clock-out gate (INV-7).** An **agent** `:clock-out` is rejected `422 ACTIVITY_REQUIRED` when the open record has **zero** non-deleted activities. The error carries `fields.activity_count = "0"`. The clock-out is otherwise unchanged (F5.1). |
| AA-8 | **Exemptions.** The gate applies to **agent self clock-out only**. **System auto-close (INV-4)** and **HR/leader manual entry / manual close (F5.6)** never require activities and are never blocked. |
| AA-9 | **Delete is soft + own + while-open.** Deleting sets `deleted_at`; the row is excluded from lists and from the AA-7 count. Only the creator may delete, only while the record is open (AA-2/AA-3). Idempotent — deleting an already-deleted/absent id → `404`. |
| AA-10 | **Read scope mirrors attendance read.** Anyone who may read the parent `Attendance` (the agent for self; shift_leader/HR/super_admin/lead per their attendance scope, F5.3/F5.5) may list its activities. |
| AA-11 | **Idempotency.** `POST` (create) and `DELETE` require an `Idempotency-Key` (CONVENTIONS §13); a replayed key returns the original result. |
| AA-12 | **Audit.** Create and delete write an audit record (actor, attendance_id, activity_id) per CONVENTIONS §16. |
| AA-13 | **List is cursor-paginated**, default sort `recorded_at:asc` (chronological day trail). |

## 6. Data model

**New entity `AttendanceActivity`** (`SWP-ACT-*`):

| Field | Type | Notes |
|---|---|---|
| `id` | `SWP-ACT-*` | PK, server-generated. |
| `attendance_id` | `SWP-ATT-*` | FK → `Attendance`. |
| `employee_id` | `SWP-EMP-*` | Creator (the agent); set from the principal (AA-2). |
| `note` | text | 1..500 chars (AA-6). |
| `recorded_at` | timestamptz | Server-set capture time (AA-5). |
| `created_at` | timestamptz | Audit. |
| `deleted_at` | timestamptz null | Soft-delete (AA-9). |

Parent `Attendance` is unchanged except that **agent clock-out reads the activity count** for AA-7. No
other schema change. Endpoints (eng owns the contract):
`POST /attendance/{id}/activities` (create, scope:self), `GET /attendance/{id}/activities` (list,
scope:attendance-read), `DELETE /attendance/{id}/activities/{activityId}` (delete, scope:self). New error
codes `ACTIVITY_REQUIRED`, `ATTENDANCE_CLOSED`, `ATTENDANCE_NOT_OPEN`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Attendance activity log

  Scenario: Log an activity right after clock-in
    Given I am the agent "Budi" with an open attendance record from today's clock-in
    When I add an activity note "Patroli lantai 1"
    Then the activity is saved with a server recorded_at timestamp
    And it appears in my activity list for today

  Scenario: Multiple activities on one record
    Given my attendance record already has the activity "Patroli lantai 1"
    When I add "Cek APAR" and then "Laporan shift"
    Then my record has three activities ordered by recorded_at

  Scenario: Clock-out blocked with no activities (INV-7)
    Given I have an open attendance record with zero activities
    When I try to clock out
    Then the clock-out is rejected with ACTIVITY_REQUIRED
    And I am prompted to log what I did first
    And I am still clocked in

  Scenario: Clock-out succeeds after at least one activity
    Given I have an open attendance record with one activity
    When I clock out
    Then the clock-out succeeds

  Scenario: Activity is scope-self
    Given another agent has an open attendance record
    When I try to add an activity to their record
    Then it is rejected and no activity is created

  Scenario: No activities after clock-out (immutable)
    Given my attendance record is already clocked out
    When I try to add an activity
    Then it is rejected with ATTENDANCE_CLOSED

  Scenario: Delete my own activity while open
    Given my open record has the activity "Cek APAR"
    When I delete that activity
    Then it no longer appears in my list
    And it no longer counts toward the clock-out gate

  Scenario: Empty note rejected
    When I add an activity with a blank note
    Then it is rejected with INVALID_REQUEST

  Scenario: System auto-close is exempt (AA-8)
    Given I have an open record with zero activities at the scheduled shift end
    When the system auto-closes the record
    Then it is closed without requiring an activity

  Scenario: Supervisor reads the activity log
    Given an agent's record has three activities
    When their shift leader opens the record
    Then the shift leader sees the three activities
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Add activity immediately after clock-in (zero delay) | Allowed (AA-4). |
| C-2 | Clock-out with exactly one activity | Allowed (AA-7). |
| C-3 | Clock-out after the only activity was deleted (count back to 0) | Blocked `ACTIVITY_REQUIRED` (AA-7, AA-9). |
| C-4 | Note with leading/trailing whitespace only | `400 INVALID_REQUEST` (AA-6 trims). |
| C-5 | Note exactly 500 / 501 chars | 500 ok; 501 → `400` (AA-6). |
| C-6 | Add activity to an `ABSENT` record (no clock-in) | `422 ATTENDANCE_NOT_OPEN` (AA-3). |
| C-7 | Add/delete activity after clock-out | `422 ATTENDANCE_CLOSED` (AA-3). |
| C-8 | Delete another agent's activity | `403`/`404`, no change (AA-2, AA-9). |
| C-9 | Replayed create with same Idempotency-Key | Same activity returned once, not duplicated (AA-11). |
| C-10 | System auto-close at shift end with zero activities | Closes, no gate (AA-8); record `Incomplete`/`Pending` per F5.2. |
| C-11 | HR/leader manual entry (F5.6) with no activities | Created, no gate (AA-8). |
| C-12 | Concurrent: agent adds activity while clock-out in flight | Server resolves on the persisted count at clock-out time; no partial state (AA-7 reads inside the clock-out tx). |
| C-13 | Long activity list (e.g. 40 notes) | Cursor-paginated, chronological (AA-13). |

## 9. Dependencies

- **F5.1** clock-out (the gate hooks the agent clock-out path) and the open-record model.
- **F5.2** auto-close (INV-4) and **F5.6** manual entry — both exempt from the gate (AA-8).
- **F5.3 / F5.5** attendance read scope (supervisor read, AA-10).
- **E1** scope:self, idempotency, audit, error envelope (CONVENTIONS §11/§13/§16).
- **AGENT-WEB-ACCESS.md** — the agent web clock card hosts the activity panel (AW-6: no `.pen` frame
  for `/me` web). Mobile clock screen hosts it on mobile.

## 10. Decisions & open questions

- ✅ **Free-text note only** in v1 (no categories/structured tasks). *(2026-06-18)*
- ✅ **`recorded_at` server-set** — "when recorded" is authoritative server time, not client-supplied. *(2026-06-18)*
- ✅ **Gate is agent-clock-out-only**; auto-close + manual entry exempt so coverage is never blocked. *(2026-06-18, INV-7)*
- ✅ **Addable anytime while open**, deletable by creator while open; immutable after clock-out. *(2026-06-18)*
- ✅ **Web + mobile**; supervisors read-only. *(2026-06-18)*
- **Open (eng):** whether the supervisor list endpoint needs its own `x-rbac` scope vs reusing the
  attendance-read group; exact per-page limit for the activity list.
