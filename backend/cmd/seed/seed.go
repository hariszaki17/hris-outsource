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
		employeeID: strPtr("SWP-EMP-1108"),
		companyID:  strPtr("SWP-CMP-0021"),
	},
	{
		email:      "super.admin@swp.test",
		password:   PasswordSuperAdmin,
		role:       "super_admin",
		employeeID: nil,
		companyID:  nil,
	},
	{
		email:      "agent.budi@swp.test",
		password:   PasswordAgent,
		role:       "agent",
		employeeID: strPtr("SWP-EMP-2891"),
		companyID:  nil,
	},
}

// Seed inserts the deterministic test personas into the database. It is
// idempotent: if a non-deleted user with the same email already exists, that
// persona is skipped (no error, no duplicate). Safe to re-run between test
// runs or after a migrate-up on an empty DB.
func Seed(ctx context.Context, pool *db.Pool) error {
	q := sqlcgen.New(pool.Pool)

	for _, p := range personas {
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
			EmployeeID:   p.employeeID,
			CompanyID:    p.companyID,
		})
		if err != nil {
			return fmt.Errorf("create user %q: %w", p.email, err)
		}
		slog.Info("seed: created user", "email", user.Email, "id", user.ID, "role", user.Role)
	}

	return nil
}
