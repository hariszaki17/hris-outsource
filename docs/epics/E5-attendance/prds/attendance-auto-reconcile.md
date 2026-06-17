# PRD · F5.7 — Attendance Auto-Reconcile (late-roster linking)

> **Epic:** E5 Attendance · **Feature:** F5.7 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_ · **Added:** 2026-06-17

---

## 1. Context & problem

Flexible clock-in (F5.1 / CI-4) lets an agent clock in when **no schedule exists** for that day: the record is saved with `schedule_id = null`, flagged `UNSCHEDULED`, and forced into the shift-leader verification queue (`verification_status = PENDING`, F5.3). In a 24/7 outsourcing operation the roster is frequently entered **late** — emergency coverage, swaps, a supervisor who publishes the week after shifts have already started. When HR/SL finally enters that shift, **nothing links the now-existing shift to the already-captured record.** The orphaned record sits in the queue as "unscheduled" forever, and the leader rubber-stamps it.

That rubber-stamping is the failure mode that turns flexible clock-in into theater. **Auto-reconcile closes the loop:** creating a shift that covers a day with an existing machine-owned unscheduled record makes the shift retroactively *adopt* that record and **re-derive its truth** (link, lateness, status, verification, payability) using the exact same rules clock-in would have applied had the shift existed at the time.

This keeps Attendance (E5) **loosely coupled** to Scheduling (E4): a late roster never blocks or loses a frontline record (capture-then-reconcile), and the verification queue only holds records that genuinely need a human.

## 2. Goals & non-goals

**Goals**
- When a workable shift is created that covers an existing **machine-owned** `UNSCHEDULED` attendance record (same employee + placement, within the shift window), link it and re-derive `status` / `is_late` / `late_minutes` / `flags` / `verification_status` / `is_payable`.
- Re-derivation is **identical** to clock-in's evaluation (same 15-minute grace) so a reconciled record is indistinguishable from one captured against a shift that existed all along.
- Preserve human decisions: a record a person already verified/rejected/escalated is **never** silently re-derived.
- Run atomically inside the schedule-create transaction; fully audited; idempotent.

**Non-goals**
- Multiple shifts per day (true multi-shift). Reconcile is built **forward-compatible** with it (window-based matching, §5 BR-AR-3) but the enabling scheduling changes are out of scope — see §10 TODOs.
- Reconcile on schedule **edit** / shift-master time change (those records were never unscheduled; existing E4→E5 propagation owns them — F5.1 CI-9).
- Dequeue/notify the shift leader when a record leaves the queue — **stubbed**, TODO (§10).
- Any change to the clock-in admission rules (one-open-record boundary, CI-5) — unchanged; orthogonal to this feature.

## 3. Actors

System (on schedule create: match → reconcile → audit), HR/Placement admin & Shift Leader (create the late roster that triggers it; see the result in F5.3/F5.5), Agent (sees the corrected status on mobile, no action).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Backend** | System | Trigger + matching + re-derivation + audit (this PRD). No new endpoint. |
| **Web** | Shift Leader / HR | Reconciled record leaves the verification queue (if it auto-approves) or stays with corrected flags (F5.3/F5.5) — driven entirely by server state. |
| **Mobile** | Agent | Sees the updated `status` / `verification_status` on the attendance detail — driven by server state. |

No external API contract change: reconcile is a server-side side-effect of the existing `POST /schedule` (and `:bulk-apply`). Surfaced only through the existing attendance read endpoints + a new audit action.

## 5. Business rules

| Ref | Rule |
|-----|------|
| **AR-1** | **Trigger.** Reconcile runs inside the same transaction as a schedule-entry **create** (`scheduling.CreateEntry`, which also backs `:bulk-apply` per cell) **only** when the new entry is a **workable shift**: `is_day_off = false`, `status = 'SCHEDULED'`, both `start_time`/`end_time` set. It does **not** run on update, shift-master time change, `is_day_off = true`, `CANCELLED_BY_LEAVE`, or a `force_replace` of an existing entry. |
| **AR-2** | **Candidate.** An `attendance` row qualifies iff **all** hold: same `employee_id` **and** same `placement_id` as the new entry; `schedule_id IS NULL`; `deleted_at IS NULL`; `'UNSCHEDULED' = ANY(flags)`; and it is **machine-owned** — `verification_status = 'PENDING'` **and** `verified_by IS NULL` **and** `rejected_by IS NULL` (i.e. not `VERIFIED`/`REJECTED`/`ESCALATED`, never touched by a human). |
| **AR-3** | **Window-only matching (multi-shift-safe).** Of the candidates, a shift adopts the record whose `check_in_at` lies within **`[shift_start − 2h, shift_end + 4h]`** (`shift_start`/`shift_end` computed from `work_date + start_time/end_time (+cross_midnight)` AT TIME ZONE `Asia/Jakarta`; `4h` reuses the flexible-checkout grace). If several qualify, the **earliest** `check_in_at`. **If none lies in the window, reconcile nothing** — the record stays `UNSCHEDULED`. There is **no "earliest of the day" fallback**: a check-in outside every shift window genuinely does not belong to this shift (correct for both single- and future multi-shift). |
| **AR-4** | **Uniqueness guard.** Skip if the new `schedule_id` already owns a non-deleted attendance row (respects partial unique index `attendance_schedule_uq`). The linking `UPDATE` is count-guarded with `WHERE schedule_id IS NULL` so a concurrent link cannot double-apply. |
| **AR-5** | **Shift snapshot.** On link, set `schedule_id` and snapshot `shift_start_at` / `shift_end_at` from the entry (same formula as `GetTodayScheduleForEmployee`). |
| **AR-6** | **Lateness re-derivation.** Recompute with the clock-in grace (15 min, `defaultGrace`): `mins = check_in_at − shift_start_at`; if `mins > grace` → `is_late = true`, `late_minutes = mins`, `status = 'LATE'`, add `LATE` flag; else `is_late = false`, `late_minutes = 0`, `status = 'PRESENT'`. An `INCOMPLETE` status (auto-closed open record) is preserved, not overwritten to PRESENT/LATE. |
| **AR-7** | **Flags + verification.** `flags := array_remove(flags, 'UNSCHEDULED')` (plus any freshly-derived `LATE`). Then: if **no flags remain** → `verification_status = 'AUTO_APPROVED'`; if **any flag remains** (e.g. `OUTSIDE_GEOFENCE`, `AUTO_CLOSED`, `LATE`) → stays `PENDING`. `OUTSIDE_GEOFENCE` and `AUTO_CLOSED` are independent facts and are **never** stripped by reconcile. |
| **AR-8** | **Payability.** If `is_payable IS NULL` (no-shift day pending an SL/HR flag, migr. 00064/00066), set `is_payable = true` — the day is now shift-backed. An explicit `true`/`false` is never overridden. |
| **AR-9** | **Human-decided records (lineage only).** A record that fails the machine-owned test in AR-2 (already `VERIFIED`/`REJECTED`/`ESCALATED`) is **not** re-derived: its `status`, `verification_status`, flags and `is_payable` are left exactly as the human set them. For data lineage the system **does** attach `schedule_id` + the shift snapshot (AR-5) so reporting/payroll ties the record to its shift, and audits it as **`ATTENDANCE_RELINKED`** (distinct from reconcile). *(Decision 2026-06-17: chosen over a full skip — payroll/reporting need the shift tie; the human decision is authoritative and stays untouched.)* |
| **AR-10** | **Atomicity & audit.** Matching + linking happen in the schedule-create tx; an unexpected reconcile error rolls back the schedule create (correctness over availability — a shift that silently fails to adopt its record is worse than a retried create). Each adoption is audited (`ATTENDANCE_RECONCILED` / `ATTENDANCE_RELINKED`) with before/after `status` + `verification_status` + `flags`, referencing the triggering `schedule_id`. |

## 6. Data model

Updates an existing `Attendance` row (see [FEATURE.md](../FEATURE.md) §4) — **no migration**: `schedule_id`, `shift_start_at`, `shift_end_at`, `status`, `is_late`, `late_minutes`, `flags` (text[]), `verification_status`, `is_payable`, `updated_at`. Reads `schedule_entries` (the new entry) for the shift window. All columns already exist.

### State transitions

| Before (unscheduled, machine-owned) | New shift says | After |
|---|---|---|
| `PENDING · [UNSCHEDULED] · PRESENT` | on-time | **`AUTO_APPROVED · [] · PRESENT`** · payable |
| `PENDING · [UNSCHEDULED] · PRESENT` | > 15 min late | **`PENDING · [LATE] · LATE`** · payable |
| `PENDING · [UNSCHEDULED, OUTSIDE_GEOFENCE]` | on-time | **`PENDING · [OUTSIDE_GEOFENCE]`** (still needs SL) |
| `PENDING · [UNSCHEDULED, AUTO_CLOSED] · INCOMPLETE` | any | **`PENDING · [AUTO_CLOSED] · INCOMPLETE`** (incompleteness survives) |
| Human `VERIFIED`/`REJECTED`/`ESCALATED` | any | verification/status **unchanged**; `schedule_id` lineage attached, audited `RELINKED` |

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Attendance auto-reconcile on late roster

  Background:
    Given the shift grace is 15 minutes
    And the match window is [shift_start - 2h, shift_end + 4h]

  Scenario: On-time unscheduled clock-in adopted by a late roster
    Given Dewi clocked in at 07:03 today with no schedule (UNSCHEDULED, PENDING)
    When HR creates a SCHEDULED entry for Dewi today 07:00-15:00
    Then her attendance schedule_id links to that entry
    And flags no longer contain UNSCHEDULED
    And status = PRESENT and verification_status = AUTO_APPROVED
    And is_payable = true
    And an ATTENDANCE_RECONCILED audit entry is written

  Scenario: Late unscheduled clock-in stays in queue but correctly labelled
    Given Budi clocked in at 07:40 today with no schedule (UNSCHEDULED, PENDING)
    When HR creates a SCHEDULED entry for Budi today 07:00-15:00
    Then status = LATE with late_minutes = 40 and flags = [LATE]
    And verification_status remains PENDING

  Scenario: Out-of-geofence survives reconcile
    Given an UNSCHEDULED, OUTSIDE_GEOFENCE clock-in exists for today, on time
    When a covering schedule is created
    Then UNSCHEDULED is removed but OUTSIDE_GEOFENCE remains
    And verification_status remains PENDING

  Scenario: Day-off entry does not adopt the record
    Given an UNSCHEDULED clock-in exists for today
    When HR marks today as is_day_off for that agent
    Then the attendance record is unchanged (still UNSCHEDULED, PENDING)

  Scenario: Check-in outside the shift window is not adopted
    Given an UNSCHEDULED clock-in at 03:00 exists for today
    When HR creates a SCHEDULED entry for today 14:00-22:00
    Then no record is reconciled (03:00 is outside [12:00, 02:00+1d])
    And the 03:00 record stays UNSCHEDULED

  Scenario: Human-decided record is not silently re-derived
    Given an UNSCHEDULED record already VERIFIED by the shift leader
    When a covering schedule is created
    Then verification_status stays VERIFIED and status is unchanged
    And only schedule_id lineage is attached (audited as ATTENDANCE_RELINKED)

  Scenario: Bulk roster reconciles each cell
    Given UNSCHEDULED clock-ins exist for Dewi and Budi today
    When HR bulk-applies a shift to both for today
    Then each agent's record is reconciled in its own transaction
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-AR-1 | Late roster entered **after** shift end | Reconciles (match by window, not "now") — the core scenario. |
| C-AR-2 | New entry is `is_day_off = true` | No reconcile; record stays `UNSCHEDULED` for SL (worked on a declared day off — genuinely needs review). |
| C-AR-3 | New entry `CANCELLED_BY_LEAVE` | No reconcile (not a real shift). |
| C-AR-4 | Cross-midnight: clock-in 00:30 for a 23:00→07:00 shift dated yesterday | Local date ≠ `work_date`, but the window-OR (AR-3) catches it via `shift_end + 4h`. |
| C-AR-5 | `:bulk-apply` creates many entries | Each cell reconciles in its own tx; partial failures isolated (existing BulkApply semantics). |
| C-AR-6 | `force_replace` (MODIFIED) of an existing entry | Skip; that day's record was already linked. |
| C-AR-7 | SL is mid-review when reconcile fires | `GetAttendanceForUpdate` row-lock serializes; SL sees the reconciled state; audit explains the change. |
| C-AR-8 | Agent **re-placed** between clock-in and roster entry (placement mismatch) | No match (`placement_id` guard, AR-2); stays `UNSCHEDULED`, logged. |
| C-AR-9 | Two unscheduled clock-ins same day (e.g. after an auto-close) and one new shift | The shift adopts the one in its window (earliest if several); the other stays `UNSCHEDULED`. Multi-shift-safe (AR-3). |

## 9. Dependencies

E4 (`scheduling.CreateEntry` trigger point + shift window), E5 F5.1 (clock-in grace + `UNSCHEDULED` semantics), F5.2 (lateness/auto-close), F5.3 (verification queue this feeds), E1 (audit), E10 (deferred dequeue notification). No new migration (all columns exist).

## 10. Decisions & open questions

- ✅ **Trigger = schedule create only**, inside the create tx (2026-06-17). Edits/time-changes already owned by E4→E5 propagation.
- ✅ **Window-only matching, no day fallback** — multi-shift-safe from day one (2026-06-17).
- ✅ **Human-decided records → lineage-only** (`schedule_id` attached, decision untouched, `RELINKED` audit) (2026-06-17).
- ✅ **Match window** `[shift_start − 2h, shift_end + 4h]`; **grace** 15 min reused from clock-in (2026-06-17).
- ✅ **Synchronous, same-tx, fail-closed**; River outbox available but not used in v1.
- **TODO(notify):** dequeue / notify the shift leader when a reconciled record flips `PENDING → AUTO_APPROVED`. Slots into the existing Phase-11 schedule-notification TODO (`schedule_service.go:245`). Deferred 2026-06-17 at owner's request.
- **TODO(multi-shift):** relax `schedule_entries` partial unique `(employee_id, work_date)` → allow multiple **non-overlapping** shifts per day (migration). Enables true multi-shift; reconcile already forward-compatible (AR-3).
- **TODO(multi-shift):** `GetTodayScheduleForEmployee` → select the shift whose window contains the clock-in instant rather than `ORDER BY start_time LIMIT 1` (earliest), so clock-in links the correct shift on a multi-shift day.
- **Note (CI-5 boundary):** the "must check out before the next check-in" rule the multi-shift direction needs is **already enforced** — `ClockIn` permits only one open record at a time (`409 ALREADY_CLOCKED_IN`; stale opens auto-close). Multiple check-ins per day already work once the prior is closed. No change here.
