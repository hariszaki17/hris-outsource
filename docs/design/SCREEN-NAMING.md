# Screen Naming Convention — brainstorm.pen

> **Status:** active · created 2026-06-19. **Authoritative** for how every frame in
> [`brainstorm.pen`](./brainstorm.pen) is named. Paired with
> [`SCREEN-INDEX.md`](./SCREEN-INDEX.md) (the registry) and
> [`SCREEN-CHANGE-PROTOCOL.md`](./SCREEN-CHANGE-PROTOCOL.md) (how changes propagate).

## Why this exists

`brainstorm.pen` is one flat canvas of ~150 screens. The board tree is organized by
**platform → role lane** (good for "what does this user see" + build-by-app-team), but
sprints are planned by **epic/feature**, which the tree can't answer. A flat, *greppable*
name encodes every axis in fixed positions, so any axis is searchable even though the tree
is role-first. The name is the join key; [`SCREEN-INDEX.md`](./SCREEN-INDEX.md) is the
queryable view.

Before this convention, ≥5 schemes coexisted (`E2 SL · Karyawan — List`, `E2 · Karyawan —
Daftar`, `Screen 1 · Kehadiran — Dashboard`, `HR · Koreksi · Antrian`, `Overlay · Tolak
Koreksi (E5)`). One scheme replaces all of them.

## The format

```
<scope> · <platform>/<role> · <Title>[ — <state>]
```

Each ` · ` separator is literal (space-middot-space). Example frame names:

```
E5 · mobile/agent · Absen
E5 · mobile/agent · Absen — outside-geofence
E1 · web/all · Login — locked
E10 · web/hr · Dashboard
E6 · web/sl · Persetujuan Cuti
E3 · web/hr · Detail Penempatan — terminated
```

### Field 1 — `scope` (epic, optionally feature)

- **Epic token** `E1`…`E11` — always present. This is the primary sprint/grep axis.
- Optional feature suffix when known and useful: `E5.F5.4`. Default to bare epic; add the
  feature only when it disambiguates (e.g. several E2 master-data sub-areas). The
  per-epic feature map lives in `docs/epics/E<#>-*/FEATURE.md` and the audit inventories
  in [`audit/`](./audit/).

### Field 2 — `platform/role`

| `platform` | meaning |
|---|---|
| `web` | React SPA console (frames ~1440w) |
| `mobile` | React Native app (frames ~390w) |

| `role` | meaning |
|---|---|
| `super` | super admin |
| `hr` | HR / placement admin |
| `sl` | shift leader (on-site supervisor) |
| `agent` | placed agent |
| `all` | pre-auth / shared (login, reset, session-expired) |

Multi-role screens: name for the **primary** role; record secondary roles in the index's
Role column (`hr,super`). Do **not** duplicate a frame per role — one frame, the index
re-slices.

### Field 3 — `Title`

The screen's human name in **Bahasa** (design copy is Bahasa-default). Sentence case.
Use the same noun the sidebar/nav uses so a screen is findable by what the user clicks.
No platform/role/state words in the Title — those are their own fields.

### Field 4 — `state` (optional)

Omit for the **canonical/default** screen. Present for every variant, lowercase kebab:

`empty` · `loading` · `error` · `locked` · `disabled` · `blocked` · `no-permission` ·
`session-expired` · `outside-geofence` · `no-gps` · `no-schedule` · `decrypt-fail` ·
`ended` · `terminated` · `resigned` · `superseded` · `success`

Add new states as needed; keep them kebab and reuse existing ones before inventing.

## Kind (index column, not in the name)

The frame name stays the same shape regardless of kind; `SCREEN-INDEX.md` tags each row:

| kind | what it is | example |
|---|---|---|
| `screen` | a real navigable screen | `E2 · web/hr · Karyawan — Daftar` |
| `overlay` | modal / drawer / bottom-sheet / popover / confirm | `E3 · web/hr · Akhiri Penempatan` |
| `panel` | a section/sub-component, not a full screen | `E2 · web/hr · Lokasi & Site (SitesPanel)` |
| `showcase` | a state-collection board (many states in one frame) | `E5 · web/hr · Form Validation States` |

Showcase/panel frames keep a parenthetical hint in the Title so they're obvious on canvas.

## Component (`comp/*`) naming — unchanged

Reusable components keep the `comp/PascalCase` convention (`comp/BtnPrimary`,
`comp/StatCard`). The design-system→code rule is 1:1 (`comp/*` ↔ `packages/ui`); see
`docs/eng/ENGINEERING.md`. This doc governs **screens**, not the library.

## Rules of thumb

1. Every screen/overlay/panel/showcase frame at canvas level **must** match the format.
2. The **POV-line container** frames (`POV line · …`) and lane frames keep their structural
   names — they are layout scaffolding, not screens, and are not indexed as screens. But
   tag each POV line with its epic in the name (`POV line · E5 · agent`) so the canvas is
   scannable.
3. When you add a screen: name it to spec **and** add its row to `SCREEN-INDEX.md` in the
   same session. An unindexed screen is invisible to sprint planning.
4. Frame **IDs never change** on rename/move — the index links by ID, so renames are safe.
5. Stale screens (feature removed) are **not** silently kept: mark `Notes: stale (<reason>)`
   in the index and either delete or move to a `Z · Archive` board.
