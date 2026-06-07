# API Conventions — hris-outsource

> Shared rules every per-epic OpenAPI spec inherits from. If a per-epic spec contradicts this doc, **this doc wins**.
>
> **Status:** v1 contract, framework-agnostic. Produced 2026-06-03.
> **Owner of changes:** product+architecture; coordinate edits with engineering.

---

## 1. API design philosophy

- **REST over JSON.** Resource-oriented; verbs in URLs only for actions that aren't CRUD (e.g., `:approve`, `:bulk-verify`).
- **Internal-only system.** All endpoints are authenticated. Public/unauthenticated endpoints are limited to `POST /api/v1/auth/login` and `POST /api/v1/auth/forgot-password`.
- **No GraphQL, no WebSocket, no SSE in v1.** Push notifications go through FCM/APNs (external infrastructure, not REST). Real-time leader dashboards use short-poll.
- **Framework-agnostic spec.** OpenAPI 3.1 YAML; the chosen backend framework picks its own routing & middleware.

---

## 2. Base URL & versioning

- **Base:** `https://api.swp.example.com/api/v1`
- **Versioning:** URL-prefix `/v1`. v2 is a parallel prefix when breaking changes are needed (e.g., `/v2`); v1 stays alive during deprecation. No version negotiation via header.
- **Deprecation:** `Sunset` HTTP response header on endpoints scheduled for removal; minimum 90-day notice.

---

## 3. Authentication

- **Header:** `Authorization: Bearer <token>` on every authenticated endpoint.
- **Token type:** opaque (server session) OR JWT. Final choice deferred to E1 platform-conventions; consumers MUST treat the token as opaque.
- **Token lifetime:** access token short-lived (~15-60 min); refresh via `POST /api/v1/auth/refresh` (E1 spec).
- **401 vs 403:**
  - `401 Unauthenticated` — missing/expired token. Client must re-auth.
  - `403 Forbidden` — token valid, but role lacks permission. Client must surface no-permission state (instance `comp/EmptyNoPermission`).
- **Session expired UX:** when client receives `401` mid-session, render the `comp/EmptySessionExpired` pattern (Wave-1 master) + re-auth flow.
- **Not purely stateless (F2.7):** the access-token check is NOT a pure stateless JWT verify — every request also validates `users.status` and `tokens_valid_after >= token.iat` (the session epoch). This enables instant revocation (offboarding / account-disable bumps the epoch) without maintaining a per-token denylist. Already-issued tokens stay cryptographically valid but fail this check on their next validation.

---

## 4. IDs

All resource IDs are **opaque strings** with the prefix `SWP-<ENTITY>-<NUMERIC>` (e.g., `SWP-EMP-1042`). Treat as strings; do not parse the numeric.

### Entity prefix table

| Prefix | Entity | Epic |
|---|---|---|
| `SWP-USR` | User account (login credential) | E1 |
| `SWP-AL` | Audit log entry | E1 |
| `SWP-EMP` | Employee (the agent/staff person) | E2 |
| `SWP-AG` | Employment Agreement (PKWT/PKWTT) | E2 |
| `SWP-CMP` | Client Company | E2 |
| `SWP-SITE` | Client Site (placement location + geofence) | E2 |
| `SWP-SVC` | Service Line | E2 |
| `SWP-POS` | Position | E2 |
| `SWP-LT` | Leave Type | E2 |
| `SWP-AC` | Attendance Code | E2 |
| `SWP-OTR` | Overtime Rule | E2 |
| `SWP-CHG` | Change Request (HR approval queue for agent edits) | E2 |
| `SWP-PL` | Placement | E3 |
| `SWP-SLA` | Shift Leader Assignment | E3 |
| `SWP-SHF` | Shift Master | E4 |
| `SWP-SCH` | Schedule entry (one shift assigned to one agent on one date) | E4 |
| `SWP-ATT` | Attendance record | E5 |
| `SWP-COR` | Attendance correction request | E5 |
| `SWP-LR` | Leave Request | E6 |
| `SWP-LQ` | Leave Quota (employee × leave_type × period) | E6 |
| `SWP-OT` | Overtime request | E7 |
| `SWP-HOL` | Public Holiday | E7 |
| `SWP-PS` | Payslip | E8 |
| `SWP-NTF` | Notification | E10 |
| `SWP-EXP` | Export job | E10 |

The numeric portion is monotonically allocated per-prefix; gaps are allowed (e.g., from soft-deletes).

### URL ID parameters

Use the full prefixed ID in URLs:

✅ `GET /api/v1/employees/SWP-EMP-1042`
❌ `GET /api/v1/employees/1042`

---

## 5. Naming

- **URL paths:** kebab-case, plural nouns (`/leave-requests`, `/client-companies`, `/audit-log`).
- **JSON fields:** snake_case (`employee_id`, `start_date`, `geofence_radius_m`).
- **Enum values:** UPPER_SNAKE_CASE for fixed sets (`status: "PENDING_L1"`), lowercase-kebab for free-form display tags.
- **Query parameters:** snake_case (`?service_line=facility_services&status=active`).
- **Action endpoints:** trailing `:verb` after the resource ID:
  - `POST /api/v1/leave-requests/SWP-LR-1042:approve-l1`
  - `POST /api/v1/attendance/SWP-ATT-10711:verify`
  - `POST /api/v1/placements/SWP-PL-882:transfer`

---

## 6. HTTP methods

| Method | Use |
|---|---|
| `GET` | Read (single or list). Idempotent, no side effects. |
| `POST` | Create OR action-with-side-effect (approve/reject/publish/transfer). Not necessarily idempotent — use idempotency key if needed (§13). |
| `PUT` | Full replacement of a resource. Rare in this system; prefer `PATCH`. |
| `PATCH` | Partial update (most edits). Body contains only changed fields. |
| `DELETE` | Soft-delete (sets `deleted_at`) unless explicitly documented as hard-delete. |

---

## 7. Status codes

| Code | Meaning |
|---|---|
| `200 OK` | Successful read or update with body |
| `201 Created` | Successful create; `Location` header points to the new resource |
| `202 Accepted` | Async operation queued (exports, bulk imports). Body returns a job ID. |
| `204 No Content` | Successful delete OR action with no useful body |
| `400 Bad Request` | Validation error; body has the error envelope |
| `401 Unauthenticated` | Missing/expired token |
| `403 Forbidden` | Authenticated but role-not-permitted |
| `404 Not Found` | Resource doesn't exist OR caller lacks visibility (treat same to avoid leaking) |
| `409 Conflict` | Invariant violation (INV-1 duplicate placement, double-shift, double-leader, etc.) |
| `410 Gone` | Resource permanently deleted (rare; usually 404) |
| `422 Unprocessable Entity` | Validation passed structurally but business rules failed (e.g., quota exceeded). Use 422 (not 400) when the error is *semantic*, not *syntactic*. |
| `429 Too Many Requests` | Rate-limited |
| `500 Internal Server Error` | Bug; `error.code: "INTERNAL"` |
| `503 Service Unavailable` | Maintenance window or upstream down |

### Status code conventions for this domain

- **Use 409 for INV-* violations** (E3 invariants, double-shift, etc.) — the request is well-formed but conflicts with current state.
- **Use 422 for quota/balance/rule violations** (E6 quota-exceeded, E7 OT < 30 min, E5 outside-geofence) — the request was processed but business rules rejected it.
- **Use 403 for RBAC violations** even if technically the resource exists.

---

## 8. Pagination

**Cursor-based by default.** Offset pagination is forbidden for tables that will exceed 100k rows (attendance, audit log, schedule).

### Request

```
GET /api/v1/audit-log?limit=50&cursor=eyJpZCI6IlNXUC1BTC0xMjA0MzgifQ
```

| Param | Default | Max | Notes |
|---|---|---|---|
| `limit` | 50 | 200 | Server may cap further on heavy tables |
| `cursor` | — | — | Opaque string returned by previous page |

### Response

```json
{
  "data": [ /* array of resources */ ],
  "next_cursor": "eyJpZCI6IlNXUC1BTC0xMjAxMjMifQ",
  "has_more": true
}
```

When `has_more: false`, omit or set `next_cursor: null`.

### Sort + filter alongside cursor

The cursor encodes the sort key. Changing sort order resets pagination — the cursor is invalidated if sort/filter params don't match its embedded state. Server should return `400 Bad Request` with `error.code: "CURSOR_MISMATCH"` in that case.

---

## 9. Filtering, sorting, searching

| Param | Usage |
|---|---|
| `q` | Free-text search across the resource's searchable fields (defined per endpoint) |
| `<field>` | Exact match on the field (e.g., `?status=ACTIVE`) |
| `<field>__in` | Multi-value: `?status__in=PENDING,APPROVED` |
| `<field>__gte`, `__lte`, `__gt`, `__lt` | Range queries (e.g., `?start_date__gte=2026-01-01`) |
| `sort` | `?sort=created_at:desc,name:asc` — comma-separated; suffix with `:asc`/`:desc` |

Filter operators applied to a non-existent field → `400 Bad Request` with `error.code: "UNKNOWN_FILTER"`.

---

## 10. Timestamps

- **Format:** ISO 8601 / RFC 3339 strings, UTC: `"2026-06-03T14:32:08Z"`.
- **Dates (no time):** `"2026-06-03"` (per RFC 3339 `full-date`).
- **Local-time fields** (e.g., shift `start_time`): `"09:00"` (HH:MM, 24-hour, always in `Asia/Jakarta` since SWP operates only in Indonesia v1).
- **Server is authoritative on `created_at`/`updated_at`** — clients must NOT send these in writes.
- **`tz_offset_minutes`** optional companion field on user-facing timestamps for client display (e.g., `+420` for WIB UTC+7).

---

## 11. Error envelope

Every 4xx/5xx response body uses:

```json
{
  "error": {
    "code": "QUOTA_EXCEEDED",
    "message": "Pengajuan melebihi kuota tersisa.",
    "fields": {
      "start_date": "Tanggal mulai di luar periode penempatan.",
      "duration_days": "Melebihi sisa kuota 3 hari."
    },
    "request_id": "req_01J5XK..."
  }
}
```

- **`code`**: UPPER_SNAKE_CASE machine-readable. Stable; clients can switch on it.
- **`message`**: human-readable; defaults to Indonesian (`Accept-Language` header may switch to `en-US`).
- **`fields`**: only on 400/422; maps form field name → field-level error. React form-validation showcases (Wave 3.3) wire to this.
- **`request_id`**: for support/debugging.

### Standard error codes (cross-cutting)

| Code | When |
|---|---|
| `INVALID_REQUEST` | Generic 400 (malformed JSON, missing required field) |
| `UNAUTHENTICATED` | 401 |
| `FORBIDDEN` | 403 |
| `NOT_FOUND` | 404 |
| `CONFLICT` | 409 generic |
| `INV_<N>_VIOLATION` | 409 for placement invariants (e.g., `INV_1_VIOLATION`) |
| `RULE_VIOLATION` | 422 generic |
| `QUOTA_EXCEEDED` | 422 (E6 leave, E7 OT) |
| `OUT_OF_GEOFENCE` | 422 (E5) |
| `SHIFT_OVER_LEAVE` | 409 (E4) |
| `DOUBLE_SHIFT` | 409 (E4) |
| `OUTSIDE_PLACEMENT_PERIOD` | 422 (E4) |
| `OUT_OF_SCOPE` | 403 (leader trying to act on non-own-company resource) |
| `RATE_LIMITED` | 429 |
| `INTERNAL` | 500 |
| `MAINTENANCE` | 503 |

Per-epic specs add domain-specific codes; they MUST follow `<DOMAIN>_<REASON>` format.

---

## 12. Validation

- **Required fields** marked `required: true` in OpenAPI. Server validates before writes.
- **Field-level errors** returned in `error.fields` (see §11).
- **Maximum string lengths** documented per schema; server truncates is forbidden — return `INVALID_REQUEST` instead.
- **Date-range fields** (start/end pairs) validated together; `end < start` → `INVALID_REQUEST` with `fields.end_date`.
- **Cross-field business rules** (e.g., agreement.end_date - start_date ≤ 5 years for PKWT) return 422 with a specific code (e.g., `PKWT_PERIOD_EXCEEDS_MAX`).

---

## 13. Idempotency

Critical create/action endpoints accept `Idempotency-Key` header (UUID v4 from client):

```
POST /api/v1/leave-requests
Idempotency-Key: 3c7e9a-...
```

Server caches the response by key for 24 h. Re-submitting with the same key returns the **same response** (including the same created resource ID). Different bodies under the same key → `409` with `error.code: "IDEMPOTENCY_KEY_REUSED"`.

Endpoints requiring idempotency key are flagged in each per-epic spec (typically: create leave/OT/correction, approve/reject actions, bulk operations, exports).

---

## 14. Bulk operations

Endpoints ending in `:bulk-*`:

- `POST /api/v1/attendance:bulk-verify`
- `POST /api/v1/overtime:bulk-approve`
- `POST /api/v1/leave-requests:bulk-approve` (rare)

Request body shape:

```json
{
  "ids": ["SWP-ATT-10711", "SWP-ATT-10712", ...],
  "note": "Optional audit note (per Wave-1 modal)"
}
```

Response shape (partial success allowed):

```json
{
  "succeeded": ["SWP-ATT-10711", ...],
  "failed": [
    {
      "id": "SWP-ATT-10712",
      "error": { "code": "...", "message": "..." }
    }
  ]
}
```

Status code: `200 OK` if any succeeded; `422` if all failed. Always 200 if ≥1 succeeded — clients render the partial-failure case from the `failed` array.

Bulk endpoints REQUIRE `Idempotency-Key`.

---

## 15. File uploads

Multipart endpoints documented as separate routes (don't mix JSON + files in one endpoint):

```
POST /api/v1/leave-requests/SWP-LR-1042/attachments
Content-Type: multipart/form-data

file: <binary>
caption: "Surat dokter"
```

- **Max size:** 10 MB per file (v1; configurable per endpoint).
- **Allowed types:** documented per endpoint (e.g., PDF/JPEG/PNG for leave docs; JPEG/PNG only for attendance photos).
- **Storage:** opaque to client. Response returns:

```json
{
  "id": "SWP-FILE-...",
  "url": "https://api.swp.example.com/api/v1/files/SWP-FILE-...",
  "name": "surat-dokter-rudi.pdf",
  "size_bytes": 245678,
  "mime": "application/pdf",
  "uploaded_at": "2026-06-03T14:32:08Z"
}
```

- **Download:** the `url` requires the same `Authorization` header; files are NOT publicly accessible.

---

## 16. Implicit side-effects (documented here, not on each endpoint)

### 16.1 Audit log

**Every** write endpoint (POST/PUT/PATCH/DELETE/action) automatically writes an audit log entry containing: actor, action, entity_type, entity_id, before-state, after-state, request_id, timestamp. Bulk endpoints write one entry per affected entity (so a bulk-approve of 10 items creates 10 audit entries).

Audit entries are queryable via E1 endpoints (`GET /api/v1/audit-log`).

Endpoints SHOULD NOT take an "audit comment" field in their body unless the field is **user-facing** (e.g., E8 HR audit-note is its own resource because the note is part of payslip metadata; that's distinct).

### 16.2 Notifications

These actions auto-dispatch notifications (created via the notification subsystem; consumed via E10 endpoints):

| Action | Recipients |
|---|---|
| Schedule published (E4) | Affected agents |
| Leave approved/rejected (E6) | Request submitter |
| OT approved/rejected/auto-detected (E7) | Request submitter |
| Attendance verification needed (E5) | Shift leader of the company |
| Attendance correction submitted (E5) | Shift leader (and HR if >7 days) |
| HR change-request submitted (E2) | HR admins |
| Agreement expiring within 30 days (E2) | HR admins (cron-driven) |
| Placement expiring within 30 days (E3) | HR admins + assigned leader (cron-driven) |

Notification dispatch is FIRE-AND-FORGET from the API's perspective. Failures don't block the originating action. Notifications are NOT a separate API call for the client.

### 16.3 Cache invalidation

Implicit; not exposed via API. Documented per epic if a write affects another epic's cached aggregates (e.g., approving a leave invalidates the E4 schedule grid for that agent's period).

---

## 17. RBAC matrix

Four roles (per CLAUDE.md):

- `super_admin` — full system config, user management, master data.
- `hr_admin` — manages employees, placements, master data; oversight & escalation approver.
- `shift_leader` — on-site supervisor for ONE client company; rosters, approvals, verification for that company's agents.
- `agent` — placed employee; clocks in/out, requests leave/OT, views own schedule.

Each endpoint in the per-epic specs declares allowed roles via the OpenAPI extension:

```yaml
x-rbac:
  roles: [hr_admin, super_admin]
  scope: company  # or 'global', 'self', 'company_or_global'
```

**Scope values:**
- `global` — caller can access all resources matching the endpoint
- `company` — caller can only access resources within their assigned company (shift leader)
- `self` — caller can only access resources for their own employee record (agent)
- `company_or_global` — leader sees their company; HR/super-admin sees all

Server enforces RBAC at controller/handler level; clients hide unauthorized actions but MUST tolerate `403` defensively.

---

## 18. Cross-epic shared endpoints

Per yesterday's decision (your call): **picker endpoints belong to their entity's epic, not a shared `/lookups/` namespace.** Examples:

- Employee picker → `GET /api/v1/employees?status=ACTIVE&q=...` (E2 endpoint, returns picker-shaped list)
- Client company picker → `GET /api/v1/client-companies?service_line=...` (E2)
- Service line picker → `GET /api/v1/service-lines` (E2)
- Position picker → `GET /api/v1/service-lines/SWP-SVC-001/positions` (E2)
- Shift leader picker → `GET /api/v1/employees?role=shift_leader&assigned=false` (E2)

This keeps RBAC and resource ownership clean.

---

## 19. Rate limiting

- **Per-user:** 600 requests/min sustained; 60 requests/sec burst.
- **Headers on every response:**
  - `X-RateLimit-Limit: 600`
  - `X-RateLimit-Remaining: 487`
  - `X-RateLimit-Reset: 1717419128` (unix seconds)
- **429 response body:** standard error envelope with `error.code: "RATE_LIMITED"` + `Retry-After` header.

Bulk endpoints are exempt from per-call counts but limited by aggregate throughput (documented per endpoint).

---

## 20. OpenAPI spec layout (per epic)

Each `docs/api/E#-name/openapi.yaml` MUST:

- Declare `openapi: 3.1.0`
- Declare `info` with title, version (`1.0.0`), description
- Reference shared schemas where possible (in v1 each spec is self-contained; v2 may extract shared components)
- Use `tags` to group endpoints by feature (matching the PRD feature names — e.g., E2 tags: `employees`, `agreements`, `client-companies`, `service-lines-positions`, `master-data`)
- Include `x-rbac` extension on every operation
- Include `x-design-screens` extension on every operation listing the .pen frame IDs that consume the endpoint (for traceability)
- Include realistic example payloads (NOT lorem ipsum) — use the personas from the design (Sari Hadi · HR Admin, Rudi Wijaya · Shift Leader at Plaza Senayan)

---

## 21. What's NOT in this contract (v1)

- WebSocket / SSE / GraphQL
- Webhooks (no third-party push)
- E9 migration API (one-shot script, no UI)
- Internal cron job endpoints (year-rollover, expiring-soon detection)
- Public/unauthenticated endpoints beyond login + forgot-password
- Mobile-specific endpoints (mobile uses the same API; client-specific behavior is handled in the app)
- File preview/transformation (clients download and render)
- Bulk import (E9-territory, future v1.1+)
