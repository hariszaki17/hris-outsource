//go:build integration

// Package attendance (repository) — DB-backed integration test for the F5.5
// listAttendance filters (AR-10 agent date-range, AR-11 status). This is the only
// test that validates the REAL ListAttendance SQL string + the date-basis predicate
// against the live schema; the service/handler tests use fakes.
//
// The date basis is COALESCE(shift_start_at, check_in_at) rendered in Asia/Jakarta
// (so auto-created ABSENT rows with a NULL check_in_at are still range-filtered, and
// scheduled rows bucket on their WIB shift day rather than the UTC check-in day).
//
// Gated behind `//go:build integration` — needs Docker. Run with:
//
//	go test -tags=integration ./internal/repository/attendance/...
package attendance

import (
	"context"
	"testing"
	"time"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// TestListAttendance_DateBasisAndStatus_Integration seeds rows for one agent and
// drives the real ListAttendance SQL, asserting:
//   - the date range buckets on the Jakarta shift day (not the UTC check-in instant),
//   - an ABSENT row (NULL check_in_at) is kept by its shift_start_at,
//   - a cross-midnight WIB check-in is bucketed on its shift day, not the UTC date,
//   - status filtering is multi-value IN.
func TestListAttendance_DateBasisAndStatus_Integration(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(t, ctx)

	pool, err := db.Open(ctx, connStr, 4)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer pool.Close()

	const empID = "SWP-EMP-LF1"

	jkt, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("load Asia/Jakarta: %v", err)
	}
	// Shift starts at 08:00 WIB on the given May day; check_in 10 min later.
	shiftAt := func(d int) time.Time { return time.Date(2026, time.May, d, 8, 0, 0, 0, jkt).UTC() }

	if _, err := pool.Exec(ctx, `SET session_replication_role = replica`); err != nil {
		t.Fatalf("disable FK: %v", err)
	}
	insert := func(id, status string, shiftStart *time.Time, checkIn *time.Time) {
		t.Helper()
		_, err := pool.Exec(ctx, `
			INSERT INTO attendance (id, employee_id, placement_id, company_id, site_id, position,
			                        shift_start_at, check_in_at, status, verification_status, lat_in, lng_in)
			VALUES ($1, $2, 'SWP-PL-LF1', 'SWP-CMP-LF1', 'SWP-SITE-LF1', 'Petugas Parkir',
			        $3, $4, $5, 'VERIFIED', -6.2, 106.8)`,
			id, empID, shiftStart, checkIn, status)
		if err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	p := func(t time.Time) *time.Time { return &t }

	// In-range (5–18 May, WIB shift day):
	in5 := shiftAt(5).Add(10 * time.Minute)
	insert("SWP-ATT-LF05", "PRESENT", p(shiftAt(5)), p(in5))
	in10 := shiftAt(10).Add(40 * time.Minute)
	insert("SWP-ATT-LF10", "LATE", p(shiftAt(10)), p(in10))
	// ABSENT: NULL check_in_at, in-range shift_start_at — must survive the range filter.
	insert("SWP-ATT-LFAB", "ABSENT", p(shiftAt(12)), nil)
	// Cross-midnight WIB: shift_start_at = 18 May 00:30 WIB = 17 May 17:30 UTC. The naive
	// UTC basis would bucket it on 17 May; the Jakarta basis correctly buckets on 18 May.
	cmShift := time.Date(2026, time.May, 18, 0, 30, 0, 0, jkt).UTC()
	insert("SWP-ATT-LFCM", "PRESENT", p(cmShift), p(cmShift.Add(5*time.Minute)))

	// Out-of-range:
	insert("SWP-ATT-LF04", "PRESENT", p(shiftAt(4)), p(shiftAt(4)))
	insert("SWP-ATT-LF19", "LATE", p(shiftAt(19)), p(shiftAt(19)))

	if _, err := pool.Exec(ctx, `SET session_replication_role = origin`); err != nil {
		t.Fatalf("restore FK: %v", err)
	}

	repo := NewAttendanceRepo(pool)
	empPtr := empID
	dateP := func(y int, m time.Month, d int) *time.Time {
		t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		return &t
	}

	// (1) date range only — inclusive 5–18 May, Jakarta shift basis.
	rows, err := repo.ListAttendance(ctx, svc.AttendanceFilter{
		EmployeeID: &empPtr,
		DateFrom:   dateP(2026, time.May, 5),
		DateTo:     dateP(2026, time.May, 18),
		Limit:      100,
	})
	if err != nil {
		t.Fatalf("ListAttendance (range): %v", err)
	}
	got := idsOf(rows)
	want := map[string]bool{
		"SWP-ATT-LF05": true, "SWP-ATT-LF10": true,
		"SWP-ATT-LFAB": true, "SWP-ATT-LFCM": true,
	}
	if !sameSet(got, want) {
		t.Errorf("range ids = %v, want %v (5–18 Mei WIB incl. ABSENT + cross-midnight)", got, want)
	}

	// (2) range + multi-status (LATE,ABSENT).
	rows, err = repo.ListAttendance(ctx, svc.AttendanceFilter{
		EmployeeID: &empPtr,
		DateFrom:   dateP(2026, time.May, 5),
		DateTo:     dateP(2026, time.May, 18),
		Status:     []string{"LATE", "ABSENT"},
		Limit:      100,
	})
	if err != nil {
		t.Fatalf("ListAttendance (range+status): %v", err)
	}
	got = idsOf(rows)
	want = map[string]bool{"SWP-ATT-LF10": true, "SWP-ATT-LFAB": true}
	if !sameSet(got, want) {
		t.Errorf("range+status ids = %v, want %v", got, want)
	}

	// (3) single status, no range.
	rows, err = repo.ListAttendance(ctx, svc.AttendanceFilter{
		EmployeeID: &empPtr,
		Status:     []string{"PRESENT"},
		Limit:      100,
	})
	if err != nil {
		t.Fatalf("ListAttendance (status): %v", err)
	}
	got = idsOf(rows)
	want = map[string]bool{"SWP-ATT-LF05": true, "SWP-ATT-LFCM": true, "SWP-ATT-LF04": true}
	if !sameSet(got, want) {
		t.Errorf("status=PRESENT ids = %v, want %v", got, want)
	}
}

func idsOf(rows []att.Attendance) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		out[r.ID] = true
	}
	return out
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
