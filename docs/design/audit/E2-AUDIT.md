# E2 Identity — Design Audit

> ⚠️ **Superseded findings (2026-06-14, EPICS §8 E11):** the profile **change-request** feature is **removed** — agent edits (phone/emergency/bank/address/photo/language) are now **instant self-edit, no approval**. All findings below about a missing/required *HR change-request approval queue/review screen* (the BLOCKER in §2, items 1/18, and the agent "Status Pengajuan" `SXqA5` flow) are **void** — do not action them. Approval now lives only in **E11** (leave/overtime), with its own screens (see SCREEN-GENERATION-PLAN E11 block).

**Date:** 2026-06-02
**.pen frame:** Z3cS3 ("▦ FEATURE GROUP · E2 Karyawan")
**Specs:** FEATURE.md + 5 PRDs (employee-profile, employment-agreement, client-company-directory, service-lines-positions, operational-master-data)

## 1. Screen inventory

| # | Screen name | Frame ID | Platform | Purpose | Reachable from (within epic) |
|---|---|---|---|---|---|
| 1 | E2 · Karyawan — Daftar (Admin list) | `WElYh` | web | Employee directory (F2.1) — table with stats, filters, tabs (Semua/Aktif/Nonaktif/Tanpa Login). HR/Admin POV. | Sidebar → "Karyawan" |
| 2 | E2 · Karyawan — Detail | `JBjBb` | web | Employee detail with 5 tabs (Profil / Penempatan / Kehadiran / Cuti & Lembur / Dokumen). Profile tab cards: Data Pribadi, Kontak, Statutori & Bank, Akun Login, Ringkasan. | Daftar row click |
| 3 | E2 · Karyawan — Tambah | `h6bDz` | web | Create-employee form, 4 sections: Data Pribadi, Kontak, Statutori & Bank, Akun Login. Footer with Batal / Simpan. | Daftar "Tambah Karyawan" CTA |
| 4 | E2 SL · Karyawan — List (scoped) | `n3wi1w` | web | Shift Leader read-only list (scoped to bound location). Add button disabled. | SL sidebar |
| 5 | E2 SL · Detail (read-only) | `rtKzk` | web | SL read-only detail; Edit button disabled; "Locked — HR only" card stub. | SL list row click |
| 6 | E2 · Aksi, status & alur (POV panel) | `yDXdl` | web (canvas) | Overlays + state tiles + 3 flow strips + RolePanel. Not a navigated screen — it's a brainstorm reference panel. | (not a real screen) |
| 7 | Agen · Profil Saya (read-only) | `s5RO1` | mobile | Agent profile view (Data Pribadi locked, Kontak/Bank chips "Dapat diajukan"), CTA "Ajukan Perubahan Data". BottomNav present. | Mobile bottom nav → Profil |
| 8 | Agen · Ajukan Perubahan | `n465cT` | mobile | Edit-request form (Telepon/Alamat/Bank editable, NIK/Nama lock-grayed). Footer "Kirim Pengajuan". | Profil CTA "Ajukan Perubahan Data" |
| 9 | Agen · Status Pengajuan | `SXqA5` | mobile | Pending change request with old → new diffs; "Riwayat Pengajuan" list (Disetujui, Ditolak). BottomNav. | (entry unclear — see 2.2) |

## 2. Dead-end findings

### 2.1 Unwired clickable components (cat a)

**[BLOCKER]** Screen `WElYh` (E2 · Karyawan — Daftar) — top-band **"Impor"** button (`H1GCO`) has no destination screen, modal, or flow. PRD F2.1 C-5 covers bulk import (migration), and EPICS.md cross-cuts with E9/E10. Expected: an Import overlay/modal (file picker + crosswalk preview + error queue) OR a hidden state explicitly marking it not-implemented. Currently it's a primary visible action with no result. Expected per F2.1 C-5 ("Bulk import on migration").

**[HIGH]** Screen `JBjBb` (E2 · Karyawan — Detail) — header has a kebab/options icon (`Mynkj`, the unlabeled 38×38 box after "Edit Profil") with no menu, no overlay, no flow tile. Standard kebab actions for an employee row/detail per F2.1 EP-7 (deactivate) and EP-3 (provision login later) should hang here. Expected: dropdown menu containing Provisikan Login / Nonaktifkan / Reaktifkan options (flows for these exist in `yDXdl` row 2 & 3 but no trigger is wired on the detail page itself).

**[HIGH]** Screen `JBjBb` (Detail) — tab strip exposes 5 tabs (Profil, Penempatan, Kehadiran, Cuti & Lembur, Dokumen). Only "Profil" content is designed. The other 4 tabs are clickable but lead to undesigned content within E2. Three of them (Penempatan / Kehadiran / Cuti & Lembur) are cross-epic surfaces (E3/E5/E6+E7); "Dokumen" has no PRD reference at all in E2. Expected: at minimum a loading/empty placeholder per design-system "no dead-flow states" rule. See §5 for cross-epic notes.

**[MEDIUM]** Screen `JBjBb` (Detail) — "Edit Profil" button (`wfC8h`) has no edit form or modal. The Tambah form (`h6bDz`) is the create variant; no edit-mode flow tile or screen exists. Expected: an edit screen or modal (PRD F2.1 EP-6/EP-5 distinguish HR-editable vs agent-editable fields).

**[MEDIUM]** Screen `WElYh` (Daftar) — per-row kebab cell (`LFlkK`, `tPL0z`, etc., the c6 cell of every row, width 52) is present on every row but has no menu, no triggers wired. The deactivate / provision flows in `yDXdl` row 2 & 3 say "Menu baris → Nonaktifkan" / "Provisikan login" — implies a row menu. Expected: row-action menu overlay (matches flow tiles in `AjzTR` and `I631t`).

**[MEDIUM]** Screen `n3wi1w` (SL List) — same row kebab cells exist but for SL these should be empty/hidden since SL is read-only (per the RoleNote and disabled Add CTA). Expected: either remove the kebab cell on SL or design a read-only "Lihat detail" affordance.

**[LOW]** Screen `WElYh` (Daftar) — pagination (`MmH2O`) buttons have no hover/disabled states, no jump-to-page overlay. Acceptable for a brainstorm but flagged for completeness.

**[LOW]** Screen `WElYh` — the "Reset" filter button (`GiiY5`) has no visible empty-state binding except via the `aeq2B` empty-state tile in the POV panel. The link between the live screen and that empty state isn't shown.

### 2.2 Orphan screens (cat b)

**[HIGH]** Screen `SXqA5` (Agen · Status Pengajuan) — no clear entry point. The Profil screen `s5RO1` CTA is "Ajukan Perubahan Data" → `n465cT` (Ajukan Perubahan). After submitting on `n465cT` footer "Kirim Pengajuan", there is no result state designed (toast/screen change). The Status Pengajuan screen is logically the destination but no transition/trigger pairs the two. Expected: a flow tile in the POV panel (`yDXdl`) showing Ajukan → Kirim → Status (toast / redirect). PRD F2.1 EP-5 "queued for HR approval" requires a visible queue/status.

**[MEDIUM]** Screen `yDXdl` (E2 · Aksi, status & alur) — this is a useful canvas-only reference panel but is NOT a navigated screen. It sits next to the Admin "ScreensRow" inside the POV line. The four flow tiles inside it (Tambah / Nonaktifkan / Provisikan) are descriptive, but they reference only Admin-side actions and entirely **omit any flow for the Agent change-request approval** (HR side of EP-5) and for the agent's own three-screen flow (s5RO1 → n465cT → SXqA5). Either rename the panel as a brainstorm artifact (label "Reference panel, not navigated") or add the missing flows.

### 2.3 Missing result states (cat c)

**[BLOCKER]** **No HR approval queue / review screen for agent change requests.** PRD F2.1 EP-5: "Agent self-edits…require HR approval before they take effect." The agent side designs the request (`n465cT`) and a self-status view (`SXqA5`), but there is **no admin web screen** for HR to see pending requests, approve, or reject. Without it, the loop is broken and the Disetujui/Ditolak chips on the agent's history have no producer. Expected: an "Antrian Persetujuan Perubahan Data" list / inbox + a review modal with Approve/Reject + reason field.

**[HIGH]** Screen `h6bDz` (Tambah) — no error/validation state designed inline on the form itself. The POV panel `KaUnK` (flow Tambah) step 3 says "Validasi · NIK unik & field wajib dicek inline" but the form sections in the actual screen show no error styling or error toast composition. The Aksi panel has a "NIK sudah terdaftar" toast (`T52Y8P`) but no wiring from the form to that toast. Expected: visible inline error state on the form (matches F2.1 acceptance criterion "Reject duplicate NIK").

**[HIGH]** Screen `h6bDz` (Tambah) — no loading/submitting state designed for the "Simpan Karyawan" CTA. The POV panel only shows step 4 "Tersimpan" toast. Expected: a "Menyimpan…" / disabled-spinner state on the button (mentioned in DESIGN-SYSTEM no-dead-flow rule).

**[HIGH]** Screen `WElYh` (Daftar) — empty-state for filter-no-result exists in the POV panel (`aeq2B`) as a tile, but not as a state shown directly inside the live table card. Expected: design the in-table empty state (the tile is a meta-spec; the actual screen needs to demo it).

**[HIGH]** No "first-load / no employees yet" empty state for the directory. Only the filtered-no-result variant exists. PRD F2.1 EP-7 implies historical retention so it may rarely apply, but DESIGN-SYSTEM still requires it.

**[MEDIUM]** Screen `JBjBb` (Detail) — no result state for "Edit Profil" (no success toast / no edit confirmation). EP-8 audit trail mention has no UI surface either.

**[MEDIUM]** Screen `n465cT` (Agen · Ajukan Perubahan) — no error state (e.g., invalid phone format), no loading state for "Kirim Pengajuan", and no inline confirmation modal "Anda yakin ingin mengirim pengajuan?".

**[MEDIUM]** Screen `s5RO1` (Profil Saya) — no loading / refresh state; no offline state (mobile-relevant).

**[LOW]** Screen `SXqA5` (Status Pengajuan) — no empty state ("Belum ada pengajuan") for an agent who has never submitted a change.

### 2.4 Untriggered overlays (cat d)

**[HIGH]** Overlay `tjkOm` (Nonaktifkan karyawan dialog, child `wlNma`) is designed but **no trigger exists on any actual screen**. The flow tile (`AjzTR`) says "Menu baris → Nonaktifkan" but the row kebab on `WElYh` / `JBjBb` is unwired (see §2.1).

**[HIGH]** Overlay `TCkci` (Provisikan Login modal, child `Q74od`) is designed but no trigger wired on screens. Same root cause as above. Expected: kebab menu on detail (and/or "Provisikan" CTA on the "Akun Login" card on `JBjBb`).

**[MEDIUM]** Toast `g4hNM` "Karyawan tersimpan" + toast `T52Y8P` "NIK sudah terdaftar" exist on the POV tile but are not bound to the create-form screen (`h6bDz`). They're documentation, not wired result states. Expected: appear on the live screen post-action.

**[MEDIUM]** No overlay for **reactivate** an inactive employee (PRD F2.1 C-3). The Nonaktif tab exists, but no Activate dialog.

**[MEDIUM]** No overlay/modal for HR approving/rejecting an agent change request (depends on the BLOCKER above).

**[LOW]** Empty-state tile (`aeq2B`) is shown as a tile preview but not used in the actual table card body.

### 2.5 Dangling back/close (cat e)

**[MEDIUM]** Screen `n465cT` (Agen · Ajukan Perubahan) — the AppBar `NbMZP` has a chevron-left back icon (`M2KA8C`) but no companion close/X icon and no visible discard-confirmation if user has unsaved changes.

**[MEDIUM]** Screen `SXqA5` (Agen · Status Pengajuan) — AppBar `Nd8ac` has chevron-left back (`w295w`), but the screen also has BottomNav, creating two competing navigation models. Expected: clarify whether this is a push view (back-only) or a tab destination (BottomNav). Same minor issue with `n465cT` — it has a footer-Kirim but no BottomNav, which is consistent with push-flow, but Status (`SXqA5`) has BottomNav, which is inconsistent.

**[LOW]** Screens `JBjBb` and `h6bDz` (Detail and Tambah) — have a "Kembali ke Daftar Karyawan" link in BackRow but no breadcrumb-like top-back affordance in the Topbar component; minor duplication question (Topbar already has hierarchy "Direktori · Karyawan / Budi Santoso" or "Tambah").

**[LOW]** Modal `Q74od` (Provisikan Login) has a close X (`CNVFA`) and a Batal button — both lead to same dismissal; OK but worth noting redundancy.

## 3. Missing screens (cat f)

This is the largest finding cluster — **4 of the 5 features in E2 have zero designed screens**.

### 3.1 F2.2 Employment Agreement (PKWT/PKWTT + comp) — entirely missing

**[BLOCKER]** No employment-agreement list screen. PRD F2.2 §4: web console "Create/renew/close agreements". Expected: a list / per-employee tab "Kontrak Kerja" or a top-level "Perjanjian Kerja" directory.

**[BLOCKER]** No agreement create/renew form (PKWT vs PKWTT switch, dates, comp fields). PRD acceptance scenarios "Create a PKWT agreement", "Create an open-ended PKWTT agreement", "Renewal creates a linked successor" all rely on UI that doesn't exist.

**[BLOCKER]** No "active vs historical" agreement view; no successor/predecessor link UI (EA-3).

**[BLOCKER]** No close-agreement flow (EA-5 — reason + effective date; cascade-to-placements warning).

**[HIGH]** No mobile "agreement summary" view for agents (EA-6, PRD §4 mobile surface). The Detail screen's tab strip doesn't even include a "Kontrak" tab — only Profil/Penempatan/Kehadiran/Cuti & Lembur/Dokumen — so agreements have no surface at all.

**[HIGH]** No comp encryption / role-gated reveal UI (EA-4 + EA-7 audit with masked old/new).

### 3.2 F2.3 Client Company Directory — entirely missing

**[BLOCKER]** No client-company list screen. PRD F2.3 §4: HR/Super Admin "Full CRUD".

**[BLOCKER]** No client-company detail screen — and crucially **no `geofence_radius_m` editor** (EPICS.md §8 locked decision, default 100m). This is a locked decision and the only place to set it.

**[BLOCKER]** No client-company create/edit form.

**[HIGH]** No map / lat-lng picker UI for geofence center (PRD CC-1 "geo needed for E5 geofencing"; C-1 "Company without geo → E5 geofencing disabled/flagged").

**[HIGH]** No deactivate-company dialog with "active placements present" warning (CC-5).

**[MEDIUM]** No mobile read-only client-company view for Agent/Shift Leader (PRD §4 mobile surface).

### 3.3 F2.4 Service Lines & Position Master — entirely missing

**[BLOCKER]** No service-line CRUD screen (SP-1 seeded 3 + admin-extendable).

**[BLOCKER]** No position CRUD screen scoped under each service line (SP-2/SP-3 uniqueness).

**[HIGH]** No deactivate-service-line dialog blocking deletion when referenced (SP-1) and corresponding deactivate-position dialog (SP-4).

### 3.4 F2.5 Operational Master Data — entirely missing

**[BLOCKER]** No Leave Types CRUD (LT-1..LT-3).

**[BLOCKER]** No Attendance Codes CRUD (AC-1..AC-3, including color picker, is_billable / needs_verification toggles).

**[BLOCKER]** No Overtime Rules CRUD (OR-1..OR-3, multiplier, min_minutes, service-line scope).

**[HIGH]** No deactivate-with-references warning (MD-1 across all three lists).

### 3.5 Cross-cutting

**[HIGH]** No bulk-import / export UI (PRD F2.1 C-5 references migration import; E10 reporting is cross-cutting). The "Impor" button on Daftar is the only nod and is unwired (see 2.1).

**[MEDIUM]** No audit-log surface for any E2 entity (cross-feature rule §6: "All actions audited"). May be E1's responsibility but no link is exposed.

**[MEDIUM]** No "Provisikan Login" affordance on `JBjBb` for an existing data-only employee (PRD F2.1 C-1). The modal exists in the POV panel (`Q74od`) but has no trigger on the detail page (e.g., the "Akun Login" card on `JBjBb` would naturally host a "Provisikan login" CTA when login is null). Marked HIGH/MEDIUM overlap with §2.1.

## 4. PRD coverage matrix

| PRD | Required screens / states | Designed | Missing |
|---|---|---|---|
| **F2.1 Employee Profile** | List, Detail, Create form, Edit form, Deactivate dialog, Provision-login modal, Reactivate dialog, HR-approval inbox for agent edits, Bulk import; mobile: Profile view, Edit-request form, Status view | List (`WElYh`), Detail (`JBjBb`), Create (`h6bDz`), SL variants (`n3wi1w`, `rtKzk`); overlays designed in POV panel only (deactivate `wlNma`, provision `Q74od`); mobile (`s5RO1`, `n465cT`, `SXqA5`) | Edit form, Reactivate, **HR approval queue/review screen for agent change requests** (EP-5), Bulk-import surface, Triggers wiring overlays to live screens, In-screen empty/error/loading states |
| **F2.2 Employment Agreement** | List/per-employee tab, Create/renew form (PKWT vs PKWTT), Close form, Successor link UI, Comp encryption reveal, Audit; mobile summary | None | **Everything** (no PKWT/PKWTT screens at all) |
| **F2.3 Client Company Directory** | List, Detail, Create/edit form, **Geofence radius editor + map**, Deactivate-with-placements warning; mobile read-only | None | **Everything** (incl. the locked-decision `geofence_radius_m` UI) |
| **F2.4 Service Lines & Positions** | Service-line CRUD list, Position CRUD scoped under line, Deactivate dialogs | None | **Everything** |
| **F2.5 Operational Master Data** | Leave types CRUD, Attendance codes CRUD (with color & boolean toggles), Overtime rules CRUD (with multiplier, scope), Deactivate-with-references warning | None | **Everything** |

## 5. Cross-epic references found

- **Detail screen `JBjBb` tab strip** lists `Penempatan` (E3 placement history), `Kehadiran` (E5 attendance), `Cuti & Lembur` (E6 leave + E7 overtime), `Dokumen` (no PRD). Each is a click-through to other epics' surfaces. Verify the target screens exist in E3/E5/E6/E7 frames; if they don't, those become E2 dead-ends.
- The subrow under HeaderCard on `JBjBb` shows "Petugas Parkir · ● Parking · Plaza Senayan" — this depends on F2.4 (position + service line) AND F2.3 (client company) AND E3 (placement). The data is fabricated in the design — verify it resolves once those entities/screens are designed.
- "Plaza Senayan" reference depends on F2.3 client-company directory (which is unbuilt — see §3.2).
- Subrow position "Petugas Parkir" / service line "Parking" depend on F2.4 (which is unbuilt — see §3.3).
- The placement-area "Penempatan" column in the Daftar table (`avdwh`, `K31uHh`, etc.) depends on E3 — verify E3 list/detail screens exist; if not, the placement linking is dangling.
- The Shift Leader scoping note "Lokasi binaan: Plaza Indonesia" presumes E1 (RBAC scope to a client company) — verify that scoping mechanism is documented in E1.

## 6. Prioritized recommendation

### BLOCKERS (block US-# or locked decisions)

1. **Design the HR approval queue/review screen for agent change requests** (PRD F2.1 EP-5). Without it, the loop from `n465cT` → `SXqA5` Disetujui/Ditolak has no producer.
2. **Design F2.2 Employment Agreement screens** end-to-end: list, create (PKWT/PKWTT), renew (successor), close (with reason + cascade-to-placement warning). Add a "Kontrak Kerja" tab on `JBjBb`.
3. **Design F2.3 Client Company Directory** screens incl. `geofence_radius_m` editor + lat/lng map picker. Locked decision in EPICS.md §8.
4. **Design F2.4 Service Lines & Positions** CRUD (service-line list + nested position list per line).
5. **Design F2.5 Operational Master Data** CRUDs: Leave Types, Attendance Codes (with color & flags), Overtime Rules.
6. **Wire row-kebab on `WElYh` to a menu overlay** that exposes Deactivate (`wlNma`) and Provision Login (`Q74od`). Mirror on `JBjBb` detail header.
7. **Design/wire the "Impor" button on `WElYh`** OR remove it until E9/E10 work brings the import surface.

### HIGH (missing key error/empty/loading or required role surface)

8. Wire the 4 undesigned tabs on `JBjBb` (Penempatan / Kehadiran / Cuti & Lembur / Dokumen) — either point to E3/E5/E6 placeholders or stub a per-tab empty state.
9. Design inline validation + loading state on `h6bDz` (NIK uniqueness, required-field highlight, "Menyimpan…" CTA state).
10. Design empty + loading states in the live `WElYh` table card body (move `aeq2B` from POV-tile into the actual screen).
11. Design "Edit Profil" overlay/screen from `JBjBb` (HR-only fields vs agent-editable distinction).
12. Add a mobile agreement-summary surface on the agent profile (EA-6, F2.2 §4).
13. Add Provision-Login CTA on the "Akun Login" card of `JBjBb` for null-login employees (PRD F2.1 C-1).

### MEDIUM (edge cases C-#)

14. Add Reactivate dialog for `JBjBb` when status is Nonaktif (C-3).
15. Add SL row-kebab cleanup (remove or design read-only action) on `n3wi1w`.
16. Add post-submit toast/redirect after `n465cT` "Kirim Pengajuan" to `SXqA5` (currently no transition shown).
17. Resolve `SXqA5` navigation ambiguity (BottomNav + back arrow on a push-flow screen).
18. Add an HR review modal for an individual agent change request (paired with item 1).
19. Add export and audit-log surfaces (cross-feature rule).

### LOW (cosmetic)

20. Add pagination disabled / hover states.
21. Add "no submissions yet" empty state on `SXqA5`.
22. Resolve Topbar breadcrumb vs BackRow redundancy on `JBjBb` / `h6bDz`.

## 7. Notes

- The `yDXdl` "Aksi, status & alur" panel is a useful brainstorm reference but should not be confused with the wired flow. It documents intentions (overlays + tile states + flow strips for Tambah / Nonaktifkan / Provisikan Login) but does NOT itself attach those overlays to triggers on the live screens (`WElYh`, `JBjBb`, `h6bDz`). The audit treats the panel as evidence of intent, not as a delivered design.
- The Admin POV is comparatively rich for F2.1 (3 web screens + mobile trio), but the four other features (F2.2–F2.5) are wholly undesigned — this is the dominant gap in E2 and is what most BLOCKERs above address.
- The Shift Leader read-only variants (`n3wi1w`, `rtKzk`) are well-conceived but only exist for the Employee surfaces; they do not cover the other E2 master-data lookups, though SL is consumer-read per cross-feature rules so this is acceptable for now.
- The locked decision `geofence_radius_m` on ClientCompany (EPICS.md §8, resolved 2026-05-29) is the single most concrete decision with **zero design surface** — flag this to product as priority 1 after the F2.1 approval-queue gap.
- All overlays designed for E2 live inside the canvas-only `yDXdl` panel as preview tiles rather than as separately-instantiated overlay frames; whether they should be promoted to root-level reusable overlay components (alongside DS · C · Overlays) is a Design-System curation question, not an E2 blocker.
