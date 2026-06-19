# Screen Change Protocol — brainstorm.pen

> **Status:** active · created 2026-06-19. How design changes propagate, and what each kind
> of change costs. Paired with [`SCREEN-NAMING.md`](./SCREEN-NAMING.md) and
> [`SCREEN-INDEX.md`](./SCREEN-INDEX.md).

## The propagation model — what is shared vs copied

A change is cheap only if the thing you edit is **shared** (one master, many instances).
The current `brainstorm.pen` shares some layers and not others — know which before you edit.

| Layer | Shared? | Edit point | Propagation |
|---|---|---|---|
| **Tokens** (color/spacing/type vars) | ✅ yes | Foundations vars / `set_variables` | O(1) — every node bound to the var updates |
| **Chrome** (Sidebar, Topbar, mobile nav) | ✅ yes — real `ref` instances | `comp/Sidebar`, `comp/Topbar`, `comp/AgentMobileNav`, `comp/SLMobileNav` masters | O(1) — all screens using the ref update |
| **Library widgets** (button, card, field, table row, status pill, toast, modal) | ⚠️ **partly** — masters exist in `comp/*` but most screens **rebuild them inline** | — | **O(screens)** today: edit must be repeated per screen |
| **Screen body layout** | ❌ per-screen | the frame | O(1) for that screen only |

### The instancing gap (measured 2026-06-19)

Spot-check of `E10 · web/hr · Dashboard` and `E5 · mobile/agent · Absen`:

- Chrome **is** instanced — `Sidebar→ref iCqTB`, `Topbar→ref caFkE`, `AgentMobileNav→ref gfptk`.
- Body widgets are **not** — the Absen `ClockInBtn` is a hand-built frame, not `ref
  comp/BtnPrimary`; the Dashboard KPI cards are raw frames, not `ref comp/StatCard`.

**Consequence:** a restyle of the primary button or KPI card is currently an O(~150) manual
sweep, not a one-master edit. This is the project's main design-scalability risk. It stems
from the `.pen` build constraint (insert-from-scratch paints blank → screens were built by
**copying** existing nodes, which detaches them from the master).

**Direction (retrofit, opportunistic — not a stop-the-world refactor):**
1. When you next touch a screen, replace its inline primary button / stat card / text field /
   table row with a `ref` to the `comp/*` master. Pay down debt where you already are.
2. Any **new** screen must instance `comp/*` for these high-frequency widgets, never rebuild.
3. Promote a widget to a `comp/*` master on its **2nd** domain-agnostic reuse (mirrors the
   `packages/ui` promotion rule in `docs/eng/ENGINEERING.md` §6).
4. Status colors only ever via `comp/StatusPill` (design-system rule).

## Change taxonomy — what to do for each

### Minor change (in-place, no index churn)
Copy tweak, single spacing fix, one screen's one state, fixing a typo in a label.
→ Edit the frame directly. No index/doc update needed (frame ID + name unchanged).

### Token change (global, O(1))
Brand color shift, spacing scale change, font swap.
→ Edit the Foundations variable. Verify a few screens visually. Never hardcode hex on screens
(design-system rule) — if you find a literal, that screen won't pick up the token: fix it.

### Shared-widget change (O(1) **if** instanced, else O(screens))
Restyle button, card, field, toast, modal.
→ Edit the `comp/*` master. **Then** check coverage: screens that rebuilt the widget inline
won't update — either accept the inconsistency, or retrofit those to `ref` (see above).
The index's `Notes` should flag known inline-rebuilt widgets so coverage is predictable.

### Single-screen content/state change (O(1), local)
New empty/error variant, changed layout of one screen.
→ Edit/duplicate the frame. If you **add a state variant**, add an index row for it
(same screen, new `state`). If you change the canonical name, update the index row.

### New feature / new screen (additive)
→ 1) Build the frame from the spec + `comp/*` (G0: build from `.pen`/spec, not assumptions).
   2) Name per [`SCREEN-NAMING.md`](./SCREEN-NAMING.md).
   3) Place under the owning **platform → role lane → POV line** (create the POV line if the
      epic has none in that lane).
   4) Add the row to [`SCREEN-INDEX.md`](./SCREEN-INDEX.md) **in the same session**.

### New role or new platform (structural)
→ New lane under the platform board (role) or new platform board. Update the index's axis
legend. Decide chrome: new role = new `comp/*Nav`/`comp/Sidebar` variant, instanced.

### Removing a feature (don't silently keep stale screens)
→ Mark affected index rows `Notes: stale (<reason + date>)`, then either `Delete` the frames
or `Move` them to a `Z · Archive` board. Known stale today (see index): **service-line**
screens (E2, dropped 2026-06-12) and **profile change-request** screens (E2, void per EPICS
§8 / E11). These should be archived or deleted in a cleanup pass.

## Minor vs major — one-line test

> **Minor** = the edit lives in exactly one frame and changes no name/ID → just edit.
> **Major** = it should propagate (token/widget), spans many frames (restyle), or changes
> the screen set (add/remove/rename) → edit the **shared master** or **update the index**,
> and verify coverage. If a "minor" copy fix is actually repeated across 10 screens, it was
> a shared-widget change in disguise — fix the master instead.
