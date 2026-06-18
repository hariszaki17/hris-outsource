# Test Cases · F5.8 — Attendance Activity Log

> **Feature:** F5.8 · **PRD:** [attendance-activity.md](attendance-activity.md) · **Status:** Draft v1
> Each `TC-#` traces to a business rule (`AA-#`), an acceptance scenario, and/or an edge case (`C-#`).
> Engineering implements these as automated tests (happy-path → Playwright E2E; rule/edge → Vitest+MSW
> on FE, Go handler tests on BE), keeping the `TC-#` id in the test name.

## Test cases

| TC | Title | Type | Preconditions | Steps | Expected | Surface/state | Traces |
|----|-------|------|---------------|-------|----------|---------------|--------|
| **TC-1** | Log activity right after clock-in | happy | Agent has an open record (just clocked in), 0 activities | Add note "Patroli lantai 1" | 201; activity saved with server `recorded_at`; appears in list | Clock card / activity panel · added | AA-1, AA-4, AA-5; AC "right after clock-in"; C-1 |
| **TC-2** | Multiple activities, chronological | happy | Open record with 1 activity | Add 2 more | List shows 3, ordered by `recorded_at` asc | activity list | AA-1, AA-13; AC "Multiple activities" |
| **TC-3** | Clock-out blocked, zero activities | negative | Open record, 0 activities | Tap clock-out | `422 ACTIVITY_REQUIRED`, `fields.activity_count="0"`; still clocked in; prompt to log | clock card · error/guard | AA-7, INV-7; AC "blocked"; |
| **TC-4** | Clock-out succeeds with ≥1 activity | happy | Open record, 1 activity | Clock out | 200; record closed | clock card · success | AA-7; AC "succeeds"; C-2 |
| **TC-5** | Clock-out blocked after deleting last activity | negative | Open record, 1 activity | Delete it, then clock-out | `422 ACTIVITY_REQUIRED` | clock card · error | AA-7, AA-9; C-3 |
| **TC-6** | Write is scope-self | negative | Another agent's open record | POST activity to their record | `403`/`404`; nothing created | API | AA-2; AC "scope-self"; C-8 |
| **TC-7** | No add after clock-out | negative | Own record already clocked out | POST activity | `422 ATTENDANCE_CLOSED` | activity panel · disabled/error | AA-3; AC "immutable"; C-7 |
| **TC-8** | No add to ABSENT record | negative | Own ABSENT record (no clock-in) | POST activity | `422 ATTENDANCE_NOT_OPEN` | API | AA-3; C-6 |
| **TC-9** | Delete own activity while open | happy | Open record with 1 activity | Delete it | Removed from list; count → 0 | activity list · removed | AA-9; AC "Delete my own" |
| **TC-10** | Empty/whitespace note rejected | negative | Open record | POST note "   " | `400 INVALID_REQUEST` (`fields.note`) | form · validation | AA-6; AC "Empty note"; C-4 |
| **TC-11** | Note length boundary | edge | Open record | POST 500-char note; POST 501-char note | 500 → 201; 501 → `400` | form · validation | AA-6; C-5 |
| **TC-12** | Idempotent create | edge | Open record | POST twice with same Idempotency-Key | One activity; same response replayed | API | AA-11; C-9 |
| **TC-13** | System auto-close exempt | happy | Open record, 0 activities, at shift end | Auto-close job runs | Record closed, no gate | server job | AA-8, INV-4; AC "auto-close exempt"; C-10 |
| **TC-14** | Manual entry exempt | happy | HR/SL manual-create with no activities | Submit manual entry | Created, no gate | manual entry | AA-8, F5.6; C-11 |
| **TC-15** | Supervisor reads activity log | happy | Agent record has 3 activities | Shift leader opens record | Sees 3 activities (read-only) | leader attendance detail | AA-10; AC "Supervisor reads" |
| **TC-16** | Clock-out gate reads count inside tx (concurrency) | edge | Open record, activity add racing clock-out | Add activity ~same time as clock-out | No partial state; decision on persisted count | server | AA-7; C-12 |
| **TC-17** | List cursor-paginated | edge | Record with 40 activities | List with limit | Cursor pages, chronological | activity list | AA-13; C-13 |

## Coverage

- **Business rules:** AA-1 (TC-1,2) · AA-2 (TC-6) · AA-3 (TC-7,8) · AA-4 (TC-1) · AA-5 (TC-1) ·
  AA-6 (TC-10,11) · AA-7 (TC-3,4,5,16) · AA-8 (TC-13,14) · AA-9 (TC-5,9) · AA-10 (TC-15) ·
  AA-11 (TC-12) · AA-12 (audit — asserted within TC-1/TC-9 backend tests) · AA-13 (TC-17).
- **Acceptance scenarios:** all 10 covered (TC-1..15).
- **Edge cases:** C-1..C-13 all covered.
- **Screen states:** added · list · empty(no activities) · validation error · clock-out guard error ·
  after-close disabled · supervisor read-only — each exercised by ≥1 TC.

**Gaps:** none. `recorded_at` exactness and audit assertions live in backend handler tests (TC-1).
