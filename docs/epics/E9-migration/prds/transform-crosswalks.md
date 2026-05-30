# PRD · F9.2 — Transform & Crosswalks

> **Epic:** E9 Data Migration · **Feature:** F9.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Staged legacy data must be reshaped into the new model: identity remapped, `employee_contracts` **split** into EmploymentAgreement + Placement, shifts deduped, links derived (schedule→placement, attendance→schedule), and values classified (service line, day_type). Every produced row needs a **crosswalk** (legacy_id → new_id) so the whole migration is idempotent and traceable.

## 2. Goals & non-goals

**Goals**
- Apply each epic's `DATA-MAPPING.md` rules to staged data.
- Produce transformed records + a `CROSSWALK` per legacy_id → new_id.
- Route anything ambiguous/unmappable to F9.3.

**Non-goals**
- Defining field semantics (per-epic mappings own that). Loading (F9.4).

## 3. Actors

Migration engineer, System (transform, crosswalk).

## 4. Platform / clients

Migration tooling (CLI/job) over staging → transformed dataset. No end-user surface.

## 5. Business rules

| Ref | Rule |
|-----|------|
| TR-1 | Transform applies the **authoritative per-epic mappings** (E2–E8 DATA-MAPPING.md); E9 does not redefine field semantics. |
| TR-2 | Every transformed row gets a **`CROSSWALK`** entry (`legacy_table`, `legacy_id`, `new_table`, `new_id`, `run_id`). |
| TR-3 | **Identity remap** (`users`/`employees` → User/Employee) is computed first; later transforms reference it. |
| TR-4 | `employee_contracts` is **split** into an EmploymentAgreement (terms/comp/dates) + a Placement (client/service-line/period). |
| TR-5 | **Derived links** are computed: schedule→placement (by employee+date), attendance→schedule, OT→attendance where possible. |
| TR-6 | **Classifications** (service line for placements/positions; day_type for OT) are applied where derivable; otherwise queued (F9.3). |
| TR-7 | Any row that can't be cleanly mapped → **`REVIEW_ITEM`** with its issue type; never dropped (INV-2). |
| TR-8 | Transform is **deterministic + re-runnable** against the same staging snapshot. |

## 6. Data model

`Crosswalk` (id, legacy_table, legacy_id, new_table, new_id, run_id); transformed-record staging.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Transform & crosswalks

  Scenario: Split a legacy contract into agreement + placement
    Given a staged employee_contracts row
    When transform runs
    Then one EmploymentAgreement and one Placement are produced
    And crosswalks link the legacy contract id to both new ids

  Scenario: Remap identity first
    Given staged users and employees
    When transform runs
    Then User and Employee are produced with a 1:1 link and crosswalks

  Scenario: Derive schedule placement link
    Given a staged schedule row for an employee+date
    When transform runs
    Then the schedule is linked to the placement active on that date
    And if none matches, a REVIEW_ITEM is raised

  Scenario: Idempotent re-run
    When transform runs twice on the same snapshot
    Then crosswalks are reused and no duplicates are produced

  Scenario: Unmappable free-text placement
    Given a placement string that matches no client company
    Then a REVIEW_ITEM (unmatched_placement) is raised
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Free-text `placement` fuzzy-matches multiple companies | Queue as ambiguous for HR to pick. |
| C-2 | Orphan `user_id` (no employee) or vice-versa | Queue orphan_identity. |
| C-3 | Ambiguous renewal/transfer chain | Best-effort predecessor link + flag (E3 G-7). |
| C-4 | Reference to a deactivated/missing master | Queue or map to a default + flag. |

## 9. Dependencies

F9.1 (staging), per-epic DATA-MAPPING docs, F9.3 (review), F9.4 (load).

## 10. Decisions & open questions

- ✅ Apply per-epic mappings; crosswalk everything; queue ambiguities.
- **Open:** matching strategy/threshold for free-text placement → company (exact + fuzzy + alias list).
