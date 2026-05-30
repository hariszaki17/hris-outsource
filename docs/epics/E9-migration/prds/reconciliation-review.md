# PRD · F9.3 — Reconciliation & Review Queue

> **Epic:** E9 Data Migration · **Feature:** F9.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Legacy data is messy — free-text placements, orphan identities, missing service-line classification, decrypt failures. Per decision, these are **resolved before go-live** via a **review queue** HR works, backed by per-run **reconciliation reports** (source vs loaded vs queued). This is the gate that guarantees clean data at launch.

## 2. Goals & non-goals

**Goals**
- Surface every `REVIEW_ITEM` from extraction/transform in an HR-workable queue.
- Provide reconciliation reports (counts per entity: source / loaded / in-review).
- Enforce a **go-live gate**: blocking items must be resolved before cutover.

**Non-goals**
- Producing the items (F9.1/F9.2). The final cutover decision (F9.5 uses this).

## 3. Actors

HR / Super Admin (resolve), Migration engineer (monitor), System (queue, report, gate).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | Work the review queue; resolve mappings/classifications/corrections. |
| **Web console** | Engineer | View reconciliation reports per run. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| RC-1 | Review items carry an `issue_type` (unmatched_placement, orphan_identity, decrypt_fail, unclassified_service_line, ambiguous_chain, …), `entity_type`, and a `payload` with context. |
| RC-2 | Resolving an item records the resolution (chosen mapping/classification/correction), `resolved_by`, and timestamp; the resolution **feeds the next transform/load run**. |
| RC-3 | Items are classified **blocking** vs **non-blocking**; **blocking items must be Resolved before cutover** (go-live gate). |
| RC-4 | Each run emits a **reconciliation report** per entity: `source_count`, `loaded_count`, `review_count` (must balance). |
| RC-5 | The queue supports **bulk** resolution (e.g., map many identical placement strings to one company). |
| RC-6 | All resolutions are audited. |

## 6. Data model

`ReviewItem` (id, entity_type, issue_type, payload, status, resolved_by, resolution); `ReconReport` (id, run_id, entity_type, source_count, loaded_count, review_count).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Reconciliation & review queue

  Scenario: Resolve an unmatched placement
    Given a REVIEW_ITEM unmatched_placement for "PLZ SNYN"
    When HR maps it to client company "Plaza Senayan"
    Then the resolution is recorded and applied on the next run

  Scenario: Bulk-map identical strings
    Given 40 records with placement "PLZ SNYN"
    When HR bulk-maps them to "Plaza Senayan"
    Then all 40 resolve at once

  Scenario: Reconciliation report balances
    When a load run completes
    Then each entity report shows source = loaded + in-review

  Scenario: Go-live gate blocks on unresolved blocking items
    Given open blocking review items exist
    When cutover readiness is checked
    Then it reports NOT READY until they are resolved

  Scenario: Non-blocking items don't block cutover
    Given only non-blocking items remain
    Then cutover may proceed (items tracked for post-launch)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Decrypt-fail items | Blocking for comp/payroll integrity; must resolve (re-key/re-source). |
| C-2 | Unclassified service line | Non-blocking if placements can launch with pending classification (per E3 decision) — confirm. |
| C-3 | Conflicting resolutions across runs | Latest resolution wins; audited. |
| C-4 | New items appear on re-run | Surfaced; counts updated. |

## 9. Dependencies

F9.1/F9.2 (item sources), F9.4 (applies resolutions), F9.5 (consumes the gate), E1 (audit).

## 10. Decisions & open questions

- ✅ Review queue resolved before go-live; reports balance; blocking gate.
- **Open:** which issue types are **blocking** vs non-blocking (e.g., is unclassified service line blocking?).
