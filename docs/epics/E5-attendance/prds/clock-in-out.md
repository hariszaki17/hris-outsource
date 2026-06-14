# PRD · F5.1 — Clock In/Out (GPS geofence)

> **Epic:** E5 Attendance · **Feature:** F5.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Agents need to record arrival and departure at their assigned site. hris-outsource captures this via **mobile GPS clock in/out**, validating location against the site's geofence and tying the record to the agent's **scheduled shift** (E4). Per decision, being outside the geofence is **allowed but flagged** (real work shouldn't be blocked by GPS drift). No selfie/QR.

## 2. Goals & non-goals

**Goals**
- Mobile clock-in/out with GPS + timestamp.
- Geofence evaluation against the site; out-of-geofence flagged.
- Link to the scheduled shift + placement; create the attendance record.

**Non-goals**
- Lateness/auto-close evaluation (F5.2). Verification (F5.3). Corrections (F5.4).

## 3. Actors

Agent (mobile), System (geofence check, persist, audit), Shift Leader (monitors, F5.3/F5.5).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | Clock in / clock out with GPS. |
| **Web / mobile** | Shift Leader | Near-live view of who's clocked in (F5.5). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| CI-1 | Clock-in captures `check_in_at` (server time, Asia/Jakarta) + device `lat/lng`. |
| CI-2 | The system resolves the **site geofence** from the agent's active placement's **`Site`** (E2 F2.6 — `site.lat`/`lng` + `site.geofence_radius_m`), not the company. A placement always has exactly one site (E3 INV-5). |
| CI-3 | If GPS is **outside** the geofence, the clock-in is **still recorded** with `in_geofence_in=false` and flagged for verification (F5.3). |
| CI-4 | The record links the agent's **scheduled shift** for that date; if none, it's recorded with `schedule_id=null` and flagged **unscheduled**. |
| CI-5 | An agent may have **only one open** attendance record (clocked-in, not out) at a time — a second clock-in is blocked until clock-out. |
| CI-6 | Clock-out captures `check_out_at` + `lat/lng` + `in_geofence_out`; closes the open record. |
| CI-7 | A default attendance code is applied at clock-in (configurable default, e.g., "Present"); leader/correction can change it. |
| CI-8 | All clock events are audited; the record is near-live to the leader (F5.5). |
| CI-9 | The **scheduled shift window** an attendance record uses is the entry's **effective** window at the moment of each clock event (E4 INV-5): `shift_start_at` is fixed to the entry's `start_time` at clock-in and does not change; `shift_end_at` continues to follow master edits on the linked `Schedule` entry until clock-out, at which point it is fixed to the entry's current `end_time`. |

## 6. Data model

Creates/updates `Attendance` (see [FEATURE.md](../FEATURE.md) §4): `check_in_at, lat_in, lng_in, in_geofence_in, schedule_id, placement_id, attendance_code_id` on clock-in; `check_out_at, lat_out, lng_out, in_geofence_out` on clock-out.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Clock in / out

  Background:
    Given I am the agent "Budi" on mobile
    And I have a "Parking Night" shift today at "Plaza Senayan"
    And "Plaza Senayan" has a geofence radius of 100m

  Scenario: Clock in inside the geofence
    When I clock in from within 100m of the site
    Then an attendance record is created with in_geofence_in true
    And it links my scheduled shift and placement

  Scenario: Clock in outside the geofence is allowed but flagged
    When I clock in from 500m away
    Then the clock-in is recorded with in_geofence_in false
    And it is flagged for shift-leader verification

  Scenario: Unscheduled clock-in is flagged
    Given I have no shift scheduled today
    When I clock in
    Then the record is created with no schedule link and flagged "unscheduled"

  Scenario: Cannot clock in twice
    Given I am already clocked in and not yet clocked out
    When I try to clock in again
    Then it is blocked until I clock out

  Scenario: Clock out
    Given I am clocked in
    When I clock out
    Then check_out_at and my location are recorded and the record closes

  Scenario: GPS unavailable
    When I try to clock in with location services off
    Then I am prompted to enable GPS
    And if unavailable, the clock-in is recorded with location missing and flagged
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Clock-in well before shift start | Allowed; earliness noted; lateness handled in F5.2. |
| C-2 | Clock-out next day (cross-midnight night shift) | Allowed; record stays tied to the shift's start date. |
| C-3 | Clock-out without an open clock-in | Blocked / prompts a correction (F5.4). |
| C-4 | Spoofed/mock GPS | Out of scope v1; flag as future anti-fraud (note). |
| C-5 | Offline at site (no connectivity) | See §10 open — queue+sync vs require connectivity. |
| C-6 | Site has no geo set (lat/lng) | Geofence check **skipped + flagged**; record still saved (E2 F2.6 ST-8). `geofence_radius_m` defaults to 100m. |

## 9. Dependencies

E4 (schedule), E3 (placement → site), E2 (**Site geofence** F2.6 + attendance codes), E1 (audit), E10 (notifications), F5.2 (evaluation).

## 10. Decisions & open questions

- ✅ GPS-only; out-of-geofence allowed + flagged; tied to schedule.
- ✅ **Geofence source = the placement's `Site`** (E2 F2.6), single circle (center + `geofence_radius_m`, default 100m) *(2026-06-03; was ClientCompany)*.
- **Open:** offline clock-in (queue + later sync) needed, or is on-site connectivity assumed?
- **Open:** anti-spoofing (mock-location detection) — v1 or later?
