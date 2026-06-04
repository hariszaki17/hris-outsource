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
