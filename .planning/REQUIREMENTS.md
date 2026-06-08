# Requirements: SWP HRIS — Mobile MVP (Agent App)

**Defined:** 2026-06-08
**Milestone:** v1.2 — Mobile MVP (Agent App)
**Core Value:** An agent can run their entire daily work loop from the phone — clock in/out at
the right site, see their schedule, fix mistakes, request leave/OT, and see their pay — against
the real Go backend.

**Build model:** Full-stack vertical slices. Each feature phase delivers the needed Go
endpoint(s) AND the React Native screen(s) together, proven end-to-end. Builds on the v1.1 Expo
scaffold (branch `feat/mobile-scaffold`). Shift-leader app is a **later milestone** (its
backend endpoints already exist — verify/approve-L1 routes are live).

## Backend reality (from coverage audit 2026-06-08)

Three states drive per-feature effort:
- **READY** — endpoint exists, implemented, agent-callable. (auth, notifications, dashboard)
- **OPEN-ROUTE** — endpoint exists + implemented, but the route guard excludes `agent`; spec
  x-rbac already allows agent self-scope. Work = open an agent-scoped route + self-filter.
  (my-attendance, my-schedule, payslip, profile self-read)
- **NEW** — no handler yet; build it. (clock-in/out + photo, agent correction POST, agent leave
  POST + attachment, agent OT POST + confirm, change-request POST)

## v1.2 Requirements (Agent)

### Shell & session (SHELL) — backend READY

- [x] **SHELL-01**: Agent logs in on mobile (E1) and the session/token persists across app restarts (secure storage).
- [x] **SHELL-02**: Authenticated tab navigation (Beranda · Attendance · Schedule · More) with a force-update gate hook (from `update-gate.ts`).
- [x] **SHELL-03**: Beranda/home dashboard renders the agent's role-shaped `GET /dashboards/me`.
- [x] **SHELL-04**: Notifications list + mark-read / mark-all (`GET /notifications`), with empty/loading/error states.

### Clock in/out + geofence (CLOCK) — backend NEW

- [x] **CLOCK-01** (BE): Implement `POST /attendance:clock-in` / `:clock-out` (+ photo upload) with server-side geofence validation against the placement's site (`lat/lng/radius_m`); out-of-fence is allowed + flagged, not blocked (F5.1).
- [x] **CLOCK-02** (FE): Clock screen captures GPS (`expo-location`), shows in/out-of-geofence state, submits clock-in/out, handles all variants (success, out-of-fence flagged, no-GPS, already-clocked, network error).
- [ ] **CLOCK-03** (FE): Optional clock photo via `expo-image-picker`/camera wired to the photo-upload endpoint.

### My attendance + correction (ATTEND) — read OPEN-ROUTE, correction NEW

- [x] **ATTEND-01** (BE): Open `GET /attendance` to `agent` self-scope (own records only) (F5.5).
- [x] **ATTEND-02** (FE): My-attendance history + detail screen (status, late minutes, geofence flag).
- [x] **ATTEND-03** (BE): Implement agent-scoped `POST /corrections` (type check_in/check_out/code, proposed time, reason; 7-day self window) (F5.4).
- [x] **ATTEND-04** (FE): File-correction form from an attendance record + correction status view.

### My schedule (SCHED) — backend OPEN-ROUTE

- [x] **SCHED-01** (BE): Open `GET /schedule` to `agent` self-scope (own placement shifts, date range) (F4.3).
- [x] **SCHED-02** (FE): Week schedule view + shift detail; shift reminder hook (local notification).

### Leave request (LEAVE) — backend NEW

- [x] **LEAVE-01** (BE): Implement agent-scoped `POST /leave-requests` (+ `GET` own list, attachment upload) with quota + schedule-impact validation; routes to SL→HR approval (F6.2).
- [x] **LEAVE-02** (FE): Leave request form (type, dates, duration, delegate, document upload via `expo-image-picker`) + own-requests list + status.

### Overtime (OT) — backend NEW

- [x] **OT-01** (BE): Implement agent-scoped `POST /overtime` (request) and `POST /overtime/{id}:confirm` (confirm auto-detected); own list (F7.2).
- [x] **OT-02** (FE): OT request + confirm-auto-detected screens + OT detail.

### Payslip (PAY) — backend OPEN-ROUTE

- [x] **PAY-01** (BE): Open `GET /payslips` (+ `{id}`) to `agent` self-scope (F8.1).
- [x] **PAY-02** (FE): Payslip history list + summary (take-home, gross, paid date); read-only.

### Profile self-service (PROFILE) — read OPEN-ROUTE, change-request NEW

- [x] **PROFILE-01** (BE): Open `GET /employees/{id}` self-read to `agent`; implement `POST /change-requests` (phone/address/bank → HR approval) (F2.1).
- [x] **PROFILE-02** (FE): Profile view + limited-edit (phone/address/bank) submitting a change-request; Pengaturan (change password, logout).

## Future (next milestone)

### Shift-leader app (MOB-LEADER) — backend READY (no BE work)

- Attendance verification queue + detail (F5.3), leave approval L1 (F6), OT approval L1 (F7), SL team dashboard + combined inbox (F10). All SL endpoints already live.

## Out of Scope (v1.2)

| Feature | Reason |
|---------|--------|
| Shift-leader app | Separate milestone; its backend is already done, so it ships fast on its own. |
| EAS build/release pipeline + real OTA URL | Infra milestone; scaffold stub stands. |
| Offline queue for clock-in | Post-MVP; MVP assumes connectivity (flag + retry only). |
| Background geofence / auto clock-out on mobile | Server already auto-closes; mobile MVP is foreground clock only. |
| PDF payslip | Backend still Excel-only; mobile shows summary fields. |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SHELL-01..04 | Phase 13 | Done (FE verified; live needs BE+device) |
| CLOCK-01..02 | Phase 14 | Done (FE+BE; live needs device GPS) |
| CLOCK-03 (photo) | Phase 14 | Deferred (additive multipart; follow-up) |
| ATTEND-01..02 | Phase 14 | Done |
| ATTEND-03..04 | Phase 15 | Done (CODE-type correction deferred) |
| SCHED-01..02 | Phase 16 | Done (include_company geo deferred) |
| LEAVE-01..02 | Phase 17 | Done (attachment + doc-required types deferred) |
| OT-01..02 | Phase 18 | Done |
| PAY-01..02 | Phase 19 | Done |
| PROFILE-01..02 | Phase 20 | Done (bank-account edit deferred) |
