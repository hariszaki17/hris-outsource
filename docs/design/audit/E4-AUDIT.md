# E4 Shift Scheduling — Design Audit

**Date:** 2026-06-02
**.pen frame:** `WnejY` ("▦ FEATURE GROUP · E4 Jadwal Shift")
**Specs:** `FEATURE.md` + 4 PRDs (`shift-master-catalog`, `daily-schedule-assignment`, `schedule-views`, `schedule-changes-swaps`)
**Decisions audited against:** EPICS.md §8 + FEATURE.md §7 (2026-05-29 lock): leader-driven v1 (agent swap deferred), one-shift-per-day, block-over-approved-leave, bulk apply-to-range, reminders evening-before + ~1h prior, auto-publish on save.

---

## 1. Screen inventory

| # | Screen name | Frame ID | Platform | Purpose | Reachable from |
|---|---|---|---|---|---|
| 1 | E4 · Master Shift | `O5JgF` | Web · HR/Super Admin | List shift templates (7 rows: Pagi, Siang, Malam, Parkir Pagi, Parkir Malam, Building Day, Cleaning Pagi). Search + line/status filters. Each row has kebab menu. | Sidebar nav (entry) |
| 2 | E4 · Tambah Shift (modal) | `Mn9ux` (modal node `O44xpk` overlaid on a Master Shift duplicate) | Web · HR/Super Admin | New-template form: Nama, Lini Layanan (opsional), Jam Mulai/Selesai, Istirahat Mulai/Selesai, Shift aktif toggle, +1-day note. Cancel / Simpan Shift. | "Tambah Shift" button on screen 1 |
| 3 | E4 · Jadwal Mingguan (Shift Leader) | `Rubba` | Web · Shift Leader | Week schedule grid for Plaza Senayan: 5 agents × 7 days. Week navigator, "Terapkan ke rentang" trigger, AutoPublish banner, legend, Cuti-locked cells (warn-bg + lock icon), plus-icon empty cells for unassigned days. | Sidebar nav (entry) |
| 4 | Agen · Jadwal Saya | `fN9AJ` | Mobile · Agent | Today highlight card (gold accent, Pagi 07:00–15:00, location line) + Mendatang list (3 future days, 2 Libur pills). Bottom nav (Beranda / Jadwal / Kehadiran / Profil). | Bottom-nav "Jadwal" (entry) |

**Total: 4 frames, 0 overlays beyond the embedded Tambah-Shift modal.**

---

## 2. Dead-end findings

### 2.1 Unwired clickable components (cat a)

| Component | Location | Lands on | Issue |
|---|---|---|---|
| Row kebab (`ellipsis-vertical` ×7) | Master Shift list (`O5JgF` rows 0–6: `XNTdy`, `wLypw`, `S0Jos9`, `fzbIq`, `FmR5o`, `f3TqHD`, `MUGIC`) | nowhere | No popover/menu for Edit / Duplicate / Deactivate. SM-5 (deactivate-not-delete) has no UI. |
| Master Shift row body (full row) | Rows in `I2djIQ` | nowhere | Per PRD users edit by clicking row → modal; no Edit Shift modal in design. |
| "Terapkan ke rentang" button | `taPhA` on Jadwal Mingguan | nowhere | Bulk apply helper is locked (C-1 / decision 2026-05-29) but no modal designed. |
| Empty cell `+` icon ×2 | Eko Wijaya row Rab/Kam cells (`l5DGq`, `kzXG5`) | nowhere | Cell-assign action has no shift-picker popover. **This is the core F4.2 interaction.** |
| Filled shift chip (cells row0–row4) | Many | nowhere | No edit/clear cell modal for F4.4 leader-direct-edit. |
| Cuti-locked cell (`ImdjW`, `iVVTr`) | Dewi Lestari Sen/Sel | nowhere | Click should explain the block ("Agent on approved leave"); no tooltip/toast designed. |
| Bell icon | Agent mobile AppBar (`CyMUr`) | nowhere | Notifications inbox not in this epic but the bell is rendered as if clickable. |
| Week-nav arrows | `rUFuv` / `QFWbW` on Jadwal Mingguan | implied state | No alternate week states designed — fine for v1 if treated as in-place state. |
| Search + 2 filter dropdowns | `xhF3r`, `J6Uk8e`, `Ejn4j` on Master Shift; `j29g2`, `IB2t0`, `PCA7I` in modal-bg | nowhere | No filter-result / no-result states. |
| Mobile day cards (`f6Z7m`, `rKbTb`, `KaG3b`, `RWtdG`) | Agent Jadwal Saya | nowhere | No shift-detail screen for agent (SV-3 site + map). |
| Bottom-nav non-active items (Beranda, Kehadiran, Profil) | `AryC6`, `nJkbg`, `wGNFB` | other epics | Acceptable (cross-epic) but no E4-internal navigation back/up from a shift-detail screen because the detail screen does not exist. |

### 2.2 Orphan screens (cat b)

None. All 4 screens are reachable from a sidebar/bottom-nav entry point.

### 2.3 Missing result states (cat c)

Critical — *every* writing action is silent:

| Missing state | PRD ref | Where it should be |
|---|---|---|
| Toast / inline success after Save Shift | SM-6 audit, INV-4 | Tambah Shift modal close → Master Shift list |
| Toast / banner after auto-publish of a cell | SA-6, INV-4 | Jadwal Mingguan after cell save |
| Toast on schedule change (Changed status) | CH-2 | Jadwal Mingguan after edit/clear |
| Toast on bulk-apply completion (with skip-count if some days blocked) | C-1 SA-1/SA-2/Leave | After Terapkan ke rentang |
| Validation error: Nama duplicate (SM-4) | SM-4 | Tambah Shift modal |
| Validation error: break outside window (SM-1) | SM-1 | Tambah Shift modal |
| Validation error: 24h shift confirm (C-1) | Open decision | Tambah Shift modal |
| Block toast: scheduling agent without active placement (SA-1) | SA-1 | Jadwal Mingguan cell click |
| Block toast: scheduling beyond placement end (SA-5) | SA-5 | Jadwal Mingguan |
| Replace-warning confirm: existing shift that day (SA-2) | SA-2 | Jadwal Mingguan cell click |
| Block toast: scheduling over approved leave | Decision 2026-05-29 | Jadwal Mingguan (Cuti cells visible but no error-on-click state) |
| Block toast: scope violation (different company) (SA-3, INV-3) | SA-3 | (would only show for HR; leader UI scopes it) |
| Agent push-notification card / banner ("Shift baru / berubah") | INV-4 | Mobile (no notification UI rendered) |
| Shift reminder notification state (evening-before + 1h prior) | SV-5 | Mobile (no reminder UI rendered) |
| Empty state: agent with no upcoming shifts (C-1 F4.3) | C-1 | Mobile Jadwal Saya |
| Empty state: leader of company with no placed agents (C-3 F4.3) | C-3 | Jadwal Mingguan |
| Empty state: filter returns no shift templates | n/a | Master Shift |
| Loading / skeleton state | n/a | All 4 screens |
| Map disabled state (no geo) (C-2 F4.3) | C-2 | Mobile detail (also missing) |
| Deactivate confirm modal | SM-5 | Master Shift |
| Edit Shift modal (separate from Tambah; or reuse with prefilled state) | F4.1 PRD §7 | Master Shift |

### 2.4 Untriggered overlays (cat d)

| Overlay | Designed? | Trigger present? |
|---|---|---|
| Tambah Shift modal (`O44xpk`) | yes | yes — "Tambah Shift" button (`Z7EOap` / `P3hKA`) |
| Edit Shift modal | **no** | row kebab / row click present but lands nowhere |
| Deactivate confirm | **no** | n/a |
| Shift-picker popover (cell-assign) | **no** | `+` icons present but unwired |
| Cell-edit / cell-clear popover | **no** | chips present but unwired |
| Bulk apply-to-range modal | **no** | "Terapkan ke rentang" button present but unwired |
| Conflict-block toast / dialog | **no** | n/a — system-driven; needs design state |
| Replace-warning confirm | **no** | n/a |
| Notification toast (auto-publish success) | **no** | n/a |
| Agent push-notification banner | **no** | n/a |
| Reminder notification | **no** | n/a |

### 2.5 Dangling back/close (cat e)

| Control | Frame | Closes to | Issue |
|---|---|---|---|
| Modal close `x` icon (`Noa4K`) | Tambah Shift | implied (Master Shift behind) | OK — visually grounded by scrim and bg screen. |
| Modal Cancel button (`T7fHSJ`) | Tambah Shift | implied | OK. |
| Modal Save button (`v5MmFb`) | Tambah Shift | should close + toast | Toast missing (see 2.3). |
| Agent mobile — no back from any shift card | Agen Jadwal Saya | n/a (no detail screen exists) | Acceptable only because shift-detail isn't designed; gap is the detail screen itself. |

---

## 3. Missing screens (cat f)

Ordered by criticality:

1. **Shift-picker popover / sheet** (F4.2 core action — the `+` cell click). Without this, no schedule can be assigned. **BLOCKER for US "assign a shift".**
2. **Cell-edit / cell-clear popover** (F4.4 leader edits v1). **BLOCKER for F4.4 v1 scope.**
3. **Bulk apply-to-range modal** (locked decision 2026-05-29; SA C-1). Date range + shift picker + agent scope + skipped-days summary. **HIGH** (decision-locked feature with no UI).
4. **Conflict-block dialog/toast set** (scheduling over leave, beyond placement, on inactive placement). Locked decision 2026-05-29 says "blocked"; no design proves the user-facing block. **HIGH** (rule-enforcement check fails).
5. **Replace-warning confirm** (SA-2). **MEDIUM.**
6. **Edit Shift modal** (or Tambah Shift in edit mode, with delete-disabled / deactivate replacement per SM-5). **HIGH** (lifecycle decision SM-5 has no UI).
7. **Deactivate-shift confirm** (SM-5). **MEDIUM.**
8. **Validation error states inside Tambah Shift** (SM-1 break, SM-4 unique title, optional 24h confirm C-1). **MEDIUM.**
9. **Auto-publish success toast** (INV-4 — every write should give feedback). **HIGH** (silent saves break user trust in auto-publish).
10. **Agent push-notification + reminder states** (INV-4, SV-5). At least: shift-change inbox card + a reminder banner / system notification mock. **HIGH** (SV-5 is decision-locked).
11. **Agent shift-detail screen** (SV-3 site + map). **MEDIUM.**
12. **Empty + loading states for all 4 screens** (C-1/C-3 F4.3, plus standard loading). **MEDIUM.**
13. **HR-Admin schedule oversight view** (F4.2 actor "HR/Super Admin schedules any company"). The Jadwal Mingguan screen is leader-scoped (Plaza Senayan hardcoded). Either confirm leader view is reused with company picker for HR, or add the picker overlay. **MEDIUM.**
14. **Day view + by-agent matrix variants** of F4.3 (SV-2: "Views: day, week, and by-agent matrix"). Only week is designed. **LOW–MEDIUM** (week may be the v1 default if the others are deferred — but FEATURE.md doesn't defer them).
15. **Shift-leader mobile/tablet view** (FEATURE.md §6: "Mobile app · Shift Leader · Quick view/edit of today's roster"). Not designed at all. **LOW** for v1 if web-only roster is acceptable, but FEATURE.md treats it as in-scope.

---

## 4. PRD coverage matrix

| PRD | Required screens/states | Designed | Missing |
|---|---|---|---|
| **F4.1 Shift Master Catalog** | List, Create, Edit, Deactivate confirm, validation errors (SM-1/SM-4), empty/loading | List ✅; Create modal ✅ | Edit modal ✗; Deactivate confirm ✗; error states (break-outside, unique title) ✗; empty/loading ✗; kebab menu popover ✗ |
| **F4.2 Daily Schedule Assignment** | Week grid (leader), shift-picker, OFF marker, block states (no placement / beyond end / over leave / scope), replace-warning, bulk-apply modal, auto-publish toast, HR oversight variant | Week grid ✅ (with Cuti-locked, OFF, `+` empty cells, AutoPublish banner) | Shift-picker popover ✗ (critical); OFF marker action ✗; all 5 block toasts ✗; replace-warning ✗; bulk-apply modal ✗; success toast ✗; HR variant ✗; leader mobile/tablet ✗ |
| **F4.3 Schedule Calendar & Agent View** | Leader day/week/by-agent views; agent mobile list + today highlight + site detail; reminders; live-update indicator; empty states (no upcoming, no agents, no geo); cross-midnight rendering | Leader week ✅; agent week list ✅ (today highlight, Libur pills); cross-midnight chip seen in master list (`+1` badge) | Leader day view ✗; by-agent matrix ✗; agent shift-detail (site + map) ✗; reminder notification state ✗; live-update indicator ✗; all empty states ✗ |
| **F4.4 Schedule Changes & Swaps** (v1 scope = leader edits only; agent swap deferred) | Cell-edit popover, cell-clear, conflict re-validation, audit/notify confirmation, change toast | Cuti-block cell visual ✅ (passive); nothing else | Cell-edit popover ✗; cell-clear ✗; conflict toasts ✗ (overlap with F4.2 list); change toast ✗. **Correctly omits** agent swap request UI (deferred). |

---

## 5. Business-rule enforcement check

- **Schedule-over-approved-leave block state:** PARTIAL. Cuti-locked cells render correctly with `warn-bg` + lock icon ("Cuti (terkunci)" legend swatch present). However, **no toast or error popover** appears on attempted click — the rule is shown visually but not on the action path. **Decision 2026-05-29 says blocked → needs an active error state, not only a passive lock.**
- **Double-shift attempt block state (one shift per day, SA-2 / INV-1):** NO. No replace-warning, no block. Cells with existing shifts have no click-affordance and no overlay.
- **Publish confirmation:** NO. Auto-publish banner is shown ("Publish otomatis…") but no **success toast / state** after the actual save. Silent auto-publish breaks the user-feedback contract.
- **Bulk apply helper:** PARTIAL. Trigger button ("Terapkan ke rentang") is present; **no modal** designed. Critical because per-day re-validation (C-1) implies a "skipped N days due to leave/no placement" report state.
- **Reminder notification states:** NO. SV-5 + Decision 2026-05-29 (evening-before + ~1h prior) — neither the evening-before push card nor the 1h-prior reminder are designed (no system-notification mock, no agent inbox).
- **Cross-midnight handling:** YES (rendering). Master Shift "Malam" 23:00–07:00 has `+1` badge (`ugXtX`, `BY7xL`). Agent list and grid do not yet show the dual-day cross-midnight visual (SV-6 "displays spanning two days") — **minor gap in F4.3.**
- **Scope (INV-3, SA-3, SV-1):** PARTIAL. Leader view hardcodes "Plaza Senayan" (good for the leader screenshot). HR variant + company-picker not designed.
- **Auto-publish to mobile (INV-4):** PARTIAL. Agent mobile shows current schedule; no change-notification UI shows that a fresh push has arrived.

---

## 6. Cross-epic references found

- **E3 Placement** — Jadwal Mingguan subtitle "5 agen ditempatkan · shift dari master, terfilter lini layanan" references active placements (SA-1) and service-line filtering (SA-4). The grid's left-rail per-agent name + line dot (`ln` frame in each row) implies a placement-driven roster. No filter UI for service line in the grid header, though.
- **E6 Leave** — Cuti-locked cells (`ImdjW`, `iVVTr` on Dewi Lestari Sen/Sel; warn-bg, lock icon, "Cuti (terkunci)" legend) overlay approved leave from E6. Locked-decision integration is present visually but not on the error path.
- **E5 Attendance** — Cross-midnight `+1` badge in Master Shift hints at the start-date attribution rule that E5 must consume. No attendance-status overlay on the schedule grid (correctly out of scope; F4.3 §2 explicitly defers it).
- **E2 Service Line** — Master Shift row dots (Parking blue, Building Mgmt green, Facility gold) + "Semua lini" for untagged templates (SM-3). Filter "Semua lini layanan" header chip present.
- **E10 Notifications** — Bell icon on agent AppBar (`CyMUr`); auto-publish banner copy "memicu notifikasi". No actual notification surfaces designed in E4.
- **E1 RBAC/Audit** — Scope visible in screen titles ("Shift Leader" lane, "HR / Super Admin" lane). No audit-trail surfaces in E4 (correct — E1 owns).

---

## 7. Prioritized recommendation

**BLOCKER (must add before v1 implementation can be specced screen-by-screen):**

1. **Shift-picker popover/sheet for cell `+` and existing-chip click** (F4.2 core US "assign a shift"). Must include: service-line filter, list of master shifts (tagged first), "Tandai Libur" option, save behavior. Without this, F4.2 has no interaction.
2. **Cell-edit / cell-clear menu** (F4.4 v1 leader edits). Reuse the shift-picker with current selection + "Clear" + "Mark Off" options.
3. **Conflict-block error state set** — at minimum one canonical "blocked" toast/inline component with copy variants for: not-placed-here, beyond-placement-end, over-approved-leave, scope-violation. Locked decision says "blocked" — design must show it.
4. **Auto-publish success toast** — every cell save / shift save should land somewhere visible (toast or row highlight). Silent saves contradict INV-4 user expectation.

**HIGH (decision-locked features without UI):**

5. **Bulk "Terapkan ke rentang" modal** with date range, shift selector, agent scope, and a per-day skip summary (so SA-1 + leave-block + SA-5 surface).
6. **Edit Shift modal** + **deactivate-confirm** (SM-5 lifecycle has no UI today). Reuse Tambah Shift modal with prefilled state and a "Nonaktifkan" footer action; replace "Hapus" if a referenced template.
7. **Replace-warning confirm** (SA-2) — short dialog when assigning a second shift to a day.
8. **Validation error states in Tambah Shift** (break outside window, duplicate title) — inline field errors using the existing form components.
9. **Agent push-notification / reminder mocks** — at minimum one notification card showing "Jadwal baru: Pagi 07:00–15:00 di Plaza Senayan" + one reminder card "Shift dimulai 1 jam lagi" (SV-5).

**MEDIUM:**

10. **Agent shift-detail screen** (SV-3 site + map) — tappable card → detail with location/geo, contacts.
11. **Empty + loading states** for all 4 screens (Master Shift no-results, agent no-upcoming, leader no-agents).
12. **HR-Admin oversight variant** of Jadwal Mingguan with a company picker (or document that HR reuses the leader screen with a topbar company switcher).
13. **Cross-midnight dual-day display** on the agent list + grid cell (SV-6).

**LOW:**

14. **Leader day view** + **by-agent matrix** view (SV-2). If deferred, mark explicitly in FEATURE.md §7 like the agent swap deferral.
15. **Shift-leader mobile/tablet roster view** (FEATURE.md §6 surface). Defer if web/tablet covers the use; document explicitly.

---

## 8. Notes

- The four designed frames are visually strong and consistent with `DESIGN-SYSTEM.md` (proper use of `$primary-soft` for HR-Admin POV, `$info-bg` for Shift-Leader POV, `$F4E9F5`/`$accent-purple` for Agent POV; semantic tokens `$ok-bg`/`$warn-bg`/`$info-bg`; mono font for times; `+1` badge for midnight-crossing).
- The legend chip "Cuti (terkunci)" + the `warn-bg` lock cells correctly anticipates the E6 integration even though E6 is a separate epic — good cross-epic awareness in the design.
- The AutoPublish banner is a nice "always-on" reassurance, but it is **not** a substitute for a per-save confirmation toast — users still need feedback that *this specific save* succeeded.
- The leader screen hardcodes "Plaza Senayan" in the topbar (`r3qFhF`) and title. For the HR-Admin variant, either add a company-picker dropdown overlay or duplicate the screen with the picker placeholder.
- The decision log (EPICS.md §8 + FEATURE.md §7) is the source of truth: **agent swap UI is correctly absent** (deferred post-v1) — do not add. **Bulk apply-to-range UI is required** (locked in v1) — currently missing the modal.
- Two open items remain in `shift-master-catalog.md` §10 (multiple breaks; 24h shift) — these don't need design until resolved, but note them in the Edit/Tambah Shift modal's "open behaviors" list.
- No node IDs in this audit refer to deleted/renamed nodes as of the read at session start; if the canvas has moved since, re-read `WnejY` before applying any fix.
