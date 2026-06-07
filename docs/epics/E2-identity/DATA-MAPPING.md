# E2 — Identity, Org & Master Data · ims-system → hris-outsource Data Mapping

> Migration analysis for identity + master data. Source: SWP prod (`lumen_swp`, MySQL). Target: hris-outsource (Postgres).
> Parent: [FEATURE.md](FEATURE.md) · Related epic: E9 Data Migration. Status: Draft v1.

---

## 1. Shape of the change

Legacy splits identity across **`users`** (login, with a tinyint `role` + `company_id` tenant) and **`employees`** (HR profile), bridged by `employees.user_id`. hris-outsource keeps the split but **drops multi-tenancy** (single org = SWP), **remaps the role enum**, and lifts compensation out of the encrypted `employee_contracts` blob into a structured **EmploymentAgreement**. Job roles (`recruitment_roles`) become the **Position** master; **service line and overtime rules have no source** and are seeded/defined fresh.

## 2. Source tables

| Table | → Target | Key columns |
|-------|----------|-------------|
| `users` | User (E1) | `id, email, password, name, nip, phone_number, company_id, role(tinyint), position, is_enabled` |
| `employees` | Employee | `id, name, nik, nip, join_at, gender, birth_*, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan, acc_number_*, user_id, last_contract_id` |
| `employee_contracts` | EmploymentAgreement | `contract_status_id, pkwt_reference, contract_start_at, contract_end_at, resign_at, gaji_pokok*, bpjs_*, pph21*` (`*`=encrypted) |
| `companies` (role=2) | ClientCompany | `id, name, address, npwp, penanggung_jawab, phone_number, is_enabled` (geo `lat/long_address` **not** migrated — see G-8) · `leader_scope` defaults `company` |
| *(none — net-new)* | Site (F2.6) | Loader **auto-creates one primary "Main Site" per ClientCompany**, geofence **empty**; geo is configured post-cutover (G-8). |
| `recruitment_roles` | Position | `id, role, alias, client, recruitment_role_type_id` |
| `recruitment_role_types` | (lookup) | `id, name` — drives PKWT/PKWTT + position category derivation |
| `leave_types` | LeaveType | `id, name, description, is_tahunan, is_document_required` |
| `attendance_codes` | AttendanceCode | `id, name, description, company_id, hari_masuk, hari_penggajian, dapat_ditagih, perlu_verifikasi, color` |
| — (none) | ServiceLine, OvertimeRule | **net-new** — seed / define fresh |

## 3. Field mapping

### `users` → User (E1)
| Legacy | → | Notes |
|---|---|---|
| `id` | `legacy_user_id` | crosswalk |
| `email` | `email` | unique, lowercased |
| `password` | `password_hash` | Laravel bcrypt — carry hash if scheme compatible, else force reset (G-2) |
| `name`,`nip`,`phone_number` | → **Employee** fields | identity attributes live on Employee now |
| `company_id` | **DROP** | single internal tenant (SWP) |
| `role` (tinyint) | `role` | **remap** legacy enum → {super_admin, hr_admin, shift_leader, agent} (need value map, G-1) |
| `position` | **DROP** | position is per-placement (E3) |
| `is_enabled` | `status` | enabled→active |
| `is_announcement` | **DROP** | legacy feature flag |

### `employees` → Employee
`name→full_name`, `nik→nik`, `nip→nip`, `join_at→join_at`, `gender`, `birth_date/place`, `npwp`, `bpjs_kesehatan`, `bpjs_ketenagakerjaan`, `acc_number_*→bank_account`, `user_id→user_id` (1:1 link), `last_contract_id`→resolve to current EmploymentAgreement, `is_posted`→drop, soft-deletes carried.

### `employee_contracts` → EmploymentAgreement
| Legacy | → | Notes |
|---|---|---|
| `contract_status_id` (→`recruitment_role_types`) | `type` + `status` | derive PKWT/PKWTT + status from the type name (G-4) |
| `pkwt_reference` | `agreement_no` | — |
| `contract_start_at` | `start_date` | — |
| `contract_end_at` | `end_date` | null/empty ⇒ PKWTT |
| `gaji_pokok` (enc) | `base_salary` (enc) | **decrypt → re-store encrypted** |
| `bpjs_*` (enc) | `bpjs_terms` (enc json) | decrypt; structure as json |
| `pph21` (enc) | `tax_profile` | decrypt |
| `resign_at` | agreement close | sets status terminated/resigned |
| `annual_leave` | → Placement entitlement (E3) / leave quota (E6) | not on agreement |
| `role_id` | → **Position** on Placement (E3) | not on agreement |

### `recruitment_roles` → Position
| Legacy | → | Notes |
|---|---|---|
| `role` | `name` | — |
| `alias` | `alias` | — |
| `client` | (classification hint) | drop or keep as note |
| `recruitment_role_type_id` | category (optional) | from `recruitment_role_types.name` |
| — | `service_line_id` | **manual classification** (no source; same approach as placement G-1) |

### `leave_types` → LeaveType
`name→name`, `description→description`, `is_tahunan→is_annual`, `is_document_required→is_document_required`.

### `attendance_codes` → AttendanceCode
`name→name`, `description→description`, `hari_masuk→is_workday`, `hari_penggajian→is_payable`, `dapat_ditagih→is_billable`, `perlu_verifikasi→needs_verification`, `color→color`, `company_id`→**drop/collapse** (single org; dedupe across companies, G-6).

## 4. Gaps & decisions

| # | Gap | Handling |
|---|-----|----------|
| G-1 | **Legacy role tinyint → new role enum** unknown | Inspect distinct `users.role` values in prod; build an explicit value map to {super_admin, hr_admin, shift_leader, agent}. Shift-leader role is mostly derived from E3 F3.4, not the legacy enum. |
| G-2 | **Password hashes** | Confirm Laravel bcrypt cost/scheme; if portable, carry the hash; otherwise force a password reset / re-invite on first login. |
| G-3 | **Service line absent** (Position + Placement) | Manual classification later (per E3 G-1 decision). Positions load with `service_line_id` pending. |
| G-4 | **PKWT/PKWTT derivation** | Read distinct `recruitment_role_types.name` values; map to type + status. Fallback: null `contract_end_at` ⇒ PKWTT. |
| G-5 | **Identity reconciliation** | Post-migration **every Employee MUST have a linked User** (1:1 non-null — F2.1 EP-4, D1/D4). Legacy `employees.user_id` that are **null** (no login) are **backfilled a User keyed on phone** (email if present), with a **temp password / forced reset** on first login (G-2). Phone is the **required** login identifier (email optional). `users` with no employee are reviewed separately (E9 review queue). Keep both legacy ids in the crosswalk. **No null `user_id` post-migration.** |
| G-6 | **Per-company master** | `attendance_codes` (and any per-company leave config) are keyed by `company_id`; collapse to one SWP-wide set, dedupe by name. |
| G-7 | **No OvertimeRule / ServiceLine source** | Seed `ServiceLine` = {Facility Services, Building Management, Parking}; define `OvertimeRule` fresh (confirm fields in E7). Not migrated. |
| G-8 | **Sites are net-new** (F2.6, 2026-06-03) | Legacy is flat (one address per `role=2` company; `role=4` ignored). The loader **auto-creates one primary "Main Site" per ClientCompany** with **empty geofence** to satisfy the required `Placement.site_id`; legacy geo is **not** carried over. HR configures geofences + splits multi-site companies post-cutover. `Placement.site_id` → the company's Main Site. |

## 5. Migration rules

1. Identity loads first (User + Employee + crosswalks), then EmploymentAgreement, then master data, before E3 placements.
2. Idempotent via crosswalks (`legacy_user_id`, `legacy_employee_id`, `legacy_contract_id`, `legacy_company_id`, `legacy_role_id`).
3. Decrypt-then-re-encrypt for all comp fields; failures go to a review queue, never silently null.
4. Detailed extract/load orchestration lives in E9; this doc owns the E2 field-level mapping + decisions.
