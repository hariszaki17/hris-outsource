# PRD · F7.1 — Overtime Rules (day-type tiers)

> **Epic:** E7 Overtime Tracking · **Feature:** F7.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Overtime in Indonesia is tiered by day type — a workday hour, a rest-day hour, and a public-holiday hour carry different multipliers. hris-outsource needs configurable **OT rules** that classify and (for future payroll) weight OT by day type, optionally per service line, with a minimum-duration threshold and a pre-approval flag. In v1 the multipliers are **reference only** (we record hours, not pay).

## 2. Goals & non-goals

**Goals**
- Define OT rules with day-type tiers (workday / rest day / holiday), optional service-line scope, `min_minutes`, and `requires_preapproval`.
- Store multipliers as reference metadata for a future payroll run.

**Non-goals**
- Computing OT pay (v1 is hours-only). Capturing/approving OT (F7.2/F7.3).

## 3. Actors

Super Admin / HR (author), System (validate, audit). Consumers: F7.2 (classification), F7.4 (aggregation).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | Super Admin / HR | CRUD OT rules + maintain the public-holiday calendar. |
| **Mobile app** | — | Not surfaced (rules are back-office). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| OR-1 | A rule defines: `day_type` (Workday/RestDay/Holiday), `multiplier` (reference), `min_minutes`, `requires_preapproval`, optional `service_line_id` (null = global). |
| OR-2 | **Precedence:** a service-line-scoped rule overrides the global rule for the same `day_type`. |
| OR-3 | `multiplier` is **reference only** in v1 (informs future payroll); OT records store hours, not money. |
| OR-4 | `min_minutes` is the threshold below which OT is not counted (F7.2 INV-5). |
| OR-5 | A **public-holiday calendar** (master) determines the Holiday day_type; a **rest day** = a day the agent has no scheduled shift (E4). |
| OR-6 | Rules are deactivated, not deleted, when referenced; all actions audited. |

## 6. Data model

`OvertimeRule`: `id, name, service_line_id (FK null), day_type, multiplier, min_minutes, requires_preapproval, status`.
`HolidayCalendar` (master): `id, date, name, recurring (bool), status`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Overtime rules

  Scenario: Define tiered rules
    Given I am HR
    When I create OT rules for Workday (1.5), RestDay (2.0), Holiday (3.0) with min_minutes 60
    Then they are saved as reference multipliers with a 60-minute threshold

  Scenario: Service-line rule overrides global
    Given a global Workday rule and a Parking Workday rule
    When OT is classified for a Parking placement on a workday
    Then the Parking rule applies (OR-2)

  Scenario: Holiday calendar drives the Holiday tier
    Given 2026-08-17 is in the holiday calendar
    When an agent works OT that day
    Then it is classified day_type Holiday

  Scenario: Rest-day classification
    Given an agent has no scheduled shift on 2026-06-14
    When they work OT that day
    Then it is classified day_type RestDay

  Scenario: Cannot delete a referenced rule
    Given OT records reference a rule
    When I try to delete it
    Then deletion is blocked; I may deactivate it
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | OT spanning midnight across two day types | Split or attribute to start day — confirm (likely start day, consistent with E4/E5). |
| C-2 | Holiday that's also the agent's rest day | Holiday tier takes precedence (confirm ordering). |
| C-3 | No rule for a day type | Fallback to global/default + flag. |
| C-4 | Recurring vs one-off holidays | `recurring` handles annual fixed dates; movable holidays added yearly. |

## 9. Dependencies

E2 (service line), E4 (schedule → rest-day), E1 (audit), F7.2/F7.4 (consumers), holiday calendar master.

## 10. Decisions & open questions

- ✅ Day-type tiers, optional per service line, multipliers = reference (hours-only v1).
- **Open:** where does the **holiday calendar** live (E2 master vs here) and who maintains it?
- **Open (C-2):** precedence when a day is both holiday and rest day.
- **Open:** confirm `min_minutes` default (e.g., 60).
