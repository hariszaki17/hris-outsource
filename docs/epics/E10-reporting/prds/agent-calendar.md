# PRD · F10.5 — Agent Web Calendar (`/me/calendar`)

> **Epic:** E10 Reporting & Notifications · **Feature:** F10.5 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

On the web self-service console (`/me/*`, see [AGENT-WEB-ACCESS.md](../../../eng/AGENT-WEB-ACCESS.md)) an
employee's own time-bound records are spread across four separate pages — `/me/schedule` (shifts),
`/me/attendance` (clock history), `/me/leave` (cuti), `/me/overtime` (lembur) — and public holidays are
not surfaced to the employee at all. To answer a simple human question — *"what does my month look
like — when do I work, when am I off, did my cuti land on a holiday?"* — the employee has to open four
tabs and mentally merge them against the national-holiday calendar.

F10.5 adds one **read-only month calendar** at `/me/calendar` that overlays all five event types
(shift · attendance · leave · overtime · holiday) on a single grid, color-coded by the design-system
status palette, with a selected-day agenda panel. It creates **no new records** and **no new
authoritative data** — it is a presentation layer over data E4–E7 already own.

## 2. Goals & non-goals

**Goals**
- One month-grid view of the signed-in employee's **own** shifts, attendance, leave, overtime, and the
  applicable public holidays — correctly scoped to self.
- Color-code every event by the **design-system status mapping** (via `StatusBadge`, never raw hex).
- Let the employee drill from any event to its **existing** detail surface (no new detail screens).
- Honor the **`Asia/Jakarta`** day boundary so events land on the correct calendar day.

**Non-goals**
- Creating, editing, approving, or withdrawing any record from the calendar (those stay on
  `/me/leave/new`, `/me/overtime/new`, the clock card, and the approval surfaces). **Read-only.**
- A leader/HR/cross-employee calendar — that is the existing leader/HR surface (F4.3 company grid,
  F6.5 team leave calendar); this PRD is **scope:self only**.
- A mobile calendar — **mobile `Jadwal` (F4.3) is untouched**; this feature is **web only**.
- Sync/export to external calendars (Google/iCal), reminders, or notifications (F10.1 owns those).

## 3. Actors

Employee (self-service baseline — `employee_type = FIELD` "agent" or `INTERNAL`), System (aggregates +
scope-enforces). No elevated role is required or granted.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** (`/me/calendar`) | Any authenticated employee | Read-only month calendar of own events + holidays; tap an event → its existing detail. |
| **Mobile app** | — | **Not in scope** — `Jadwal` (F4.3) stays shift-only and unchanged. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| CAL-1 | **Scope: self.** The calendar shows only the signed-in employee's own records, resolved from the JWT principal — never a body/query `employee_id` (INV-3; AGENT-WEB-ACCESS §4 `scope: self`). |
| CAL-2 | **Five overlaid sources** on one month grid: **shift** (E4 `Schedule`), **attendance** (E5 `Attendance`), **leave/cuti** (E6 `Leave`), **overtime/lembur** (E7 `OvertimeRecord`), **public holiday** (E7 `HolidayCalendar`). No new entity is introduced. |
| CAL-3 | **Color-coding via the design-system status map only** (`StatusBadge` / DESIGN-SYSTEM §2 — never raw hex). **Shift** → neutral/info. **Attendance** → Hadir=ok(teal), Terlambat=warn, Tdk Lengkap=`#ED962F`(orange), Absen=bad. **Leave** → Menunggu=warn, Disetujui=ok, Ditolak=bad. **Overtime** → by `status` (pending=warn, confirmed/approved=ok) with the `day_type` (Workday/RestDay/Holiday) shown in the agenda row. **Holiday** → tinted day cell + holiday name. |
| CAL-4 | **Default layout:** month grid with **today highlighted**, prev/next-month navigation and a "Hari ini" (jump-to-today) control, plus a **selected-day agenda panel** listing that day's events in full. Selecting a day populates the panel; the current day is selected on first load. |
| CAL-5 | **Read-only — tap to detail.** Selecting an event opens its **existing** detail/edit surface: shift → `/me/schedule` (its row/detail), leave → `/me/leave` (the request), overtime → `/me/overtime` (the request), attendance → `/me/attendance` (the day's record). The calendar itself exposes **no** create/edit/approve action. |
| CAL-6 | **Multiple events per day.** A day cell shows up to a fixed number of event markers (dots/chips) and a "+k lainnya" overflow indicator; the **agenda panel lists all** of the selected day's events with no truncation. |
| CAL-7 | **Jakarta day boundary.** Event-to-day assignment uses the `Asia/Jakarta` datetime layer (`@swp/shared/datetime`) — never raw `new Date`. A shift/attendance/overtime that crosses midnight is anchored to its **work date** (the date the source record carries: `Schedule.work_date`, `OvertimeRecord.work_date`, the attendance schedule's work date). |
| CAL-8 | **Multi-day leave spans.** A leave record covering `start_date..end_date` renders a marker on **every** covered calendar day in range (CAL-9 of F6.x duration excludes non-working days for *quota* math — but the calendar **displays the full span** as requested, with status color). |
| CAL-9 | **Holiday display is independent.** A public holiday tints its day cell regardless of whether the employee also has a shift/leave that day; both render. A holiday with no other event still shows its name in the cell and agenda. |
| CAL-10 | **Month-windowed fetch.** Data is fetched for the visible month's date range (plus the leading/trailing days shown to complete the grid weeks). Navigating months refetches that window. List sources use cursor pagination per AGENT-WEB-ACCESS §6. |
| CAL-11 | **Legend.** A persistent legend explains the type/color mapping (shift, attendance status, leave status, overtime, holiday) so the grid is self-describing. |
| CAL-12 | **Partial-source resilience.** If one source fails while others succeed, the calendar renders the available overlays and surfaces a non-blocking inline notice for the failed source (it does not blank the whole month). All-sources failure shows the error state. |

## 6. Data model

**No new entities.** Read projections across existing epics:

| Overlay | Source entity | Key fields read | Endpoint (already exists unless noted) |
|---|---|---|---|
| Shift | E4 `Schedule` | `work_date`, `shift_master_id` (times), `placement_id` (site), `status` | `GET /schedule?employee_id={self}` (scope:self) |
| Attendance | E5 `Attendance` | `schedule_id`, `check_in_at`, `check_out_at`, `status`, `verification_status` | `GET /attendance` (self-filtered) |
| Leave | E6 `Leave` | `leave_type_id`, `start_date`, `end_date`, `status` | `GET /leave-requests` (self) |
| Overtime | E7 `OvertimeRecord` | `work_date`, `start_at`, `end_at`, `day_type`, `status`, `source` | `GET /overtime` (self) |
| Holiday | E7 `HolidayCalendar` | `date`, `name`, `recurring` | holiday read by date-range *(eng: confirm/extend a read endpoint — see Dependencies)* |

**RBAC:** a new baseline capability key **`self.calendar`** → route `/me/calendar`, carried by every
authenticated employee (mirrors the other `self.*` keys in AGENT-WEB-ACCESS §4; not part of
`WEB_ROLES`). `/me/calendar` reads the endpoints above; scope is **server-enforced** (the API resolves
the employee from the token), client gate is defense-in-depth only (ENGINEERING C1).

**Aggregation strategy is eng's to decide** (AGENT-WEB-ACCESS AW-1 prefers no new endpoints): either
client-side merge of the existing `/me` source endpoints for the month window, or a thin aggregate
`GET /calendar/me?from&to`. Either way no new persisted data.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Agent web calendar

  Scenario: Month grid shows my own events overlaid
    Given I am the employee "Budi" with a shift, an approved cuti, and a logged overtime this month
    When I open /me/calendar
    Then I see a month grid with today highlighted
    And the shift day, the cuti days, and the overtime day each show a color-coded marker
    And the current day's agenda panel lists that day's events

  Scenario: Scope is self
    Given I am "Budi"
    When the calendar loads
    Then it shows only my own records
    And no other employee's events are ever shown

  Scenario: Color-coding follows the status map
    Given I have an approved cuti and a pending cuti this month
    When I view the month
    Then the approved cuti renders in the "ok" (teal) status color
    And the pending cuti renders in the "warn" (amber) status color

  Scenario: Public holiday is shown
    Given 17 August is a public holiday in HolidayCalendar
    When I view August
    Then the 17 August cell is tinted and labelled with the holiday name
    And it renders even if I also have a shift or cuti that day

  Scenario: Multi-day cuti spans every covered day
    Given I have an approved cuti from the 10th to the 12th
    When I view the month
    Then the 10th, 11th, and 12th each show the cuti marker

  Scenario: Tap an event opens its existing detail
    Given my month shows an overtime marker on the 5th
    When I select that overtime event
    Then I am taken to that overtime request on /me/overtime

  Scenario: Read-only — no create from the calendar
    When I view /me/calendar
    Then there is no control to create or edit a shift, cuti, overtime, or attendance record

  Scenario: Navigate to another month
    Given I am viewing this month
    When I go to the next month
    Then the grid and overlays refetch for that month's date range

  Scenario: Day with many events shows overflow then full list
    Given I have four events on the 8th and the cell shows three markers
    When I select the 8th
    Then the agenda panel lists all four events with no truncation

  Scenario: One source fails, others still render (CAL-12)
    Given the overtime source errors but shifts, attendance, leave, and holidays load
    When I open the month
    Then the available overlays render
    And an inline notice indicates overtime could not be loaded
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Month with no events at all | Grid renders with holidays only (if any); agenda panel shows a friendly empty state ("Tidak ada agenda hari ini"); no error. |
| C-2 | Brand-new employee, no records anywhere | Empty calendar with onboarding-friendly empty state; holidays still show. |
| C-3 | Multi-day cuti crossing a month boundary | Markers render on the covered days within the visible month; spilling days show when the adjacent month is opened (CAL-8, CAL-10). |
| C-4 | Overnight shift / overtime crossing midnight | Anchored to its `work_date`, shown on one day, not split across two (CAL-7). |
| C-5 | Holiday coincides with a worked shift | Both render: cell tinted as holiday **and** carries the shift/attendance marker (CAL-9). |
| C-6 | Rejected cuti | Shown in "bad" (red) status color so the employee sees the outcome on the day they had requested; still read-only (CAL-3, CAL-5). |
| C-7 | One source slow/unavailable | Partial render + inline per-source notice (CAL-12); not a full-page error. |
| C-8 | All sources fail | Error state with retry; no partial grid. |
| C-9 | Timezone correctness near midnight | A 23:30 clock-in stays on the correct Jakarta day; verified against the `@swp/shared/datetime` layer (CAL-7). |
| C-10 | Dense month (e.g. shift every day + attendance every day) | Cell markers capped with "+k lainnya"; grid stays legible; agenda is the full source of truth (CAL-6). |
| C-11 | No-access (unauthenticated / session expired) | Redirect to login like any `/me/*` route; never renders another employee's data. |

## 9. Dependencies

- **E4** `Schedule` (F4.3), **E5** `Attendance` (F5.5), **E6** `Leave` (F6.x), **E7** `OvertimeRecord`
  + `HolidayCalendar` (F7.1) — read sources.
- **AGENT-WEB-ACCESS.md** — the `/me/*` self-service surface, `self.*` baseline keys, the design/G0
  deviation (AW-6), and the component/i18n/datetime rules this screen obeys.
- **E1** — token/scope enforcement (scope:self).
- **Eng to confirm:** a **holiday read endpoint** by date-range for the self surface (`HolidayCalendar`
  is currently HR/Super-Admin-managed in E7; reading it for the calendar may need a `self`-scoped or
  auth-only read op). And the **aggregation strategy** (client merge vs `GET /calendar/me`).

## 10. Decisions & open questions

- ✅ **Read-only aggregate** — the calendar creates no records and adds no authoritative data; all
  writes stay on their existing surfaces (CAL-5). *(2026-06-18)*
- ✅ **Web only; mobile `Jadwal` untouched** — F4.3 mobile stays shift-only. *(2026-06-18, per product)*
- ✅ **New `/me/calendar` route + `self.calendar` baseline key** — not a second "Kalender" tab; a new
  self-service page alongside `/me/schedule`, which stays the shift-list/management surface. *(2026-06-18)*
- ✅ **Design follows AW-6 (G0 deviation)** — as a `/me/*` self-service web screen, F10.5 is **not
  authored in `brainstorm.pen` first**; the design reference is this PRD's layout/states + `packages/ui`
  + the mobile frame layouts, consistent with [AGENT-WEB-ACCESS AW-6](../../../eng/AGENT-WEB-ACCESS.md).
  *(2026-06-18)*
- **Open (eng):** holiday read-endpoint scope; aggregation strategy (client merge vs `GET /calendar/me`);
  exact per-day marker cap (k) before "+lainnya".
