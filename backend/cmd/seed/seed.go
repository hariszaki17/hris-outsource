package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
)

// Known passwords for the four deterministic test personas. Exported so the
// E2E harness (01-01) and persona registry can reference the same values
// without hard-coding literals in multiple places.
const (
	PasswordHRAdmin     = "Pass1ng-Garuda!"
	PasswordShiftLeader = "Lead3r-Senayan!"
	PasswordSuperAdmin  = "Sup3r-Admin-2026!"
	PasswordAgent       = "Ag3nt-Budi-2026!"
)

// persona holds the data required to seed a single user row.
type persona struct {
	email      string
	phone      string // E.164 login identifier (D2)
	password   string
	role       string
	fullName   string
	employeeID *string
	companyID  *string
}

func strPtr(s string) *string { return &s }

// personas is the authoritative list of Phase-1 test users.
// Later phases append their own fixtures (client company "Plaza Senayan",
// site, service line, employee, placement, etc.) to this same Seed function,
// so the seed grows per phase per the harness spec.
//
// Phase markers:
//   - Phase 1: four core personas (hr_admin, shift_leader, super_admin, agent)
//   - Phase 2+: add client-company row "Plaza Senayan" (SWP-CMP-0021), employee
//     records, placements, shifts, etc. as each epic's screens are wired up.
var personas = []persona{
	{
		email:      "sari.hadi@swp.test",
		phone:      "+628110000042",
		password:   PasswordHRAdmin,
		role:       "hr_admin",
		fullName:   "Sari Hadi",
		employeeID: strPtr("SWP-EMP-1042"),
		companyID:  nil,
	},
	{
		// Shift leader scoped to "Plaza Senayan". The companies table lands in
		// Phase 3; SWP-CMP-0021 is the deterministic literal used across the
		// harness spec (FK not enforced until the companies migration is applied).
		email:      "rudi.wijaya@swp.test",
		phone:      "+628110001108",
		password:   PasswordShiftLeader,
		role:       "shift_leader",
		fullName:   "Rudi Wijaya",
		employeeID: strPtr("SWP-EMP-1108"),
		companyID:  strPtr("SWP-CMP-0021"),
	},
	{
		email:      "super.admin@swp.test",
		phone:      "+628110000001",
		password:   PasswordSuperAdmin,
		role:       "super_admin",
		fullName:   "Super Admin",
		employeeID: nil,
		companyID:  nil,
	},
	{
		email:      "agent.budi@swp.test",
		phone:      "+628110002891",
		password:   PasswordAgent,
		role:       "agent",
		fullName:   "Budi Santoso",
		employeeID: strPtr("SWP-EMP-2891"),
		companyID:  nil,
	},
}

// extraPersonas are additional users added in Phase 2 so the user list
// has enough rows to exercise cursor pagination in the E1 screens.
var extraPersonas = []persona{
	{
		email:      "dewi.lestari@swp.test",
		phone:      "+628110003001",
		password:   "Dew1-Lestari-2026!",
		role:       "agent",
		fullName:   "Dewi Lestari",
		employeeID: strPtr("SWP-EMP-3001"),
		companyID:  nil,
	},
	{
		email:      "agus.pratama@swp.test",
		phone:      "+628110003002",
		password:   "Agus-Pr4tama-2026!",
		role:       "shift_leader",
		fullName:   "Agus Pratama",
		employeeID: strPtr("SWP-EMP-3002"),
		companyID:  strPtr("SWP-CMP-0021"),
	},
	{
		email:      "bambang.admin@swp.test",
		phone:      "+628110003003",
		password:   "B4mbang-Admin-2026!",
		role:       "hr_admin",
		fullName:   "Bambang Sutrisno",
		employeeID: strPtr("SWP-EMP-3003"),
		companyID:  nil,
	},
	{
		// LEAD persona (service-line operational approver). users.role='lead' is a
		// STORED role (not derived). companyID stays nil — a lead's company SET is
		// resolved per-request from lead_assignments (see seedPlacements below). This
		// lead covers SWP-CMP-0021 AND SWP-CMP-0022 so cross-company scope can be
		// exercised end-to-end.
		email:      "joko.lead@swp.test",
		phone:      "+628110003004",
		password:   "Joko-Pr4tama-2026!",
		role:       "lead",
		fullName:   "Joko Pratama",
		employeeID: strPtr("SWP-EMP-3004"),
		companyID:  nil,
	},
}

// Seed inserts the deterministic test personas into the database. It is
// idempotent: if a non-deleted user with the same email already exists, that
// persona is skipped (no error, no duplicate). Safe to re-run between test
// runs or after a migrate-up on an empty DB.
func Seed(ctx context.Context, pool *db.Pool) error {
	q := sqlcgen.New(pool.Pool)

	// -----------------------------------------------------------------------
	// Phase 4 (04-02): Seed employee rows BEFORE the persona user loop.
	//
	// CRITICAL ORDERING: The persona user rows reference employeeID literals
	// (SWP-EMP-1042, SWP-EMP-1108, SWP-EMP-2891, SWP-EMP-3001/3002/3003).
	// Those IDs must exist in the employees table before CreateUser sets
	// employee_id on each user (FK not enforced but the row must exist for
	// /auth/me to resolve the employee record).
	// -----------------------------------------------------------------------
	if err := seedEmployees(ctx, pool); err != nil {
		return fmt.Errorf("seed employees: %w", err)
	}

	allPersonas := append(personas, extraPersonas...)

	for _, p := range allPersonas {
		existing, err := q.GetUserByEmail(ctx, p.email)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check existing user %q: %w", p.email, err)
		}
		if err == nil {
			// User already exists — skip (idempotent).
			slog.Info("seed: skipping existing user", "email", p.email, "id", existing.ID)
			continue
		}

		hash, err := auth.HashPassword(p.password)
		if err != nil {
			return fmt.Errorf("hash password for %q: %w", p.email, err)
		}

		user, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
			Email:        strPtr(p.email),
			Phone:        strPtr(p.phone),
			PasswordHash: hash,
			Role:         p.role,
			FullName:     p.fullName,
			EmployeeID:   p.employeeID,
			CompanyID:    p.companyID,
		})
		if err != nil {
			return fmt.Errorf("create user %q: %w", p.email, err)
		}

		// Back-fill the reverse link employees.user_id so has_login (derived from
		// user_id) and the role/assigned employee filters resolve. CreateUser only
		// sets users.employee_id; the 1:1 link is bidirectional (see EP-3 /
		// SetEmployeeUserID). Without this, every seeded employee reads has_login=false.
		if p.employeeID != nil {
			if err := q.SetEmployeeUserID(ctx, sqlcgen.SetEmployeeUserIDParams{
				UserID: &user.ID,
				ID:     *p.employeeID,
			}); err != nil {
				return fmt.Errorf("link employee %q to user %q: %w", *p.employeeID, user.ID, err)
			}
		}
		slog.Info("seed: created user", "email", user.Email, "id", user.ID, "role", user.Role)
	}

	// Seed audit_log rows so the E1 audit-log screen has content.
	// Guard: skip if we already have audit rows (idempotent re-runs).
	if err := seedAuditLog(ctx, pool); err != nil {
		return fmt.Errorf("seed audit_log: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 3 (03-02): Seed client companies + sites.
	// "Plaza Senayan" SWP-CMP-0021 is the shift_leader persona's company scope
	// target — its literal ID is referenced in the Phase-1 persona and must
	// exist in client_companies for FK to resolve. Inserted with ON CONFLICT DO
	// NOTHING so re-runs are idempotent.
	// -----------------------------------------------------------------------
	if err := seedClientCompanies(ctx, pool); err != nil {
		return fmt.Errorf("seed client_companies: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 3 (03-03): Seed service lines + Parking positions.
	// 3 canonical service lines (SWP-SVC-001/002/003) with explicit IDs so E2E
	// tests can reference deterministic IDs. Parking gets 2 seeded positions
	// (SWP-POS-014, SWP-POS-015) per the OpenAPI spec examples.
	// -----------------------------------------------------------------------
	if err := seedServiceLines(ctx, pool); err != nil {
		return fmt.Errorf("seed service_lines: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 3 (03-04): Seed operational master data.
	// Canonical leave types, attendance codes, and default overtime rule so the
	// E2 master-data screens have content on first load.
	// -----------------------------------------------------------------------
	if err := seedMasterData(ctx, pool); err != nil {
		return fmt.Errorf("seed master_data: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 4 (04-03): Seed employment agreements + attachments.
	// FK: employment_agreements → employees (must run AFTER seedEmployees).
	// FK: agreement_attachments → employment_agreements (must run after agreements).
	// -----------------------------------------------------------------------
	if err := seedAgreements(ctx, pool); err != nil {
		return fmt.Errorf("seed agreements: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 4 (04-04) + EP-5 redesign (2026-06-11): Seed pending change-requests.
	// FK: change_requests → employees (must run AFTER seedEmployees); the SL
	// company-scope routing also relies on seedPlacements (run later) placing the
	// submitter at a client company. Three PENDING CRs:
	//   SWP-CHG-2117  Budi @ CMP-0022  MULTIPLE (phone+bank)  → SL out-of-company 403
	//   SWP-CHG-2119  Dewi @ CMP-0021  MULTIPLE (phone+bank)  → SL bank-split → HR finalize
	//   SWP-CHG-2120  Dewi @ CMP-0021  EMERGENCY_CONTACT only → SL/HR full approve / reject
	// -----------------------------------------------------------------------
	if err := seedChangeRequests(ctx, pool); err != nil {
		return fmt.Errorf("seed change_requests: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 5 (05-02): Seed placements + shift-leader assignment (E3).
	// FK: placements → employees / agreements / client_companies / client_sites /
	// service_lines / positions (must run AFTER seedAgreements + seedServiceLines +
	// seedClientCompanies). Adds the persona agreements that were missing first.
	// -----------------------------------------------------------------------
	if err := seedPlacements(ctx, pool); err != nil {
		return fmt.Errorf("seed placements: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 6 (06-02): Seed E4 scheduling fixtures.
	// FK: schedule_entries → placements/employees/shift_masters (must run AFTER
	// seedPlacements). Seeds shift masters, a couple of in-week schedule entries
	// at CMP-0021, and one approved_leave_days row so SHIFT_OVER_LEAVE fires.
	// -----------------------------------------------------------------------
	if err := seedScheduling(ctx, pool); err != nil {
		return fmt.Errorf("seed scheduling: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 7 (07-02): Seed E5 attendance + correction fixtures.
	// FK: attendance → placements/employees/schedule_entries/client_companies;
	// attendance_corrections → attendance/employees/client_companies (must run
	// AFTER seedScheduling). Plants AUTO_APPROVED + PENDING exceptions at
	// CMP-0021/CMP-0022, a leader-own ESCALATED record (VERIFY_OWN_RECORD), and
	// PENDING corrections (one in-window for approve, one for reject).
	// -----------------------------------------------------------------------
	if err := seedAttendance(ctx, pool); err != nil {
		return fmt.Errorf("seed attendance: %w", err)
	}
	if err := seedCorrections(ctx, pool); err != nil {
		return fmt.Errorf("seed corrections: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 8 (08-02): Seed E6 leave fixtures — quotas + Pending leave_requests
	// (web/HR/leader APPROVAL targets; agent CREATE is mobile-only / out of web
	// scope). Runs AFTER seedScheduling so SWP-SCH-6002 exists to overlap for the
	// INV-3 loop-closer E2E. FK: leave_requests → employees/placements/companies/
	// leave_types; leave_quotas → employees/leave_types; leave_approvals →
	// leave_requests. Idempotent (ON CONFLICT (id) DO NOTHING / NOT EXISTS guard).
	// -----------------------------------------------------------------------
	if err := seedLeave(ctx, pool); err != nil {
		return fmt.Errorf("seed leave: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 9 (09-02): Seed E7 overtime + holiday fixtures — the web HR/leader
	// OT APPROVAL targets (agent capture/auto-detect is mobile/system / out of
	// web scope; OT records incl. PENDING_AGENT_CONFIRM candidates are seeded
	// directly). FK: overtime → employees/placements/companies/overtime_rules/
	// holidays; holidays is the master that feeds day_type. Holidays MUST seed
	// before overtime (SWP-OT-30009 references SWP-HOL-9001 for HOLIDAY_IN_USE).
	// Idempotent (ON CONFLICT (id) DO NOTHING).
	// -----------------------------------------------------------------------
	if err := seedHolidays(ctx, pool); err != nil {
		return fmt.Errorf("seed holidays: %w", err)
	}
	if err := seedOvertime(ctx, pool); err != nil {
		return fmt.Errorf("seed overtime: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 10 (10-02): Seed E8 payroll fixtures — the web HR/Super-Admin
	// payroll archive targets. Payslips are HISTORICAL/read-only (no payroll
	// run in this milestone), so they are seeded directly with money encrypted
	// AES-256-GCM (INV-2) under the SAME PAYROLL_ENCRYPTION_KEY the API decrypts
	// with. Includes a DELIBERATELY-CORRUPT ciphertext row (SWP-PS-90119) so the
	// API's Decrypt returns ErrDecrypt at read time and the row surfaces
	// DECRYPT_FAIL honestly (not a hardcoded flag), plus two audit notes on it.
	// FK: payslips → employees (SWP-EMP-1042/1108/2891/3001). Idempotent.
	// -----------------------------------------------------------------------
	if err := seedPayroll(ctx, pool); err != nil {
		return fmt.Errorf("seed payroll: %w", err)
	}

	// -----------------------------------------------------------------------
	// Phase 11 (11-02): Seed E10 notifications so the in-app inbox list +
	// mark-read flows render for the personas. These are SEED-ONLY (the
	// auto-dispatch loop-closer is proven by a REAL action in 11-04). Recipients
	// are the persona EMPLOYEE ids (SWP-EMP-1042 HR Sari, SWP-EMP-2891 agent Budi)
	// — deterministic, and the notification service scopes List on the principal's
	// (user id, employee id) pair so both render. Mixed read/unread across kinds.
	// Explicit SWP-NTF-9000x ids + ON CONFLICT (id) DO NOTHING for idempotent E2E.
	// -----------------------------------------------------------------------
	if err := seedNotifications(ctx, pool); err != nil {
		return fmt.Errorf("seed notifications: %w", err)
	}

	return nil
}

// seedNotifications inserts ~6 in-app notification fixtures (mixed read/unread,
// across kinds) for the seeded personas so the notifications list + mark-read +
// mark-all-read flows render. Idempotent (ON CONFLICT (id) DO NOTHING).
func seedNotifications(ctx context.Context, pool *db.Pool) error {
	now := time.Now()
	ts := func(d time.Duration) time.Time { return now.Add(-d) }

	type notif struct {
		id          string
		recipientID string
		kind        string
		title       string
		body        string
		dlEpic      string
		dlEntityID  *string
		dlPath      string
		actorID     *string
		actorLabel  string
		isCritical  bool
		readAt      *time.Time // nil = unread
		createdAt   time.Time
	}

	hr := "SWP-EMP-1042"    // Sari Hadi (hr_admin persona's employee id)
	agent := "SWP-EMP-2891" // Budi Santoso (agent persona's employee id)
	read := ts(30 * time.Minute)
	sysActor := "SWP-USR-00002"

	rows := []notif{
		// HR Sari — a fresh leave-request-submitted (unread, critical).
		{"SWP-NTF-90001", hr, "LEAVE_REQUEST_SUBMITTED", "Pengajuan cuti baru",
			"Dewi Lestari mengajukan cuti 1 hari.", "E6", strPtr("SWP-LR-8002"), "/leave-requests/SWP-LR-8002",
			strPtr("SWP-EMP-3001"), "Dewi Lestari", true, nil, ts(2 * time.Hour)},
		// HR Sari — attendance verify needed (read).
		{"SWP-NTF-90002", hr, "ATTENDANCE_VERIFY_NEEDED", "Verifikasi kehadiran",
			"Beberapa catatan kehadiran menunggu verifikasi.", "E5", nil, "/attendance?status=PENDING",
			nil, "system", false, &read, ts(1 * 24 * time.Hour)},
		// HR Sari — a placement expiring (unread, system actor).
		{"SWP-NTF-90003", hr, "PLACEMENT_EXPIRING", "Penempatan akan berakhir",
			"1 penempatan berakhir dalam 30 hari.", "E3", strPtr("SWP-PL-5002"), "/placements/SWP-PL-5002",
			nil, "system", false, nil, ts(3 * time.Hour)},
		// Agent Budi — leave approved (unread, critical).
		{"SWP-NTF-90004", agent, "LEAVE_APPROVED", "Cuti disetujui",
			"Pengajuan cuti Anda disetujui.", "E6", strPtr("SWP-LR-8005"), "/leave-requests/SWP-LR-8005",
			&sysActor, "HR Admin", true, nil, ts(90 * time.Minute)},
		// Agent Budi — OT approved (read).
		{"SWP-NTF-90005", agent, "OT_APPROVED", "Lembur disetujui",
			"Pengajuan lembur Anda disetujui.", "E7", strPtr("SWP-OT-30005"), "/overtime/SWP-OT-30005",
			&sysActor, "HR Admin", true, &read, ts(2 * 24 * time.Hour)},
		// Agent Budi — attendance verify needed (unread).
		{"SWP-NTF-90006", agent, "ATTENDANCE_VERIFY_NEEDED", "Kehadiran diverifikasi",
			"Catatan kehadiran Anda telah diverifikasi.", "E5", nil, "/attendance",
			&sysActor, "HR Admin", false, nil, ts(45 * time.Minute)},
	}

	for _, n := range rows {
		_, err := pool.Pool.Exec(ctx, `
			INSERT INTO notifications (
				id, recipient_id, kind, title, body,
				deep_link_epic, deep_link_entity_id, deep_link_path,
				actor_id, actor_label, is_critical, read_at, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (id) DO NOTHING`,
			n.id, n.recipientID, n.kind, n.title, n.body,
			n.dlEpic, n.dlEntityID, n.dlPath,
			n.actorID, n.actorLabel, n.isCritical, n.readAt, n.createdAt,
		)
		if err != nil {
			return fmt.Errorf("insert notification %s: %w", n.id, err)
		}
	}
	slog.Info("seed: notifications", "count", len(rows))
	return nil
}

// seedHolidays inserts the E7 public-holiday calendar fixtures:
//   - SWP-HOL-9001  referenced by SWP-OT-30009 (APPROVED) → in_use, delete blocked
//   - SWP-HOL-9002  free, deletable
//
// Both NATIONAL, clearly-in-range Asia/Jakarta dates (current-year, distinct days).
func seedHolidays(ctx context.Context, pool *db.Pool) error {
	monday := mondayOfCurrentWeek(time.Now())
	// Two distinct in-range dates that do NOT collide with the OT work_dates.
	hol1 := monday.AddDate(0, 0, -14).Format("2006-01-02") // referenced by SWP-OT-30009
	hol2 := monday.AddDate(0, 0, 21).Format("2006-01-02")  // free to delete

	const hQ = `
		INSERT INTO holidays (id, name, holiday_date, category, recurring, applicable_service_lines)
		VALUES ($1, $2, $3::date, 'NATIONAL', false, '{}')
		ON CONFLICT (id) DO NOTHING`
	holidays := []struct {
		id, name, date string
	}{
		{"SWP-HOL-9001", "Hari Libur Nasional (terpakai)", hol1},
		{"SWP-HOL-9002", "Hari Libur Nasional (bebas hapus)", hol2},
	}
	for _, h := range holidays {
		if _, err := pool.Pool.Exec(ctx, hQ, h.id, h.name, h.date); err != nil {
			return fmt.Errorf("seed holiday %q: %w", h.id, err)
		}
		slog.Info("seed: upserted holiday", "id", h.id, "date", h.date)
	}
	return nil
}

// seedOvertime inserts E7 overtime fixtures so the web confirm/approval flows + the
// every-scenario E2E have real targets. All anchor on the current week (Asia/Jakarta-
// safe). Placements: Dewi=SWP-PL-5004 @ CMP-0021, Rudi=SWP-PL-5001 @ CMP-0021 (his
// OWN → SELF_APPROVAL_FORBIDDEN), Budi=SWP-PL-5002 @ CMP-0022 (OUT_OF_SCOPE for Rudi).
//
// Rows:
//   - SWP-OT-30001 PENDING_AGENT_CONFIRM AUTO_DETECTED @ CMP-0021 (confirm target)
//   - SWP-OT-30002 PENDING_L1 WORKDAY @ CMP-0021 (Rudi L1 target)
//   - SWP-OT-30003 PENDING_HR @ CMP-0021 (HR final target)
//   - SWP-OT-30004 PENDING_L1 Rudi's OWN record (SELF_APPROVAL_FORBIDDEN)
//   - SWP-OT-30005 PENDING_L1 @ CMP-0022 (OUT_OF_SCOPE for Rudi)
//   - SWP-OT-30006 PENDING_L1 counted<30 skipped_too_short (OT_BELOW_MIN target)
//   - SWP-OT-30007 APPROVED + SWP-OT-30008 REJECTED (terminal list-filter rows)
//   - SWP-OT-30009 HOLIDAY APPROVED referencing SWP-HOL-9001 (HOLIDAY_IN_USE source)
//   - SWP-OT-30010 RESTDAY PENDING_L1 @ CMP-0021
func seedOvertime(ctx context.Context, pool *db.Pool) error {
	monday := mondayOfCurrentWeek(time.Now())
	d := func(off int) string { return monday.AddDate(0, 0, off).Format("2006-01-02") }
	parking := "SWP-SVC-003"
	rule := "SWP-OTR-001"
	hol1 := "SWP-HOL-9001"

	const otQ = `
		INSERT INTO overtime (
			id, employee_id, company_id, placement_id, attendance_id, service_line_id,
			work_date, planned_start_time, planned_end_time, actual_start_time, actual_end_time,
			cross_midnight, source, status, day_type, worked_minutes, counted_minutes,
			min_minutes_threshold, skipped_too_short, reference_multiplier, overtime_rule_id,
			holiday_id, flagged_no_preapproval, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6,
			$7::date, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17,
			30, $18, $19, $20,
			$21, $22, $23, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	type ot struct {
		id, employeeID, companyID, placementID string
		attendanceID                           *string
		workDate                               string
		actualStart, actualEnd                 *string
		source, status, dayType                string
		worked, counted                        int
		skipped                                bool
		multiplier                             float64
		holidayID                              *string
		flagged                                bool
		reason                                 *string
	}
	s := func(v string) *string { return &v }
	rows := []ot{
		// Confirm target: auto-detected, awaiting agent confirm. attendance_id left
		// NULL (no seeded SWP-ATT row to satisfy the FK; the web confirm flow does
		// not depend on the linked attendance record).
		{"SWP-OT-30001", "SWP-EMP-3001", "SWP-CMP-0021", "SWP-PL-5004", nil, d(-1), s("17:00"), s("19:30"), "AUTO_DETECTED", "PENDING_AGENT_CONFIRM", "WORKDAY", 150, 150, false, 1.5, nil, false, nil},
		// L1 target (Rudi approves Dewi).
		{"SWP-OT-30002", "SWP-EMP-3001", "SWP-CMP-0021", "SWP-PL-5004", nil, d(-1), s("17:00"), s("20:32"), "REQUESTED", "PENDING_L1", "WORKDAY", 212, 210, false, 1.5, nil, false, s("Cover rekan absen.")},
		// HR final target.
		{"SWP-OT-30003", "SWP-EMP-3001", "SWP-CMP-0021", "SWP-PL-5004", nil, d(-2), s("17:00"), s("19:00"), "REQUESTED", "PENDING_HR", "WORKDAY", 120, 120, false, 1.5, nil, false, s("Lembur rutin.")},
		// SELF_APPROVAL_FORBIDDEN target: Rudi's OWN OT.
		{"SWP-OT-30004", "SWP-EMP-1108", "SWP-CMP-0021", "SWP-PL-5001", nil, d(-1), s("17:00"), s("19:00"), "REQUESTED", "PENDING_L1", "WORKDAY", 120, 120, false, 1.5, nil, false, s("Lembur leader sendiri.")},
		// OUT_OF_SCOPE target for Rudi: CMP-0022.
		{"SWP-OT-30005", "SWP-EMP-2891", "SWP-CMP-0022", "SWP-PL-5002", nil, d(-1), s("17:00"), s("19:00"), "REQUESTED", "PENDING_L1", "WORKDAY", 120, 120, false, 1.5, nil, false, s("Lembur di perusahaan lain.")},
		// OT_BELOW_MIN target: counted < 30, skipped_too_short.
		{"SWP-OT-30006", "SWP-EMP-3001", "SWP-CMP-0021", "SWP-PL-5004", nil, d(-3), s("17:00"), s("17:20"), "REQUESTED", "PENDING_L1", "WORKDAY", 20, 0, true, 1.5, nil, false, s("Lembur singkat.")},
		// Terminal rows for list filters.
		{"SWP-OT-30007", "SWP-EMP-3001", "SWP-CMP-0021", "SWP-PL-5004", nil, d(-5), s("17:00"), s("19:30"), "REQUESTED", "APPROVED", "WORKDAY", 150, 150, false, 1.5, nil, false, s("Lembur disetujui.")},
		{"SWP-OT-30008", "SWP-EMP-3001", "SWP-CMP-0021", "SWP-PL-5004", nil, d(-6), s("17:00"), s("18:00"), "REQUESTED", "REJECTED", "WORKDAY", 60, 60, false, 1.5, nil, false, s("Lembur ditolak.")},
		// HOLIDAY APPROVED referencing SWP-HOL-9001 (HOLIDAY_IN_USE source).
		{"SWP-OT-30009", "SWP-EMP-3001", "SWP-CMP-0021", "SWP-PL-5004", nil, d(-14), s("09:00"), s("12:00"), "WORKED_WITHOUT_REQUEST", "APPROVED", "HOLIDAY", 180, 180, false, 3.0, &hol1, true, s("Kerja saat hari libur.")},
		// RESTDAY pending row.
		{"SWP-OT-30010", "SWP-EMP-3001", "SWP-CMP-0021", "SWP-PL-5004", nil, d(-7), s("09:00"), s("11:00"), "REQUESTED", "PENDING_L1", "RESTDAY", 120, 120, false, 2.0, nil, false, s("Lembur hari istirahat.")},
	}
	for _, r := range rows {
		if _, err := pool.Pool.Exec(ctx, otQ,
			r.id, r.employeeID, r.companyID, r.placementID, r.attendanceID, parking,
			r.workDate, nil, nil, r.actualStart, r.actualEnd,
			false, r.source, r.status, r.dayType, r.worked, r.counted,
			r.skipped, r.multiplier, rule,
			r.holidayID, r.flagged, r.reason,
		); err != nil {
			return fmt.Errorf("seed overtime %q: %w", r.id, err)
		}
		slog.Info("seed: upserted overtime", "id", r.id, "status", r.status, "day_type", r.dayType)
	}

	// SWP-OT-30009 carries an L1+HR approval trail so the FE detail timeline renders
	// (it is APPROVED). Idempotent via NOT EXISTS (bigserial has no id to ON CONFLICT).
	const oaQ = `
		INSERT INTO overtime_approvals (overtime_id, level, decision, approver_id, approver_name, reason)
		SELECT $1, $2, $3, $4, $5, $6
		WHERE NOT EXISTS (
			SELECT 1 FROM overtime_approvals WHERE overtime_id = $1 AND level = $2)`
	trails := []struct {
		otID         string
		level        int
		decision     string
		approverID   string
		approverName string
	}{
		{"SWP-OT-30009", 1, "APPROVED", "SWP-EMP-1108", "Rudi Wijaya"},
		{"SWP-OT-30009", 2, "APPROVED", "SWP-USR-00002", "HR Admin"},
		{"SWP-OT-30007", 1, "APPROVED", "SWP-EMP-1108", "Rudi Wijaya"},
		{"SWP-OT-30007", 2, "APPROVED", "SWP-USR-00002", "HR Admin"},
		{"SWP-OT-30008", 1, "REJECTED", "SWP-EMP-1108", "Rudi Wijaya"},
	}
	for _, t := range trails {
		if _, err := pool.Pool.Exec(ctx, oaQ, t.otID, t.level, t.decision, t.approverID, t.approverName, nil); err != nil {
			return fmt.Errorf("seed overtime_approval for %q L%d: %w", t.otID, t.level, err)
		}
	}
	slog.Info("seed: upserted overtime approvals", "count", len(trails))
	return nil
}

// seedPayroll inserts E8 payslip fixtures (F8.1/F8.2) so the web HR/Super-Admin
// payroll archive + detail + audit-note + export E2E have real targets. Money is
// encrypted AES-256-GCM under PAYROLL_ENCRYPTION_KEY (the SAME key the API
// decrypts with) so the FINAL rows decrypt cleanly and the DECRYPT_FAIL row
// (garbage ciphertext) surfaces honestly. Periods 2025-11 / 2025-12 are clearly in
// range. Idempotent (ON CONFLICT (id) DO NOTHING; child line-items dedupe via the
// payslip ON CONFLICT — they are only inserted on a fresh payslip).
//
// Rows:
//   - SWP-PS-90121 SWP-EMP-1042 (Budi) 2025-12 FINAL + full breakdown + benefits + 2 notes? (no)
//   - SWP-PS-90122 SWP-EMP-1108 2025-12 FINAL (volume)
//   - SWP-PS-90123 SWP-EMP-2891 2025-11 FINAL
//   - SWP-PS-90124 SWP-EMP-3001 2025-11 FINAL (extra volume)
//   - SWP-PS-90119 SWP-EMP-1108 (Rudi) 2025-12 DECRYPT_FAIL (garbage ciphertext) + 2 audit notes
func seedPayroll(ctx context.Context, pool *db.Pool) error {
	keyB64 := os.Getenv("PAYROLL_ENCRYPTION_KEY")
	if keyB64 == "" {
		slog.Warn("seed: PAYROLL_ENCRYPTION_KEY unset — skipping payroll seed (E8 archive will be empty)")
		return nil
	}
	cipher, err := crypto.NewFromBase64(keyB64)
	if err != nil {
		return fmt.Errorf("seed payroll: build cipher: %w", err)
	}
	enc := func(money string) ([]byte, error) { return cipher.Encrypt(money) }

	// --- payslip headers (encrypted summary money) ---
	const psQ = `
		INSERT INTO payslips (
			id, employee_id, employee_name, placement_id, year, month, paid_on,
			working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
			status, source_system, source_id)
		VALUES ($1, $2, $3, NULL, $4, $5, $6::date,
			$7, $8, $9, $10,
			$11, 'lumen_swp', $12)
		ON CONFLICT (id) DO NOTHING
		RETURNING id`

	type money struct{ ge, gd, th string }
	type payslip struct {
		id, employeeID, name string
		year, month          int
		paidOn               string
		workingDays          *int
		m                    *money // nil → all *_enc NULL (not used here)
		sourceID             string
		decryptFail          bool
	}
	wd := func(n int) *int { return &n }
	finals := []payslip{
		{"SWP-PS-90121", "SWP-EMP-1042", "Budi Santoso", 2025, 12, "2025-12-28", wd(22), &money{"8500000.00", "1175000.00", "7325000.00"}, "44218", false},
		{"SWP-PS-90122", "SWP-EMP-1108", "Rudi Hartono", 2025, 12, "2025-12-28", wd(21), &money{"7200000.00", "980000.00", "6220000.00"}, "44219", false},
		{"SWP-PS-90123", "SWP-EMP-2891", "Andi Pratama", 2025, 11, "2025-11-28", wd(20), &money{"6800000.00", "910000.00", "5890000.00"}, "44201", false},
		{"SWP-PS-90124", "SWP-EMP-3001", "Dewi Lestari", 2025, 11, "2025-11-28", wd(22), &money{"7000000.00", "950000.00", "6050000.00"}, "44202", false},
	}

	insertedFresh := map[string]bool{}
	for _, p := range finals {
		geEnc, e1 := enc(p.m.ge)
		gdEnc, e2 := enc(p.m.gd)
		thEnc, e3 := enc(p.m.th)
		if e1 != nil || e2 != nil || e3 != nil {
			return fmt.Errorf("seed payslip %q: encrypt money: %v/%v/%v", p.id, e1, e2, e3)
		}
		var wdArg any
		if p.workingDays != nil {
			wdArg = *p.workingDays
		}
		var returnedID string
		err := pool.Pool.QueryRow(ctx, psQ,
			p.id, p.employeeID, p.name, p.year, p.month, p.paidOn,
			wdArg, geEnc, gdEnc, thEnc, "FINAL", p.sourceID,
		).Scan(&returnedID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				slog.Info("seed: payslip already present (skip lines)", "id", p.id)
				continue // ON CONFLICT DO NOTHING → no RETURNING row; lines already seeded
			}
			return fmt.Errorf("seed payslip %q: %w", p.id, err)
		}
		insertedFresh[p.id] = true
		slog.Info("seed: upserted payslip", "id", p.id, "period", fmt.Sprintf("%d-%02d", p.year, p.month), "status", "FINAL")
	}

	// --- DECRYPT_FAIL row: deliberately-corrupt ciphertext (AES-GCM Open rejects).
	// Raw garbage bytea (NOT produced by Encrypt) → the API's Decrypt returns
	// ErrDecrypt at read time, so the row surfaces decrypt_fail honestly.
	garbage := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd}
	var dfReturnedID string
	dfErr := pool.Pool.QueryRow(ctx, psQ,
		"SWP-PS-90119", "SWP-EMP-1108", "Rudi Hartono", 2025, 12, "2025-12-28",
		nil, garbage, garbage, garbage, "DECRYPT_FAIL", "44216",
	).Scan(&dfReturnedID)
	if dfErr != nil && !errors.Is(dfErr, pgx.ErrNoRows) {
		return fmt.Errorf("seed payslip SWP-PS-90119 (decrypt-fail): %w", dfErr)
	}
	slog.Info("seed: upserted DECRYPT_FAIL payslip", "id", "SWP-PS-90119")

	// --- breakdown for SWP-PS-90121 (only when freshly inserted) ---
	if insertedFresh["SWP-PS-90121"] {
		if err := seedPayslipBreakdown(ctx, pool, enc, "SWP-PS-90121"); err != nil {
			return err
		}
	}

	// --- audit notes on the DECRYPT_FAIL row (migration-review channel) ---
	const noteQ = `
		INSERT INTO payslip_audit_notes (id, payslip_id, seq, text, author_id, author_name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::timestamptz)
		ON CONFLICT (id) DO NOTHING`
	notes := []struct {
		id, payslipID, text, authorID, authorName, createdAt string
		seq                                                  int
	}{
		{"SWP-PS-90119-NOTE-1", "SWP-PS-90119", "Decrypt failed pada migrasi 2026-05-30; ditahan untuk review tim migrasi (lihat tiket MIG-318).", "SWP-EMP-9001", "Sari Hadi", "2026-05-30T08:14:22Z", 1},
		{"SWP-PS-90119-NOTE-2", "SWP-PS-90119", "Konfirmasi key payroll lama dengan finance team — kunci tidak cocok untuk record ini.", "SWP-EMP-9001", "Sari Hadi", "2026-05-31T03:42:09Z", 2},
	}
	for _, n := range notes {
		if _, err := pool.Pool.Exec(ctx, noteQ, n.id, n.payslipID, n.seq, n.text, n.authorID, n.authorName, n.createdAt); err != nil {
			return fmt.Errorf("seed audit note %q: %w", n.id, err)
		}
	}
	slog.Info("seed: upserted payslip audit notes", "count", len(notes))
	return nil
}

// seedPayslipBreakdown inserts the encrypted earnings/deductions/benefits lines for
// a freshly-seeded payslip (matches the openapi getPayslip example breakdown).
func seedPayslipBreakdown(ctx context.Context, pool *db.Pool, enc func(string) ([]byte, error), payslipID string) error {
	const compQ = `
		INSERT INTO payslip_components (payslip_id, kind, name, value_enc, for_bpjs, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)`
	comps := []struct {
		kind, name, value string
		forBPJS           bool
	}{
		{"EARNING", "Gaji Pokok", "6500000.00", true},
		{"EARNING", "Tunjangan Transport", "1200000.00", false},
		{"EARNING", "Tunjangan Makan", "800000.00", false},
		{"DEDUCTION", "BPJS Kesehatan (1%)", "65000.00", true},
		{"DEDUCTION", "BPJS Ketenagakerjaan (JHT 2%)", "130000.00", true},
		{"DEDUCTION", "PPh 21", "915000.00", false},
	}
	for i, c := range comps {
		ve, eerr := enc(c.value)
		if eerr != nil {
			return fmt.Errorf("seed component %q: encrypt: %w", c.name, eerr)
		}
		if _, err := pool.Pool.Exec(ctx, compQ, payslipID, c.kind, c.name, ve, c.forBPJS, i); err != nil {
			return fmt.Errorf("seed component %q: %w", c.name, err)
		}
	}

	const benQ = `
		INSERT INTO payslip_benefits (payslip_id, name, value_enc, sort_order)
		VALUES ($1, $2, $3, $4)`
	benefits := []struct{ name, value string }{
		{"BPJS Kesehatan (employer 4%)", "260000.00"},
		{"BPJS JKK", "16900.00"},
	}
	for i, b := range benefits {
		ve, eerr := enc(b.value)
		if eerr != nil {
			return fmt.Errorf("seed benefit %q: encrypt: %w", b.name, eerr)
		}
		if _, err := pool.Pool.Exec(ctx, benQ, payslipID, b.name, ve, i); err != nil {
			return fmt.Errorf("seed benefit %q: %w", b.name, err)
		}
	}
	slog.Info("seed: upserted payslip breakdown", "id", payslipID, "components", len(comps), "benefits", len(benefits))
	return nil
}

// seedLeave inserts E6 leave fixtures so the web approval flows + quota mgmt +
// calendar + the INV-3 loop-closer E2E have real targets. All dates anchor on the
// CURRENT week's Monday (Asia/Jakarta-safe, clearly in range), period = current year.
//
// leave_quotas (annual SWP-LT-001, calendar year):
//   - Dewi (SWP-EMP-3001): total 12, used 4 → remaining 8 (clean final-approve target)
//   - Budi (SWP-EMP-2891): total 12, used 11 → remaining 1 (near-exhausted →
//     BALANCE_RECHECK_FAILED / override target)
//
// leave_requests (all seeded Pending/terminal — web/mobile CREATE out of scope):
//   - SWP-LR-8001  Dewi @ CMP-0021, PENDING_L1, monday+4 (Fri) → Rudi L1 target
//   - SWP-LR-8002  Dewi @ CMP-0021, PENDING_HR (leader-approved; +leave_approvals
//     {L1,APPROVED}) → HR final target
//   - SWP-LR-8003  Budi @ CMP-0022, PENDING_HR, 3 days vs remaining 1 →
//     BALANCE_RECHECK/override target (no leader at CMP-0022 → no_leader=true)
//   - SWP-LR-8004  Budi @ CMP-0022, PENDING_L1 → leader OUT_OF_SCOPE target (Rudi
//     leads CMP-0021, gets 403 on :approve-l1)
//   - SWP-LR-8005  Dewi @ CMP-0021, APPROVED terminal (list filter + calendar)
//   - SWP-LR-8006  Dewi @ CMP-0021, REJECTED terminal (list filter)
//   - SWP-LR-8007  Dewi @ CMP-0021, PENDING_HR, start=end=monday+2 (Wed) OVERLAPPING
//     SWP-SCH-6002 → HR :approve-final fires INV-3 (schedule → CANCELLED_BY_LEAVE /
//     DTO LEAVE + approved_leave_days insert), the production over-leave source.
func seedLeave(ctx context.Context, pool *db.Pool) error {
	monday := mondayOfCurrentWeek(time.Now())
	wed := monday.AddDate(0, 0, 2).Format("2006-01-02") // SWP-SCH-6002 overlap (INV-3)
	thu := monday.AddDate(0, 0, 3).Format("2006-01-02")
	fri := monday.AddDate(0, 0, 4).Format("2006-01-02")
	year := monday.Year()
	periodStart := fmt.Sprintf("%d-01-01", year)
	periodEnd := fmt.Sprintf("%d-12-31", year)

	// --- leave_grants (F6.1 grant-lot ledger; explicit ids for deterministic E2E) ---
	// Each demo agent gets an ANNUAL pool lot (expires year-end). Dewi also gets an
	// earmarked MATERNITY lot HR pre-funded (only a maternity request may draw it,
	// LQ-10). consumed_days is seeded directly so balances are non-trivial; matching
	// leave_consumptions rows are written below to keep Σ consumption == lot.consumed.
	const lgQ = `
		INSERT INTO leave_grants
			(id, employee_id, amount_days, effective_from, expires_at, source, earmark, remark, consumed_days, pending_days, created_by)
		VALUES ($1, $2, $3, $4::date, $5::date, $6, $7, $8, $9, 0, 'system-seed')
		ON CONFLICT (id) DO NOTHING`
	matExpiry := fmt.Sprintf("%d-03-31", year+1)
	grants := []struct {
		id, employeeID   string
		amount, consumed int
		expires          string
		source           string
		earmark          *string
		remark           string
	}{
		{"SWP-LG-8001", "SWP-EMP-3001", 12, 4, periodEnd, "ANNUAL", nil, "Hibah kuota tahunan " + fmt.Sprint(year) + " (Dewi)."},
		{"SWP-LG-8002", "SWP-EMP-2891", 12, 11, periodEnd, "ANNUAL", nil, "Hibah kuota tahunan " + fmt.Sprint(year) + " (Budi)."},
		{"SWP-LG-8003", "SWP-EMP-3001", 90, 0, matExpiry, "MATERNITY", strPtr("MATERNITY"), "Pre-fund cuti melahirkan (LQ-11)."},
	}
	for _, g := range grants {
		if _, err := pool.Pool.Exec(ctx, lgQ, g.id, g.employeeID, g.amount, periodStart, g.expires, g.source, g.earmark, g.remark, g.consumed); err != nil {
			return fmt.Errorf("seed leave_grant %q: %w", g.id, err)
		}
		slog.Info("seed: upserted leave grant", "id", g.id, "employee_id", g.employeeID, "remaining", g.amount-g.consumed, "earmark", g.earmark)
	}

	// --- leave_requests (explicit ids; all Pending/terminal targets) ---
	const lrQ = `
		INSERT INTO leave_requests
			(id, employee_id, placement_id, company_id, service_line_id, leave_type_id,
			 start_date, end_date, duration_days, reason, status, no_leader, assigned_leader_id, created_by)
		VALUES ($1, $2, $3, $4, $5, 'SWP-LT-001',
			 $6::date, $7::date, $8, $9, $10, $11, $12, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	parking := "SWP-SVC-003"
	rudi := "SWP-EMP-1108" // shift leader of CMP-0021
	priorApproved := monday.AddDate(0, 0, -7).Format("2006-01-02")
	priorRejected := monday.AddDate(0, 0, -5).Format("2006-01-02")
	budiEnd := monday.AddDate(0, 0, 6).Format("2006-01-02")
	type lr struct {
		id, employeeID, placementID, companyID string
		serviceLine                            *string
		start, end                             string
		days                                   int
		reason, status                         string
		noLeader                               bool
		assignedLeader                         *string
	}
	requests := []lr{
		{"SWP-LR-8001", "SWP-EMP-3001", "SWP-PL-5004", "SWP-CMP-0021", &parking, fri, fri, 1, "Keperluan keluarga.", "PENDING_L1", false, &rudi},
		{"SWP-LR-8002", "SWP-EMP-3001", "SWP-PL-5004", "SWP-CMP-0021", &parking, thu, thu, 1, "Kontrol rumah sakit.", "PENDING_HR", false, &rudi},
		{"SWP-LR-8003", "SWP-EMP-2891", "SWP-PL-5002", "SWP-CMP-0022", &parking, fri, budiEnd, 3, "Acara keluarga 3 hari.", "PENDING_HR", true, nil},
		{"SWP-LR-8004", "SWP-EMP-2891", "SWP-PL-5002", "SWP-CMP-0022", &parking, thu, thu, 1, "Izin pribadi.", "PENDING_L1", false, nil},
		{"SWP-LR-8005", "SWP-EMP-3001", "SWP-PL-5004", "SWP-CMP-0021", &parking, priorApproved, priorApproved, 1, "Cuti yang sudah disetujui.", "APPROVED", false, &rudi},
		{"SWP-LR-8006", "SWP-EMP-3001", "SWP-PL-5004", "SWP-CMP-0021", &parking, priorRejected, priorRejected, 1, "Pengajuan yang ditolak.", "REJECTED", false, &rudi},
		{"SWP-LR-8007", "SWP-EMP-3001", "SWP-PL-5004", "SWP-CMP-0021", &parking, wed, wed, 1, "Cuti yang menimpa jadwal (INV-3).", "PENDING_HR", false, &rudi},
	}
	for _, r := range requests {
		if _, err := pool.Pool.Exec(ctx, lrQ,
			r.id, r.employeeID, r.placementID, r.companyID, r.serviceLine,
			r.start, r.end, r.days, r.reason, r.status, r.noLeader, r.assignedLeader,
		); err != nil {
			return fmt.Errorf("seed leave_request %q: %w", r.id, err)
		}
		slog.Info("seed: upserted leave request", "id", r.id, "employee_id", r.employeeID, "status", r.status)
	}

	// --- leave_approvals: SWP-LR-8002 carries an L1-APPROVED decision row so the
	// FE timeline renders the two-stage path + the PENDING_HR marker. Idempotent
	// via NOT EXISTS (bigserial has no deterministic id to ON CONFLICT on).
	const laQ = `
		INSERT INTO leave_approvals
			(leave_request_id, stage, decision, actor_id, actor_role, decision_note, is_override)
		SELECT 'SWP-LR-8002', 'L1', 'APPROVED', 'SWP-EMP-1108', 'shift_leader', 'Coverage aman.', false
		WHERE NOT EXISTS (
			SELECT 1 FROM leave_approvals WHERE leave_request_id = 'SWP-LR-8002' AND stage = 'L1')`
	if _, err := pool.Pool.Exec(ctx, laQ); err != nil {
		return fmt.Errorf("seed leave_approval for SWP-LR-8002: %w", err)
	}
	slog.Info("seed: upserted leave approval", "leave_request_id", "SWP-LR-8002", "stage", "L1")

	// --- leave_consumptions: tie each lot's consumed_days to APPROVED requests so
	// Σ consumption.days per lot == lot.consumed_days (the F6.1 ledger invariant).
	// Dewi's lot SWP-LG-8001 consumed 4 → attribute to the APPROVED SWP-LR-8005
	// (1 day) + the historical SWP-LR-8006 placeholder is REJECTED so we add a 3-day
	// APPROVED back-history request (SWP-LR-8008). Budi's lot SWP-LG-8002 consumed 11
	// → one APPROVED back-history request SWP-LR-8009.
	const lrHistQ = `
		INSERT INTO leave_requests
			(id, employee_id, placement_id, company_id, service_line_id, leave_type_id,
			 start_date, end_date, duration_days, reason, status, no_leader, assigned_leader_id, created_by)
		VALUES ($1, $2, $3, $4, $5, 'SWP-LT-001', $6::date, $7::date, $8, $9, 'APPROVED', false, $10, 'system-seed')
		ON CONFLICT (id) DO NOTHING`
	h1s := monday.AddDate(0, 0, -30).Format("2006-01-02")
	h1e := monday.AddDate(0, 0, -28).Format("2006-01-02")
	h2s := monday.AddDate(0, 0, -60).Format("2006-01-02")
	h2e := monday.AddDate(0, 0, -50).Format("2006-01-02")
	hist := []struct {
		id, emp, pl, cmp string
		start, end       string
		days             int
		leader           *string
	}{
		{"SWP-LR-8008", "SWP-EMP-3001", "SWP-PL-5004", "SWP-CMP-0021", h1s, h1e, 3, &rudi},
		{"SWP-LR-8009", "SWP-EMP-2891", "SWP-PL-5002", "SWP-CMP-0022", h2s, h2e, 11, nil},
	}
	for _, r := range hist {
		if _, err := pool.Pool.Exec(ctx, lrHistQ, r.id, r.emp, r.pl, r.cmp, &parking, r.start, r.end, r.days, "Cuti historis (backfill saldo).", r.leader); err != nil {
			return fmt.Errorf("seed historical leave_request %q: %w", r.id, err)
		}
	}
	const lcQ = `
		INSERT INTO leave_consumptions (id, leave_request_id, grant_id, days)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`
	cons := []struct {
		id, req, grant string
		days           int
	}{
		{"SWP-LC-8001", "SWP-LR-8005", "SWP-LG-8001", 1},  // Dewi approved (current week)
		{"SWP-LC-8002", "SWP-LR-8008", "SWP-LG-8001", 3},  // Dewi historical (1+3 = 4 == consumed)
		{"SWP-LC-8003", "SWP-LR-8009", "SWP-LG-8002", 11}, // Budi historical (== consumed)
	}
	for _, c := range cons {
		if _, err := pool.Pool.Exec(ctx, lcQ, c.id, c.req, c.grant, c.days); err != nil {
			return fmt.Errorf("seed leave_consumption %q: %w", c.id, err)
		}
		slog.Info("seed: upserted leave consumption", "id", c.id, "grant_id", c.grant, "days", c.days)
	}

	return nil
}

// seedAuditLog inserts ~5 audit_log rows covering entity_types user and placement,
// including one system row (actor_user_id NULL). Idempotent: skips if any rows exist.
func seedAuditLog(ctx context.Context, pool *db.Pool) error {
	var count int
	if err := pool.Pool.QueryRow(ctx, "SELECT count(*) FROM audit_log").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		slog.Info("seed: audit_log already has rows, skipping", "count", count)
		return nil
	}

	type auditRow struct {
		actorUserID *string
		actorRole   *string
		action      string
		entityType  string
		entityID    string
		before      *string // JSON or nil
		after       *string // JSON or nil
	}

	rows := []auditRow{
		{
			actorUserID: nil,
			actorRole:   nil,
			action:      "CREATE",
			entityType:  "user",
			entityID:    "SWP-USR-system-init",
			before:      nil,
			after:       strPtr(`{"note":"system initialised"}`),
		},
		{
			actorUserID: strPtr("SWP-USR-00001"),
			actorRole:   strPtr("super_admin"),
			action:      "CREATE",
			entityType:  "user",
			entityID:    "SWP-USR-00002",
			before:      nil,
			after:       strPtr(`{"email":"sari.hadi@swp.test","role":"hr_admin"}`),
		},
		{
			actorUserID: strPtr("SWP-USR-00001"),
			actorRole:   strPtr("hr_admin"),
			action:      "user.change_role",
			entityType:  "user",
			entityID:    "SWP-USR-00003",
			before:      strPtr(`{"role":"agent"}`),
			after:       strPtr(`{"role":"shift_leader","reason":"promoted on site"}`),
		},
		{
			actorUserID: strPtr("SWP-USR-00001"),
			actorRole:   strPtr("hr_admin"),
			action:      "CREATE",
			entityType:  "placement",
			entityID:    "SWP-PL-00001",
			before:      nil,
			after:       strPtr(`{"employee_id":"SWP-EMP-1042","company_id":"SWP-CMP-0021"}`),
		},
		{
			actorUserID: strPtr("SWP-USR-00001"),
			actorRole:   strPtr("hr_admin"),
			action:      "user.deactivate",
			entityType:  "user",
			entityID:    "SWP-USR-00004",
			before:      strPtr(`{"status":"active"}`),
			after:       strPtr(`{"status":"disabled","reason":"contract ended"}`),
		},
	}

	const insertQ = `
		INSERT INTO audit_log
			(id, actor_user_id, actor_role, action, entity_type, entity_id,
			 before_state, after_state, request_id, created_at)
		VALUES
			('SWP-AL-' || swp_next_id('AL'), $1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, NULL, now())`

	for _, row := range rows {
		if _, err := pool.Pool.Exec(ctx, insertQ,
			row.actorUserID, row.actorRole,
			row.action, row.entityType, row.entityID,
			row.before, row.after,
		); err != nil {
			return err
		}
	}
	slog.Info("seed: inserted audit_log rows", "count", len(rows))
	return nil
}

// seedEmployees inserts Phase-4 employee fixtures for all persona IDs.
// All inserts use ON CONFLICT (id) DO NOTHING so re-runs are idempotent.
//
// ORDERING CONTRACT: This function MUST be called before the persona user-seed
// loop (which sets employee_id on each user row). See Seed() for details.
//
// Personas:
//   - SWP-EMP-1042  Sari Hadi       (hr_admin persona)
//   - SWP-EMP-1108  Rudi Wijaya     (shift_leader persona)
//   - SWP-EMP-2891  Budi Santoso    (agent persona — has phone + BCA bank for change-request E2E)
//   - SWP-EMP-3001  Dewi Lestari    (extra agent persona)
//   - SWP-EMP-3002  Agus Pratama    (extra shift_leader persona)
//   - SWP-EMP-3003  Bambang Sutrisno (extra hr_admin persona)
func seedEmployees(ctx context.Context, pool *db.Pool) error {
	type employee struct {
		id                    string
		fullName              string
		nik                   string
		nip                   string
		joinAt                string // YYYY-MM-DD
		gender                string
		phone                 *string
		emailPersonal         *string
		address               *string
		birthDate             *string // YYYY-MM-DD
		birthPlace            *string
		npwp                  *string
		bpjsKesehatan         *string
		bpjsKetenagakerjaan   *string
		bankName              *string
		bankAccountNumber     *string
		bankAccountHolderName *string
		emergencyContactName  *string
		emergencyContactPhone *string
	}

	bca := "BCA"
	bcaAccount := "1234567890"
	budiName := "Budi Santoso"
	budiPhone := "+62-812-3344-5566"

	employees := []employee{
		{
			id:       "SWP-EMP-1042",
			fullName: "Sari Hadi",
			nik:      "3175001505900042",
			nip:      "1042",
			joinAt:   "2020-03-01",
			gender:   "FEMALE",
		},
		{
			id:       "SWP-EMP-1108",
			fullName: "Rudi Wijaya",
			nik:      "3175001505900108",
			nip:      "1108",
			joinAt:   "2019-07-15",
			gender:   "MALE",
		},
		{
			id:                    "SWP-EMP-2891",
			fullName:              "Budi Santoso",
			nik:                   "3175001505902891",
			nip:                   "2891",
			joinAt:                "2021-01-10",
			gender:                "MALE",
			phone:                 &budiPhone,
			bankName:              &bca,
			bankAccountNumber:     &bcaAccount,
			bankAccountHolderName: &budiName,
		},
		{
			id:                    "SWP-EMP-3001",
			fullName:              "Dewi Lestari",
			nik:                   "3175001505903001",
			nip:                   "3001",
			joinAt:                "2022-04-01",
			gender:                "FEMALE",
			phone:                 strPtr("+62-812-3001-0001"),
			emailPersonal:         strPtr("dewi.lestari.personal@gmail.com"),
			address:               strPtr("Jl. Melawai Raya No. 12, Kebayoran Baru, Jakarta Selatan 12160"),
			birthDate:             strPtr("1995-08-17"),
			birthPlace:            strPtr("Bandung"),
			npwp:                  strPtr("09.123.456.7-011.000"),
			bpjsKesehatan:         strPtr("0001234567890"),
			bpjsKetenagakerjaan:   strPtr("23B1234567"),
			bankName:              strPtr("BCA"),
			bankAccountNumber:     strPtr("2891037001"),
			bankAccountHolderName: strPtr("Dewi Lestari"),
			emergencyContactName:  strPtr("Sapto Lestari"),
			emergencyContactPhone: strPtr("+62-812-9000-1234"),
		},
		{
			id:       "SWP-EMP-3002",
			fullName: "Agus Pratama",
			nik:      "3175001505903002",
			nip:      "3002",
			joinAt:   "2022-04-01",
			gender:   "MALE",
		},
		{
			id:       "SWP-EMP-3003",
			fullName: "Bambang Sutrisno",
			nik:      "3175001505903003",
			nip:      "3003",
			joinAt:   "2022-04-01",
			gender:   "MALE",
		},
		{
			// Employee row backing the `lead` persona (Joko Pratama). Required so
			// users.employee_id resolves and the lead's lead_assignments FK holds.
			id:       "SWP-EMP-3004",
			fullName: "Joko Pratama",
			nik:      "3175001505903004",
			nip:      "3004",
			joinAt:   "2021-09-01",
			gender:   "MALE",
		},
	}

	// ON CONFLICT DO UPDATE so a re-seed enriches an existing row (e.g. Dewi's full
	// profile) without clobbering intentionally-omitted values: every nullable column
	// is COALESCE(EXCLUDED.col, employees.col), so a NULL in the seed keeps the stored
	// value. status is left untouched (not reset on re-seed).
	const empQ = `
		INSERT INTO employees
			(id, full_name, nik, nip, join_at, gender,
			 phone, email_personal, address, birth_date, birth_place,
			 npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
			 bank_name, bank_account_number, bank_account_holder_name,
			 emergency_contact_name, emergency_contact_phone,
			 status)
		VALUES ($1, $2, $3, $4, $5::date, $6,
		        $7, $8, $9, $10::date, $11,
		        $12, $13, $14,
		        $15, $16, $17,
		        $18, $19,
		        'active')
		ON CONFLICT (id) DO UPDATE SET
			full_name                = COALESCE(EXCLUDED.full_name, employees.full_name),
			nik                      = COALESCE(EXCLUDED.nik, employees.nik),
			nip                      = COALESCE(EXCLUDED.nip, employees.nip),
			join_at                  = COALESCE(EXCLUDED.join_at, employees.join_at),
			gender                   = COALESCE(EXCLUDED.gender, employees.gender),
			phone                    = COALESCE(EXCLUDED.phone, employees.phone),
			email_personal           = COALESCE(EXCLUDED.email_personal, employees.email_personal),
			address                  = COALESCE(EXCLUDED.address, employees.address),
			birth_date               = COALESCE(EXCLUDED.birth_date, employees.birth_date),
			birth_place              = COALESCE(EXCLUDED.birth_place, employees.birth_place),
			npwp                     = COALESCE(EXCLUDED.npwp, employees.npwp),
			bpjs_kesehatan           = COALESCE(EXCLUDED.bpjs_kesehatan, employees.bpjs_kesehatan),
			bpjs_ketenagakerjaan     = COALESCE(EXCLUDED.bpjs_ketenagakerjaan, employees.bpjs_ketenagakerjaan),
			bank_name                = COALESCE(EXCLUDED.bank_name, employees.bank_name),
			bank_account_number      = COALESCE(EXCLUDED.bank_account_number, employees.bank_account_number),
			bank_account_holder_name = COALESCE(EXCLUDED.bank_account_holder_name, employees.bank_account_holder_name),
			emergency_contact_name   = COALESCE(EXCLUDED.emergency_contact_name, employees.emergency_contact_name),
			emergency_contact_phone  = COALESCE(EXCLUDED.emergency_contact_phone, employees.emergency_contact_phone)`

	for _, e := range employees {
		if _, err := pool.Pool.Exec(ctx, empQ,
			e.id, e.fullName, e.nik, e.nip, e.joinAt, e.gender,
			e.phone, e.emailPersonal, e.address, e.birthDate, e.birthPlace,
			e.npwp, e.bpjsKesehatan, e.bpjsKetenagakerjaan,
			e.bankName, e.bankAccountNumber, e.bankAccountHolderName,
			e.emergencyContactName, e.emergencyContactPhone,
		); err != nil {
			return fmt.Errorf("seed employee %q: %w", e.id, err)
		}
		slog.Info("seed: upserted employee", "id", e.id, "name", e.fullName)
	}

	return nil
}

// seedClientCompanies inserts the Phase-3 client company + site fixtures.
// All inserts use ON CONFLICT (id) DO NOTHING so re-runs are idempotent.
//
// Companies:
//   - SWP-CMP-0021  "Plaza Senayan"     — shift_leader persona's company scope target
//   - SWP-CMP-0022  "Mall Kelapa Gading" — extra company for list/pagination E2E
//
// Sites:
//   - SWP-SITE-0001  Plaza Senayan Main (primary, geo set → geofence_active=true)
//   - SWP-SITE-0002  Mall Kelapa Gading Main (primary, no geo)
func seedClientCompanies(ctx context.Context, pool *db.Pool) error {
	type company struct {
		id          string
		name        string
		address     string
		leaderScope string
	}
	companies := []company{
		{
			id:          "SWP-CMP-0021",
			name:        "Plaza Senayan",
			address:     "Jl. Asia Afrika No. 8, Jakarta Pusat 10270",
			leaderScope: "company",
		},
		{
			id:          "SWP-CMP-0022",
			name:        "Mall Kelapa Gading",
			address:     "Jl. Boulevard Raya, Jakarta Utara 14240",
			leaderScope: "company",
		},
	}

	const companyQ = `
		INSERT INTO client_companies (id, name, address, leader_scope, status)
		VALUES ($1, $2, $3, $4, 'active')
		ON CONFLICT (id) DO NOTHING`

	for _, c := range companies {
		if _, err := pool.Pool.Exec(ctx, companyQ, c.id, c.name, c.address, c.leaderScope); err != nil {
			return fmt.Errorf("seed company %q: %w", c.id, err)
		}
		slog.Info("seed: upserted client company", "id", c.id, "name", c.name)
	}

	// Sites — use explicit IDs so E2E tests can reference them deterministically.
	// SWP-SITE-0001: Plaza Senayan Main — geo set so geofence_active=true.
	// SWP-SITE-0002: Mall Kelapa Gading Main — no geo (geofence_active=false).
	type site struct {
		id              string
		companyID       string
		name            string
		address         string
		geoLat          *float64
		geoLng          *float64
		geofenceRadiusM int
		isPrimary       bool
	}
	lat := -6.2253
	lng := 106.7995
	sites := []site{
		{
			id:              "SWP-SITE-0001",
			companyID:       "SWP-CMP-0021",
			name:            "Plaza Senayan Main",
			address:         "Jl. Asia Afrika No. 8, Jakarta Pusat 10270",
			geoLat:          &lat,
			geoLng:          &lng,
			geofenceRadiusM: 100,
			isPrimary:       true,
		},
		{
			id:              "SWP-SITE-0002",
			companyID:       "SWP-CMP-0022",
			name:            "Mall Kelapa Gading Main",
			address:         "Jl. Boulevard Raya, Jakarta Utara 14240",
			geoLat:          nil,
			geoLng:          nil,
			geofenceRadiusM: 100,
			isPrimary:       true,
		},
	}

	const siteQ = `
		INSERT INTO client_sites
			(id, client_company_id, name, address, geo_lat, geo_lng, geofence_radius_m, is_primary, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active')
		ON CONFLICT (id) DO NOTHING`

	for _, s := range sites {
		if _, err := pool.Pool.Exec(ctx, siteQ,
			s.id, s.companyID, s.name, s.address,
			s.geoLat, s.geoLng, s.geofenceRadiusM, s.isPrimary,
		); err != nil {
			return fmt.Errorf("seed site %q: %w", s.id, err)
		}
		slog.Info("seed: upserted client site", "id", s.id, "name", s.name)
	}

	return nil
}

// seedServiceLines inserts the Phase-3 service line + position fixtures.
// All inserts use ON CONFLICT (id) DO NOTHING so re-runs are idempotent.
//
// Service lines (explicit IDs — deterministic for E2E):
//   - SWP-SVC-001  "Facility Services"
//   - SWP-SVC-002  "Building Management"
//   - SWP-SVC-003  "Parking"
//
// Positions under Parking (per OpenAPI spec examples):
//   - SWP-POS-014  "Petugas Parkir" alias "Parking Attendant"
//   - SWP-POS-015  "Koordinator Lokasi" alias "Parking Supervisor"
func seedServiceLines(ctx context.Context, pool *db.Pool) error {
	type serviceLine struct {
		id   string
		name string
	}
	lines := []serviceLine{
		{id: "SWP-SVC-001", name: "Facility Services"},
		{id: "SWP-SVC-002", name: "Building Management"},
		{id: "SWP-SVC-003", name: "Parking"},
	}

	const lineQ = `
		INSERT INTO service_lines (id, name, status)
		VALUES ($1, $2, 'active')
		ON CONFLICT (id) DO NOTHING`

	for _, l := range lines {
		if _, err := pool.Pool.Exec(ctx, lineQ, l.id, l.name); err != nil {
			return fmt.Errorf("seed service_line %q: %w", l.id, err)
		}
		slog.Info("seed: upserted service line", "id", l.id, "name", l.name)
	}

	type position struct {
		id            string
		serviceLineID string
		name          string
		alias         string
	}
	positions := []position{
		{
			id:            "SWP-POS-014",
			serviceLineID: "SWP-SVC-003",
			name:          "Petugas Parkir",
			alias:         "Parking Attendant",
		},
		{
			id:            "SWP-POS-015",
			serviceLineID: "SWP-SVC-003",
			name:          "Koordinator Lokasi",
			alias:         "Parking Supervisor",
		},
	}

	const posQ = `
		INSERT INTO positions (id, service_line_id, name, alias, status)
		VALUES ($1, $2, $3, $4, 'active')
		ON CONFLICT (id) DO NOTHING`

	for _, p := range positions {
		if _, err := pool.Pool.Exec(ctx, posQ, p.id, p.serviceLineID, p.name, p.alias); err != nil {
			return fmt.Errorf("seed position %q: %w", p.id, err)
		}
		slog.Info("seed: upserted position", "id", p.id, "name", p.name)
	}

	return nil
}

// seedAgreements inserts Phase-4 employment-agreement + attachment fixtures.
// All inserts use ON CONFLICT (id) DO NOTHING so re-runs are idempotent.
//
// Agreements:
//   - SWP-AG-7001  ACTIVE PKWT for Budi Santoso (SWP-EMP-2891)
//     contract "PKWT/SWP/2026/0142", 2026-06-01 → 2027-05-31
//
// Attachments:
//   - SWP-FILE-9001  signed_agreement for SWP-AG-7001 — minimal valid PDF bytes
func seedAgreements(ctx context.Context, pool *db.Pool) error {
	// Insert the PKWT agreement.
	const agQ = `
		INSERT INTO employment_agreements
			(id, employee_id, type, agreement_no, start_date, end_date, status,
			 base_salary_idr, bpjs_terms, tax_profile, comp_effective_date, created_by)
		VALUES
			($1, $2, $3, $4, $5::date, $6::date, 'active',
			 5200000,
			 '{"kesehatan_employer_pct":4.0,"kesehatan_employee_pct":1.0,"ketenagakerjaan_employer_pct":6.24,"ketenagakerjaan_employee_pct":3.0}'::jsonb,
			 'PTKP_K0', '2026-06-01'::date, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	if _, err := pool.Pool.Exec(ctx, agQ,
		"SWP-AG-7001",
		"SWP-EMP-2891",
		"PKWT",
		"PKWT/SWP/2026/0142",
		"2026-06-01",
		"2027-05-31",
	); err != nil {
		return fmt.Errorf("seed agreement SWP-AG-7001: %w", err)
	}
	slog.Info("seed: upserted agreement", "id", "SWP-AG-7001", "employee_id", "SWP-EMP-2891")

	// Insert the signed-agreement attachment.
	// Blob is a minimal valid 1.4 PDF (enough for the download handler to serve bytes).
	minimalPDF := []byte("%PDF-1.4\n1 0 obj<</Type /Catalog /Pages 2 0 R>>endobj\n" +
		"2 0 obj<</Type /Pages /Kids[3 0 R]/Count 1>>endobj\n" +
		"3 0 obj<</Type /Page /Parent 2 0 R /MediaBox[0 0 3 3]>>endobj\n" +
		"xref\n0 4\n0000000000 65535 f \n" +
		"trailer<</Size 4 /Root 1 0 R>>\nstartxref\n%%EOF\n")

	const attQ = `
		INSERT INTO agreement_attachments
			(id, agreement_id, category, caption, file_name, mime, size_bytes, blob, uploaded_by)
		VALUES
			($1, $2, 'signed_agreement', 'PKWT Budi Santoso 2026', $3, 'application/pdf', $4, $5, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	if _, err := pool.Pool.Exec(ctx, attQ,
		"SWP-FILE-9001",
		"SWP-AG-7001",
		"pkwt-budi.pdf",
		int64(len(minimalPDF)),
		minimalPDF,
	); err != nil {
		return fmt.Errorf("seed attachment SWP-FILE-9001: %w", err)
	}
	slog.Info("seed: upserted attachment", "id", "SWP-FILE-9001", "agreement_id", "SWP-AG-7001")

	return nil
}

// seedMasterData inserts Phase-3 operational master-data fixtures.
// All inserts use ON CONFLICT (id) DO NOTHING so re-runs are idempotent.
//
// Leave types: the SWP "Fitur Ijin" 18-code catalog (per-type ledger, EPICS §8
// 2026-06-12). Each carries its own cap_basis (ANNUAL_POOL | PER_EVENT |
// PER_MONTH | PER_YEAR_COUNT | UNCAPPED | LIFETIME_ONCE | SERVICE_UNPAID).
//   - SWP-LT-001  CT     ANNUAL_POOL (12d)   — the annual type leave fixtures key off
//   - SWP-LT-002  SDSKD  UNCAPPED, doc       — sick with doctor's letter
//   - SWP-LT-003..018    CTHO/STSD/CH/CIM/CM/CKA/CMA/KGD/CKM/CRM/CTN/CAP/CIH/CIU/CPR/CLTP
//
// Attendance codes:
//   - SWP-AC-001  code PRESENT  label "Hadir"     color #0F8B8D  is_workday=true  is_paid=true  is_billable=true  needs_verification=true
//   - SWP-AC-002  code LATE     label "Terlambat"  color #E07A2A  same flags as PRESENT
//
// Overtime rules:
//   - SWP-OTR-001  "Default OT"  service_line_id=NULL  weekday_rate=1.5 restday_rate=2.0 holiday_rate=3.0
//     min_minutes=30 max_minutes_per_day=240 pre_approval_required=true
func seedMasterData(ctx context.Context, pool *db.Pool) error {
	// --- Leave types (SWP "Fitur Ijin" 18-code catalog; per-type ledger,
	// EPICS §8 "E6 — Leave" 2026-06-12). Each type carries its own cap_basis so
	// statutory/sick/religious leave meters in its own window and never depletes
	// the annual pool. SWP-LT-001 stays the annual type (existing leave fixtures
	// key off it by id). capValue nil = uncapped/variable (validated by document).
	type leaveType struct {
		id               string
		code             string
		name             string
		description      string
		category         string
		capBasis         string
		capValue         *int
		capUnit          string
		paid             bool
		gender           string
		requiresDocument bool
		noticeDays       int
		minServiceYears  int
		leadDays         int
		trailDays        int
		color            string
	}
	ci := func(n int) *int { return &n }

	leaveTypes := []leaveType{
		{"SWP-LT-001", "CT", "Cuti Tahunan Pegawai PKWT", "Cuti tahunan 12 hari untuk pegawai PKWT.", "ANNUAL", "ANNUAL_POOL", ci(12), "DAYS", true, "ANY", false, 0, 0, 0, 0, "#188E4D"},
		{"SWP-LT-002", "SDSKD", "Sakit dengan surat keterangan dokter", "Cuti sakit dengan surat dokter; durasi sesuai ketentuan.", "SICK", "UNCAPPED", nil, "DAYS", true, "ANY", true, 0, 0, 0, 0, "#E07A2A"},
		{"SWP-LT-003", "CTHO", "Cuti Tahunan Head Office", "Cuti tahunan 12 hari untuk pegawai Head Office.", "ANNUAL", "ANNUAL_POOL", ci(12), "DAYS", true, "ANY", false, 0, 0, 0, 0, "#188E4D"},
		{"SWP-LT-004", "STSD", "Sakit tanpa surat dokter", "Sakit tanpa surat dokter, maksimal 5 kali setahun.", "SICK", "PER_YEAR_COUNT", ci(5), "COUNT", true, "ANY", false, 0, 0, 0, 0, "#E07A2A"},
		{"SWP-LT-005", "CH", "Cuti Haid", "Cuti haid hari ke-1 dan ke-2; per bulan.", "MENSTRUAL", "PER_MONTH", ci(2), "DAYS", true, "FEMALE", false, 0, 0, 0, 0, "#C0497B"},
		{"SWP-LT-006", "CIM", "Istri melahirkan atau keguguran", "Istri pegawai melahirkan atau keguguran.", "LIFE_EVENT", "PER_EVENT", ci(2), "DAYS", true, "MALE", true, 0, 0, 0, 0, "#3D6FB4"},
		{"SWP-LT-007", "CM", "Pernikahan sendiri (pertama)", "Pernikahan pertama pegawai sendiri.", "LIFE_EVENT", "LIFETIME_ONCE", ci(3), "DAYS", true, "ANY", true, 0, 0, 0, 0, "#3D6FB4"},
		{"SWP-LT-008", "CKA", "Khitanan / Baptisan anak", "Khitanan atau baptisan anak pegawai.", "LIFE_EVENT", "PER_EVENT", ci(2), "DAYS", true, "ANY", true, 0, 0, 0, 0, "#3D6FB4"},
		{"SWP-LT-009", "CMA", "Menikahkan anak", "Pegawai menikahkan anak.", "LIFE_EVENT", "PER_EVENT", ci(2), "DAYS", true, "ANY", true, 0, 0, 0, 0, "#3D6FB4"},
		{"SWP-LT-010", "KGD", "Gawat darurat (antar keluarga ke RS)", "Mengantar orang tua/mertua/suami/istri/anak dalam keadaan gawat darurat; 2 hari dalam 1 bulan.", "IMPORTANT", "PER_MONTH", ci(2), "DAYS", true, "ANY", true, 0, 0, 0, 0, "#8A6D3B"},
		{"SWP-LT-011", "CKM", "Kematian keluarga inti", "Suami/istri/orang tua/mertua/anak/menantu meninggal dunia.", "BEREAVEMENT", "PER_EVENT", ci(2), "DAYS", true, "ANY", false, 0, 0, 0, 0, "#5A5A5A"},
		{"SWP-LT-012", "CRM", "Kematian anggota serumah lain", "Anggota keluarga lain yang tinggal serumah meninggal dunia.", "BEREAVEMENT", "PER_EVENT", ci(1), "DAYS", true, "ANY", false, 0, 0, 0, 0, "#5A5A5A"},
		{"SWP-LT-013", "CTN", "Tugas negara / pengadilan / kewajiban UU", "Mengemban tugas negara, panggilan pengadilan, atau kewajiban berdasarkan UU; sesuai ketentuan.", "CIVIC", "UNCAPPED", nil, "DAYS", true, "ANY", true, 0, 0, 0, 0, "#8A6D3B"},
		{"SWP-LT-014", "CAP", "Cuti Alasan Penting", "Cuti karena alasan penting; sesuai ketentuan dan persetujuan.", "IMPORTANT", "UNCAPPED", nil, "DAYS", true, "ANY", true, 0, 0, 0, 0, "#8A6D3B"},
		{"SWP-LT-015", "CIH", "Cuti Ibadah Haji (pertama)", "Ibadah haji pertama; sesuai program haji + 5 hari sebelum berangkat + 5 hari sesudah tiba.", "RELIGIOUS", "LIFETIME_ONCE", nil, "DAYS", true, "ANY", true, 30, 0, 5, 5, "#188E4D"},
		{"SWP-LT-016", "CIU", "Cuti Ibadah Umroh (pertama)", "Ibadah umroh pertama; maksimal 12 hari kerja.", "RELIGIOUS", "LIFETIME_ONCE", ci(12), "DAYS", true, "ANY", true, 30, 0, 0, 0, "#188E4D"},
		{"SWP-LT-017", "CPR", "Cuti Perjalanan Rohani (pertama)", "Perjalanan rohani pertama; sesuai ketentuan.", "RELIGIOUS", "LIFETIME_ONCE", nil, "DAYS", true, "ANY", true, 30, 0, 0, 0, "#188E4D"},
		{"SWP-LT-018", "CLTP", "Cuti di luar tanggungan Perusahaan", "Cuti di luar tanggungan; maks 12 bulan, sekali, min 5 tahun masa kerja, tidak dibayar.", "UNPAID", "SERVICE_UNPAID", ci(365), "DAYS", false, "ANY", true, 30, 5, 0, 0, "#5A5A5A"},
	}

	const ltQ = `
		INSERT INTO leave_types
			(id, code, name, description, category, cap_basis, cap_value, cap_unit,
			 paid, gender, requires_document, notice_days, min_service_years,
			 lead_days, trail_days, default_annual_quota, is_annual, color, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, 'active')
		ON CONFLICT (id) DO NOTHING`

	for _, lt := range leaveTypes {
		// Back-compat: keep default_annual_quota / is_annual coherent with cap_basis.
		isAnnual := lt.capBasis == "ANNUAL_POOL"
		defaultAnnualQuota := 0
		if isAnnual && lt.capValue != nil {
			defaultAnnualQuota = *lt.capValue
		}
		if _, err := pool.Pool.Exec(ctx, ltQ,
			lt.id, lt.code, lt.name, lt.description, lt.category, lt.capBasis,
			lt.capValue, lt.capUnit, lt.paid, lt.gender, lt.requiresDocument,
			lt.noticeDays, lt.minServiceYears, lt.leadDays, lt.trailDays,
			defaultAnnualQuota, isAnnual, lt.color,
		); err != nil {
			return fmt.Errorf("seed leave_type %q: %w", lt.id, err)
		}
		slog.Info("seed: upserted leave type", "id", lt.id, "code", lt.code)
	}

	// --- Attendance codes ---
	type attendanceCode struct {
		id                string
		code              string
		label             string
		description       string
		color             string
		isWorkday         bool
		isPaid            bool
		isBillable        bool
		needsVerification bool
	}

	attendanceCodes := []attendanceCode{
		{
			id:                "SWP-AC-001",
			code:              "PRESENT",
			label:             "Hadir",
			description:       "Agen hadir dan bekerja pada hari yang bersangkutan.",
			color:             "#0F8B8D",
			isWorkday:         true,
			isPaid:            true,
			isBillable:        true,
			needsVerification: true,
		},
		{
			id:                "SWP-AC-002",
			code:              "LATE",
			label:             "Terlambat",
			description:       "Agen hadir namun melewati jam masuk yang ditetapkan.",
			color:             "#E07A2A",
			isWorkday:         true,
			isPaid:            true,
			isBillable:        true,
			needsVerification: true,
		},
	}

	const acQ = `
		INSERT INTO attendance_codes
			(id, code, label, description, color, is_workday, is_paid, is_billable, needs_verification, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'active')
		ON CONFLICT (id) DO NOTHING`

	for _, ac := range attendanceCodes {
		if _, err := pool.Pool.Exec(ctx, acQ,
			ac.id, ac.code, ac.label, ac.description, ac.color,
			ac.isWorkday, ac.isPaid, ac.isBillable, ac.needsVerification,
		); err != nil {
			return fmt.Errorf("seed attendance_code %q: %w", ac.id, err)
		}
		slog.Info("seed: upserted attendance code", "id", ac.id, "code", ac.code)
	}

	// --- Overtime rules ---
	// SWP-OTR-001: global default overtime rule (service_line_id = NULL).
	const otrQ = `
		INSERT INTO overtime_rules
			(id, name, service_line_id, weekday_rate, restday_rate, holiday_rate,
			 min_minutes, max_minutes_per_day, pre_approval_required, status)
		VALUES ($1, $2, NULL, $3, $4, $5, $6, $7, $8, 'active')
		ON CONFLICT (id) DO NOTHING`

	if _, err := pool.Pool.Exec(ctx, otrQ,
		"SWP-OTR-001", "Default OT",
		1.5, 2.0, 3.0,
		30, 240, true,
	); err != nil {
		return fmt.Errorf("seed overtime_rule SWP-OTR-001: %w", err)
	}
	slog.Info("seed: upserted overtime rule", "id", "SWP-OTR-001", "name", "Default OT")

	return nil
}

// seedChangeRequests inserts the EP-5 pending change-request fixtures (redesigned
// 2026-06-11). All inserts use ON CONFLICT (id) DO NOTHING so re-runs are idempotent.
//
// Budi Santoso (SWP-EMP-2891) has phone "+62-812-3344-5566" and BCA bank account
// "1234567890"; Dewi Lestari (SWP-EMP-3001) has empty profile fields — both seeded
// in seedEmployees, so the approval detail renders a meaningful old→new diff.
//
// Change requests (status: pending):
//
//	SWP-CHG-2117  Budi @ CMP-0022  MULTIPLE (phone+bank)  — SL out-of-company 403 target
//	SWP-CHG-2119  Dewi @ CMP-0021  MULTIPLE (phone+bank)  — SL bank-split → HR finalize
//	SWP-CHG-2120  Dewi @ CMP-0021  EMERGENCY_CONTACT only — SL/HR full approve / reject
func seedChangeRequests(ctx context.Context, pool *db.Pool) error {
	// Change-request fixtures aligned with the 2026-06-11 redesign (E2 EP-5):
	// tiers shifted (address → instant, emergency-contact → approval) and approval
	// now routes to the on-site shift leader (company scope) with HR bank-split.
	//
	// The shift leader Rudi (SWP-EMP-1108) leads SWP-CMP-0021 (Plaza Senayan), where
	// Dewi (SWP-EMP-3001) is placed. Budi (SWP-EMP-2891) is placed at SWP-CMP-0022
	// (Mall Kelapa Gading) — OUT of Rudi's company scope. These placements are seeded
	// later in seedPlacements; both demo agents carry an active placement company so
	// the SL company-scope resolution (GetEmployeeCompanyID → GuardCompany) works.
	//
	//   SWP-CHG-2117  Budi @ CMP-0022  MULTIPLE (phone + bank)  → SL Rudi OUT_OF_SCOPE (403)
	//   SWP-CHG-2119  Dewi @ CMP-0021  MULTIPLE (phone + bank)  → SL applies phone, bank
	//                                                              escalates to HR (partial)
	//   SWP-CHG-2120  Dewi @ CMP-0021  EMERGENCY_CONTACT only   → SL fully approves
	const crQ = `
		INSERT INTO change_requests
			(id, employee_id, changes, request_type, note, submitted_at)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6::timestamptz)
		ON CONFLICT (id) DO NOTHING`

	// SWP-CHG-2117: MULTIPLE (phone + bank) for Budi @ CMP-0022 — the cross-company
	// 403 target for shift leader Rudi (who leads CMP-0021).
	if _, err := pool.Pool.Exec(ctx, crQ,
		"SWP-CHG-2117",
		"SWP-EMP-2891",
		`{"phone":"+62-812-9988-7766","bank_account":{"bank_name":"BCA","account_number":"9999000011","account_holder_name":"Budi Santoso"}}`,
		"MULTIPLE",
		"Ganti nomor & rekening baru",
		"2026-06-03T08:00:00Z",
	); err != nil {
		return fmt.Errorf("seed change_request SWP-CHG-2117: %w", err)
	}
	slog.Info("seed: upserted change request", "id", "SWP-CHG-2117", "type", "MULTIPLE")

	// SWP-CHG-2119: MULTIPLE (phone + bank) for Dewi @ CMP-0021 — the in-company
	// bank-split target. SL Rudi approves → phone applied + bank escalated to HR
	// (status=partially_approved, bank_pending=true); HR finalizes the bank field.
	if _, err := pool.Pool.Exec(ctx, crQ,
		"SWP-CHG-2119",
		"SWP-EMP-3001",
		`{"phone":"+62-813-5566-7788","bank_account":{"bank_name":"Mandiri","account_number":"1440011223344","account_holder_name":"Dewi Lestari"}}`,
		"MULTIPLE",
		"Pindah nomor & rekening gaji",
		"2026-06-04T07:15:00Z",
	); err != nil {
		return fmt.Errorf("seed change_request SWP-CHG-2119: %w", err)
	}
	slog.Info("seed: upserted change request", "id", "SWP-CHG-2119", "type", "MULTIPLE")

	// SWP-CHG-2120: EMERGENCY_CONTACT only for Dewi @ CMP-0021 — a non-bank request
	// the shift leader can fully approve (no escalation).
	if _, err := pool.Pool.Exec(ctx, crQ,
		"SWP-CHG-2120",
		"SWP-EMP-3001",
		`{"emergency_contact":{"name":"Siti Lestari","phone":"+62-877-1234-9000"}}`,
		"EMERGENCY_CONTACT",
		"Perbarui kontak darurat",
		"2026-06-04T09:30:00Z",
	); err != nil {
		return fmt.Errorf("seed change_request SWP-CHG-2120: %w", err)
	}
	slog.Info("seed: upserted change request", "id", "SWP-CHG-2120", "type", "EMERGENCY_CONTACT")

	return nil
}

// seedPlacements inserts Phase-5 (E3) placement + shift-leader fixtures.
// All inserts use ON CONFLICT (id) DO NOTHING so re-runs are idempotent.
//
// First adds the persona agreements that seedAgreements did not create
// (only SWP-AG-7001/Budi exists), since a placement references an active
// agreement:
//   - SWP-AG-7002  ACTIVE PKWTT  for Sari Hadi   (SWP-EMP-1042)
//   - SWP-AG-7003  ACTIVE PKWT   for Rudi Wijaya (SWP-EMP-1108)
//   - SWP-AG-7004  ACTIVE PKWT   for Dewi Lestari (SWP-EMP-3001)
//
// Placements (lifecycle_status=ACTIVE):
//   - SWP-PL-5001  Rudi  @ SWP-CMP-0021 / SWP-SITE-0001 / Parking      (he leads where he is placed → INV-2/4 hold)
//   - SWP-PL-5002  Budi  @ SWP-CMP-0022 / SWP-SITE-0002 / Parking
//   - SWP-PL-5003  Sari  @ SWP-CMP-0021 / SWP-SITE-0001 / Building Mgmt (open-ended)
//   - SWP-PL-5004  Dewi  @ SWP-CMP-0021 / SWP-SITE-0001 / Parking      (end_date = today+20d → DTO derives EXPIRING)
//
// Shift-leader assignment:
//   - SWP-SLA-3001  Rudi (SWP-EMP-1108) @ SWP-CMP-0021 (company-scope, assigned_by 'system-seed')
func seedPlacements(ctx context.Context, pool *db.Pool) error {
	const agQ = `
		INSERT INTO employment_agreements
			(id, employee_id, type, agreement_no, start_date, end_date, status,
			 base_salary_idr, bpjs_terms, tax_profile, comp_effective_date, created_by)
		VALUES
			($1, $2, $3, $4, $5::date, $6, 'active',
			 4900000,
			 '{"kesehatan_employer_pct":4.0,"kesehatan_employee_pct":1.0,"ketenagakerjaan_employer_pct":6.24,"ketenagakerjaan_employee_pct":3.0}'::jsonb,
			 'PTKP_K0', $5::date, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	type agreement struct {
		id, employeeID, typ, no, start string
		end                            *string
	}
	endPKWT := "2026-12-31"
	agreements := []agreement{
		{"SWP-AG-7002", "SWP-EMP-1042", "PKWTT", "PKWTT/SWP/2026/0042", "2020-03-01", nil},
		{"SWP-AG-7003", "SWP-EMP-1108", "PKWT", "PKWT/SWP/2026/0108", "2026-01-01", &endPKWT},
		{"SWP-AG-7004", "SWP-EMP-3001", "PKWT", "PKWT/SWP/2026/3001", "2026-01-01", &endPKWT},
	}
	for _, a := range agreements {
		if _, err := pool.Pool.Exec(ctx, agQ, a.id, a.employeeID, a.typ, a.no, a.start, a.end); err != nil {
			return fmt.Errorf("seed agreement %q: %w", a.id, err)
		}
		slog.Info("seed: upserted agreement", "id", a.id, "employee_id", a.employeeID)
	}

	// Placements. Insert with explicit IDs (the column DEFAULT only fires when id
	// is omitted; an explicit id is honoured) so E2E targets are deterministic.
	const plQ = `
		INSERT INTO placements
			(id, employee_id, agreement_id, client_company_id, site_id, service_line_id,
			 position_id, start_date, end_date, lifecycle_status, status_changed_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::date, $9, $10, now(), 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	today := time.Now()
	expEnd := today.AddDate(0, 0, 20).Format("2006-01-02") // SWP-PL-5004 EXPIRING window
	dewiStart := today.AddDate(0, 0, -100).Format("2006-01-02")

	type placement struct {
		id, employeeID, agreementID, companyID, siteID, serviceLineID, positionID, start string
		end                                                                              *string
	}
	endRudi := "2026-12-31"
	endBudi := "2026-12-31"
	placements := []placement{
		{"SWP-PL-5001", "SWP-EMP-1108", "SWP-AG-7003", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-003", "SWP-POS-014", "2026-01-01", &endRudi},
		{"SWP-PL-5002", "SWP-EMP-2891", "SWP-AG-7001", "SWP-CMP-0022", "SWP-SITE-0002", "SWP-SVC-003", "SWP-POS-014", "2026-02-01", &endBudi},
		{"SWP-PL-5003", "SWP-EMP-1042", "SWP-AG-7002", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-002", "SWP-POS-015", "2026-03-01", nil},
		{"SWP-PL-5004", "SWP-EMP-3001", "SWP-AG-7004", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-003", "SWP-POS-014", dewiStart, &expEnd},
	}
	for _, p := range placements {
		if _, err := pool.Pool.Exec(ctx, plQ,
			p.id, p.employeeID, p.agreementID, p.companyID, p.siteID, p.serviceLineID,
			p.positionID, p.start, p.end, "ACTIVE",
		); err != nil {
			return fmt.Errorf("seed placement %q: %w", p.id, err)
		}
		slog.Info("seed: upserted placement", "id", p.id, "employee_id", p.employeeID)

		// One "create" history row per placement (so the detail Riwayat panel renders).
		const histQ = `
			INSERT INTO placement_history (placement_id, action, status_after, effective_date)
			VALUES ($1, 'create', 'ACTIVE', $2::date)
			ON CONFLICT DO NOTHING`
		if _, err := pool.Pool.Exec(ctx, histQ, p.id, p.start); err != nil {
			return fmt.Errorf("seed placement_history for %q: %w", p.id, err)
		}
	}

	// One active shift-leader assignment at SWP-CMP-0021: Rudi (company-scope).
	const slaQ = `
		INSERT INTO shift_leader_assignments
			(id, client_company_id, site_id, employee_id, assigned_by)
		VALUES ($1, $2, NULL, $3, 'system-seed')
		ON CONFLICT (id) DO NOTHING`
	if _, err := pool.Pool.Exec(ctx, slaQ, "SWP-SLA-3001", "SWP-CMP-0021", "SWP-EMP-1108"); err != nil {
		return fmt.Errorf("seed shift_leader_assignment SWP-SLA-3001: %w", err)
	}
	slog.Info("seed: upserted shift_leader_assignment", "id", "SWP-SLA-3001", "employee_id", "SWP-EMP-1108")

	// Lead assignments: Joko (SWP-EMP-3004) is the `lead` covering BOTH seeded
	// companies (SWP-CMP-0021 + SWP-CMP-0022). Two ACTIVE rows (unassigned_at NULL,
	// one per company) — the auth middleware derives Principal.CompanyIDs from these
	// at request time, scoping his placement arrangement + L2 approvals. Explicit
	// SWP-LA-xxxx ids (the column DEFAULT only fires when id is omitted).
	const laQ = `
		INSERT INTO lead_assignments
			(id, client_company_id, site_id, employee_id, assigned_by)
		VALUES ($1, $2, NULL, $3, 'system-seed')
		ON CONFLICT (id) DO NOTHING`
	leadRows := []struct{ id, companyID string }{
		{"SWP-LA-4001", "SWP-CMP-0021"},
		{"SWP-LA-4002", "SWP-CMP-0022"},
	}
	for _, lr := range leadRows {
		if _, err := pool.Pool.Exec(ctx, laQ, lr.id, lr.companyID, "SWP-EMP-3004"); err != nil {
			return fmt.Errorf("seed lead_assignment %q: %w", lr.id, err)
		}
		slog.Info("seed: upserted lead_assignment", "id", lr.id, "employee_id", "SWP-EMP-3004", "company_id", lr.companyID)
	}

	return nil
}

// mondayOfCurrentWeek returns the Monday (00:00 UTC-stamped) of the week containing
// `now`, where "now" is resolved to its **Asia/Jakarta calendar date** first. This
// anchors the seeded week on the same WIB day the web grid + agent dashboard derive
// "today" from (todayJakartaIso / DashboardRepo.AgentTodayShift) so the seed's
// rudi/dewi/leave/free dates line up exactly with the grid cells, even when the
// process clock is UTC and the UTC date is a day behind WIB at the midnight boundary.
func mondayOfCurrentWeek(now time.Time) time.Time {
	jkt, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		jkt = time.FixedZone("WIB", 7*3600)
	}
	nj := now.In(jkt)
	d := time.Date(nj.Year(), nj.Month(), nj.Day(), 0, 0, 0, 0, time.UTC)
	// Go: Sunday=0..Saturday=6. ISO Monday-start offset.
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset)
}

// seedScheduling inserts Phase-6 (E4) shift-master + schedule + approved-leave
// fixtures. All inserts are idempotent (ON CONFLICT DO NOTHING). FK: schedule
// entries → placements/employees/shift_masters (runs AFTER seedPlacements).
//
// Shift masters (explicit deterministic ids; column DEFAULT only fires when id
// is omitted, an explicit id is honoured):
//   - SWP-SHF-001  "Pagi"  07:00–15:00  break 12:00–13:00  service_line NULL (all lines)
//   - SWP-SHF-002  "Malam" 23:00–07:00  (cross_midnight=true)  service_line SWP-SVC-003 (Parking)
//
// Schedule entries (so the grid renders agents at CMP-0021) — dated a few days
// into the CURRENT week (Tuesday/Wednesday), inside each placement window:
//   - SWP-SCH-6001  Rudi (SWP-EMP-1108, SWP-PL-5001) on monday+1 — "Pagi" SCHEDULED
//   - SWP-SCH-6002  Dewi (SWP-EMP-3001, SWP-PL-5004) on monday+2 — "Pagi" SCHEDULED
//
// Approved-leave day (exercises SHIFT_OVER_LEAVE) — Thursday (monday+3), a date
// NOT taken by Dewi's schedule entry so 06-04 can attempt to schedule her there:
//   - approved_leave_days: SWP-EMP-3001 / leave_date=monday+3 / SWP-LR-44210 / ANNUAL
//
// NOTE for 06-03 / 06-04: Budi (SWP-EMP-2891) is placed at CMP-0022 (SWP-PL-5002)
// — he is the leader-scope-403 target (Rudi leads CMP-0021, cannot touch Budi).
func seedScheduling(ctx context.Context, pool *db.Pool) error {
	// --- Shift masters ---
	const shfQ = `
		INSERT INTO shift_masters
			(id, name, start_time, end_time, break_start, break_end,
			 service_line_id, cross_midnight, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	type shiftMaster struct {
		id, name, start, end string
		breakStart, breakEnd *string
		serviceLineID        *string
		crossMidnight        bool
	}
	bs := "12:00"
	be := "13:00"
	parking := "SWP-SVC-003"
	masters := []shiftMaster{
		{id: "SWP-SHF-001", name: "Pagi", start: "07:00", end: "15:00", breakStart: &bs, breakEnd: &be, serviceLineID: nil, crossMidnight: false},
		{id: "SWP-SHF-002", name: "Malam", start: "23:00", end: "07:00", breakStart: nil, breakEnd: nil, serviceLineID: &parking, crossMidnight: true},
	}
	for _, m := range masters {
		if _, err := pool.Pool.Exec(ctx, shfQ,
			m.id, m.name, m.start, m.end, m.breakStart, m.breakEnd,
			m.serviceLineID, m.crossMidnight,
		); err != nil {
			return fmt.Errorf("seed shift_master %q: %w", m.id, err)
		}
		slog.Info("seed: upserted shift master", "id", m.id, "name", m.name)
	}

	monday := mondayOfCurrentWeek(time.Now())
	rudiDate := monday.AddDate(0, 0, 1).Format("2006-01-02")  // Tuesday
	dewiDate := monday.AddDate(0, 0, 2).Format("2006-01-02")  // Wednesday
	leaveDate := monday.AddDate(0, 0, 3).Format("2006-01-02") // Thursday (over-leave target)

	// TODAY (Asia/Jakarta) — matches the agent dashboard's today_shift resolution
	// (DashboardRepo.AgentTodayShift uses jakartaNow). Gives the agent persona
	// (Budi, SWP-EMP-2891 @ SWP-PL-5002) a clockable shift so the self-service
	// Kehadiran screen ("Absen Sekarang") is enabled/testable.
	jkt, locErr := time.LoadLocation("Asia/Jakarta")
	if locErr != nil {
		jkt = time.FixedZone("WIB", 7*3600)
	}
	agentToday := time.Now().In(jkt).Format("2006-01-02")

	// --- Schedule entries (snapshot Pagi 07:00–15:00 onto each cell) ---
	const schQ = `
		INSERT INTO schedule_entries
			(id, employee_id, placement_id, service_line_id, shift_master_id,
			 start_time, end_time, cross_midnight, work_date, status, is_day_off, created_by)
		VALUES ($1, $2, $3, $4, 'SWP-SHF-001', '07:00', '15:00', false, $5::date, 'SCHEDULED', false, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	type entry struct {
		id, employeeID, placementID, serviceLineID, date string
	}
	entries := []entry{
		{"SWP-SCH-6001", "SWP-EMP-1108", "SWP-PL-5001", "SWP-SVC-003", rudiDate},
		{"SWP-SCH-6002", "SWP-EMP-3001", "SWP-PL-5004", "SWP-SVC-003", dewiDate},
		// SWP-SCH-6003 — scheduled shift for the E5 true-ABSENT fixture (SWP-ATT-9009):
		// the agent was scheduled but never clocked in.
		{"SWP-SCH-6003", "SWP-EMP-1042", "SWP-PL-5003", "SWP-SVC-002", rudiDate},
		// SWP-SCH-6004 — the agent persona's shift for TODAY (enables the /me clock CTA).
		{"SWP-SCH-6004", "SWP-EMP-2891", "SWP-PL-5002", "SWP-SVC-003", agentToday},
	}
	for _, e := range entries {
		if _, err := pool.Pool.Exec(ctx, schQ, e.id, e.employeeID, e.placementID, e.serviceLineID, e.date); err != nil {
			return fmt.Errorf("seed schedule_entry %q: %w", e.id, err)
		}
		slog.Info("seed: upserted schedule entry", "id", e.id, "employee_id", e.employeeID, "work_date", e.date)
	}

	// --- Approved-leave day (SHIFT_OVER_LEAVE fixture) ---
	const aldQ = `
		INSERT INTO approved_leave_days (employee_id, leave_date, leave_request_id, leave_type)
		VALUES ($1, $2::date, $3, $4)
		ON CONFLICT (employee_id, leave_date) DO NOTHING`
	if _, err := pool.Pool.Exec(ctx, aldQ, "SWP-EMP-3001", leaveDate, "SWP-LR-44210", "ANNUAL"); err != nil {
		return fmt.Errorf("seed approved_leave_days SWP-LR-44210: %w", err)
	}
	slog.Info("seed: upserted approved leave day", "employee_id", "SWP-EMP-3001", "leave_date", leaveDate, "leave_request_id", "SWP-LR-44210")

	return nil
}

// seedAttendance inserts Phase-7 (E5) attendance fixtures so the verification
// queue + detail + single + bulk flows have honest exception records to act on.
// All inserts are idempotent (ON CONFLICT (id) DO NOTHING). Dates anchor a few
// days into the CURRENT week (Asia/Jakarta-safe, well inside the correction
// window) so the corrections approve/reject + window checks are exercisable.
//
// Geofence/lateness/auto-close are STORED columns (07-01) — set directly here;
// there is no mobile clock pipeline. Site coords ~ Plaza Senayan; radius 100m.
//
// Records (explicit ids; column DEFAULT only fires when id is omitted):
//   - SWP-ATT-9001  Dewi  @ CMP-0021/PL-5004  AUTO_APPROVED (clean; NOT in queue)
//   - SWP-ATT-9002  Dewi  @ CMP-0021/PL-5004  PENDING, flags={LATE}, is_late, late_minutes=18
//   - SWP-ATT-9003  Sari  @ CMP-0021/PL-5003  PENDING, flags={OUTSIDE_GEOFENCE}, in_geofence=false
//   - SWP-ATT-9004  Dewi  @ CMP-0021/PL-5004  PENDING, flags={AUTO_CLOSED}, auto_closed, check_out_at NULL
//   - SWP-ATT-9005  Budi  @ CMP-0022/PL-5002  PENDING, flags={LATE}  → cross-company OUT_OF_SCOPE target
//   - SWP-ATT-9006  Rudi  @ CMP-0021/PL-5001  ESCALATED, flags={LATE,ESCALATED}  → VERIFY_OWN_RECORD target
func seedAttendance(ctx context.Context, pool *db.Pool) error {
	// site_id/position_id are denormalized from the row's placement (subqueries —
	// keeps the positional param list stable). schedule_id is now per-row ($4) so the
	// ABSENT fixture can carry a scheduled shift; check_in_at/lat_in/lng_in are nullable
	// (a true ABSENT row has none).
	const attQ = `
		INSERT INTO attendance
			(id, employee_id, placement_id, schedule_id, company_id, service_line,
			 site_id, position_id,
			 shift_start_at, shift_end_at, check_in_at, check_out_at,
			 lat_in, lng_in, lat_out, lng_out, wfo,
			 is_late, late_minutes, worked_minutes, auto_closed,
			 in_geofence, in_distance_m, out_geofence, out_distance_m, geofence_radius_m,
			 status, verification_status, flags)
		VALUES
			($1, $2, $3, $4, $5, $6,
			 (SELECT site_id FROM placements WHERE id = $3),
			 (SELECT position_id FROM placements WHERE id = $3),
			 $7, $8, $9, $10,
			 $11, $12, $13, $14, true,
			 $15, $16, $17, $18,
			 $19, $20, $21, $22, 100,
			 $23, $24, $25)
		ON CONFLICT (id) DO NOTHING`

	// Site centroid (Plaza Senayan-ish) — in-geofence captures sit near it.
	const latC = -6.2256
	const lngC = 106.7997

	// Anchor shift instants a few days into the current week (in-window for
	// corrections). check_in_at is a timestamptz; we render RFC3339 UTC.
	monday := mondayOfCurrentWeek(time.Now())
	shiftDay := monday.AddDate(0, 0, 1)                                                              // Tuesday of this week
	shiftStart := time.Date(shiftDay.Year(), shiftDay.Month(), shiftDay.Day(), 0, 0, 0, 0, time.UTC) // 07:00 WIB = 00:00 UTC
	shiftEnd := shiftStart.Add(8 * time.Hour)                                                        // 15:00 WIB
	onTimeIn := shiftStart                                                                           // 07:00 WIB
	lateIn := shiftStart.Add(18 * time.Minute)                                                       // 07:18 WIB (18m late)
	normalOut := shiftEnd                                                                            // 15:00 WIB

	ss := shiftStart.Format(time.RFC3339)
	se := shiftEnd.Format(time.RFC3339)
	worked := int32(480)

	type att struct {
		id, employeeID, placementID, companyID, serviceLine string
		scheduleID                                          *string
		checkIn                                             *time.Time // nil = true ABSENT (no clock-in)
		checkOut                                            *time.Time
		latIn, lngIn                                        *float64 // nil = true ABSENT (no clock-in GPS)
		latOut, lngOut                                      *float64
		isLate                                              bool
		lateMinutes                                         int32
		workedMinutes                                       *int32
		autoClosed                                          bool
		inGeofence                                          *bool
		inDistanceM                                         *int32
		outGeofence                                         *bool
		outDistanceM                                        *int32
		status, verification                                string
		flags                                               string // postgres array literal
	}

	out := normalOut
	latInP := latC
	lngInP := lngC
	latOut := latC
	lngOut := lngC
	onTimeInP := onTimeIn
	lateInP := lateIn
	inTrue := true
	inFalse := false
	d32 := int32(32)
	dFar := int32(420)
	schAbsent := "SWP-SCH-6003" // scheduled shift backing the true-ABSENT fixture

	rows := []att{
		// 9001 — clean AUTO_APPROVED (complete, on-time, in-geofence). NOT in queue.
		{
			id: "SWP-ATT-9001", employeeID: "SWP-EMP-3001", placementID: "SWP-PL-5004",
			companyID: "SWP-CMP-0021", serviceLine: "parking",
			checkIn: &onTimeInP, latIn: &latInP, lngIn: &lngInP, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: false, lateMinutes: 0, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "PRESENT", verification: "AUTO_APPROVED", flags: "{}",
		},
		// 9002 — PENDING LATE (18m). Correction CHECK_IN target (in-window).
		{
			id: "SWP-ATT-9002", employeeID: "SWP-EMP-3001", placementID: "SWP-PL-5004",
			companyID: "SWP-CMP-0021", serviceLine: "parking",
			checkIn: &lateInP, latIn: &latInP, lngIn: &lngInP, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: true, lateMinutes: 18, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "LATE", verification: "PENDING", flags: "{LATE}",
		},
		// 9003 — PENDING OUTSIDE_GEOFENCE (in_geofence=false).
		{
			id: "SWP-ATT-9003", employeeID: "SWP-EMP-1042", placementID: "SWP-PL-5003",
			companyID: "SWP-CMP-0021", serviceLine: "building_management",
			checkIn: &onTimeInP, latIn: &latInP, lngIn: &lngInP, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: false, lateMinutes: 0, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inFalse, inDistanceM: &dFar, outGeofence: &inTrue, outDistanceM: &d32,
			status: "PRESENT", verification: "PENDING", flags: "{OUTSIDE_GEOFENCE}",
		},
		// 9004 — PENDING AUTO_CLOSED (no clock-out). Correction CHECK_OUT target.
		{
			id: "SWP-ATT-9004", employeeID: "SWP-EMP-3001", placementID: "SWP-PL-5004",
			companyID: "SWP-CMP-0021", serviceLine: "parking",
			checkIn: &onTimeInP, latIn: &latInP, lngIn: &lngInP, checkOut: nil, latOut: nil, lngOut: nil,
			isLate: false, lateMinutes: 0, workedMinutes: nil, autoClosed: true,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: nil, outDistanceM: nil,
			status: "INCOMPLETE", verification: "PENDING", flags: "{AUTO_CLOSED}",
		},
		// 9005 — CMP-0022 PENDING LATE → cross-company OUT_OF_SCOPE for Rudi.
		{
			id: "SWP-ATT-9005", employeeID: "SWP-EMP-2891", placementID: "SWP-PL-5002",
			companyID: "SWP-CMP-0022", serviceLine: "parking",
			checkIn: &lateInP, latIn: &latInP, lngIn: &lngInP, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: true, lateMinutes: 18, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "LATE", verification: "PENDING", flags: "{LATE}",
		},
		// 9006 — Rudi's OWN ESCALATED record → VERIFY_OWN_RECORD target.
		{
			id: "SWP-ATT-9006", employeeID: "SWP-EMP-1108", placementID: "SWP-PL-5001",
			companyID: "SWP-CMP-0021", serviceLine: "parking",
			checkIn: &lateInP, latIn: &latInP, lngIn: &lngInP, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: true, lateMinutes: 18, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "LATE", verification: "ESCALATED", flags: "{LATE,ESCALATED}",
		},
		// 9007 / 9008 — VERIFIED billable rows (E10 11-02b): so /reports/
		// attendance-billable + the dashboard return non-empty for CMP-0021
		// (HR global + Rudi's leader scope). attendance_code PRESENT is is_billable.
		{
			id: "SWP-ATT-9007", employeeID: "SWP-EMP-3001", placementID: "SWP-PL-5004",
			companyID: "SWP-CMP-0021", serviceLine: "parking",
			checkIn: &onTimeInP, latIn: &latInP, lngIn: &lngInP, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: false, lateMinutes: 0, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "PRESENT", verification: "VERIFIED", flags: "{VERIFIED}",
		},
		{
			id: "SWP-ATT-9008", employeeID: "SWP-EMP-1042", placementID: "SWP-PL-5003",
			companyID: "SWP-CMP-0021", serviceLine: "building_management",
			checkIn: &onTimeInP, latIn: &latInP, lngIn: &lngInP, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: false, lateMinutes: 0, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "PRESENT", verification: "VERIFIED", flags: "{VERIFIED}",
		},
		// 9009 — TRUE ABSENT (CR / F5.2 INV-5): scheduled shift (SWP-SCH-6003), NO
		// clock-in (check_in_at NULL), NO clock-in GPS (lat_in/lng_in NULL), PENDING.
		// The CHECK_IN-correction re-eval (BR CR-9) target: approving a clock-in flips
		// ABSENT → PRESENT/LATE.
		{
			id: "SWP-ATT-9009", employeeID: "SWP-EMP-1042", placementID: "SWP-PL-5003",
			companyID: "SWP-CMP-0021", serviceLine: "building_management", scheduleID: &schAbsent,
			checkIn: nil, latIn: nil, lngIn: nil, checkOut: nil, latOut: nil, lngOut: nil,
			isLate: false, lateMinutes: 0, workedMinutes: nil, autoClosed: false,
			inGeofence: nil, inDistanceM: nil, outGeofence: nil, outDistanceM: nil,
			status: "ABSENT", verification: "PENDING", flags: "{ABSENT}",
		},
	}

	for _, a := range rows {
		if _, err := pool.Pool.Exec(ctx, attQ,
			a.id, a.employeeID, a.placementID, a.scheduleID, a.companyID, a.serviceLine,
			ss, se, nullableTime(a.checkIn), nullableTime(a.checkOut),
			a.latIn, a.lngIn, a.latOut, a.lngOut,
			a.isLate, a.lateMinutes, a.workedMinutes, a.autoClosed,
			a.inGeofence, a.inDistanceM, a.outGeofence, a.outDistanceM,
			a.status, a.verification, a.flags,
		); err != nil {
			return fmt.Errorf("seed attendance %q: %w", a.id, err)
		}
		slog.Info("seed: upserted attendance", "id", a.id, "employee_id", a.employeeID, "verification_status", a.verification)
	}

	// E10 (11-02b): bind the VERIFIED rows to the billable PRESENT code (SWP-AC-001,
	// is_billable=true) so /reports/attendance-billable reports non-zero billable
	// hours. The shared attendance insert leaves attendance_code_id NULL, so we set
	// it here only for the billable fixtures.
	if _, err := pool.Pool.Exec(ctx,
		`UPDATE attendance SET attendance_code_id = 'SWP-AC-001'
		 WHERE id IN ('SWP-ATT-9007','SWP-ATT-9008') AND attendance_code_id IS NULL`,
	); err != nil {
		return fmt.Errorf("seed attendance billable code: %w", err)
	}

	return nil
}

// seedCorrections inserts Phase-7 (E5) PENDING correction fixtures so the
// corrections queue + approve/reject flows have real targets. Idempotent
// (ON CONFLICT (id) DO NOTHING). Both target CMP-0021 records inside the 7-day
// window so HR/leader approve works; OUTSIDE_CORRECTION_WINDOW is driven directly
// by the 07-03 contract test via the exported CheckCorrectionWindow seam (the
// correction-CREATE endpoint is out of web scope).
//
//   - SWP-COR-8001  PENDING/CHECK_OUT on SWP-ATT-9004 (proposes a clock-out time;
//     original_snapshot captures the auto_closed=true / INCOMPLETE state) → approve target.
//   - SWP-COR-8002  PENDING/CHECK_IN on SWP-ATT-9002 (proposes an on-time check-in) → reject target.
func seedCorrections(ctx context.Context, pool *db.Pool) error {
	const corQ = `
		INSERT INTO attendance_corrections
			(id, attendance_id, requester_id, company_id, type,
			 proposed_check_in_at, proposed_check_out_at, proposed_attendance_code_id,
			 reason, evidence_file_id, status, original_snapshot, attendance_shift_date)
		VALUES
			($1, $2, $3, $4, $5,
			 $6, $7, NULL,
			 $8, $9, 'PENDING', $10::jsonb, $11::date)
		ON CONFLICT (id) DO NOTHING`

	monday := mondayOfCurrentWeek(time.Now())
	shiftDay := monday.AddDate(0, 0, 1) // Tuesday (same as attendance records)
	shiftDate := shiftDay.Format("2006-01-02")
	shiftStart := time.Date(shiftDay.Year(), shiftDay.Month(), shiftDay.Day(), 0, 0, 0, 0, time.UTC)
	proposedOut := shiftStart.Add(8*time.Hour + 10*time.Minute).Format(time.RFC3339) // 15:10 WIB
	proposedIn := shiftStart.Format(time.RFC3339)                                    // 07:00 WIB (on time)

	type correction struct {
		id, attendanceID, requesterID, companyID, typ string
		proposedIn, proposedOut                       *string
		reason, evidenceFileID, snapshot              string
	}
	pOut := proposedOut
	pIn := proposedIn
	evidence := "SWP-FILE-cor-9001"
	corrections := []correction{
		{
			id: "SWP-COR-8001", attendanceID: "SWP-ATT-9004", requesterID: "SWP-EMP-3001",
			companyID: "SWP-CMP-0021", typ: "CHECK_OUT",
			proposedIn: nil, proposedOut: &pOut,
			reason:         "Lupa clock-out, sudah pulang pukul 15:10.",
			evidenceFileID: evidence,
			snapshot:       `{"check_out_at": null, "auto_closed": true, "status": "INCOMPLETE"}`,
		},
		{
			id: "SWP-COR-8002", attendanceID: "SWP-ATT-9002", requesterID: "SWP-EMP-3001",
			companyID: "SWP-CMP-0021", typ: "CHECK_IN",
			proposedIn: &pIn, proposedOut: nil,
			reason:         "Clock-in tercatat telat karena GPS lambat; sebenarnya tepat waktu.",
			evidenceFileID: evidence,
			snapshot:       `{"check_in_at": null, "is_late": true, "late_minutes": 18, "status": "LATE"}`,
		},
	}

	for _, c := range corrections {
		if _, err := pool.Pool.Exec(ctx, corQ,
			c.id, c.attendanceID, c.requesterID, c.companyID, c.typ,
			c.proposedIn, c.proposedOut,
			c.reason, c.evidenceFileID, c.snapshot, shiftDate,
		); err != nil {
			return fmt.Errorf("seed correction %q: %w", c.id, err)
		}
		slog.Info("seed: upserted correction", "id", c.id, "attendance_id", c.attendanceID, "type", c.typ)
	}

	return nil
}

// nullableTime renders a *time.Time as an RFC3339 string or nil (for a NULL
// timestamptz bind).
func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}
