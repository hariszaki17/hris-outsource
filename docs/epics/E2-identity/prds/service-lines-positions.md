# PRD · F2.4 — Service Lines & Position Master

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.4 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

SWP operates across three **service lines** — Facility Services, Building Management, Parking — and each has its own **positions** (Parking → Attendant/Supervisor; Building → Technician/Engineer; Facility → Cleaning Crew/Supervisor). Service line drives shift & attendance rules downstream (E4/E5), and positions are chosen per placement (E3 BR-9). Legacy had job roles in `recruitment_roles` but **no service-line concept** — this feature introduces it and scopes positions beneath it.

## 2. Goals & non-goals

**Goals**
- Maintain the service-line list (seeded with the three) and positions scoped under each.
- Provide the controlled position list E3 selects from per placement.

**Non-goals**
- Shift/attendance rules themselves (E4/E5 — they *reference* service line). Placement (E3).

## 3. Actors

Super Admin (primary), HR Admin (positions), System (validate, audit). Read consumers: E3, E4, E5.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | Super Admin / HR | CRUD service lines & positions. |
| **Mobile app** | Agent / Shift Leader | Read-only (a position label shown on their placement/schedule). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| SP-1 | ServiceLine is seeded with **Facility Services, Building Management, Parking**; admins may add more, but lines **cannot be deleted while referenced** (deactivate instead). |
| SP-2 | Every Position belongs to **exactly one** service line. |
| SP-3 | Position `name` is **unique within its service line** (the same label may exist under different lines). |
| SP-4 | Positions are deactivated, not hard-deleted, when referenced by placements/history. |
| SP-5 | The active position list, filtered by a placement's service line, is what E3 offers at placement time. |
| SP-6 | All actions audited (E1). |

## 6. Data model

`ServiceLine`: `id, name (unique), status`.
`Position`: `id, service_line_id (FK), name, alias, status` — unique `(service_line_id, name)`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Service lines & positions

  Scenario: Seeded service lines exist
    Then the service lines "Facility Services", "Building Management", and "Parking" exist by default

  Scenario: Define a position under a service line
    Given I am a super admin
    When I add the position "Parking Attendant" under "Parking"
    Then it is available to select for placements in the "Parking" line

  Scenario: Same position name allowed under different lines
    Given "Supervisor" exists under "Parking"
    When I add "Supervisor" under "Building Management"
    Then both are accepted as distinct positions

  Scenario: Reject duplicate position within a line
    Given "Parking Attendant" exists under "Parking"
    When I add "Parking Attendant" under "Parking" again
    Then it is blocked with a uniqueness error

  Scenario: Cannot delete a referenced service line
    Given placements reference the "Parking" line
    When I try to delete "Parking"
    Then deletion is blocked and I may only deactivate it

  Scenario: Placement position list is line-scoped
    Given a placement in service line "Building Management"
    Then only "Building Management" positions are selectable (E3)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Add a 4th service line later | Allowed (SP-1); flows downstream to shift/attendance rules. |
| C-2 | Deactivate a position still used by active placements | Allowed for *new* selection only; existing placements keep it. |
| C-3 | Migration: `recruitment_roles` → positions with no service line | Imported with `service_line_id` pending; classified manually (DATA-MAPPING G-3). |
| C-4 | Reassign a position to a different service line | Discouraged/blocked if referenced; create a new position instead. |

## 9. Dependencies

E1 (RBAC/audit), E3 (placement position selection), E4/E5 (service-line-driven rules), E9 (migration).

## 10. Decisions & open questions

- ✅ Position scoped by service line; service lines seeded (3), admin-extendable.
- **Open:** confirm the initial position catalog per line with SWP (drives the manual classification of migrated `recruitment_roles`).
