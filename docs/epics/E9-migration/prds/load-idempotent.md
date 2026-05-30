# PRD · F9.4 — Load & Idempotent Re-runs

> **Epic:** E9 Data Migration · **Feature:** F9.4 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Transformed + reconciled data must land in hris-outsource (Postgres) in the **right dependency order** and in a way that can be **re-run safely** — repeated dry-runs against staging, then the final run at cutover — without creating duplicates. The crosswalk makes loads upserts.

## 2. Goals & non-goals

**Goals**
- Load entities in dependency order.
- Upsert by crosswalk (idempotent re-runs / dry-runs).
- Emit per-entity load reconciliation (source vs loaded).

**Non-goals**
- Transform (F9.2). Cutover decision (F9.5).

## 3. Actors

Migration engineer, System (load, upsert, report).

## 4. Platform / clients

Migration tooling (CLI/job) → Postgres. Reports surface in F9.3.

## 5. Business rules

| Ref | Rule |
|-----|------|
| LD-1 | Load order: **identity/master (E2) → placement (E3) → schedule (E4) → attendance (E5) → leave (E6) → overtime (E7) → payroll (E8)**. |
| LD-2 | Each row is **upserted by crosswalk** (`legacy_id → new_id`): existing → update, new → insert + write crosswalk (INV-1). |
| LD-3 | A row whose dependencies aren't loaded yet is **deferred/queued**, not force-inserted with dangling refs. |
| LD-4 | Loads are **transactional per batch**; a failed batch rolls back and is reported (no partial corruption). |
| LD-5 | **Dry-run mode** loads into a scratch/clone for validation without affecting the target. |
| LD-6 | After each entity, emit `source_count` vs `loaded_count` (+ deferred/queued) for F9.3. |
| LD-7 | Historical-state rules are honored (e.g., attendance imported `Verified`, leaves final status) per per-epic mappings. |

## 6. Data model

Writes target tables (E2–E8) + `Crosswalk`; load stats on `MigrationRun`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Load & idempotent re-runs

  Scenario: Load in dependency order
    When the load runs
    Then identity/master load before placements, which load before schedules, etc.

  Scenario: Idempotent upsert
    Given a legacy employee already loaded (crosswalk exists)
    When I re-run the load
    Then the existing record is updated, not duplicated

  Scenario: Defer when a dependency is missing
    Given a schedule whose placement isn't loaded yet
    Then the schedule load is deferred until the placement exists

  Scenario: Batch failure rolls back
    Given a batch hits a constraint error
    Then that batch rolls back and is reported
    And other batches are unaffected

  Scenario: Dry-run does not touch production target
    When I run in dry-run mode
    Then data loads into a scratch clone and the live target is untouched
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Re-run after resolving review items | Resolved rows now load; previously-loaded rows upsert unchanged. |
| C-2 | Partial prior run | Crosswalk lets the re-run continue without duplicates. |
| C-3 | High-volume attendance load | Batched/streamed; progress reported. |
| C-4 | Constraint mismatch (new schema stricter than legacy) | Row → review item; not force-loaded. |

## 9. Dependencies

F9.2 (transformed data + crosswalks), F9.3 (resolutions), target schemas (E2–E8), F9.5 (final run).

## 10. Decisions & open questions

- ✅ Dependency-ordered, crosswalk-upsert, transactional batches, dry-run support.
- **Open:** batch sizes / parallelism for the high-volume tables to fit the maintenance window.
