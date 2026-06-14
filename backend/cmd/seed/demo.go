package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
)

// =============================================================================
// DEMO SEED PROFILE (cmd/seed -demo)
// =============================================================================
//
// SeedDemo lays a rich, production-LIKE dataset on TOP of the deterministic E2E
// fixtures created by Seed(). It is ONLY reachable behind the -demo flag (see
// main.go) so the default `go run ./cmd/seed` path — the one the Playwright E2E
// harness (frontend/e2e/lib/backend.ts) invokes — stays byte-for-byte unchanged.
//
// Goal: a developer who logs into the web console after `go run ./cmd/seed -demo`
// sees a populated system (many companies, sites, agents, placements, schedules,
// attendance, leave, overtime, payslips, notifications) rather than the thin
// E2E-only rows.
//
// ----------------------------------------------------------------------------
// ID BANDS (DISJOINT from all seed.go literals — verified by grep)
// ----------------------------------------------------------------------------
// seed.go uses: CMP 00xx, SITE 00xx, SVC 00x, POS 01x, EMP 10xx/28xx/30xx/90xx,
// AG 70xx, PL 00001/50xx, SLA 30xx, SHF 00x, SCH 60xx, ATT 90xx, COR 80xx,
// LR 80xx/44210, LQ 80xx, OT 300xx, HOL 90xx, PS 901xx, NTF 900xx, USR 000xx.
//
// The demo uses a clearly-separate HIGH band so NOTHING collides:
//
//   client_companies          SWP-CMP-1001 .. SWP-CMP-1008
//   client_sites              SWP-SITE-2001 ..                 (~25 sites)
//   positions                 FREE-TEXT (positions master removed 2026-06-12;
//                             labels chosen per company's dominant service line)
//   employees (agents)        SWP-EMP-20001 .. SWP-EMP-20120
//   employment_agreements     SWP-AG-20001 .. SWP-AG-20120
//   placements                SWP-PL-20001 ..                  (active + terminal/history)
//   shift_leader_assignments  SWP-SLA-2001 .. SWP-SLA-2008
//   shift_masters             SWP-SHF-201 .. SWP-SHF-203
//   schedule_entries          SWP-SCH-2xxxxx                   (~30 days rotation)
//   attendance                SWP-ATT-2xxxxx
//   attendance_corrections    SWP-COR-2xxx
//   leave_requests            SWP-LR-21xxx
//   leave_quotas              SWP-LQ-2xxx
//   overtime                  SWP-OT-22xxx
//   holidays                  SWP-HOL-201 .. SWP-HOL-204
//   payslips                  SWP-PS-23xxx
//   payslip audit notes       SWP-PS-23xxx-NOTE-N
//   notifications             SWP-NTF-24xxx
//
// ----------------------------------------------------------------------------
// DETERMINISM + IDEMPOTENCY
// ----------------------------------------------------------------------------
//   - All randomness comes from a FIXED-SEED rand.Rand (seed 20260605) so every
//     re-run produces identical names/variations.
//   - Every INSERT uses ON CONFLICT (id) DO NOTHING (or a NOT EXISTS guard for
//     bigserial-PK child tables), mirroring seed.go, so re-runs are idempotent.
//   - A sentinel short-circuit: if SWP-CMP-1001 already exists, SeedDemo logs and
//     returns nil immediately (skips the whole profile).
//   - Constraints are HONORED, never disabled: INV-1 (<=1 active placement per
//     agent), EA-2 (<=1 active agreement per employee), INV-2 (<=1 active leader
//     per company), INV-4 (leader is placed at the company), schedule_entries
//     partial-unique (employee_id, work_date) (NEVER double-booked), all CHECK
//     enums, and every FK.
// =============================================================================

const demoSentinelCompany = "SWP-CMP-1001"

// demoRand is the fixed-seed source for all name/variation choices.
var demoRand = rand.New(rand.NewSource(20260605))

// demoNow is captured once so all relative dates within a run are coherent.
var demoNow = time.Now()

// SeedDemo inserts the production-like demo dataset. It assumes Seed() has
// already run (personas, global service lines, master data, the two E2E
// companies). Idempotent + deterministic. Safe to re-run.
func SeedDemo(ctx context.Context, pool *db.Pool) error {
	// Sentinel short-circuit: if the first demo company exists, assume the whole
	// profile is already seeded and skip (idempotent fast path).
	var exists bool
	if err := pool.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM client_companies WHERE id = $1)`, demoSentinelCompany,
	).Scan(&exists); err != nil {
		return fmt.Errorf("demo: check sentinel: %w", err)
	}
	if exists {
		slog.Info("demo: sentinel company present — demo profile already seeded, skipping", "id", demoSentinelCompany)
		return nil
	}

	slog.Info("demo: seeding production-like dataset (this is additive; default seed already ran)")

	// Strict dependency order: org → positions → employees+agreements →
	// placements(+leaders) → schedules → attendance(+corrections) →
	// leave(+quotas) → overtime(+holidays) → payroll → notifications/audit.
	d, err := buildDemoCompanies(ctx, pool)
	if err != nil {
		return fmt.Errorf("demo companies/sites: %w", err)
	}
	if err := seedDemoEmployees(ctx, pool, d); err != nil {
		return fmt.Errorf("demo employees/agreements: %w", err)
	}
	if err := seedDemoPlacements(ctx, pool, d); err != nil {
		return fmt.Errorf("demo placements: %w", err)
	}
	if err := seedDemoLeaders(ctx, pool, d); err != nil {
		return fmt.Errorf("demo leaders: %w", err)
	}
	if err := seedDemoSchedules(ctx, pool, d); err != nil {
		return fmt.Errorf("demo schedules: %w", err)
	}
	if err := seedDemoAttendance(ctx, pool, d); err != nil {
		return fmt.Errorf("demo attendance: %w", err)
	}
	if err := seedDemoLeave(ctx, pool, d); err != nil {
		return fmt.Errorf("demo leave: %w", err)
	}
	if err := seedDemoHolidays(ctx, pool); err != nil {
		return fmt.Errorf("demo holidays: %w", err)
	}
	if err := seedDemoOvertime(ctx, pool, d); err != nil {
		return fmt.Errorf("demo overtime: %w", err)
	}
	if err := seedDemoPayroll(ctx, pool, d); err != nil {
		return fmt.Errorf("demo payroll: %w", err)
	}
	if err := seedDemoNotifications(ctx, pool, d); err != nil {
		return fmt.Errorf("demo notifications: %w", err)
	}
	if err := seedDemoAuditLog(ctx, pool, d); err != nil {
		return fmt.Errorf("demo audit_log: %w", err)
	}

	slog.Info("demo: complete",
		"companies", len(d.companies), "sites", d.siteCount,
		"agents", len(d.agents), "placements", d.placementCount)
	return nil
}

// -----------------------------------------------------------------------------
// In-memory model carried between the demo generation steps.
// -----------------------------------------------------------------------------

type demoSite struct {
	id        string
	companyID string
	name      string
	address   string
	lat       float64
	lng       float64
	radius    int
	isPrimary bool
}

type demoCompany struct {
	id          string
	name        string
	address     string
	serviceLine string // dominant service line id (SWP-SVC-00x) — IN-MEMORY ONLY,
	// used to pick a free-text position label set (service_lines table removed 2026-06-12)
	slSlug string // dominant service line slug — IN-MEMORY ONLY (no longer stored)
	sites       []*demoSite
}

// demoAgent is a generated employee + its active placement linkage.
type demoAgent struct {
	empID     string
	agID      string
	name      string
	gender    string
	companyID string
	siteID    string
	svcID     string
	slSlug    string
	position  string // free-text position label (positions master removed 2026-06-12)
	plID      string // active placement id
	isLeader  bool
	terminal  bool // this agent's placement is terminal/history (not active)
}

type demoData struct {
	companies      []*demoCompany
	agents         []*demoAgent
	siteCount      int
	placementCount int
	// fast lookups
	companyByID map[string]*demoCompany
}

// -----------------------------------------------------------------------------
// Indonesian name / data pools (deterministic; chosen via demoRand).
// -----------------------------------------------------------------------------

var demoFirstMale = []string{
	"Budi", "Agus", "Andi", "Dedi", "Eko", "Fajar", "Gunawan", "Hadi", "Indra",
	"Joko", "Kurnia", "Lukman", "Made", "Nanda", "Oka", "Putra", "Rizki", "Surya",
	"Teguh", "Wahyu", "Yusuf", "Bayu", "Cahyo", "Doni", "Firman",
}
var demoFirstFemale = []string{
	"Sari", "Dewi", "Ayu", "Bunga", "Citra", "Fitri", "Gita", "Hana", "Indah",
	"Kartika", "Lestari", "Maya", "Nadia", "Oktavia", "Putri", "Rina", "Siti",
	"Tari", "Wulan", "Yuni", "Anisa", "Bella", "Cinta", "Dina", "Eka",
}
var demoLast = []string{
	"Santoso", "Wijaya", "Pratama", "Lestari", "Hartono", "Saputra", "Nugroho",
	"Hidayat", "Setiawan", "Kusuma", "Mahendra", "Ramadhan", "Suryadi", "Wibowo",
	"Permana", "Anggraini", "Maulana", "Firmansyah", "Halim", "Iskandar",
	"Gunawan", "Purnomo", "Cahyono", "Damar", "Yulianto",
}
var demoStreets = []string{
	"Jl. Melati", "Jl. Mawar", "Jl. Anggrek", "Jl. Kenanga", "Jl. Cempaka",
	"Jl. Flamboyan", "Jl. Bougenville", "Jl. Dahlia", "Jl. Teratai", "Jl. Seroja",
}
var demoKota = []string{
	"Jakarta Pusat", "Jakarta Selatan", "Jakarta Utara", "Jakarta Barat",
	"Jakarta Timur", "Tangerang", "Bekasi", "Depok",
}

func demoPick(s []string) string { return s[demoRand.Intn(len(s))] }

// demoNIK builds a plausible 16-digit Indonesian KTP number deterministically.
// The demo band uses a 32-77 prefix region distinct from the seed.go 3175 NIKs.
func demoNIK(seq int) string {
	return fmt.Sprintf("32%014d", 71000000000000+int64(seq))
}

// -----------------------------------------------------------------------------
// Step 1: companies + sites
// -----------------------------------------------------------------------------

// demoCompanySpec is the static catalog of 8 demo client companies. Each gets a
// dominant service line and 2-4 sites. Geofences are real Jakarta-area coords.
type demoCompanySpec struct {
	id, name, address, svcID, slSlug string
	siteNames                        []string
	baseLat, baseLng                 float64
}

var demoCompanySpecs = []demoCompanySpec{
	{"SWP-CMP-1001", "Grand Indonesia", "Jl. M.H. Thamrin No. 1, Jakarta Pusat 10310", "SWP-SVC-002", "building_management",
		[]string{"GI East Mall", "GI West Mall", "GI Tower", "GI Sky Bridge"}, -6.1952, 106.8205},
	{"SWP-CMP-1002", "Pacific Place", "Jl. Jend. Sudirman Kav. 52-53, Jakarta Selatan 12190", "SWP-SVC-002", "building_management",
		[]string{"PP Mall", "PP Office Tower", "PP Residences"}, -6.2244, 106.8094},
	{"SWP-CMP-1003", "Senayan City", "Jl. Asia Afrika Lot 19, Jakarta Pusat 10270", "SWP-SVC-003", "parking",
		[]string{"SC Basement P1", "SC Basement P2", "SC Rooftop"}, -6.2270, 106.7990},
	{"SWP-CMP-1004", "Kota Kasablanka", "Jl. Casablanca Raya Kav. 88, Jakarta Selatan 12870", "SWP-SVC-003", "parking",
		[]string{"Kokas Parking A", "Kokas Parking B"}, -6.2235, 106.8430},
	{"SWP-CMP-1005", "Central Park Mall", "Jl. Letjen S. Parman Kav. 28, Jakarta Barat 11470", "SWP-SVC-001", "facility_services",
		[]string{"CP Main", "CP Tribeca", "CP APL Tower", "CP Garden"}, -6.1770, 106.7900},
	{"SWP-CMP-1006", "Gandaria City", "Jl. Sultan Iskandar Muda, Jakarta Selatan 12240", "SWP-SVC-001", "facility_services",
		[]string{"GC Mall", "GC Office", "GC Sky Garden"}, -6.2440, 106.7835},
	{"SWP-CMP-1007", "Mall Taman Anggrek", "Jl. Letjen S. Parman Kav. 21, Jakarta Barat 11470", "SWP-SVC-001", "facility_services",
		[]string{"MTA Mall", "MTA Condominium"}, -6.1785, 106.7920},
	{"SWP-CMP-1008", "Lippo Mall Puri", "Jl. Puri Indah Raya Blok U1, Jakarta Barat 11610", "SWP-SVC-003", "parking",
		[]string{"LMP Parking North", "LMP Parking South", "LMP Valet"}, -6.1880, 106.7390},
}

func buildDemoCompanies(ctx context.Context, pool *db.Pool) (*demoData, error) {
	d := &demoData{companyByID: map[string]*demoCompany{}}

	const companyQ = `
		INSERT INTO client_companies (id, name, address, leader_scope, pic_name, phone, email, status)
		VALUES ($1, $2, $3, 'company', $4, $5, $6, 'active')
		ON CONFLICT (id) DO NOTHING`

	const siteQ = `
		INSERT INTO client_sites
			(id, client_company_id, name, code, address, geo_lat, geo_lng, geofence_radius_m, is_primary, pic_name, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'active')
		ON CONFLICT (id) DO NOTHING`

	siteSeq := 2001
	for _, spec := range demoCompanySpecs {
		pic := demoPick(demoFirstMale) + " " + demoPick(demoLast)
		phone := fmt.Sprintf("+62-21-%07d", 5000000+demoRand.Intn(900000))
		email := "pic@" + sanitizeSlug(spec.name) + ".co.id"
		if _, err := pool.Pool.Exec(ctx, companyQ, spec.id, spec.name, spec.address, pic, phone, email); err != nil {
			return nil, fmt.Errorf("seed demo company %q: %w", spec.id, err)
		}

		c := &demoCompany{
			id: spec.id, name: spec.name, address: spec.address,
			serviceLine: spec.svcID, slSlug: spec.slSlug,
		}

		for i, sn := range spec.siteNames {
			// Small deterministic geo jitter around the company base coords.
			latJit := (demoRand.Float64() - 0.5) * 0.004
			lngJit := (demoRand.Float64() - 0.5) * 0.004
			site := &demoSite{
				id:        fmt.Sprintf("SWP-SITE-%04d", siteSeq),
				companyID: spec.id,
				name:      sn,
				address:   spec.address,
				lat:       spec.baseLat + latJit,
				lng:       spec.baseLng + lngJit,
				radius:    75 + demoRand.Intn(126), // 75..200m
				isPrimary: i == 0,
			}
			code := fmt.Sprintf("S%02d", i+1)
			sitePIC := demoPick(demoFirstMale) + " " + demoPick(demoLast)
			if _, err := pool.Pool.Exec(ctx, siteQ,
				site.id, site.companyID, site.name, code, site.address,
				site.lat, site.lng, site.radius, site.isPrimary, sitePIC,
			); err != nil {
				return nil, fmt.Errorf("seed demo site %q: %w", site.id, err)
			}
			c.sites = append(c.sites, site)
			siteSeq++
			d.siteCount++
		}

		d.companies = append(d.companies, c)
		d.companyByID[c.id] = c
	}

	slog.Info("demo: seeded companies + sites", "companies", len(d.companies), "sites", d.siteCount)
	return d, nil
}

// sanitizeSlug lowercases + strips spaces/punctuation for a fake email domain.
func sanitizeSlug(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+32)
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "demo"
	}
	return string(out)
}

// -----------------------------------------------------------------------------
// Step 2: position labels (FREE-TEXT — the positions master was removed
// 2026-06-12). Positions are now plain text on placements + attendance, so there
// is nothing to seed: demoPositionFor just returns deterministic free-text labels
// per dominant service line of the company.
// -----------------------------------------------------------------------------

// demoPositionFor returns the attendant + supervisor free-text position labels for
// a company's dominant service line slug (svcID retained only to pick a label set).
func demoPositionFor(svcID string) (attendant, supervisor string) {
	switch svcID {
	case "SWP-SVC-001":
		return "Petugas Kebersihan", "Pengawas Kebersihan"
	case "SWP-SVC-002":
		return "Teknisi Gedung", "Koordinator Gedung"
	default: // parking
		return "Petugas Parkir", "Koordinator Lokasi"
	}
}

// -----------------------------------------------------------------------------
// Step 3: employees + employment agreements + a few change requests
// -----------------------------------------------------------------------------

const demoAgentCount = 120

func seedDemoEmployees(ctx context.Context, pool *db.Pool, d *demoData) error {
	const empQ = `
		INSERT INTO employees
			(id, full_name, nik, nip, join_at, gender, birth_date, phone, address,
			 bank_name, bank_account_number, bank_account_holder_name, status)
		VALUES ($1, $2, $3, $4, $5::date, $6, $7::date, $8, $9, $10, $11, $12, 'active')
		ON CONFLICT (id) DO NOTHING`

	const agQ = `
		INSERT INTO employment_agreements
			(id, employee_id, type, agreement_no, start_date, end_date, status,
			 base_salary_idr, bpjs_terms, tax_profile, comp_effective_date, created_by)
		VALUES
			($1, $2, $3, $4, $5::date, $6, 'active',
			 $7,
			 '{"kesehatan_employer_pct":4.0,"kesehatan_employee_pct":1.0,"ketenagakerjaan_employer_pct":6.24,"ketenagakerjaan_employee_pct":3.0}'::jsonb,
			 $8, $5::date, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	banks := []string{"BCA", "BNI", "BRI", "Mandiri", "CIMB Niaga", "Permata"}
	taxProfiles := []string{"PTKP_TK0", "PTKP_K0", "PTKP_K1", "PTKP_K2"}

	// Distribute 120 agents across the 8 companies round-robin so each company
	// has roughly 15 agents (enough for a 24/7 rotation + a leader).
	companies := d.companies
	for i := 0; i < demoAgentCount; i++ {
		seq := 20001 + i
		empID := fmt.Sprintf("SWP-EMP-%05d", seq)
		agID := fmt.Sprintf("SWP-AG-%05d", seq)

		// Deterministic gender + name.
		female := demoRand.Intn(2) == 0
		var first, gender string
		if female {
			first = demoPick(demoFirstFemale)
			gender = "FEMALE"
		} else {
			first = demoPick(demoFirstMale)
			gender = "MALE"
		}
		name := first + " " + demoPick(demoLast)

		c := companies[i%len(companies)]
		site := c.sites[demoRand.Intn(len(c.sites))]
		attendant, supervisor := demoPositionFor(c.serviceLine)
		// ~1 in 8 agents is a supervisor-grade (and a leader candidate).
		pos := attendant
		isLeaderCandidate := false
		if i%len(companies) == 0 { // first agent assigned to each company is the leader-grade
			pos = supervisor
			isLeaderCandidate = true
		}

		// Join date 1-5 years ago.
		yearsAgo := 1 + demoRand.Intn(5)
		join := demoNow.AddDate(-yearsAgo, -demoRand.Intn(12), -demoRand.Intn(28))
		joinStr := join.Format("2006-01-02")

		// Birth date 22-50 years ago.
		birth := demoNow.AddDate(-(22 + demoRand.Intn(28)), -demoRand.Intn(12), -demoRand.Intn(28)).Format("2006-01-02")

		phone := fmt.Sprintf("+62-81%d-%04d-%04d", 1+demoRand.Intn(8), demoRand.Intn(10000), demoRand.Intn(10000))
		addr := fmt.Sprintf("%s No. %d, %s", demoPick(demoStreets), 1+demoRand.Intn(150), demoPick(demoKota))
		bank := demoPick(banks)
		acct := fmt.Sprintf("%010d", demoRand.Int63n(9000000000)+1000000000)

		if _, err := pool.Pool.Exec(ctx, empQ,
			empID, name, demoNIK(seq), fmt.Sprintf("%d", seq), joinStr, gender, birth,
			phone, addr, bank, acct, name,
		); err != nil {
			return fmt.Errorf("seed demo employee %q: %w", empID, err)
		}

		// Agreement: mix PKWT (fixed-term) and PKWTT (indefinite). ~35% PKWTT.
		isPKWTT := demoRand.Intn(100) < 35
		var agType, agNo string
		var endDate any
		var compStart string = join.Format("2006-01-02")
		_ = compStart
		if isPKWTT {
			agType = "PKWTT"
			agNo = fmt.Sprintf("PKWTT/SWP/2026/%05d", seq)
			endDate = nil
		} else {
			agType = "PKWT"
			agNo = fmt.Sprintf("PKWT/SWP/2026/%05d", seq)
			// End date 6-18 months out from now (a few will be inside 30 days via placements).
			endDate = demoNow.AddDate(0, 6+demoRand.Intn(13), 0).Format("2006-01-02")
		}
		baseSalary := 4500000 + demoRand.Intn(4000001) // 4.5M..8.5M
		tax := demoPick(taxProfiles)
		// Agreement start = max(join, 2026-01-01) so it is a current active agreement.
		agStart := join
		if agStart.Before(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
			agStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		if _, err := pool.Pool.Exec(ctx, agQ,
			agID, empID, agType, agNo, agStart.Format("2006-01-02"), endDate, baseSalary, tax,
		); err != nil {
			return fmt.Errorf("seed demo agreement %q: %w", agID, err)
		}

		d.agents = append(d.agents, &demoAgent{
			empID: empID, agID: agID, name: name, gender: gender,
			companyID: c.id, siteID: site.id, svcID: c.serviceLine, slSlug: c.slSlug,
			position: pos, isLeader: isLeaderCandidate,
		})
	}

	slog.Info("demo: seeded employees + agreements", "count", len(d.agents))

	// The change_requests table was dropped (E11, migration 00061); the demo no
	// longer seeds a HR change-request queue.
	return nil
}

// -----------------------------------------------------------------------------
// Step 4: placements (exactly ONE active per agent; some terminal/history)
// -----------------------------------------------------------------------------

func seedDemoPlacements(ctx context.Context, pool *db.Pool, d *demoData) error {
	const plQ = `
		INSERT INTO placements
			(id, employee_id, agreement_id, client_company_id, site_id,
			 position, start_date, end_date, lifecycle_status,
			 status_changed_at, ended_reason, ended_at, resign_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date, $8, $9, now(), $10, $11, $12, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	const histQ = `
		INSERT INTO placement_history (placement_id, action, status_after, effective_date, notes)
		VALUES ($1, $2, $3, $4::date, $5)
		ON CONFLICT DO NOTHING`

	plSeq := 20001
	for i, a := range d.agents {
		plID := fmt.Sprintf("SWP-PL-%05d", plSeq)
		plSeq++

		// Start 3-24 months ago.
		monthsAgo := 3 + demoRand.Intn(22)
		start := demoNow.AddDate(0, -monthsAgo, -demoRand.Intn(28))
		startStr := start.Format("2006-01-02")

		// Decide lifecycle. ~12% of agents get a TERMINAL/history placement
		// instead of active; the rest are ACTIVE (so INV-1 holds: exactly one
		// active per agent — each agent has exactly one placement row here).
		// A handful of actives end within 30 days (EXPIRING-derivable window).
		var lifecycle string
		var endStr any
		var endedReason, endedAt, resignAt any

		terminalRoll := demoRand.Intn(100)
		switch {
		case terminalRoll < 4: // ENDED (end_of_term)
			lifecycle = "ENDED"
			past := demoNow.AddDate(0, 0, -(10 + demoRand.Intn(60)))
			endStr = past.Format("2006-01-02")
			endedReason = "END_OF_TERM"
			endedAt = past.Format("2006-01-02")
			a.terminal = true
		case terminalRoll < 7: // TERMINATED
			lifecycle = "TERMINATED"
			past := demoNow.AddDate(0, 0, -(10 + demoRand.Intn(90)))
			endStr = past.Format("2006-01-02")
			endedReason = "TERMINATED"
			endedAt = past.Format("2006-01-02")
			a.terminal = true
		case terminalRoll < 9: // RESIGNED
			lifecycle = "RESIGNED"
			past := demoNow.AddDate(0, 0, -(10 + demoRand.Intn(120)))
			endStr = past.Format("2006-01-02")
			endedReason = "RESIGNED"
			resignAt = past.Format("2006-01-02")
			a.terminal = true
		case terminalRoll < 12: // TRANSFERRED
			lifecycle = "TRANSFERRED"
			past := demoNow.AddDate(0, 0, -(10 + demoRand.Intn(60)))
			endStr = past.Format("2006-01-02")
			endedReason = "TRANSFERRED"
			endedAt = past.Format("2006-01-02")
			a.terminal = true
		default:
			lifecycle = "ACTIVE"
			// ~1 in 6 actives expire within 30 days; rest end 4-14 months out
			// or are open-ended (PKWTT-style).
			switch r := demoRand.Intn(6); {
			case r == 0: // expiring soon
				endStr = demoNow.AddDate(0, 0, 5+demoRand.Intn(25)).Format("2006-01-02")
			case r == 1: // open-ended
				endStr = nil
			default:
				endStr = demoNow.AddDate(0, 4+demoRand.Intn(11), 0).Format("2006-01-02")
			}
		}

		if _, err := pool.Pool.Exec(ctx, plQ,
			plID, a.empID, a.agID, a.companyID, a.siteID, a.position,
			startStr, endStr, lifecycle, endedReason, endedAt, resignAt,
		); err != nil {
			return fmt.Errorf("seed demo placement %q: %w", plID, err)
		}
		a.plID = plID
		d.placementCount++

		// History: create row always; a terminal row when applicable.
		if _, err := pool.Pool.Exec(ctx, histQ, plID, "create", "ACTIVE", startStr, "Penempatan dibuat (demo)."); err != nil {
			return fmt.Errorf("seed demo placement_history(create) for %q: %w", plID, err)
		}
		if a.terminal {
			action := map[string]string{"ENDED": "end", "TERMINATED": "terminate", "RESIGNED": "resign", "TRANSFERRED": "transfer"}[lifecycle]
			eff := startStr
			if endedAt != nil {
				eff = endedAt.(string)
			} else if resignAt != nil {
				eff = resignAt.(string)
			}
			if _, err := pool.Pool.Exec(ctx, histQ, plID, action, lifecycle, eff, "Status terminal (demo)."); err != nil {
				return fmt.Errorf("seed demo placement_history(%s) for %q: %w", action, plID, err)
			}
		}
		_ = i
	}
	slog.Info("demo: seeded placements + history", "count", d.placementCount)
	return nil
}

// -----------------------------------------------------------------------------
// Step 5: shift-leader assignments (exactly one active leader per company; the
// leader must be actively placed at that company → INV-2 + INV-4).
// -----------------------------------------------------------------------------

func seedDemoLeaders(ctx context.Context, pool *db.Pool, d *demoData) error {
	const slaQ = `
		INSERT INTO shift_leader_assignments
			(id, client_company_id, site_id, employee_id, assigned_by, notes)
		VALUES ($1, $2, NULL, $3, 'system-seed', $4)
		ON CONFLICT (id) DO NOTHING`

	slaSeq := 2001
	count := 0
	for _, c := range d.companies {
		// Find an ACTIVE leader-grade agent placed at this company.
		var leader *demoAgent
		for _, a := range d.agents {
			if a.companyID == c.id && a.isLeader && !a.terminal {
				leader = a
				break
			}
		}
		// Fallback: any ACTIVE agent at this company (still satisfies INV-4).
		if leader == nil {
			for _, a := range d.agents {
				if a.companyID == c.id && !a.terminal {
					leader = a
					break
				}
			}
		}
		if leader == nil {
			slog.Warn("demo: no active agent at company for leader — skipping", "company", c.id)
			continue
		}
		slaID := fmt.Sprintf("SWP-SLA-%04d", slaSeq)
		slaSeq++
		notes := "Shift leader (demo) di " + c.name
		if _, err := pool.Pool.Exec(ctx, slaQ, slaID, c.id, leader.empID, notes); err != nil {
			return fmt.Errorf("seed demo leader %q: %w", slaID, err)
		}
		count++
	}
	slog.Info("demo: seeded shift_leader_assignments", "count", count)
	return nil
}

// -----------------------------------------------------------------------------
// Step 6: schedule entries (~30 days of 24/7 rotation). NEVER double-book an
// agent on a date (the partial unique index on (employee_id, work_date)).
// -----------------------------------------------------------------------------

func seedDemoSchedules(ctx context.Context, pool *db.Pool, d *demoData) error {
	// Three demo shift masters: Pagi / Sore / Malam (24/7 coverage).
	const shfQ = `
		INSERT INTO shift_masters
			(id, name, start_time, end_time, break_start, break_end, cross_midnight, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true, 'system-seed')
		ON CONFLICT (id) DO NOTHING`
	bs, be := "12:00", "13:00"
	masters := []struct {
		id, name, start, end string
		bsv, bev             *string
		cross                bool
	}{
		{"SWP-SHF-201", "Demo Pagi", "07:00", "15:00", &bs, &be, false},
		{"SWP-SHF-202", "Demo Sore", "15:00", "23:00", nil, nil, false},
		{"SWP-SHF-203", "Demo Malam", "23:00", "07:00", nil, nil, true},
	}
	for _, m := range masters {
		if _, err := pool.Pool.Exec(ctx, shfQ, m.id, m.name, m.start, m.end, m.bsv, m.bev, m.cross); err != nil {
			return fmt.Errorf("seed demo shift_master %q: %w", m.id, err)
		}
	}

	type shiftDef struct {
		masterID, start, end string
		cross                bool
	}
	shifts := []shiftDef{
		{"SWP-SHF-201", "07:00", "15:00", false},
		{"SWP-SHF-202", "15:00", "23:00", false},
		{"SWP-SHF-203", "23:00", "07:00", true},
	}

	const schQ = `
		INSERT INTO schedule_entries
			(id, employee_id, placement_id, shift_master_id,
			 start_time, end_time, cross_midnight, work_date, status, is_day_off, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::date, 'SCHEDULED', $9, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	// Only ACTIVE-placement agents are scheduled.
	active := make([]*demoAgent, 0, len(d.agents))
	for _, a := range d.agents {
		if !a.terminal {
			active = append(active, a)
		}
	}

	// 30 days centered on today: 21 days back .. 8 days forward.
	startDay := dayFloor(demoNow.AddDate(0, 0, -21))
	schSeq := 200001
	count := 0
	for dayOff := 0; dayOff < 30; dayOff++ {
		work := startDay.AddDate(0, 0, dayOff)
		workStr := work.Format("2006-01-02")
		for ai, a := range active {
			// Deterministic rotation: assign a shift index by (agentIndex+day)%3.
			// ~1 in 6 cells is a day-off (no shift master).
			rotation := (ai + dayOff) % 3
			isDayOff := ((ai*7 + dayOff*3) % 6) == 0

			schID := fmt.Sprintf("SWP-SCH-%06d", schSeq)
			schSeq++
			if isDayOff {
				if _, err := pool.Pool.Exec(ctx, schQ,
					schID, a.empID, a.plID, nil, nil, nil, false, workStr, true,
				); err != nil {
					return fmt.Errorf("seed demo schedule(day-off) %q: %w", schID, err)
				}
			} else {
				s := shifts[rotation]
				if _, err := pool.Pool.Exec(ctx, schQ,
					schID, a.empID, a.plID, s.masterID, s.start, s.end, s.cross, workStr, false,
				); err != nil {
					return fmt.Errorf("seed demo schedule %q: %w", schID, err)
				}
			}
			count++
		}
	}
	slog.Info("demo: seeded schedule_entries", "count", count, "days", 30, "agents", len(active))
	return nil
}

// dayFloor returns midnight UTC of the given day.
func dayFloor(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// -----------------------------------------------------------------------------
// Step 7: attendance derived from PAST schedule entries (+ a few corrections).
// -----------------------------------------------------------------------------

func seedDemoAttendance(ctx context.Context, pool *db.Pool, d *demoData) error {
	const attQ = `
		INSERT INTO attendance
			(id, employee_id, placement_id, schedule_id, company_id,
			 site_id, position, attendance_code_id,
			 shift_start_at, shift_end_at, check_in_at, check_out_at,
			 lat_in, lng_in, lat_out, lng_out, wfo,
			 is_late, late_minutes, worked_minutes, auto_closed,
			 in_geofence, in_distance_m, out_geofence, out_distance_m, geofence_radius_m,
			 status, verification_status, flags)
		VALUES
			($1, $2, $3, NULL, $4,
			 (SELECT site_id FROM placements WHERE id = $3),
			 (SELECT position FROM placements WHERE id = $3),
			 $5,
			 $6, $7, $8, $9,
			 $10, $11, $12, $13, true,
			 $14, $15, $16, $17,
			 $18, $19, $20, $21, $22,
			 $23, $24, $25)
		ON CONFLICT (id) DO NOTHING`

	const corQ = `
		INSERT INTO attendance_corrections
			(id, attendance_id, requester_id, company_id, type,
			 proposed_check_in_at, proposed_check_out_at, proposed_attendance_code_id,
			 reason, status, original_snapshot, attendance_shift_date)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, NULL, $8, $9, $10::jsonb, $11::date)
		ON CONFLICT (id) DO NOTHING`

	siteByID := map[string]*demoSite{}
	for _, c := range d.companies {
		for _, s := range c.sites {
			siteByID[s.id] = s
		}
	}

	active := make([]*demoAgent, 0, len(d.agents))
	for _, a := range d.agents {
		if !a.terminal {
			active = append(active, a)
		}
	}

	attSeq := 200001
	corSeq := 2001
	count := 0
	corrections := 0
	// Generate attendance for past days only (yesterday back 14 days), one
	// Pagi shift per agent per day, ~ matching the rotation. Keep volume sane.
	for dayOff := 1; dayOff <= 14; dayOff++ {
		work := dayFloor(demoNow.AddDate(0, 0, -dayOff))
		// 07:00 WIB = 00:00 UTC anchor for shift start.
		shiftStart := work
		shiftEnd := shiftStart.Add(8 * time.Hour)
		for ai, a := range active {
			// Schedule only ~ every agent on weekdays; skip ~1/6 (day-off parity).
			if ((ai*7 + dayOff*3) % 6) == 0 {
				continue
			}
			site := siteByID[a.siteID]
			lat, lng, radius := -6.2256, 106.7997, 100
			if site != nil {
				lat, lng, radius = site.lat, site.lng, site.radius
			}

			attID := fmt.Sprintf("SWP-ATT-%06d", attSeq)
			attSeq++

			// Roll a record outcome.
			roll := demoRand.Intn(100)
			var status, vstatus, flags string
			var isLate bool
			var lateMin int
			var autoClosed bool
			var checkOut any = shiftEnd.Format(time.RFC3339)
			var latOut any = lat
			var lngOut any = lng
			var workedMin any = 480
			inGeo, outGeo := true, true
			inDist, outDist := 20+demoRand.Intn(40), 20+demoRand.Intn(40)
			checkIn := shiftStart

			switch {
			case roll < 70: // clean AUTO_APPROVED
				status, vstatus, flags = "PRESENT", "AUTO_APPROVED", "{}"
			case roll < 82: // late → PENDING
				isLate = true
				lateMin = 5 + demoRand.Intn(40)
				checkIn = shiftStart.Add(time.Duration(lateMin) * time.Minute)
				status, vstatus, flags = "LATE", "PENDING", "{LATE}"
			case roll < 90: // outside geofence → PENDING
				inGeo = false
				inDist = radius + 50 + demoRand.Intn(300)
				status, vstatus, flags = "PRESENT", "PENDING", "{OUTSIDE_GEOFENCE}"
			case roll < 96: // auto-closed (no clock-out) → PENDING
				autoClosed = true
				checkOut = nil
				latOut = nil
				lngOut = nil
				workedMin = nil
				outGeo = false
				outDist = 0
				status, vstatus, flags = "INCOMPLETE", "PENDING", "{AUTO_CLOSED}"
			default: // VERIFIED clean (billable)
				status, vstatus, flags = "PRESENT", "VERIFIED", "{VERIFIED}"
			}

			// attendance_code: bind clean/verified to PRESENT (billable), late to LATE.
			var codeID any
			switch {
			case flags == "{LATE}":
				codeID = "SWP-AC-002"
			case vstatus == "AUTO_APPROVED" || vstatus == "VERIFIED":
				codeID = "SWP-AC-001"
			default:
				codeID = nil
			}

			var inGeoV, outGeoV any = inGeo, outGeo
			if autoClosed {
				outGeoV = nil
			}

			if _, err := pool.Pool.Exec(ctx, attQ,
				attID, a.empID, a.plID, a.companyID, codeID,
				shiftStart.Format(time.RFC3339), shiftEnd.Format(time.RFC3339),
				checkIn.Format(time.RFC3339), checkOut,
				lat, lng, latOut, lngOut,
				isLate, lateMin, workedMin, autoClosed,
				inGeoV, inDist, outGeoV, outDist, radius,
				status, vstatus, flags,
			); err != nil {
				return fmt.Errorf("seed demo attendance %q: %w", attID, err)
			}
			count++

			// ~1 in 40 PENDING auto-closed records gets a PENDING correction.
			if autoClosed && demoRand.Intn(3) == 0 {
				corID := fmt.Sprintf("SWP-COR-%04d", corSeq)
				corSeq++
				proposedOut := shiftEnd.Add(10 * time.Minute).Format(time.RFC3339)
				snap := `{"check_out_at": null, "auto_closed": true, "status": "INCOMPLETE"}`
				if _, err := pool.Pool.Exec(ctx, corQ,
					corID, attID, a.empID, a.companyID, "CHECK_OUT",
					nil, proposedOut, "Lupa clock-out (demo).", "PENDING", snap,
					work.Format("2006-01-02"),
				); err != nil {
					return fmt.Errorf("seed demo correction %q: %w", corID, err)
				}
				corrections++
			}
		}
	}
	slog.Info("demo: seeded attendance + corrections", "attendance", count, "corrections", corrections)
	return nil
}

// -----------------------------------------------------------------------------
// Step 8: leave quotas + leave requests (across states) + approvals.
// -----------------------------------------------------------------------------

func seedDemoLeave(ctx context.Context, pool *db.Pool, d *demoData) error {
	year := demoNow.Year()
	periodStart := fmt.Sprintf("%d-01-01", year)
	periodEnd := fmt.Sprintf("%d-12-31", year)

	const lqQ = `
		INSERT INTO leave_quotas
			(id, employee_id, leave_type_id, period, period_start, period_end, total, used, pending)
		VALUES ($1, $2, 'SWP-LT-001', $3, $4::date, $5::date, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING`

	const lrQ = `
		INSERT INTO leave_requests
			(id, employee_id, placement_id, company_id, leave_type_id,
			 start_date, end_date, duration_days, reason, status, no_leader, assigned_leader_id, created_by)
		VALUES ($1, $2, $3, $4, 'SWP-LT-001', $5::date, $6::date, $7, $8, $9, $10, $11, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	// Leader lookup per company (for assigned_leader_id on PENDING rows).
	leaderByCompany := map[string]string{}
	for _, c := range d.companies {
		for _, a := range d.agents {
			if a.companyID == c.id && a.isLeader && !a.terminal {
				leaderByCompany[c.id] = a.empID
				break
			}
		}
	}

	active := make([]*demoAgent, 0, len(d.agents))
	for _, a := range d.agents {
		if !a.terminal {
			active = append(active, a)
		}
	}

	// Quotas for the first ~60 active agents.
	lqSeq := 2001
	quotaCount := 0
	for i, a := range active {
		if i >= 60 {
			break
		}
		total := 12
		used := demoRand.Intn(9)    // 0..8
		pending := demoRand.Intn(3) // 0..2
		lqID := fmt.Sprintf("SWP-LQ-%04d", lqSeq)
		lqSeq++
		if _, err := pool.Pool.Exec(ctx, lqQ, lqID, a.empID, year, periodStart, periodEnd, total, used, pending); err != nil {
			return fmt.Errorf("seed demo leave_quota %q: %w", lqID, err)
		}
		quotaCount++
	}

	// ~40 leave requests across states. E11 collapsed the two pending levels into a
	// single engine-driven PENDING (the line being decided lives on the approval
	// instance, not the status), so all pending rows are just PENDING.
	states := []string{"PENDING", "PENDING", "PENDING", "PENDING", "APPROVED", "APPROVED", "REJECTED"}
	reasons := []string{
		"Keperluan keluarga.", "Kontrol kesehatan.", "Acara pernikahan.",
		"Mudik ke kampung halaman.", "Urusan pribadi.", "Anak sakit.",
	}
	lrSeq := 21001
	reqCount := 0
	for i := 0; i < 40 && i < len(active); i++ {
		a := active[i*2%len(active)]
		state := states[i%len(states)]
		lrID := fmt.Sprintf("SWP-LR-%05d", lrSeq)
		lrSeq++

		// Date range: a few days, anchored relative to now per state.
		var base time.Time
		switch state {
		case "APPROVED", "REJECTED":
			base = demoNow.AddDate(0, 0, -(5 + demoRand.Intn(40))) // past
		default:
			base = demoNow.AddDate(0, 0, 2+demoRand.Intn(20)) // future
		}
		days := 1 + demoRand.Intn(3)
		start := dayFloor(base)
		end := start.AddDate(0, 0, days-1)

		leader := leaderByCompany[a.companyID]
		var assignedLeader any
		noLeader := false
		if leader != "" && leader != a.empID {
			assignedLeader = leader
		} else {
			noLeader = true
			assignedLeader = nil
		}

		if _, err := pool.Pool.Exec(ctx, lrQ,
			lrID, a.empID, a.plID, a.companyID,
			start.Format("2006-01-02"), end.Format("2006-01-02"), days,
			demoPick(reasons), state, noLeader, assignedLeader,
		); err != nil {
			return fmt.Errorf("seed demo leave_request %q: %w", lrID, err)
		}
		reqCount++

		// The leave decision trail now lives in the E11 approval engine
		// (approval_instances + approval_actions); the legacy leave_approvals table
		// was dropped (migration 00061), so no per-request decision rows are seeded
		// here. The demo terminal states (APPROVED/REJECTED) are carried by the
		// leave_requests.status alone.
		_ = leader
	}
	slog.Info("demo: seeded leave quotas + requests", "quotas", quotaCount, "requests", reqCount)
	return nil
}

// -----------------------------------------------------------------------------
// Step 9: holidays + overtime (across states + tiers).
// -----------------------------------------------------------------------------

func seedDemoHolidays(ctx context.Context, pool *db.Pool) error {
	const hQ = `
		INSERT INTO holidays (id, name, holiday_date, category, recurring)
		VALUES ($1, $2, $3::date, $4, $5)
		ON CONFLICT (id) DO NOTHING`
	year := demoNow.Year()
	holidays := []struct {
		id, name, date, cat string
		recurring           bool
	}{
		{"SWP-HOL-201", "Tahun Baru (Demo)", fmt.Sprintf("%d-01-01", year), "NATIONAL", true},
		{"SWP-HOL-202", "Hari Buruh Internasional (Demo)", fmt.Sprintf("%d-05-01", year), "NATIONAL", true},
		{"SWP-HOL-203", "Hari Kemerdekaan RI (Demo)", fmt.Sprintf("%d-08-17", year), "NATIONAL", true},
		{"SWP-HOL-204", "Cuti Bersama (Demo)", demoNow.AddDate(0, 0, -7).Format("2006-01-02"), "CUSTOM", false},
	}
	for _, h := range holidays {
		if _, err := pool.Pool.Exec(ctx, hQ, h.id, h.name, h.date, h.cat, h.recurring); err != nil {
			return fmt.Errorf("seed demo holiday %q: %w", h.id, err)
		}
	}
	slog.Info("demo: seeded holidays", "count", len(holidays))
	return nil
}

func seedDemoOvertime(ctx context.Context, pool *db.Pool, d *demoData) error {
	const otQ = `
		INSERT INTO overtime
			(id, employee_id, company_id, placement_id, attendance_id,
			 work_date, planned_start_time, planned_end_time, actual_start_time, actual_end_time,
			 cross_midnight, source, status, day_type, worked_minutes, counted_minutes,
			 min_minutes_threshold, skipped_too_short, reference_multiplier, overtime_rule_id,
			 holiday_id, flagged_no_preapproval, reason, created_by)
		VALUES ($1, $2, $3, $4, NULL,
			$5::date, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15,
			30, $16, $17, 'SWP-OTR-001',
			$18, $19, $20, 'system-seed')
		ON CONFLICT (id) DO NOTHING`

	leaderByCompany := map[string]string{}
	for _, c := range d.companies {
		for _, a := range d.agents {
			if a.companyID == c.id && a.isLeader && !a.terminal {
				leaderByCompany[c.id] = a.empID
				break
			}
		}
	}

	active := make([]*demoAgent, 0, len(d.agents))
	for _, a := range d.agents {
		if !a.terminal {
			active = append(active, a)
		}
	}

	type otState struct {
		status, source string
	}
	// E11 collapsed the OT two-level approval levels into a single engine-driven
	// PENDING; WITHDRAWN folded into CANCELLED. PENDING_AGENT_CONFIRM stays the
	// pre-chain candidate state.
	stateMix := []otState{
		{"PENDING_AGENT_CONFIRM", "AUTO_DETECTED"},
		{"PENDING", "REQUESTED"},
		{"PENDING", "REQUESTED"},
		{"PENDING", "REQUESTED"},
		{"APPROVED", "REQUESTED"},
		{"APPROVED", "WORKED_WITHOUT_REQUEST"},
		{"REJECTED", "REQUESTED"},
		{"CANCELLED", "REQUESTED"},
	}
	tiers := []struct {
		dayType    string
		multiplier float64
	}{
		{"WORKDAY", 1.5},
		{"WORKDAY", 1.5},
		{"RESTDAY", 2.0},
		{"HOLIDAY", 3.0},
	}

	otSeq := 22001
	count := 0
	for i := 0; i < 50 && i < len(active); i++ {
		a := active[(i*3)%len(active)]
		st := stateMix[i%len(stateMix)]
		tier := tiers[i%len(tiers)]
		otID := fmt.Sprintf("SWP-OT-%05d", otSeq)
		otSeq++

		work := dayFloor(demoNow.AddDate(0, 0, -(1 + demoRand.Intn(20))))
		workStr := work.Format("2006-01-02")
		worked := 60 + demoRand.Intn(180) // 60..240
		counted := (worked / 30) * 30
		skipped := counted < 30
		flagged := st.source == "WORKED_WITHOUT_REQUEST"

		var holidayID any
		if tier.dayType == "HOLIDAY" {
			holidayID = "SWP-HOL-204" // the recent custom holiday
		}

		actualStart := "17:00"
		actualEnd := fmt.Sprintf("%02d:%02d", 17+worked/60, worked%60)

		if _, err := pool.Pool.Exec(ctx, otQ,
			otID, a.empID, a.companyID, a.plID,
			workStr, "17:00", "20:00", actualStart, actualEnd,
			false, st.source, st.status, tier.dayType, worked, counted,
			skipped, tier.multiplier, holidayID, flagged, "Lembur operasional (demo).",
		); err != nil {
			return fmt.Errorf("seed demo overtime %q: %w", otID, err)
		}
		count++

		// The OT decision trail now lives in the E11 approval engine
		// (approval_instances + approval_actions); the legacy overtime_approvals
		// table was dropped (migration 00061), so no per-OT decision rows are seeded
		// here. Terminal states are carried by overtime.status alone.
		_ = leaderByCompany[a.companyID]
	}
	slog.Info("demo: seeded overtime", "count", count)
	return nil
}

// -----------------------------------------------------------------------------
// Step 10: payroll — ~2 months of FINAL payslips for a subset of agents, money
// encrypted under PAYROLL_ENCRYPTION_KEY + components + benefits + audit notes.
// -----------------------------------------------------------------------------

func seedDemoPayroll(ctx context.Context, pool *db.Pool, d *demoData) error {
	keyB64 := os.Getenv("PAYROLL_ENCRYPTION_KEY")
	if keyB64 == "" {
		slog.Warn("demo: PAYROLL_ENCRYPTION_KEY unset — skipping demo payroll (archive will be empty for demo agents)")
		return nil
	}
	cipher, err := crypto.NewFromBase64(keyB64)
	if err != nil {
		return fmt.Errorf("demo payroll: build cipher: %w", err)
	}
	enc := func(money string) ([]byte, error) { return cipher.Encrypt(money) }

	const psQ = `
		INSERT INTO payslips
			(id, employee_id, employee_name, placement_id, year, month, paid_on,
			 working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
			 status, source_system, source_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date, $8, $9, $10, $11, 'FINAL', 'lumen_swp', $12)
		ON CONFLICT (id) DO NOTHING
		RETURNING id`

	const compQ = `
		INSERT INTO payslip_components (payslip_id, kind, name, value_enc, for_bpjs, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)`
	const benQ = `
		INSERT INTO payslip_benefits (payslip_id, name, value_enc, sort_order)
		VALUES ($1, $2, $3, $4)`
	const noteQ = `
		INSERT INTO payslip_audit_notes (id, payslip_id, seq, text, author_id, author_name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::timestamptz)
		ON CONFLICT (id) DO NOTHING`

	active := make([]*demoAgent, 0, len(d.agents))
	for _, a := range d.agents {
		if !a.terminal {
			active = append(active, a)
		}
	}

	// Two periods: 2026-04 and 2026-05 (clearly in range relative to demoNow 2026-06).
	periods := []struct {
		year, month int
		paidOn      string
	}{
		{2026, 4, "2026-04-28"},
		{2026, 5, "2026-05-28"},
	}

	psSeq := 23001
	payslipCount := 0
	noteIdx := 0
	subset := 40
	if subset > len(active) {
		subset = len(active)
	}
	for i := 0; i < subset; i++ {
		a := active[i]
		for _, p := range periods {
			psID := fmt.Sprintf("SWP-PS-%05d", psSeq)
			psSeq++

			base := 4500000 + demoRand.Intn(4000001)
			transport := 800000 + demoRand.Intn(600001)
			meal := 500000 + demoRand.Intn(500001)
			gross := base + transport + meal
			bpjsKes := base / 100     // 1%
			bpjsJht := base * 2 / 100 // 2%
			pph := gross * 5 / 100    // rough 5%
			deductions := bpjsKes + bpjsJht + pph
			take := gross - deductions
			workingDays := 20 + demoRand.Intn(3)

			geEnc, e1 := enc(money2(gross))
			gdEnc, e2 := enc(money2(deductions))
			thEnc, e3 := enc(money2(take))
			if e1 != nil || e2 != nil || e3 != nil {
				return fmt.Errorf("demo payslip %q: encrypt: %v/%v/%v", psID, e1, e2, e3)
			}

			var returnedID string
			scanErr := pool.Pool.QueryRow(ctx, psQ,
				psID, a.empID, a.name, a.plID, p.year, p.month, p.paidOn,
				workingDays, geEnc, gdEnc, thEnc, fmt.Sprintf("DEMO-%05d-%d%02d", i, p.year, p.month),
			).Scan(&returnedID)
			if scanErr != nil {
				if errors.Is(scanErr, pgx.ErrNoRows) {
					continue // already present (idempotent): skip lines
				}
				return fmt.Errorf("demo payslip %q: %w", psID, scanErr)
			}
			payslipCount++

			// Components.
			comps := []struct {
				kind, name, value string
				forBPJS           bool
			}{
				{"EARNING", "Gaji Pokok", money2(base), true},
				{"EARNING", "Tunjangan Transport", money2(transport), false},
				{"EARNING", "Tunjangan Makan", money2(meal), false},
				{"DEDUCTION", "BPJS Kesehatan (1%)", money2(bpjsKes), true},
				{"DEDUCTION", "BPJS Ketenagakerjaan (JHT 2%)", money2(bpjsJht), true},
				{"DEDUCTION", "PPh 21", money2(pph), false},
			}
			for ci, c := range comps {
				ve, eerr := enc(c.value)
				if eerr != nil {
					return fmt.Errorf("demo component %q: encrypt: %w", c.name, eerr)
				}
				if _, err := pool.Pool.Exec(ctx, compQ, psID, c.kind, c.name, ve, c.forBPJS, ci); err != nil {
					return fmt.Errorf("demo component %q: %w", c.name, err)
				}
			}
			// Benefits.
			bens := []struct{ name, value string }{
				{"BPJS Kesehatan (employer 4%)", money2(base * 4 / 100)},
				{"BPJS JKK", money2(base * 24 / 10000)},
			}
			for bi, b := range bens {
				ve, eerr := enc(b.value)
				if eerr != nil {
					return fmt.Errorf("demo benefit %q: encrypt: %w", b.name, eerr)
				}
				if _, err := pool.Pool.Exec(ctx, benQ, psID, b.name, ve, bi); err != nil {
					return fmt.Errorf("demo benefit %q: %w", b.name, err)
				}
			}

			// A couple of audit notes on every ~10th payslip.
			if i%10 == 0 {
				noteIdx++
				created := demoNow.AddDate(0, 0, -3).UTC().Format(time.RFC3339)
				note1 := psID + "-NOTE-1"
				if _, err := pool.Pool.Exec(ctx, noteQ, note1, psID, 1,
					"Slip gaji diverifikasi tim payroll (demo).", "SWP-EMP-1042", "Sari Hadi", created); err != nil {
					return fmt.Errorf("demo audit note %q: %w", note1, err)
				}
			}
		}
	}
	slog.Info("demo: seeded payslips + breakdown", "payslips", payslipCount, "agents", subset, "periods", len(periods))
	return nil
}

// money2 renders an integer rupiah amount as a 2-decimal Money string.
func money2(n int) string { return fmt.Sprintf("%d.00", n) }

// -----------------------------------------------------------------------------
// Step 11: notifications (mixed read/unread, across kinds) + richer audit_log.
// -----------------------------------------------------------------------------

func seedDemoNotifications(ctx context.Context, pool *db.Pool, d *demoData) error {
	const ntfQ = `
		INSERT INTO notifications
			(id, recipient_id, kind, title, body,
			 deep_link_epic, deep_link_entity_id, deep_link_path,
			 actor_id, actor_label, is_critical, read_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO NOTHING`

	kinds := []struct {
		kind, title, body, epic, path string
		critical                      bool
	}{
		{"LEAVE_REQUEST_SUBMITTED", "Pengajuan cuti baru", "Seorang agen mengajukan cuti.", "E6", "/leave-requests", true},
		{"LEAVE_APPROVED", "Cuti disetujui", "Pengajuan cuti Anda disetujui.", "E6", "/leave-requests", true},
		{"OT_APPROVED", "Lembur disetujui", "Pengajuan lembur Anda disetujui.", "E7", "/overtime", true},
		{"ATTENDANCE_VERIFY_NEEDED", "Verifikasi kehadiran", "Beberapa catatan kehadiran menunggu verifikasi.", "E5", "/attendance?status=PENDING", false},
		{"PLACEMENT_EXPIRING", "Penempatan akan berakhir", "Sebuah penempatan berakhir dalam 30 hari.", "E3", "/placements", false},
		{"CHANGE_REQUEST_SUBMITTED", "Permintaan perubahan data", "Agen mengajukan perubahan data pribadi.", "E2", "/change-requests", false},
	}

	// Recipients: the HR persona + a sample of demo leaders/agents.
	recipients := []string{"SWP-EMP-1042", "SWP-EMP-2891"}
	for _, c := range d.companies {
		for _, a := range d.agents {
			if a.companyID == c.id && a.isLeader && !a.terminal {
				recipients = append(recipients, a.empID)
				break
			}
		}
	}
	// Add a handful of regular agents.
	for i := 0; i < 6 && i < len(d.agents); i++ {
		recipients = append(recipients, d.agents[i*5%len(d.agents)].empID)
	}

	sysActor := "SWP-USR-00002"
	ntfSeq := 24001
	count := 0
	target := 80
	for count < target {
		for _, rcp := range recipients {
			if count >= target {
				break
			}
			k := kinds[count%len(kinds)]
			ntfID := fmt.Sprintf("SWP-NTF-%05d", ntfSeq)
			ntfSeq++

			created := demoNow.Add(-time.Duration(count*37) * time.Minute)
			var readAt any
			if count%2 == 0 { // half read
				r := created.Add(20 * time.Minute)
				readAt = r
			}
			var actorID any = sysActor
			if k.kind == "LEAVE_REQUEST_SUBMITTED" || k.kind == "CHANGE_REQUEST_SUBMITTED" {
				actorID = nil // system/agent-origin
			}

			if _, err := pool.Pool.Exec(ctx, ntfQ,
				ntfID, rcp, k.kind, k.title, k.body,
				k.epic, nil, k.path,
				actorID, "system", k.critical, readAt, created,
			); err != nil {
				return fmt.Errorf("seed demo notification %q: %w", ntfID, err)
			}
			count++
		}
	}
	slog.Info("demo: seeded notifications", "count", count, "recipients", len(recipients))
	return nil
}

// seedDemoAuditLog adds a richer set of audit rows so dashboards / billable
// reports render populated. Idempotent via the unique SWP-AL ids generated by
// the swp_next_id allocator — but to keep idempotency we guard on a marker row.
func seedDemoAuditLog(ctx context.Context, pool *db.Pool, d *demoData) error {
	// Guard: skip if our demo marker already inserted.
	var exists bool
	if err := pool.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM audit_log WHERE entity_id = $1)`, "SWP-DEMO-AUDIT-MARKER",
	).Scan(&exists); err != nil {
		return fmt.Errorf("demo audit_log: check marker: %w", err)
	}
	if exists {
		slog.Info("demo: audit_log demo rows already present, skipping")
		return nil
	}

	const insertQ = `
		INSERT INTO audit_log
			(id, actor_user_id, actor_role, action, entity_type, entity_id,
			 before_state, after_state, request_id, created_at)
		VALUES
			('SWP-AL-' || swp_next_id('AL'), $1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, NULL, $8::timestamptz)`

	type row struct {
		actor, role, action, etype, eid, before, after string
		hoursAgo                                       int
	}
	rows := []row{
		{"", "", "CREATE", "user", "SWP-DEMO-AUDIT-MARKER", "", `{"note":"demo dataset seeded"}`, 72},
	}
	// Placement create rows for the first few demo placements.
	for i := 0; i < 8 && i < len(d.agents); i++ {
		a := d.agents[i]
		rows = append(rows, row{
			actor: "SWP-USR-00001", role: "hr_admin", action: "CREATE",
			etype: "placement", eid: a.plID,
			after:    fmt.Sprintf(`{"employee_id":%q,"company_id":%q}`, a.empID, a.companyID),
			hoursAgo: 60 - i*5,
		})
	}
	// A few leave-approval + overtime-approval audit rows.
	for i := 0; i < 6 && i < len(d.agents); i++ {
		a := d.agents[i]
		rows = append(rows, row{
			actor: "SWP-USR-00002", role: "hr_admin", action: "leave.approve_final",
			etype: "leave_request", eid: fmt.Sprintf("SWP-LR-%05d", 21001+i),
			before:   `{"status":"PENDING_HR"}`,
			after:    `{"status":"APPROVED"}`,
			hoursAgo: 30 - i*3,
		})
		_ = a
	}

	for _, r := range rows {
		var actor, role, before, after any
		if r.actor != "" {
			actor = r.actor
		}
		if r.role != "" {
			role = r.role
		}
		if r.before != "" {
			before = r.before
		}
		if r.after != "" {
			after = r.after
		}
		created := demoNow.Add(-time.Duration(r.hoursAgo) * time.Hour).UTC().Format(time.RFC3339)
		if _, err := pool.Pool.Exec(ctx, insertQ, actor, role, r.action, r.etype, r.eid, before, after, created); err != nil {
			return fmt.Errorf("demo audit_log row %q: %w", r.eid, err)
		}
	}
	slog.Info("demo: seeded audit_log", "count", len(rows))
	return nil
}
