# PRD · F11.1 — Approval Template Management

> **Epic:** E11 Approvals · **Feature:** F11.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Different client companies want different approval chains. SWP needs HR to configure, **per company**, who signs off on that company's agents' requests (leave, overtime, …) and in what order — without code changes. A template is an **ordered chain of 2–3 lines**; each line is a set of users where **any one** approval clears it (OR); lines clear **in sequence**. The 3rd line is optional (typically a super-admin sign-off). Because templates are **live** (no per-instance snapshot), saving an edit must safely re-base any in-flight requests for that company.

## 2. Goals & non-goals

**Goals**
- Per-company CRUD of an approval template: 2–3 ordered lines, each a multi-user OR-set.
- Restrict authoring to HR admin + super admin (`approvals.template.manage`).
- Validate on save (≥2 lines; each line ≥1 **active** member) and re-base pending instances (INV-6).

**Non-goals**
- Executing the chain (F11.2). The inbox (F11.3). Role-ref membership (deferred — explicit users only).

## 3. Actors

HR/Placement Admin & Super Admin (author), System (validate, version, reset pending instances, audit), line members (notified on reset).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | Open a company's approval template; add/reorder lines (2–3); assign 1..* members per line; save. |
| **Web console** | Super Admin | Same; super admin is also the implicit fallback approver when **no** template exists (F11.2, INV-7). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| TM-1 | A client company has **0 or 1** approval template (INV-1). Create or edit is per company. |
| TM-2 | A template has **2 or 3 ordered lines** (`line_no` 1..3). **Line 3 is optional**; the minimum is **2**. Fewer than 2 → `INVALID_REQUEST`. |
| TM-3 | Each line has **≥1 member**; members are **active SWP staff users** (employment not ended, INV-J). An empty line or an inactive/offboarded member → `422 APPROVAL_LINE_INVALID` (field-level). |
| TM-4 | A line is an **OR-set**: at execution, any one member clears it (F11.2). The same user may appear on multiple lines. |
| TM-5 | Only `approvals.template.manage` holders (HR admin, super admin) may create/edit/delete a template. |
| TM-6 | Saving an edit **bumps `version`** and **resets all non-terminal instances** for that company to `current_line = 1` on the new version; prior actions are kept (audit) but no longer count; new line-1 members are notified (INV-6). |
| TM-7 | Deleting a template reverts the company to the **super-admin fallback** (INV-7); pending instances reset accordingly (TM-6). |
| TM-8 | All create/edit/delete actions are audited (E1); the response reflects the new `version`. |

## 6. Data model

`ApprovalTemplate`: `id (SWP-APT-*), company_id (unique FK), version, created_by, created_at, updated_at`.
`ApprovalLine`: `id (SWP-APL-*), template_id (FK), line_no (1..3)`.
`ApprovalLineMember`: `line_id (FK), user_id (FK)` — OR-set per line.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Approval template management

  Background:
    Given I am an HR admin with "approvals.template.manage"
    And the client company "Plaza Senayan" has no template yet

  Scenario: Create a two-line template
    When I add line 1 with members [Rudi, Sari] and line 2 with member [Sari Hadi]
    And I save
    Then a template is created for "Plaza Senayan" at version 1
    And line 1 is satisfied by Rudi OR Sari (OR-set)

  Scenario: Optional third line
    When I add a third line with member [the super admin]
    And I save
    Then the template has 3 ordered lines

  Scenario: Reject fewer than two lines
    When I save a template with only line 1
    Then it is rejected with INVALID_REQUEST (minimum two lines)

  Scenario: Reject an empty or inactive-member line
    When I save a line with no members, or a member whose employment has ended
    Then it is rejected with APPROVAL_LINE_INVALID on that line

  Scenario: Editing re-bases pending requests
    Given "Plaza Senayan" has pending leave/OT instances mid-chain
    When I edit the template and save
    Then its version is bumped
    And every non-terminal instance resets to line 1 on the new chain
    And the new line-1 members are notified
    And the old decisions remain visible in the audit trail but no longer count

  Scenario: Non-authorized user cannot manage templates
    Given I am a shift leader (no approvals.template.manage)
    Then I cannot create or edit a company's approval template (403)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Requester is the only member of a line | Allowed to save; at execution that line is **self-blocked** and clears only by super-admin bypass (F11.2 INV-3/INV-5). Save **warns** but does not block. |
| C-2 | Same user on lines 1 and 2 | Allowed (TM-4); each line is satisfied independently. |
| C-3 | Member offboarded after save, with pending instances | Template now invalid on next edit (TM-3); pending instances handled by super-admin bypass + HR re-edit (INV-J). |
| C-4 | Edit during a burst of submissions | Reset (TM-6) runs in the save transaction; instances created after the save are on the new version. |
| C-5 | Delete template with pending instances | Reverts to fallback (TM-7); pending instances reset to the super-admin line. |
| C-6 | Company with `leader_scope = site` | Template is **company-level** in v1 (one per company, INV-1); site-level templates are out of scope. |

## 9. Dependencies

E2 (client companies; active-employee check), E1 (users, RBAC `approvals.template.manage`, audit), F11.2 (consumes the template; reset target), E10 (notify reset line-1 members).

## 10. Decisions & open questions

- ✅ One template per company; 2–3 ordered lines; line 3 optional; OR-set membership; live + pending reset (2026-06-14, EPICS §8 E11).
- ✅ Explicit-user membership only in v1 (no role-refs).
- **Open:** site-level templates for `leader_scope = site` companies (v1: company-level only).
- **Open:** soft warning vs hard block when a saved line is sole-member-equals-a-likely-requester (v1: warn only).
