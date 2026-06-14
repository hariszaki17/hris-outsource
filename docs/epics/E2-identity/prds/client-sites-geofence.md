# PRD · F2.6 — Client Sites & Geofence

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.6 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

A partnering company rarely *is* a single place. A client like a property group operates several
physical locations — Plaza Senayan, Plaza Indonesia, Plaza Semanggi — and agents are placed at a
**specific** one. The system needs each of these locations as a managed **Site**, because the
**attendance geofence** (the GPS center + radius an agent must clock in within, E5) is a per-location
fact, not a per-company one.

Until 2026-06-03 the model was deliberately **flat**: geofence (`lat`, `lng`, `geofence_radius_m`)
lived directly on `ClientCompany`, with sub-sites explicitly out of scope (E2 non-goal; DATA-MAPPING
G-6 ignored legacy `role=4` sub-companies). This PRD **reverses that decision** (EPICS §8, 2026-06-03):
it introduces `Site` as a first-class child of `ClientCompany` and relocates the geofence onto it.

## 2. Goals & non-goals

**Goals**
- A managed list of **Sites under each client company** (name, physical address, optional on-site PIC).
- Per-site **geofence** config: center `lat`/`lng` + `geofence_radius_m` (single circle, default 100m).
- A guaranteed **primary "Main Site"** so single-location companies and defaults Just Work.
- Sites as the **placement target** (E3) and the **geofence source** (E5).

**Non-goals**
- Clock-in/geofence *evaluation* behavior (E5 — F2.6 only stores the geofence).
- Placement itself (E3) and shift-leader assignment mechanics (E3 F3.4 — F2.6 only owns `leader_scope` as a company attribute via F2.3).
- Multi-circle or polygon geofences (post-v1; v1 is a single circle).
- Internal SWP org / non-client locations.

## 3. Actors

HR/Placement Admin & Super Admin (author sites + geofence), System (validate, auto-provision the
primary site, audit). Read consumers: E3 (placement), E5 (geofence), E10 (per-site reporting). Agents
& shift leaders see only the site they're placed at, via placement/attendance (not as a directory).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | Full CRUD of a company's sites; set/move the primary site; set the geofence (map pin + radius). |
| **Mobile app** | Agent / Shift Leader | Read-only: the site they're placed at (name, address) — surfaced via placement/attendance, not a directory. |

## 5. User stories

- **US-1** — *As an HR admin, I want to add multiple sites to a client company, so that I can place agents at the specific location they actually work.*
- **US-2** — *As an HR admin, I want to set a geofence (center + radius) per site, so that attendance is validated against the right place.*
- **US-3** — *As an HR admin, I want a company to always have one default "Main Site", so that single-location companies need no extra setup.*
- **US-4** — *As an HR admin, I want to choose whether one shift leader covers the whole company or one per site, so that large multi-site clients can have an on-site leader at each location.*
- **US-5** — *As an agent, I want to see the site (name, address) I'm placed at, so that I know exactly where to clock in.*

## 6. Functional requirements & business rules

| Ref | Rule |
|-----|------|
| ST-1 | A Site requires: **parent company**, **name**, **address**. Geo (`lat`/`lng`) and on-site PIC/phone are optional; `geofence_radius_m` defaults to **100**. |
| ST-2 | Site `name` is **unique within its company** (INV-5); duplicate names across different companies are fine. |
| ST-3 | A company has **≥1 Site**; **exactly one** is `is_primary`. Creating a company **auto-creates** a primary **"Main Site"** (F2.3). The primary flag can be **moved** to another site but never left empty. |
| ST-4 | Site status ∈ {Active, Inactive}. Only **Active** sites can receive new placements (E3 BR-3). |
| ST-5 | A site referenced by any placement (active or historical) **cannot be hard-deleted** — only deactivated. |
| ST-6 | Deactivating a site with **active placements** warns and requires those placements ended/transferred first (or blocks). The company's **last active site** cannot be deactivated while the company is active. |
| ST-7 | **Geofence = a single circle**: center (`lat`,`lng`) + `geofence_radius_m`. (Multi-circle/polygon are post-v1.) E5 treats out-of-geofence as **allowed + flagged**, not blocked. |
| ST-8 | A site **without geo** is allowed; E5 geofencing for it is **disabled + flagged** until geo is added (mirrors prior ClientCompany behavior). |
| ST-9 | `ClientCompany.leader_scope` (`company` \| `site`, default `company`) determines the **leadership unit** for E3 F3.4: `company` = one leader for all sites; `site` = one leader **per active site**. |
| ST-10 | All create/update/deactivate/primary-change actions are **audited** (E1). |

## 7. Data model

`Site`: `id, client_company_id (FK), name, code (nullable), address, lat (nullable), lng (nullable),
geofence_radius_m (default 100), is_primary (bool), pic_name (nullable), phone (nullable), status,
created_by`.

- Unique: `(client_company_id, name)`; partial-unique `(client_company_id) where is_primary` (one primary per company).
- `ClientCompany` **loses** `lat`, `lng`, `geofence_radius_m` (moved here) and **gains** `leader_scope`. See [F2.3](client-company-directory.md).
- `Placement.site_id` (E3) is a **required** FK → Site, with `site.client_company_id` = `placement.client_company_id`.

## 8. Acceptance criteria (Gherkin)

```gherkin
Feature: Client sites & geofence

  Background:
    Given I am signed in as an HR admin
    And the client company "Plaza Group" exists

  Scenario: A new company gets a primary Main Site automatically
    When I create the client company "Mall Kelapa Gading"
    Then a primary site "Main Site" is created under it
    And it is immediately a valid placement target

  Scenario: Add a second site with a geofence
    When I add a site "Plaza Senayan" to "Plaza Group" with an address
    And I set its geofence center and a radius of 100m
    Then the site is saved as Active with an active geofence
    And it becomes selectable as a placement target

  Scenario: Reject a duplicate site name within the same company
    Given "Plaza Group" already has a site "Plaza Senayan"
    When I add another site named "Plaza Senayan" to "Plaza Group"
    Then creation is blocked with a uniqueness error

  Scenario: Move the primary flag, never leave it empty
    Given "Plaza Group" has sites "Main Site" (primary) and "Plaza Senayan"
    When I mark "Plaza Senayan" as primary
    Then "Plaza Senayan" becomes primary and "Main Site" is no longer primary
    And the company still has exactly one primary site

  Scenario: A site without geo is allowed but flagged for E5
    When I add a site "Annex" without setting geo-coordinates
    Then the site is saved
    And E5 geofencing for "Annex" is disabled until geo is added

  Scenario: Block placement into an inactive site
    Given the site "Old Wing" is Inactive
    When an HR admin tries to place an agent at "Old Wing"
    Then it is blocked (E3 BR-3)

  Scenario: Prevent deactivating a site with active placements
    Given "Plaza Senayan" has active placements
    When I try to deactivate "Plaza Senayan"
    Then I am warned to end/transfer those placements first
```

## 9. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Site without geo | Allowed; E5 geofencing disabled/flagged for that site until geo added (ST-8). |
| C-2 | Deactivate the company's only/primary site | Blocked while the company is active (ST-6); deactivate the company instead (F2.3). |
| C-3 | Company switches `leader_scope` company→site while it has one company-level leader | Existing company-level assignment is flagged for re-designation; HR must name a leader per active site (E3 F3.4). |
| C-4 | Migration: legacy company with one address | Loader creates one primary "Main Site" per company, **geofence empty**; HR sets geo post-cutover (E9). |
| C-5 | Reassign primary to a site, then deactivate the old one | Allowed once it has no active placements; primary already moved (ST-3/ST-6). |
| C-6 | Two admins add same-named site to one company concurrently | Unique `(company, name)` constraint makes the second commit fail (ST-2). |

## 10. Dependencies

E1 (RBAC/audit), **F2.3** (parent ClientCompany + `leader_scope`; auto-creates the primary site),
E3 (placement targets a Site; `leader_scope` drives F3.4), E5 (geofence center/radius source),
E9 (migration auto-creates the Main Site), E10 (per-site reporting/coverage).

## 11. Decisions & open questions

**Resolved (2026-06-03 — EPICS §8):**
- ✅ Sites are first-class (ClientCompany 1→N Site); geofence relocated from ClientCompany onto Site.
- ✅ Every company has ≥1 Site; one auto primary "Main Site"; Placement targets a Site (required).
- ✅ Geofence model = single circle (center + radius, default 100m); out-of-geofence allowed + flagged (E5).
- ✅ `leader_scope` (company | site, default company) on ClientCompany drives per-company vs per-site leadership.
- ✅ Migration: sites net-new; loader auto-creates one Main Site/company; geofences configured post-cutover.

- ✅ **Operating hours / 24-7 flag = out of v1** *(2026-06-03)*. A Site carries no schedule metadata in v1; coverage views (E4/E10) don't depend on it. Revisit if per-site coverage analytics need it.
- **Open:** when `leader_scope=site`, may a single agent lead **two sites** of the same company in a pinch, or is it strictly one site per leader (mirroring INV-3)? (assumed strict 1:1 with a site.)
