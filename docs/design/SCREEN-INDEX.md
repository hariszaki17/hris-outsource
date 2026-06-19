# Screen Index — brainstorm.pen (living registry)

> **Status:** active · created 2026-06-19 from a full scan of the current `brainstorm.pen`.
> **Single source of truth for screen discovery & sprint planning.** Supersedes the
> per-epic inventories in [`audit/`](./audit/) for *discovery* (the audits remain the record
> of point-in-time findings). Conventions: [`SCREEN-NAMING.md`](./SCREEN-NAMING.md). Change
> rules: [`SCREEN-CHANGE-PROTOCOL.md`](./SCREEN-CHANGE-PROTOCOL.md). Web build tracker:
> [`../eng/SCREEN-GENERATION-PLAN.md`](../eng/SCREEN-GENERATION-PLAN.md).

## How to use this

- **"What ships this sprint?"** → filter by `Epic` (and fill the `Sprint` column as you plan).
- **"All states of X?"** → find the screen, read its `State` rows.
- **Open it in the .pen** → use the `Frame` ID with `batch_get`. IDs are stable across
  rename/move, so this index never breaks on a rename.
- **Storage vs discovery:** the `.pen` board tree is `platform → role lane → POV line`; this
  index is the epic/feature re-slice on top. They are the same screens, two views.
- `Canonical` = the target name per [`SCREEN-NAMING.md`](./SCREEN-NAMING.md). Where it differs
  from `Current`, that row is part of the rename pass (P3).

**Legend** — Kind: `screen`·`overlay`·`panel`·`showcase`. Plat/Role per naming doc.
`⚠ stale` = feature removed; archive/delete in cleanup.

**Counts:** ~150 frames · 11 epics (E9 = migration, script-only, admin console TBD).

---

## E1 — Foundations / Auth

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `lKRjr` | E1 · Login (Web) | web/all | screen | — | E1 · web/all · Login | | |
| `JRq3Z` | E1 · Login — Gagal | web/all | screen | error | E1 · web/all · Login — error | | |
| `N2IdlJ` | E1 · Login — Terkunci sementara | web/all | screen | locked | E1 · web/all · Login — locked | | |
| `QVifb` | E1 · Login — Akun nonaktif | web/all | screen | disabled | E1 · web/all · Login — disabled | | |
| `etsMo` | E1 · Lupa Kata Sandi (Web) | web/all | screen | — | E1 · web/all · Lupa Kata Sandi | | |
| `vz7oI` | E1 · Tautan Reset Terkirim (Web) | web/all | screen | success | E1 · web/all · Tautan Reset Terkirim | | |
| `N1c1X` | E1 · Reset Kata Sandi (Web) | web/all | screen | — | E1 · web/all · Reset Kata Sandi | | |
| `b8BGef` | E1 · Reset Berhasil (Web) | web/all | screen | success | E1 · web/all · Reset Berhasil | | |
| `fVinX` | E1 · Pengaturan (Hub) | web/hr | screen | — | E1 · web/hr · Pengaturan (Hub) | | |
| `m3sWh` | E1 · Pengaturan (Settings) | web/hr | screen | — | E1 · web/hr · Pengaturan | | dup of Hub? reconcile |
| `kHNWT` | E1 · Pengguna & Peran (HR) | web/hr | screen | — | E1 · web/hr · Pengguna & Peran | | |
| `tg1Cf` | E1 · Pengguna & Peran — Hasil kosong | web/hr | screen | empty | E1 · web/hr · Pengguna & Peran — empty | | |
| `TqMQ6` | E1 · Pengguna & Peran — Tanpa izin (POV: SL) | web/sl | screen | no-permission | E1 · web/sl · Pengguna & Peran — no-permission | | |
| `Z4wYS` | E1 · Pengguna & Peran — Row Kebab (popover) | web/hr | overlay | — | E1 · web/hr · Row Kebab Pengguna | | |
| `K9DQR` | E1 · Pengguna & Peran — Edit Drawer | web/hr | overlay | — | E1 · web/hr · Edit Pengguna | | |
| `BWWxD` | E1 · Ubah Peran (modal) | web/hr | overlay | — | E1 · web/hr · Ubah Peran | | |
| `VHFRo` | E1 · Nonaktifkan Pengguna (confirm) | web/hr | overlay | — | E1 · web/hr · Nonaktifkan Pengguna | | |
| `ALEcX` | E1 · Aktifkan Pengguna (confirm) | web/hr | overlay | — | E1 · web/hr · Aktifkan Pengguna | | |
| `yUoOm` | E1 · Kirim Reset Kata Sandi (confirm) | web/hr | overlay | — | E1 · web/hr · Kirim Reset Kata Sandi | | |
| `rtJRB` | E1 · Audit Log (HR) | web/hr | screen | — | E1 · web/hr · Audit Log | | |
| `Zxv9P` | E1 · Audit Log — Detail Drawer | web/hr | overlay | — | E1 · web/hr · Detail Audit Log | | |
| `Dm1OO` | E1 · Audit Log — Hasil kosong | web/hr | screen | empty | E1 · web/hr · Audit Log — empty | | |
| `GVoQi` | E1 · Audit Log — Tanpa izin (POV: SL) | web/sl | screen | no-permission | E1 · web/sl · Audit Log — no-permission | | |
| `cwU1Q` | E1 · Sesi Berakhir (full-page) | web/all | screen | session-expired | E1 · web/all · Sesi Berakhir | | |
| `unET0` | E1 · Tanpa Izin (full-page) | web/all | screen | no-permission | E1 · web/all · Tanpa Izin | | |
| `Y09E0` | Agen · Login (Mobile) | mobile/all | screen | — | E1 · mobile/all · Login | | |
| `XouNm` | Agen · Login — Gagal | mobile/all | screen | error | E1 · mobile/all · Login — error | | |
| `PiWlc` | Agen · Login — Terkunci | mobile/all | screen | locked | E1 · mobile/all · Login — locked | | |
| `YG9jg` | Agen · Login — Akun nonaktif | mobile/all | screen | disabled | E1 · mobile/all · Login — disabled | | |
| `wHKXQ` | Agen · Lupa Kata Sandi (Mobile) | mobile/all | screen | — | E1 · mobile/all · Lupa Kata Sandi | | |
| `Kl6wT` | Agen · Tautan Reset Terkirim (Mobile) | mobile/all | screen | success | E1 · mobile/all · Tautan Reset Terkirim | | |
| `Y6feM` | Agen · Reset Kata Sandi (Mobile) | mobile/all | screen | — | E1 · mobile/all · Reset Kata Sandi | | |
| `gNzLP` | Agen · Reset Berhasil (Mobile) | mobile/all | screen | success | E1 · mobile/all · Reset Berhasil | | |
| `So1Dc` | E1 · Form Validation States (showcase) | web | showcase | — | E1 · web · Form Validation States | | |

---

## E2 — Identity (employees, master data, client companies)

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `WElYh` | E2 · Karyawan — Daftar | web/hr | screen | — | E2 · web/hr · Karyawan — Daftar | | |
| `JBjBb` | E2 · Karyawan — Detail | web/hr | screen | — | E2 · web/hr · Karyawan — Detail | | |
| `h6bDz` | E2 · Karyawan — Tambah | web/hr | screen | — | E2 · web/hr · Karyawan — Tambah | | |
| `a55lE` | E2 · Karyawan Detail — Nonaktif | web/hr | screen | disabled | E2 · web/hr · Karyawan — Detail — disabled | | |
| `n3wi1w` | E2 SL · Karyawan — List (scoped) | web/sl | screen | — | E2 · web/sl · Karyawan — Daftar | | scoped to leader's company |
| `rtKzk` | E2 SL · Detail (read-only) | web/sl | screen | — | E2 · web/sl · Karyawan — Detail | | read-only |
| `f8mBr` | E2 · Data Master — Hub | web/hr | screen | — | E2 · web/hr · Data Master (Hub) | | |
| `HII8C` | E2 · Jenis Cuti — Daftar | web/hr | screen | — | E2 · web/hr · Jenis Cuti — Daftar | | |
| `rMNJT` | E2 · Modal · Tambah/Edit Jenis Cuti | web/hr | overlay | — | E2 · web/hr · Tambah/Edit Jenis Cuti | | |
| `R5xoi` | E2 · Kode Kehadiran — Daftar | web/hr | screen | — | E2 · web/hr · Kode Kehadiran — Daftar | | |
| `u8eXaW` | E2 · Modal · Tambah/Edit Kode Kehadiran | web/hr | overlay | — | E2 · web/hr · Tambah/Edit Kode Kehadiran | | |
| `SnXpE` | E2 · Aturan Lembur — Daftar | web/hr | screen | — | E2 · web/hr · Aturan Lembur — Daftar | | |
| `JYmgi` | E2 · Modal · Tambah/Edit Aturan Lembur | web/hr | overlay | — | E2 · web/hr · Tambah/Edit Aturan Lembur | | |
| `hb7vL` | E2 · Modal · Tambah/Edit Posisi | web/hr | overlay | — | E2 · web/hr · Tambah/Edit Posisi | | posisi now on employee |
| `tNMfN` | E2 · Popover · Row Kebab Menu | web/hr | overlay | — | E2 · web/hr · Row Kebab Master | | |
| `qIpsj` | E2 · Perusahaan Klien — Daftar | web/hr | screen | — | E2 · web/hr · Perusahaan Klien — Daftar | | |
| `OmuQT` | E2 · Perusahaan Klien — Detail (Plaza Senayan) | web/hr | screen | — | E2 · web/hr · Perusahaan Klien — Detail | | |
| `ZmJnZ` | E2 · Perusahaan Klien — Tambah | web/hr | screen | — | E2 · web/hr · Perusahaan Klien — Tambah | | |
| `n7GljN` | E2 · Drawer · Site & Geofence (F2.6) | web/hr | overlay | — | E2.F2.6 · web/hr · Site & Geofence | | |
| `BFDvk` | E2 · Panel · Lokasi & Site (SitesPanel) | web/hr | panel | — | E2 · web/hr · Lokasi & Site (SitesPanel) | | |
| `mS8rP` | E2 · Perjanjian Kerja — Daftar | web/hr | screen | — | E2 · web/hr · Perjanjian Kerja — Daftar | | |
| `Cu0qg` | E2 · Perjanjian Kerja — Detail (PKT-2026-0042) | web/hr | screen | — | E2 · web/hr · Perjanjian Kerja — Detail | | |
| `gxqjg` | E2 · Perjanjian Kerja — Buat (PKWT) | web/hr | screen | — | E2 · web/hr · Perjanjian Kerja — Buat | | |
| `Us5IQ` | E2 · Panel · Kontrak Akan Berakhir (F2.7) | web/hr | panel | — | E2.F2.7 · web/hr · Kontrak Akan Berakhir | | |
| `JH0LX` | E2 · Panel · Pemimpin Shift | web/hr | panel | — | E2 · web/hr · Pemimpin Shift (panel) | | |
| `d7NQ97` | E2 · Panel · Penempatan Aktif (roster) | web/hr | panel | — | E2 · web/hr · Penempatan Aktif (panel) | | overlaps E3 roster |
| `ghHnh` | E2 · Wave 3.4 — Row treatment reference | web/hr | panel | — | E2 · web/hr · Row treatment (ref) | | design reference |
| `bImeO` | E2 · Form Validation States (showcase) | web | showcase | — | E2 · web · Form Validation States | | |
| `vV79c` | E2 · Lini Layanan — Daftar | web/hr | screen | — | — | | ⚠ stale — service line dropped 2026-06-12 |
| `I8WeKy` | E2 · Lini Layanan — Detail (Parking) | web/hr | screen | — | — | | ⚠ stale — service line |
| `IwKfo` | E2 · Modal · Tambah/Edit Lini Layanan | web/hr | overlay | — | — | | ⚠ stale — service line |
| `Ckteo` | E2 · Antrian Persetujuan — HR | web/hr | screen | — | — | | ⚠ stale — change-request void (EPICS §8/E11) |
| `L8lbE` | E2 · Drawer · Detail Pengajuan Perubahan | web/hr | overlay | — | — | | ⚠ stale — change-request void |
| `tgnZP` | E2 · Modal · Tolak Pengajuan Perubahan | web/hr | overlay | — | — | | ⚠ stale — change-request void |

---

## E3 — Placement

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `C2SSLA` | E3 · Penempatan — Perusahaan | web/hr | screen | — | E3 · web/hr · Penempatan — Perusahaan | | |
| `nLN4d` | E3 · Roster — Plaza Senayan | web/hr | screen | — | E3 · web/hr · Roster | | |
| `xy3Of` | E3 · Roster — Akan Berakhir <30 hari | web/hr | screen | — | E3 · web/hr · Roster — expiring | | |
| `g3OzZz` | E3 · Buat Penempatan | web/hr | screen | — | E3 · web/hr · Buat Penempatan | | |
| `pFR79` | E3 · Detail Penempatan | web/hr | screen | — | E3 · web/hr · Detail Penempatan | | |
| `MffYZ` | E3 · Detail Penempatan — Ended | web/hr | screen | ended | E3 · web/hr · Detail Penempatan — ended | | |
| `VPIiW` | E3 · Detail Penempatan — Terminated | web/hr | screen | terminated | E3 · web/hr · Detail Penempatan — terminated | | |
| `MS2fi` | E3 · Detail Penempatan — Resigned | web/hr | screen | resigned | E3 · web/hr · Detail Penempatan — resigned | | |
| `nneYt` | E3 · Detail Penempatan — Superseded | web/hr | screen | superseded | E3 · web/hr · Detail Penempatan — superseded | | |
| `VrG6t` | E3 · Overlay — Transfer Penempatan | web/hr | overlay | — | E3 · web/hr · Transfer Penempatan | | |
| `OrRhu` | E3 · Overlay — Perpanjang Penempatan | web/hr | overlay | — | E3 · web/hr · Perpanjang Penempatan | | |
| `NTFgQ` | E3 · Overlay — Akhiri Penempatan | web/hr | overlay | — | E3 · web/hr · Akhiri Penempatan | | |
| `n3Fc2d` | E3 · Overlay — Hentikan Paksa (Terminate) | web/hr | overlay | — | E3 · web/hr · Hentikan Paksa | | |
| `Naszr` | E3 · Overlay — Catat Resign | web/hr | overlay | — | E3 · web/hr · Catat Resign | | |
| `o5Txgg` | E3 SL · Roster (read-only) | web/sl | screen | — | E3 · web/sl · Roster | | read-only |
| `erZYn` | E3 · Overlays — Pemimpin Shift (Assign/Reassign/INV) | web/hr | showcase | — | E3 · web/hr · Pemimpin Shift Overlays | | |
| `TXYXW` | E3 · Overlay — Row Kebab + Row→Detail | web/hr | showcase | — | E3 · web/hr · Roster Row Overlays | | |
| `Fhcoo` | E3 · Overlays — Buat Penempatan State Variants | web/hr | showcase | — | E3 · web/hr · Buat Penempatan States | | |
| `u61WkG` | E3 · Extras — Roster empty + No-leader chip | web/hr | showcase | — | E3 · web/hr · Roster Extras | | |
| `BMENY` | E3 · Overlay — Transfer Penempatan | web/hr | overlay | — | — | | dup of `VrG6t` — dedupe |
| `JSO5b` | E3 · Overlay — Transfer + Replacement | web/hr | overlay | — | E3 · web/hr · Transfer + Replacement | | |
| `JCiA7` | E3 · Overlay — Transfer Result States | web/hr | showcase | — | E3 · web/hr · Transfer Result States | | |
| `hwFaA` | E3 · Overlay — Perpanjang Penempatan | web/hr | overlay | — | — | | dup of `OrRhu` — dedupe |
| `K80bIJ` | E3 · Overlays — Akhiri/Hentikan/Resign | web/hr | showcase | — | E3 · web/hr · Lifecycle Overlays | | |

---

## E4 — Shift Scheduling

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `O5JgF` | E4 · Master Shift | web/hr | screen | — | E4 · web/hr · Master Shift | | |
| `Mn9ux` | E4 · Tambah Shift (modal) | web/hr | overlay | — | E4 · web/hr · Tambah Shift | | |
| `FBXYm` | E4 · Edit Shift (modal) | web/hr | overlay | — | E4 · web/hr · Edit Shift | | |
| `iqH3A` | E4 · Deactivate Shift (confirm) | web/hr | overlay | — | E4 · web/hr · Nonaktifkan Shift | | |
| `STdXt` | E4 · Reactivate Toggle (shift detail) | web/hr | panel | — | E4 · web/hr · Reactivate Toggle (panel) | | |
| `Rubba` | E4 · Jadwal Mingguan (Shift Leader) | web/sl | screen | — | E4 · web/sl · Jadwal Mingguan | | |
| `BfUbA` | E4 · Shift Picker Popover (cell click) | web/sl | overlay | — | E4 · web/sl · Shift Picker | | |
| `CJK1Q` | E4 · Cell Edit Menu (filled cell click) | web/sl | overlay | — | E4 · web/sl · Cell Edit Menu | | |
| `IDxV8` | E4 · Bulk Apply to Range (modal) | web/sl | overlay | — | E4 · web/sl · Bulk Apply to Range | | |
| `FX6iz` | E4 · Conflict-block Toast States (showcase) | web | showcase | — | E4 · web · Conflict Toasts | | |
| `DpZUA` | E4 · Auto-publish + Clear Toasts (showcase) | web | showcase | — | E4 · web · Publish Toasts | | |

---

## E5 — Attendance

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `sZCLW` | Screen 1 · Kehadiran — Dashboard | web/hr | screen | — | E5 · web/hr · Kehadiran — Dashboard | | legacy "Screen N" name |
| `UEG2J` | Screen 2 · Verifikasi Kehadiran | web/hr | screen | — | E5 · web/hr · Verifikasi Kehadiran | | legacy name |
| `VY894` | Screen 3 · Detail Verifikasi | web/hr | screen | — | E5 · web/hr · Detail Verifikasi | | legacy name |
| `gWOlU` | Screen · Buat Kehadiran Manual | web/hr | screen | — | E5 · web/hr · Buat Kehadiran Manual | | legacy name |
| `TW7gB` | Overlay · Tolak Koreksi | web/hr | overlay | — | E5 · web/hr · Tolak Koreksi | | |
| `QfamL` | HR · Koreksi · Antrian | web/hr | screen | — | E5 · web/hr · Koreksi — Antrian | | |
| `sSKtK` | HR · Koreksi · Detail | web/hr | screen | — | E5 · web/hr · Koreksi — Detail | | |
| `Mo6vc` | Overlay · Tolak Koreksi (E5) | web/hr | overlay | — | — | | dup of `TW7gB` — dedupe |
| `iDkj0` | Overlay · Verifikasi Massal (E5) | web/hr | overlay | — | E5 · web/hr · Verifikasi Massal | | |
| `V2QL7` | E5 SL · Team Attendance — Plaza Senayan | web/sl | screen | — | E5 · web/sl · Kehadiran Tim | | |
| `MsXnm` | E5 SL · Verifikasi — Plaza Senayan | web/sl | screen | — | E5 · web/sl · Verifikasi | | |
| `RZPQz` | E5 SL · Detail Verifikasi — Plaza Senayan | web/sl | screen | — | E5 · web/sl · Detail Verifikasi | | |
| `hWAZc` | SL · Verifikasi Kehadiran (Antrian) | mobile/sl | screen | — | E5 · mobile/sl · Verifikasi — Antrian | | |
| `S4zMo` | SL Mobile · Detail Verifikasi (Bottom Sheet) | mobile/sl | overlay | — | E5 · mobile/sl · Detail Verifikasi | | |
| `UA2nt` | SL Mobile · Tolak Verifikasi (Bottom Sheet) | mobile/sl | overlay | — | E5 · mobile/sl · Tolak Verifikasi | | |
| `wplKL` | SL Mobile · Verifikasi Massal (Bottom Sheet) | mobile/sl | overlay | — | E5 · mobile/sl · Verifikasi Massal | | |
| `Iek78` | Agen · Absen (Clock In/Out) | mobile/agent | screen | — | E5 · mobile/agent · Absen | | inline btn — retrofit to comp/BtnPrimary |
| `KIb82` | Agen · Absen · Clock-Out variant | mobile/agent | screen | — | E5 · mobile/agent · Absen — clockout | | |
| `hgM53` | Agen · Absen · Di luar geofence | mobile/agent | screen | outside-geofence | E5 · mobile/agent · Absen — outside-geofence | | |
| `W6RqO` | Agen · Absen · Tanpa jadwal (flagged) | mobile/agent | screen | no-schedule | E5 · mobile/agent · Absen — no-schedule | | |
| `pPjz2` | Agen · Absen · GPS tidak tersedia | mobile/agent | screen | no-gps | E5 · mobile/agent · Absen — no-gps | | |
| `GJI1a` | Agen · Riwayat Kehadiran (Mobile) | mobile/agent | screen | — | E5 · mobile/agent · Riwayat Kehadiran | | |
| `CzLHW` | Agen · Detail Kehadiran (Mobile) | mobile/agent | screen | — | E5 · mobile/agent · Detail Kehadiran | | |
| `l6UYy` | Agen · Riwayat Kehadiran — Filter Terlambat | mobile/agent | screen | — | E5 · mobile/agent · Riwayat — filter-late | | |
| `txgoB` | Agen · Riwayat Kehadiran — Sheet Rentang | mobile/agent | overlay | — | E5 · mobile/agent · Riwayat — Rentang | | |
| `x2rDk` | Agen · Riwayat Kehadiran — Kalender Custom | mobile/agent | overlay | — | E5 · mobile/agent · Riwayat — Kalender | | |
| `ss5bp` | E5 · Form Validation States (showcase) | web | showcase | — | E5 · web · Form Validation States | | |

---

## E6 — Leave

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `yho5i` | E6 · Persetujuan Cuti (HR L2) | web/hr | screen | — | E6 · web/hr · Persetujuan Cuti (L2) | | |
| `DJrBn` | E6 · Detail Pengajuan Cuti | web/hr | screen | — | E6 · web/hr · Detail Pengajuan Cuti | | |
| `eHXWF` | E6 · Detail Pengajuan Cuti — No-leader (HR sole) | web/hr | screen | — | E6 · web/hr · Detail Pengajuan Cuti — no-leader | | |
| `ZlnfW` | E6 · Detail Pengajuan Cuti — Saldo berubah (LA-5) | web/hr | screen | — | E6 · web/hr · Detail Pengajuan Cuti — saldo-changed | | |
| `P6HZ7E` | E6 · Kuota & Hibah Cuti (HR) | web/hr | screen | — | E6 · web/hr · Kuota & Hibah Cuti | | |
| `IennJ` | E6 · Rincian Kuota per Jenis (HR drill-in) | web/hr | screen | — | E6 · web/hr · Rincian Kuota per Jenis | | |
| `CGCnL` | E6 · Sesuaikan Kuota (modal) | web/hr | overlay | — | E6 · web/hr · Sesuaikan Kuota | | |
| `S8y0G7` | E6 · Tambah Kuota (modal) | web/hr | overlay | — | E6 · web/hr · Tambah Kuota | | |
| `s5niW` | E6 · Kalender Cuti (HR) | web/hr | screen | — | E6 · web/hr · Kalender Cuti | | |
| `qb0S0` | E6 SL · Persetujuan Cuti (L1) | web/sl | screen | — | E6 · web/sl · Persetujuan Cuti (L1) | | |
| `Hzbbv` | E6 SL · Detail Pengajuan Cuti (L1) | web/sl | screen | — | E6 · web/sl · Detail Pengajuan Cuti | | |
| `YvYcr` | E6 SL · Kalender Cuti Tim | web/sl | screen | — | E6 · web/sl · Kalender Cuti Tim | | |
| `jQ8bz` | SL · Persetujuan Cuti (Antrian) | mobile/sl | screen | — | E6 · mobile/sl · Persetujuan Cuti — Antrian | | |
| `NTf2v` | SL Mobile · Detail Persetujuan Cuti (Bottom Sheet) | mobile/sl | overlay | — | E6 · mobile/sl · Detail Persetujuan Cuti | | |
| `AYFle` | SL Mobile · Tolak Cuti (Bottom Sheet) | mobile/sl | overlay | — | E6 · mobile/sl · Tolak Cuti | | |
| `QT92D` | Agen · Ajukan Cuti | mobile/agent | screen | — | E6 · mobile/agent · Ajukan Cuti | | |
| `v7ZzMm` | Agen · Ajukan Cuti — Kuota terlampaui (blocked) | mobile/agent | screen | blocked | E6 · mobile/agent · Ajukan Cuti — blocked-quota | | |
| `H24N3P` | Agen · Ajukan Cuti — Dokumen wajib (blocked) | mobile/agent | screen | blocked | E6 · mobile/agent · Ajukan Cuti — blocked-doc | | |
| `hjCYy` | Agen · Status Pengajuan Cuti | mobile/agent | screen | — | E6 · mobile/agent · Status Pengajuan Cuti | | |
| `o1BUa` | Agen · Cuti Saya (Saldo & Riwayat) | mobile/agent | screen | — | E6 · mobile/agent · Cuti Saya | | |
| `x3WGup` | E6 · Wave 3.4 — Batalkan/Persingkat Cuti Disetujui | web/hr | showcase | — | E6 · web/hr · Batalkan/Persingkat Cuti | | |
| `WWUPQ` | E6 · Form Validation States (showcase) | web | showcase | — | E6 · web · Form Validation States | | |

---

## E7 — Overtime

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `H1eBN` | E7 · Persetujuan Lembur (HR L2) | web/hr | screen | — | E7 · web/hr · Persetujuan Lembur (L2) | | |
| `uG6mQ` | E7 · Detail Lembur (HR Review) | web/hr | screen | — | E7 · web/hr · Detail Lembur | | |
| `vd4na` | E7 · Aturan OT & Kalender Libur (HR) | web/hr | screen | — | E7 · web/hr · Aturan OT & Kalender Libur | | |
| `JEmCk` | E7 · Rekap Lembur (HR) | web/hr | screen | — | E7 · web/hr · Rekap Lembur | | |
| `Vh2P9` | E7 SL · Persetujuan Lembur (L1) | web/sl | screen | — | E7 · web/sl · Persetujuan Lembur (L1) | | |
| `S2NIK` | SL · Persetujuan Lembur (Antrian) | mobile/sl | screen | — | E7 · mobile/sl · Persetujuan Lembur — Antrian | | |
| `MNJFv` | SL Mobile · Detail Lembur (Bottom Sheet) | mobile/sl | overlay | — | E7 · mobile/sl · Detail Lembur | | |
| `wDLQu` | Agen · Ajukan Lembur | mobile/agent | screen | — | E7 · mobile/agent · Ajukan Lembur | | |
| `mzCUA` | Agen · OT Terdeteksi (Konfirmasi) | mobile/agent | screen | — | E7 · mobile/agent · OT Terdeteksi | | |
| `nd3KT` | Agen · Lembur Saya (Riwayat) | mobile/agent | screen | — | E7 · mobile/agent · Lembur Saya | | |
| `STI8j` | E7 · Wave 3.4 — Tarik kembali OT (agent) | mobile/agent | showcase | — | E7 · mobile/agent · Tarik Kembali OT | | ⚑ loose in lane (P4 drift) |
| `YGLK3` | E7 · Overlays & States Showcase | web/hr | showcase | — | E7 · web/hr · Overlays & States | | |
| `chuB7` | E7 · Form Validation States (showcase) | web | showcase | — | E7 · web · Form Validation States | | |

---

## E8 — Payroll (read-only history)

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `jBgLn` | E8 · Arsip Payroll (HR) | web/hr | screen | — | E8 · web/hr · Arsip Payroll | | |
| `JaScP` | E8 · Detail Slip Gaji (HR) | web/hr | screen | — | E8 · web/hr · Detail Slip Gaji | | |
| `q8JxjZ` | E8 · Detail Slip Gaji (HR) · Decrypt-fail variant | web/hr | screen | decrypt-fail | E8 · web/hr · Detail Slip Gaji — decrypt-fail | | |
| `BDHMZ` | HR Audit-Note · Drawer & states | web/hr | panel | — | E8 · web/hr · Audit-Note (panel) | | |
| `dRfK9` | Empty & Access-denied · State-cards | web/hr | showcase | — | E8 · web/hr · Empty & Access-denied States | | |
| `v8uQX2` | Agen · Slip Gaji (Daftar) | mobile/agent | screen | — | E8 · mobile/agent · Slip Gaji — Daftar | | |
| `ocsq4` | Agen · Detail Slip (Ringkasan) | mobile/agent | screen | — | E8 · mobile/agent · Detail Slip | | |

---

## E9 — Migration

No design frames exist (E9 is ~80% backend). A reconciliation-review queue + cutover/validation
admin console is required per [`audit/E9-AUDIT.md`](./audit/E9-AUDIT.md) — **design TBD**. When
built: `E9 · web/super · …`.

---

## E10 — Reporting / Dashboards / Inbox / Notifications

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `ETi5H` | E10 · Dashboard (HR) | web/hr | screen | — | E10 · web/hr · Dashboard | | KPI cards inline — retrofit to comp/StatCard |
| `DhzyL` | E10 · Dashboard (Super Admin) | web/super | screen | — | E10 · web/super · Dashboard | | |
| `EF8AZ` | E10 · Laporan Kehadiran & Jam Billable (HR) | web/hr | screen | — | E10 · web/hr · Laporan Kehadiran & Billable | | |
| `BXWGd` | E10 · Kotak Masuk (HR) | web/hr | screen | — | E10 · web/hr · Kotak Masuk | | |
| `i0qW8` | E10 · Pusat Notifikasi (HR) | web/hr | screen | — | E10 · web/hr · Pusat Notifikasi | | |
| `y4bTp2` | E10 · Kotak Masuk — Review Perubahan (modal) | web/hr | overlay | — | E10 · web/hr · Review Perubahan | | |
| `HUMzF` | E10 · Kotak Masuk — Review Perubahan (SL · bank→HR) | web/sl | overlay | — | E10 · web/sl · Review Perubahan | | |
| `OJRpG` | E10 · Kotak Masuk — Tolak Perubahan (modal) | web/hr | overlay | — | E10 · web/hr · Tolak Perubahan | | |
| `OIL9n` | E10 · Kotak Masuk — Perubahan Disetujui (toast) | web/hr | overlay | success | E10 · web/hr · Perubahan Disetujui (toast) | | |
| `g8DCqL` | E10 · Kotak Masuk — Perubahan Ditolak (toast) | web/hr | overlay | — | E10 · web/hr · Perubahan Ditolak (toast) | | |
| `RiSPW` | E10 SL · Dashboard Tim | web/sl | screen | — | E10 · web/sl · Dashboard Tim | | |
| `T9puC` | Section · Leader compliance panel | web/sl | panel | — | E10 · web/sl · Leader Compliance (panel) | | |
| `UMzuO` | E10 · Beranda Pemimpin Shift (Mobile) | mobile/sl | screen | — | E10 · mobile/sl · Beranda | | ⚑ loose in lane (P4 drift); dup of `ZdGeE`? |
| `ZdGeE` | SL · Beranda Tim | mobile/sl | screen | — | E10 · mobile/sl · Beranda Tim | | |
| `nL5OT` | SL · Notifikasi | mobile/sl | screen | — | E10 · mobile/sl · Notifikasi | | |
| `skCcH` | SL · Inbox Persetujuan (Gabungan) | mobile/sl | screen | — | E10 · mobile/sl · Inbox Persetujuan | | |
| `NCbVf` | SL · Profil | mobile/sl | screen | — | E10 · mobile/sl · Profil | | |
| `WKYgI` | Agen · Notifikasi | mobile/agent | screen | — | E10 · mobile/agent · Notifikasi | | |
| `unjVt` | Agen · Pengajuan (Hub) | mobile/agent | screen | — | E10 · mobile/agent · Pengajuan (Hub) | | |
| `nwlSV` | Agen Web · Dasbor (gabungan) | web/agent | screen | — | E10 · web/agent · Dasbor | | |
| `DVHhZ` | Agen Web · Notifikasi | web/agent | screen | — | E10 · web/agent · Notifikasi | | |
| `HcTQb` | Agen Web · Pengajuan | web/agent | screen | — | E10 · web/agent · Pengajuan | | |
| `i0zzG` | Agen Web · Akun | web/agent | screen | — | E10 · web/agent · Akun | | |

### E10 — Agent web (self-service) — attendance/leave/OT/profile actions

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `tn0qq` | Agen Web · Absen Masuk (modal) | web/agent | overlay | — | E5 · web/agent · Absen Masuk | | epic E5 surface |
| `gsyFM` | Agen Web · Absen Keluar (modal) | web/agent | overlay | — | E5 · web/agent · Absen Keluar | | epic E5 |
| `x8nQ6U` | Agen Web · Absen — Di Luar Area (modal) | web/agent | overlay | outside-geofence | E5 · web/agent · Absen — outside-geofence | | epic E5 |
| `FhVcC` | Agen Web · Ajukan Koreksi (modal) | web/agent | overlay | — | E5 · web/agent · Ajukan Koreksi | | epic E5 |
| `j2C0So` | Agen Web · Ajukan Cuti (modal) | web/agent | overlay | — | E6 · web/agent · Ajukan Cuti | | epic E6 |
| `fXv8w` | Agen Web · Ajukan Lembur (modal) | web/agent | overlay | — | E7 · web/agent · Ajukan Lembur | | epic E7 |
| `ByIkH` | Agen Web · Status Pengajuan (modal) | web/agent | overlay | — | E10 · web/agent · Status Pengajuan | | |
| `Lli7o` | Agen Web · Ubah Profil (modal) | web/agent | overlay | — | E2 · web/agent · Ubah Profil | | epic E2 |
| `rVKHu` | Agen Web · Absen — States & Overlays | web/agent | showcase | — | E5 · web/agent · Absen States | | |
| `t8PRZ` | Agen Web · Jadwal — State | web/agent | showcase | — | E4 · web/agent · Jadwal States | | |
| `U02dM` | Agen Web · Cuti — State | web/agent | showcase | — | E6 · web/agent · Cuti States | | |
| `YdC6S` | Agen Web · Ajukan Cuti — State | web/agent | showcase | — | E6 · web/agent · Ajukan Cuti States | | |
| `NWAgj` | Agen Web · Lembur — State | web/agent | showcase | — | E7 · web/agent · Lembur States | | |
| `iV0UY` | Agen Web · Ajukan Lembur — State | web/agent | showcase | — | E7 · web/agent · Ajukan Lembur States | | |
| `eFV6z` | Agen Web · Slip Gaji — State | web/agent | showcase | — | E8 · web/agent · Slip Gaji States | | |
| `MLKWr` | Agen Web · Profil — State | web/agent | showcase | — | E2 · web/agent · Profil States | | |
| `qoIWS` | Agen Web · Notifikasi — State | web/agent | showcase | — | E10 · web/agent · Notifikasi States | | |
| `w35aBQ` | Agen Web · Koreksi — State | web/agent | showcase | — | E5 · web/agent · Koreksi States | | |

### E5 — Agent mobile (profile/correction self-service, epic-tagged here for completeness)

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `s5RO1` | Agen · Profil Saya (read-only) | mobile/agent | screen | — | E2 · mobile/agent · Profil Saya | | epic E2 |
| `n465cT` | Agen · Ajukan Perubahan | mobile/agent | screen | — | E2 · mobile/agent · Ajukan Perubahan | | epic E2 (now instant edit) |
| `SXqA5` | Agen · Status Pengajuan | mobile/agent | screen | — | — | | ⚠ stale — change-request void (EPICS §8/E11) |
| `mqGEi` | Agen · Penempatan Saya | mobile/agent | screen | — | E3 · mobile/agent · Penempatan Saya | | epic E3 |
| `Gp9PZ` | Agen · Koreksi · Form (Mobile) | mobile/agent | screen | — | E5 · mobile/agent · Koreksi — Form | | |
| `LsihV` | Agen · Koreksi · Tracker (Mobile) | mobile/agent | screen | — | E5 · mobile/agent · Koreksi — Tracker | | |
| `hH3yR` | Agen · Koreksi · Detail (Mobile) | mobile/agent | screen | — | E5 · mobile/agent · Koreksi — Detail | | |

---

## E11 — Approvals (cross-cutting chain)

| Frame | Current name | Plat/Role | Kind | State | Canonical | Sprint | Notes |
|---|---|---|---|---|---|---|---|
| `d7tFAM` | E11 · Template Persetujuan (HR) | web/hr | screen | — | E11 · web/hr · Template Persetujuan | | |
| `yv7Gs` | E11 · Kotak Masuk Persetujuan (HR/Lead) | web/hr | screen | — | E11 · web/hr · Kotak Masuk Persetujuan | | |
| `OHseV` | E11 · Detail Permintaan (rantai persetujuan) | web/hr | screen | — | E11 · web/hr · Detail Permintaan | | |
| `KT3Jz` | E11 · Overlay — Bypass (Super Admin) | web/super | overlay | — | E11 · web/super · Bypass | | |
| `uoTwN` | E11 · Overlay — Konfirmasi Reset Pending | web/hr | overlay | — | E11 · web/hr · Konfirmasi Reset Pending | | |
| `DxK66` | E11 · Kotak Masuk Persetujuan (SL mobile) | mobile/sl | screen | — | E11 · mobile/sl · Kotak Masuk Persetujuan | | |
| `viUFF` | E11 · Sheet — Setujui (SL mobile) | mobile/sl | overlay | — | E11 · mobile/sl · Setujui | | |
| `PGrLa` | E11 · Status Pengajuan — rantai (Agen mobile) | mobile/agent | screen | — | E11 · mobile/agent · Status Pengajuan | | |

---

## Reconciliation — resolved & open

**Resolved 2026-06-19:**
- ✅ **Stale screens archived** → moved to new `Z · Archive` board (`mdzxW`): service-line
  (`vV79c`, `I8WeKy`, `IwKfo`), change-request (`Ckteo`, `L8lbE`, `tgnZP`, `SXqA5`). Names
  prefixed `⚠ STALE`. Their rows above keep the `⚠ stale` Note + `Canonical = —`.
- ✅ **Duplicates deleted:** `BMENY`≈`VrG6t`, `hwFaA`≈`OrRhu`, `Mo6vc`≈`TW7gB`. Rows removed
  from the tables.
- ✅ **Junk deleted:** `yDXdl` ("E2 · Aksi, status & alur", width 40000).
- ✅ **Drift fixed:** `STI8j` moved into agent E7 row (`px7m7`); `UMzuO` into SL Beranda row
  (`A24Kr`).

**Still open:**
1. **Possible dup screens — confirm intent:** `m3sWh` vs `fVinX` (Pengaturan Hub vs Settings);
   `UMzuO` ("Beranda", 900h) vs `ZdGeE` ("Beranda Tim", 844h) — now adjacent in `A24Kr`,
   look related but not identical. Decide whether to merge.
2. **Instancing retrofit (ongoing, per change-protocol):** `Iek78` ClockInBtn, `ETi5H` KPI
   cards rebuilt inline instead of `comp/*` refs. Pay down opportunistically.
3. **POV-line names:** tag each `POV line · …` container with its epic for canvas scannability
   (deferred — low priority, doesn't affect the index).
