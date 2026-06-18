-- F8.5 Payroll Period Close — period FSM + completeness + summary snapshot +
-- clarification + adjustment ledger queries (PC-1..PC-15). All month anchoring on
-- the Asia/Jakarta wall-clock date, matching the established attendance basis
-- (COALESCE(shift_start_at, check_in_at) AT TIME ZONE 'Asia/Jakarta'; C-PC-2: a
-- record belongs to its shift-start month). Guarded transitions return the updated
-- row (or pgx.ErrNoRows when the guard rejects the state) so the service can map a
-- stale-state attempt to a 409 / 422.

-- ============================================================================
-- payroll_periods — FSM (PC-1, PC-7, PC-12)
-- ============================================================================

-- name: GetPayrollPeriod :one
-- The period for a month, if it already exists (NULL row -> service auto-creates).
SELECT id, year, month, status, locked_by, locked_at, force_locked,
       force_lock_reason, reopened_by, reopened_at, created_at, updated_at
FROM payroll_periods
WHERE year = sqlc.arg(year) AND month = sqlc.arg(month) AND deleted_at IS NULL;

-- name: GetPayrollPeriodForUpdate :one
-- Row-locked read for the lock/reopen transitions (re-check under the tx).
SELECT id, year, month, status, locked_by, locked_at, force_locked,
       force_lock_reason, reopened_by, reopened_at, created_at, updated_at
FROM payroll_periods
WHERE year = sqlc.arg(year) AND month = sqlc.arg(month) AND deleted_at IS NULL
FOR UPDATE;

-- name: CreatePayrollPeriod :one
-- Auto-create OPEN on first access (PC-1). ON CONFLICT keeps a concurrent creator's
-- row (the UNIQUE(year,month) race) — DO UPDATE no-ops so RETURNING always yields the
-- live row.
INSERT INTO payroll_periods (year, month, status)
VALUES (sqlc.arg(year), sqlc.arg(month), 'OPEN')
ON CONFLICT (year, month) DO UPDATE SET updated_at = payroll_periods.updated_at
RETURNING id, year, month, status, locked_by, locked_at, force_locked,
          force_lock_reason, reopened_by, reopened_at, created_at, updated_at;

-- name: LockPayrollPeriod :one
-- Guarded OPEN/REOPENED -> LOCKED transition (PC-7). Stamps locked_by/at + the
-- force fields. The WHERE status guard makes a double-lock a no-op (0 rows -> the
-- service treats it as already-locked).
UPDATE payroll_periods
SET status            = 'LOCKED',
    locked_by         = sqlc.narg(locked_by),
    locked_at         = now(),
    force_locked      = sqlc.arg(force_locked),
    force_lock_reason = sqlc.narg(force_lock_reason),
    updated_at        = now()
WHERE id = sqlc.arg(id)
  AND status IN ('OPEN','REOPENED')
  AND deleted_at IS NULL
RETURNING id, year, month, status, locked_by, locked_at, force_locked,
          force_lock_reason, reopened_by, reopened_at, created_at, updated_at;

-- name: ReopenPayrollPeriod :one
-- Guarded LOCKED -> REOPENED transition (PC-12), super-admin only (enforced in the
-- service). Clears the lock stamps; stamps reopened_by/at. Note status goes to
-- REOPENED (the §6 FSM) but the period is mutable again like OPEN (CanLock covers it).
UPDATE payroll_periods
SET status            = 'REOPENED',
    locked_by         = NULL,
    locked_at         = NULL,
    force_locked      = false,
    force_lock_reason = NULL,
    reopened_by       = sqlc.narg(reopened_by),
    reopened_at       = now(),
    updated_at        = now()
WHERE id = sqlc.arg(id)
  AND status = 'LOCKED'
  AND deleted_at IS NULL
RETURNING id, year, month, status, locked_by, locked_at, force_locked,
          force_lock_reason, reopened_by, reopened_at, created_at, updated_at;

-- ============================================================================
-- Completeness aggregation (PC-2..PC-5) — per employee, for (year, month).
-- The hard query. expected/clean/on_leave/blockers per FIELD-vs-INTERNAL employee.
-- Month anchor: attendance via COALESCE(shift_start_at, check_in_at) Jakarta date;
-- schedule via work_date; leave via leave_date; OT via work_date.
-- ============================================================================

-- name: ListPeriodCompleteness :many
-- Cursor-paged (employee_id ASC keyset). One row per employee that has ANY
-- schedule / attendance / leave / OT activity in the month. Counts:
--   expected            = workable schedule entries (is_day_off=false,
--                         status in SCHEDULED/MODIFIED) in-month
--   recorded            = attendance rows in-month
--   clean               = attendance with verification_status in VERIFIED/AUTO_APPROVED
--   on_leave            = approved_leave_days in-month
--   attendance_blockers = attendance PENDING/ESCALATED + open(check_out_at NULL)
--                         + is_payable-NULL no-shift (schedule_id NULL)
--   pending_ot          = overtime in-month with status='PENDING'
--   pending_leave       = leave_requests overlapping the month with status in
--                         (PENDING_L1, PENDING_HR)
-- coverage_gap is computed in the service (expected - (clean + on_leave), floored).
WITH
months AS (
    SELECT make_date(sqlc.arg(year)::int, sqlc.arg(month)::int, 1) AS m_start,
           (make_date(sqlc.arg(year)::int, sqlc.arg(month)::int, 1) + interval '1 month')::date AS m_end
),
sched AS (
    SELECT se.employee_id,
           count(*) FILTER (
               WHERE se.is_day_off = false
                 AND se.status IN ('SCHEDULED','MODIFIED')
           ) AS expected
    FROM schedule_entries se, months
    WHERE se.deleted_at IS NULL
      AND se.work_date >= months.m_start
      AND se.work_date <  months.m_end
    GROUP BY se.employee_id
),
att AS (
    SELECT a.employee_id,
           count(*) AS recorded,
           count(*) FILTER (
               WHERE a.verification_status IN ('VERIFIED','AUTO_APPROVED')
           ) AS clean,
           count(*) FILTER (
               WHERE a.verification_status IN ('PENDING','ESCALATED')
                  OR a.check_out_at IS NULL
                  OR (a.schedule_id IS NULL AND a.is_payable IS NULL)
           ) AS attendance_blockers
    FROM attendance a, months
    WHERE a.deleted_at IS NULL
      AND (COALESCE(a.shift_start_at, a.check_in_at) AT TIME ZONE 'Asia/Jakarta')::date >= months.m_start
      AND (COALESCE(a.shift_start_at, a.check_in_at) AT TIME ZONE 'Asia/Jakarta')::date <  months.m_end
    GROUP BY a.employee_id
),
lv AS (
    SELECT ald.employee_id,
           count(*) AS on_leave
    FROM approved_leave_days ald, months
    WHERE ald.leave_date >= months.m_start
      AND ald.leave_date <  months.m_end
    GROUP BY ald.employee_id
),
ot AS (
    SELECT o.employee_id,
           count(*) FILTER (WHERE o.status = 'PENDING') AS pending_ot
    FROM overtime o, months
    WHERE o.deleted_at IS NULL
      AND o.work_date >= months.m_start
      AND o.work_date <  months.m_end
    GROUP BY o.employee_id
),
plv AS (
    SELECT lr.employee_id,
           count(*) FILTER (WHERE lr.status IN ('PENDING_L1','PENDING_HR')) AS pending_leave
    FROM leave_requests lr, months
    WHERE lr.deleted_at IS NULL
      AND lr.start_date <  months.m_end
      AND lr.end_date   >= months.m_start
    GROUP BY lr.employee_id
),
ids AS (
    SELECT employee_id FROM sched
    UNION SELECT employee_id FROM att
    UNION SELECT employee_id FROM lv
    UNION SELECT employee_id FROM ot
    UNION SELECT employee_id FROM plv
)
SELECT e.id AS employee_id,
       e.full_name AS employee_name,
       e.employee_type,
       COALESCE(sched.expected, 0)::int             AS expected,
       COALESCE(att.recorded, 0)::int               AS recorded,
       COALESCE(att.clean, 0)::int                  AS clean,
       COALESCE(lv.on_leave, 0)::int                AS on_leave,
       COALESCE(att.attendance_blockers, 0)::int    AS attendance_blockers,
       COALESCE(ot.pending_ot, 0)::int              AS pending_ot,
       COALESCE(plv.pending_leave, 0)::int          AS pending_leave
FROM ids
JOIN employees e        ON e.id = ids.employee_id
LEFT JOIN sched ON sched.employee_id = ids.employee_id
LEFT JOIN att   ON att.employee_id   = ids.employee_id
LEFT JOIN lv    ON lv.employee_id    = ids.employee_id
LEFT JOIN ot    ON ot.employee_id    = ids.employee_id
LEFT JOIN plv   ON plv.employee_id   = ids.employee_id
WHERE (sqlc.narg(cursor_id)::text IS NULL OR e.id > sqlc.narg(cursor_id)::text)
ORDER BY e.id ASC
LIMIT sqlc.arg(page_limit);

-- name: CountOpenClarifications :one
-- Number of OPEN clarifications for a period (the "awaiting reply" badge on the
-- cockpit header, PC-13 / F8.5). Restored 2026-06-18 — the source query had drifted
-- out of periods.sql while the generated func + period_repo.go call remained.
SELECT count(*)::int AS open_count
FROM clarification_requests
WHERE period_id = sqlc.arg(period_id)::text
  AND status = 'OPEN'
  AND deleted_at IS NULL;

-- name: CountFieldBlockers :one
-- The cockpit per-tab blocker tally (PC-7 gate) — FIELD employees ONLY (PC-5).
-- Returns the three counts the lock precondition checks (attendance / overtime /
-- leave). attendance = PENDING/ESCALATED + open + is_payable-NULL no-shift +
-- coverage gaps (expected schedule rows with neither a clean record nor leave is
-- approximated here as the attendance-blocker rows; the per-employee coverage gap
-- is surfaced in ListPeriodCompleteness, this aggregate counts the record-level
-- blockers which gate the same way).
WITH
months AS (
    SELECT make_date(sqlc.arg(year)::int, sqlc.arg(month)::int, 1) AS m_start,
           (make_date(sqlc.arg(year)::int, sqlc.arg(month)::int, 1) + interval '1 month')::date AS m_end
),
field_emp AS (
    SELECT id FROM employees WHERE employee_type = 'FIELD' AND deleted_at IS NULL
)
SELECT
    (SELECT count(*) FROM attendance a, months
       WHERE a.deleted_at IS NULL
         AND a.employee_id IN (SELECT id FROM field_emp)
         AND (COALESCE(a.shift_start_at, a.check_in_at) AT TIME ZONE 'Asia/Jakarta')::date >= months.m_start
         AND (COALESCE(a.shift_start_at, a.check_in_at) AT TIME ZONE 'Asia/Jakarta')::date <  months.m_end
         AND (a.verification_status IN ('PENDING','ESCALATED')
              OR a.check_out_at IS NULL
              OR (a.schedule_id IS NULL AND a.is_payable IS NULL))
    )::int AS attendance_blockers,
    (SELECT count(*) FROM overtime o, months
       WHERE o.deleted_at IS NULL
         AND o.employee_id IN (SELECT id FROM field_emp)
         AND o.work_date >= months.m_start
         AND o.work_date <  months.m_end
         AND o.status = 'PENDING'
    )::int AS overtime_blockers,
    (SELECT count(*) FROM leave_requests lr, months
       WHERE lr.deleted_at IS NULL
         AND lr.employee_id IN (SELECT id FROM field_emp)
         AND lr.start_date <  months.m_end
         AND lr.end_date   >= months.m_start
         AND lr.status IN ('PENDING_L1','PENDING_HR')
    )::int AS leave_blockers;

-- ============================================================================
-- Lock snapshot (PC-8) — materialize the immutable per-FIELD-employee summary.
-- One INSERT-SELECT off the SAME aggregation as completeness, FIELD employees only.
-- ============================================================================

-- name: SnapshotPeriodSummaries :execrows
-- Materialize PeriodEmployeeSummary for every FIELD employee with month activity
-- (PC-8). present/late/absent/no_shift_payable/worked from attendance; payable_days
-- = clean+payable rows; approved_ot_minutes from APPROVED OT; paid/unpaid leave from
-- approved_leave_days. Idempotent via ON CONFLICT DO NOTHING (a re-lock can't dup).
WITH
months AS (
    SELECT make_date(sqlc.arg(year)::int, sqlc.arg(month)::int, 1) AS m_start,
           (make_date(sqlc.arg(year)::int, sqlc.arg(month)::int, 1) + interval '1 month')::date AS m_end
),
field_emp AS (
    SELECT id FROM employees WHERE employee_type = 'FIELD' AND deleted_at IS NULL
),
att AS (
    SELECT a.employee_id,
           count(*) FILTER (
               WHERE a.verification_status IN ('VERIFIED','AUTO_APPROVED')
                 AND a.status <> 'ABSENT'
                 AND a.is_payable IS DISTINCT FROM false
           ) AS payable_days,
           count(*) FILTER (WHERE a.status IN ('PRESENT','LATE','INCOMPLETE')) AS present_days,
           count(*) FILTER (WHERE a.is_late = true) AS late_days,
           count(*) FILTER (WHERE a.status = 'ABSENT') AS absent_days,
           count(*) FILTER (
               WHERE a.schedule_id IS NULL AND a.is_payable = true
           ) AS no_shift_payable_days,
           COALESCE(sum(a.worked_minutes) FILTER (WHERE a.worked_minutes IS NOT NULL), 0) AS worked_minutes
    FROM attendance a, months
    WHERE a.deleted_at IS NULL
      AND a.employee_id IN (SELECT id FROM field_emp)
      AND (COALESCE(a.shift_start_at, a.check_in_at) AT TIME ZONE 'Asia/Jakarta')::date >= months.m_start
      AND (COALESCE(a.shift_start_at, a.check_in_at) AT TIME ZONE 'Asia/Jakarta')::date <  months.m_end
    GROUP BY a.employee_id
),
ot AS (
    SELECT o.employee_id,
           COALESCE(sum(o.counted_minutes) FILTER (WHERE o.status = 'APPROVED'), 0) AS approved_ot_minutes
    FROM overtime o, months
    WHERE o.deleted_at IS NULL
      AND o.employee_id IN (SELECT id FROM field_emp)
      AND o.work_date >= months.m_start
      AND o.work_date <  months.m_end
    GROUP BY o.employee_id
),
lv AS (
    SELECT ald.employee_id,
           count(*) FILTER (WHERE ald.is_payable = true) AS paid_leave_days,
           count(*) FILTER (WHERE ald.is_payable IS DISTINCT FROM true) AS unpaid_leave_days
    FROM approved_leave_days ald, months
    WHERE ald.employee_id IN (SELECT id FROM field_emp)
      AND ald.leave_date >= months.m_start
      AND ald.leave_date <  months.m_end
    GROUP BY ald.employee_id
),
ids AS (
    SELECT employee_id FROM att
    UNION SELECT employee_id FROM ot
    UNION SELECT employee_id FROM lv
)
INSERT INTO period_employee_summaries (
    period_id, employee_id, payable_days, present_days, late_days, absent_days,
    no_shift_payable_days, worked_minutes, approved_ot_minutes,
    paid_leave_days, unpaid_leave_days
)
SELECT sqlc.arg(period_id)::text,
       ids.employee_id,
       COALESCE(att.payable_days, 0)::int,
       COALESCE(att.present_days, 0)::int,
       COALESCE(att.late_days, 0)::int,
       COALESCE(att.absent_days, 0)::int,
       COALESCE(att.no_shift_payable_days, 0)::int,
       COALESCE(att.worked_minutes, 0)::int,
       COALESCE(ot.approved_ot_minutes, 0)::int,
       COALESCE(lv.paid_leave_days, 0)::int,
       COALESCE(lv.unpaid_leave_days, 0)::int
FROM ids
LEFT JOIN att ON att.employee_id = ids.employee_id
LEFT JOIN ot  ON ot.employee_id  = ids.employee_id
LEFT JOIN lv  ON lv.employee_id  = ids.employee_id
ON CONFLICT (period_id, employee_id) DO NOTHING;

-- name: VoidPeriodSummaries :execrows
-- Reopen voids the immutable summary set (PC-12). Hard DELETE — the snapshot is
-- regenerated on the next lock.
DELETE FROM period_employee_summaries WHERE period_id = sqlc.arg(period_id);

-- name: ListPeriodSummaries :many
-- The immutable per-employee summary set (post-lock; §9 GET summary). Cursor-paged
-- (employee_id ASC keyset). LEFT JOIN employees for the display name.
SELECT s.id, s.period_id, s.employee_id, e.full_name AS employee_name,
       s.payable_days, s.present_days, s.late_days, s.absent_days,
       s.no_shift_payable_days, s.worked_minutes, s.approved_ot_minutes,
       s.paid_leave_days, s.unpaid_leave_days, s.created_at
FROM period_employee_summaries s
LEFT JOIN employees e ON e.id = s.employee_id
WHERE s.period_id = sqlc.arg(period_id)
  AND (sqlc.narg(cursor_id)::text IS NULL OR s.employee_id > sqlc.narg(cursor_id)::text)
ORDER BY s.employee_id ASC
LIMIT sqlc.arg(page_limit);

-- ============================================================================
-- clarification_requests (PC-13)
-- ============================================================================

-- name: CreateClarificationRequest :one
INSERT INTO clarification_requests (
    source_type, source_id, period_id, raised_by, target_employee_id, question
) VALUES (
    sqlc.arg(source_type), sqlc.arg(source_id), sqlc.narg(period_id),
    sqlc.narg(raised_by), sqlc.arg(target_employee_id), sqlc.arg(question)
)
RETURNING id, source_type, source_id, period_id, raised_by, target_employee_id,
          question, status, answer, answered_by, answered_at, created_at, updated_at;

-- name: GetClarificationRequest :one
SELECT id, source_type, source_id, period_id, raised_by, target_employee_id,
       question, status, answer, answered_by, answered_at, created_at, updated_at
FROM clarification_requests
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetClarificationRequestForUpdate :one
SELECT id, source_type, source_id, period_id, raised_by, target_employee_id,
       question, status, answer, answered_by, answered_at, created_at, updated_at
FROM clarification_requests
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
FOR UPDATE;

-- name: AnswerClarificationRequest :one
-- OPEN -> ANSWERED (PC-13). Guarded to OPEN (0 rows -> already answered/closed).
UPDATE clarification_requests
SET status      = 'ANSWERED',
    answer      = sqlc.arg(answer),
    answered_by = sqlc.narg(answered_by),
    answered_at = now(),
    updated_at  = now()
WHERE id = sqlc.arg(id) AND status = 'OPEN' AND deleted_at IS NULL
RETURNING id, source_type, source_id, period_id, raised_by, target_employee_id,
          question, status, answer, answered_by, answered_at, created_at, updated_at;

-- name: SetClarificationStatus :one
-- HR closes the loop: RESOLVED or CANCELLED (PC-13). Allowed from OPEN or ANSWERED.
UPDATE clarification_requests
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status IN ('OPEN','ANSWERED')
  AND deleted_at IS NULL
RETURNING id, source_type, source_id, period_id, raised_by, target_employee_id,
          question, status, answer, answered_by, answered_at, created_at, updated_at;

-- ============================================================================
-- payroll_adjustments — the F8.3 carry-forward ledger (PC-9)
-- ============================================================================

-- name: CreatePayrollAdjustment :one
-- Emit a PENDING, unvalued adjustment for a post-lock change (PC-9). amount stays
-- NULL until the F8.3 run values it.
INSERT INTO payroll_adjustments (
    employee_id, source_type, source_id, origin_year, origin_month, note
) VALUES (
    sqlc.arg(employee_id), sqlc.arg(source_type), sqlc.arg(source_id),
    sqlc.arg(origin_year), sqlc.arg(origin_month), sqlc.narg(note)
)
RETURNING id, employee_id, source_type, source_id, origin_year, origin_month,
          note, amount, status, applied_run_id, created_at, updated_at;

-- ============================================================================
-- Clarification target resolution (PC-13 / C-PC-6): for a source record, the
-- owning agent + company; then the company's active shift-leader (else the agent).
-- ============================================================================

-- name: GetAttendanceOwner :one
SELECT employee_id, company_id FROM attendance WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetOvertimeOwner :one
SELECT employee_id, company_id FROM overtime WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetLeaveRequestOwner :one
SELECT employee_id, company_id FROM leave_requests WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetActiveCompanyLeaderEmployee :one
-- The active company-scope shift-leader's employee id (PC-13 target). NULL row when
-- the company has no active leader -> the service falls back to the agent (C-PC-6).
SELECT employee_id
FROM shift_leader_assignments
WHERE client_company_id = sqlc.arg(client_company_id)
  AND site_id IS NULL
  AND unassigned_at IS NULL
LIMIT 1;

-- name: ListPayrollAdjustments :many
-- Pending/applied ledger for an employee (run consumption + cockpit). Cursor-paged
-- (id DESC keyset).
SELECT id, employee_id, source_type, source_id, origin_year, origin_month,
       note, amount, status, applied_run_id, created_at, updated_at
FROM payroll_adjustments
WHERE deleted_at IS NULL
  AND (sqlc.narg(employee_id)::text IS NULL OR employee_id = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(status)::text      IS NULL OR status      = sqlc.narg(status)::text)
  AND (sqlc.narg(cursor_id)::text   IS NULL OR id < sqlc.narg(cursor_id)::text)
ORDER BY id DESC
LIMIT sqlc.arg(page_limit);
