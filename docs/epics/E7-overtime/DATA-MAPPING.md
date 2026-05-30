# E7 — Overtime · ims-system → hris-outsource Data Mapping

> Migration analysis for overtime. Source: SWP prod (`lumen_swp`, MySQL). Target: hris-outsource (Postgres).
> Parent: [FEATURE.md](FEATURE.md) · Related epic: E9 Data Migration. Status: Draft v1.

---

## 1. Shape of the change

Legacy `overtimes` is a flat list (employee + duration + start/end **time** + free-text `manager_name`/`project_name` + a `status_id` lookup), with **no rules, no day-type, and no attendance link**. hris-outsource adds **OvertimeRule** (day-type tiers), **day_type classification**, an optional **attendance link** (auto-detect), and **two-level approval** (reconstructed as final state for history). OvertimeRule and the public-holiday calendar are **net-new** (no legacy source).

## 2. Source tables

| Table | → Target | Key columns |
|-------|----------|-------------|
| `overtimes` | OvertimeRecord | `id, employee_id, status_id, manager_name, project_name, notes, duration, start_at(time), end_at(time), issued_date` |
| `overtime_statuses` | (status lookup) | status names → normalized enum |
| — (none) | OvertimeRule, HolidayCalendar | **net-new** — seed/define fresh |

## 3. Field mapping — `overtimes` → OvertimeRecord

| Legacy | → | Notes |
|---|---|---|
| `id` | `legacy_overtime_id` | crosswalk |
| `employee_id` | `employee_id` | remap |
| `status_id` (→`overtime_statuses`) | `status` | normalize → {Pending, LeaderApproved, Approved, Rejected, Cancelled}; historical → final state (G-2) |
| `duration` | `duration_minutes` | **confirm unit** (minutes vs hours) in prod data (G-1) |
| `start_at` / `end_at` (time) | `start_at` / `end_at` | times only in legacy |
| `issued_date` | `work_date` / `created_at` | derive the OT date from `issued_date` (G-7) |
| `notes` | `notes` | — |
| `manager_name`, `project_name` | (drop or fold into `notes`) | free text; `project_name` may hint client/site (informational) (G-6) |
| — | `day_type` | **derive** from `work_date` + schedule (E4) + holiday calendar; historical unknown → default `Workday` + flag (G-3) |
| — | `source` | historical → `Requested` (or `Unknown`) |
| — | `placement_id` | derive active placement on `work_date` (E3) |
| — | `attendance_id` | null for history (no legacy link) |
| — | `OvertimeApproval` rows | legacy single status → impute one final record; per-level history not reconstructable (G-2) |

## 4. Gaps & decisions

| # | Gap | Handling |
|---|-----|----------|
| G-1 | **`duration` unit** | Inspect prod values to confirm minutes vs hours; normalize to `duration_minutes`. |
| G-2 | **Status + approval history** | Map `overtime_statuses` names to the new enum; import final state only; impute a single approval record. |
| G-3 | **No day_type in legacy** | Can't reliably classify historical OT — default `Workday` + flag; only go-forward OT gets accurate tiering. |
| G-4 | **Identity remap** | `employee_id` via crosswalk. |
| G-5 | **No rules / holiday calendar** | Seed `OvertimeRule` tiers + `HolidayCalendar` fresh (define in setup); not migrated. |
| G-6 | **Free-text manager/project** | `manager_name` drop; `project_name` optionally kept as a note. |
| G-7 | **OT date ambiguity** | Legacy stores start/end as TIME only + `issued_date` (datetime) — derive `work_date` from `issued_date`; flag mismatches. |

## 5. Migration rules

1. Runs after E2 (employees), E3 (placements); E5 (attendance) optional for linking (historical stays unlinked).
2. Historical OT imported with final status, hours carried, `day_type` defaulted+flagged.
3. Idempotent via `legacy_overtime_id` crosswalk.
4. Orchestration in E9; this doc owns the E7 field-level mapping.
