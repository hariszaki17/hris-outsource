-- E10 F10.2 dashboard aggregations (GET /dashboards/me). Read-only live counts
-- over the existing E2..E8 tables — no rollup table (live for honesty, CONTEXT
-- decision). All scope params are nullable: NULL = global (HR/super), a company id
-- scopes to one company (shift_leader). The service passes role-derived scope.
--
-- SCHEMA-ALIGNMENT NOTE [11-01 DECISION]: the plan prose used placeholder enum
-- values that differ from the real Phase-7..9 schema. Aligned to reality:
--   * attendance.verification_status = 'PENDING' (NOT 'PENDING_VERIFY') —
--     00026_attendance.sql CHECK (AUTO_APPROVED/PENDING/VERIFIED/REJECTED/ESCALATED).
--     The leader queue counts PENDING + ESCALATED (both await action).
--   * leave_requests.status IN ('PENDING_L1','PENDING_HR') — 00028.
--   * overtime.status IN ('PENDING_L1','PENDING_HR') (NOT a bare 'PENDING') — 00031.
--   * placements use client_company_id + lifecycle_status + end_date — 00020.
--   * attendance carries company_id directly (denormalized); leave_requests +
--     overtime carry company_id (denormalized) too; schedule_entries does NOT, so
--     LeaderTodayStatus joins schedule_entries -> placements for company scope.

-- name: CountPendingAttendanceVerify :one
-- Attendance awaiting leader verification (PENDING or ESCALATED).
SELECT count(*)::bigint AS pending
FROM attendance
WHERE deleted_at IS NULL
  AND verification_status IN ('PENDING','ESCALATED')
  AND (sqlc.narg(company_id)::text IS NULL OR company_id = sqlc.narg(company_id)::text);

-- name: CountPendingLeaveApprove :one
-- Leave requests pending either approval level.
SELECT count(*)::bigint AS pending
FROM leave_requests
WHERE deleted_at IS NULL
  AND status IN ('PENDING_L1','PENDING_HR')
  AND (sqlc.narg(company_id)::text IS NULL OR company_id = sqlc.narg(company_id)::text);

-- name: CountPendingLeaveApproveHR :one
-- HR-level-only pending leave (HrDashboard.kpis.leave_pending = PENDING_HR).
SELECT count(*)::bigint AS pending
FROM leave_requests
WHERE deleted_at IS NULL
  AND status = 'PENDING_HR'
  AND (sqlc.narg(company_id)::text IS NULL OR company_id = sqlc.narg(company_id)::text);

-- name: CountPendingOtApprove :one
-- Overtime pending either approval level.
SELECT count(*)::bigint AS pending
FROM overtime
WHERE deleted_at IS NULL
  AND status IN ('PENDING_L1','PENDING_HR')
  AND (sqlc.narg(company_id)::text IS NULL OR company_id = sqlc.narg(company_id)::text);

-- name: CountExpiringPlacements30d :one
-- Active/expiring placements ending within the next 30 days (inclusive of today).
SELECT count(*)::bigint AS expiring
FROM placements
WHERE deleted_at IS NULL
  AND lifecycle_status IN ('ACTIVE','EXTENDED','EXPIRING')
  AND end_date IS NOT NULL
  AND end_date >= sqlc.arg(today)::date
  AND end_date <= (sqlc.arg(today)::date + 30)
  AND (sqlc.narg(company_id)::text IS NULL OR client_company_id = sqlc.narg(company_id)::text);

-- name: CountExpiringAgreements30d :one
-- Employment agreements ending within the next 30 days (HrDashboard).
SELECT count(*)::bigint AS expiring
FROM employment_agreements
WHERE deleted_at IS NULL
  AND status = 'active'
  AND end_date IS NOT NULL
  AND end_date >= sqlc.arg(today)::date
  AND end_date <= (sqlc.arg(today)::date + 30);

-- name: CountActivePlacements :one
-- HrDashboard.kpis.active_placements.
SELECT count(*)::bigint AS total
FROM placements
WHERE deleted_at IS NULL
  AND lifecycle_status IN ('ACTIVE','EXTENDED','EXPIRING');

-- name: CountActiveCompanies :one
-- HrDashboard.kpis.active_companies = companies with >=1 active placement.
SELECT count(DISTINCT client_company_id)::bigint AS total
FROM placements
WHERE deleted_at IS NULL
  AND lifecycle_status IN ('ACTIVE','EXTENDED','EXPIRING');

-- name: LeaderTodayStatus :one
-- LeaderDashboard.today: today's shift roll-up for one company. shifts_total from
-- schedule_entries (joined to placements for company scope); the attendance-derived
-- counts read today's attendance rows (check_in_at::date = today). COALESCE to 0.
WITH sched AS (
    SELECT count(*)::bigint AS shifts_total
    FROM schedule_entries se
    JOIN placements p ON p.id = se.placement_id
    WHERE se.deleted_at IS NULL
      AND se.is_day_off = false
      AND se.work_date = sqlc.arg(today)::date
      AND (sqlc.narg(company_id)::text IS NULL OR p.client_company_id = sqlc.narg(company_id)::text)
),
att AS (
    SELECT
        count(*) FILTER (WHERE check_out_at IS NOT NULL OR check_in_at IS NOT NULL)::bigint AS clocked_in,
        count(*) FILTER (WHERE is_late)::bigint                                              AS late_count,
        count(*) FILTER (WHERE status = 'ABSENT')::bigint                                    AS absent_count,
        count(*) FILTER (WHERE verification_status IN ('PENDING','ESCALATED'))::bigint       AS pending_verifications
    FROM attendance
    WHERE deleted_at IS NULL
      AND check_in_at::date = sqlc.arg(today)::date
      AND (sqlc.narg(company_id)::text IS NULL OR company_id = sqlc.narg(company_id)::text)
)
SELECT
    COALESCE(sched.shifts_total, 0)        AS shifts_total,
    COALESCE(att.clocked_in, 0)            AS clocked_in,
    COALESCE(att.late_count, 0)            AS late_count,
    COALESCE(att.absent_count, 0)          AS absent_count,
    COALESCE(att.pending_verifications, 0) AS pending_verifications
FROM sched, att;

-- name: AgentRecentAttendance :one
-- AgentDashboard.recent_attendance: last-7-day present/late/absent for one agent.
SELECT
    count(*) FILTER (WHERE status IN ('PRESENT','LATE'))::bigint AS last_7d_present,
    count(*) FILTER (WHERE is_late)::bigint                      AS last_7d_late,
    count(*) FILTER (WHERE status = 'ABSENT')::bigint            AS last_7d_absent
FROM attendance
WHERE deleted_at IS NULL
  AND employee_id = sqlc.arg(employee_id)
  AND check_in_at::date >= (sqlc.arg(today)::date - 7)
  AND check_in_at::date <= sqlc.arg(today)::date;

-- name: CountPendingRequestsForEmployee :one
-- AgentDashboard.pending_requests: this agent's own pending leave + OT.
SELECT
    (SELECT count(*) FROM leave_requests lr
       WHERE lr.deleted_at IS NULL AND lr.employee_id = sqlc.arg(employee_id)
         AND lr.status IN ('PENDING_L1','PENDING_HR'))::bigint AS leave_pending,
    (SELECT count(*) FROM overtime o
       WHERE o.deleted_at IS NULL AND o.employee_id = sqlc.arg(employee_id)
         AND o.status IN ('PENDING_AGENT_CONFIRM','PENDING_L1','PENDING_HR'))::bigint AS ot_pending;

-- =====================================================================
-- SuperAdminWidgets (DB-7) — admin-only dashboard block. Present ONLY when
-- role=super_admin; the queries below are global (no scope param). Each backs one
-- sub-widget of openapi schemas.SuperAdminWidgets.
-- =====================================================================

-- name: CountActiveUsers :one
-- user_access.active_users: login accounts with status 'active' (00002_users).
SELECT count(*)::bigint AS total
FROM users
WHERE deleted_at IS NULL
  AND status = 'active';

-- name: CountOffboardedUsers30d :one
-- user_access.offboarded_30d: users disabled within the last 30 days (F2.7).
-- The schema records offboarding as status='disabled' + a tokens_valid_after epoch
-- bump (00038); updated_at carries the disable instant. We count disabled users
-- whose tokens_valid_after (the revocation instant) falls in the last 30 days.
SELECT count(*)::bigint AS total
FROM users
WHERE deleted_at IS NULL
  AND status = 'disabled'
  AND tokens_valid_after >= (sqlc.arg(now_ts)::timestamptz - INTERVAL '30 days');

-- name: OrgRollupsByPosition :many
-- org_rollups: per-position headcount (distinct placed employees) + active
-- placement count, over non-terminal placements (mirrors CountActivePlacements).
-- Grouped by the FREE-TEXT placement position (decision 2026-06-12: service_line
-- removed, position is a plain text column on placements — no master/enum).
SELECT
    p.position                                AS position,
    count(DISTINCT p.employee_id)::bigint     AS headcount,
    count(*)::bigint                          AS active_placements
FROM placements p
WHERE p.deleted_at IS NULL
  AND p.lifecycle_status IN ('ACTIVE','EXTENDED','EXPIRING','PENDING_START')
GROUP BY p.position
ORDER BY p.position;

-- name: CountBankApprovalsPending :one
-- pending_grants.bank_approvals: change-requests with a bank change escalated to
-- HR/super-admin (00048 bank_pending flag → change_requests_bank_pending_idx; see
-- rbac.CanApproveBank / PARTIALLY_APPROVED).
SELECT count(*)::bigint AS total
FROM change_requests
WHERE bank_pending;

-- name: RecentAuditEntries :many
-- recent_audit: last sensitive admin actions, newest first (capped by row_limit ~8).
-- The schema (00004_audit_log) stores actor_user_id/actor_role/action/entity_type/
-- entity_id; the service composes actor_label/target_label from these (no
-- human-name columns exist on audit_log).
SELECT id, actor_user_id, actor_role, action, entity_type, entity_id, created_at
FROM audit_log
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);
