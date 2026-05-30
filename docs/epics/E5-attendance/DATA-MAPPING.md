# E5 — Attendance · ims-system → hris-outsource Data Mapping

> Migration analysis for attendance. Source: SWP prod (`lumen_swp`, MySQL). Target: hris-outsource (Postgres).
> Parent: [FEATURE.md](FEATURE.md) · Related epic: E9 Data Migration. Status: Draft v1.

---

## 1. Shape of the change

Legacy `attendance_users` is a flat check-in/out log keyed on `user_id`, with lat/lng and `is_late`/`checked_out_by_system` flags — but **no link to a schedule/shift**. hris-outsource ties each record to the E4 **schedule** (to judge lateness) and the E3 **placement** (for the site/geofence). Migration must **derive** those links and **not retro-flag** history into the live verification queue. Attendance is also the **highest-volume** table — migration is batched.

## 2. Source tables

| Table | → Target | Key columns |
|-------|----------|-------------|
| `attendance_users` | Attendance | `id, user_id, attendance_code_id, check_in, check_out, long/lat_check_in, long/lat_check_out, is_wfo, is_late, checked_out_by_system` |
| `attendance_corrections` | AttendanceCorrection | `id, attendance_user_id, type, corrected_time, requested_at, status, notes, current_level, rejected_by, rejected_at` |

## 3. Field mapping

### `attendance_users` → Attendance
| Legacy | → | Notes |
|---|---|---|
| `id` | `legacy_attendance_id` | crosswalk |
| `user_id` | `employee_id` | remap via identity crosswalk (users→employees, E2 G-5) |
| `attendance_code_id` | `attendance_code_id` | remap to migrated AttendanceCode |
| `check_in` | `check_in_at` | — |
| `check_out` | `check_out_at` | nullable |
| `lat_check_in`/`long_check_in` | `lat_in`/`lng_in` | string→decimal |
| `lat_check_out`/`long_check_out` | `lat_out`/`lng_out` | nullable |
| `is_late` | `is_late` | carry as-is (recompute only if a schedule match is found) |
| `checked_out_by_system` | `auto_closed` | — |
| `is_wfo` | (drop / informational) | legacy WFO flag; confirm semantics (G-4) |
| — | `schedule_id` | **derive**: match `employee_id` + date(`check_in`) to an E4 schedule; null if none (G-1) |
| — | `placement_id` | **derive**: active placement on `check_in` date (E3) |
| — | `in_geofence_in/out` | **unknown for history** → null (no legacy radius); don't retro-flag (G-3) |
| — | `status` | derive (Present/Late/Incomplete) from times; historical |
| — | `verification_status` | **set `Verified`** for historical (closed) records (G-5) |

### `attendance_corrections` → AttendanceCorrection
| Legacy | → | Notes |
|---|---|---|
| `attendance_user_id` | `attendance_id` | remap |
| `type` (check_in\|check_out) | `type` | — |
| `corrected_time` | `corrected_time` | — |
| `requested_at` | `created_at` | — |
| `status` (pending\|approved\|rejected\|applied) | `status` | normalize to {Pending,Approved,Rejected,Applied} |
| `notes` | `notes` | — |
| `current_level`, `rejected_by`, `rejected_at` | → `decided_by`/`decided_at` | **collapse multi-level approval** to single-level (leader/HR), G-7 |

## 4. Gaps & decisions

| # | Gap | Handling |
|---|-----|----------|
| G-1 | **No schedule link in legacy** | Derive `schedule_id` by employee + date; historical rows that don't match a migrated schedule keep `schedule_id` null (still valid attendance, just unscheduled). |
| G-2 | **Identity remap** | `user_id` → Employee via crosswalk; orphans flagged. |
| G-3 | **Geofence unknown historically** | No legacy radius → set `in_geofence_*` null for history; never retroactively flag old records. |
| G-4 | **`is_wfo` semantics** | Confirm meaning (work-from-office / on-site). Likely drop or map to an informational flag. |
| G-5 | **Don't flood the verification queue** | Historical records imported as `Verified` (already closed); only go-forward records use exceptions-only verification. |
| G-6 | **Volume** | Highest-volume table — migrate in **batches/streamed**, with a confirmed **history window** (all vs last N months). Decide with E9. |
| G-7 | **Multi-level correction approval** | Legacy `current_level` flow collapses to single-level approval (shift leader, escalate HR). |

## 5. Migration rules

1. Runs after E2 (employees/codes), E3 (placements), E4 (schedules) so all crosswalks exist.
2. Batched/streamed by date range; idempotent via `legacy_attendance_id` / correction crosswalk.
3. Historical → `Verified`; geofence flags null; lateness carried (recomputed only on schedule match).
4. Orchestration in E9; this doc owns the E5 field-level mapping.
