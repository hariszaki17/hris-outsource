//go:build integration

// Package attendance (repository) — DB-backed integration test for the F5.1
// flexible-check-in auto-close path: it exercises the REAL AutoCloseAttendance SQL
// (plus GetOpenAttendance / GetAttendance) against a throwaway Postgres spun by
// testcontainers, with the full goose schema applied. This is the only test that
// validates the query string + column mapping against the live schema; the service /
// handler tests use fakes.
//
// Gated behind `//go:build integration` — needs Docker. Run with:
//   go test -tags=integration ./internal/repository/attendance/...
// `make test` (plain `go test ./...`) does NOT run it.
package attendance

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // "pgx" database/sql driver for goose
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	migrations "github.com/hariszaki17/hris-outsource/backend/db/migrations"
	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// startPostgres boots a throwaway Postgres and applies the goose schema. Returns the
// connection string; registers container teardown on t.Cleanup.
func startPostgres(t *testing.T, ctx context.Context) string {
	t.Helper()
	pg, err := postgres.Run(ctx, "postgres:16",
		postgres.WithDatabase("hris"),
		postgres.WithUsername("hris"),
		postgres.WithPassword("hris"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("conn string: %v", err)
	}

	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer sqlDB.Close()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.RunContext(ctx, "up", sqlDB, "."); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	return connStr
}

// TestAutoCloseAttendance_Integration inserts one open attendance row (FK enforcement
// disabled so no full placement/employee chain is needed), then drives the real repo:
// GetOpenAttendance → GetAttendance → AutoCloseAttendance → re-read, asserting the row
// was closed at the supplied shift_end with auto_closed=true, the AUTO_CLOSED flag,
// status INCOMPLETE, and worked_minutes set.
func TestAutoCloseAttendance_Integration(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(t, ctx)

	pool, err := db.Open(ctx, connStr, 4)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer pool.Close()

	const (
		attID = "SWP-ATT-IT1"
		empID = "SWP-EMP-IT1"
	)
	checkIn := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Second)
	shiftEnd := checkIn.Add(att.FallbackShiftHours) // computed close instant

	// Insert an OPEN row with FK checks off (session_replication_role=replica) so we
	// don't have to seed employees/placements/companies. Only NOT-NULL-without-default
	// columns are supplied; the rest take their schema defaults.
	if _, err := pool.Exec(ctx, `SET session_replication_role = replica`); err != nil {
		t.Fatalf("disable FK: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO attendance (id, employee_id, placement_id, company_id, service_line, site_id, position_id, check_in_at, lat_in, lng_in)
		VALUES ($1, $2, 'SWP-PL-IT1', 'SWP-CMP-IT1', 'parking', 'SWP-SITE-IT1', 'SWP-POS-IT1', $3, -6.2, 106.8)`,
		attID, empID, checkIn)
	if err != nil {
		t.Fatalf("insert open row: %v", err)
	}
	if _, err := pool.Exec(ctx, `SET session_replication_role = origin`); err != nil {
		t.Fatalf("restore FK: %v", err)
	}

	repo := NewClockRepo(pool)

	// GetOpenAttendance finds the open row.
	openID, found, err := repo.GetOpenAttendance(ctx, empID)
	if err != nil || !found {
		t.Fatalf("GetOpenAttendance: id=%q found=%v err=%v", openID, found, err)
	}
	if openID != attID {
		t.Fatalf("open id = %q, want %q", openID, attID)
	}

	// AutoCloseAttendance closes it in a tx at the computed shift_end.
	worked := int(shiftEnd.Sub(checkIn).Minutes())
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	closedID, ok, err := repo.AutoCloseAttendance(ctx, tx, svc.AutoCloseRow{
		ID:                 openID,
		CheckOutAt:         shiftEnd,
		WorkedMinutes:      worked,
		Flags:              []string{string(att.FlagAutoClosed)},
		Status:             string(att.StatusIncomplete),
		VerificationStatus: string(att.VerificationPending),
	})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("AutoCloseAttendance: %v", err)
	}
	if !ok || closedID != attID {
		t.Fatalf("auto-close: ok=%v id=%q, want true %q", ok, closedID, attID)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Re-read and assert the persisted state.
	rec, err := repo.GetAttendance(ctx, attID)
	if err != nil {
		t.Fatalf("GetAttendance: %v", err)
	}
	if rec.CheckOutAt == nil {
		t.Fatal("check_out_at is nil, want set")
	}
	if !rec.CheckOutAt.UTC().Equal(shiftEnd) {
		t.Errorf("check_out_at = %v, want %v (shift_end)", rec.CheckOutAt.UTC(), shiftEnd)
	}
	if !rec.AutoClosed {
		t.Error("auto_closed = false, want true")
	}
	if rec.Status != att.StatusIncomplete {
		t.Errorf("status = %q, want INCOMPLETE", rec.Status)
	}
	if rec.WorkedMinutes == nil || *rec.WorkedMinutes != worked {
		t.Errorf("worked_minutes = %v, want %d", rec.WorkedMinutes, worked)
	}
	hasAutoClosed := false
	for _, f := range rec.Flags {
		if f == att.FlagAutoClosed {
			hasAutoClosed = true
		}
	}
	if !hasAutoClosed {
		t.Errorf("flags = %v, want AUTO_CLOSED present", rec.Flags)
	}

	// The row is no longer open.
	if _, stillOpen, _ := repo.GetOpenAttendance(ctx, empID); stillOpen {
		t.Error("row still reported open after auto-close")
	}
}
