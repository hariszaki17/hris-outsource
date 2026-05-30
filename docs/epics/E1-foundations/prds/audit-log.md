# PRD · F1.3 — Comprehensive Audit Log

> **Epic:** E1 Foundations & Platform · **Feature:** F1.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Nearly every PRD in this project says "…and the action is audited." HR/compliance for an outsourcing firm needs a defensible record of who did what — placements, approvals, comp changes, attendance verifications, migrations. This feature provides one **comprehensive, immutable audit log** the whole system writes to.

## 2. Goals & non-goals

**Goals**
- Record **every mutation** (create/update/delete) with actor, action, entity, before/after, ip, timestamp.
- Make it **immutable** and **queryable** (by entity, actor, time).

**Non-goals**
- Analytics dashboards (E10). Permissions (F1.2).

## 3. Actors

System (writes), HR/Super Admin (reads/searches), auditors.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | Search/view audit history. |
| **Go API** | — | Writes an entry on every mutation. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| AL-1 | Every create/update/delete across modules writes an `AuditLog` entry (INV-4). |
| AL-2 | An entry captures: `actor_user_id` (or `system`), `action`, `entity_type`, `entity_id`, `before`, `after`, `ip`, `created_at`. |
| AL-3 | Entries are **append-only / immutable** (no edit/delete). |
| AL-4 | Sensitive values (comp/payroll) are **masked** in `before`/`after`; the fact-of-change is logged, not the cleartext amounts. |
| AL-5 | Searchable by entity (type+id), actor, action, and time range. |
| AL-6 | System/automated actions (jobs, migration) are attributed to `system` with context. |
| AL-7 | Access to audit is **HR/Super Admin** only. |

## 6. Data model

`AuditLog` (id, actor_user_id, action, entity_type, entity_id, before json, after json, ip, created_at) — FEATURE §4.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Comprehensive audit log

  Scenario: Mutation writes an audit entry
    When an HR admin updates a placement
    Then an audit entry records actor, before, after, and time

  Scenario: Immutability
    When anyone attempts to edit or delete an audit entry
    Then it is not permitted

  Scenario: Sensitive values masked
    When a compensation field changes
    Then the audit logs that it changed without storing the cleartext amounts

  Scenario: System actions attributed
    When the auto-clock-out job closes a record
    Then the audit entry is attributed to "system"

  Scenario: Search by entity
    Given many audit entries
    When HR searches by a placement id
    Then they see that placement's change history

  Scenario: Access restricted
    When an agent tries to view the audit log
    Then access is denied
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | High write volume | Async/append-optimized write path; never blocks the user action. |
| C-2 | Bulk operations | Logged per affected entity (or summarized with detail) — confirm granularity. |
| C-3 | Migration writes | Attributed to `system`/migration run id. |
| C-4 | Retention | Long-term retention + archival (see §10). |

## 9. Dependencies

Used by all epics (E2–E10) + migration (E9); F1.1/F1.2 (actor/scope).

## 10. Decisions & open questions

- ✅ Comprehensive, immutable, masked-sensitive, restricted-access audit.
- **Open:** **retention period** + archival/storage strategy (high volume).
- **Open:** bulk-action audit granularity (per-row vs summarized).
