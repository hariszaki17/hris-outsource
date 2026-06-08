# PRD · F2.3 — Client Company Directory

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Placements (E3) point at a client company's **site** — the malls, office towers, and properties SWP services. This directory owns the **company** record: its statutory/billing info (name, registered address, NPWP, PIC) and `leader_scope`. The **physical locations + attendance geofence** live on its **Sites** ([F2.6](client-sites-geofence.md)) — a company has one or more. In legacy these were `companies` rows with `role=2`, mixed into a self-referential tree alongside SWP's own internal units.

## 2. Goals & non-goals

**Goals**
- A managed directory of client companies (name, registered address, NPWP, PIC, contact, `leader_scope`).
- Active/inactive status; safe deactivation when referenced by placements.
- On create, **auto-provision a primary "Main Site"** (F2.6) so the company is immediately placeable.

**Non-goals**
- Placement itself (E3). Internal SWP org (out of scope — flat).
- Sites + geofence config → **[F2.6](client-sites-geofence.md)** (per-location, was previously folded in here).

## 3. Actors

HR/Placement Admin & Super Admin (author), System (validate, audit). Read consumers: E3, E5.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR/Super Admin | Full CRUD of client companies. **List** shows the directory; its only per-row action is **Aktifkan/Nonaktifkan** (no row kebab) — create and edit live elsewhere. **Edit** is a dedicated full-page screen reached from the **detail** page (route `/client-companies/$id/edit`), not a drawer. The detail **"Profil" tab** shows statutory/billing fields + `leader_scope` only; **Sites & geofence are on the "Lokasi & Site" tab** (F2.6), never duplicated in Profil. |
| **Mobile app** | Agent / Shift Leader | Read-only: see the client they're placed at (name, address, geo) — surfaced via placement/attendance, not as a directory. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| CC-1 | A client company requires: name, registered address. NPWP, PIC, phone are optional but recommended. **Geofence/geo lives on Sites** (F2.6), not here. |
| CC-1b | `leader_scope ∈ {company, site}`, default `company` — drives per-company vs per-site shift leadership (E3 F3.4). |
| CC-1c | Creating a company **auto-creates one primary "Main Site"** (F2.6 ST-3); the company is then a valid placement target via that site. |
| CC-2 | `name` and `NPWP` (when provided) are **unique**. |
| CC-3 | Status ∈ {Active, Inactive}. Only **Active** companies can receive new placements (E3 BR-3). |
| CC-4 | A company referenced by any placement **cannot be hard-deleted** — only deactivated. |
| CC-5 | Deactivating a company with **active placements** warns and requires those placements to be ended/transferred first (or blocks). |
| CC-6 | All actions audited (E1). |

## 6. Data model

`ClientCompany`: `id, name (unique), address (registered/billing), leader_scope (company|site, default company), npwp (unique nullable), pic_name, phone, email, status, created_by`. (`legacy_company_id` crosswalk for migration.)

> **Geofence relocated (2026-06-03):** `lat`, `lng`, `geofence_radius_m` moved **off ClientCompany onto `Site`** (F2.6, EPICS §8). The company keeps only its registered address. Each company has ≥1 Site, and E5 resolves the geofence from the agent's placement **site**.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Client company directory

  Scenario: Create a client company
    Given I am an HR admin
    When I create "Plaza Group" with a registered address
    Then it is saved as Active
    And a primary "Main Site" is auto-created (F2.6)
    And it becomes selectable as a placement target via that site

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

  Scenario: Geofence lives on the site, not the company
    Given the company "Plaza Group" has a site "Plaza Senayan" with lat/lng set (F2.6)
    Then E5 attendance uses the site's coordinates as the geofence center
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Company/site without geo | Allowed; E5 geofencing for that **site** is disabled/flagged until geo added (F2.6 ST-8). |
| C-2 | Reactivate an inactive company | Allowed; becomes a valid placement target again. |
| C-3 | Migration: legacy role=3/4 rows | Not imported as ClientCompany (DATA-MAPPING G-6). |
| C-4 | Duplicate by alias/typo across legacy data | Reconciled during migration (E3 G-2 / review queue). |

## 9. Dependencies

E1 (RBAC/audit), E3 (placement target), E5 (geofence), E9 (migration), E10 (reporting).

## 10. Decisions & open questions

- ✅ ClientCompany = `companies.role=2`. ~~flat (no sub-sites)~~ **superseded 2026-06-03**: companies now have one or more **Sites** (F2.6); geofence moved to Site. (EPICS §8.)
- ✅ `leader_scope` (company | site) added to support per-site shift leaders (E3 F3.4).
- ✅ **UI/flow** *(resolved 2026-06-07, EPICS §8)* — edit is a **full-page screen from the detail page** (`/client-companies/$id/edit`), not a drawer (the `EditClientCompanyDrawer` is removed); the **list's only row action is Aktifkan/Nonaktifkan** (no row kebab, still guarded by CC-5); the detail **"Profil" tab** does **not** duplicate Sites/geofence (those live only in the "Lokasi & Site" tab, F2.6).
- **Open:** should deactivation with active placements **hard-block** or **warn-and-guide**? (currently warn-and-guide / block-until-resolved.)
