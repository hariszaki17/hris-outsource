# Screen Generation Plan & Tracker — hris-outsource (web)

> The single, resumable plan any session uses to generate **every screen** from the design
> ([`docs/design/brainstorm.pen`](../design/brainstorm.pen)) into `frontend/apps/web`.
> It is also the **progress tracker** — check the boxes as screens land so the next session
> knows exactly what remains.
>
> **Status:** active · created 2026-06-03. **Stack/rules:** [WEB-STACK.md](WEB-STACK.md) ·
> [ENGINEERING.md](ENGINEERING.md). **Design inventory source:** the design audit
> ([audit/COMPLETION-REPORT.md](../design/audit/COMPLETION-REPORT.md), `SUMMARY.md`) —
> ~126 screens + ~30 reusable masters across **9 product epics** (E9 is script-only, no UI).

---

## 0. PRIME RULE — read before generating anything (ENGINEERING.md §6 G0)

**Build from the `.pen`, never from assumptions. The design file is the visual contract.**
A screen built without opening its `.pen` frame is a process violation, even if it "looks fine."
(This rule exists because the login screen was first built as a centered card when the `.pen`
defines a split-screen — see ENGINEERING.md G0.)

Per screen, in order:
1. `get_editor_state(include_schema: true)` — **once per session** (lists screens + `comp/*`).
2. `batch_get` the epic's surface container (ids in §4), then the **specific screen frame and
   all its state variants** (`… — Gagal`, `… — Terkunci`, empty/loading/error), `readDepth` 4–5.
3. `batch_get` the `comp/*` instances the frame uses (resolve to `packages/ui` components).
4. `get_screenshot` the frame for visual fidelity (sparingly — flagship/ambiguous screens).
5. Build to match **layout, structure, copy (Bahasa), tokens, and every state variant**.
6. Record the resolved frame id(s) in this tracker and check the box.

`.pen` is encrypted — **only** the Pencil MCP tools (never Read/Grep/Edit). Token-efficient
workflow rules: CLAUDE.md "Token-efficient `.pen` workflow".

---

## 1. Definition of Done (per screen)

A screen item may be checked **only when all hold**:

- [ ] Matches the `.pen` frame: layout, spacing, typography, copy, and **every state variant**
      present in the design (default · loading/skeleton · empty · filtered-zero · error/retry ·
      no-permission · saving) — the "no dead-flow" rule (ENGINEERING.md B2 / DESIGN-SYSTEM §6).
- [ ] Composed from `packages/ui` (atoms/molecules) + tokens only — **no raw hex**, no one-off
      restyles (G1–G4). Missing primitive → add to `packages/ui` first (§3).
- [ ] Data via the **generated** TanStack Query hooks (`@swp/api-client/*`); errors through the
      `classifyError` mapper; field errors → RHF; mutations carry idempotency keys (B1/C3).
- [ ] All copy via i18n (Bahasa default); all dates via the `Asia/Jakarta` layer (E4).
- [ ] Role/scope gating from the `x-rbac` permission map; client RBAC is defense-in-depth (C1).
- [ ] `pnpm typecheck && pnpm lint && pnpm test` green; route renders (verify with the dev
      server / a screenshot for non-trivial screens).
- [ ] Commit cites the `F#`/`BR-#`/`C-#` and the `.pen` frame id (A1/G0).

---

## 2. How to use this tracker

- Work **top-down**: Phase 0 (components) → Phase 1 (shell) → Phase 2 epics in dependency order
  (E1→E2→E3→E4→E5→E6→E7→E8→E10). Screens depend on the components and shell above them.
- Pick the **first unchecked item**, do the DoD, check it, and fill its `frame:` id.
- **Reconcile, don't trust blindly:** screen lists below are derived from the design audit
  (feature-accurate). On entering an epic, `batch_get` its container and **add any screen the
  list missed** (check the epic's "reconciled against live `.pen`" box). The `.pen` wins.
- One epic (or one design-system section) per session keeps `.pen` payloads small (CLAUDE.md).
- Status markers in headings: 🔲 not started · 🟡 in progress · ✅ done.

Legend per row: `surface` (web/mobile) · `roles` · `→ target path` · `comp:` deps · `frame:` `.pen` id.

---

## 3. Phase 0 — Design-system component library (`packages/ui`) 🟡

The 30 `comp/*` masters in the `.pen` map 1:1 to `packages/ui` (G4). Screens below assume these
exist. **Build/finish these before the screens that use them.**

Built so far (✅): Button, Input, Checkbox, StatusBadge, IdChip, DateText, FormField/FormSection,
StateView (loading/empty/error/no-permission), Banner, **Avatar, Sidebar, Topbar, Toast (+provider),
Skeleton, EmptyState, Modal/ConfirmDialog** *(Phase-0 chrome & feedback batch, 2026-06-03)*,
**SearchField, FilterSelect, Toggle, DataTable+CursorPagination, StatCard, StatusBadge `dot`,
SettingsSubnav, AuditTrail (Viewer/Inline/Drawer)** *(Phase-0 data & form batch, 2026-06-03)*,
**Drawer (generic right-sheet: Drawer/Header/Body/Footer)** *(E1 batch — used by Edit-User + Audit-detail, reused E3–E8)*.
Remaining Phase-0 (deferred): Export-modal family + Notif cards → E10; Pickers → E2/E3 forms.

> **MSW action-path fix (E1, 2026-06-03):** `{id}:action` endpoints generated unparseable MSW paths
> (`:userId:deactivate`). Fixed once for ALL epics via a post-gen step
> (`packages/api-client/scripts/patch-msw-action-paths.mjs`, wired into `pnpm gen`) that rewrites
> action-colon paths to anchored RegExps; all action handlers re-included in `mocks.ts`. Also
> regenerated `public/mockServiceWorker.js` to match msw 2.7.0 (was 2.14.6 → 0 page errors).

Remaining masters → components:

- [x] **Sidebar** — `comp/Sidebar` `iCqTB` → `packages/ui/src/molecules/sidebar.tsx`. Dark nav, active/scope states.
      Compound (`Sidebar`/`SidebarBrand`/`SidebarSectionLabel`/`SidebarNavItem`(asChild→Link)/`SidebarSpacer`/`SidebarFooter`), data-driven.
- [x] **Topbar** — `comp/Topbar` `caFkE` → `packages/ui/src/molecules/topbar.tsx`. `Topbar`+`Breadcrumb`+`TopbarSearch`+`TopbarIconButton`+`TopbarUser`.
- [x] **Avatar** — `comp/Avatar` `YVANc` → `packages/ui/src/molecules/avatar.tsx`. brand/neutral tone · rounded/circle · size prop.
- [x] **StatCard** — `comp/StatCard` `lmwet` → `molecules/stat-card.tsx` (tone-driven icon chip). *(see data/form batch below)*
- [x] **StatusPill** — `comp/StatusPill` `qxONU` reconciled into `StatusBadge` via a `dot` prop (G3 — one canonical status concept).
- [x] **Toast** family — `Toast` `PtJHa` master + `ToastSuccess` `ofb0U` · `ToastError` `zaisr` ·
      `ToastWarn` `d8u3Q` · `ToastInfo` `onGI4` · `ToastQueued` `lC1k8` → `toast.tsx`: one `Toast`+tone prop, plus
      `ToastProvider`/`useToast`/`Toaster` (context+timers, zero deps). Wired into app `providers.tsx`.
- [x] **Skeleton** family — `SkeletonLine` `jcW4k` · `SkeletonAvatar` `e3rdpj` · `SkeletonCard`
      `NmWCA` · `SkeletonTableRow` `PRMOL` → `skeleton.tsx`: canonical `Skeleton`(+circle) · `SkeletonCard` · `SkeletonTableRow`.
- [x] **Empty** family — `EmptyState` `WTymt` + `EmptyFilteredZero` `BNr4w` · `EmptyFresh` `mrACi` ·
      `EmptyNoPermission` `MRbzz` · `EmptySessionExpired` `iwcgE` → `empty-state.tsx`: one `EmptyState`+`variant`.
      *(Reconcile: supersedes `StateView`'s plain empty/no-permission for real screens; StateView keeps loading/error until list screens migrate.)*
- [x] **Modal family** — `ModalReject` `EnabP` · `ModalBulkApprove` `r4KZl5` · `ModalDestructive`
      `V4LG8` · `ModalDiscardChanges` `z0kH0b` → `modal.tsx`: one `Modal`(+`ModalHeader`/`Body`/`Footer`) + `ConfirmDialog`
      (Radix Dialog: focus-trap/ESC/a11y). All 4 .pen modals = ConfirmDialog usages.
- [ ] **Export modal** family — `ModalExportStep1Format` `PN3mn` · `Step2Progress` `Q3dllJ` ·
      `Step3Success` `lJ2iU` · `ModalExportError` `zOpT1` (multi-step; E10 owns, many consumers).
- [ ] **Notif cards** — `NotifCardUnread` `CQBqd` · `NotifCardRead` `zTbmw` (one card + read state). *(deferred → E10)*
- [x] **AuditTrail** — `AuditTrailViewer` `jzBi0` · `AuditTrailDrawer` `BUAHW` · `AuditTrailInline` `qtz6q` →
      `audit-trail.tsx`: data-driven (`AuditEntry[]`); Drawer on Radix Dialog (right sheet — generic `Drawer` extraction is a follow-up).
- [ ] **Pickers** — `PickerEmployee` `ZOZ5x` · `PickerClientCompany` `GpyLu` · `PickerServiceLine`
      `vkwQo` · `PickerPosition` `Nz6iR` · `PickerShiftLeader` `fg4kI` (cross-epic FK pickers). *(deferred → E2/E3 forms)*
- [x] **Fields** — `SearchField` `vJBJZ` · `FilterSelect` `t60nEC` (native select) · `Toggle` `Uma0O` (role=switch)
      → `primitives/{search-field,filter-select,toggle}.tsx`. *(`TextField` `nVepR` reconciled to existing `FormField`+`Input`; `Checkbox` already exists.)*
- [x] **Button variants** — `BtnPrimary` `Y7IwcG`/`BtnSecondary` `TR9pR`/`BtnGhost` `AEl1Q`/`BtnDanger` `d5HQSI`
      map 1:1 to existing `Button` `variant` prop (primary/secondary/ghost/destructive). Verified — no new component (G3).
- [x] **DataTable** — derived from `E2 · Karyawan — Daftar` `WElYh` → `molecules/{data-table,cursor-pagination}.tsx`:
      generic column-config `DataTable<T>` (loading/empty states, bulk-select, row-kebab slot) + `CursorPagination` (D1 cursor,
      not offset). *Virtualization deferred* — API is virtualization-ready (note in file).
- [x] **SettingsSubnav** — `comp/SettingsSubnav` `WhMQv` → `molecules/settings-subnav.tsx` (`SettingsSubnav`+`SettingsSubnavItem`, asChild).

---

## 4. Phase 1 — App shell, routing, providers ✅

- [x] Providers (QueryClient + i18n + ToastProvider) · TanStack Router + auth guard · `classifyError` mapper · MSW.
- [x] **Data foundation** (fixed 2026-06-03, first data screen): hand-authored `@swp/api-client/{e1,e6}` barrels
      (Orval `tags-split` emits no root barrel) · **fetch mutator returns `{data,status,headers}`** (matches Orval's
      fetch-client contract — was body-only) · MSW service worker initialized + `VITE_ENABLE_MSW=true` (.env.local).
      *(Known: user `{id}:action` MSW paths can't be parsed by path-to-regexp; those handlers excluded in `mocks.ts`.)*
- [x] **Login (web)** — F1.1 · all · `→ features/auth/login-screen.tsx` · comp: Button,Input,Checkbox,
      FormField,Banner · frame: `lKRjr` (+ Gagal `JRq3Z`). *(default state + failed-login done; see E1 for remaining login variants.)*
- [x] **App shell** — real `comp/Sidebar` `iCqTB` + `comp/Topbar` `caFkE`; role-aware nav. `→ app/shell.tsx`.
      Composes the Phase-0 Sidebar/Topbar; nav filtered by `SessionUser.role` via interim role map
      (`app/nav.ts` + `@swp/shared` `Role`; will be replaced by the generated `x-rbac` map, A2). Adds
      `useCurrentUser` hook, `auth.login()`/`SessionUser`, `UserMenu` organism (Settings + Keluar),
      breadcrumb from active route. `nav.test.ts` asserts the gating. *(Phase-1 shell batch, 2026-06-03.)*

**Epic surface containers** (open these first per epic, via `batch_get`):

| Epic | `.pen` web container | `.pen` mobile container |
|---|---|---|
| E1 Foundations | `teUIY` | `tQ8ei` |
| E2 Karyawan | `G0D87V` | `Hbj6C` |
| E3 Penempatan | `j2giE` | `znQiw` |
| E4 Jadwal Shift | `mi0kN` | `CbiZ9` |
| E5 Kehadiran | `W83QJ` | `h8QJ0r` |
| E6 Cuti | `Anidb` | `lF575` |
| E7 Lembur | `BnEnb` | `EG3xg` |
| E8 Payroll | `OaAdZ` | `v8XYAl` |
| E10 Laporan & Notifikasi | `JifD6` | `WFUVA` |

---

## 5. Phase 2 — Web screens by epic (dependency order)

> Target root: `frontend/apps/web/src/features/<epic>/`. Mobile screens (React Native) are in
> Phase 3 — deferred until `apps/mobile` is scaffolded. Each epic: tick "reconciled" after you
> diff the list against its live `.pen` container and add any missing frame.

### E1 — Foundations ✅  · web container `teUIY`
- [x] Reconciled against live `.pen` *(24 web frames; auth set built, admin console + global states remain)*
- [x] Login — default + failed (see Phase 1) · frames `lKRjr`,`JRq3Z`
- [x] Login — **Terkunci sementara** (locked) · `→ features/auth/login-screen.tsx` (search param `?error=locked`) · comp: Banner(icon) · frame `N2IdlJ`
- [x] Login — **Akun nonaktif** (disabled) · login-screen `?error=disabled` · comp: Banner(icon=shield-x) · frame `QVifb`
- [x] Forgot password (web) — form + **Tautan terkirim** state · `→ features/auth/forgot-password-screen.tsx` · comp: AuthLayout,FormField,Button · frames `etsMo`,`vz7oI`
- [x] Reset password (web) — form + live req-checklist + **Berhasil** state · `→ features/auth/reset-password-screen.tsx` · frames `N1c1X`,`b8BGef`
- [x] **Pengguna & Peran (users) — list** + filters + row-kebab + states · `→ features/e1-foundations/users-screen.tsx` · frame `kHNWT`
      First **data-driven** screen: `useListUsers` (generated) over MSW · DataTable · filters in typed URL search params (D1) · cursor pagination · loading/empty/filtered/error/no-permission states.
- [x] **Settings shell + layout** — `SettingsSubnav` rail + `<Outlet>` · `→ features/e1-foundations/settings-layout.tsx` · routes `/settings`(Ringkasan)·`/users`·`/audit-log`·`/general`.
- [x] Tambah Pengguna (create user) — modal/form + validation · `→ features/e1-foundations/user-overlays.tsx` · frame `iXs2R`/`FGkC2`
- [x] Ubah Peran (modal) + Edit user (drawer) + row-kebab + send-reset + (de)activate confirms · user-overlays.tsx · frames `BWWxD`/`y4qyuS`, `K9DQR`/`xmWHa`, `Zjzvo`, `oXZNQ`, `cACO9`
- [x] Audit log — list (F1.3) + **detail drawer** (before→after diff) · `→ features/e1-foundations/audit-log-screen.tsx`,`audit-detail-drawer.tsx` · frames `rtJRB`/`N3EBSr`, `Zxv9P`/`x5wrt`
- [x] Pengaturan — Ringkasan hub + General (PlatformSettings) · `→ settings-overview-screen.tsx`,`settings-general-screen.tsx` · frames `fVinX`/`E7WOwh`, `m3sWh`/`tch6k`
- [x] Session-expired re-auth state · `→ features/e1-foundations/global-states.tsx` (`/session-expired`) · comp: EmptySessionExpired `iwcgE`
- [x] No-permission / 403 state · global-states.tsx (`/forbidden`) + per-screen inline · comp: EmptyNoPermission `MRbzz`,`TqMQ6`

### E2 — Identity / Karyawan & Master Data ✅  · web container `G0D87V`
- [x] Reconciled against live `.pen` *(23 web frames; Admin POV + SL scoped variants)*
- [x] **Pickers** (comp/* → generic `Combobox` in `@swp/ui` + 5 domain pickers) · `ZOZ5x`,`GpyLu`,`vkwQo`,`Nz6iR`,`fg4kI` → `features/e2-identity/pickers/*`
- [x] Daftar Karyawan — list + stat cards + tabs + filters + row-kebab + **SL scoped** · `→ employees-screen.tsx` · frames `WElYh`,`n3wi1w`
- [x] Karyawan Detail — Profil + cross-epic tabs (Penempatan/Kehadiran/Cuti&Lembur deep-links) + SL read-only · `→ employee-detail-screen.tsx` · frames `JBjBb`,`rtKzk`
- [x] Tambah/Edit Karyawan — form (RHF + hand-zod) + overlays · `→ employee-form.tsx`,`employee-overlays.tsx` · frame `h6bDz`,`tNMfN`
- [x] HR change-request queue + detail drawer + reject modal · `→ change-requests-screen.tsx`,`change-request-overlays.tsx` · frame `Ckteo`
- [x] **F2.2 Employment Agreement** — list · detail · create (PKWT/PKWTT) · renew · close · `→ agreements-screen.tsx`,`agreement-detail-screen.tsx`,`agreement-form.tsx` · frames `mS8rP`,`Cu0qg`,`gxqjg`
- [x] **F2.3 Client Company** — list · detail + **geofence_radius_m editor + map placeholder** + geofence-disabled banner (D11) · create · `→ client-companies-screen.tsx`,`client-company-detail-screen.tsx`,`client-company-form.tsx` · frames `qIpsj`,`OmuQT`,`ZmJnZ`,`oYgYe`
- [x] **F2.4 Service Lines + Positions** — list · detail (nested positions) · modals · `→ service-lines-screen.tsx`,`service-line-detail-screen.tsx` · frames `vV79c`,`I8WeKy`,`IwKfo`,`hb7vL`
- [x] **F2.5 Operational Master Data** — hub + Leave Types · Attendance Codes (color+flags) · Overtime Rules (30-min min) CRUD + modals · `→ master-data-hub-screen.tsx`,`leave-types-screen.tsx`,`attendance-codes-screen.tsx`,`overtime-rules-screen.tsx` · frames `f8mBr`,`HII8C`,`R5xoi`,`SnXpE`,`rMNJT`,`u8eXaW`,`JYmgi`

### E3 — Placement / Penempatan 🔲  · web container `j2giE`
- [ ] Reconciled against live `.pen`
- [ ] Company roster / placement list + **expiring-soon** filtered list · hr_admin/shift_leader(scope)
- [ ] Placement Detail — terminal variants: Active · Scheduled · Expiring · Ended · Terminated · **Resigned** (`MS2fi`) · Superseded · comp: StatusBadge,AuditTrailInline
- [ ] Create Placement — form + error variants (INV-1) · comp: PickerEmployee,PickerClientCompany,PickerServiceLine
- [ ] Transfer modal (F3.3) · Renew modal · End/Terminate confirm+reason · **Resign modal** (`ModalResign`)
- [ ] Shift-Leader assignment — picker (`PickerShiftLeader`) + Assign / Reassign + **INV-2/3/4** states (`ModalAssign`,`ModalReassign`)
- [ ] Row-kebab actions

### E4 — Shift Scheduling / Jadwal 🔲  · web container `mi0kN`
- [ ] Reconciled against live `.pen`
- [ ] Shift master catalog — list + Tambah/Edit Shift + Deactivate
- [ ] Schedule grid — week, by company (F4.2) · shift_leader(scope)/hr_admin
- [ ] **Shift-picker popover** (core F4.2) + cell-edit/clear menu
- [ ] Conflict toasts — over-leave · double-shift · beyond-placement · out-of-scope · coverage-warn
- [ ] Bulk apply-to-range · Auto-publish toast

### E5 — Attendance / Kehadiran 🔲  · web container `W83QJ`
- [ ] Reconciled against live `.pen`
- [ ] Attendance dashboard (F5.5) — HR (cross-company) + Shift Leader (own-company) · comp: StatCard,StatusBadge
- [ ] Verification queue + **detail** (F5.3) + **bulk-verify** (ModalBulkApprove) + **reject modal** (ModalReject)
- [ ] HR escalation badges + filter (leaders' own records)
- [ ] **F5.4 Corrections** — leader/HR queue · correction detail · reject · (agent tracker is mobile)

### E6 — Leave / Cuti 🔲  · web container `Anidb`
- [ ] Reconciled against live `.pen`
- [ ] Leave quotas (HR) — list + adjust modal + bulk-grant (preview/apply) (F6.1)
- [ ] HR leave detail + queues (F6.3) · **SL Leave Detail (L1 variant)** · no-leader timeline variant
- [ ] Reject modal + approve/reject toasts · quota-exceeded + missing-doc errors · balance-recheck fail
- [ ] Leave calendars (HR/team) + approved+pending toggle (F6.5, D6)
- [ ] Cancel / shorten approved leave

### E7 — Overtime / Lembur 🔲  · web container `BnEnb`
- [ ] Reconciled against live `.pen`
- [ ] OT records / Rekap (F7.4) + export entry
- [ ] OT approval queue (F7.3) + **bulk-approve selection** + reject modal
- [ ] **OT detail (web)** — the central decision UI · auto-detect confirm result · worked-without-request flag · <30m skipped · holiday-beats-rest-day · withdraw
- [ ] Create/Edit OT Rule (F7.1) · Add/Edit Holiday (HR-maintained calendar)

### E8 — Payroll (read-only) 🔲  · web container `OaAdZ`
- [ ] Reconciled against live `.pen`
- [ ] HR payroll archive — list (F8.2) + payslip detail + "FINAL · Read-only" pill
- [ ] decrypt-fail detail variant · HR audit-note drawer · empty states · access-denied
- [ ] Export (Excel-only v1, D5) — via Phase-0 Export modal family

### E10 — Reporting & Notifications 🔲  · web container `JifD6`
- [ ] Reconciled against live `.pen`
- [ ] Dashboards (F10.2) — HR (`ETi5H`) · **Super Admin variant** (`DhzyL`, same-as-HR + label, D1) · SL
- [ ] Notification center (F10.1) + empty + **mark-read transition** + stale-link state
- [ ] Attendance/billable report (F10.3) + empty + pending-records callout
- [ ] **Export framework** (owner) — format → progress → success → error (Phase-0 modal family)
- [ ] Approval inbox + empty state

---

## 6. Phase 3 — Mobile (React Native, `apps/mobile`) — DEFERRED 🔲

> Mobile is a separate surface ([WEB-STACK §3](WEB-STACK.md)); `apps/mobile` is a placeholder.
> **Do not start until `apps/mobile` is scaffolded.** Listed here for completeness so nothing is
> lost. Roles: agent (all), shift_leader (D7 — leader mobile surfaces are designed and in-scope).

- [ ] E1 mobile: Login (`Y09E0`) + Gagal (`XouNm`) + Terkunci (`PiWlc`) + Akun nonaktif (`YG9jg`) · forgot/reset · profile/Pengaturan
- [ ] E2 mobile: agent profile view/edit (phone/address/bank → change request)
- [ ] E4 mobile: agent week schedule (F4.3) + shift reminders
- [ ] E5 mobile: clock-in/out (F5.1) + variants (clock-out · out-of-geofence · unscheduled · GPS-unavailable) · agent attendance history/detail · **SL verification queue + detail**
- [ ] E6 mobile: agent leave request + status (F6.2) · **SL leave queue + detail**
- [ ] E7 mobile: agent OT request/confirm (F7.2) + OT detail bottom-sheet · **SL OT approval**
- [ ] E8 mobile: agent payslip history + summary (F8.1)
- [ ] E10 mobile: agent Beranda/dashboard + empty · **SL dashboard + notifications + combined inbox** · SLMobileNav (`fdVo7`)

---

## 7. Progress summary (update as you go)

| Phase / Epic | Screens (approx) | Done |
|---|---|---|
| Phase 0 — components | ~18 groups | 25 of ~27 masters (chrome+feedback + data/form + **Drawer** done; remaining: Export modal, Notif cards, Pickers — deferred to their epics) |
| Phase 1 — shell + login | 3 | 3 (providers, login, **app shell**) ✅ |
| E1 Foundations (web) | 12 | 12 ✅ (auth set + Users CRUD/overlays + Audit list+drawer + Settings hub/general + global states) |
| E2 Karyawan (web) | ~9 features | 9 ✅ (employees+SL · detail · form · change-req queue · agreements · client-companies+geofence · service-lines+positions · master-data×3 · **Pickers/Combobox**) |
| E3 Penempatan (web) | ~6 | 0 |
| E4 Jadwal (web) | ~5 | 0 |
| E5 Kehadiran (web) | ~4 | 0 |
| E6 Cuti (web) | ~5 | 0 |
| E7 Lembur (web) | ~4 | 0 |
| E8 Payroll (web) | ~3 | 0 |
| E10 Reporting (web) | ~5 | 0 |
| Phase 3 — mobile | ~8 epics | 0 (deferred) |

> Counts are feature-level approximations from the audit; the real number includes per-state
> variants. The live `.pen` is the source of truth — reconcile per epic (§5).
