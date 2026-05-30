# PRD · F2.5 — Operational Master Data (leave / attendance / overtime)

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.5 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

The time-tracking epics (E5 Attendance, E6 Leave, E7 Overtime) all depend on admin-defined reference data: **leave types**, **attendance codes**, and **overtime rules**. E2 owns these definitions (CRUD + lifecycle); the *behavior* that consumes them lives in the respective epics. Legacy had `leave_types` and `attendance_codes` (per-company), but **no overtime-rules table** — so OvertimeRule is net-new.

## 2. Goals & non-goals

**Goals**
- Manage the three master lists with the flags each downstream epic needs.
- Safe lifecycle (deactivate, never hard-delete) since records reference them.

**Non-goals**
- Leave balances/requests (E6), attendance capture (E5), OT requests/calc execution (E7). Those consume these definitions.

## 3. Actors

Super Admin (primary), HR Admin, System (validate, audit). Read consumers: E5, E6, E7.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | Super Admin / HR | CRUD leave types, attendance codes, overtime rules. |
| **Mobile app** | Agent / Shift Leader | Read-only — these appear as selectable options/labels when requesting leave/OT or viewing attendance status. |

## 5. Master definitions & business rules

### Leave types
| Ref | Rule |
|-----|------|
| LT-1 | Fields: `name`, `description`, `is_annual` (annual/tahunan), `is_document_required`. |
| LT-2 | `is_annual` types are what leave quotas (E6) accrue against. |
| LT-3 | `is_document_required` types force a document upload on request (E6). |

### Attendance codes
| Ref | Rule |
|-----|------|
| AC-1 | Fields: `name`, `description`, `is_workday`, `is_payable`, `is_billable`, `needs_verification`, `color`. |
| AC-2 | `is_billable` marks codes chargeable to the client (relevant to outsource billing/reporting, E10). |
| AC-3 | `needs_verification` codes require shift-leader verification in attendance (E5). |

### Overtime rules (net-new)
| Ref | Rule |
|-----|------|
| OR-1 | Fields: `name`, `service_line_id` (nullable = global), `multiplier`, `min_minutes`, `requires_preapproval`. |
| OR-2 | A rule may be **scoped to a service line** (e.g., Parking 24/7 vs office hours) or global. |
| OR-3 | Field set is **provisional** — to be confirmed against Indonesian OT regulation and SWP practice in E7. |

### Common
| Ref | Rule |
|-----|------|
| MD-1 | All three are **deactivated, not deleted**, when referenced. |
| MD-2 | Names are unique within each master list. |
| MD-3 | All actions audited (E1). |

## 6. Data model

`LeaveType`: `id, name (unique), description, is_annual, is_document_required, status`.
`AttendanceCode`: `id, name (unique), description, is_workday, is_payable, is_billable, needs_verification, color, status`.
`OvertimeRule`: `id, name (unique), service_line_id (FK nullable), multiplier, min_minutes, requires_preapproval, status`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Operational master data

  Scenario: Create an annual leave type requiring documents
    Given I am a super admin
    When I create a leave type "Sick Leave" with is_annual=false and is_document_required=true
    Then requests of this type will require a document upload (E6)

  Scenario: Create a billable attendance code needing verification
    When I create an attendance code "Overtime Present" with is_billable=true and needs_verification=true
    Then attendance using this code is flagged billable and must be verified by a shift leader (E5)

  Scenario: Create a service-line-scoped overtime rule
    When I create an overtime rule "Parking Night OT" scoped to "Parking" with multiplier 2.0 and min_minutes 60
    Then it applies only to Parking overtime calculations (E7)

  Scenario: Cannot delete a referenced leave type
    Given leave requests reference "Annual Leave"
    When I try to delete it
    Then deletion is blocked and I may only deactivate it

  Scenario: Unique names within a list
    Given an attendance code "Present" exists
    When I create another "Present"
    Then it is blocked with a uniqueness error
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Deactivate a leave type with open requests | New requests can't use it; in-flight ones complete. |
| C-2 | Migration: per-company `attendance_codes` | Collapse to one SWP-wide set, dedupe by name (DATA-MAPPING G-6). |
| C-3 | OvertimeRule with no service line | Treated as global default. |
| C-4 | Conflicting OT rules (global + line) for the same OT | Line-scoped rule wins over global (confirm precedence in E7). |
| C-5 | Color clash between attendance codes | Allowed (cosmetic); optionally warn. |

## 9. Dependencies

E1 (RBAC/audit), E5 (attendance codes), E6 (leave types/quotas), E7 (overtime rules), E9 (migration), E10 (billable reporting).

## 10. Decisions & open questions

- ✅ E2 owns master definitions; behavior in E5/E6/E7.
- **Open (defer to E7):** confirm the OvertimeRule field set + precedence against Indonesian OT regulation (e.g., 1.5× first hour, 2× subsequent) and SWP practice.
- **Open:** are leave types / attendance codes ever genuinely per-service-line, or one SWP-wide set? (assumed: SWP-wide.)
