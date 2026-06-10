# PRD · F5.6 — Manual Attendance Entry (Buat Kehadiran Manual)

> **Epic:** E5 Attendance · **Feature:** F5.6 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md)

---

## 1. Context & problem

Agents who forget to clock in/out, whose GPS fails to capture, or whose mobile app is unavailable, end up with no attendance record. The result: missing payroll context, gaps in billable reporting, and unnecessary corrections (F5.4) that could be avoided upfront.

HR and shift leaders need a **manual entry path** to create an attendance record for any employee on any date, bypassing GPS/geofence entirely. This is a **traceable override** — the record carries the `MANUAL_ENTRY` flag and `created_by` so the audit trail is clear.

## 2. Goals & non-goals

**Goals**
- HR/SL can create an attendance record for any employee by employee + date + check-in time.
- Automatic resolution of the employee's active placement and today's schedule.
- Lateness and early clock-out evaluation against the schedule when one exists.
- `MANUAL_ENTRY` flag + `created_by` traceability — the record is clearly distinguishable from organic clock-ins.
- Always `PENDING` verification — another HR/leader must verify.

**Non-goals**
- Geofence evaluation (bypassed). Photo capture (not needed). Corrections path for fixing existing records (F5.4). Bulk manual creation.

## 3. Actors

| Actor | Involvement |
|---|---|
| **HR / Super Admin** | Creates manual attendance for any employee across all companies. |
| **Shift Leader** | Creates manual attendance for employees **within their own company** (scope enforcement). |

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin / Shift Leader | Full-page form with employee search, date picker, autofill, time inputs, and note. |

## 5. Business rules

| Ref | Rule | Source |
|-----|------|--------|
| MR-1 | Server resolves employee's active placement; rejects `422 NO_ACTIVE_PLACEMENT` if none. | FEATURE.md |
| MR-2 | `check_out_at >= check_in_at` required; `400 INVALID_REQUEST` if violated. | FEATURE.md |
| MR-3 | Always created with `verification_status=PENDING` + `MANUAL_ENTRY` flag. | FEATURE.md |
| MR-4 | If schedule exists for today, lateness evaluation runs against it (15 min grace); sets `LATE`/`EARLY` flags as applicable. | FEATURE.md |
| MR-5 | No schedule → unscheduled manual entry; no lateness evaluation. | FEATURE.md |
| MR-6 | Geofence bypassed: `geofence_in = { inside: true, distance_m: 0, radius_m: 0 }`, `lat_in`/`lng_in` are `null`. | FEATURE.md |
| MR-7 | `worked_minutes` computed server-side from check_in → check_out; 0 if negative. | FEATURE.md |
| MR-8 | `WFO = true` always (manual entry implies on-site). | FEATURE.md |
| MR-9 | Audit record written with source `manual_entry`. | FEATURE.md |
| MR-10 | Idempotency required (same `Idempotency-Key` + body → safe replay). | FEATURE.md |
| MR-11 | Note optional, stored as `note` text. | FEATURE.md |
| MR-12 | `created_by` set from JWT principal of creating user (HR/SL), stored on `attendance.created_by`. | FEATURE.md |
| MR-13 | Shift leader can create only for employees whose active placement belongs to the SL's own company; `422 OUT_OF_SCOPE` if violated. | FEATURE.md |

## 6. Fields

### ManualCreateRequest (POST body)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `employee_id` | string (SWP-EMP) | ✅ | Target employee |
| `check_in_at` | string (RFC3339) | ✅ | Clock-in timestamp |
| `check_out_at` | string (RFC3339) | ❌ | Clock-out timestamp |
| `note` | string | ❌ | Free-text reason |

### AutofillResponse (GET autofill response)

| Field | Type | Description |
|-------|------|-------------|
| `employee_name` | string | Employee display name |
| `company_name` | string | Placement company name |
| `site_name` | string\|null | Placement site name |
| `position_name` | string\|null | Placement position name |
| `service_line` | string | Service line enum |
| `schedule_id` | string\|null | Today's schedule ID (null if none) |
| `shift_start_at` | string\|null | Schedule shift start (null if none) |
| `shift_end_at` | string\|null | Schedule shift end (null if none) |

## 7. Gherkin acceptance criteria

### AC-1: Happy path — HR creates manual attendance with schedule

```gherkin
Given the employee "Budi Santoso" (SWP-EMP-1042) has an active placement at "Plaza Senayan"
  And the employee has a schedule today with shift 07:00–15:00
When HR opens the manual attendance page
  And searches for "Budi" and selects SWP-EMP-1042
  And selects date "2026-06-04"
  And the autofill returns company "Plaza Senayan", site "Lobby Utama", schedule 07:00–15:00
  And HR enters check_in_at "2026-06-04T08:00:00+07:00"
  And submits
Then the system creates an attendance record with:
  | Field              | Value                                             |
  |--------------------|---------------------------------------------------|
  | employee_id        | SWP-EMP-1042                                      |
  | status             | LATE                                              |
  | late_minutes       | 60                                                |
  | verification_status| PENDING                                           |
  | flags              | [MANUAL_ENTRY, LATE]                              |
  | schedule_id        | SWP-SCH-22041                                     |
  | geofence_in        | { inside: true, distance_m: 0, radius_m: 0 }     |
  | lat_in             | null                                              |
  | lng_in             | null                                              |
  | wfo                | true                                              |
  | created_by         | (HR's SWP-EMP id)                                 |
  | check_in_at        | 2026-06-04T01:00:00Z (UTC)                        |
```

### AC-2: Happy path — HR creates manual attendance with check-out

```gherkin
Given the employee has an active placement and a schedule today (07:00–15:00)
When HR enters check_in_at "2026-06-04T07:00:00+07:00" and check_out_at "2026-06-04T15:00:00+07:00"
  And submits
Then the record has:
  | Field          | Value            |
  |----------------|------------------|
  | status         | PRESENT          |
  | late_minutes   | 0                |
  | worked_minutes | 480              |
  | flags          | [MANUAL_ENTRY]   |
```

### AC-3: No active placement → 422

```gherkin
Given the employee has NO active placement
When HR submits a manual attendance for this employee
Then the response is 422 with error code NO_ACTIVE_PLACEMENT
```

### AC-4: Check-out before check-in → 400

```gherkin
Given the employee has an active placement
When HR enters check_out_at before check_in_at
Then the response is 400 with error code INVALID_REQUEST
  And the response includes field "check_out_at"
```

### AC-5: No schedule → unscheduled, no lateness evaluation

```gherkin
Given the employee has an active placement but NO schedule today
When HR creates manual attendance with check_in_at "2026-06-04T10:00:00+07:00"
Then the record has:
  | Field       | Value                          |
  |-------------|--------------------------------|
  | schedule_id | null                           |
  | status      | PRESENT                        |
  | flags       | [MANUAL_ENTRY, UNSCHEDULED]    |
  | late_minutes| 0                              |
```

### AC-6: Shift leader creates within own company → success

```gherkin
Given the shift leader leads "Plaza Senayan" (SWP-CMP-0021)
  And the employee's active placement is at "Plaza Senayan"
When the shift leader creates a manual attendance for this employee
Then the response is 201
  And created_by is set to the shift leader's SWP-EMP id
```

### AC-7: Shift leader creates for employee outside own company → 422 OUT_OF_SCOPE

```gherkin
Given the shift leader leads "Plaza Senayan" (SWP-CMP-0021)
  And the employee's active placement is at "Grand Indonesia" (SWP-CMP-0022)
When the shift leader creates a manual attendance for this employee
Then the response is 422 with error code OUT_OF_SCOPE
```

### AC-8: Autofill with no placement → 422

```gherkin
Given the employee has NO active placement
When HR calls GET /attendance:manual-autofill?employee_id=SWP-EMP-NOPL&date=2026-06-04
Then the response is 422 with error code NO_ACTIVE_PLACEMENT
```

### AC-9: Autofill missing params → 400

```gherkin
When HR calls GET /attendance:manual-autofill?employee_id=SWP-EMP-1042 (no date)
Then the response is 400 with error code INVALID_REQUEST
```

### AC-10: Idempotency replay → same response

```gherkin
Given the same Idempotency-Key and same request body
When the request is replayed
Then the response is the same 201 with Idempotent-Replayed: true header
```

### AC-11: Autofill returns schedule when available

```gherkin
Given the employee has an active placement and a schedule for 2026-06-04 (shift 07:00–15:00)
When HR calls GET /attendance:manual-autofill?employee_id=SWP-EMP-1002&date=2026-06-04
Then the response contains:
  | Field           | Value              |
  |-----------------|--------------------|
  | employee_name   | (employee name)    |
  | company_name    | (company name)     |
  | schedule_id     | SWP-SCH-xxxx       |
  | shift_start_at  | 2026-06-04T07:00:00Z |
  | shift_end_at    | 2026-06-04T15:00:00Z |
```

### AC-12: Autofill returns no schedule when absent

```gherkin
Given the employee has an active placement but NO schedule for 2026-06-04
When HR calls GET /attendance:manual-autofill?employee_id=SWP-EMP-1002&date=2026-06-04
Then the response contains schedule_id: null, shift_start_at: null, shift_end_at: null
```

## 8. Cases (edge & error)

| Ref | Case | Handling |
|-----|------|----------|
| C-1 | Employee ID not found | Server resolves active placement; not found → `422 NO_ACTIVE_PLACEMENT` (deliberately not `404` — the employee may exist but have no placement). |
| C-2 | Check-out before check-in | `400 INVALID_REQUEST` with field `check_out_at`. |
| C-3 | Future check-in date | Allowed (HR may write attendance for today + past only in practice; no server restriction on future). |
| C-4 | `check_out_at` without `check_in_at` | Not possible — `check_in_at` is required. |
| C-5 | Employee has multiple placements | Server resolves the **single active placement** (E3 INV-1: no overlapping active placements). |
| C-6 | `check_out_at` equals `check_in_at` | Valid (0 minutes worked). |
| C-7 | SL scope: SL's own employee ID is the target | Allowed — SL can create manual attendance for themselves (but it still goes to PENDING for another HR/leader to verify). |
| C-8 | Super admin creating for any company | Always allowed (no scope restriction). |
| C-9 | Already exists for same employee+date+shift? | Not blocked — multiple manual records for the same shift date are technically possible (the attendance table has no unique constraint on employee_id+date). The downstream evaluation (F5.2) and verification (F5.3) handle duplicates via scheduling evaluation rules. HR is expected to self-police. |

## 9. Dependencies

| Dependency | Why |
|------------|-----|
| E3 (Placement) | Resolve employee's active placement for company/site/position/service line. |
| E4 (Schedule) | Resolve today's schedule for lateness/early evaluation. |
| E5 F5.2 (Evaluation) | Lateness, early clock-out, and status computation (reused logic). |
| Migration 00046 | `created_by` column on `attendance` table. |

## 10. Design references

- **Asana C3W — 18 Kehadiran Manual · Buat**: page-based form with employee search, date picker, placement card, schedule card, check-in/out times, note, submit.
- The frontend screen component is at `frontend/apps/web/src/features/e5-attendance/manual-attendance-create-screen.tsx`.

## 11. Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| PRD exists? | ✅ Yes | Inline rules in FEATURE.md were sufficient during initial implementation; promoted to full PRD on 2026-06-10. |
| Shift leader allowed? | ✅ Yes (scoped) | Actual practice requires SL to create attendance for missed check-ins; MR-13 prevents cross-company abuse. |
| `attendance_code_id` on request? | ❌ Removed | No code picker in UI; `MANUAL_ENTRY` flag covers the classification. |
| Page-based vs modal? | ✅ Full page | Form has enough fields (employee search, date, autofill placement card, schedule card, times, note, submit) to warrant a full page. |
| `created_by` traceability? | ✅ Stored on record | JWT principal of the creating user enables audit trail of who manually overrode clock-in. |
| Geofence bypass? | ✅ Always | Manual entry assumes on-site presence. |
| Verification status? | ✅ Always PENDING | Another HR/leader must verify; no auto-approve for manual entries. |
