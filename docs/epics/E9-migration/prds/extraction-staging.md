# PRD · F9.1 — Extraction & Staging

> **Epic:** E9 Data Migration · **Feature:** F9.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Before any transform, we need a **consistent, decrypted snapshot** of SWP prod (`lumen_swp`) in a staging area we can iterate on safely — without loading or risking the live legacy system. Several legacy fields (comp/payroll) are encrypted via the app's `DBEncryption` cast and must be decrypted with the legacy key.

## 2. Goals & non-goals

**Goals**
- Take a consistent snapshot (dump or read replica) of `lumen_swp`.
- Land it in staging; decrypt the `DBEncryption` fields with the legacy key.
- Capture a `MigrationRun` with source snapshot reference + stats.

**Non-goals**
- Transform/mapping (F9.2). Touching the live legacy DB beyond read.

## 3. Actors

Migration engineer/ops, System (snapshot, decrypt). 

## 4. Platform / clients

Migration tooling (CLI/job) reading a dump/replica; staging database. No mobile/web user surface (reports surface in F9.3).

## 5. Business rules

| Ref | Rule |
|-----|------|
| EX-1 | Extraction reads from a **dump or read replica** of `lumen_swp` — never writes to or locks the live prod DB. |
| EX-2 | The snapshot is **point-in-time consistent** (single dump / consistent replica position) and its reference is recorded on the `MigrationRun`. |
| EX-3 | Encrypted fields (`DBEncryption`: gaji_pokok, bpjs_*, pph21, payslip/salary values) are **decrypted with the legacy key** during staging. |
| EX-4 | A **decrypt failure** produces a `REVIEW_ITEM` (`decrypt_fail`) — never a null/blank value. |
| EX-5 | Staging is **re-creatable** from a fresh snapshot (idempotent extraction). |
| EX-6 | No PII/comp leaves the secured environment; staging is access-controlled + audited. |

## 6. Data model

Staging tables mirroring source; `MigrationRun` (id, started_at, status, source_snapshot, stats).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Extraction & staging

  Scenario: Snapshot and stage the legacy DB
    Given a read replica / dump of lumen_swp
    When I run extraction
    Then a consistent snapshot is staged
    And a MigrationRun records the snapshot reference and row counts

  Scenario: Decrypt encrypted comp fields
    Given the legacy encryption key is configured
    When staging processes employee_contracts and payslips
    Then comp/payroll fields are decrypted into staging

  Scenario: Decrypt failure is queued, not nulled
    Given a record fails to decrypt
    Then a REVIEW_ITEM of type decrypt_fail is raised
    And the field is not silently nulled

  Scenario: Re-run extraction
    When I re-run extraction from a fresh snapshot
    Then staging is rebuilt cleanly (idempotent)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Replica lag / inconsistent dump | Use a consistent snapshot position; abort if inconsistent. |
| C-2 | Schema drift since mapping was written | Detect unexpected columns/tables; flag for mapping review. |
| C-3 | Very large tables (attendance) | Stream/batch the extract. |
| C-4 | Key rotated/incorrect | Fail fast with a clear error before mass decrypt. |

## 9. Dependencies

Legacy DB access + encryption key, secured environment, F9.2 (consumes staging).

## 10. Decisions & open questions

- ✅ Dump/replica extraction; decrypt with available key; review-item on failure.
- **Open:** dump vs replica specifics + how the consistent point-in-time is taken.
