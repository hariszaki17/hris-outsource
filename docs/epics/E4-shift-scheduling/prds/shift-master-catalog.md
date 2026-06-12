# PRD · F4.1 — Work-Shift Master Catalog

> **Epic:** E4 Shift Configuration & Scheduling · **Feature:** F4.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Scheduling needs a controlled set of **shift templates** — named working-hour windows with break times — that leaders pick from instead of typing hours each time. SWP runs 24/7 sites, so the catalog must handle **cross-midnight** shifts. It's a single global catalog used across all companies.

## 2. Goals & non-goals

**Goals**
- Maintain global shift templates (title, start/end, break window).
- Correctly represent cross-midnight shifts.
- Safe lifecycle (deactivate, not delete) since schedules reference templates.

**Non-goals**
- Assigning shifts to agents (F4.2). Attendance rules per shift (E5). Rotations/coverage (not in scope).

## 3. Actors

Super Admin / HR Admin (author), System (validate, audit). Read consumers: F4.2/F4.3, E5.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | Super Admin / HR | CRUD shift templates. |
| **Mobile app** | Agent / Shift Leader | Read-only — a shift's title/times appear on schedules. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| SM-1 | A template requires: `title`, `start_at`, `end_at`. Break (`start_break`, `end_break`) is optional but, if set, must fall **within** the working window. |
| SM-2 | If `end_at <= start_at`, the shift **spans midnight**; `spans_midnight` is set automatically and duration computed across the day boundary. |
| SM-4 | `title` is unique within the catalog. |
| SM-5 | Templates are **deactivated, not hard-deleted**, when referenced by any schedule. |
| SM-6 | All actions audited (E1). |

## 6. Data model

`ShiftMaster`: `id, title (unique), start_at (time), end_at (time), start_break (time, null), end_break (time, null), spans_midnight (bool), status, created_by`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Work-shift master catalog

  Scenario: Create a day shift with a break
    Given I am a super admin
    When I create "Morning" 07:00–15:00 with break 12:00–13:00
    Then the template is saved as active

  Scenario: Create a cross-midnight night shift
    When I create "Night" 23:00–07:00
    Then spans_midnight is set true
    And the duration is computed as 8 hours across the day boundary

  Scenario: Reject a break outside the shift window
    When I create "Morning" 07:00–15:00 with break 16:00–17:00
    Then it is blocked because the break is outside the working window

  Scenario: Cannot delete a referenced template
    Given schedules reference "Morning"
    When I try to delete it
    Then deletion is blocked and I may only deactivate it

  Scenario: Unique title
    Given "Morning" exists
    When I create another "Morning"
    Then it is blocked with a uniqueness error
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Shift exactly 24h (start == end) | Treated as full-day; confirm allowed or block. |
| C-2 | Multiple breaks | Not supported in v1 (single break window); flag if needed. |
| C-3 | Deactivate a template used by future schedules | New selection disabled; existing future schedules keep it (or are flagged). |
| C-4 | Migration dedupe | Identical shifts across companies collapse into one (DATA-MAPPING G-1). |

## 9. Dependencies

E1 (RBAC/audit), F4.2/F4.3 (consumers), E5 (attendance policy per shift), E9 (migration).

## 10. Decisions & open questions

- ✅ Global catalog (independent of service line), cross-midnight supported. *(Service-line tag dropped 2026-06-12 — service line removed project-wide.)*
- **Open:** single break window enough, or multiple breaks needed for long shifts?
- **Open:** allow a 24h shift (start == end)?
