# PRD · F1.4 — Platform Conventions & App Shell

> **Epic:** E1 Foundations & Platform · **Feature:** F1.4 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Consistency across a Go API + React web + mobile app needs shared conventions: one language (**Bahasa Indonesia**), one canonical timezone (**Asia/Jakarta**), uniform API behavior (errors, pagination, validation), and a common app shell/navigation driven by role. Setting these once in E1 prevents drift across the nine domain epics.

## 2. Goals & non-goals

**Goals**
- Bahasa Indonesia localization (i18n-ready) across web + mobile.
- Asia/Jakarta as the canonical timezone for all date/time logic + display.
- Standard API conventions (error envelope, pagination, validation).
- Role-based app shell/navigation for web + mobile.

**Non-goals**
- Domain features (E2–E10). Auth/RBAC/audit (F1.1–F1.3).

## 3. Actors

All users (consume), Engineers (build to conventions), System.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | Staff | Shell + nav + ID localization. |
| **Mobile app** | Agent / Leader | Shell + nav + ID localization. |
| **Go API** | — | Standard error/pagination/validation contract. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| PC-1 | UI is **Bahasa Indonesia**; strings are externalized (i18n-ready) so a second language could be added later. |
| PC-2 | **Asia/Jakarta** is the canonical timezone for all evaluation (lateness, auto-close, period boundaries) and display; storage in UTC, render in WIB. |
| PC-3 | API uses a **consistent error envelope** (code, message, field errors), **pagination** (cursor/offset standard), and **server-side validation** with structured field errors. |
| PC-4 | The **app shell** renders **role-based navigation** (agent vs leader vs HR vs super admin) on both web and mobile. |
| PC-5 | Money renders as **IDR**; dates/numbers use Indonesian formatting. |
| PC-6 | Conventions are documented and shared so all epics conform. |

## 6. Data model

Config/constants (locale, timezone, formats). No domain entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Platform conventions & app shell

  Scenario: Indonesian UI
    When any user opens the app
    Then the interface is in Bahasa Indonesia

  Scenario: Canonical timezone
    Given a night shift evaluated for lateness
    Then the calculation uses Asia/Jakarta, not server UTC

  Scenario: Consistent API errors
    When a request fails validation
    Then the API returns the standard error envelope with field-level errors

  Scenario: Role-based navigation
    Given a shift leader logs in
    Then they see leader navigation (roster, approvals), not super-admin config

  Scenario: IDR + Indonesian formatting
    Then monetary values render as IDR and dates use Indonesian formatting
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Daylight handling | WIB has no DST; fixed offset — simple but documented. |
| C-2 | Missing translation string | Fallback to a key/default; flagged in dev. |
| C-3 | Cross-midnight rendering | Consistent display per the E4/E5 start-date rule. |
| C-4 | Future second language | i18n structure already supports adding EN (E1 decision was ID-only). |

## 9. Dependencies

Underpins all epics; F1.1–F1.3 (auth/rbac/audit integrate into the shell).

## 10. Decisions & open questions

- ✅ Bahasa Indonesia; Asia/Jakarta canonical; standard API conventions; role-based shell.
- **Open:** pagination style (cursor vs offset) standard for the API.
- **Open:** confirm IDR-only / no multi-currency (single-country operation).
