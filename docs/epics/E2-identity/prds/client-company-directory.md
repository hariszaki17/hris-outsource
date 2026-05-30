# PRD · F2.3 — Client Company Directory

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Placements (E3) point at **client companies** — the malls, office towers, and properties SWP services. The system needs a clean directory of these companies with the geo and statutory info that downstream features need (attendance geofencing in E5, reporting in E10). In legacy these were `companies` rows with `role=2`, mixed into a self-referential tree alongside SWP's own internal units.

## 2. Goals & non-goals

**Goals**
- A managed directory of client companies (name, address, geo-coordinates, NPWP, PIC, contact).
- Active/inactive status; safe deactivation when referenced by placements.

**Non-goals**
- Placement itself (E3). Internal SWP org (out of scope — flat). Sub-company/site hierarchy (not used by SWP — DATA-MAPPING G-6).

## 3. Actors

HR/Placement Admin & Super Admin (author), System (validate, audit). Read consumers: E3, E5.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR/Super Admin | Full CRUD of client companies. |
| **Mobile app** | Agent / Shift Leader | Read-only: see the client they're placed at (name, address, geo) — surfaced via placement/attendance, not as a directory. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| CC-1 | A client company requires: name, address. Geo (lat/lng), NPWP, PIC, phone are optional but recommended (geo needed for E5 geofencing). |
| CC-2 | `name` and `NPWP` (when provided) are **unique**. |
| CC-3 | Status ∈ {Active, Inactive}. Only **Active** companies can receive new placements (E3 BR-3). |
| CC-4 | A company referenced by any placement **cannot be hard-deleted** — only deactivated. |
| CC-5 | Deactivating a company with **active placements** warns and requires those placements to be ended/transferred first (or blocks). |
| CC-6 | All actions audited (E1). |

## 6. Data model

`ClientCompany`: `id, name (unique), address, lat, lng, geofence_radius_m (default 100), npwp (unique nullable), pic_name, phone, email, status, created_by`. (`legacy_company_id` crosswalk for migration.)

> `geofence_radius_m` (per-site, default 100m) drives E5 attendance geofencing — resolved in the 2026-05-29 open-items review (EPICS.md §8).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Client company directory

  Scenario: Create a client company
    Given I am an HR admin
    When I create "Plaza Senayan" with address and geo-coordinates
    Then it is saved as Active
    And it becomes selectable as a placement target

  Scenario: Reject a duplicate company name
    Given "Plaza Senayan" exists
    When I create another company named "Plaza Senayan"
    Then creation is blocked with a uniqueness error

  Scenario: Block placement into an inactive company
    Given "Old Tower" is Inactive
    When an HR admin tries to place an agent there
    Then it is blocked (E3 BR-3)

  Scenario: Prevent deactivating a company with active placements
    Given "Plaza Senayan" has active placements
    When I try to deactivate it
    Then I am warned to end/transfer those placements first

  Scenario: Geo is available for attendance geofencing
    Given "Plaza Senayan" has lat/lng set
    Then E5 attendance can use it as the geofence center
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Company without geo | Allowed; E5 geofencing for that site is disabled/flagged until geo added. |
| C-2 | Reactivate an inactive company | Allowed; becomes a valid placement target again. |
| C-3 | Migration: legacy role=3/4 rows | Not imported as ClientCompany (DATA-MAPPING G-6). |
| C-4 | Duplicate by alias/typo across legacy data | Reconciled during migration (E3 G-2 / review queue). |

## 9. Dependencies

E1 (RBAC/audit), E3 (placement target), E5 (geofence), E9 (migration), E10 (reporting).

## 10. Decisions & open questions

- ✅ ClientCompany = `companies.role=2`; flat (no sub-sites) for now.
- **Open:** should deactivation with active placements **hard-block** or **warn-and-guide**? (currently warn-and-guide / block-until-resolved.)
