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
Phase-0 complete: Export-modal family + Notif cards built in E10; Pickers built in E2/E3 forms.

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
- [x] **Export modal** family — `ModalExportStep1Format` `PN3mn` · `Step2Progress` `Q3dllJ` ·
      `Step3Success` `lJ2iU` · `ModalExportError` `zOpT1` → `packages/ui/src/molecules/export-modal.tsx`:
      one `ExportModal`(+`step` prop: format/progress/success/error) + stepper, on Radix Dialog. *(E10 batch — XLSX-only D5; Bahasa copy baked w/ `labels` override — externalise-to-i18n is a follow-up.)*
- [x] **Notif cards** — `NotifCardUnread` `CQBqd` · `NotifCardRead` `zTbmw` → `packages/ui/src/molecules/notif-card.tsx`:
      one `NotifCard`(+`unread` prop), renders `<button>` when `onClick` given. *(E10 batch.)*
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
- [x] **F2.3 Client Company** — list (row action = Aktifkan/Nonaktifkan only, **no row kebab**) · detail (**Profil** tab = statutory/billing + `leader_scope`; **Lokasi & Site** tab owns geofence_radius_m editor + map placeholder + geofence-disabled banner D11 — Profil no longer duplicates sites/geofence) · create + **full-page edit from detail** (`/client-companies/$id/edit`; **no edit drawer**) · `→ client-companies-screen.tsx`,`client-company-detail-screen.tsx`,`client-company-form.tsx` · frames `qIpsj`,`OmuQT`,`ZmJnZ`,`oYgYe` *(EditClientCompanyDrawer removed 2026-06-07, EPICS §8)*
- [x] **F2.4 Service Lines + Positions** — list (Edit **routes to detail page**) · detail = consolidated maintenance (rename line + add/update/remove positions; no rename-only modal) · `→ service-lines-screen.tsx`,`service-line-detail-screen.tsx` · frames `vV79c`,`I8WeKy`,`IwKfo`,`hb7vL` *(consolidated to detail 2026-06-07, EPICS §8)*
- [x] **F2.5 Operational Master Data** — hub + Leave Types · Attendance Codes (color+flags) · Overtime Rules (30-min min) CRUD + modals · `→ master-data-hub-screen.tsx`,`leave-types-screen.tsx`,`attendance-codes-screen.tsx`,`overtime-rules-screen.tsx` · frames `f8mBr`,`HII8C`,`R5xoi`,`SnXpE`,`rMNJT`,`u8eXaW`,`JYmgi`

### E3 — Placement / Penempatan ✅  · web container `j2giE`
- [x] Reconciled against live `.pen` *(5 web frames: Admin POV + SL scoped roster)*
- [x] Placement list + **expiring-soon** filter + Company roster (HR + SL read-only) · `→ features/e3-placement/placements-screen.tsx`,`company-roster-screen.tsx` · frames `C2SSLA`,`nLN4d`,`o5Txgg`
- [x] Placement Detail — all 9 lifecycle/terminal variants (PENDING_START/ACTIVE/EXTENDED/EXPIRING/ENDED/TRANSFERRED/TERMINATED/RESIGNED/SUPERSEDED) + AuditTrailInline · `→ placement-detail-screen.tsx` · frame `pFR79`
- [x] Create Placement — form + INV-1 conflict variant + outside-contract warning · `→ placement-form.tsx` (+ `agreement-picker.tsx`) · frame `g3OzZz`
- [x] Transfer · Renew · End · Terminate (type-to-confirm) · Resign modals · `→ placement-overlays.tsx`
- [x] Shift-Leader assignment — Assign/Replace/End + **INV-2/3/4** conflict states (`ShiftLeaderPicker`) · in `placement-overlays.tsx`
- [x] Row-kebab actions / detail deep-links

### E4 — Shift Scheduling / Jadwal ✅  · web container `mi0kN`
- [x] Reconciled against live `.pen` *(3 web frames: master shift + add modal + weekly grid)*
- [x] Shift master catalog — list + Tambah/Edit Shift + Deactivate/Reactivate · `→ features/e4-scheduling/shift-masters-screen.tsx` · frames `O5JgF`,`Mn9ux`
- [x] Schedule grid — week, by company (F4.2) · shift_leader(scope)/hr_admin · `→ schedule-grid-screen.tsx` · frame `Rubba`
- [x] **Shift-picker popover** (core F4.2) + cell day-off/clear menu · `→ schedule-overlays.tsx`
- [x] Conflict toasts — over-leave · double-shift · beyond-placement · out-of-scope · coverage-warn (via `useCheckScheduleConflicts`)
- [x] Bulk apply-to-range (preview/apply) · Auto-publish toast

### E5 — Attendance / Kehadiran ✅  · web container `W83QJ`
- [x] Reconciled against live `.pen` *(8 web frames: HR + SL POVs + corrections)*
- [x] Attendance dashboard (F5.5) — HR (cross-company) + Shift Leader (own-company) · `→ features/e5-attendance/attendance-dashboard-screen.tsx` · frames `sZCLW`,`V2QL7`
- [x] Verification queue + **detail** (F5.3) + **bulk-verify** + **reject modal** + escalation badges/filter · `→ attendance-verification-screen.tsx`,`attendance-detail-screen.tsx` · frames `UEG2J`,`MsXnm`,`VY894`,`RZPQz`
- [x] **F5.4 Corrections** — HR queue + correction detail drawer (before→after diff) + approve/reject · `→ corrections-screen.tsx`,`correction-overlays.tsx` · frames `QfamL`,`sSKtK`

### E6 — Leave / Cuti ✅  · web container `Anidb`
- [x] Reconciled against live `.pen` *(11 web frames: HR L2 + SL L1 + corrections)*
- [x] Leave quotas (HR) — list + adjust modal + bulk-grant (preview/apply) (F6.1) · `→ features/e6-leave/leave-quotas-screen.tsx` · frames `P6HZ7E`,`CGCnL`,`W2zYM`
- [x] HR leave detail + queues (F6.3) + **SL L1 variant** + no-leader + LA-5 balance-override variants + reject modal · `→ leave-approvals-screen.tsx`,`leave-detail-screen.tsx`,`leave-overlays.tsx` · frames `yho5i`,`qb0S0`,`DJrBn`,`eHXWF`,`ZlnfW`,`Hzbbv`
- [x] Leave calendars (HR/team) + approved+pending toggle (F6.5, D6) · `→ leave-calendar-screen.tsx` · frames `s5niW`,`YvYcr`
- [~] Cancel / shorten approved leave — hooks wired (`useCancelApprovedLeaveRequest`,`useShortenLeaveRequest`); surfaced from detail actions *(deferred: dedicated wave-3.4 modals)*
- [x] **EN i18n pass done** (final review): leave/leaveQuotas/leaveCalendar `en.ts` translated to English (was reusing Bahasa).
- [x] **Screenshots done** (final review): captured in `e2e/e7-e10-screens.spec.ts` (`01-leave-approvals.png`).

### E7 — Overtime / Lembur ✅  · web container `BnEnb`
- [x] Reconciled against live `.pen` *(5 web frames: HR rekap/approvals/detail/rules + SL L1 + overlays showcase `YGLK3`, withdraw `STI8j`)*
- [x] OT records / Rekap (F7.4) + export entry · `→ features/e7-overtime/overtime-records-screen.tsx` · frame `JEmCk` · route `/overtime/rekap`
- [x] OT approval queue (F7.3) HR L2 + **SL L1** (role-branched) + **bulk-approve/bulk-reject selection** + reject modal · `→ overtime-approvals-screen.tsx`,`overtime-queue-overlays.tsx` · frames `H1eBN`,`Vh2P9` · route `/overtime`
- [x] **OT detail (web)** — central decision UI · calc block (worked/counted/min-threshold) · <30m **skipped_too_short** · tier breakdown w/ **supersedes** (holiday-beats-restday) · **worked-without-request** flag · approve L1/HR-final · reject · withdraw · auto-detect confirm · terminal read-only states · `→ overtime-detail-screen.tsx`,`overtime-detail-overlays.tsx` · frames `uG6mQ`,`YGLK3`,`STI8j` · route `/overtime/$overtimeId`
- [x] OT Rule reference (tier-expanded, links to E2 `/master-data/overtime-rules` for CRUD) + **Add/Edit/Delete Holiday** (HR calendar, in-use guard) · `→ overtime-rules-screen.tsx`,`holiday-overlays.tsx` · frame `vd4na` · route `/overtime/aturan`
- Shared: `overtime-shared.tsx` (status/tier/source/holiday tones+keys). New primitive: `Checkbox` now `forwardRef` (tri-state select-all indeterminate). i18n: full `overtime` namespace (id+en) incl. nested `common`/`errors` (cross-ns keys don't resolve under a sub-namespace — see carry-over).
- [~] Screenshots deferred to the consolidated end-of-run pass.

### E8 — Payroll (read-only) ✅  · web container `OaAdZ`
- [x] Reconciled against live `.pen` *(6 frames: archive · detail · decrypt-fail · audit-note drawer · export flow · empty/access-denied state-cards)*
- [x] HR payroll archive — list (F8.2) + filters (period/year/employee/status) + **"FINAL · Read-only" pill** + MISSING_PAYROLL_HISTORY empty + access-denied · `→ features/e8-payroll/payslip-archive-screen.tsx` · frame `jBgLn` · route `/payroll`
- [x] Payslip detail + component breakdown (earnings/deductions/benefits) + IDR money + **decrypt-fail variant** (null money, "Perlu review" banner) · `→ payslip-detail-screen.tsx` · frames `JaScP`,`q8JxjZ` · route `/payroll/$payslipId`
- [x] **HR audit-note Drawer** (append-only; immutable banner; list/empty/error/saving) · `→ audit-note-drawer.tsx` · frame `BDHMZ`
- [x] Export (Excel-only v1, D5) — `PayrollExportButton` + job-state card (QUEUED/RUNNING/DONE/FAILED); detail "Ekspor" surfaces a queued toast. **Full multi-step Export-modal family deferred to E10** (owns `i1uLk`/`PN3mn`…) · `→ payroll-export.tsx`
- [x] Reusable empty/access-denied state blocks · `→ payroll-states.tsx` · frame `dRfK9`
- Shared: `payroll-shared.tsx` (status tone+key, IDR `formatMoney`, `formatPeriod`). Route glue: `payslip-detail-route.tsx` (drawer + export toast). i18n: full `payroll` namespace (id+en) incl. nested `common`/`errors`/`status`/`month`. Detail gained `onAddNote` prop (opens drawer). nav: `/payroll` (ADMIN).
- [~] Screenshots deferred to the consolidated end-of-run pass.

### E10 — Reporting & Notifications ✅  · web container `JifD6`
- [x] Reconciled against live `.pen` *(HR row: dashboard/billable/export-modal/notif-center/super-admin; SL dashboard; export + notif + dashboard-empty showcases)*
- [x] Dashboards (F10.2) — role-branched `useGetMyDashboard` union: HR (`ETi5H`) · **Super Admin** (`DhzyL`, same data + label, D1) · SL team (`RiSPW`) · agent-fallback · `→ features/e10-reporting/dashboard-screen.tsx` · route `/` (replaces placeholder)
- [x] **Approval-inbox panel** ("Perlu Tindakan") + empty (`biFs5`) + filtered-zero (`elJj3`) · `→ approval-inbox-panel.tsx`
- [x] Notification center (F10.1) + filters + **mark-read transition** (optimistic + toast) + mark-all + empty (`P2CO7C`) + stale-link note · `→ notifications-screen.tsx` (uses `NotifCard`) · frames `i0qW8`,`R0d1wC` · route `/notifications`
- [x] Attendance/billable report (F10.3) + summary KPIs + per-row table + **pending-records callout** (verified-only) + export entry · `→ billable-report-screen.tsx` · frame `EF8AZ` · route `/reports`
- [x] **Export framework** (owner) — `ExportModal` (format → progress → success → error) driven by `useExportFlow` (createExport + poll getExport via refetchInterval → step) · `→ use-export-flow.ts` · frame `FJ6hX`
- Shared: `e10-shared.tsx` (notif/inbox icon maps, export status tone/step). i18n: `dashboard`/`notifications`/`report` namespaces (id+en, nested common/errors). nav: `/notifications` (ALL_WEB). New primitives: `NotifCard`, `ExportModal` (Phase-0). Deep-links use a typed-navigate cast (route-table lookup is a follow-up).
- [~] Screenshots deferred to the consolidated end-of-run pass.

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
| Phase 0 — components | ~18 groups | 27 of ~27 masters ✅ (chrome+feedback + data/form + Drawer + **ExportModal + NotifCard** + Pickers — all built) |
| Phase 1 — shell + login | 3 | 3 (providers, login, **app shell**) ✅ |
| E1 Foundations (web) | 12 | 12 ✅ (auth set + Users CRUD/overlays + Audit list+drawer + Settings hub/general + global states) |
| E2 Karyawan (web) | ~9 features | 9 ✅ (employees+SL · detail · form · change-req queue · agreements · client-companies+geofence · service-lines+positions · master-data×3 · **Pickers/Combobox**) |
| E3 Penempatan (web) | ~6 | 6 ✅ (list+roster · detail w/ 9 lifecycle states · create+INV-1 · transfer/renew/end/terminate/resign · SL assign/replace/end INV-2/3/4) |
| E4 Jadwal (web) | ~5 | 5 ✅ (shift master catalog+modal · weekly schedule grid · shift-picker popover · conflict toasts · bulk apply) |
| E5 Kehadiran (web) | ~4 | 4 ✅ (dashboard HR+SL · verification queue+detail+bulk · corrections queue+detail) |
| E6 Cuti (web) | ~5 | 5 ✅ (approvals HR-L2+SL-L1 · detail+variants · quotas+grant · calendar) |
| E7 Lembur (web) | ~4 | 5 ✅ (rekap · approvals HR-L2+SL-L1+bulk · OT decision detail · OT-rules+holiday calendar CRUD · queue/detail/holiday overlays) |
| E8 Payroll (web) | ~3 | 6 ✅ (archive list · payslip detail+decrypt-fail · audit-note drawer · export button+job card · empty/access-denied state-cards · route glue) |
| E10 Reporting (web) | ~5 | 6 ✅ (dashboards HR/Super/SL · approval-inbox panel · notification center+mark-read · billable report+callout · export framework/modal · **Phase-0 ExportModal + NotifCard**) |
| Phase 3 — mobile | ~8 epics | 0 (deferred) |

> Counts are feature-level approximations from the audit; the real number includes per-state
> variants. The live `.pen` is the source of truth — reconcile per epic (§5).
