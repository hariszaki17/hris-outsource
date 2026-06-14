# Agent Web Access — Spec (self-service console for the AGENT role)

> **Status:** Adopted 2026-06-10 · **Owner:** eng · **Binds:** `apps/web`, `packages/{shared,ui,api-client}`
> **Supersedes:** the "`agent` is mobile-only / no web permissions" stance in
> [NAVIGATION-AND-RBAC.md](./NAVIGATION-AND-RBAC.md) §4 — ratified in [EPICS.md](../EPICS.md) §8.

This doc specifies which **pages, features and data** the **agent** role can reach on the **web
console** (`apps/web`), and the **web clock-in** feature (porting the mobile clock flow to the
browser). It is the source of truth for the build; screens trace to it.

---

## 1. Why

Agents already have the full self-service surface on the **React Native mobile app**. Some agents
work from sites with a shared desktop/kiosk or simply prefer the browser, and HR wants a single
web origin where an agent can clock in and manage their own records. The **backend is already
agent-ready**: every clock + self-service endpoint declares `x-rbac: { roles: [agent], scope: self }`
and identifies the agent from the **JWT principal** (never a body `employee_id`). So this is a
**frontend-only** port — no new endpoints, no schema changes.

This does **not** change the internal-only tenancy decision (EPICS §2): agents are SWP staff with
logins, not clients. The separate **client portal** (NAVIGATION-AND-RBAC §5) remains a distinct,
unratified concern.

## 2. Scope

**In scope** — full parity with the mobile agent surface, on the web, plus **web clock-in**:

| Page (web route) | Mobile analog | Purpose |
|---|---|---|
| `/me` | `(app)/index` | Personal dashboard: greeting, today's shift, OT-hours-this-month, unread notifications |
| `/me/attendance` | `(app)/attendance` | **Clock-in / clock-out** card + own attendance history; file correction per row |
| `/me/schedule` | `(app)/schedule` | Own weekly shift schedule (read-only) |
| `/me/leave` | `leave` | Own leave requests (list) + "Ajukan Cuti" |
| `/me/leave/new` | `leave-new` | Create leave request |
| `/me/overtime` | `overtime` | Own overtime requests (list) + confirm / withdraw + "Ajukan Lembur" |
| `/me/overtime/new` | `overtime-new` | Create overtime request |
| `/me/profile` | `profile` | View profile; **instant-edit** phone/address/emergency/bank/photo/language (no approval, 2026-06-14) |
| `/me/payslip` | `payslip` | Own payslips (list) |
| `/me/notifications` | `(app)/notifications` | Notification inbox; mark read / mark all read |
| `/me/correction` (route w/ `attendanceId`,`date` search params) | `correction` | File attendance correction (7-day window) |

**Out of scope (this pass):** clock-in **photo capture** (deferred exactly as on mobile — the
`photo_id` field stays optional and unwired); password reset screens (auth epic owns these);
any admin/leader capability; a separate `apps/agent-portal` (we reuse `apps/web`).

## 3. Decisions (ratified 2026-06-10)

| # | Decision | Rationale |
|---|---|---|
| AW-1 | **Same app** (`apps/web`) with an agent nav backbone, not a separate SPA. | Reuses shell, auth, router, api-client; no new build/deploy surface. |
| AW-2 | Agent routes live under the **`/me/*`** prefix. | Avoids collision with admin screens that already own `/attendance`, `/schedule`, `/leave`, `/overtime` (those are the HR/leader verification/approval surfaces). `/me/*` reads unambiguously as "my own". |
| AW-3 | New **`self.*` capability keys** gate agent pages; the `agent` role bundle is these keys; `agent` joins `WEB_ROLES`. | Keeps RBAC permission-keyed (NAVIGATION-AND-RBAC). Admin roles don't carry `self.*`, so agent nav never shows for them and vice-versa. |
| AW-4 | The shell picks the **nav backbone by role** (`agent` → agent nav; staff → admin nav); item visibility within a backbone stays permission-filtered. | The agent IA is fundamentally different from the admin IA — a single merged sidebar would be confusing. Backbone-by-role is a presentation choice; the capability gate remains permission-keyed (defense-in-depth, ENGINEERING.md C1). |
| AW-5 | **Web clock-in matches mobile**: GPS required (browser Geolocation), out-of-geofence override flow, photo optional/deferred. | One behavior across surfaces; the server logic is identical (same endpoints). |
| AW-6 | **G0 deviation, documented**: there are **no agent web `.pen` frames**; screens are built pragmatically by reusing `packages/ui` and adapting the mobile frame layouts, not authored in `brainstorm.pen` first. | Agent web access was decided after the design system; authoring frames first would block the port. Frames may be back-filled later; until then this doc + the mobile frames (`Iek78`, `PAOwr`, `fN9AJ`, `QT92D`, `o1BUa`, `wDLQu`, `nd3KT`, `e8Sw1`, `WKYgI`) are the design reference. |

## 4. RBAC — capability keys & data scope

New permission keys (capability axis, added to `packages/shared/src/rbac.ts`):

| Key | Grants | Server endpoints (already exist) |
|---|---|---|
| `self.dashboard` | `/me` | `GET /dashboards/me` |
| `self.attendance` | `/me/attendance`, `/me/correction` | `POST /attendance:clock-in`, `:clock-out`, `GET /attendance` (self-filtered), `POST /corrections`, `GET /corrections` |
| `self.schedule` | `/me/schedule` | `GET /schedule?employee_id={self}` |
| `self.leave` | `/me/leave`, `/me/leave/new` | `GET/POST /leave-requests`, `GET /leave-types`, `GET /leave-balances/by-employee/{self}` |
| `self.overtime` | `/me/overtime`, `/me/overtime/new` | `GET/POST /overtime`, `POST /overtime/{id}:confirm`, `:withdraw` |
| `self.profile` | `/me/profile` | `GET /employees/{self}`, `PATCH /me/profile` *(instant self-edit; change-requests removed 2026-06-14)* |
| `self.payslip` | `/me/payslip` | `GET /payslips` (self) |

`agent` role bundle = **all seven** `self.*` keys. `/me/notifications` is auth-only (no key), like
the existing `/notifications`. **Data scope is server-enforced** (`scope: self` → the API resolves
the agent from the token and rejects/filters anyone else's rows); the client never sends another
employee's id. Client gates are **defense-in-depth only** (ENGINEERING.md C1).

## 5. Web clock-in (the detailed feature)

Ports `apps/mobile/app/(app)/attendance.tsx` to the browser. Lives at `/me/attendance` as a
**clock card** (top) + **attendance history** (below), mirroring mobile.

**Geolocation:** a new helper `apps/web/src/lib/geolocation.ts` wraps `navigator.geolocation
.getCurrentPosition` → `{ lat, lng } | null` (null on permission-denied / unavailable / timeout).
Requires a secure context (https or `localhost`) — fine for the dev server and prod.

**Clock-in flow** (`POST /attendance:clock-in`):
1. Acquire coords. If null → toast `clock.gpsDenied`, abort.
2. `mutateAsync({ lat, lng, gps_available: true, wfo: true, force_outside_geofence: false })`.
3. On `422 OUT_OF_GEOFENCE` → open a **ConfirmDialog** showing `fields.distance_m` / `radius_m`
   (`clock.outsideTitle` / `clock.outsideMsg`); on confirm, retry with `force_outside_geofence: true`.
4. On `409 ALLREADY_CLOCKED_IN` → refetch + info toast `clock.alreadyIn`.
5. On `422 GPS_UNAVAILABLE` → toast `clock.gpsUnavailable`. Else → generic `clock.error`.
6. On success → invalidate `['/attendance']`, success toast `clock.successIn`.

**Clock-out flow** (`POST /attendance:clock-out`): acquire coords → `mutateAsync({ lat, lng,
gps_available: true })`. Out-of-geofence on clock-out **never blocks** (server flags only).
`409 NOT_CLOCKED_IN` → toast `clock.notIn`. Success → invalidate + `clock.successOut`.

**Open-state detection:** `open = items.find(a => a.check_in_at && !a.check_out_at)` — an ABSENT
row has no `check_in_at` and must not count as open (matches mobile).

**States (B2, no dead flows):** acquiring-GPS (button spinner) · clocked-in vs not · out-of-geofence
confirm · already/not-clocked-in · GPS denied/unavailable · generic error · history loading/empty/error.

## 6. Component & layout rules (this pass)

- Reuse `packages/ui`: `StateView`, `StatusBadge`, `Button`, `Card`-style surfaces (the
  `rounded-xl border border-border bg-surface` pattern used across feature screens), `FormField`,
  `Input`, `useToast`, `ConfirmDialog`, `DataTable`/`StatCard` where they fit. **No new `packages/ui`
  components** unless a second domain-agnostic reuse appears (G2). A small agent page-shell wrapper
  (centered max-width column with header) may live in `features/agent/` as an organism.
- **Status colors only via `StatusBadge`** (DESIGN-SYSTEM §2 maps). Attendance: Hadir→ok(teal),
  Terlambat→warn, Tdk Lengkap→`#ED962F`, Absen→bad; verification auto→neutral, pending→warn,
  verified→info, rejected→bad.
- **i18n:** all copy via keys (Bahasa default, `en` fallback) under an **`agent` namespace**
  in `packages/shared/src/i18n/{id,en}.ts` — mirroring the mobile `m:*` strings.
- **Dates/times** via the `Asia/Jakarta` datetime layer (`@swp/shared/datetime`), never raw `new Date`.
- **Cursor pagination only** on history/list screens.

## 7. Build plan (foundation → parallel screens)

**Foundation (single-threaded — shared files):** `rbac.ts` keys + agent bundle + `WEB_ROLES`;
`nav.ts` `AGENT_NAV_ITEMS` + `/me/*` route requirements; `auth.ts`/`login` agent landing (`/me`);
`shell.tsx` role-based backbone + agent redirect from `/`; `geolocation.ts`; the full `agent` i18n
namespace (id+en); `router.tsx` `/me/*` route registration pointing at per-screen files; one stub
file per screen so the tree compiles.

**Screens (parallel — one feature file each, no shared-file edits):** `/me/attendance` (clock),
`/me` dashboard, `/me/schedule`, `/me/leave`(+new), `/me/overtime`(+new), `/me/profile`,
`/me/payslip`, `/me/notifications`(+`/me/correction`). Each traces to its mobile analog + the
endpoints in §4 and obeys §5–6.

## 8. Traceability

Epics: E5 (clock/attendance/corrections), E4 (schedule), E6 (leave), E7 (overtime), E2 (profile),
E8 (payslip), E10 (dashboard/notifications). RBAC: CONVENTIONS `x-rbac`. Mobile reference impl:
`apps/mobile/app/**`. Design reference frames listed in AW-6.
