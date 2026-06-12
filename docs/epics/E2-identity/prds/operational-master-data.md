# PRD · F2.5 — Operational Master Data (leave / attendance / overtime)

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.5 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

The time-tracking epics (E5 Attendance, E6 Leave, E7 Overtime) all depend on admin-defined reference data: **leave types**, **attendance codes**, and **overtime rules**. E2 owns these definitions (CRUD + lifecycle); the *behavior* that consumes them lives in the respective epics. Legacy had `leave_types` and `attendance_codes` (per-company), but **no overtime-rules table** — so OvertimeRule is net-new.

## 2. Goals & non-goals

**Goals**
- Manage the three master lists with the flags each downstream epic needs.
- Safe lifecycle (deactivate, never hard-delete) since records reference them.

**Non-goals**
- Leave balances/requests (E6), attendance capture (E5), OT requests/calc execution (E7). Those consume these definitions.

## 3. Actors

Super Admin (primary), HR Admin, System (validate, audit). Read consumers: E5, E6, E7.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | Super Admin / HR | CRUD leave types, attendance codes, overtime rules. |
| **Mobile app** | Agent / Shift Leader | Read-only — these appear as selectable options/labels when requesting leave/OT or viewing attendance status. |

## 5. Master definitions & business rules

### Leave types
> **Model (2026-06-12):** `leave_type` is the **leave entitlement/cap axis** — each type carries its own cap mechanics. This **reverses** the 2026-06-08 "label + doc-gate + color only / one pool" model. See [EPICS.md §8](../../../EPICS.md) "E6 — Leave" + [E6 FEATURE §4/§7](../../E6-leave/FEATURE.md).

| Ref | Rule |
|-----|------|
| LT-1 | Fields: `code` (unique, e.g. `CTHO`/`SDSKD`), `name`, `description`, `category`, **`cap_basis`** (how the entitlement is capped — see LT-4), `cap_value`, `cap_unit` (`DAYS`\|`COUNT`), `paid`, `gender` (`ANY`\|`FEMALE`\|`MALE`), `requires_document`, `notice_days` (advance-request days), `min_service_years` (eligibility), `lead_days`/`trail_days` (extra paid days around an event window, e.g. hajj), `color`, `status`. |
| LT-2 | **`cap_basis` drives how E6 meters the type** (not a single flag). One of: `ANNUAL_POOL` (accruing yearly quota, expires year-end, no carryover — the only depleting pool), `PER_EVENT` (fixed days **each occurrence**, no running balance), `PER_MONTH` (cap resets each calendar month), `PER_YEAR_COUNT` (max **occurrences** per year), `UNCAPPED` (no cap; bounded only by document/authority), `LIFETIME_ONCE` (once per employment), `SERVICE_UNPAID` (eligibility-gated unpaid, once). |
| LT-3 | `requires_document` types force a document upload on request (E6 INV-5). `gender`≠`ANY`, `notice_days`, `min_service_years` are **request-time eligibility gates** enforced in E6. `paid=false` (e.g. `CLTP`) flags the leave as **unpaid** for payroll (E8). |
| LT-4 | The active catalog is **seeded** from SWP's `Fitur Ijin` policy (18 codes — see §5a). HR may add/edit types; the seeded set is the authoritative starting point. **`ANNUAL_POOL` annual entitlement** still sources `employment_agreements.annual_leave_entitlement_days` (E2), not `cap_value`. |

### 5a. Seeded leave-type catalog (SWP `Fitur Ijin`)

> Source: SWP HR policy "Istilah singkatan Fitur ijin" (18 distinct codes; `CTN` covers the 3 civic sub-reasons). `DAYS` = max days; `COUNT` = max occurrences. `cap_value` blank = uncapped/variable (validated by document).

| Code | Name (Bahasa) | Category | cap_basis | cap_value · unit | Paid | Gender | Doc | Notice (d) | Min svc (y) | Notes |
|------|---------------|----------|-----------|------------------|------|--------|-----|-----------|------------|-------|
| **CTHO** | Cuti Tahunan Head Office | ANNUAL | `ANNUAL_POOL` | 12 · DAYS | ✓ | ANY | — | 0 | 0 | Annual entitlement sources E2 agreement (HO staff). |
| **CT** | Cuti Tahunan Pegawai PKWT | ANNUAL | `ANNUAL_POOL` | 12 · DAYS | ✓ | ANY | — | 0 | 0 | Same 12-day annual pool (PKWT field agent). One annual type per employee, by employment class. |
| **SDSKD** | Sakit dengan surat keterangan dokter | SICK | `UNCAPPED` | — | ✓ | ANY | ✓ | 0 | 0 | "Sesuai ketentuan" — bounded by doctor's letter. |
| **STSD** | Sakit tanpa surat dokter | SICK | `PER_YEAR_COUNT` | 5 · COUNT | ✓ | ANY | — | 0 | 0 | Max 5 occurrences/year; >1 day requires a doctor's letter (→ treat as SDSKD). |
| **CH** | Cuti Haid | MENSTRUAL | `PER_MONTH` | 2 · DAYS | ✓ | FEMALE | — | 0 | 0 | Days 1–2 of menstruation; notify, no doc. |
| **CIM** | Istri melahirkan atau keguguran | LIFE_EVENT | `PER_EVENT` | 2 · DAYS | ✓ | MALE | ✓ | 0 | 0 | Employee's wife gives birth / miscarries. |
| **CM** | Pernikahan sendiri (pertama) | LIFE_EVENT | `LIFETIME_ONCE` | 3 · DAYS | ✓ | ANY | ✓ | 0 | 0 | First own marriage. |
| **CKA** | Khitanan / Baptisan anak | LIFE_EVENT | `PER_EVENT` | 2 · DAYS | ✓ | ANY | ✓ | 0 | 0 | Circumcision/baptism of employee's child. |
| **CMA** | Menikahkan anak | LIFE_EVENT | `PER_EVENT` | 2 · DAYS | ✓ | ANY | ✓ | 0 | 0 | Marrying off a child. |
| **KGD** | Gawat darurat (antar keluarga ke RS) | IMPORTANT | `PER_MONTH` | 2 · DAYS | ✓ | ANY | ✓ | 0 | 0 | "2 hari dalam 1 bulan"; doctor's note. |
| **CKM** | Kematian keluarga inti | BEREAVEMENT | `PER_EVENT` | 2 · DAYS | ✓ | ANY | — | 0 | 0 | Spouse/parent/in-law/child/menantu dies. |
| **CRM** | Kematian anggota serumah lain | BEREAVEMENT | `PER_EVENT` | 1 · DAYS | ✓ | ANY | — | 0 | 0 | Other household member dies. |
| **CTN** | Tugas negara / pengadilan / kewajiban UU | CIVIC | `UNCAPPED` | — | ✓ | ANY | ✓ | 0 | 0 | "Sesuai ketentuan"; covers policy rows 13–15. |
| **CAP** | Cuti Alasan Penting | IMPORTANT | `UNCAPPED` | — | ✓ | ANY | ✓ | 0 | 0 | "Sesuai ketentuan"; HR-discretionary with proof. |
| **CIH** | Cuti Ibadah Haji (pertama) | RELIGIOUS | `LIFETIME_ONCE` | — | ✓ | ANY | ✓ | 30 | 0 | Hajj program + `lead_days`=5 / `trail_days`=5; duration validated by program dates. |
| **CIU** | Cuti Ibadah Umroh (pertama) | RELIGIOUS | `LIFETIME_ONCE` | 12 · DAYS | ✓ | ANY | ✓ | 30 | 0 | First umroh. |
| **CPR** | Cuti Perjalanan Rohani (pertama) | RELIGIOUS | `LIFETIME_ONCE` | — | ✓ | ANY | ✓ | 30 | 0 | "Sesuai ketentuan". |
| **CLTP** | Cuti di luar tanggungan Perusahaan | UNPAID | `SERVICE_UNPAID` | 365 · DAYS | ✗ | ANY | ✓ | 30 | 5 | ≤12 months, once per employment, ≥5 yrs service, **unpaid**. |

Religious leave (`CIH`/`CIU`/`CPR`) requires applying ≥1 month ahead (`notice_days=30`) and reduces no annual quota. `lead_days`/`trail_days` add paid days around an event window (hajj: +5 before, +5 after the official program).

### Attendance codes
| Ref | Rule |
|-----|------|
| AC-1 | Fields: `name`, `description`, `is_workday`, `is_payable`, `is_billable`, `needs_verification`, `color`. |
| AC-2 | `is_billable` marks codes chargeable to the client (relevant to outsource billing/reporting, E10). |
| AC-3 | `needs_verification` codes require shift-leader verification in attendance (E5). |

### Overtime rules (net-new)
| Ref | Rule |
|-----|------|
| OR-1 | Fields: `name`, `multiplier`, `min_minutes`, `requires_preapproval`. |
| OR-2 | Overtime rules are **global only** — there is no service-line (or any other) scope axis *(2026-06-12, EPICS §8 — service_line removed entirely)*. |
| OR-3 | Field set is **provisional** — to be confirmed against Indonesian OT regulation and SWP practice in E7. |

### Common
| Ref | Rule |
|-----|------|
| MD-1 | All three are **deactivated, not deleted**, when referenced. |
| MD-2 | Names are unique within each master list. |
| MD-3 | All actions audited (E1). |

## 6. Data model

`LeaveType`: `id, code (unique), name (unique), description, category, cap_basis (enum), cap_value (int, nullable), cap_unit (DAYS|COUNT), paid (bool), gender (ANY|FEMALE|MALE), requires_document (bool), notice_days (int), min_service_years (int), lead_days (int), trail_days (int), color, status`. *(2026-06-12 — `is_annual` is replaced by `cap_basis = ANNUAL_POOL`.)*
`AttendanceCode`: `id, name (unique), description, is_workday, is_payable, is_billable, needs_verification, color, status`.
`OvertimeRule`: `id, name (unique), multiplier, min_minutes, requires_preapproval, status` — **global only** (no scope FK).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Operational master data

  Scenario: Create a per-occurrence statutory leave type requiring documents
    Given I am a super admin
    When I create a leave type code "CKA" with cap_basis=PER_EVENT, cap_value=2, requires_document=true
    Then requests of this type are capped at 2 days per occurrence and require a document upload (E6)

  Scenario: Create a billable attendance code needing verification
    When I create an attendance code "Overtime Present" with is_billable=true and needs_verification=true
    Then attendance using this code is flagged billable and must be verified by a shift leader (E5)

  Scenario: Create a global overtime rule
    When I create an overtime rule "Night OT" with multiplier 2.0 and min_minutes 60
    Then it applies globally to overtime calculations (E7)

  Scenario: Cannot delete a referenced leave type
    Given leave requests reference "Annual Leave"
    When I try to delete it
    Then deletion is blocked and I may only deactivate it

  Scenario: Unique names within a list
    Given an attendance code "Present" exists
    When I create another "Present"
    Then it is blocked with a uniqueness error
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Deactivate a leave type with open requests | New requests can't use it; in-flight ones complete. |
| C-2 | Migration: per-company `attendance_codes` | Collapse to one SWP-wide set, dedupe by name (DATA-MAPPING G-6). |
| C-5 | Color clash between attendance codes | Allowed (cosmetic); optionally warn. |

## 9. Dependencies

E1 (RBAC/audit), E5 (attendance codes), E6 (leave types/quotas), E7 (overtime rules), E9 (migration), E10 (billable reporting).

## 10. Decisions & open questions

- ✅ E2 owns master definitions; behavior in E5/E6/E7.
- ✅ **`leave_type` is the cap axis (2026-06-12)** — each type carries its own `cap_basis`/`cap_value` mechanics (LT-1..LT-4); seeded from SWP's 18-code `Fitur Ijin` policy (§5a). Reverses the 2026-06-08 "one pool / label-only" model. See [EPICS.md §8](../../../EPICS.md).
- ✅ **OvertimeRule is global only** *(2026-06-12, EPICS §8)* — the service-line scope axis and global-vs-line precedence are dropped along with `service_line`. One SWP-wide rule set.
- **Open (defer to E7):** confirm the OvertimeRule field set against Indonesian OT regulation (e.g., 1.5× first hour, 2× subsequent) and SWP practice.
