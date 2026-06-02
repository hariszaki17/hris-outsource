# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

This is a **specification and design repository** — not an application codebase (yet). It holds the product decomposition, PRDs, data-migration mappings, and the visual design system for a **from-scratch rebuild of SWP's HRIS**, focused on managing **outsourced agents**. There is no Go/React/Postgres code here; the planned stack (Backend: Go · Frontend: React · DB: Postgres) is documented but unbuilt. When asked to "implement" something, you are almost always editing Markdown specs, the design system, or the `.pen` design file — not writing application code, unless the repo has since grown a code tree.

There are no build/lint/test commands. The artifacts are documents.

## Domain in one paragraph

**PT Saranawisesa Properindo (SWP)** is an outsourcing provider supplying agents across three **service lines** (Facility Services, Building Management, Parking) on shift-heavy, 24/7 client sites. The system is **internal-only**: only SWP staff log in. **Client companies are data, not tenants.** Four roles: super admin · HR/placement admin · **shift leader** (on-site supervisor, exactly 1 per client company) · agent. The differentiator is **Placement** as a first-class entity (in the legacy system it was just a string): an agent is *placed* at a client company, in a service line, for a contract period, with full history. Scheduling, attendance, leave, and overtime all hang off the placement record. The rebuild replaces the legacy `ims-system` (Laravel Lumen + Next.js, MySQL `lumen_swp`); **E9 migrates everything** (transform-and-load MySQL → Postgres, including read-only payroll history).

## Document structure & conventions

```
docs/
  EPICS.md                          ← master index: 10 epics, build sequencing, AUTHORITATIVE decision log (§8)
  epics/E<#>-<name>/
    FEATURE.md                      ← features (F<#>.<#>), actors, domain ER diagram, invariants, BPMN-style Mermaid flows
    prds/<feature>.md               ← per-feature PRD: user stories, business rules (BR-#), Gherkin AC, edge cases (C-#)
    DATA-MAPPING.md                 ← E2–E8 only: legacy lumen_swp (MySQL) → new Postgres model
  design/
    DESIGN-SYSTEM.md                ← single source of truth for look AND behavior; read before designing any screen
    brainstorm.pen                  ← the visual component library + screens (Pencil .pen file)
    swp-logo.png
```

The hierarchy is **epic → feature → PRD**. Totals: 10 epics, 43 PRDs, 7 data-mapping docs. Epics are dependency-ordered (E1 Foundations → E2 Identity → E3 Placement → E4 Scheduling → E5 Attendance → E6 Leave / E7 Overtime; E8 Payroll, E9 Migration, E10 Reporting are cross-cutting). The full table and build graph live in `docs/EPICS.md` §5–7.

### Where decisions live (important)
- **`docs/EPICS.md` §8 is the authoritative decision log.** When it conflicts with a per-epic `FEATURE.md` "Still open" section, **§8 wins** — per-epic docs are reconciled progressively.
- Within docs, `✅` = explicitly chosen decision; `*(default)*` = sensible default applied, overridable. Preserve these markers when editing.
- Decisions are dated (e.g. "Resolved 2026-05-29"). Use absolute dates, not relative ones.
- **Invariants** (e.g. INV-1: one active placement per agent) are stated in `FEATURE.md` §4 and referenced by ID from PRDs. Business rules (`BR-#`) and cases (`C-#`) cross-reference each other across the feature/PRD boundary — keep IDs stable when editing.

### When authoring or editing specs
- Match the existing structure exactly: PRDs follow a fixed section order (Context → Goals/Non-goals → Actors → User stories US-# → Functional requirements/BR-# → Data model → Gherkin AC → Cases C-# → Dependencies → Decisions). FEATURE docs use Mermaid (`flowchart`, `stateDiagram-v2`, `erDiagram`) for every workflow.
- Domain facts are grounded in **Indonesian labor law** (e.g. `PKWT` fixed-term vs `PKWTT` indefinite employment agreements; alih daya / outsourcing means the employment relationship is SWP↔agent, the placement is only a work *designation*). Don't contradict these.
- Migration docs reference real legacy schema (`employee_contracts`, `companies.role=2` = client company, `DBEncryption` cast on payroll columns, identity split between `users.id` and `employees.id`). Treat these as factual source-system details, not invented.

## Working with the design (`.pen` file + design system)

- **`.pen` files are encrypted — never use Read/Grep/Edit on them.** Access `docs/design/brainstorm.pen` only through the **Pencil MCP tools** (`get_editor_state` with `include_schema:true` first, then `batch_get`/`batch_design`/etc.).
- Before designing any screen, read `docs/design/DESIGN-SYSTEM.md`. Its working rules: design section-by-section (Foundations → Components → Overlays → Flows → Screens); **no dead-flow states** (every action leads to a designed result — modal, toast, error, empty, loading); **reuse `comp/*` library components**; **use design tokens, never raw hex**.
- Brand: primary green `#188E4D` is reserved for brand/primary actions, so the positive "present" *status* color is **teal**, not green. Full token table and attendance status→semantic mapping are in DESIGN-SYSTEM.md §2.

**Token-efficient `.pen` workflow** — design sessions burn tokens fast because every MCP call dumps large JSON (and screenshots are images) into context, and edits/screenshots bust the prompt cache:
- **Fetch the schema once** per session (`get_editor_state(include_schema:true)`); don't re-request it.
- **`batch_get` specific nodes**, not the whole file.
- **`batch_design`** multiple edits in one call instead of many round-trips.
- **Screenshot only when you need visual verification**, not after every edit.
- **One session per design section** (Foundations, then Components, …); `/clear` or start fresh between them so big payloads don't accumulate. `/compact` if a session gets long.

## Model strategy (which model for which phase)

Pick the model by **difficulty/ambiguity of the work**, not by "design vs code". Switch with `/model`, and keep design-system setup and screen generation in separate sessions so the large `.pen` context doesn't carry over.

| Work | Model | Why |
|------|-------|-----|
| Brainstorm, epics/PRDs, design-system **foundations** (tokens, base components, invariants) | **Opus** | Everything downstream reuses these; mistakes propagate. Worth the quality once. |
| **Screen generation** from a finished design system | **Sonnet** | Assembly work — pick `comp/*`, apply tokens, wire flows. Low ambiguity, pattern-following; faster and spares the Opus rate limit. |
| Repetitive code (CRUD endpoints, standard React forms/handlers) | **Sonnet** | Mechanical and well-specified by the PRDs. |
| Hard code — **E9 migration** (MySQL→Postgres transform-and-load, identity split `users.id` vs `employees.id`, `DBEncryption`), **placement invariants** (INV-1..4), state machines | **Opus** | Harder than the screens. Don't downgrade just because "it's code" — the migration epic is the gnarliest work in the project. |

Note: switching to Sonnet does **not** shrink `.pen` schema/screenshot payloads (same token volume on any model) — it just uses a lighter rate-limit bucket and runs cheaper/faster. Still apply the token-efficiency workflow above (one session per section, fetch schema once, screenshot sparingly).

## Notes

- This is an Obsidian vault (`.obsidian/`) — docs use Obsidian-flavored Markdown and relative links between files; keep links working when moving/renaming.
- IDs in the product use a `MIG-xxxxx` convention (rendered in mono per the design system).
