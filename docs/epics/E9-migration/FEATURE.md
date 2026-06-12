# E9 — Data Migration · Feature Document

> **Epic:** E9 Data Migration · **Status:** Draft v1 · **Parent:** [EPICS.md](../../EPICS.md)
> One-time, big-bang **transform-and-load** of SWP prod (`lumen_swp`, MySQL) into hris-outsource (Postgres), orchestrating the field-level mappings defined per epic.

---

## 1. Goal & outcome

Move **everything** from the legacy SWP production database into the new model, **once**, with confidence: extract a snapshot, transform under the new schema (using each epic's `DATA-MAPPING.md`), reconcile anything that can't be cleanly mapped via a **review queue resolved before go-live**, load in dependency order, validate, then **cut over** in a single switch. Re-runnable and idempotent until the final run.

> Field-level mappings live in each epic's DATA-MAPPING.md: [E2](../E2-identity/DATA-MAPPING.md) · [E3](../E3-placement/DATA-MAPPING.md) · [E4](../E4-shift-scheduling/DATA-MAPPING.md) · [E5](../E5-attendance/DATA-MAPPING.md) · [E6](../E6-leave/DATA-MAPPING.md) · [E7](../E7-overtime/DATA-MAPPING.md) · [E8](../E8-payroll/DATA-MAPPING.md). **E9 owns orchestration, not field semantics.**

## 2. Actors & roles

| Actor | Involvement |
|---|---|
| **Migration engineer / ops** | Runs extraction/transform/load; owns the runbook + rollback. |
| **HR / Super Admin** | Resolves the reconciliation review queue (unmatched placements, identity, ambiguous chains). |
| **System (migration tooling)** | Extract, decrypt, transform, crosswalk, load, validate, report. |

## 3. Scope

**In scope:** extraction & staging, transform + crosswalks, reconciliation/review queue, ordered idempotent load, cutover + validation + rollback.
**Out of scope:** the field-level mappings themselves (per-epic DATA-MAPPING docs); ongoing sync (big-bang, not parallel-run).

## 4. Migration infrastructure

```mermaid
erDiagram
    MIGRATION_RUN ||--o{ CROSSWALK : "produces"
    MIGRATION_RUN ||--o{ REVIEW_ITEM : "raises"
    MIGRATION_RUN ||--o{ RECON_REPORT : "emits"

    MIGRATION_RUN {
        bigint id PK
        datetime started_at
        datetime finished_at
        string status
        string source_snapshot
        json stats
    }
    CROSSWALK {
        bigint id PK
        string legacy_table
        bigint legacy_id
        string new_table
        bigint new_id
        bigint run_id FK
    }
    REVIEW_ITEM {
        bigint id PK
        string entity_type
        string issue_type "unmatched_placement|orphan_identity|decrypt_fail|ambiguous_chain"
        json payload
        string status "Open|Resolved"
        bigint resolved_by FK
    }
    RECON_REPORT {
        bigint id PK
        bigint run_id FK
        string entity_type
        int source_count
        int loaded_count
        int review_count
    }
```

**Invariants:**
- **INV-1:** **idempotent + re-runnable** — every loaded row is keyed by a `CROSSWALK` (legacy_id → new_id); re-runs upsert, never duplicate.
- **INV-2:** **nothing silently dropped** — any unmappable/ambiguous row becomes a `REVIEW_ITEM`.
- **INV-3:** load respects **dependency order** (identity/master → placement → schedule → attendance → leave → overtime → payroll).
- **INV-4:** **big-bang** — the source is frozen for the final run; no two-way sync.
- **INV-5:** decrypt-then-re-encrypt all legacy `DBEncryption` fields using the **available legacy key**; decrypt failures → `REVIEW_ITEM`, never null.

## 5. Features

| ID | Feature | PRD |
|----|---------|-----|
| **F9.1** | Extraction & Staging | [extraction-staging.md](prds/extraction-staging.md) |
| **F9.2** | Transform & Crosswalks | [transform-crosswalks.md](prds/transform-crosswalks.md) |
| **F9.3** | Reconciliation & Review Queue | [reconciliation-review.md](prds/reconciliation-review.md) |
| **F9.4** | Load & Idempotent Re-runs | [load-idempotent.md](prds/load-idempotent.md) |
| **F9.5** | Cutover, Validation & Rollback | [cutover-validation.md](prds/cutover-validation.md) |

## 6. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Migration tooling (CLI/job)** | Engineer / ops | Extract, transform, load, validate; runbook. |
| **Web console** | HR / Super Admin | Reconciliation review queue; validation/recon reports. |
| **Mobile** | — | Not applicable. |

---

### F9.1 — Extraction & Staging

Take a consistent snapshot of `lumen_swp` (dump or read replica), land it in a **staging area**, and **decrypt** the `DBEncryption` fields with the legacy key — without touching the live legacy system.

```mermaid
flowchart TD
    subgraph SRC[ims-system SWP prod]
        A1[(lumen_swp MySQL)]
    end
    subgraph MIG[Migration tooling]
        A1 --> B1[Snapshot: dump / read replica]
        B1 --> B2[Load into staging]
        B2 --> B3[Decrypt comp/payroll with legacy key]
        B3 --> B4{Decrypt ok?}
        B4 -- No --> B5[REVIEW_ITEM decrypt_fail]
        B4 -- Yes --> B6[Staged + ready to transform]
    end
```

**Entities:** staging tables, `MigrationRun`. **Depends on:** legacy key (E9 INV-5), DB access.

---

### F9.2 — Transform & Crosswalks

Apply each epic's mapping to staged data: remap identity, split `employee_contracts` into EmploymentAgreement + Placement, dedupe shifts, derive links (schedule→placement, attendance→schedule), classify (day_type) — writing a **CROSSWALK** for every legacy_id → new_id. Position copies straight across as free-text (no classification step).

```mermaid
flowchart LR
    subgraph MIG[Transform]
        T1[Staged data] --> T2[Apply per-epic DATA-MAPPING rules]
        T2 --> T3[Build CROSSWALK legacy_id -> new_id]
        T2 --> T4{Clean map?}
        T4 -- No --> T5[REVIEW_ITEM]
        T4 -- Yes --> T6[Transformed records ready to load]
    end
```

**Entities:** `Crosswalk`, transformed records. **Depends on:** F9.1, per-epic mappings.

---

### F9.3 — Reconciliation & Review Queue

Anything ambiguous — free-text `placement` → ClientCompany, orphan identities, ambiguous renewal chains, decrypt failures — becomes a **review item** that HR resolves **before go-live**. Each run emits a reconciliation report (counts in/out/queued).

```mermaid
flowchart TD
    subgraph SYS[System]
        R1[REVIEW_ITEMs from transform] --> R2[Reconciliation report per entity]
    end
    subgraph HR[HR / Super Admin - web]
        R2 --> R3[Work the review queue]
        R3 --> R4[Resolve: map / classify / correct]
        R4 --> R5[Mark Resolved]
    end
    subgraph GATE[Go-live gate]
        R5 --> G1{All blocking items resolved?}
        G1 -- No --> R3
        G1 -- Yes --> G2[Eligible for cutover]
    end
```

**Entities:** `ReviewItem`, `ReconReport`. **Depends on:** F9.2.

---

### F9.4 — Load & Idempotent Re-runs

Load transformed records into Postgres in **dependency order**, keyed by crosswalk so re-runs **upsert** (no duplicates). Supports repeated dry-runs against staging before the final run.

```mermaid
flowchart TD
    subgraph MIG[Load]
        L1[Transformed + resolved data] --> L2[Order: identity/master -> placement -> schedule -> attendance -> leave -> overtime -> payroll]
        L2 --> L3{Crosswalk exists?}
        L3 -- Yes --> L4[Update existing]
        L3 -- No --> L5[Insert + write crosswalk]
        L4 --> L6[(Postgres)]
        L5 --> L6
        L6 --> L7[Recon report: source vs loaded]
    end
```

**Entities:** target tables (E2–E8), `Crosswalk`. **Depends on:** F9.2, F9.3.

---

### F9.5 — Cutover, Validation & Rollback

The big-bang switch: freeze legacy, run the final migration, run **validation gates** (counts, spot-checks, balances), get **go/no-go**, switch traffic to hris-outsource — with a documented **rollback** if validation fails.

```mermaid
flowchart TD
    subgraph CUT[Cutover runbook]
        C1([Freeze ims-system]) --> C2[Final migration run]
        C2 --> C3[Validation gates: counts, balances, spot-checks]
        C3 --> C4{Pass?}
        C4 -- No --> C5[Rollback: stay on ims-system, fix, retry]
        C4 -- Yes --> C6[Switch users to hris-outsource]
        C6 --> C7[Post-cutover monitoring]
    end
```

**Entities:** `MigrationRun` (final), validation results. **Depends on:** F9.1–F9.4.

---

## 7. Decisions & open questions

**Resolved (2026-05-29):**
- ✅ **Big-bang** one-time cutover (freeze → migrate → validate → switch); no parallel-run.
- ✅ **Legacy encryption key available** — decrypt comp/payroll, re-encrypt in Postgres.
- ✅ **Direct DB dump / read replica** of `lumen_swp` for extraction.
- ✅ **Review queue resolved before go-live** for unmatched/ambiguous records.

**Resolved — open-items review (2026-05-29), see [EPICS.md §8](../../EPICS.md):**
- ✅ **History window** = migrate **everything** incl. full attendance (plan a larger migration + validation window).
- ✅ **Blocking review items** = `decrypt_fail`, `orphan_identity`, `unmatched_placement`; non-blocking = `ambiguous_chain`. *(`unclassified_service_line` removed 2026-06-12 — service line dropped project-wide; position is free-text, copied verbatim, never queued.)*
- ✅ **Placement-string matching** = exact + alias list + fuzzy-with-manual-confirm.
- ✅ **Post-cutover** = keep `lumen_swp` read-only ~6–12 months.

**Still open (sized during dry-runs / ops):**
1. Maintenance-window length + rehearsal schedule.
2. Exact validation-gate thresholds + required sign-offs.
3. Timing of HR's manual role-enum classification relative to cutover.
