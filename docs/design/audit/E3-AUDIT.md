# E3 Placement — Design Audit

**Date:** 2026-06-02
**.pen frame:** zTfKp (▦ FEATURE GROUP · E3 Penempatan)
**Specs:** FEATURE.md + 5 PRDs (agent-placement, placement-lifecycle, replacement-transfer, shift-leader-assignment, company-roster)

---

## 1. Screen inventory

| # | Screen name | Frame ID | Platform | Purpose | Reachable from |
|---|---|---|---|---|---|
| 1 | E3 · Penempatan — Perusahaan (Companies list / index) | `C2SSLA` | Web · HR Admin | Top-level company grid w/ stats (Active clients, active placements, expiring, no-leader). Each company card shows lines, agent count, leader chip. | Sidebar → Penempatan |
| 2 | E3 · Roster — Plaza Senayan | `nLN4d` | Web · HR Admin | Per-company placement roster table with filters + history toggle, company header w/ leader, "Buat Penempatan" CTA, "Ganti" leader. | Company card click; "Buat Penempatan" CTA |
| 3 | E3 · Buat Penempatan (Create form) | `g3OzZz` | Web · HR Admin | Form: Agent & Employment Agreement → Placement (company/line/position) → Period & Terms (dates, leave entitlement, salary ref, notes). Footer: Batal / Simpan. | "Buat Penempatan" CTA (from companies list or roster) |
| 4 | E3 · Detail Penempatan | `pFR79` | Web · HR Admin | Single placement detail. Header w/ agent + actions [Perpanjang / Transfer / Akhiri]. Lifecycle tracker (Draft → Aktif → Akan berakhir → Berakhir). Left col: Detail + Riwayat chain. Right col: Perjanjian Kerja + Shift leader card. | (Implicit) roster row click — but no row deep-link explicitly wired |
| 5 | E3 SL · Roster (read-only) | `o5Txgg` | Web · Shift Leader | SL-scoped roster (single company, no create/leader-change actions; Buat Penempatan and Ganti are `enabled:false`). | Sidebar → Penempatan (auto-scoped) |
| 6 | Agen · Penempatan Saya | `mqGEi` | Mobile · Agent | Agent's own active placement (period, leave entitlement, leader name, expiry warn) + historical entries (Superseded, Transfer). | Bottom nav · Beranda |

---

## 2. Dead-end findings

### 2.1 Unwired clickable components (cat a)

| # | Element | Location | Severity | Note |
|---|---|---|---|---|
| a-1 | Row kebab menu (`ellipsis-vertical`) icon on every roster row (`I6ZLD6`, `z9hT0p`) | Roster + SL Roster row c5 | **HIGH** | Per-row kebab present but **no row-action menu/popover designed** anywhere in E3. Menu component does exist generically (DS · C `Yavck`), but no E3 instance defining its items (View detail / Renew / Transfer / End / Reassign as leader). |
| a-2 | "Perpanjang" (Renew) button `FoXSn` on Detail header | Detail · Header actions | **BLOCKER** | No renewal modal/form designed. PRD F3.2 LC-7 (Renewal creates successor with `predecessor_id`) requires its own period-input flow + 1-day buffer validation. Button has no result state. |
| a-3 | "Transfer" button `EP67A` on Detail header | Detail · Header actions | **BLOCKER** | No transfer form/modal anywhere. F3.3 is an entire epic feature (pick new company + service line + position + period) and is **wholly undesigned**. Critical dead-end. |
| a-4 | "Akhiri" (End) button `YTjbq` on Detail header | Detail · Header actions | **BLOCKER** | No confirm-end dialog. F3.2 LC-5 requires reason + effective date input → `Terminated`. Generic confirm dialogs exist in DS · C (`IY3vg`) but no E3 instance. |
| a-5 | "Ganti" (Reassign leader) button `FnHo7` on Company header | Roster · CompanyHeader leader card | **BLOCKER** | F3.4 Shift-Leader Assignment is the entirety of one PRD — no Assign/Reassign Leader screen or modal designed (picker for active agents at this company, with INV-3 + INV-4 enforcement). |
| a-6 | Pagination buttons (`t26CE`, `aCFH1`, page chips `xrSj7/W0lEm/y3b4V2`) | Roster · Pagination `bCAFU` | LOW | Implicit standard pagination; acceptable, but no hover/active design surfaced beyond the lone active chip. |
| a-7 | Company card chevron-right (`rL7Xq`, `U135b`, `j57S4Q`, `Is91K`, `KbbQW`, `ffg4B`) | Companies list · CompanyGrid cards | MEDIUM | Cards visually clickable (chevron-right affordance) but no hover/pressed state, and link target (Roster) is correctly implied. |
| a-8 | History chain entries `OZIVc`, `d0wSRo`, `sUlul` on Detail · Riwayat | Detail · Left col Riwayat | MEDIUM | Visually card-like history items; not clear whether they are clickable to navigate to the predecessor's placement detail. No hover state, no chevron. |
| a-9 | Service-line chips on company card (`hiSwV`, `yEVFD`, …) | Companies list cards | LOW | Chips look filterable but no chip-click filter behavior designed. |
| a-10 | "Sertakan riwayat" Toggle (`k6xus` HR · `Pooii` SL) | Roster filters | LOW | Toggle visible. No designed "active + history visible" state of the table. |
| a-11 | TopBar breadcrumb segment "Penempatan" (`l172b`) inside `caFkE` | All HR screens topbar | LOW | Crumb implies navigation up to a global E3 root, which matches screen #1. Acceptable. |
| a-12 | Mobile bell icon `o5M3vq` | Mobile · AppBar | LOW | No notification panel designed for mobile; cross-epic (E10) but worth flagging. |
| a-13 | "Preview" pill `eNTIp` on Buat Penempatan title | Create form · TitleBand | MEDIUM | Pill suggests a "Preview" interaction (likely opens a preview drawer showing the resolved placement before save). No preview drawer/state designed. |

### 2.2 Orphan screens (cat b)

None. All 6 screens are reachable from sidebar / row click / button click (with caveats — see cat e on the row→detail edge).

### 2.3 Missing result states (cat c)

| # | Action | Expected result state | Severity | Status |
|---|---|---|---|---|
| c-1 | Save placement (`lvLMR` "Simpan Penempatan") | Success toast + redirect to detail; on failure → field errors & banner | **HIGH** | No success toast/error variant of the form designed. DS · C has generic Toast (`tplBu`) but no F3.1 instance. |
| c-2 | "Block: agent already has an active placement" (F3.1 BR-2 / Gherkin block-double-booking) | Inline error on agent field + offer to End/Transfer | **BLOCKER** | The "AgentNote" green confirmation (`ZU0v2`) is the *only* designed state — no error variant ("Agent sudah memiliki penempatan aktif. Akhiri / Transfer"). |
| c-3 | Inactive client company (F3.1 BR-3) | Block + message "Company is not active" | **HIGH** | Not designed. |
| c-4 | 1-day buffer violation (F3.1 BR-2 / Gherkin same-day handover) | Date-field error w/ "earliest allowed start" | **HIGH** | The neutral info-tone `KkMG9` mentions auto-cap but no error/red-state of the date fields. |
| c-5 | PKWT auto-cap notice on save (F3.1 BR-1b) | Toast "End date adjusted to agreement end (31 Des 2026)" | MEDIUM | Notice copy is present pre-save (`LyJgz`) but no post-save acknowledgment toast designed. |
| c-6 | "Company has no leader" warning (F3.1 BR-8) | Inline banner on save / Detail | MEDIUM | Companies list has a "Tanpa shift leader" status pill on Menara BCA (`h22BP`) — good — but no in-form warning at create-time, and no Detail-level warning banner. |
| c-7 | Backdating reason required (F3.1 BR-6) | Required textarea reveal when start_date < today | **HIGH** | Footer only has a static hint string `weOEl`. No conditional `backdate_reason` field. |
| c-8 | Empty roster (F3.5 C-7) | Empty state w/ CTA "Create first placement" | MEDIUM | Not designed; Plaza Senayan only shows the populated state. |
| c-9 | Loading state for roster / save | Loading overlay or skeleton | MEDIUM | DS · C has generic Loading overlay (`sdnXZ`) but no E3 instance binding it. |
| c-10 | Terminal-state placement detail (Ended / Terminated / Resigned / Superseded) | Read-only state with disabled actions | **HIGH** | Detail is only designed for Active+Expiring. PRD F3.2 LC-1 marks terminal states immutable (Super Admin override only) — that disabled state is undesigned. |
| c-11 | "Scheduled" status placement detail | Variant w/ "Activates on …" banner | MEDIUM | Lifecycle tracker shows Draft → Aktif as the only highlighted path; no Scheduled-as-current variant. |
| c-12 | Mobile · no active placement | Empty/standby state for agent | LOW | Only the "has placement" variant exists. |
| c-13 | Mobile · transfer/expiry notifications detail | Pushed-from-notification state | LOW | Cross-epic E10. |

### 2.4 Untriggered overlays (cat d)

No E3-specific overlays exist at all (no modal, confirm, drawer, popover instance attached to E3 screens). All the dead-end buttons (`Perpanjang`, `Transfer`, `Akhiri`, `Ganti`) would map onto DS · C overlay templates, but none of those mappings are instantiated.

| # | Required overlay | Maps to DS · C template | Severity | Designed? |
|---|---|---|---|---|
| d-1 | Renew placement (modal, form) | `s7aRM6` Modal — form | BLOCKER | No |
| d-2 | Transfer placement (modal, form — multi-section) | `s7aRM6` Modal — form | BLOCKER | No |
| d-3 | End / Terminate placement (confirm + reason) | `IY3vg` Confirm dialogs | BLOCKER | No |
| d-4 | Assign/Reassign Shift Leader (modal w/ picker of agents placed at this company) | `s7aRM6` Modal — form | BLOCKER | No |
| d-5 | Row-kebab popover menu (per-row actions) | `Yavck` Menus & popovers | HIGH | No |
| d-6 | Save success / failure Toast | `tplBu` Toasts | HIGH | No instance |
| d-7 | Loading overlay (save submitting / roster loading) | `sdnXZ` Loading | MEDIUM | No instance |
| d-8 | Page-level banner ("This company has no leader — assign one") | `pkzVy` Banners | MEDIUM | No instance |

### 2.5 Dangling back/close (cat e)

| # | Element | Severity | Note |
|---|---|---|---|
| e-1 | Roster row → Detail navigation | HIGH | Detail screen `pFR79` exists, but no explicit clickable affordance on the row drives there (kebab `Yrjyg` is unwired; row is not styled as a link). The link from Roster to Detail is implicit. |
| e-2 | Create form "Batal" (Cancel) `uHdtV` | LOW | Cancel button exists but no destination; assumed back to companies list. No confirm-discard dialog when form is dirty. |
| e-3 | Detail screen back / close affordance | MEDIUM | Topbar shows breadcrumb ("Penempatan" → "Budi Santoso") implying back-up nav, but there is no explicit Back arrow on the Detail screen. |
| e-4 | Companies list "Buat Penempatan" `LLRBD` vs Roster "Buat Penempatan" `AdRjE` | LOW | Two entry points to the same form. Companies-list version has no company pre-selected; this is correctly modeled in the form (the company is a form field), but no "pre-selected from roster" variant of the form exists. |
| e-5 | History chain entries on Detail (`OZIVc`, `d0wSRo`, `sUlul`) | MEDIUM | No back-navigation/forward navigation between chained placements is designed. |

---

## 3. Missing screens (cat f)

| # | Missing artifact | Specs covered | Severity |
|---|---|---|---|
| f-1 | **Transfer Placement** form/modal (F3.3 full PRD) | TR-1..TR-9 — pick new company/line/position/period + warn-on-no-leader + atomic vacate-leader path | **BLOCKER** |
| f-2 | **Renew Placement** form/modal (F3.2 LC-7) | Predecessor → successor creation, 1-day buffer validation, PKWT auto-cap re-evaluation | **BLOCKER** |
| f-3 | **End / Terminate Placement** confirm + reason form (F3.2 LC-5) | Reason text, effective date, blocks if leader (triggers SL-6 vacancy) | **BLOCKER** |
| f-4 | **Record Resignation** (F3.2 LC-6) | `resign_at`, immediate or future-dated; closes placement, may close employment | HIGH |
| f-5 | **Assign / Reassign Shift Leader** picker modal (F3.4 full PRD) | Pick candidate from active agents placed at company; block INV-3/INV-4; ends previous assignment atomically | **BLOCKER** |
| f-6 | **Shift Leader vacancy state** on Company header + roster (F3.4 SL-7) | No-leader badge + "Tetapkan" CTA + escalation note ("Persetujuan dialihkan ke HR") | HIGH |
| f-7 | **Roster empty state** (F3.5 C-7) | "Belum ada penempatan — Buat Penempatan" with link | MEDIUM |
| f-8 | **Roster — history-visible state** (F3.5 RO-2) | Toggle ON: shows Ended / Terminated / Resigned / Transferred / Superseded rows w/ distinct status pills | MEDIUM |
| f-9 | **Form error states** (F3.1 BR-2/3/4/6) | Overlap error, inactive-company error, end<start error, backdating-without-reason error | HIGH |
| f-10 | **Backdating reason field** (F3.1 BR-6) | Conditional textarea revealed when start_date < today | HIGH |
| f-11 | **Terminal / Scheduled placement Detail variants** (F3.2 LC-1) | Disabled actions, lifecycle tracker dimmed past current state, banners for resign/transfer reason | HIGH |
| f-12 | **Mobile · No placement / Scheduled placement** (US-4) | "Belum ditempatkan" / "Akan dimulai pada …" empty/scheduled states | MEDIUM |
| f-13 | **Mobile · Placement detail / history** | Currently history items are flat cards on the home screen — no tap-to-expand state | LOW |
| f-14 | **Notification-driven Mobile screens** (E10 dependency, F3.1 BR-7) | Activation, expiring, transfer notifications viewed | LOW (cross-epic) |
| f-15 | **Super Admin override state** for terminal placements (F3.2 LC-1) | "Override" affordance gated by role | LOW |

---

## 4. PRD coverage matrix

| PRD | Required screens/states | Designed | Missing |
|---|---|---|---|
| **F3.1 Agent Placement (create & activate)** | Create form; success path; overlap-error; inactive-company error; end-before-start error; same-day-handover error; PKWT auto-cap notice; PKWTT open-ended; backdating-with-reason; no-leader warning | Create form (`g3OzZz`), auto-cap info note (`KkMG9`), agent-OK note (`ZU0v2`) | Error variants (c-2/c-3/c-4), backdating reason field (c-7), no-leader inline warn at create, save toast (c-1), preview drawer (a-13) |
| **F3.2 Lifecycle & Status** | State machine UI surface on Detail; Renew form; Terminate form; Resignation form; expiring banner; terminal-state read-only variant; Scheduled variant | Lifecycle tracker (`AiyOw`), expiring warn pill (`EAbcx`/`bf9vD`), action buttons present | Renew modal (f-2), Terminate confirm+reason (f-3), Resign form (f-4), terminal-state variant (c-10), Scheduled variant (c-11) |
| **F3.3 Replacement & Transfer** | Transfer form; atomic-failure error; leader-vacate cascade; warn-on-no-leader-at-destination; history chain visibility | Transfer button (`EP67A`) — *only* | Entire flow (f-1). History chain *is* shown on Detail (`uH4M7`) and Mobile, but transfer-as-an-action is undesigned |
| **F3.4 Shift-Leader Assignment** | Assign picker; Reassign confirm; Vacancy state + escalation; SL-2/SL-3 block messages; auto-vacate audit display | "Ganti" button + leader card on company header; "Tanpa shift leader" stat card (`p1EtPd`); no-leader warn pill (`h22BP`) on Menara BCA | Assign/Reassign modal (f-5), no-leader page banner & CTA on roster (f-6), block messages, auto-vacate notification UI |
| **F3.5 Company Roster** | Roster table w/ filters; service-line / status / period filters; include-history toggle; empty state; export action; SL-scoped variant | Roster (`nLN4d`), filters (`KEdDw`), history toggle (`k6xus`), export button (`lTTzy`), SL read-only variant (`o5Txgg`) — well covered | Empty state (f-7), history-visible state (f-8), export confirm/toast |

---

## 5. Invariant enforcement check

| Invariant | UI surface | Enforced / Error state designed? |
|---|---|---|
| **INV-1** — agent has at most one *active* placement | Create form Agent box (`F4iHL`); success/conflict note (`ZU0v2`) | **Partial.** Only the green "valid for placement" state is drawn. The blocking error variant (Gherkin: "Block double-booking") is not designed (c-2). |
| **INV-2** — exactly one shift leader per company w/ active placements | Company header leader card (`ppe9B`); "Tanpa shift leader" stat (`p1EtPd`); warn pill on Menara BCA (`h22BP`) | **Partial.** The "has leader" and "no leader" passive states exist on the Companies grid. The active reassign flow that maintains the invariant (Assign Leader modal, ending previous on reassign) is **not** designed (f-5). |
| **INV-3** — shift leader leads exactly one company | (none) | **Not enforced in UI.** No "Candidate already leads another company" block message exists; this would live inside the missing Assign Leader modal (f-5). |
| **INV-4** — designated leader must be actively placed at that company | (none) | **Not enforced in UI.** Picker candidate list (placed-here-only) is undesigned (f-5). The "Block: must be placed here" error state from PRD F3.4 C1 is missing. |
| **F3.1 BR-2** — 1-day buffer / no overlap on persist | Date fields (`G0r7Ir` start / `e6Id4` end) | Date fields present; **no error variant** for buffer violation (c-4). |
| **F3.2 LC-1** — terminal states immutable | Detail action bar (`waTmo`) | Buttons always shown enabled; no "Ended/Terminated/Resigned" variant disabling them (c-10). |
| **F3.2 LC-3** — Expiring 30 days before end | Detail expiry warn (`EAbcx`/`bf9vD`); Lifecycle tracker dot (`GzYDQ`) | **Designed**. The 47-day-remaining example shows the Expiring affordance well. |
| **F3.5 RO-4** — SL sees only own company | SL roster (`o5Txgg`) has Buat Penempatan + Ganti disabled (`enabled:false`) | **Designed** for the read-only scope, but the "deep-link to other company → 403" error state (PRD C-4) is undesigned. |

---

## 6. Cross-epic references found

- **E2 (Master data) — implicit:** Create form pulls Agent (`F4iHL`), Employment Agreement (`LI2Vx`), Client Company (`KOVdv`), Service Line (`R1Vhjt`), Position (`k5dIb`). These are presented as plain text-fields without picker affordances; the picker overlays (which presumably live in E2) are referenced but not visually instanced here.
- **E4 (Scheduling):** None visible. Detail page doesn't link to "View this agent's schedule" or "Roster's shift schedule" — a likely cross-link given the PRD F3.5 row actions intent.
- **E5 (Attendance):** None visible. No "Attendance summary" widget on Detail; no per-row "Today's status" on Roster.
- **E6/E7 (Leave/OT):** None visible on Detail. PRD F3.4 SL-7 (escalation to HR when no leader) has no UI affordance here — must surface in E6/E7 approval flows.
- **E8 (Payroll):** `base_salary_ref` form field (`CQxMl`) reflects E8 dependency (read-only). No cross-link to payroll history from Detail.
- **E9 (Migration):** Riwayat chain (`uH4M7`) and Mobile history (`TUUA7`, `aANuj`) imply migrated historical placements display. No "Imported from legacy" badge designed (legacy `placement` text-string heritage is invisible).
- **E10 (Notifications + Export):** Export button `lTTzy` present on company header (F3.5 export). Bell `o5M3vq` on mobile is the only notifications affordance and has no destination.
- **E1 (Audit/RBAC):** Footer hint "Backdating diizinkan dengan alasan (tercatat di audit)" `weOEl` is the only explicit audit-log reference. No audit-trail viewer linked from Detail.

---

## 7. Prioritized recommendation

**P0 — BLOCKERS (ship-stops; without these the epic's core verbs dead-end):**
1. **Transfer Placement modal** (f-1 / a-3 / d-2) — entire PRD F3.3 has zero visual surface beyond a button.
2. **Renew Placement modal** (f-2 / a-2 / d-1) — required for the lifecycle's most common transition.
3. **End / Terminate confirm + reason dialog** (f-3 / a-4 / d-3) — PRD F3.2 LC-5.
4. **Assign / Reassign Shift Leader modal** (f-5 / a-5 / d-4) — INV-2/3/4 are unenforceable in UI without this.
5. **Roster row → Detail navigation + row-kebab popover** (a-1 / e-1 / d-5) — the per-row actions (Detail, Renew, Transfer, End, Make leader) need a wired entry point.
6. **Overlap-error variant of Create form** (c-2) — INV-1 protection.

**P1 — HIGH (ship-blockers for the polish bar in DESIGN-SYSTEM.md "no dead-flow states"):**
7. Save success/failure toasts + form error variants (c-1, c-3, c-4, c-7, f-9, f-10).
8. Terminal-state Detail variant (c-10 / f-11) — Ended / Terminated / Resigned / Superseded / Transferred + disabled actions.
9. Shift-leader vacancy state on company header + roster (f-6) — currently only as a stat-card chip on Menara BCA, never on a roster.
10. Record Resignation form (f-4).
11. Roster pagination & loading states (c-9, c-12, mobile empty).

**P2 — MEDIUM:**
12. Roster empty state (f-7) and history-visible state (f-8).
13. Scheduled-status placement Detail variant (c-11).
14. Preview drawer on Create form (a-13).
15. History-chain navigation between linked placements (e-5 / a-8).
16. Mobile no-placement / scheduled variants (f-12).

**P3 — LOW:**
17. Service-line chip filter on companies grid (a-9).
18. Hover/active states for company cards, pagination chips, toggles (a-6, a-7, a-10).
19. Detail back-affordance (e-3), cancel-discard confirm on Create (e-2).
20. Super Admin override state on terminal placements (f-15).

---

## 8. Notes

- **Strengths:** The Detail screen's lifecycle tracker (`AiyOw`), expiring warn pill, and history chain (`uH4M7`) are well-considered and faithfully render F3.2's state machine. The Companies grid is structurally sound — stats, leader chips, no-leader warn (`h22BP`) — and previews INV-2 at a glance. The SL read-only variant (`o5Txgg`) correctly disables Create + Reassign via `enabled:false` overrides — clean approach to RBAC scoping.
- **Brand-token compliance:** Status colors largely follow DESIGN-SYSTEM.md §2 (Aktif → `$ok-*` teal, Akan berakhir → `$warn-*`, Transfer → `$orange-*`, Superseded → neutral). No raw hex misuses spotted in E3 frames.
- **Structural gap:** The .pen file has *no* E3-specific overlay instances at all. DS · C provides all the templates (Modal-form, Confirm, Drawer, Toast, Banner, Menu, Loading) — they're ready to be instantiated. The audit's BLOCKERs all reduce to "instantiate these templates and wire them to the existing buttons."
- **Detail screen entry point:** The path Roster → Detail is the most-used navigation in this epic and is currently implicit. Either the entire row should be clickable (chevron-right at end, like the companies grid) **or** a kebab popover should be designed; the lone unwired kebab today is the worst of both worlds.
- **PRDs are stable**, decisions in EPICS.md §8 / FEATURE.md §7 are all resolved; this is a design-coverage gap, not a spec gap. No PRD edits are needed before closing the BLOCKERs.
- **Pencil session note:** schema fetched once; 6 batch_get calls; no edits, no screenshots.
