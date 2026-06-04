package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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

	// 04-04 change-requests: append seedChangeRequests call here.

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
