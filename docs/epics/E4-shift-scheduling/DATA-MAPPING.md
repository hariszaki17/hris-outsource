# E4 — Shift & Scheduling · ims-system → hris-outsource Data Mapping

> Migration analysis for shifts + schedules. Source: SWP prod (`lumen_swp`, MySQL). Target: hris-outsource (Postgres).
> Parent: [FEATURE.md](FEATURE.md) · Related epic: E9 Data Migration. Status: Draft v1.

---

## 1. Shape of the change

Legacy `shifts` are **per company** (`company_id`); hris-outsource makes the shift master a single **global** catalog, so legacy shifts collapse + dedupe. Legacy `schedules` key on `user_id`; the new `Schedule` keys on `employee_id` and additionally links the **placement** active on that date (legacy had no such link).

## 2. Source tables

| Table | → Target | Key columns |
|-------|----------|-------------|
| `shifts` | ShiftMaster | `id, company_id, title, start_at(time), end_at(time), start_break, end_break, status` |
| `schedules` | Schedule | `id, user_id, shift_id, date` |

## 3. Field mapping

### `shifts` → ShiftMaster
| Legacy | → | Notes |
|---|---|---|
| `id` | `legacy_shift_id` | crosswalk |
| `title` | `title` | dedupe across companies by (title + times) |
| `start_at` | `start_at` | — |
| `end_at` | `end_at` | if `end_at <= start_at` ⇒ set `spans_midnight = true` |
| `start_break` / `end_break` | same | nullable |
| `company_id` | **DROP** | global catalog; used only to dedupe identical shifts |
| `status` | `status` | active/inactive |

### `schedules` → Schedule
| Legacy | → | Notes |
|---|---|---|
| `id` | `legacy_schedule_id` | crosswalk |
| `user_id` | `employee_id` | remap via identity crosswalk (`users.id` → `employees.id` → new `Employee`) |
| `shift_id` | `shift_master_id` | remap via shift crosswalk (to the deduped master row) |
| `date` | `work_date` | — |
| — | `placement_id` | **derive**: the agent's active placement on `work_date` (E3); if none/ambiguous → review queue |
| — | `status` | default `Scheduled` |

## 4. Gaps & decisions

| # | Gap | Handling |
|---|-----|----------|
| G-1 | **Per-company shift dedupe** | Collapse identical shifts (same title + start/end/break) across companies into one master row; remap schedules to the surviving id. |
| G-3 | **Schedule → placement link missing** | Derive `placement_id` from the active placement on each `work_date`; rows with no active placement (or overlapping placements) go to a review queue, not silently dropped. |
| G-4 | **Identity remap** | `schedules.user_id` → Employee via the users↔employees crosswalk (E2 G-5); orphan user_ids flagged. |
| G-5 | **Historical vs future schedules** | Decide cutoff: migrate all history, or only schedules from a date forward (future + recent). Past schedules are mostly needed for attendance reconciliation (E5). Confirm window with E9. |
| G-6 | **Cross-midnight rows** | Set `spans_midnight` on the master; schedule attribution to start date (per FEATURE §7 open item). |

## 5. Migration rules

1. Runs after E2 (employees) and E3 (placements) so identity + placement crosswalks exist.
2. Shift master loads first (dedup), then schedules (remap user→employee, shift→master, derive placement).
3. Idempotent via `legacy_shift_id` / `legacy_schedule_id` crosswalks.
4. Orchestration in E9; this doc owns the E4 field-level mapping.
