# Engineering Principles & Code-Generation Rules — hris-outsource

> How we build. The durable rules that bind every coding session — the engineering counterpart to
> [EPICS.md §8](../EPICS.md) (product decisions) and [docs/api/CONVENTIONS.md](../api/CONVENTIONS.md) (API contract).
> Stack specifics live in [WEB-STACK.md](WEB-STACK.md).
>
> **Status:** v1 locked 2026-06-03. **Scope:** web console (the first surface we build). Backend (Go) and
> mobile (React Native) principles are added here as those surfaces begin.
> ✅ = explicitly chosen · *(default)* = sensible default, overridable.
>
> **Goals these rules serve:** production-grade · traceable · fast · secure · maintainable · scalable.

---

## 1. Traceability — make it mechanical, not manual

The specs already carry stable IDs; code cites them rather than re-describing the work.

- **A1 ✅** Every commit/PR references the spec ID it implements: feature (`F6.2`), business rule (`BR-#`),
  invariant (`INV-#`), or case (`C-#`). The Gherkin AC in each PRD is the acceptance contract.
- **A2 ✅** Provenance is generated, not written by hand:
  - `x-rbac` on each operation → a **client permission map** in `packages/api-client` (drives UI gating).
  - `x-design-screens` on each operation → an **endpoint ↔ `.pen` frame** index (links code to the design).
  These are build artifacts, so "which screen/role/permission uses this endpoint" is queryable, never guessed.
- **A3** *(default)* Resource IDs use the `SWP-<ENTITY>-x` convention ([CONVENTIONS §4](../api/CONVENTIONS.md))
  as **branded string types** (`type EmployeeId = string & { readonly __brand: 'SWP-EMP' }`) so an
  `SWP-EMP` id cannot be passed where an `SWP-PL` is expected. IDs are opaque — never parse the numeric.

## 2. Error & state handling — mirror the contract and the design system

- **B1 ✅** One API error boundary parses the shared `ErrorEnvelope` ([CONVENTIONS §11](../api/CONVENTIONS.md))
  and routes by `error.code`:
  | Signal | UX |
  |---|---|
  | `error.fields` present | map to React Hook Form field errors |
  | `401` / `UNAUTHENTICATED` | re-auth flow (`comp/EmptySessionExpired`) |
  | `403` / `FORBIDDEN` / `OUT_OF_SCOPE` | no-permission state (`comp/EmptyNoPermission`) |
  | `409` / `INV_*_VIOLATION` / `DOUBLE_SHIFT` | conflict UI for that invariant |
  | `422` / `QUOTA_EXCEEDED` / `OUT_OF_GEOFENCE` / `RULE_VIOLATION` | business-rule toast |
  | `404` | not-found (treated same as no-visibility, per contract) |
  One mapper, used everywhere — no ad-hoc error handling per screen.
- **B2 ✅** **No dead-flow states** (mirrors [DESIGN-SYSTEM.md §0.2](../design/DESIGN-SYSTEM.md)). Every async
  surface implements: loading/skeleton · empty · error/retry · no-permission · saving/disabled. A screen
  missing one is incomplete and fails review.
- **B3 ✅** Optimistic updates only where safe and reversible; for invariant-guarded writes (placements,
  scheduling, verification) wait for the server and reconcile the TanStack Query cache on success.

## 3. Security — this is an HRIS holding payroll + PII

- **C1 ✅** Client RBAC is **defense-in-depth, never the gate.** The UI hides unauthorized actions using the
  `x-rbac` permission map, but the **Go API is the source of truth**; the client always tolerates `403`/`404`
  defensively even for actions it thought were allowed.
- **C2 ✅** No tokens or PII in `localStorage`, URL params, or logs. Auth goes through the thin auth
  abstraction (token model locked with the backend, [WEB-STACK §6](WEB-STACK.md)); leaning in-memory access +
  httpOnly refresh cookie.
- **C3 ✅** `Idempotency-Key` (UUID v4) is auto-attached to every create / action / bulk mutation
  ([CONVENTIONS §13–14](../api/CONVENTIONS.md)) — handled in the client layer, not per call site.
- **C4** *(default)* Strict CSP; no `dangerouslySetInnerHTML` without sanitization; dependency audit
  (`pnpm audit` / Biome) in CI; secrets never in the bundle (build-time env only, no secrets shipped to the SPA).

## 4. Performance & scale

- **D1 ✅** **Cursor pagination only** — never offset — on heavy lists; tables for attendance / audit-log /
  schedule are **virtualized** ([CONVENTIONS §8](../api/CONVENTIONS.md)). Filters/cursors live in typed URL
  search params (TanStack Router) so views are shareable and the cache key is stable.
- **D2 ✅** Leader dashboards use **short-poll**, not tight loops (no WS/SSE in v1 by contract). Poll intervals
  are explicit and backed off when the tab is hidden.
- **D3** *(default)* Route-level code splitting; a bundle-size budget enforced in CI; images/assets optimized;
  TanStack Query `staleTime` tuned per resource volatility (master data long, attendance short).

## 5. Maintainability & code-generation rules — how code is authored here

- **E1 ✅** Feature-folder structure: `apps/web/src/features/<epic>/` mirrors `docs/epics/<E#-name>/`. Code is
  navigable straight from the specs.
- **E2 ✅** **Never hand-edit generated files** (`packages/api-client` Orval output). Need a change → change the
  spec and regenerate. Contract drift fails CI.
- **E3 ✅** **Tokens over literals** — never paste raw hex; use the design-token theme
  ([DESIGN-SYSTEM.md §0.4](../design/DESIGN-SYSTEM.md)). **Reuse before building** — compose from `packages/ui`
  (shadcn primitives) before hand-rolling; if a primitive is missing, add it to `packages/ui` first, then use it
  (mirrors the `.pen` "reuse `comp/*`" rule).
- **E4 ✅** **All user-facing strings through i18n** (Bahasa keys, `en-US` fallback) — no hardcoded copy.
  **All dates through the TZ-aware layer** (`Asia/Jakarta`) — no raw `new Date()` formatting; shift `HH:MM`
  fields are treated as `Asia/Jakarta` local per [CONVENTIONS §10](../api/CONVENTIONS.md).
- **E5** *(default)* Match the surrounding code's idiom, naming, and comment density. Small, single-purpose
  modules; colocate component + test + styles. Prefer composition over configuration flags.

## 6. Component architecture & design-system mapping — the reference for screen generation

This is the contract for **how UI code is structured and how the design system becomes code.** Screen
generation assembles from these layers; it does not hand-roll. Pairs with
[DESIGN-SYSTEM.md](../design/DESIGN-SYSTEM.md) §0.3 (reuse), §2 (tokens/status), §5 (shell),
§6 (interaction catalogue + form grid), §7 (role POV).

- **G0 ✅ Build from the `.pen`, never from assumptions — the design file is the visual contract.**
  Before building OR refactoring any screen, open [`docs/design/brainstorm.pen`](../design/brainstorm.pen)
  through the **Pencil MCP tools** (`.pen` is encrypted — never Read/Grep it):
  1. `get_editor_state(include_schema: true)` once per session — lists screens + the `comp/*` library.
  2. `batch_get` the specific screen frame (e.g. `E1 · Login (Web)`) **and its state variants**
     (e.g. `… — Gagal`, `… — Terkunci`, `… — Akun nonaktif`) at `readDepth` 4–5, plus the `comp/*`
     instances it uses.
  3. `get_screenshot` of that frame for visual fidelity (sparingly — flagship/ambiguous screens).
  Then match the frame's **layout, structure, copy (Bahasa), and every state variant** in code. Cite
  the `.pen` frame id in the PR (the `x-design-screens` map links endpoints → frames). A screen built
  without consulting its `.pen` frame is a process violation, even if it "looks fine."
  The resumable per-screen checklist + frame-id map is [SCREEN-GENERATION-PLAN.md](SCREEN-GENERATION-PLAN.md).
  _(Added 2026-06-03 after the login screen was first built from assumptions — a centered card — when
  the `.pen` defines a split-screen brand+form layout.)_

- **G1 ✅ Atomic layering — five layers, fixed homes:**
  | Layer | Lives in | What | `.pen` analogue |
  |---|---|---|---|
  | **Tokens** | `packages/design-tokens` | color/space/type/radius/elevation → Tailwind theme. The *only* source of these values. | DESIGN-SYSTEM §2–4 |
  | **Primitives (atoms)** | `packages/ui` | owned shadcn components themed to tokens: Button, Input, Select, Checkbox, Badge, Avatar… | base `comp/*` |
  | **Molecules** | `packages/ui` | domain-agnostic compositions reused everywhere: `FormField` (label+control+error, wired to `error.fields`), `DataTable` (virtualized), `CursorPagination`, `Modal`/`Drawer`/`ConfirmDialog`, `Toast`, `Banner`, `EmptyState`/`ErrorState`/`Skeleton`/`NoPermission`, `StatusBadge`, `IdChip` (mono `SWP-*`), `DateText` (TZ-aware) | composite `comp/*` |
  | **Organisms (feature components)** | `apps/web/src/features/<epic>/components` | domain-specific, compose molecules: `LeaveRequestForm`, `AttendanceVerifyTable`, `PlacementTimeline`. **Never** in `packages/ui`. | feature screens' parts |
  | **Templates / screens** | `apps/web/src/features/<epic>/` | page layouts on the app shell (DESIGN-SYSTEM §5), assembled from organisms | `.pen` screen frames |

- **G2 ✅ Promotion rule (rule of two + domain-agnostic test).** A component is born in its feature. The
  **second** feature that needs it → promote to `packages/ui` as a molecule — **only if it carries no domain
  knowledge.** Anything that knows about leave/placement/attendance/etc. stays a feature organism, forever.
  (Rule-of-three is too late for a design-system-driven app.)
- **G3 ✅ One canonical component per concept — no parallel variants.** Exactly one `Button`, one `Modal`, one
  `DataTable`. New needs are met by **props / composition / slots**, never by forking a second component.
  Mirrors the `.pen` rule: reuse `comp/*`; if a primitive is missing, **add it to `packages/ui` first, then
  use it.** A one-off restyle of an existing component is a bug.
- **G4 ✅ Design-system → code is mechanical and 1:1.**
  - Each `.pen` `comp/*` maps to exactly one `packages/ui` component (same name where practical).
  - The DESIGN-SYSTEM §6 interaction catalogue (modal · drawer · confirm · toast · banner; loading · empty ·
    error/retry · no-permission · saving) maps to **named molecules** — so "no dead-flow" (B2) is enforced
    structurally: the state has a component, or it isn't done.
  - Status colors come **only** from `StatusBadge` using the DESIGN-SYSTEM §2 status→semantic maps
    (attendance, placement) — never inline color. Teal = present, green = brand only.
  - The form-field grid discipline (DESIGN-SYSTEM §6, 2-column rhythm, full-width spans) is encoded in
    `FormSection`/`FormField`, not re-implemented per screen.
  - Role divergence (DESIGN-SYSTEM §7 POV lines) is driven by the `x-rbac` permission map (A2), not by
    duplicating screens per role.
- **G5** *(default)* **Component conventions:** typed props (no `any`); **composition over boolean-flag
  explosion** (prefer compound components / `asChild` / slots, per shadcn+Radix); forward refs on primitives;
  accessible by default (Radix semantics, labels, focus order); colocate `component + test + stories`.

## 7. Testing — the AC is the test spec

- **F1 ✅** Each PRD's **Gherkin acceptance criteria** drive the test plan: critical scenarios become Playwright
  E2E flows; business rules (`BR-#`) and edge cases (`C-#`) become Vitest cases.
- **F2 ✅** Tests run against **MSW** (handlers generated from spec examples) so they execute without the Go API
  and stay pinned to the contract.
- **F3** *(default)* Coverage gates favor invariant/rule logic over UI snapshots; every fixed bug gets a
  regression test citing its `C-#` / `BR-#`.

## 8. Decision log & deferred

**Locked 2026-06-03:** A1, A2, B1, B2, B3, C1, C2, C3, D1, D2, E1–E4, G0–G4, F1, F2 (markers above).

**Deferred (decided with the backend / at infra phase):**
- Auth token model + lifetimes ([WEB-STACK §6](WEB-STACK.md)).
- Go service principles (project layout, error mapping to `ErrorEnvelope`, migration epic E9 patterns) — added
  to this doc when backend work begins.
- CI/CD specifics, deploy target, observability/logging conventions.
