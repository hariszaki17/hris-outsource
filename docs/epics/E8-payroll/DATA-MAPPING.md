# E8 — Payroll · ims-system → hris-outsource Data Mapping

> Migration analysis for historical payroll. Source: SWP prod (`lumen_swp`, MySQL). Target: hris-outsource (Postgres).
> Parent: [FEATURE.md](FEATURE.md) · Related epic: E9 Data Migration. Status: Draft v1.

---

## 1. Shape of the change

Payroll migrates **as-is for history** (read-only). The main work is **decrypting** the legacy monetary fields (legacy uses a `DBEncryption` Eloquent cast on comp columns) and re-storing them encrypted, plus remapping identity. Payslip **summary** fields surface to agents; the **component line items** + benefits are retained for HR only.

## 2. Source tables

| Table | → Target | Key columns |
|-------|----------|-------------|
| `employee_payslips` | Payslip | `id, employee_id, total_hari_kerja, tanggal_dibayarkan, gaji_pokok*, bpjs_ks*, bpjs_tk_*, pph21*, total_gross_penerimaan*, total_gross_pengurangan*, take_home_pay*, is_posted, year, month` |
| `employee_salaries` | SalaryComponent | `id, employee_salary_column_id, employee_payslip_id, value*, is_amount_for_bpjs` |
| `employee_salary_columns` | (component names) | component/line-item definitions |
| `employee_benefits` | Benefit | `id, employee_id, name/value*` |

`*` = encrypted via legacy `DBEncryption` cast.

## 3. Field mapping

### `employee_payslips` → Payslip
| Legacy | → | Notes |
|---|---|---|
| `id` | `legacy_payslip_id` | crosswalk |
| `employee_id` | `employee_id` | remap |
| `year`, `month` | `year`, `month` | period |
| `tanggal_dibayarkan` | `paid_on` | pay date |
| `total_hari_kerja` | `working_days` | string→int |
| `total_gross_penerimaan` * | `gross_earnings` | **decrypt → re-encrypt** |
| `total_gross_pengurangan` * | `gross_deductions` | decrypt → re-encrypt |
| `take_home_pay` * | `take_home_pay` | decrypt → re-encrypt |
| `gaji_pokok`, `bpjs_*`, `pph21` * | payslip component values (archive) | decrypt; retained for HR archive (not in agent summary) |
| `is_posted` | `is_posted` | — |
| `show_all_benefit` | drop | legacy flag |
| `created_at/by`, `deleted_at` | carry | — |

### `employee_salaries` → SalaryComponent
| Legacy | → | Notes |
|---|---|---|
| `employee_payslip_id` | `payslip_id` | remap |
| `employee_salary_column_id` | `name` (resolve via `employee_salary_columns`) | denormalize the component name |
| `value` * | `value` | decrypt → re-encrypt |
| `is_amount_for_bpjs` | `for_bpjs` | — |

### `employee_benefits` → Benefit
`employee_id` → remap; benefit name/value (decrypt value) → `name`/`value`.

## 4. Gaps & decisions

| # | Gap | Handling |
|---|-----|----------|
| G-1 | **Encryption** | Decrypt all `*` fields with the legacy app key (DBEncryption), re-store encrypted in Postgres; failures → review queue, never null. |
| G-2 | **Identity remap** | `employee_id` via crosswalk (E2). |
| G-3 | **Summary vs components** | Payslip-level fields (gaji_pokok/bpjs/pph21) overlap the `employee_salaries` line items — keep payslip summary fields for display, retain line items in the HR archive; avoid double-counting in any total. |
| G-4 | **Volume** | Payslips ≈ employees × months — moderate; batch by period. |
| G-5 | **Retention** | Confirm retention policy + whether any purge is allowed (FEATURE §7 Q1). |
| G-6 | **Read-only** | No re-computation on import; values carried verbatim (decrypted). |

## 5. Migration rules

1. Runs after E2 (employees). Read-only import; no payroll recomputation.
2. Decrypt → re-encrypt all monetary fields; idempotent via crosswalks.
3. Orchestration in E9; this doc owns the E8 field-level mapping.
