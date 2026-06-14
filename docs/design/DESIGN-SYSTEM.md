# hris-outsource — Design System & Rules

> The single source of truth for how the product looks **and behaves**. Read this before
> generating any feature screen. Lives alongside `brainstorm.pen` (the visual library) in
> `docs/design/`. The `.pen` file holds the canvas; this doc holds the rules.

---

## 0. Working rules (process)

1. **Design section by section.** Build and review one section at a time — Foundations →
   Components → Overlays → Interaction Flows → Feature screens — so the flow stays
   understandable. Don't jump ahead; finish and confirm a section before the next.
2. **No dead-flow states.** Every interactive element must lead somewhere that is **designed**.
   If a button opens a modal, the modal exists in the file. If an action succeeds, the toast/
   success state exists. If it can fail, the error/empty/loading states exist. An action with no
   documented result is a bug in the design.
3. **Reuse components.** New screens are assembled from the components in the library
   (`comp/*` reusable nodes) and the tokens below — not hand-rolled. If something is missing,
   add it to the library first, then use it.
4. **Tokens over literals.** Use variable references (`$primary`, `$text`, `$ok-tx`, …); never
   paste raw hex into a screen.
5. **Read ALL of a feature's PRDs before generating it.** Before any screen generation, read every
   PRD under that epic — and within each, the **Actors** and **Platform/clients** tables decide
   *which role uses which screen on which platform*. Build the POV lines from those tables, not from a
   subset or assumption. A role that appears in a feature's platform table **gets that screen in its
   POV line** (scoped per its rules). Common miss: a screen is shared by multiple roles differing only
   by scope (e.g. E5 dashboard & verification are used by **both** HR/Super Admin *and* Shift Leader —
   HR cross-company, leader own-company). Map every (role × surface) cell before drawing. _(Rule added
   after the E5 miss where Shift Leader lacked a dashboard and HR lacked verification.)_
6. **Full page + viewport line (always).** A screen frame is **always the full height of its
   content** — never truncate to fit the device. When the content runs **beyond the 1024 viewport**,
   draw the **viewport fold line** (1px line + pill at `y:1024`) to mark where the device cuts off, and
   still render every section below it. When content fits, the page is its natural height (≤1024) and
   the marker reads "Fits viewport". Pinned footers/pagination stay pinned; the fold pill must not
   collide with them (offset it to empty space). Full mechanics in §7.

---

## 1. Brand

- **Company:** PT Saranawisesa Properindo (**SWP**). Product: **HRIS Outsource** (internal SWP ops).
- **Logo:** the four-color ribbon "S" (`swp-logo.png`, from ims-system `Brand.png`). On the dark
  sidebar it sits on a **white rounded chip** so the gold/green/blue/purple stay legible.
  Wordmark: "SaranaWisesa" + sub "HRIS Outsource".
- **Primary brand color = green `#188E4D`.** Green is reserved for **brand & primary actions**.
  Because of that, the positive/"present" *status* color is **teal**, not green (see §2).

## 2. Color tokens

| Token | Hex | Use |
|---|---|---|
| `primary` | `#188E4D` | primary buttons, active nav, links, focus |
| `primary-strong` | `#0E6033` | hover/pressed primary |
| `primary-soft` | `#E7F4EC` | tints: active-nav bg, selected rows, badges |
| `text` / `text-2` / `text-3` | `#18181B` / `#52525B` / `#9CA3AF` | primary / secondary / muted text |
| `app-bg` / `surface` / `surface-2` | `#F3F4F6` / `#FFFFFF` / `#F9FAFB` | page / card / subtle fill |
| `border` / `border-soft` | `#E5E7EB` / `#EEF0F2` | dividers, card borders |
| `sidebar` / `sidebar-hover` / `sidebar-text` | `#18181B` / `#27272A` / `#A1A1AA` | dark nav |
| **Status — ok (teal)** | bg `#F0FDFA` · bd `#99F6E4` · tx `#0F766E` | Hadir, healthy, in-radius |
| **Status — warn (amber)** | bg `#FFFAEB` · bd `#FEDF89` · tx `#B54708` | Terlambat, pending |
| **Status — bad (SWP red)** | bg `#FBEAE9` · bd `#F1C5C0` · tx `#BF4A40` | Absen, rejected, error |
| **Status — info (blue)** | bg `#EFF8FF` · bd `#B2DDFF` · tx `#175CD3` | informational, terverifikasi |
| **Accents (from logo)** | gold `#F5A800` · green `#5E8C2A` · blue `#0B5FAE` · purple `#8E0E8E` | service-line / category coding, charts |
| `scrim` | `#18181B` @ ~70% | modal/drawer backdrop |

### Status → semantic mapping (attendance)
Hadir → ok · Terlambat → warn · Tdk Lengkap → `#ED962F` (onprogress orange) · Absen → bad ·
Verifikasi: Otomatis → neutral grey · Menunggu → warn · Terverifikasi → info · Ditolak → bad.

### Status → semantic mapping (placement, E3)
Draft → neutral · Terjadwal (Scheduled) → info · Aktif (Active) → ok · Akan berakhir (Expiring) →
warn · Berakhir (Ended) → neutral · Diberhentikan (Terminated) → bad · Resigned → bad ·
Superseded → neutral · Transfer (Transferred) → orange.

## 3. Typography
- **UI:** Inter. **Canonical ramp** — every UI text snaps to one of these variants, no ad-hoc sizes:

  | Variant | Size/Weight | Use |
  |---|---|---|
  | `pageTitle` | 30/700 | biggest screen title |
  | `section` | 22/700 | section heading |
  | `displayTitle` | 22/700 **Poppins** | brand wordmark + auth headlines (see Display below) |
  | `screenTitle` | 20/700 | tab / app-bar title (mobile) |
  | `cardTitle` | 19/700 | card heading |
  | `subtitle` | 15/700 | card/date headers (e.g. "Sel, 9 Jun 2026") |
  | `strong` | 14/600 | emphasised body |
  | `body` | 14/400 | body |
  | `label` | 13/700 | field / card-section labels (e.g. "Clock-in") |
  | `secondary` | 13/400 | secondary body |
  | `caption` | 12/400 | caption (use `weight` for 12/500·600·700 sublines) |
  | `badge` | 11/600 | flag/pill chips, table header (+0.5ls) |
  | `micro` | 10/500 | tab-bar labels |
  | `metric` | 28/700 | stat numbers |
  | `buttonLg` | 16/700 | primary button label |

- **Mono:** IBM Plex Mono — IDs (SWP-EMP-xxxx, SWP-PL-xxxx, etc. per `docs/api/CONVENTIONS.md`), times, coordinates. Variants `monoLg` (20/700, masuk/keluar tiles) and `monoHero` (46/700, live clock); any variant can opt into mono via the `mono` flag (e.g. a 13/500 mono value = `label` + `mono` + `weight="medium"`).
- **Mobile component contract:** ALL mobile text goes through `src/ui/Text.tsx` with a `variant` (+ optional `weight: regular|medium|semibold|bold` and `mono`). Size/family are applied via inline **style** (not className): NativeWind v4 lets a component's variant class shadow caller font classes, and arbitrary `text-[Npx]` / `font-mono-*` are unreliable. **Never hand-roll** `fontSize`/`fontFamily`/`text-[Npx]`/`font-*` on a Text — pick a variant. Weight = the font FAMILY (Inter_400/500/600/700 · IBMPlexMono_400/500/700 · Poppins_700); RN can't synthesize weights.
- **Display/brand:** Poppins/Gilroy (login & marketing only). On auth screens this is the
  **`displayTitle`** treatment = 22/700 in Poppins — used for the brand wordmark *and* every
  auth-screen headline (e.g. "Lupa kata sandi?", "Setel kata sandi baru"). Code: `Text`
  `variant="displayTitle"` (mobile `src/ui/Text.tsx`) — distinct from `section` (same size,
  Inter) so the global section ramp stays Inter elsewhere.

## 4. Spacing / radius / elevation
- **Spacing scale:** 2 · 4 · 6 · 8 · 10 · 12 · 14 · 16 · 18 · 20 · 24 · 32.
- **Radius:** 6 controls · 7–8 buttons/inputs · 10–12 cards · 999 pills/avatars.
- **Elevation:** card = 1px `border`; raised/overlay = soft shadow (y8, blur24, ~8% black).

## 5. Layout shell (every authenticated screen)
Dark sidebar (240) · white topbar (64, breadcrumb left + user right) · `app-bg` content (pad 24,
gap 16–20): **title band** (title + primary action) → optional **filter row** → **content card(s)**.

## 6. Interaction patterns (the "no dead-flow" catalogue)
Overlays: **modal** (forms), **drawer** (detail/edit), **confirm dialog** (destructive/approve).
Feedback: **toast** (success/error/info/warn), **inline** field validation, **banner** (page alert).
Async/data: **loading/skeleton**, **empty**, **error/retry**, **no-permission**, **saving/disabled**.
Forms: standard layout, field states (default/focus/error/disabled), multi-step, **bulk-action** flow.

**Form field grid (must hold):** every field row in a form section uses the **same column count**
(default **2 columns**, each `fill_container` so they compute to equal width and align top-to-bottom
across rows). Never mix a 3-up row into a 2-up section — it breaks the column rhythm and leaves
fields with no column to sit under. A field that needs the full width (e.g. *Alamat*) spans the row
as a single `fill_container` child; a lone trailing field occupies **one column** (left) with the
other column left empty — it does **not** stretch full width. Same discipline for read-only
detail grids.

Each core verb has a documented flow (see the "Interaction Flows" section of `brainstorm.pen`):
create → form modal → (validation error) → success toast; approve/verify (single & bulk) →
confirm → progress → toast + state change; reject → reason modal → toast; delete → confirm →
toast; export → options modal → toast/download; filter → loading → results | empty; row → drawer/detail.

## 7. Feature grouping & POV lines (canvas convention)

Every feature's screens live inside one **feature group** — a frame with a plain background
(`#E6E8EC`) that visually bundles them. Each group has:

- A **feature banner** (dark `#18181B`, full width): SWP logo chip · code+name (e.g. "E2 · Karyawan")
  · one-paragraph brief · **roles involved** as chips.
- One **POV line per role** whose screens genuinely diverge. A POV line = a **mini banner**
  (role-tinted, left accent: Admin→green, Shift Leader→blue, Agen→purple) reading
  `POV — <Role> · <access summary>`, then a horizontal row of that role's screens.
- Roles that differ only by notes (not separate screens) stay as the role panel + on-screen
  notes; give them a POV line only when their screens actually differ.

When a feature spans **multiple platforms** (web console vs mobile agent app), nest a **platform
sub-group** under the feature hood — a `PLATFORM · <name>` header band, then that platform's POV
lines beneath it. E.g. E2 → *Web Console* (Admin + Shift Leader lines) and *Mobile* (Agen line).
Different layout systems (desktop 1440 vs phone) stay in their own platform sub-group.

**Scroll model (runtime):** the live app is a fixed viewport (desktop 1440×1024); sidebar + topbar
fixed; the **primary block fills the content height and its body scrolls**, with pagination/
action-footer **pinned to that block's bottom** (never floating, never clipped against the frame edge).

**Canvas height convention (for review & MCP legibility):** on the canvas, screen frames grow to
their **full content height** so nothing is hidden below the fold — a reviewer (or an agent reading
the file) sees every section that exists, not just the first 1024px. Each screen carries a **fold
marker**: a 1px line + pill at `y:1024` (`layoutPosition:"absolute"`) labelled `Viewport fold ·
1024px` when content scrolls past the fold, or `Fits viewport · 1024px` when it doesn't. The sidebar
is `fill_container` height so it spans the full (grown) frame; the screen frame gets the measured
content height as a fixed value. This keeps runtime scroll behavior documented while making the full
design inspectable.

Naming: group `▦ FEATURE GROUP · <code> <name>`; screens `<code> · <name> — <screen>`.

---

## 8. Slicing contract (design → React)

> **Read this before slicing any screen to code.** It is the bridge between the `.pen` canvas and
> the React codebase so the slicing agent resolves component identity, tokens, data binding, and
> behaviour **from fact, not assumption**. The `.pen` is the visual source; this section is the
> semantic key to it. When the two disagree, re-read the node over MCP and trust the file's geometry,
> but trust THIS doc for *intent* (what a node *means*, what's dynamic, what an action does).

### 8.0 How to read the design over MCP
- Tools: `get_variables` (tokens), `get_editor_state(include_schema:true)` (schema + top-level node
  + reusable component list), `batch_get([ids])` (full subtree of a node), `snapshot_layout(parentId)`
  (geometry/sizing only), `export_nodes`/`get_screenshot` (pixels). Never `Read`/`Grep` the `.pen`.
- `batch_get` sometimes returns extra top-level frames — filter to the IDs you asked for.
- Quirk: freshly edited nodes can render blank briefly; for visual checks re-shoot or `export_nodes`.
- **A node's `name` is its semantic label** (`comp/StatusPill`, `Nav Karyawan`, `Row · Tanggal
  Bergabung`). Generic auto-IDs are not meaningful; rely on `name` + this doc.

### 8.1 Component → React map
Reusable components live near the top of the doc (`reusable:true`). Slice each to **one React
component**. Instances (`type:"ref"`) = usages; their `descendants` overrides = the props passed.

| `.pen` node | id | React component | Props (and where they live in the node) | Variants / states |
|---|---|---|---|---|
| comp/StatusPill | `qxONU` | `<StatusPill status label/>` | `label` = text `VPQWU`; dot = `ZE8Ck` | `status ∈ ok\|warn\|bad\|info\|orange\|neutral` → fill `$<s>-bg`, stroke `$<s>-bd`, dot+text `$<s>-tx` |
| comp/Avatar | `YVANc` | `<Avatar initials/>` | `initials` = text `z5Q636` (2 chars) | size 38, radius 9, `$primary-soft`/`$primary` |
| comp/StatCard | `lmwet` | `<StatCard label value sub icon/>` | head `mPbJy`, value `X4fGq`, sub `R8iju2` | — |
| comp/Sidebar | `iCqTB` | `<Sidebar active/>` | nav items: Dashboard `E6hncY`, Karyawan `DkjqM`, Penempatan `mTyVi`, Jadwal Shift `G6Xba`, Kehadiran `VFNOe`, Cuti `m8FgOH`, Lembur `PBlID`, Laporan `MM6qG`; brand `K8rS6`; footer `EqHHS` | active item = fill `$sidebar-hover` + left border 3px `$primary` |
| comp/Topbar | `caFkE` | `<Topbar breadcrumb user/>` | left/breadcrumb `R1f15`, right/search+user `hxnE9` | — |
| comp/FilterSelect | `t60nEC` | `<FilterSelect label options/>` | label `OO3wI`, chevron `IKJhE` | open/closed |
| comp/SearchField | `vJBJZ` | `<SearchField placeholder/>` | placeholder `T7tdYf`, icon `UG61h` | — |
| comp/BtnPrimary | `Y7IwcG` | `<Button variant="primary">` | icon `P6AwTN`, label `NWbfs` | see note ↓ |
| comp/BtnSecondary | `TR9pR` | `<Button variant="secondary">` | icon `r0YlIj`, label `WBHtp` | |
| comp/BtnGhost | `AEl1Q` | `<Button variant="ghost">` | icon `okPQv`, label `g6GK7` | |
| comp/BtnDanger | `d5HQSI` | `<Button variant="danger">` | icon `xvot9`, label `NM92f` | |
| comp/TextField | `nVepR` | `<TextField label value placeholder state/>` | label `h5FFw`, box `Wzyx2` | `state ∈ default\|focus\|error\|disabled` |
| comp/Checkbox | `LwfGi` | `<Checkbox checked label/>` | box `m6tQe`, label `e3rXD` | checked/unchecked |
| comp/Toggle | `Uma0O` | `<Toggle on/>` | knob `IkDX3` | on/off |
| comp/Toast | `PtJHa` | `<Toast variant title desc/>` | icon `PRWXg`, text `LitBn`, close `ESljA` | `variant ∈ success\|error\|info\|warn` |

**Button note:** the four `Btn*` components are one visual family — slice them as a **single
`<Button variant>`** (primary/secondary/ghost/danger), not four components. Same for any pill/badge:
prefer one component with a `status`/`variant` prop over per-color copies.

### 8.2 Token → CSS variable map
Every `$name` in the file is a token from `get_variables`. Emit each as a CSS custom property
`--<name>` (and/or a Tailwind theme key). **Never hardcode a hex that matches a token** — bind the var.

| Group | Tokens |
|---|---|
| Brand | `primary #188E4D` · `primary-strong #0E6033` · `primary-soft #E7F4EC` |
| Text | `text #18181B` · `text-2 #52525B` · `text-3 #9CA3AF` |
| Surface | `app-bg #F3F4F6` · `surface #FFFFFF` · `surface-2 #F9FAFB` |
| Border | `border #E5E7EB` · `border-soft #EEF0F2` |
| Sidebar | `sidebar #18181B` · `sidebar-hover #27272A` · `sidebar-text #A1A1AA` |
| Status ok (teal) | `ok-bg #F0FDFA` · `ok-bd #99F6E4` · `ok-tx #0F766E` |
| Status warn | `warn-bg #FFFAEB` · `warn-bd #FEDF89` · `warn-tx #B54708` |
| Status bad | `bad-bg #FBEAE9` · `bad-bd #F1C5C0` · `bad-tx #BF4A40` |
| Status info | `info-bg #EFF8FF` · `info-bd #B2DDFF` · `info-tx #175CD3` |
| Status orange | `orange-bg #FFF3E8` · `orange-bd #FDD9B5` · `orange-tx #C2410C` |
| Accents | `accent-gold #F5A800` · `accent-green #5E8C2A` · `accent-blue #0B5FAE` · `accent-purple #8E0E8E` |
| Misc | `scrim #18181BB3` |
| Fonts | `font-sans = Inter` · `font-mono = IBM Plex Mono` |

### 8.3 Layout primitives → CSS
- Screen frame = a route/page. Shell = `<Sidebar>` (fixed 240) + main column (`<Topbar>` 64 + content).
- `layout:vertical|horizontal` → flexbox `flex-direction column|row`; `gap` → `gap`; `padding` →
  `padding`; `justifyContent`/`alignItems` map 1:1. `layout:none` → absolute children (`x`/`y`).
- `fill_container` → `flex:1` / `width:100%` along the axis; `fit_content` → `width:fit-content` /
  natural height; a fixed number → fixed px. A row of `fill_container` siblings = equal columns.
- **Tables** follow Table → Row → Cell (frame) → content. Each cell is a frame; slice to `<td>`/grid cell.
- **Fold marker** (`Viewport fold · 1024px` line) is a **canvas-only annotation** — do NOT slice it.
  It marks the runtime viewport edge; see §7 for the runtime scroll model (sticky topbar/sidebar,
  scroll body, pinned footer/pagination).

### 8.4 E2 — Employee data model (the entity behind Karyawan screens)
Screens by role / platform (all under feature group `Z3cS3`):
- **Web · HR/Admin** (canonical): List `WElYh` · Detail `JBjBb` · Form (Tambah/Edit) `h6bDz`.
- **Web · Shift Leader** (read-only, company/location-scoped): scoped List `n3wi1w` (no Tambah, scope
  banner) · read-only Detail `rtKzk` (no Edit; Statutori & Akun Login hidden → replaced by a locked
  note; role can't manage credentials or see statutory fields).
- **Mobile · Agen** (self-service phone, 390×844): Profil Saya `s5RO1` (read-only; statutory locked,
  Kontak/Bank flagged editable) · Ajukan Perubahan `n465cT` (only `phone`/`address`/bank editable,
  statutory locked, submits to HR review) · Status Pengajuan `SXqA5` (request states:
  Menunggu→`warn`, Disetujui→`info`, Ditolak→`bad`).

Role access rule of thumb: HR/Admin = full CRUD + credential management (regenerate temp password,
deactivate; every employee already has a login) + statutory; Shift Leader = read-only within their
location, no statutory/credentials; Agen = own record, read-only except a request flow for
phone/address/bank that HR must approve. Fields, grouped as on the form:

| Group | Field | Type | Rules |
|---|---|---|---|
| Pribadi | `fullName` | string | required; HR-only edit |
| | `nik` | string(16–18 digit) | required, **unique**; HR-only edit |
| | `nip` | string | optional |
| | `gender` | enum `L\|P` | |
| | `birthPlace` | string | required |
| | `birthDate` | date | |
| | `joinDate` | date | required |
| Kontak | `phone` | string | agent self-editable |
| | `email` | string | |
| | `address` | string(multiline) | agent self-editable |
| Statutori & Bank | `npwp` | string | HR-only edit |
| | `bpjsHealth` | string | |
| | `bpjsEmployment` | string | |
| | `bankName` | string | agent self-editable |
| | `bankAccount` | string | agent self-editable |
| | `accountHolder` | string | |
| Akun Login (opt 1:1) | `hasLogin` | bool (Toggle) | login is optional |
| | `loginEmail`/`username` | string | |
| | `role` | enum `super_admin\|hr_admin\|shift_leader\|agent` | default `agent` |
| Status | `active` | bool | **deactivate, never delete** |

Roles (access actors): `super_admin`, `hr_admin`, `shift_leader`, `agent`. Platforms: web console
(admin/HR/shift-leader) + mobile (agent). On Detail/Form, any **static-looking value text bound to one
of these fields is dynamic** — bind it, don't hardcode the sample (e.g. "Budi Santoso" = `fullName`).

### 8.5 Interaction map (verb → result; no dead-flow)
The designed result of every action lives on the **Overlays** page `hoY3q` and **Interaction flows**
page `yTcDc`. Wire React handlers to these:

| Trigger | Result (designed) |
|---|---|
| Tambah / Edit (primary btn) | form screen/modal → inline validation on error → success **Toast** |
| Table row click | navigate to **Detail** |
| Deactivate / destructive | **confirm dialog** → success Toast + status change (never hard delete) |
| Verify / approve (single & bulk) | confirm → progress → Toast + state change |
| Reject | reason modal → Toast |
| Export | options modal → Toast / download |
| Filter / search | loading → results **or** empty state |
| Field validation | inline error state on `TextField` (`state="error"`) |
| No permission (role-gated) | no-permission / disabled state |

State→token mapping for status pills/badges is in **§2** (Hadir→ok teal, Terlambat→warn, Tdk
Lengkap→orange, Absen→bad; Otomatis→neutral, Menunggu→warn, Terverifikasi→info, Ditolak→bad).

### 8.6 Slicing rules of thumb
1. One `reusable` component → one React component; instance overrides → props. Don't re-slice the
   same component differently per screen.
2. Bind tokens, never hex. Bind data fields (§8.4), never sample copy.
3. Respect the shell: Sidebar + Topbar are shared layout, not per-page.
4. Honour the runtime scroll model (§7), ignore the fold-marker annotation.
5. If intent is unclear, the answer is in this doc or the Overlays/Flows pages — **look before assuming**.

### 8.7 E3 — Placement (the differentiator)
Screens (under feature group `zTfKp`):
- **Web · HR/Placement Admin**: Companies overview `C2SSLA` · Company Roster `nLN4d` · Buat
  Penempatan (create) `g3OzZz` · Detail + Lifecycle `pFR79`.
- **Web · Shift Leader**: read-only own-company Roster `o5Txgg` (no create / end / transfer /
  assign-leader; export allowed; scoped to the one company they lead).
- **Mobile · Agen**: Penempatan Saya `mqGEi` (active placement + history, read-only).

Placement `status` enum (LC-1) → token: `Draft`→neutral · `Scheduled`(Terjadwal)→info ·
`Active`(Aktif)→ok · `Expiring`(Akan berakhir)→warn · `Ended`(Berakhir)→neutral ·
`Terminated`(Diberhentikan)→bad · `Resigned`→bad · `Superseded`→neutral · `Transferred`→orange.

Key entities (read FEATURE.md for full ER): **Placement** (`employee_id`, `employment_agreement_id`,
`client_company_id`, `service_line_id`, `position_id`, `start_date`, `end_date?`, `status`,
`annual_leave_entitlement`, `base_salary_ref`, `predecessor_id`, `ended_reason`) · **ShiftLeaderAssignment**
(`client_company_id`↔`employee_id`, strict 1:1) · **EmploymentAgreement** (PKWT/PKWTT, lives in E2).
Invariants: one active placement per agent (INV-1); exactly one shift leader per company (INV-2/3);
the leader must be actively placed there (INV-4). **Renewal/transfer = a new linked record**
(`predecessor_id` chain) — history is never edited in place. Service lines: Facility / Building Mgmt
/ Parking. The roster/detail "Leader" badge marks the agent who is that company's shift leader.

### 8.8 E4 — Shift Scheduling
Screens (feature group `WnejY`):
- **Web · HR/Super Admin**: Master Shift catalog `O5JgF` · Tambah Shift (modal over the catalog) `Mn9ux`.
- **Web · Shift Leader**: Daily Schedule Grid (week) `Rubba` — rows = placed agents × 7 day columns;
  cell states: **shift chip** (dot+name+time) / **Libur** (`Off`) / **`+` empty** (assign) / **Cuti**
  (leave-locked, blocked); today column tinted; auto-publish banner; "Terapkan ke rentang" bulk helper.
- **Mobile · Agen**: Jadwal Saya `fN9AJ` (today's shift hero + upcoming list, read-only).

Entities: **ShiftMaster** (`title`, `start_at`, `end_at`, `start_break?`, `end_break?`,
`service_line_id?` tag, `spans_midnight`, `status`) · **Schedule** (`employee_id`, `shift_master_id`,
`placement_id`, `work_date`, `status` ∈ `Scheduled|Off|Changed`); unique `(employee_id, work_date)`.
Rules: one shift/agent/day (INV-1); only agents with an **active placement** that date (INV-2); a
shift leader schedules **own company only** (INV-3); **save = auto-publish + notify** (INV-4 — no
draft/approval gate). Shift picker shows all active shift masters. **Scheduling over
approved leave is blocked** (the Cuti cell is locked). **Cross-midnight** shift attributes to its
start date (shown with a `+1` moon badge). Shift-master status: Aktif→ok · Nonaktif→neutral.

### 8.9 E5 — Attendance
Screens (feature group `cjTd7`):
- **Web · HR/Super Admin** (cross-company, all): Kehadiran Dashboard `sZCLW` · Verifikasi Kehadiran
  queue `UEG2J` (exceptions; bulk verify/reject; also handles no-leader-company escalations) · Detail
  Verifikasi `VY894` (GPS/geofence map, event timeline, verify/reject).
- **Web · Shift Leader** (own company only — scope banner + locked company filter; can't self-verify →
  escalates to HR): Team Attendance `V2QL7` · Verifikasi `MsXnm` · Detail Verifikasi `RZPQz`. Same
  screens as HR, scoped — per F5.3 VF-2 & F5.5 AR-1.
- **Navigation/IA**: sidebar **Kehadiran → Dashboard** is the main attendance page. The verification
  queue is a focused sub-view reached from the Dashboard via the **"Verifikasi (N)"** title-band button
  + the "Belum diverifikasi" stat + exception rows (no separate sidebar item). Shift-leader counts are
  own-company; HR counts are cross-company.
- **Mobile · Agen**: Absen `Iek78` (GPS clock in/out — live time, geofence status chip, big Clock In,
  today's masuk/keluar) · Riwayat Kehadiran `PAOwr` (history + status + verification badges).

Entities: **Attendance** (`employee_id`, `schedule_id?`, `placement_id`, `attendance_code_id`,
`check_in_at`, `check_out_at?`, `lat/lng_in/out`, `in_geofence_in/out`, `is_late`, `late_minutes`,
`auto_closed`, `status` ∈ Present|Late|Incomplete|Absent, `verification_status` ∈
AutoApproved|Pending|Verified|Rejected) · **AttendanceCorrection** (`type`, `corrected_time`, `status`).
Status→token: Present(Hadir)→ok · Late(Terlambat)→warn · Incomplete(Tidak lengkap)→bad ·
Absent→bad · Pulang awal (early-out flag)→orange. Verification→token: AutoApproved(Terverifikasi
otomatis)→info · Pending(Menunggu)→warn · Verified→ok · Rejected→bad.
Rules: **GPS geofence only** (center + radius per **Site**, E2 F2.6, default 100m); **out-of-geofence allowed +
flagged** (not blocked); late grace **15 min**; **exceptions-only verification** (clean records
AutoApproved, only late/out-of-geofence/auto-closed/absent/code-flagged reach the queue);
**auto-clock-out at shift end** → Incomplete + Pending; cross-midnight attributes to start date;
leaders' own exceptions escalate to HR (no self-verify); self-correction window **7 days**; **billable
= verified records only** (feeds E10).

### 8.10 E6 — Leave (Cuti)
Screens (group `EduUv`):
- **Web · HR/Super Admin**: Persetujuan Cuti L2 `yho5i` · **Detail Pengajuan `DJrBn`** (full record +
  approval trail + balance impact + delegate-as-suggested-backfill + coverage note) · Kuota `P6HZ7E`
  (+ **Sesuaikan Kuota modal `CGCnL`** · **Terbitkan Kuota Tahunan modal `W2zYM`**) · Kalender Cuti `s5niW`.
- **Web · Shift Leader** (own company, scoped): Persetujuan Cuti L1 `qb0S0` · Kalender Cuti Tim `YvYcr`.
- **Mobile · Agen**: Ajukan Cuti `QT92D` (type/dates/**doc upload**/delegate + balance) · Status Pengajuan
  `hjCYy` (approval timeline) · Cuti Saya `o1BUa` (balance + history).

_Design-review refinements (2026-05-31):_ approval-queue **Detail** opens `DJrBn` (was a dead-end);
quota **Sesuaikan** → `CGCnL` (new total/remaining + **required reason**, LQ-6); grant button renamed
**"Terbitkan Kuota Tahunan"** → confirm/setup `W2zYM` (period · default entitlement · pro-rata · preview
count; repairs, doesn't overwrite used). **Coverage model:** placement (E3, long-term) ≠ delegation
(E6, informational suggestion) ≠ coverage (E4 scheduling). Approved leave → cleared shifts become
**uncovered slots** the leader backfills (same company+line); delegate shown as **non-binding suggested**
backfill; **no auto-substitution / cross-company borrow in v1**. Calendar **clash is service-line-aware**
(≥2 same-line agents off = "perlu pengganti").

Entities: **LeaveQuota** (`total`/`used`/`remaining`, per quota-type, per period; expires period-end) ·
**LeaveRequest** (`type`, `start/end`, `duration_days`, `reason`, `document?`, `delegate?`, `status`) ·
**LeaveApproval** (`level`, `approver`, `decision`, `reason`). Status→token: Pending/Menunggu→warn ·
LeaderApproved→info · Approved/Disetujui→ok · Rejected/Ditolak→bad · Cancelled/Ditarik→neutral.
Rules: annual = **lump grant per calendar-year, expires at period end, no carryover**; **over-balance
annual blocked** (INV-1); **document mandatory** if leave type requires it; **two-level approval**
Shift Leader (L1) → HR (L2), reject at either level → Rejected + reason; **balance re-checked at final
approval**; **no self-approve** (escalate); approved leave **clears scheduled shifts** (E4 Cuti cell) +
**suppresses Absent** (E5); **pro-rata** grant for probation/mid-year joiners; no-leader company → L1
escalates to HR.

### 8.11 E7 — Overtime (Lembur)
Screens (group `SGrJK`):
- **Web · HR/Super Admin**: Persetujuan Lembur L2 `H1eBN` · Aturan OT & Kalender Libur `vd4na` · Rekap Lembur `JEmCk`.
- **Web · Shift Leader** (scoped): Persetujuan Lembur L1 `Vh2P9`.
- **Mobile · Agen**: Ajukan Lembur `wDLQu` · OT Terdeteksi (konfirmasi auto-detect) `mzCUA` · Lembur Saya `nd3KT`.

Entities: **OvertimeRule** (`day_type`, `multiplier` ref, `min_minutes`, `requires_preapproval`,
`service_line_id?`) · **Overtime** (`work_date`, `start/end`, `duration_minutes`, `day_type`,
`source` ∈ Requested|AutoDetected, `attendance_id?`, `status`) · **OvertimeApproval** · **HolidayCalendar**
(`date`, `name`, `recurring`). Day-type tiers: Workday=Hari Kerja · RestDay=Hari Libur · Holiday=Hari
Besar. Source: Auto-deteksi→info(sparkles) · Request→neutral(user). Status→token same as E6.
Rules: **multipliers are reference-only** (v1 records hours, no money); **service-line rule overrides
global**; `min_minutes` threshold (below = ignored); **auto-detect** from verified attendance past
shift-end ≥ min_minutes (needs agent confirmation); **two-level approval** SL→HR (no self-approve, bulk
allowed); **approved-only counts** in reports; cross-midnight → start day; rekap groups hours by tier →
feeds payroll context (E8) + billing (E10).

### 8.12 E8 — Payroll (read-only history)
Screens (group `KLvfV`): no Shift Leader line (actors = Agent + HR only).
- **Web · HR/Super Admin**: Arsip Payroll `jBgLn` (searchable list + confidentiality banner) · Detail
  Slip Gaji `JaScP` (full components + benefits + export).
- **Mobile · Agen**: Slip Gaji daftar `v8uQX2` · Detail Slip ringkasan `ocsq4` (**summary only**).

Entities: **Payslip** (`period`, `paid_on`, `working_days`, `gross_earnings`, `gross_deductions`,
`take_home_pay`) + **SalaryComponent** line items + **Benefit** (HR archive only). Rules: **READ-ONLY**
(no create/edit, INV-1); **agent sees own summaries only — NO component breakdown** (PH-5); **HR sees
full archive** (components + benefits) + export; monetary fields **decrypted on read** for authorized
viewers; **every view/export audited**; exports carry a **confidentiality marking**; decryption failure
→ row flagged "Perlu review" (don't crash). Sidebar nav uses **Laporan** (no dedicated payroll item).

### 8.13 E10 — Reporting & Notifications (Laporan)
Screens (group `w5JgL`):
- **Web · HR/Super Admin**: Dashboard `ETi5H` (cross-co KPIs + bar charts + action list) · Laporan
  Kehadiran & Jam Billable `EF8AZ` (+ **Ekspor modal** `FJ6hX`) · Pusat Notifikasi `i0qW8`.
- **Web · Shift Leader** (scoped): Dashboard Tim `RiSPW`.
- **Mobile · Agen**: Notifikasi `WKYgI` · Beranda `e8Sw1` (personal dashboard).

Entities: **Notification** (`type`, `payload` deep-link, read/unread) · **ExportJob** (`report_type`,
`filters`, `format`, `status`). Rules: notifications **push + in-app only**, scoped to recipient,
deep-link to the source feature, **critical categories override mute**; dashboards **scoped** (agent
own / SL company / HR all), widgets deep-link, near-live; **billable report = verified + billable-code
only, hours-only (no rates), pending excluded**, SL sees own company; **export framework** xlsx/pdf/csv
honors the on-screen filters+scope (no escalation), **large → queued + notify**, audited, **point-in-time**,
sensitive exports carry confidentiality marking. Charts: simple flex bar charts (value-proportional
fixed heights), never absolute-overlaid.

### 8.14 E1 — Foundations & Platform
Screens (group `N08erR`):
- **Web · Auth & Admin (Super Admin/HR)**: Login `lKRjr` (split brand panel + auth card) · Pengguna &
  Peran `kHNWT` (RBAC + company scope) · Audit Log `rtJRB` · Pengaturan `m3sWh` (Settings).
- **Mobile · All roles**: Login `Y09E0`.

Entities: **User** (`email`, password hash, `role`, `active`, `last_login_at`) · **Role** — fixed 4:
`super_admin` / `hr_admin` / `shift_leader` / `agent` · scope via E3 **ShiftLeaderAssignment** ·
**AuditLog** (`actor_user_id|system`, `action`, `entity_type`, `entity_id`, `before`, `after`, `ip`,
`created_at` — **append-only / immutable**, sensitive values **masked**). Role badge tokens:
super_admin→`accent-purple` · hr_admin→`primary` · shift_leader→`accent-blue` · agent→neutral.
Rules: **email+password** (hashed), only active users log in, lockout after 5 fails, sessions revocable;
**RBAC server-side on every request** (UI hiding is not enforcement); shift_leader **company-scoped**,
agent **self-scope**, HR/super **cross-company**; audit **HR/Super Admin only**. Platform conventions:
**Bahasa Indonesia** UI (i18n-ready), **Asia/Jakarta WIB (UTC+7, no DST)** canonical (store UTC, render
WIB), **IDR** + Indonesian formatting, **role-based nav** on web + mobile. **Login uses Poppins display
font + a split brand panel** (not the standard sidebar shell).

### 8.15 E11 — Approvals (configurable engine) *(added 2026-06-14)*
Routing is now a per-company **template** of ordered lines (OR within a line, sequential across lines);
leave/OT route through it. Screens live in the platform×role boards (not a per-epic group — the canvas
was reorganized into `PLATFORM · WEB CONSOLE` `nNYxY` + `PLATFORM · MOBILE` `yPwPD`, lanes by role).
- **Web · HR/Super Admin** (Lane · Other roles `Uy7CG`, POV line `udZWc`): Template editor `d7tFAM`
  (2–3 line cards, each an OR-set of member chips + "Tambah anggota", "lalu (berurutan)" connectors,
  optional removable Baris 3, live-reset warning) · Kotak Masuk `yv7Gs` (type tabs + table with a
  **Baris N/M** column + Setujui/Tolak row actions) · Detail Permintaan `OHseV` (request card + **chain
  timeline**: done line shows OR-clearer + "tidak perlu bertindak", current line ringed, action trail;
  **super-admin Bypass** card). Overlays: Bypass modal `KT3Jz` (reason required), Reset-pending confirm
  `uoTwN`; **reject reuses `comp/ModalReject` `EnabP`**.
- **Mobile · Shift Leader** (Lane `Iavxr`, POV `sXgHB`): approver inbox `DxK66` (current-line cards +
  Setujui/Tolak) · approve **bottom sheet** `viUFF` (mini chain progress + optional note).
- **Mobile · Agent** (Lane `AikTF`, POV `zTfPi`): Status Pengajuan `PGrLa` (read-only **chain timeline**
  replacing L1/L2 + action history).

Entities (read E11 FEATURE.md): **ApprovalTemplate** (`company_id` unique, `version`, lines) ·
**ApprovalLine** (`line_no` 1..3, OR-set members) · **ApprovalInstance** (`request_type`, `request_id`,
`current_line`, `status` ∈ PENDING|APPROVED|REJECTED) · **ApprovalAction** (`APPROVE|REJECT|BYPASS`,
reason, append-only). Status→token: Pending/Menunggu→warn · current line→warn · cleared line→ok ·
upcoming→neutral · Rejected→bad · Bypass→`accent-purple`. Rules: **routing = line membership**
(server-enforced, not a `*.approve` perm); OR within a line, sequential across; **no self-approval**
(INV-3); **super-admin bypass** with reason (INV-5); **no template → super-admin fallback** (INV-7);
**live template + pending reset** on edit (INV-6); leave/OT side-effects fire from the engine's
`OnApproved`/`OnRejected` hook. **Profile change-requests removed** — the E2 `Ckteo` CR queue +
agent CR mobile screens are deleted; profile edits are instant self-edit.

---

> **Coverage:** §8.1–§8.15 now document the full component library + every designed feature
> (E1–E8, E10, E11). E9 Migration is back-end (no UI). Canvas is organized into two platform boards
> (`nNYxY` web · `yPwPD` mobile) with per-role lanes and POV lines.
