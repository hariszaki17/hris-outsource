# E10 Reporting — Design Audit

**Date:** 2026-06-02
**.pen frame:** `w5JgL` (▦ FEATURE GROUP · E10 Laporan & Notifikasi)
**Specs:** `docs/epics/E10-reporting/FEATURE.md` + 4 PRDs (dashboards, notifications, attendance-billable-report, export-framework) + `EPICS.md` §8
**Scope:** Audit only — no edits to the `.pen` file.

---

## 1. Screen inventory

| # | Screen name | Frame ID | Platform | Purpose | Reachable from |
|---|---|---|---|---|---|
| 1 | E10 · Dashboard (HR) | `ETi5H` | Web (HR/Super Admin) | Cross-company KPIs, billable trend, "Perlu Tindakan" panel | Sidebar (Dashboard); from any other E10 screen |
| 2 | E10 · Laporan Kehadiran & Jam Billable (HR) | `EF8AZ` | Web (HR) | The v1 priority report (F10.3): filters, stats, agent table, export button | Sidebar (Laporan); dashboard deep-links (implicit) |
| 3 | E10 · Ekspor Laporan (modal) | `FJ6hX` | Web | Format choice modal (Excel/PDF/CSV) over the report screen with scrim | "Ekspor" button on report (`MxNSO`); cancel/close back to report |
| 4 | E10 · Pusat Notifikasi (HR) | `i0qW8` | Web (HR) | In-app notification center: tabs, sections (HARI INI / KEMARIN), mark-all-read | Sidebar (Notifikasi); Topbar bell (implicit) |
| 5 | E10 SL · Dashboard Tim | `RiSPW` | Web (Shift Leader) | Team/company-scoped KPIs, billable chart, "Perlu Tindakan" panel | Sidebar (Dashboard) |
| 6 | Agen · Notifikasi | `WKYgI` | Mobile (Agent) | In-app mobile notification list with sections (HARI INI / KEMARIN) | BottomNav "Notifikasi" tab (active) |
| 7 | Agen · Beranda (Dashboard) | `e8Sw1` | Mobile (Agent) | Personal dashboard: next shift + clock-in, leave balance, OT this month, pending request | BottomNav "Beranda" tab (active) |

**Total designed in E10 frame:** 7 screens (4 web, 2 mobile, 1 modal overlay).

---

## 2. Dead-end findings

### 2.1 Unwired clickable components (cat a)

- **HR/SL Dashboard "Perlu Tindakan" rows (`SIsts` and `yuLeS`)** — Four rows each ("Verifikasi kehadiran", "Persetujuan cuti", "Persetujuan lembur", "Penempatan akan berakhir") each carry a count pill and a `chevron-right`, but none of these point to designed targets *within E10*. PRD `dashboards.md` §DB-5 requires deep-links to underlying features. These are expected to deep-link into E5/E6/E7/E3 screens — **needs cross-epic verification** (see §6). No E10-internal "leader combined approval inbox" screen exists; the dashboard's Perlu Tindakan list is the de facto entry point.
- **HR/SL Dashboard topbar `caFkE` (`SSyhN`, `G6vDFZ`)** — Topbar component is reused (likely contains a bell/avatar). The bell→notification-center route is not visually distinguished/wired in the audit; only the sidebar entry to `i0qW8` is explicit. Acceptable if the Topbar component itself implements the bell (verify in `caFkE` master).
- **Notification list cards (web `LZn66`/`nqQJU`, mobile `A3W2r`)** — Each card has a payload deep-link target (NT-4). Targets are *implicit* (presumed to navigate to the underlying request/exception/schedule), but no designed mobile or web detail screen "after notification tap" lives inside E10 (these belong to E4/E5/E6/E7). The notification screens do not exhibit a "stale deep link" graceful state (PRD `notifications.md` C-4).
- **"Tandai semua dibaca" pill (web `B9XPDU`)** — Designed as a clickable button. There is no designed *post-state* (toast confirmation, or list updated with all dots removed). PRD `notifications.md` AC "Mark read" requires unread → read transition.
- **Per-notification unread dots (`WslFv`, `meAQc`, `LsYPf`, `eOrnJ`, `inLk6`)** — Implicit "tap to mark read" affordance, but no marked-read state or transition shown.
- **Mobile notifikasi list (`A3W2r`)** — Notification cards do not include any "Tandai semua dibaca" affordance or per-item read action; only visual dots.
- **Export modal "Ekspor" Go button (`KnZN9`)** — Clickable, but no progress/queued/success/error result state is designed (see §2.3).
- **Report filter pills (`J9UA5`, `EwlEB`, `IwZYN`, `R7pj2` for HR; `hp2EI`, `qeLeO`, `f7VDr`, `G94kL` for SL)** — Filter dropdowns shown but no expanded filter-overlay state (date picker, multi-select dropdown). Acceptable if relying on shared `comp/FilterSelect` master, but the *opened* state of these filters is not shown anywhere in E10.
- **Agent Beranda "Clock In" button (`XsPTq`)** — Cross-epic: deep-links into E5 attendance. Verify E5 has the Clock-In flow.
- **Agent Beranda "Pending" leave card (`hRFNM`)** — Clickable surface (deep-link to E6 request detail). Cross-epic; verify E6.

### 2.2 Orphan screens (cat b)

None. Every E10 screen is reachable via sidebar (web) or bottom-nav (mobile) entry points. The export modal is correctly invoked from the report screen.

### 2.3 Missing result states (cat c) — **critical**

- **Export progress state (queued/in-progress)** — PRD `export-framework.md` EX-3 + AC "Large export is queued" → not designed. Modal `uSW4W` only shows format-selection step; no "Sedang menyiapkan…", spinner, queued-job indicator, or "Notifikasi akan dikirim saat siap" confirmation. **CRITICAL**.
- **Export success state + download trigger** — Not designed. EX-1 + AC "the file is generated immediately and downloadable" requires a success affordance (toast with download link, or modal success step). **CRITICAL**.
- **Export error / size-too-large state** — Not designed. PRD `export-framework.md` C-1 (job failed) and C-2 (large PDF warn) → no error modal, no warning banner on PDF selection. **CRITICAL**.
- **Notification "mark as read" post-state** — No after-state (toast "3 ditandai dibaca", or list with dots removed). NT-4 read/unread transition has no designed result. **CRITICAL**.
- **Notification empty state** — No "Tidak ada notifikasi" state designed for either web `i0qW8` or mobile `WKYgI`. PRD `notifications.md` (and basic UX) requires this. **CRITICAL**.
- **Notification loading state** — Not designed (skeleton/spinner).
- **Dashboard empty states** — PRD `dashboards.md` C-1 (new user no data) and C-3 (leader with no agents) explicit → **no empty/getting-started variants** designed for HR `ETi5H`, SL `RiSPW`, or Agent `e8Sw1`. **CRITICAL** (esp. agent: a freshly-onboarded agent with no shift/leave/OT/billable has nothing to render in `NextShift`, `Sisa Cuti`, `Lembur bulan ini`, `PERMINTAAN`).
- **Approval inbox empty state** — The "Perlu Tindakan" panel on HR/SL dashboards always shows non-zero counts. PRD `dashboards.md` DB-3 implies that when no approvals are pending, this panel should render an empty/cleared state. Not designed. **CRITICAL** for the leader workflow (the entire reason they open the app).
- **Report empty / no-data state** — `EF8AZ` and `RiSPW` reports always show populated tables; no "Tidak ada data untuk filter ini" state. Per PRD `attendance-billable-report.md` BR-6 (unverified excluded) and C-1, a filter combination can plausibly yield zero rows. Missing.
- **Report "pending records" callout** — `attendance-billable-report.md` C-1 says "optionally shown as pending"; the report table footer shows totals only, no pending-rows section/banner.
- **Report loading state** — No skeleton/spinner during run.

### 2.4 Untriggered overlays (cat d)

- **Filter dropdown panels** — The four `FilterSelect` instances on each report screen have no designed expanded/popover variant in E10. (May be covered by component master.)
- **Notification preferences UI** — PRD `notifications.md` NT-5 + EPICS §8 locked decision "all-on v1, mute non-critical later" → no settings entrypoint or stub UI reserved. Acceptable for v1 (decision locked all-on), but **no future-hook is visible** — recommend at least a disabled settings affordance in the notification center.
- **Confirmation modal for "Tandai semua dibaca"** — Not designed; could be a soft confirm or silent operation; no decision visible.
- **Date range picker overlay** — Implicit on "Periode: Mei 2026" filter; no overlay state.

### 2.5 Dangling back/close (cat e)

- **Export modal `uSW4W`** — Close icon `jarNL` (`x`) and "Batal" button `x5ZiLw` both lead back to the underlying report screen — correctly designed (scrim `vdgv4` returns to `EF8AZ`).
- **Mobile screens** — No back arrow needed since both are top-level bottom-nav tabs. OK.
- **Web sidebar nav** — Sidebar (`caFkE`/`iCqTB`) is consistent across screens. OK.

No dangling close handlers identified beyond §2.3 (the modal's "Ekspor" Go button has no destination — covered there).

---

## 3. Missing screens (cat f)

Severity legend: **P0** = blocks the user story / locked decision; **P1** = explicit PRD case; **P2** = nice-to-have / standard UX hygiene.

| Sev | Missing screen / state | Source |
|---|---|---|
| **P0** | **Export progress / queued state** (modal step 2) | `export-framework.md` EX-3, AC, F10.4 mermaid |
| **P0** | **Export success state** with download trigger (modal step 3 or toast) | `export-framework.md` EX-1, AC |
| **P0** | **Export error state** (failed job; PDF-size warning) | `export-framework.md` C-1, C-2 |
| **P0** | **Notification empty state** (web + mobile) | `notifications.md` (implicit), UX hygiene |
| **P0** | **Notification mark-as-read result** (toast or refreshed list) | `notifications.md` AC "Mark read", NT-4 |
| **P0** | **Dashboard empty state — Agent** (new agent, no shift/leave/OT/requests) | `dashboards.md` C-1 |
| **P0** | **Approval inbox empty state** (HR + SL "Perlu Tindakan" zero-count) | `dashboards.md` DB-3 |
| **P1** | **Dashboard empty state — HR** (no companies / fresh tenant) | `dashboards.md` C-1 |
| **P1** | **Dashboard empty state — Shift Leader** (no agents placed yet) | `dashboards.md` C-3 |
| **P1** | **Super Admin dashboard variant** | EPICS roles list — distinct from HR? May share HR design; needs an explicit decision label |
| **P1** | **Report empty/no-data state** (filter yields zero rows) | `attendance-billable-report.md` C-1, BR-6 |
| **P1** | **Report "pending records" callout** (unverified shown separately) | `attendance-billable-report.md` C-1 |
| **P1** | **Stale-deep-link graceful state** (notification target deleted) | `notifications.md` C-4 |
| **P1** | **Notification group/batched item** (bulk schedule publish) | `notifications.md` C-2 |
| **P1** | **Critical-mute override visual indicator** | `notifications.md` NT-5, C-3 |
| **P1** | **Mobile leader dashboard** (Shift Leader is also a mobile user per F10.2 §4) | `dashboards.md` §4 — Mobile = "Agent / Leader"; only Agent mobile is designed |
| **P1** | **Mobile leader approval inbox** (leader combined queue on mobile) | Implicit from F10.2 + F10.1 |
| **P1** | **Notification loading state** / **Report loading state** | UX hygiene |
| **P1** | **Notification preferences entrypoint** (disabled stub in v1) | `notifications.md` NT-5 + §8 decision |
| **P1** | **Export retention/expiry hint** in modal footer (if a file may expire) | `export-framework.md` C-4 |
| **P2** | **Sensitive-export confidentiality marking** preview | `export-framework.md` EX-5, AC |
| **P2** | **Agent "no upcoming shift" variant** (off-duty days) | UX hygiene |
| **P2** | **Web mobile-responsive view for the report** (PRD calls report "not a primary mobile surface" — OK to skip) | `attendance-billable-report.md` §4 |

---

## 4. PRD coverage matrix

| PRD | Required screens/states | Designed | Missing |
|---|---|---|---|
| `notifications.md` (F10.1) | Web center, mobile center, mark-read, empty, loading, stale-link, batched, prefs entrypoint | Web center (`i0qW8`), mobile center (`WKYgI`); cards with unread dots; "Tandai semua dibaca" affordance on web | Mark-read result; empty; loading; stale-link state; batched item; mobile mark-read affordance; preferences UI/stub |
| `dashboards.md` (F10.2) | HR, SL, Agent, Super Admin dashboards (web + mobile where applicable); empty states; deep-links | HR web (`ETi5H`), SL web (`RiSPW`), Agent mobile (`e8Sw1`); deep-link rows | Super Admin variant (or explicit "same as HR" decision); SL mobile; HR/SL/Agent empty states; approval-inbox empty state; freshness/loading indicator |
| `attendance-billable-report.md` (F10.3) | Filtered report (HR + SL), export trigger, empty, pending-callout, loading | HR (`EF8AZ`) + SL (`IZPG8`) report; export trigger button | Empty state; pending records section; loading state; corrections-after-export point-in-time messaging |
| `export-framework.md` (F10.4) | Format choice modal, progress/queued, success/download, error, sensitive-marking, retention hint | Format choice modal (`uSW4W`) | Progress/queued; success+download; error; PDF-size warn; sensitive marking; retention hint |

---

## 5. Role-coverage check

- Super admin dashboard: **NO** (no distinct frame; presumed identical to HR — needs explicit decision marker, since EPICS lists Super Admin and HR as separate roles)
- HR admin dashboard: **YES** (`ETi5H`)
- Shift leader dashboard: **YES** web (`RiSPW`); **NO** mobile variant
- Agent dashboard: **YES** mobile (`e8Sw1`); not required on web per F10.2 §4
- Approval inbox (leader combined): **PARTIAL** — exists as the "Perlu Tindakan" panel inside the leader dashboard (`SIsts`/`yuLeS`); no standalone combined-queue screen; no empty state; no item-level UI (just counters)
- Export progress/success/error: **NO** (format-selection only)
- Notification empty/loading: **NO**

---

## 6. Cross-epic references found

Dashboards and the approval-inbox panel deep-link heavily into other epics. Synthesis should verify these targets exist in the relevant epic frames:

- **From HR/SL "Perlu Tindakan" rows (`SIsts`, `yuLeS`):**
  - "Verifikasi kehadiran" → **E5** attendance verification screen
  - "Persetujuan cuti" → **E6** leave approval list/detail
  - "Persetujuan lembur" → **E7** OT approval list/detail
  - "Penempatan akan berakhir" → **E3** placement detail / expiring-soon view
- **From HR/SL report (`EF8AZ`, `IZPG8`):**
  - Filters reference **E3** (ClientCompany, ServiceLine), **E5** (Attendance), **E2** (AttendanceCode `is_billable`), **E1** (audit log for exports)
  - Agent rows could deep-link to **E2** employee profile or **E3** placement
- **From notification cards (`LZn66`, `nqQJU`, web `i0qW8`; mobile `A3W2r`):**
  - "Jadwal diperbarui / dipublikasikan" → **E4** schedule view
  - "Konfirmasi lembur" / "Lembur disetujui" → **E7** OT request / approval
  - "Cuti disetujui" → **E6** leave detail
  - "Verifikasi kehadiran" → **E5** verification queue
  - "Auto clock-out" → **E5** attendance detail
  - "Penempatan akan berakhir" → **E3** placement detail
  - "Pengingat shift" → **E4** schedule / **E5** clock-in
- **From Agent Beranda (`e8Sw1`):**
  - `NextShift` "Clock In" (`XsPTq`) → **E5** clock-in flow
  - `Sisa Cuti` (`lD3ZP`) → **E6** leave balance/request
  - `Lembur bulan ini` (`x631dJ`) → **E7** OT history/request
  - `PERMINTAAN` pending row (`hRFNM`) → **E6** leave-request detail
- **From Topbar (`caFkE` instances `SSyhN`, `G6vDFZ`, `uk02H`, `jKCYu`, `KrY8K`):** bell → web notification center `i0qW8`
- **From Sidebar (`iCqTB` instances):** standard nav across all epics

---

## 7. Prioritized recommendation

**P0 — must add before E10 is "designed":**
1. **Export progress + success + error states.** The Ekspor modal currently has no terminal state for its primary action. Without these, the F10.4 user story is functionally undesigned. Recommend: add 3 modal step variants (Step 1 Format → Step 2 Progress/Queued → Step 3 Success-with-download; plus an Error variant). Bonus: a queued-job toast pattern in the topbar when the requester navigates away.
2. **Notification empty state (web + mobile)** and **mark-as-read result.** Reuse the empty-state pattern from DS / other epics. Add a subtle toast on "Tandai semua dibaca". Show the read-state version of a card (no dot) somewhere in the .pen so the visual delta is documented.
3. **Approval-inbox empty state** on the leader/HR dashboard "Perlu Tindakan" panel. Even a single "Tidak ada tindakan tertunda — semua bersih ✅" state per PRD `dashboards.md` DB-3.
4. **Agent dashboard empty state.** `dashboards.md` C-1 is explicit; a new agent has no shift, no requests, no OT. Without this, the layout breaks.

**P1 — to close the PRD matrix:**
5. **Super Admin variant or explicit decision marker** that Super Admin = HR view (mark in EPICS §8 or `FEATURE.md`).
6. **Shift Leader mobile dashboard + mobile notifications variant** (F10.2 §4 says leader is on mobile too).
7. **Report empty state** + **pending-records callout** (`attendance-billable-report.md` C-1).
8. **Notification preferences entrypoint stub** (disabled in v1, but reserved per §8 decision).
9. **Stale-deep-link, batched-item, and critical-override variants** in the notification center (`notifications.md` C-2, C-3, C-4).
10. **Loading states** for notifications, report, and dashboard widgets (NT-6, BR-6, DB-6 freshness).

**P2 — polish:**
11. **Sensitive-export confidentiality marking** preview in modal for payroll exports (EX-5).
12. **PDF-size warning** when PDF format chosen on large result (C-2).
13. **Retention/expiry hint** in export footer (C-4).
14. **Agent off-duty / no upcoming shift** variant of Beranda.

---

## 8. Notes

- The "Perlu Tindakan" panel on dashboards is doing double duty as both a dashboard widget *and* the leader's combined approval inbox. Either (a) accept it as the inbox (and add empty-state, item-level UI, "see all" deep-link), or (b) add a dedicated combined-approval-inbox screen in E10. The PRD `dashboards.md` DB-3 leans toward (a). Decision should be recorded.
- Topbar (`caFkE`) and Sidebar (`iCqTB`) are reused components from the DS. Their bell/avatar/notification-badge behaviors must be verified at the *component master* level — not visible from this audit. Particularly: does the Topbar bell carry an unread-count badge and route to `i0qW8`?
- The web Pusat Notifikasi (`i0qW8`) shows three tab pills besides "Semua": "Belum dibaca", "Persetujuan", "Jadwal". Mobile `WKYgI` does not show these tab pills (verified: mobile Body uses `HARI INI`/`KEMARIN` section labels only). Decision: mobile filter-by-category — out of v1?
- Per EPICS §8 locked decisions: **notifications all-on v1** — acceptable to skip preferences UI entirely, but a placeholder is recommended so the future "mute non-critical" doesn't require a new IA slot.
- Per F10.3 §4: report is "not a primary surface" on mobile — OK that no mobile report screens exist. Document this explicitly so it isn't flagged as a gap.
- Billing math = hours only (EPICS §8): the report table correctly shows only hours (Worked / Billable / Payable), no rate or amount columns. ✅
- Billable = verified only (INV-4): the "Tingkat Verifikasi 98%" stat card on the report (`SVe1n`/`UKr3c`) reinforces this. ✅ Worth also adding a "Excluded: 8j unverified" line near totals.
- All seven E10 frames consistently use design-system tokens (`$primary`, `$primary-soft`, `$surface`, `$text`, `$warn-tx`, `$ok-tx`, `$bad-tx`, `$info-tx`, etc.) — no raw hex outside the FeatureBanner header chrome. ✅
- Total severity counts: **8 P0**, **12 P1**, **4 P2** = **24 prioritized gaps** before E10 design is complete.
