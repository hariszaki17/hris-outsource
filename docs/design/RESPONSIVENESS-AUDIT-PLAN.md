# Responsiveness Audit & Implementation Plan — hris-outsource

> **Status:** Plan phase · **Created:** 2026-06-20 · **Author:** Engineering
> **Scope:** Web console (full responsive re-architecture) + Mobile app (360–430 width range)
> **Stack:** Tailwind v4 (web) · NativeWind v3 / React Native (mobile)
> **Principles:** Mobile-first · touch-friendly (44px min) · no dead-flow · tokens over literals

---

## 0. Audit Summary

| Surface | Total Items | 🔴 Critical | 🟡 High | 🔵 Medium | ⚪ Low | Est. Total Effort |
|---------|------------|-----------|---------|----------|--------|-------------------|
| Web — Foundation | 13 | 3 | 6 | 4 | 0 | 5–7 days |
| Web — Auth Screens | 4 | 0 | 1 | 3 | 0 | 1 day |
| Web — Feature Screens | ~52 | 0 | 15 | 28 | 9 | 12–18 days |
| Mobile — Components | 8 | 0 | 2 | 5 | 1 | 2–3 days |
| Mobile — Screens | 28 | 1 | 6 | 14 | 7 | 5–8 days |
| Cross-cutting | 3 | 0 | 1 | 2 | 0 | 1–2 days |
| **TOTAL** | **~108** | **4** | **31** | **56** | **17** | **26–39 days** |

---

## 1. Responsive Breakpoint Strategy

### 1.1 Web (Tailwind v4)

| Breakpoint | Width | Target Device | Role in this app |
|-----------|-------|---------------|------------------|
| *(default)* | < 640px | Phone (portrait) | Mobile-first base: hamburger menu, 1-col, card view |
| `sm` | ≥ 640px | Phone (landscape) / small tablet | 2-col stat cards, larger touch targets |
| `md` | ≥ 768px | Tablet (portrait) | Sidebar visible (overlay), 2-col forms |
| `lg` | ≥ 1024px | Tablet (landscape) / small desktop | Full sidebar (240px), 4-col stat cards, data table |
| `xl` | ≥ 1280px | Desktop (current design target) | All columns visible, full layout |
| `2xl` | ≥ 1536px | Large desktop | Max-width content constraint |

**Rule:** Every utility class starts from the mobile default. Desktop overrides use `lg:` prefix. Never build desktop-first and undo with `max-lg:`.

### 1.2 Mobile (React Native)

| Category | Width Range | Devices |
|----------|------------|---------|
| Small | 360–375px | iPhone SE, older Android |
| Standard | 390–414px | iPhone 14, iPhone 14 Pro, most Android |
| Large | 428–430px | iPhone 14 Pro Max, iPhone 15 Pro Max |

**Rule:** Use `flex` layouts (never fixed pixel-widths for primary containers). Use `useWindowDimensions()` for cases requiring width-dependent layout (e.g., stat tile grids, weekly schedule). Use percentage-based padding (`px-[5%]`) instead of fixed `px-6` on small devices.

---

## 2. Web App — Foundation Layer (Phase 0)

> These 13 components in `packages/ui` are the building blocks. Fix them first — every screen benefits.

### 2.1 🔴 CRITICAL — Shell & Navigation

#### GAP-W01: AppShell (`apps/web/src/app/shell.tsx`)
- **Severity:** 🔴 Critical
- **Current:** `<div className="flex h-full">` — sidebar always visible, no hamburger, no mobile layout
- **Gap:** On mobile, sidebar (240px) takes 60%+ of viewport; content area is unusable
- **Fix:**
  - Mobile default: Sidebar hidden. Topbar shows hamburger button. Sidebar opens as a slide-over drawer (Radix Dialog, `left` anchored, scrim backdrop).
  - Desktop (`lg:`): Existing flex-row layout (sidebar 240px + main).
  - Hamburger button: added to `Topbar`'s left slot via a `showMenu` prop.
  - Sidebar drawer needs its own compound component: `<MobileSidebarDrawer>` wraps `<Sidebar>` in a Radix Dialog.
- **Effort:** 1.5 days
- **Components affected:** `shell.tsx`, `sidebar.tsx`, `topbar.tsx` (new `MobileSidebarDrawer` in `packages/ui`)

#### GAP-W02: Sidebar (`packages/ui/src/molecules/sidebar.tsx`)
- **Severity:** 🔴 Critical
- **Current:** `w-60` fixed, `h-full`, no responsive variants
- **Gap:** Must support two render contexts: inline (desktop) and inside a drawer (mobile)
- **Fix:**
  - Add `variant?: 'inline' | 'drawer'` prop (default `'inline'`)
  - Inline: `w-60` (existing). Drawer: full width inside the drawer panel
  - When in drawer, add a close button at the top
  - Add `onNavigate?: () => void` callback — drawer closes after nav on mobile
- **Effort:** 0.5 day
- **Dependency:** GAP-W01

#### GAP-W03: Topbar (`packages/ui/src/molecules/topbar.tsx`)
- **Severity:** 🟡 High
- **Current:** `h-16`, flex row, no hamburger slot, breadcrumb shows full path
- **Gap:** Needs hamburger button for mobile, breadcrumb overflow on narrow screens
- **Fix:**
  - Add `showMenuButton?: boolean` + `onMenuClick?: () => void` props
  - Breadcrumb: on mobile default, show only last crumb with a back-chevron ("←" if there are parents); on `md:`, show full path
  - Topbar height: `h-14` on mobile, `h-16` on `md:` (conserves vertical space)
  - Right section: notification bell stays; user menu can collapse to avatar-only on mobile
- **Effort:** 0.5 day
- **Dependency:** GAP-W01

### 2.2 🟡 HIGH — Data Display

#### GAP-W04: DataTable (`packages/ui/src/molecules/data-table.tsx`)
- **Severity:** 🟡 High
- **Current:** Full HTML `<table>` with all columns, no responsive strategy
- **Gap:** Tables with 6+ columns overflow on mobile (attendance: name, company, clock-in, status, verification, action)
- **Fix — Card View Strategy (recommended over horizontal scroll):**
  - Default (mobile): Render as a vertical stack of cards. Each card = one row. Column 1 (primary identifier — e.g. name+avatar) becomes card title. Remaining columns become label:value pairs. Row actions at bottom.
  - `lg:`: Render as the full table.
  - Add a `priority` field to `Column<T>`: `'primary'` | `'secondary'` | `'hidden-mobile'`. Primary = card title. Secondary = card fields. Hidden-mobile = only in table view.
  - Alternative for dense admin views: `responsive?: 'cards' | 'scroll'` — scroll adds `overflow-x-auto` wrapper.
  - New component: `<DataTableCardView<T>>` exported from same module.
- **Effort:** 2 days
- **Files:** `data-table.tsx` (add card view + column priority), new `data-table-card.tsx`

#### GAP-W05: StatCard (`packages/ui/src/molecules/stat-card.tsx`)
- **Severity:** 🟡 High
- **Current:** Used in 4-column grids (`grid-cols-4` in every dashboard/list screen)
- **Gap:** 4 columns don't fit on mobile; even 2 is tight at 360px
- **Fix:**
  - StatCard itself is fine (flex-1 inside grid). The fix belongs in each screen's grid.
  - Provide a canonical `StatCardGrid` wrapper in `packages/ui`:
    - Default: `grid grid-cols-2 gap-3`
    - `sm:`: `grid-cols-2` (same)
    - `lg:`: `grid-cols-4`
    - Handles the 3-stat edge case: `grid-cols-2` on mobile (3rd item spans or leaves gap)
  - This eliminates per-screen grid repetition.
- **Effort:** 0.5 day
- **Files:** New `stat-card-grid.tsx`

### 2.3 🟡 HIGH — Forms & Inputs

#### GAP-W06: FormField / FormSection (`packages/ui/src/molecules/form-field.tsx`)
- **Severity:** 🟡 High
- **Current:** `FormSection` uses a 2-column grid per DESIGN-SYSTEM §6 (form field grid)
- **Gap:** 2 columns on 360px screen = 156px each — too narrow for label+input+error
- **Fix:**
  - `FormSection`: Default `grid-cols-1`, `lg:grid-cols-2`
  - Full-width fields (e.g. Alamat) already span both columns — no change needed when they're the only column
  - `FormField` internal: `flex-col` is already vertical — no change needed
  - Label font: `text-[13px]` stays (reads fine on mobile)
- **Effort:** 0.25 day
- **Files:** `form-field.tsx`

#### GAP-W07: FilterSelect / SearchField (`primitives/`)
- **Severity:** 🔵 Medium
- **Current:** `SearchField` has `w-[220px]` fixed; `FilterSelect` has native `<select>`
- **Gap:** Fixed width wastes mobile space; filter row breaks into multiple lines
- **Fix:**
  - `SearchField`: Default `w-full`, `sm:w-[220px]`
  - Provide a `FilterRow` wrapper in `packages/ui`:
    - Default: `flex flex-col gap-2` (stacked)
    - `sm:`: `flex flex-row flex-wrap gap-2 items-end`
    - Each filter child: `w-full sm:w-auto`
  - Multiple screens build their own filter rows — this canonicalises them.
- **Effort:** 0.5 day
- **Files:** `search-field.tsx`, new `filter-row.tsx`

### 2.4 🔵 MEDIUM — Overlays & Feedback

#### GAP-W08: Modal / ConfirmDialog (`packages/ui/src/molecules/modal.tsx`)
- **Severity:** 🔵 Medium
- **Current:** Fixed `max-w-lg` (512px); no mobile full-width variant
- **Gap:** On mobile, modals should be full-width bottom sheets or near-full-width with small margin
- **Fix:**
  - Default: `m-4 w-[calc(100vw-32px)] max-w-lg` — leaves 16px margin on each side
  - Desktop: existing `max-w-lg`
  - Touch-friendly close button: 44×44px minimum hit area
  - Content area: `max-h-[calc(100vh-120px)] overflow-y-auto` for long forms
- **Effort:** 0.25 day
- **Files:** `modal.tsx`

#### GAP-W09: Drawer (`packages/ui/src/molecules/drawer.tsx`)
- **Severity:** 🔵 Medium
- **Current:** Right-sheet, fixed width
- **Gap:** On mobile, drawer should take full width (or near-full)
- **Fix:**
  - Default: `w-[calc(100vw-32px)] max-w-md`
  - Desktop: existing fixed width
  - Ensure `max-h-[100vh]` and internal scroll for long content
- **Effort:** 0.25 day
- **Files:** `drawer.tsx`

#### GAP-W10: Toast (`packages/ui/src/molecules/toast.tsx`)
- **Severity:** ⚪ Low
- **Current:** Fixed position bottom-right
- **Gap:** On mobile, bottom-right may be obscured by mobile browser chrome
- **Fix:**
  - Default: `bottom-4 left-4 right-4` (full-width bottom bar)
  - `sm:`: `bottom-4 right-4 left-auto` (existing corner position)
- **Effort:** 0.1 day
- **Files:** `toast.tsx`

#### GAP-W11: Breadcrumb (in `topbar.tsx`)
- **Severity:** 🔵 Medium
- **Current:** Shows full path: "Dashboard > Karyawan > Detail"
- **Gap:** Long paths overflow on narrow screens
- **Fix:**
  - Default (mobile): Show only the CURRENT page label, prefixed with "←" if there's a parent
  - `md:`: Full breadcrumb path
  - This is already covered in GAP-W03 (Topbar), documented here for completeness.
- **Effort:** Included in GAP-W03

#### GAP-W12: ExportModal (`packages/ui/src/molecules/export-modal.tsx`)
- **Severity:** ⚪ Low
- **Current:** Multi-step modal with stepper
- **Gap:** Stepper may wrap on narrow screens
- **Fix:** Same modal width fix as GAP-W08 covers this
- **Effort:** 0 (covered by GAP-W08)

#### GAP-W13: SettingsSubnav (`packages/ui/src/molecules/settings-subnav.tsx`)
- **Severity:** 🔵 Medium
- **Current:** Horizontal tab row
- **Gap:** Tabs overflow/wrap on mobile
- **Fix:**
  - Default: Horizontal scrollable row (`overflow-x-auto`, `flex-nowrap`)
  - `md:`: Existing full-width tab row
- **Effort:** 0.2 day
- **Files:** `settings-subnav.tsx`

---

## 3. Web App — Auth Screens (Phase 1)

### 3.1 🟡 HIGH

#### GAP-W14: AuthLayout (`features/auth/auth-layout.tsx`)
- **Severity:** 🟡 High
- **Current:** Brand panel `hidden lg:flex` ✓ (already responsive). Form card `w-[380px]`
- **Gap:** `w-[380px]` exceeds 360px phone width (20px overflow with `p-10` parent)
- **Fix:**
  - Form card: `w-full max-w-[380px]` — shrinks on narrow screens
  - Parent padding: `p-6 sm:p-10` — less padding on mobile
  - Mobile: add a compact brand header above the form (logo + wordmark, not the full brand panel)
    - `lg:hidden` inline logo+wordmark row at the top of the form column
- **Effort:** 0.5 day

#### GAP-W15: LoginScreen (`features/auth/login-screen.tsx`)
- **Severity:** 🔵 Medium
- **Current:** Form inside AuthLayout card
- **Gap:** Inputs, button, "Ingat saya" checkbox — all fit but touch targets need checking
- **Fix:**
  - Input height: ensure 44px minimum touch target
  - "Ingat saya" checkbox + label: touch target 44px
  - "Lupa kata sandi" link: touch target 44px
  - Button: full width on mobile, `h-12` for touch
- **Effort:** 0.2 day

#### GAP-W16: ForgotPasswordScreen / ResetPasswordScreen / ChangePasswordScreen
- **Severity:** ⚪ Low
- **Current:** Same AuthLayout, same card
- **Gap:** Same as GAP-W14/W15
- **Fix:** Covered by GAP-W14/W15 — additional screens just need a visual pass
- **Effort:** 0.2 day (visual pass across all 3)

---

## 4. Web App — Feature Screens (Phase 2, per epic)

> After Phase 0+1, every feature screen is structurally responsive through the fixed foundation components (responsive shell, table→card view, 1-col forms, stat card grid). This section audits per-screen specifics that are NOT covered by foundation fixes.

### 4.1 E1 — Foundations (`features/e1-foundations/`)

#### GAP-W17: UsersScreen (`users-screen.tsx`) — 🟡 High
- **Current:** Full table + 4 stat cards + filter row + tabs
- **Foundation coverage:** StatCardGrid fixes stat cards. DataTable card view fixes table. FilterRow fixes filters.
- **Remaining gaps:**
  - Tab bar ("Semua | Aktif | Nonaktif"): ensure horizontal scroll on mobile (`overflow-x-auto flex-nowrap`)
  - "Tambah Pengguna" button: ensure it doesn't overlap with tabs on narrow screens → stack button below title on mobile
- **Effort:** 0.3 day

#### GAP-W18: AuditLogScreen (`audit-log-screen.tsx`) — 🔵 Medium
- **Current:** Desktop table with many columns (timestamp, actor, action, entity, before/after, IP)
- **Foundation coverage:** DataTable card view handles this
- **Remaining gaps:** Detail drawer (GAP-W09 covers it)
- **Effort:** 0.1 day (verify)

#### GAP-W19: SettingsLayout (`settings-layout.tsx`) — 🔵 Medium
- **Current:** Horizontal subnav tabs + `<Outlet>`
- **Gap:** Subnav tabs overflow on mobile
- **Fix:** GAP-W13 (SettingsSubnav) covers this
- **Effort:** 0 (covered)

#### GAP-W20: SettingsGeneralScreen / SettingsOverviewScreen — ⚪ Low
- **Current:** Form fields / read-only settings
- **Foundation coverage:** FormField (GAP-W06) covers form layout
- **Effort:** 0 (covered)

#### GAP-W21: UserOverlays (`user-overlays.tsx`) — 🔵 Medium
- **Current:** Modal forms (Tambah Pengguna, Ubah Peran, Edit User drawer)
- **Foundation coverage:** Modal (GAP-W08) + Drawer (GAP-W09) + FormField (GAP-W06)
- **Effort:** 0.1 day (verify touch targets on buttons in modals)

#### GAP-W22: GlobalStates (`global-states.tsx`) — ⚪ Low
- **Current:** SessionExpired, NoPermission screens
- **Gap:** Centered content — naturally responsive
- **Effort:** 0 (no gap)

### 4.2 E2 — Identity / Karyawan (`features/e2-identity/`)

#### GAP-W23: EmployeesScreen (`employees-screen.tsx`) — 🟡 High
- **Current:** Full table + 4 stat cards + complex filters (search, status, company, employee_type, tabs)
- **Foundation coverage:** DataTable + StatCardGrid + FilterRow
- **Remaining gaps:**
  - Filter row has 3 filters + search — verify FilterRow stacking works for this density
  - Row-kebab menu: ensure dropdown doesn't overflow viewport on mobile
- **Effort:** 0.3 day

#### GAP-W24: EmployeeDetailScreen (`employee-detail-screen.tsx`) — 🟡 High
- **Current:** Tabs (Profil | Penempatan | Kehadiran | Cuti & Lembur) + detail cards
- **Gap:** Tabs overflow on mobile; detail card sections may have wide content
- **Fix:**
  - Tabs: horizontal scroll on mobile (same pattern as user tabs)
  - Detail sections: 2-col read-only grid → single column on mobile
  - Provide a `DetailGrid` wrapper: `grid grid-cols-1 lg:grid-cols-2 gap-4`
- **Effort:** 0.4 day

#### GAP-W25: EmployeeForm (`employee-form.tsx`) — 🔵 Medium
- **Current:** Long form with 2-column grid sections (Pribadi, Kontak, Statutori & Bank, Akun Login)
- **Foundation coverage:** FormSection (GAP-W06) → 1-col mobile, 2-col desktop
- **Remaining gaps:** Long form → ensure "Simpan" button is sticky or at bottom; scroll container full height
- **Effort:** 0.2 day

#### GAP-W26: ClientCompaniesScreen (`client-companies-screen.tsx`) — 🔵 Medium
- **Current:** Table list + search
- **Foundation coverage:** DataTable + SearchField
- **Effort:** 0.1 day (verify)

#### GAP-W27: ClientCompanyDetailScreen (`client-company-detail-screen.tsx`) — 🟡 High
- **Current:** Tabs (Profil, Lokasi & Site, Template Persetujuan) + detail sections + map
- **Gap:** Same as employee detail — tabs + 2-col grids + map sizing
- **Fix:** Same tab + detail grid pattern. Map: full width on mobile, fixed height on desktop.
- **Effort:** 0.4 day

#### GAP-W28: ClientCompanyForm / SiteForm — 🔵 Medium
- **Current:** Form pages
- **Foundation coverage:** FormSection (GAP-W06)
- **Effort:** 0.1 day (verify)

#### GAP-W29: AgreementsScreen / AgreementDetailScreen / AgreementForm — 🔵 Medium
- **Current:** List + detail + form
- **Foundation coverage:** DataTable + FormSection
- **Effort:** 0.2 day (verify)

#### GAP-W30: MasterDataHubScreen — ⚪ Low
- **Current:** Hub with 3 card links
- **Gap:** Cards in a grid — naturally responsive
- **Effort:** 0.05 day (verify)

#### GAP-W31: LeaveTypesScreen / AttendanceCodesScreen / OvertimeRulesScreen — 🔵 Medium
- **Current:** Data tables + modals for create/edit
- **Foundation coverage:** DataTable + Modal + FormSection
- **Effort:** 0.3 day (verify across 3 screens)

### 4.3 E3 — Placement (`features/e3-placement/`)

#### GAP-W32: PlacementsScreen (`placements-screen.tsx`) — 🟡 High
- **Current:** Full table + filters (company, status, position, expiring-soon)
- **Foundation coverage:** DataTable + FilterRow
- **Effort:** 0.2 day

#### GAP-W33: CompanyRosterScreen (`company-roster-screen.tsx`) — 🔵 Medium
- **Current:** Table grouped by company
- **Foundation coverage:** DataTable
- **Effort:** 0.1 day

#### GAP-W34: PlacementDetailScreen (`placement-detail-screen.tsx`) — 🟡 High
- **Current:** 9 lifecycle variants + audit trail + detail card
- **Gap:** Lifecycle timeline (horizontal) may overflow on mobile
- **Fix:**
  - Timeline: `overflow-x-auto` on mobile with horizontal scroll; desktop shows full timeline
  - Detail sections: single column on mobile
  - Audit trail drawer: GAP-W09 covers
- **Effort:** 0.4 day

#### GAP-W35: PlacementForm / PlacementOverlays — 🔵 Medium
- **Current:** Create placement form + transfer/renew/end/terminate/resign modals
- **Foundation coverage:** FormSection + Modal
- **Effort:** 0.2 day

### 4.4 E4 — Shift Scheduling (`features/e4-scheduling/`)

#### GAP-W36: ShiftMastersScreen (`shift-masters-screen.tsx`) — 🔵 Medium
- **Current:** Table + Tambah Shift modal
- **Foundation coverage:** DataTable + Modal
- **Effort:** 0.1 day

#### GAP-W37: ScheduleGridScreen (`schedule-grid-screen.tsx`) — 🟡 High
- **Current:** Weekly grid: rows = agents × 7 day columns
- **Gap:** 7 columns at ~200px each = 1400px+ — worst overflow offender
- **Fix (most complex responsiveness challenge):**
  - Default (mobile): Show TODAY only (1 column). Swipe left/right or use day-picker to change day. OR show 3 days (today + 2) with horizontal scroll.
  - `md:`: Show 7-day week with horizontal scroll
  - `xl:`: Full week without scroll
  - Add a day-navigation control: `< Day >` with "Today" button for mobile
  - Alternative: Vertical list of agents with their week assignments stacked (takes more vertical space but no horizontal scroll)
  - **Decision needed:** Card-stack vs horizontal-scroll vs day-picker approach
- **Effort:** 1.5 days (largest single-screen effort)

#### GAP-W38: ScheduleOverlays (`schedule-overlays.tsx`) — 🔵 Medium
- **Current:** Shift picker popover, day-off/clear menu
- **Gap:** Popover position may overflow on mobile
- **Fix:** Ensure popover uses `position: fixed` with viewport boundary detection
- **Effort:** 0.3 day

### 4.5 E5 — Attendance (`features/e5-attendance/`)

#### GAP-W39: AttendanceDashboardScreen (`attendance-dashboard-screen.tsx`) — 🟡 High
- **Current:** 4 stat cards + table with many columns + filters
- **Foundation coverage:** StatCardGrid + DataTable + FilterRow
- **Remaining gaps:** "Verifikasi (N)" button in title band — ensure it stacks below title on mobile
- **Effort:** 0.2 day

#### GAP-W40: AttendanceVerificationScreen (`attendance-verification-screen.tsx`) — 🟡 High
- **Current:** Verification queue table + bulk actions
- **Foundation coverage:** DataTable (card view on mobile)
- **Remaining gaps:** Bulk action bar — ensure select-all checkbox + action buttons fit on mobile
- **Fix:** Stacked bulk bar on mobile (select count + actions below); inline on desktop
- **Effort:** 0.3 day

#### GAP-W41: AttendanceDetailScreen / CorrectionScreens — 🔵 Medium
- **Current:** Detail cards + GPS map + event timeline
- **Gap:** Map sizing on mobile; timeline overflow
- **Fix:** Map: full width on mobile. Timeline: single-column on mobile (instead of 2-col left/right)
- **Effort:** 0.3 day

### 4.6 E6 — Leave (`features/e6-leave/`)

#### GAP-W42: LeaveApprovalsScreen (`leave-approvals-screen.tsx`) — 🟡 High
- **Current:** L2/L1 queues + detail + bulk actions
- **Foundation coverage:** DataTable
- **Effort:** 0.2 day

#### GAP-W43: LeaveDetailScreen (`leave-detail-screen.tsx`) — 🔵 Medium
- **Current:** Full leave request detail with approval trail + balance impact
- **Gap:** Detail sections may be wide; approval timeline horizontal
- **Fix:** Single-column detail on mobile; timeline vertical stack
- **Effort:** 0.3 day

#### GAP-W44: LeaveQuotasScreen / LeaveCalendarScreen — 🔵 Medium
- **Current:** Quota table + calendar view
- **Gap:** Calendar may need day-by-day view on mobile instead of full month grid
- **Fix:**
  - Calendar: `grid-cols-7` reduces to `grid-cols-1` list view on mobile (group by day with agent names)
  - Quota table: DataTable covers
- **Effort:** 0.4 day

#### GAP-W45: LeaveOverlays (`leave-overlays.tsx`) — 🔵 Medium
- **Current:** Approve/reject modals + quota adjust/grant modals
- **Foundation coverage:** Modal (GAP-W08)
- **Effort:** 0.1 day

### 4.7 E7 — Overtime (`features/e7-overtime/`)

#### GAP-W46: OvertimeApprovalsScreen / OvertimeRecordsScreen — 🟡 High
- **Current:** L2/L1 approval tables + Rekap table
- **Foundation coverage:** DataTable
- **Effort:** 0.2 day

#### GAP-W47: OvertimeDetailScreen (`overtime-detail-screen.tsx`) — 🔵 Medium
- **Current:** Decision UI with calc block + tier breakdown + approvals
- **Gap:** Calc block layout may overflow
- **Fix:** Stack calc rows vertically on mobile
- **Effort:** 0.3 day

#### GAP-W48: OvertimeRulesScreen / HolidayOverlays — 🔵 Medium
- **Current:** Rules reference table + holiday calendar + overlays
- **Foundation coverage:** DataTable + Modal
- **Effort:** 0.2 day

### 4.8 E8 — Payroll (`features/e8-payroll/`)

#### GAP-W49: PayslipArchiveScreen (`payslip-archive-screen.tsx`) — 🔵 Medium
- **Current:** Table + period/year/employee/status filters
- **Foundation coverage:** DataTable + FilterRow
- **Effort:** 0.1 day

#### GAP-W50: PayslipDetailScreen (`payslip-detail-screen.tsx`) — 🔵 Medium
- **Current:** Component breakdown (earnings/deductions/benefits) + IDR money
- **Gap:** Earnings/deductions side-by-side may overflow
- **Fix:** Stack earnings and deductions vertically on mobile
- **Effort:** 0.2 day

#### GAP-W51: PeriodCloseScreen (`period-close-screen.tsx`) — 🔵 Medium
- **Current:** Cockpit with table + blockers + actions
- **Foundation coverage:** DataTable + Modal
- **Effort:** 0.2 day

### 4.9 E10 — Reporting (`features/e10-reporting/`)

#### GAP-W52: DashboardScreen / BillableReportScreen / NotificationsScreen — 🟡 High
- **Current:** KPI widgets + bar charts + report table + notification cards
- **Gap:** Dashboard widget grid (3–4 columns) overflows; bar charts need width adaptation
- **Fix:**
  - Widget grid: `grid-cols-1 sm:grid-cols-2 lg:grid-cols-4`
  - Bar charts: proportional width via CSS `%` (they're div-based, not SVG — should already scale)
  - Notif cards: already vertical stack ✓
- **Effort:** 0.3 day

### 4.10 E11 — Approvals (`features/e11-approvals/`)

#### GAP-W53: ApprovalInboxScreen / ApprovalDetailScreen — 🔵 Medium
- **Current:** Inbox table + chain timeline + bypass modal
- **Foundation coverage:** DataTable + Modal
- **Remaining gaps:** Chain timeline (horizontal row of line members) may overflow
- **Fix:** Timeline wraps on mobile (vertical stack of lines, each line shows member chips in a flex-wrap row)
- **Effort:** 0.3 day

#### GAP-W54: ApprovalTemplateEditorScreen — 🔵 Medium
- **Current:** Line cards with member multi-select
- **Gap:** Member chips + "Tambah anggota" may overflow
- **Fix:** Flex-wrap on member chips; ensure modal width GAP-W08 applies
- **Effort:** 0.2 day

### 4.11 Agent Self-Service (`features/agent/`)

#### GAP-W55: Agent screens (`me-*.tsx`) — 🔵 Medium
- **Current:** Agent-facing web screens (used by agents on desktop browser)
- **Foundation coverage:** Most are simple card/list layouts
- **Gap:** These are simpler screens (single-user view) — fewer responsiveness issues
- **Effort:** 0.3 day (verify all ~8 agent screens)

---

## 5. Mobile App — Component Layer (Phase 3)

> React Native + NativeWind is inherently more responsive (flex layouts), but fixed sizes and
> pixel-based padding are the primary gaps. Audited against 360px (small Android), 390px (iPhone 14), 430px (Pro Max).

### 5.1 🔴 CRITICAL

#### GAP-M01: attendance.tsx — Clock-in screen layout
- **Severity:** 🔴 Critical
- **Current:** 1036-line component. Live clock (`monoHero`, 46px), GPS status chip, big Clock In button, today's masuk/keluar tiles
- **Gap:** On 360px screen, 46px clock + surrounding elements may crowd; button may feel small on larger screens
- **Fix:**
  - Live clock: use `useWindowDimensions()` to scale between 36px (360w) and 46px (430w)
  - Clock-in button: `w-[80%] max-w-[320px] self-center h-16` (touch-friendly, proportionally sized)
  - Masuk/keluar tiles: `flex-row` with equal flex — already responsive ✓
  - GPS status chip: ensure text doesn't truncate on narrow screens
  - Activity log section: scrollable, already fine
- **Effort:** 0.5 day
- **Files:** `app/(app)/attendance.tsx`

### 5.2 🟡 HIGH

#### GAP-M02: Screen (`src/ui/Screen.tsx`)
- **Severity:** 🟡 High
- **Current:** Fixed `px-6 py-8` (24px / 32px)
- **Gap:** On 360px screen, `px-6` = 24px × 2 = 48px = 13% of screen width wasted on padding. On 430px screen, same 24px is only 11%.
- **Fix:**
  - Use percentage-based horizontal padding: `px-[5%]` (18px on 360w, 21.5px on 430w)
  - OR: Use `px-4` on small, `px-6` on larger (via `useWindowDimensions`)
  - Vertical padding: `py-6` — 24px is fine vertically
  - This is a SINGLE change that affects EVERY mobile screen
- **Effort:** 0.2 day

#### GAP-M03: Text scaling (`src/ui/Text.tsx`)
- **Severity:** 🟡 High
- **Current:** Fixed font sizes in RAMP (30, 22, 20, 19, 15, 14, 13, 12, 11, 10, 28, 46)
- **Gap:** On 360px screen, 30px pageTitle takes 8.3% of width. On 430px, 7%. The `monoHero` (46px) is 12.8% vs 10.7%. Fonts are proportionally large on small screens.
- **Fix:**
  - Option A (recommended): Respect system font scale. React Native's `allowFontScaling` is already on by default. Ensure all Text components allow scaling (don't set `allowFontScaling={false}` anywhere).
  - Option B: Dynamic scaling. Use `useWindowDimensions()` to scale the two largest variants (`pageTitle` 30→26, `monoHero` 46→38) on narrow screens.
  - Option C: No change. The type ramp is designed for mobile; 30px pageTitle is standard.
  - **Recommendation:** Audit for `allowFontScaling={false}` first. If none found, fonts are fine. The ramp was designed for mobile (390px canvas). Only `monoHero` (46px live clock) might need attention on 360px — covered in GAP-M01.
- **Effort:** 0.1 day (audit for allowFontScaling suppression)

#### GAP-M04: Button touch targets (`src/ui/Button.tsx`)
- **Severity:** 🟡 High
- **Current:** Unknown — need to check if buttons meet 44pt minimum
- **Gap:** Small buttons may be hard to tap on mobile
- **Fix:**
  - Ensure minimum height: `min-h-[44px]`
  - Ensure `px-5` minimum horizontal padding (20px each side)
  - Icon-only buttons: `min-w-[44px] min-h-[44px]`
- **Effort:** 0.2 day

#### GAP-M05: Card (`src/ui/Card.tsx`)
- **Severity:** 🔵 Medium
- **Current:** Unknown — need to check padding and layout
- **Gap:** Fixed padding may be too large on small screens
- **Fix:** Use percentage-based or responsive padding
- **Effort:** 0.1 day

#### GAP-M06: BottomSheet (`src/ui/BottomSheet.tsx`)
- **Severity:** 🔵 Medium
- **Current:** Bottom sheet with content
- **Gap:** Content height may overflow on short screens (667px iPhone SE)
- **Fix:** `max-h-[80vh]` constraint with internal scroll
- **Effort:** 0.2 day

#### GAP-M07: ApprovalActionSheet / ApprovalChain (`src/ui/ApprovalActionSheet.tsx`, `ApprovalChain.tsx`)
- **Severity:** 🔵 Medium
- **Current:** Action sheet with member chips + chain timeline
- **Gap:** Chain timeline (horizontal member chips per line) may overflow
- **Fix:** Flex-wrap on member chips; vertical line stack
- **Effort:** 0.3 day

#### GAP-M08: StatusBadge (`src/ui/StatusBadge.tsx`)
- **Severity:** ⚪ Low
- **Current:** Token-based coloring + fixed size
- **Gap:** Should be fine (small component, token-driven)
- **Effort:** 0 (verify only)

### 5.3 Mobile Screens (Feature Layer)

#### GAP-M09: leader-beranda.tsx — 🟡 High
- **Current:** Stat tiles in `flex-row gap-2` (4 tiles per row)
- **Gap:** On 360px screen, 4 tiles at equal flex = 82px each — very narrow for "15" (number) + "Hadir" (label)
- **Fix:**
  - `flex-row flex-wrap`: 2 tiles per row on ≤390px, 4 per row on wider
  - OR: Use `useWindowDimensions()` to decide column count
  - Section cards: fine as-is (vertical stack)
- **Effort:** 0.3 day

#### GAP-M10: profile.tsx + profile-edit.tsx — 🔵 Medium
- **Current:** Read-only field rows + editable form
- **Gap:** Field rows are vertical (label above value) — already mobile-friendly ✓
- **Effort:** 0.1 day (verify)

#### GAP-M11: sl-verifikasi.tsx — 🔵 Medium
- **Current:** Verification queue list (E5 + E11 segments)
- **Gap:** List items with status badge + action buttons — buttons may crowd
- **Fix:** Ensure action buttons stack below on very narrow screens; use `flex-wrap`
- **Effort:** 0.2 day

#### GAP-M12: leave.tsx — 🔵 Medium
- **Current:** Leave history list with status badges
- **Gap:** Standard list — responsive by default
- **Effort:** 0.1 day (verify)

#### GAP-M13: leave-new.tsx — 🟡 High
- **Current:** Leave request form (type, dates, doc upload, delegate, reason)
- **Gap:** Date range picker + form fields on narrow screen
- **Fix:**
  - Date inputs: `flex-row` on wider screens, `flex-col` on narrow
  - Document upload button: full width on narrow screens
  - Delegate picker: uses bottom sheet — GAP-M06 covers
- **Effort:** 0.3 day

#### GAP-M14: overtime.tsx / overtime-new.tsx — 🔵 Medium
- **Current:** OT history + OT request form
- **Gap:** Similar to leave screens — form field responsiveness
- **Effort:** 0.3 day (both screens)

#### GAP-M15: pengajuan.tsx — 🔵 Medium
- **Current:** Hub screen with CTA cards (Cuti, Lembur, Koreksi, Kalender)
- **Gap:** Card grid — 2×2 on mobile; ensure minimum card width
- **Effort:** 0.2 day

#### GAP-M16: schedule.tsx — 🟡 High
- **Current:** Weekly schedule view for agent
- **Gap:** 7-day week view needs horizontal adaptation (similar to web schedule grid)
- **Fix:**
  - 7-day horizontal scroll OR 3-day sliding window on narrow screens
  - Each day cell: ensure shift name + time fit on narrow cells
- **Effort:** 1 day

#### GAP-M17: notifications.tsx — ⚪ Low
- **Current:** NotifCard list
- **Gap:** Vertical stack — already responsive ✓
- **Effort:** 0.05 day (verify)

#### GAP-M18: attendance-history.tsx — 🔵 Medium
- **Current:** History list with status badges + verification badges
- **Gap:** Row layout may be tight on narrow screens
- **Effort:** 0.2 day

#### GAP-M19: attendance-detail.tsx — 🔵 Medium
- **Current:** Single attendance record detail
- **Gap:** Horizontal detail sections (time in/out, GPS, verification) — ensure stacking
- **Effort:** 0.2 day

#### GAP-M20: kehadiran.tsx — 🔵 Medium
- **Current:** List of attendance records
- **Gap:** Standard list — similar to M18
- **Effort:** 0.15 day

#### GAP-M21: approval-status.tsx — 🔵 Medium
- **Current:** Chain timeline (from E11)
- **Gap:** GAP-M07 covers chain component
- **Effort:** 0 (covered)

#### GAP-M22: payslip.tsx — 🔵 Medium
- **Current:** Payslip list + summary detail
- **Gap:** Detail screen — summary only (no component breakdown for agent) → simple layout
- **Effort:** 0.1 day (verify)

#### GAP-M23: correction.tsx / correction-picker.tsx / correction-detail.tsx / correction-tracker.tsx — 🔵 Medium
- **Current:** Correction request flow (4 screens)
- **Gap:** Form screens + picker modals → Modal (GAP-W08 port) + FormField patterns
- **Effort:** 0.4 day (all 4)

#### GAP-M24: more.tsx — ⚪ Low
- **Current:** Settings menu (list of links)
- **Gap:** Simple list — naturally responsive
- **Effort:** 0.05 day (verify)

#### GAP-M25: change-password.tsx — ⚪ Low
- **Current:** Password change form (3 fields)
- **Gap:** Simple form — fine
- **Effort:** 0.05 day (verify)

#### GAP-M26: clarification-answer.tsx — ⚪ Low
- **Current:** Answer clarification form
- **Gap:** Simple form — fine
- **Effort:** 0.05 day (verify)

#### GAP-M27: Auth screens (login, forgot-password, reset-password, reset-sent, reset-success) — 🔵 Medium
- **Current:** Auth forms with brand header
- **Gap:** Similar to web auth — ensure inputs fit and touch targets are large
- **Effort:** 0.3 day (5 screens)

#### GAP-M28: App tab layout (`(app)/_layout.tsx`) — ⚪ Low
- **Current:** Bottom tabs already responsive (native tab bar)
- **Gap:** None — tab bar is OS-native and adapts
- **Effort:** 0

---

## 6. Cross-Cutting Concerns (Phase 4)

### 6.1 🟡 HIGH

#### GAP-X01: Touch Target Audit
- **Severity:** 🟡 High
- **Scope:** All interactive elements across web + mobile
- **Standard:** WCAG 2.1: 44×44 CSS pixels minimum touch target (24×24 for inline links)
- **Actions:**
  - Web: Audit all `<button>`, `<a>`, `<input>`, `<select>`, checkbox labels
  - Mobile: Audit all `<Pressable>`, `<TouchableOpacity>`, `<Button>`
  - Add `min-h-[44px] min-w-[44px]` to the button primitive (web `packages/ui`, mobile `src/ui/Button.tsx`)
  - Run a script/lint rule to flag non-compliant touch targets
- **Effort:** 1 day (audit + fixes)

### 6.2 🔵 MEDIUM

#### GAP-X02: Viewport Meta & Mobile Browser Chrome
- **Severity:** 🔵 Medium
- **Scope:** Web app only
- **Current:** Unknown — need to check `index.html` for viewport meta tag
- **Actions:**
  - Ensure `<meta name="viewport" content="width=device-width, initial-scale=1">` is present
  - Add `theme-color` meta for browser chrome coloring
  - Add `apple-mobile-web-app-capable` for "Add to Home Screen" behavior
  - Test iOS Safari bottom bar (safe area) — ensure content isn't hidden behind the URL bar
- **Effort:** 0.2 day

#### GAP-X03: Font Scaling / Accessibility
- **Severity:** 🔵 Medium
- **Scope:** Both platforms
- **Actions:**
  - Web: Test at 200% browser zoom. Ensure layouts don't break.
  - Mobile: Audit for `allowFontScaling={false}` — remove any instances (GAP-M03 covers this)
  - Web: Use `rem` units for font sizes in Tailwind (already `text-sm` etc. — these are `rem`-based ✓)
- **Effort:** 0.3 day

---

## 7. Implementation Phases & Priority Ordering

### Phase 0 — Foundation (5–7 days) 🔴🔴🔴
> Everything depends on this. No feature screen work starts until Phase 0 is done.

| ID | Item | Sev | Effort | Depends On |
|----|------|-----|--------|------------|
| W01 | AppShell — mobile sidebar drawer | 🔴 | 1.5d | — |
| W02 | Sidebar — drawer variant | 🔴 | 0.5d | W01 |
| W03 | Topbar — hamburger + responsive breadcrumb | 🟡 | 0.5d | W01 |
| W04 | DataTable — card view + column priority | 🟡 | 2d | — |
| W05 | StatCardGrid — canonical wrapper | 🟡 | 0.5d | — |
| W06 | FormSection — 1-col mobile, 2-col desktop | 🟡 | 0.25d | — |
| W07 | FilterRow + responsive SearchField | 🔵 | 0.5d | — |
| W08 | Modal — mobile full-width | 🔵 | 0.25d | — |
| W09 | Drawer — mobile full-width | 🔵 | 0.25d | — |
| W10 | Toast — mobile full-width bottom | ⚪ | 0.1d | — |
| W13 | SettingsSubnav — horizontal scroll | 🔵 | 0.2d | — |
| X02 | Viewport meta + browser chrome | 🔵 | 0.2d | — |

### Phase 1 — Auth Screens (1 day)
> Quick wins; these are the first screens users see.

| ID | Item | Sev | Effort |
|----|------|-----|--------|
| W14 | AuthLayout — responsive card + mobile brand header | 🟡 | 0.5d |
| W15 | LoginScreen — touch targets | 🔵 | 0.2d |
| W16 | Forgot/Reset/ChangePassword — visual pass | ⚪ | 0.2d |

### Phase 2A — Core Feature Screens (6–8 days)
> E1 (Foundations), E2 (Karyawan), E3 (Penempatan) — the most-used screens.

| ID | Item | Sev | Effort |
|----|------|-----|--------|
| W17 | UsersScreen | 🟡 | 0.3d |
| W18 | AuditLogScreen | 🔵 | 0.1d |
| W21 | UserOverlays | 🔵 | 0.1d |
| W23 | EmployeesScreen | 🟡 | 0.3d |
| W24 | EmployeeDetailScreen | 🟡 | 0.4d |
| W25 | EmployeeForm | 🔵 | 0.2d |
| W26 | ClientCompaniesScreen | 🔵 | 0.1d |
| W27 | ClientCompanyDetailScreen | 🟡 | 0.4d |
| W28 | ClientCompanyForm / SiteForm | 🔵 | 0.1d |
| W29 | Agreements (3 screens) | 🔵 | 0.2d |
| W30 | MasterDataHub | ⚪ | 0.05d |
| W31 | MasterData (3 CRUD screens) | 🔵 | 0.3d |
| W32 | PlacementsScreen | 🟡 | 0.2d |
| W33 | CompanyRosterScreen | 🔵 | 0.1d |
| W34 | PlacementDetailScreen | 🟡 | 0.4d |
| W35 | PlacementForm + Overlays | 🔵 | 0.2d |

### Phase 2B — Scheduling & Attendance (4–5 days)
> The hardest screens: schedule grid + attendance dashboard.

| ID | Item | Sev | Effort |
|----|------|-----|--------|
| W36 | ShiftMastersScreen | 🔵 | 0.1d |
| W37 | ScheduleGridScreen | 🟡 | 1.5d |
| W38 | ScheduleOverlays | 🔵 | 0.3d |
| W39 | AttendanceDashboardScreen | 🟡 | 0.2d |
| W40 | AttendanceVerificationScreen | 🟡 | 0.3d |
| W41 | AttendanceDetail + Corrections | 🔵 | 0.3d |

### Phase 2C — Leave, Overtime, Payroll (4–5 days)

| ID | Item | Sev | Effort |
|----|------|-----|--------|
| W42 | LeaveApprovalsScreen | 🟡 | 0.2d |
| W43 | LeaveDetailScreen | 🔵 | 0.3d |
| W44 | LeaveQuotas + Calendar | 🔵 | 0.4d |
| W45 | LeaveOverlays | 🔵 | 0.1d |
| W46 | OvertimeApprovals + Records | 🟡 | 0.2d |
| W47 | OvertimeDetailScreen | 🔵 | 0.3d |
| W48 | OvertimeRules + Holidays | 🔵 | 0.2d |
| W49 | PayslipArchiveScreen | 🔵 | 0.1d |
| W50 | PayslipDetailScreen | 🔵 | 0.2d |
| W51 | PeriodCloseScreen | 🔵 | 0.2d |

### Phase 2D — Reporting, Approvals, Agent (2–3 days)

| ID | Item | Sev | Effort |
|----|------|-----|--------|
| W52 | Dashboard + Billable + Notifications | 🟡 | 0.3d |
| W53 | ApprovalInbox + Detail | 🔵 | 0.3d |
| W54 | ApprovalTemplateEditor | 🔵 | 0.2d |
| W55 | Agent screens (all 8) | 🔵 | 0.3d |

### Phase 3A — Mobile Foundation Components (2–3 days)

| ID | Item | Sev | Effort |
|----|------|-----|--------|
| M02 | Screen — responsive padding | 🟡 | 0.2d |
| M03 | Text — font scaling audit | 🟡 | 0.1d |
| M04 | Button — touch targets | 🟡 | 0.2d |
| M05 | Card — responsive padding | 🔵 | 0.1d |
| M06 | BottomSheet — max-height | 🔵 | 0.2d |
| M07 | ApprovalActionSheet + Chain | 🔵 | 0.3d |
| M08 | StatusBadge — verify | ⚪ | 0d |

### Phase 3B — Mobile Screens (5–8 days)

| ID | Item | Sev | Effort |
|----|------|-----|--------|
| M01 | attendance.tsx — clock-in layout | 🔴 | 0.5d |
| M09 | leader-beranda.tsx — stat tiles | 🟡 | 0.3d |
| M13 | leave-new.tsx — form | 🟡 | 0.3d |
| M16 | schedule.tsx — weekly view | 🟡 | 1d |
| M10 | profile + profile-edit | 🔵 | 0.1d |
| M11 | sl-verifikasi.tsx | 🔵 | 0.2d |
| M12 | leave.tsx | 🔵 | 0.1d |
| M14 | overtime + overtime-new | 🔵 | 0.3d |
| M15 | pengajuan.tsx | 🔵 | 0.2d |
| M18 | attendance-history.tsx | 🔵 | 0.2d |
| M19 | attendance-detail.tsx | 🔵 | 0.2d |
| M20 | kehadiran.tsx | 🔵 | 0.15d |
| M22 | payslip.tsx | 🔵 | 0.1d |
| M23 | correction screens (4) | 🔵 | 0.4d |
| M27 | auth screens (5) | 🔵 | 0.3d |
| M17 | notifications.tsx | ⚪ | 0.05d |
| M24 | more.tsx | ⚪ | 0.05d |
| M25 | change-password.tsx | ⚪ | 0.05d |
| M26 | clarification-answer.tsx | ⚪ | 0.05d |

### Phase 4 — Cross-Cutting Polish (1–2 days)

| ID | Item | Sev | Effort |
|----|------|-----|--------|
| X01 | Touch target audit + fixes | 🟡 | 1d |
| X03 | Font scaling / accessibility audit | 🔵 | 0.3d |

---

## 8. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Sidebar drawer + shell changes break existing screens | Medium | High | Merge Phase 0 to a feature branch; QA all screens before merging to main |
| DataTable card view degrades admin productivity | Medium | Medium | Card view is mobile-only (default). Desktop keeps full table. Admin users on desktop unaffected. |
| Schedule grid (W37) redesign is complex | High | Medium | Prototype 3 approaches (day-picker, horizontal-scroll, card-stack); pick with screenshot feedback before coding |
| Mobile font scaling may break layouts | Low | Medium | Test on largest system font size (accessibility setting) before shipping |
| Touch target changes may shift existing layouts | Low | Low | Increase padding NOT margin — internal spacing grows but layout position stays |

---

## 9. Open Decisions

These need product/design input before execution:

1. **W37 — Schedule Grid mobile approach:** Day-picker (1 day view, swipe) vs horizontal-scroll (3–7 days visible, scroll sideways) vs card-stack (vertical list of agents × their week)? Card-stack is most mobile-native but deviates most from the `.pen` design.

2. **W04 — DataTable mobile card layout:** Which columns appear in the card view? The `priority` field lets each screen decide, but should we have a sensible default (first N columns = card)?

3. **M02 — Mobile Screen padding:** Fixed `px-4` for all sizes vs percentage-based vs width-dependent? Percentage is most consistent.

4. **M03 — Mobile font sizes:** Dynamic scaling for largest variants (pageTitle 30→26, monoHero 46→38) on ≤375px screens, or leave as-is since the type ramp was designed for 390px mobile canvas?

---

## 10. Verification Strategy

### Per-Phase Verification
- After Phase 0: Verify responsive shell on 4 viewports (360, 390, 768, 1440)
- After each Phase 2 epic: Verify all screens in that epic at 360 + 1440
- After Phase 3A: Verify mobile components at 360 + 390 + 430
- After Phase 3B: Verify all mobile screens at 360 + 430
- Final: Full Playwright E2E run at 3 viewports (mobile 390, tablet 768, desktop 1440)

### Testing Tools
- Web: Chrome DevTools device toolbar (iPhone SE, iPhone 14, Pixel 5, iPad, Desktop)
- Mobile: iOS Simulator (iPhone SE 3rd gen, iPhone 14, iPhone 15 Pro Max) + Android emulator (Pixel 6a, 360dp)
- Automated: Playwright viewport tests + Maestro mobile flows

---

> **Next Step:** Present plan for review. Do not execute until approved.
> Once approved, start with Phase 0 (GAP-W01: AppShell mobile sidebar drawer).
