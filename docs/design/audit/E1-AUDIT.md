# E1 Foundations — Design Audit

**Date:** 2026-06-02
**.pen frame:** N08erR
**Specs:** FEATURE.md + 4 PRDs (authentication, rbac-roles, audit-log, platform-conventions)

## 1. Screen inventory

| # | Screen name | Frame ID | Platform | Purpose | Reachable from (within epic) |
|---|---|---|---|---|---|
| 1 | Login (split brand panel) | `lKRjr` | Web · all roles | Email + password login + "Tetap masuk" + "Lupa kata sandi?" | Entry point (pre-auth) |
| 2 | Pengguna & Peran | `kHNWT` | Web · super_admin/hr_admin | RBAC user list (Tambah / search / filter peran + status / table w/ kebab) | Sidebar > Pengaturan > Pengguna & Peran (breadcrumb only) |
| 3 | Audit Log | `rtJRB` | Web · super_admin/hr_admin | Immutable audit history (search, entity/action/date filters, Ekspor, 6 sample rows + counter) | Sidebar > Pengaturan > Audit Log (breadcrumb only) |
| 4 | Pengaturan (Umum) | `m3sWh` | Web · super_admin | Localization / Security / Role-nav / About (4 cards) | Sidebar > Pengaturan (sidebar footer item) |
| 5 | Login (mobile) | `Y09E0` | Mobile · all roles | Email + password login + "Lupa kata sandi?" — no "Tetap masuk" toggle | Entry point (pre-auth) |

**Total designed screens in E1: 5** (4 web, 1 mobile). FeatureBanner = `trrya`. Web platform sub-group = `teUIY`, Mobile sub-group = `tQ8ei`.

## 2. Dead-end findings

### 2.1 Unwired clickable components (cat a)

- **[BLOCKER]** Screen `lKRjr` Login web (and `Y09E0` Login mobile) — button **"Lupa kata sandi?"** has no destination. No password-reset request screen, no "email sent" confirmation screen, no "set new password" screen exists anywhere in E1. Required by AU-4 (BR) and Gherkin scenario "Password reset" in `authentication.md` §7. Expected: a `Lupa Kata Sandi` request screen → toast/confirmation → `Reset Password` screen with new-password form → success toast → back to login.

- **[BLOCKER]** Screen `kHNWT` Pengguna & Peran — **"Tambah Pengguna"** button (`Em2E8`, BtnPrimary instance) has no destination. No "Tambah Pengguna" form modal/drawer/screen exists. Required by F1.2 RBAC (creating a user with role) and FEATURE §7 decision "super_admin + hr_admin may assign roles". Expected: form modal with email, role (4 options), employee FK (E2), initial status; success toast.

- **[BLOCKER]** Screen `kHNWT` Pengguna & Peran — every row has a **kebab/ellipsis icon** (e.g. `q3YG0` inside `iQJRp`) on each of 5 user rows but no menu popover designed for that action set. The kebab implies row actions (Edit, Ubah peran, Nonaktifkan, Reset password) — none of these are wired. Per RB-6 "Role assignment/changes are restricted (super_admin; hr_admin per policy) and audited" the role-change UI must exist somewhere.

- **[HIGH]** Screen `kHNWT` Pengguna & Peran — the **"Plaza Senayan"** text in the Cakupan column (`gDMfA` inside `LSPO8`) is rendered in green/primary indicating it's a link. No annotation indicates the link target. Likely a cross-epic E2/E3 navigation (company detail) — should be marked as cross-epic in the file or annotated.

- **[HIGH]** Screen `rtJRB` Audit Log — each table row has no chevron or expansion affordance, yet AL-2 requires `before` / `after` JSON which is shown summarized in the **Perubahan** column (e.g. `status: Menunggu → Disetujui`). To inspect full before/after JSON (required by AL-5 search by entity), a row-click → detail drawer (showing full JSON, masking sensitive AL-4) is the natural expectation per design-system §8.5 ("Table row click → navigate to Detail"). No such drawer is designed.

- **[MEDIUM]** Screen `m3sWh` Pengaturan — multiple rows display **chevron-right** icons indicating they navigate somewhere, but no destination is designed for any of them:
  - "Format Tanggal" → `LOMdO` chevron
  - "Mata Uang" → `M2gTw4` chevron
  - "Versi" → `l6iTcY` chevron
  - "Stack" → `m0NkA` chevron
  - "Sumber data lama" → `avtLr` chevron
  Per PC-1, PC-2, PC-5 these are locked-in v1 defaults (Bahasa Indonesia / Asia/Jakarta / IDR), so the chevrons either should not be there (cosmetic) or should lead to read-only drawers. The "Tentang" section chevrons (Versi/Stack/Sumber data) similarly imply navigation without target. Recommend: drop the chevrons OR design a generic info drawer.

- **[MEDIUM]** Screen `m3sWh` Pengaturan — the **"Tetap masuk di mobile (refresh token)"** Toggle (right side of Keamanan card) has no associated save/confirm flow. Toggling it should generate a toast (per design-system §6) — flow is implicit.

- **[LOW]** Screen `lKRjr` / `Y09E0` Login — eye/show-password icons (visible in the password TextField) have no documented toggle state.

### 2.2 Orphan screens (cat b)

- **[LOW]** No orphans strictly inside E1 — all 5 screens are reachable from a documented entry (Login is the entry; Pengguna/Audit/Pengaturan are sidebar-nav-reachable via the standard shell). However, this needs cross-epic verification:
  - Pengguna & Peran (`kHNWT`), Audit Log (`rtJRB`), and Pengaturan (`m3sWh`) are reached via the **sidebar footer "Pengaturan"** item (see footer item visible at bottom of left sidebar) — but the sidebar (`comp/Sidebar` in `fJVJR`) is shared across all features, and its only documented active states are for Dashboard / Karyawan / Penempatan / Jadwal Shift / Kehadiran / Cuti / Lembur / Laporan. The "Pengaturan" footer item (`EqHHS`) exists in the master sidebar but there is no entry pattern showing how the user opens "Pengguna & Peran" vs "Audit Log" from "Pengaturan" — no settings hub / sub-menu / tabbed parent screen exists.

### 2.3 Missing result states (cat c)

- **[BLOCKER]** No **failed-login** state on `lKRjr` / `Y09E0`. AU-5 (lockout) and Gherkin "Wrong password is rate-limited" require: wrong-password error inline + lockout banner ("akun dikunci sementara — coba lagi dalam X menit"). The footer caption "Setelah 5 kali gagal, akun dikunci sementara" is informational — but the actual rate-limit/lockout UI state is not designed.

- **[BLOCKER]** No **disabled-account rejection** state on `lKRjr` / `Y09E0`. AU-2 Gherkin "Disabled account cannot log in" requires a designed rejection state (toast or inline error: "Akun Anda nonaktif. Hubungi admin.").

- **[HIGH]** No **password-reset success / "link sent"** toast or screen — AU-4 + C-2 Gherkin "Reset requested for unknown email → generic response (no enumeration)". Required for the password-reset flow regardless of valid/invalid email.

- **[HIGH]** No **"set new password" / completed-reset** screen — AU-4 Gherkin: "And using it sets a new password".

- **[HIGH]** No **"Sesi berakhir / session expired"** state — C-3 "Token expiry mid-session" requires graceful re-login UI. AU-6 "Sessions can be revoked (logout-all / on disable)" implies a "Anda telah keluar oleh admin" state — none designed.

- **[HIGH]** No **empty state** for Pengguna & Peran (`kHNWT`) when filters/search yield zero results. Per design-system §6 every filter must lead to "results OR empty". Pengguna has search + 2 filter selects.

- **[HIGH]** No **empty state** for Audit Log (`rtJRB`) when filters yield zero rows. Same reason.

- **[MEDIUM]** No **"no permission" state** for routes role-gated to super_admin/hr_admin only (per AL-7 "Access to audit is HR/Super Admin only" and RB-2 "Enforcement is server-side"). If a shift_leader navigates to `/audit-log` directly, what do they see? No designed state.

- **[MEDIUM]** No **loading / skeleton** state for any of the four web screens. Generic loading overlay exists on Overlays page (`w9f8A`) — acceptable for reuse, but no E1-specific loading is mapped.

- **[MEDIUM]** No **success toast** designed after "Tambah Pengguna" / "Ubah peran" / "Nonaktifkan akun" — would be reusing the generic Toast component from `o7wJLz`, but the verb-to-toast mapping is undocumented for E1.

- **[LOW]** No **"Tetap masuk" toggle save confirmation** on Pengaturan (`m3sWh`). Toggle changes are silent — design-system rule "no dead-flow" requires either a toast or auto-save indicator.

### 2.4 Untriggered overlays (cat d)

No new overlays are designed inside E1 itself (E1 reuses generic overlays from `hoY3q`). However:

- **[MEDIUM]** The generic confirm-dialog templates in `hoY3q` (`xhC1r`: "Setujui kehadiran?", "Tolak & minta koreksi", "Hapus jadwal shift?") are E5/E4-themed. There is **no E1-themed confirm dialog** for "Nonaktifkan akun pengguna?" / "Ubah peran pengguna?" — both are destructive/auditable actions per RB-6 that should have explicit confirms.

### 2.5 Dangling back/close (cat e)

- **[MEDIUM]** Login screens have no Topbar/Sidebar (correct — pre-auth), so no back affordance. **Successful login destination is not annotated** on `lKRjr` or `Y09E0`. Per F1.2 RB-1..RB-5 + PC-4 the post-login landing is **role-based**:
  - super_admin / hr_admin → Dashboard `ETi5H` (E10)
  - shift_leader → Dashboard Tim `RiSPW` (E10)
  - agent → Beranda mobile `e8Sw1` (E10)
  No connection / note documents this routing in the E1 frame.

- **[LOW]** Pengaturan, Pengguna & Peran, Audit Log all have **breadcrumbs in topbar** ("Pengaturan > Audit Log" etc.) — the **breadcrumb parent "Pengaturan"** is a link visually but its destination overlaps with the `m3sWh` Pengaturan screen which only shows the "Umum" tab. Whether clicking the breadcrumb returns to a settings hub (currently the same screen as `m3sWh`?) is ambiguous.

## 3. Missing screens (cat f)

- **[BLOCKER]** Missing: **"Lupa Kata Sandi" (Forgot Password) — web** — required by `authentication.md` §7 Gherkin "Password reset" + AU-4. Should be a screen reached from "Lupa kata sandi?" link on `lKRjr`. Layout: email input + "Kirim Link Reset" primary button + back-to-login link.

- **[BLOCKER]** Missing: **"Lupa Kata Sandi" (Forgot Password) — mobile** — same as above, reached from `Y09E0`.

- **[BLOCKER]** Missing: **"Reset Password" (set new password)** — both platforms — required by AU-4 ("using it sets a new password"). Layout: new password + confirm + "Simpan" + success toast → redirect to Login.

- **[BLOCKER]** Missing: **"Tambah Pengguna"** form modal / screen — required by F1.2 RB-1..RB-6 (provisioning + role assignment + audit). Fields: email, role select (4), link-to-Employee (E2), status. Confirm + toast.

- **[BLOCKER]** Missing: **"Edit Pengguna / Ubah Peran"** drawer or modal — required by RB-6 "Role assignment/changes are restricted ... and audited". No row-action surface exists.

- **[BLOCKER]** Missing: **"Nonaktifkan Akun" confirm dialog + result** — required by AU-2 (only active users may log in) + AU-6 ("Sessions can be revoked ... on disable"). Per design system §6 destructive actions require a confirm dialog.

- **[HIGH]** Missing: **"Audit Log — Detail Entry" drawer** — required by AL-2 / AL-5 (full before/after JSON, masked per AL-4, searchable by entity). The table abbreviates `before/after` to one line in the Perubahan column; the full record must be reachable to satisfy "Search by entity" Gherkin (`audit-log.md` §7).

- **[HIGH]** Missing: **Login failed / locked-out state** (inline error + lockout banner) — required by AU-5 + C-2 + Gherkin "Wrong password is rate-limited".

- **[HIGH]** Missing: **Disabled-account login rejection** — AU-2 Gherkin "Disabled account cannot log in".

- **[HIGH]** Missing: **"Reset link sent" toast / confirmation screen** — required by `authentication.md` C-2 (generic response — no account enumeration).

- **[HIGH]** Missing: **Session-expired re-auth screen / banner** — required by C-3 (token expiry mid-session) + AU-6 (revoke-on-disable triggers a user-visible kick).

- **[MEDIUM]** Missing: **Empty state for Pengguna & Peran** (filtered to zero).

- **[MEDIUM]** Missing: **Empty state for Audit Log** (filtered to zero — also matters for a freshly-deployed system).

- **[MEDIUM]** Missing: **No-permission state** when a shift_leader or agent hits an HR-only route (audit log, pengguna & peran) — required by RB-2 + design-system §6 "no-permission / disabled state".

- **[MEDIUM]** Missing: **Settings hub / parent navigation** — Pengguna & Peran, Audit Log, and Pengaturan Umum are three siblings under a single sidebar "Pengaturan" footer item. There's no tabbed parent or settings landing showing how the three sub-pages relate. Either nest them under a left sub-nav, or design tabs on Pengaturan, or document the IA.

- **[LOW]** Missing: **POV lines** for E1 — the design-system §7 says feature groups need a POV line per role whose screens differ. E1 has all four roles using the login screen, but Pengguna/Audit/Pengaturan are super_admin + hr_admin only — no POV header documents this scope difference (cf. how E2 Karyawan has explicit POV — HR/Admin, Shift Leader, Agen lines).

- **[LOW]** Missing: **Mobile parity** for role-based features visible to shift_leader on mobile. The DESIGN-SYSTEM §8.14 says "Mobile · All roles: Login `Y09E0`" — so by-design only Login is on mobile for E1 — but no mobile profile/account view exists (mobile agents need to see "My account" to log out / change password). Logout is not designed on mobile either. (May be E2's Profil Saya — cross-epic check needed.)

## 4. PRD coverage matrix

| PRD | Required screens/states | Designed | Missing |
|---|---|---|---|
| **F1.1 Authentication** | Login web, Login mobile, Forgot password, Reset password, Failed-login/lockout, Disabled-account rejection, Reset-link-sent confirmation, Session-expired re-auth | Login web (`lKRjr`), Login mobile (`Y09E0`) | Forgot password (web+mobile), Reset password, Failed/lockout state, Disabled-account state, Reset-sent confirmation, Session-expired re-auth |
| **F1.2 RBAC / Roles** | User list (with role badges), Add user form, Edit/change-role action, Deactivate confirm, No-permission state, Empty state, Role badge tokens | Pengguna & Peran list (`kHNWT`), role badges via `qxONU` | Add user form/modal, Edit/Ubah peran drawer, Nonaktifkan confirm, No-permission state, Empty-results state |
| **F1.3 Audit Log** | Audit list (filters/search/export), Entry detail (before/after JSON, masked), No-permission state, Empty state | Audit Log list (`rtJRB`) + Ekspor btn | Audit entry Detail drawer, No-permission state, Empty state |
| **F1.4 Platform Conventions** | App shell (Sidebar+Topbar), Role-based nav, Settings page with locale/tz/format/currency, Toggle for mobile-stay-logged-in | Sidebar (`iCqTB`) + Topbar (`caFkE`) + Pengaturan Umum (`m3sWh`) | Settings sub-pages (Format Tanggal / Mata Uang / Versi / Stack / Sumber data), Settings hub / IA between Pengguna+Audit+Pengaturan, role-specific sidebar variants (only one Sidebar with hr_admin active state currently — no super_admin / shift_leader / agent variants in E1 frame) |

## 5. Cross-epic references found

- **Sidebar (`iCqTB`)** is the canonical app shell — used by E1 admin screens and all of E2–E10 web. Active nav items reference: Dashboard (E10), Karyawan (E2), Penempatan (E3), Jadwal Shift (E4), Kehadiran (E5), Cuti (E6), Lembur (E7), Laporan (E10), Pengaturan footer (E1).
- **Topbar (`caFkE`)** — shared shell; user pill (right side) shows `Rudi Wijaya · Shift Leader` — note: the audit-screen `rtJRB` / pengguna screen `kHNWT` both render the **Shift Leader** user pill, but their content is super_admin/hr_admin-only per PRD. Visual inconsistency between persona and screen access.
- **"Plaza Senayan"** link in Cakupan column of Pengguna table → likely points to **E3 ClientCompany detail / Roster** (`nLN4d`) — cross-epic navigation not annotated.
- **Post-login routing** depends on E10 dashboards (`ETi5H`, `RiSPW`) and mobile Beranda (`e8Sw1`) — not noted in E1.
- **AuditLog rows** reference entities from many epics (`Cuti #LR-1042` E6, `Kehadiran #ATT-10711` E5, `Pengguna #U-204` E1, `Penempatan #PL-882` E3, `Payslip #PS-5521` E8). Per AL-5, search-by-entity should deep-link to the source — that linkage is undesigned.
- **Logout** — the user pill in `caFkE` is the natural place but there's no logout menu/popover designed; this terminates the session (AU-6).

## 6. Prioritized recommendation

**BLOCKER (ship-blocking — primary user stories broken):**
1. Design **"Lupa Kata Sandi" → "Reset Password" → success-toast → back-to-login** flow for web AND mobile (covers AU-4, C-2).
2. Design **"Tambah Pengguna" form modal** (email, role select 4, link-to-employee from E2, initial status) + success toast (covers RB-1, RB-6, F2.1 provisioning hook).
3. Design **row-action menu** for Pengguna & Peran (Edit profil · Ubah peran · Reset password · Nonaktifkan akun) + the **"Ubah peran" drawer** + the **"Nonaktifkan akun" confirm dialog** (covers RB-6, AU-2, AU-6).
4. Design **failed-login inline error + lockout banner** on Login web + mobile (covers AU-5, C-2 Gherkin).
5. Design **disabled-account rejection** inline error / toast (covers AU-2 Gherkin).

**HIGH (key flow result-states missing):**
6. Design **Audit Log entry Detail drawer** (full before/after with masked sensitive per AL-4; deep-link affordance back to source entity per AL-5).
7. Design **empty states** for Pengguna & Peran (filtered zero) and Audit Log (filtered zero / fresh-deploy zero).
8. Design **no-permission state** (for agent / shift_leader hitting an HR-only URL) — reusable across F1.3 / F1.2.
9. Design **session-expired re-auth** banner/screen + **"reset link sent" generic confirmation** (covers C-3, C-2).
10. Annotate **post-login routing** by role on `lKRjr` / `Y09E0` (super_admin/hr → E10 `ETi5H`, leader → E10 `RiSPW`, agent → E10 `e8Sw1`).
11. Annotate the **"Plaza Senayan"** cross-epic link target (E3 company detail).

**MEDIUM (secondary paths / IA):**
12. Resolve the **Settings IA**: either tabs on Pengaturan or a left sub-nav grouping Pengguna & Peran / Audit Log / Umum under the "Pengaturan" footer item.
13. Remove or wire the **chevrons on Pengaturan rows** (Format Tanggal / Mata Uang / Versi / Stack / Sumber data) — currently imply edit but v1 is locked per FEATURE §7.
14. Add an **E1-themed confirm dialog** template for "Ubah peran" and "Nonaktifkan akun" (don't reuse E5/E4 themed ones).
15. Document **"Tetap masuk" toggle save state** (toast or inline auto-save indicator on `m3sWh`).
16. Add **POV lines** to E1 documenting that login is all-roles but Pengguna & Peran / Audit Log / Pengaturan are super_admin + hr_admin only (per design-system §7).

**LOW:**
17. Design a **logout menu** on the Topbar user pill (or document logout location — possibly cross-epic on profile).
18. Fix persona inconsistency on `kHNWT` / `rtJRB`: the Topbar shows a **Shift Leader** user pill but the screen is HR-only — change the persona to HR/super-admin.

## 7. Notes

- The mobile login (`Y09E0`) intentionally omits the "Tetap masuk" toggle from the web login — the toggle is on Pengaturan (`m3sWh`) "Tetap masuk di mobile (refresh token)" — but the toggle is on the **web** Pengaturan screen, and a Pengaturan screen for mobile doesn't exist. So an agent on mobile cannot toggle their own stay-signed-in preference: the only access path appears to be HR/Admin via web, which contradicts AU-3 + the design intent that agents enable long sessions.
- The Audit Log table has only 6 sample rows and a single-line counter ("Menampilkan 6 dari 2.418 entri") — **no pagination controls** (no Previous/Next, no page-size selector). Per AL-1 (every mutation logged) + C-1 (high write volume), pagination is mandatory at scale.
- The audit log Ekspor button (`WpS0u`) uses a BtnSecondary, which is fine; the generic Ekspor modal (`yPNyD` in flows) is reusable. Mark this in the design system as the route.
- The Pengaturan screen states "Bahasa Indonesia" is locked (v1 badge) which aligns with PC-1, and "Asia/Jakarta · WIB (UTC+7) · tanpa DST" aligns with PC-2 / C-1 — wording matches PRD verbatim, good.
- The Pengguna table renders a chip for each role with appropriate colors (HR Admin = green, Super Admin = purple, Shift Leader = blue, Agen = neutral) which exactly matches DESIGN-SYSTEM §8.14 role badge tokens — correct.
- The user pill in Topbar of `rtJRB` and `kHNWT` shows **"Rudi Wijaya · Shift Leader"** which is incongruous: only super_admin and hr_admin can access these screens (RB-5, AL-7). Either change the sample user or note this is an inconsistency for the synthesis agent to flag.
- No connection / arrow / note nodes were observed within E1 explaining flows between screens (e.g., from `kHNWT` row-kebab → expected drawer); the design relies entirely on naming + visual conventions, which the design system explicitly says is insufficient ("Every interactive element must lead somewhere that is designed").
- The DESIGN-SYSTEM.md §8.14 already lists the screens that should exist for E1, but the actual `.pen` only contains the 5 enumerated above. The doc and the file are out of sync only modestly — the missing-states findings (BLOCKERs) are real gaps, not doc mismatches.
