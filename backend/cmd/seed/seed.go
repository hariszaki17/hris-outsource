package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
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
		password:   PasswordShiftLeader,
		role:       "shift_leader",
		fullName:   "Rudi Wijaya",
		employeeID: strPtr("SWP-EMP-1108"),
		companyID:  strPtr("SWP-CMP-0021"),
	},
	{
		email:      "super.admin@swp.test",
		password:   PasswordSuperAdmin,
		role:       "super_admin",
		fullName:   "Super Admin",
		employeeID: nil,
		companyID:  nil,
	},
	{
		email:      "agent.budi@swp.test",
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
		password:   "Dew1-Lestari-2026!",
		role:       "agent",
		fullName:   "Dewi Lestari",
		employeeID: strPtr("SWP-EMP-3001"),
		companyID:  nil,
	},
	{
		email:      "agus.pratama@swp.test",
		password:   "Agus-Pr4tama-2026!",
		role:       "shift_leader",
		fullName:   "Agus Pratama",
		employeeID: strPtr("SWP-EMP-3002"),
		companyID:  strPtr("SWP-CMP-0021"),
	},
	{
		email:      "bambang.admin@swp.test",
		password:   "B4mbang-Admin-2026!",
		role:       "hr_admin",
		fullName:   "Bambang Sutrisno",
		employeeID: strPtr("SWP-EMP-3003"),
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
			Email:        p.email,
			PasswordHash: hash,
			Role:         p.role,
			FullName:     p.fullName,
			EmployeeID:   p.employeeID,
			CompanyID:    p.companyID,
		})
		if err != nil {
			return fmt.Errorf("create user %q: %w", p.email, err)
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
	// Phase 4 (04-04): Seed pending change-requests.
	// FK: change_requests → employees (must run AFTER seedEmployees).
	// Two PENDING CRs against Budi Santoso (SWP-EMP-2891) so the HR queue
	// renders content on first load and the diff (old→new) is meaningful:
	//   SWP-CHG-2117  MULTIPLE (phone + bank_account change)
	//   SWP-CHG-2118  ADDRESS change
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

	// --- leave_quotas (explicit ids for deterministic E2E) ---
	const lqQ = `
		INSERT INTO leave_quotas
			(id, employee_id, leave_type_id, period, period_start, period_end, total, used, pending)
		VALUES ($1, $2, 'SWP-LT-001', $3, $4::date, $5::date, $6, $7, 0)
		ON CONFLICT (id) DO NOTHING`
	quotas := []struct {
		id, employeeID string
		total, used    int
	}{
		{"SWP-LQ-8001", "SWP-EMP-3001", 12, 4},  // Dewi: remaining 8
		{"SWP-LQ-8002", "SWP-EMP-2891", 12, 11}, // Budi: remaining 1
	}
	for _, q := range quotas {
		if _, err := pool.Pool.Exec(ctx, lqQ, q.id, q.employeeID, year, periodStart, periodEnd, q.total, q.used); err != nil {
			return fmt.Errorf("seed leave_quota %q: %w", q.id, err)
		}
		slog.Info("seed: upserted leave quota", "id", q.id, "employee_id", q.employeeID, "remaining", q.total-q.used)
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
		bankName              *string
		bankAccountNumber     *string
		bankAccountHolderName *string
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
			id:       "SWP-EMP-3001",
			fullName: "Dewi Lestari",
			nik:      "3175001505903001",
			nip:      "3001",
			joinAt:   "2022-04-01",
			gender:   "FEMALE",
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
	}

	const empQ = `
		INSERT INTO employees
			(id, full_name, nik, nip, join_at, gender,
			 phone, bank_name, bank_account_number, bank_account_holder_name,
			 status)
		VALUES ($1, $2, $3, $4, $5::date, $6,
		        $7, $8, $9, $10,
		        'active')
		ON CONFLICT (id) DO NOTHING`

	for _, e := range employees {
		if _, err := pool.Pool.Exec(ctx, empQ,
			e.id, e.fullName, e.nik, e.nip, e.joinAt, e.gender,
			e.phone, e.bankName, e.bankAccountNumber, e.bankAccountHolderName,
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
// Leave types:
//   - SWP-LT-001  "Cuti Tahunan"   code ANNUAL  is_annual=true  requires_document=false
//   - SWP-LT-002  "Cuti Sakit"     code SICK    is_annual=false requires_document=true
//
// Attendance codes:
//   - SWP-AC-001  code PRESENT  label "Hadir"     color #0F8B8D  is_workday=true  is_paid=true  is_billable=true  needs_verification=true
//   - SWP-AC-002  code LATE     label "Terlambat"  color #E07A2A  same flags as PRESENT
//
// Overtime rules:
//   - SWP-OTR-001  "Default OT"  service_line_id=NULL  weekday_rate=1.5 restday_rate=2.0 holiday_rate=3.0
//     min_minutes=30 max_minutes_per_day=240 pre_approval_required=true
func seedMasterData(ctx context.Context, pool *db.Pool) error {
	// --- Leave types ---
	type leaveType struct {
		id                 string
		name               string
		code               string
		description        string
		defaultAnnualQuota int
		isAnnual           bool
		requiresDocument   bool
		color              string
	}

	leaveTypes := []leaveType{
		{
			id:                 "SWP-LT-001",
			name:               "Cuti Tahunan",
			code:               "ANNUAL",
			description:        "Cuti tahunan wajib sesuai peraturan ketenagakerjaan.",
			defaultAnnualQuota: 12,
			isAnnual:           true,
			requiresDocument:   false,
			color:              "#188E4D",
		},
		{
			id:                 "SWP-LT-002",
			name:               "Cuti Sakit",
			code:               "SICK",
			description:        "Cuti sakit dengan surat dokter.",
			defaultAnnualQuota: 0,
			isAnnual:           false,
			requiresDocument:   true,
			color:              "#E07A2A",
		},
	}

	const ltQ = `
		INSERT INTO leave_types
			(id, name, code, description, default_annual_quota, is_annual, requires_document, color, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active')
		ON CONFLICT (id) DO NOTHING`

	for _, lt := range leaveTypes {
		if _, err := pool.Pool.Exec(ctx, ltQ,
			lt.id, lt.name, lt.code, lt.description,
			lt.defaultAnnualQuota, lt.isAnnual, lt.requiresDocument, lt.color,
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

// seedChangeRequests inserts Phase-4 pending change-request fixtures.
// All inserts use ON CONFLICT (id) DO NOTHING so re-runs are idempotent.
//
// Budi Santoso (SWP-EMP-2891) has phone "+62-812-3344-5566" and BCA bank
// account "1234567890" seeded in seedEmployees — these are the "old" values
// so the HR approval detail renders a meaningful old→new diff.
//
// Change requests:
//
//	SWP-CHG-2117  MULTIPLE  — phone + bank_account change (status: pending)
//	SWP-CHG-2118  ADDRESS   — address change              (status: pending)
func seedChangeRequests(ctx context.Context, pool *db.Pool) error {
	// SWP-CHG-2117: MULTIPLE (phone + bank_account).
	const crQ = `
		INSERT INTO change_requests
			(id, employee_id, changes, request_type, note, submitted_at)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6::timestamptz)
		ON CONFLICT (id) DO NOTHING`

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

	// SWP-CHG-2118: ADDRESS change.
	if _, err := pool.Pool.Exec(ctx, crQ,
		"SWP-CHG-2118",
		"SWP-EMP-2891",
		`{"address":"Jl. Melati 5, Jakarta Selatan"}`,
		"ADDRESS",
		nil,
		"2026-06-03T09:30:00Z",
	); err != nil {
		return fmt.Errorf("seed change_request SWP-CHG-2118: %w", err)
	}
	slog.Info("seed: upserted change request", "id", "SWP-CHG-2118", "type", "ADDRESS")

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

	return nil
}

// mondayOfCurrentWeek returns the Monday (00:00 Asia/Jakarta-anchored UTC date)
// of the week containing now. Schedule fixtures are placed a few days INTO this
// week so they land inside the visible grid AND avoid the Asia/Jakarta-vs-UTC
// midnight boundary the fixed E2E clock derives statuses at (05-03 TZ note).
func mondayOfCurrentWeek(now time.Time) time.Time {
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
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
	const attQ = `
		INSERT INTO attendance
			(id, employee_id, placement_id, schedule_id, company_id, service_line,
			 shift_start_at, shift_end_at, check_in_at, check_out_at,
			 lat_in, lng_in, lat_out, lng_out, wfo,
			 is_late, late_minutes, worked_minutes, auto_closed,
			 in_geofence, in_distance_m, out_geofence, out_distance_m, geofence_radius_m,
			 status, verification_status, flags)
		VALUES
			($1, $2, $3, NULL, $4, $5,
			 $6, $7, $8, $9,
			 $10, $11, $12, $13, true,
			 $14, $15, $16, $17,
			 $18, $19, $20, $21, 100,
			 $22, $23, $24)
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
		checkIn                                             time.Time
		checkOut                                            *time.Time
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
	latOut := latC
	lngOut := lngC
	inTrue := true
	inFalse := false
	d32 := int32(32)
	dFar := int32(420)

	rows := []att{
		// 9001 — clean AUTO_APPROVED (complete, on-time, in-geofence). NOT in queue.
		{
			id: "SWP-ATT-9001", employeeID: "SWP-EMP-3001", placementID: "SWP-PL-5004",
			companyID: "SWP-CMP-0021", serviceLine: "parking",
			checkIn: onTimeIn, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: false, lateMinutes: 0, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "PRESENT", verification: "AUTO_APPROVED", flags: "{}",
		},
		// 9002 — PENDING LATE (18m). Correction CHECK_IN target (in-window).
		{
			id: "SWP-ATT-9002", employeeID: "SWP-EMP-3001", placementID: "SWP-PL-5004",
			companyID: "SWP-CMP-0021", serviceLine: "parking",
			checkIn: lateIn, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: true, lateMinutes: 18, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "LATE", verification: "PENDING", flags: "{LATE}",
		},
		// 9003 — PENDING OUTSIDE_GEOFENCE (in_geofence=false).
		{
			id: "SWP-ATT-9003", employeeID: "SWP-EMP-1042", placementID: "SWP-PL-5003",
			companyID: "SWP-CMP-0021", serviceLine: "building_management",
			checkIn: onTimeIn, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: false, lateMinutes: 0, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inFalse, inDistanceM: &dFar, outGeofence: &inTrue, outDistanceM: &d32,
			status: "PRESENT", verification: "PENDING", flags: "{OUTSIDE_GEOFENCE}",
		},
		// 9004 — PENDING AUTO_CLOSED (no clock-out). Correction CHECK_OUT target.
		{
			id: "SWP-ATT-9004", employeeID: "SWP-EMP-3001", placementID: "SWP-PL-5004",
			companyID: "SWP-CMP-0021", serviceLine: "parking",
			checkIn: onTimeIn, checkOut: nil, latOut: nil, lngOut: nil,
			isLate: false, lateMinutes: 0, workedMinutes: nil, autoClosed: true,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: nil, outDistanceM: nil,
			status: "INCOMPLETE", verification: "PENDING", flags: "{AUTO_CLOSED}",
		},
		// 9005 — CMP-0022 PENDING LATE → cross-company OUT_OF_SCOPE for Rudi.
		{
			id: "SWP-ATT-9005", employeeID: "SWP-EMP-2891", placementID: "SWP-PL-5002",
			companyID: "SWP-CMP-0022", serviceLine: "parking",
			checkIn: lateIn, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: true, lateMinutes: 18, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "LATE", verification: "PENDING", flags: "{LATE}",
		},
		// 9006 — Rudi's OWN ESCALATED record → VERIFY_OWN_RECORD target.
		{
			id: "SWP-ATT-9006", employeeID: "SWP-EMP-1108", placementID: "SWP-PL-5001",
			companyID: "SWP-CMP-0021", serviceLine: "parking",
			checkIn: lateIn, checkOut: &out, latOut: &latOut, lngOut: &lngOut,
			isLate: true, lateMinutes: 18, workedMinutes: &worked, autoClosed: false,
			inGeofence: &inTrue, inDistanceM: &d32, outGeofence: &inTrue, outDistanceM: &d32,
			status: "LATE", verification: "ESCALATED", flags: "{LATE,ESCALATED}",
		},
	}

	for _, a := range rows {
		if _, err := pool.Pool.Exec(ctx, attQ,
			a.id, a.employeeID, a.placementID, a.companyID, a.serviceLine,
			ss, se, a.checkIn.Format(time.RFC3339), nullableTime(a.checkOut),
			latC, lngC, a.latOut, a.lngOut,
			a.isLate, a.lateMinutes, a.workedMinutes, a.autoClosed,
			a.inGeofence, a.inDistanceM, a.outGeofence, a.outDistanceM,
			a.status, a.verification, a.flags,
		); err != nil {
			return fmt.Errorf("seed attendance %q: %w", a.id, err)
		}
		slog.Info("seed: upserted attendance", "id", a.id, "employee_id", a.employeeID, "verification_status", a.verification)
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
