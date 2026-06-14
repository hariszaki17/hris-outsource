# PRD · F2.4 — Service Lines & Position Master · **RETIRED (2026-06-12)**

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.4 · **Status:** Retired
> **Parent:** [FEATURE.md](../FEATURE.md)

---

## Retirement notice

This feature is **retired**. Per the 2026-06-12 decision (EPICS §8):

- **`service_line` is removed entirely** from the product — no ServiceLine entity, no seed set, no scope axis anywhere (placements, shifts, overtime, holidays, reporting).
- **Position is FREE-TEXT** — there is **no Position master, no CRUD, no `service_line_id` scope, no uniqueness, and no `SWP-POS` id**. A position is a plain string captured **per placement** (E3 [agent-placement.md](../../E3-placement/prds/agent-placement.md) BR-9).

## What replaces it

- **Position entry** is a free-text field on each Placement (E3). The same agent may hold a different position string at a different company.
- **Position typeahead** — a search endpoint returns `DISTINCT` existing placement position values to aid consistent entry (no canonical list, no enforcement). Picking a suggestion is convenience only; a brand-new string is always allowed.
- **Reporting rollups** that previously grouped by service line now **group by position (free-text)** — see E10.

The prior invariants **INV-3** (position belongs to one service line) and **INV-4** (service line fixed seed set) are **deleted** from [FEATURE.md](../FEATURE.md). The `POST /service-lines` / `POST /service-lines/{id}/positions` endpoints and the service-line detail screen are **dropped**.
