// Package attendance_test — E5 attendance + corrections contract tests.
//
// Shared test plumbing: a fakeTx (Exec no-op so audit.Record works inside InTx),
// a fakeTxRunner, in-memory fake repos implementing the 07-02 service ports
// (svc.AttendanceRepository + svc.CorrectionRepository), an in-memory idempotency
// middleware mirroring the real Postgres-backed contract (replay same body / 409
// IDEMPOTENCY_KEY_REUSED on a different body), and a newHarness that mounts the
// REAL attendance.Service + correction.Service + attendance.Handler over the fakes
// on a chi.Router with a mutable-principal middleware (swap role/company/employee
// per case).
//
// Mirrors internal/handler/scheduling/scheduling_testkit_test.go EXACTLY: same
// fakeTx shape, same decodeBody helper, same dynamic-principal injection pattern.
package attendance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	attendancehandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// ---------------------------------------------------------------------------
// fakeTx — only Exec is needed (audit.Record); all other methods panic.
// ---------------------------------------------------------------------------

type fakeTx struct{}

func (f *fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return f, nil }
func (f *fakeTx) Commit(_ context.Context) error          { return nil }
func (f *fakeTx) Rollback(_ context.Context) error        { return nil }
func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	panic("fakeTx: Query not implemented")
}
func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("fakeTx: QueryRow not implemented")
}
func (f *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	panic("fakeTx: CopyFrom not implemented")
}
func (f *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults {
	panic("fakeTx: SendBatch not implemented")
}
func (f *fakeTx) LargeObjects() pgx.LargeObjects {
	panic("fakeTx: LargeObjects not implemented")
}
func (f *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	panic("fakeTx: Prepare not implemented")
}
func (f *fakeTx) Conn() *pgx.Conn { return nil }

var _ pgx.Tx = (*fakeTx)(nil)

type fakeTxRunner struct{}

func (f *fakeTxRunner) InTx(_ context.Context, fn func(pgx.Tx) error) error {
	return fn(&fakeTx{})
}

// fakeScheduleEntry holds an employee's schedule projection for manual create tests.
type fakeScheduleEntry struct {
	scheduleID string
	shiftStart time.Time
	shiftEnd   time.Time
}

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

func errObject(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	e, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("response has no error object: %v", body)
	}
	return e
}

func errCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	return strOf(errObject(t, decodeBody(t, rr))["code"])
}

func strOf(v any) string {
	s, _ := v.(string)
	return s
}

func strp(s string) *string { return &s }

func itoa(n int) string { return strconv.Itoa(n) }

// fixedNow is the deterministic clock for all attendance tests (Asia/Jakarta-safe;
// 12:00 WIB on 2026-06-04 — mirrors the scheduling test clock).
var fixedNow = time.Date(2026, 6, 4, 5, 0, 0, 0, time.UTC)

// ymd is a YYYY-MM-DD date in UTC (matches the seed fixture convention).
func ymd(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// in-memory idempotency middleware — mirrors the real Postgres-backed contract
// (CONVENTIONS §13): same key + same body → replay stored response; same key +
// different body → 409 IDEMPOTENCY_KEY_REUSED; missing key → pass-through.
//
// SEAM NOTE (documented in the SUMMARY): the production middleware persists to
// the idempotency_keys table via *db.Pool, which the fake harness cannot stand
// up. This stub reproduces the same observable contract at the router boundary
// (scoped by principal user id) so the replay / reuse behaviour is asserted here
// without Postgres; the Postgres store itself is exercised by the 07-04 E2E.
// ---------------------------------------------------------------------------

type stubIdempotency struct {
	mu    sync.Mutex
	store map[string]idemEntry // scopedKey -> stored response
}

type idemEntry struct {
	reqHash string
	status  int
	body    []byte
}

func newStubIdempotency() *stubIdempotency {
	return &stubIdempotency{store: map[string]idemEntry{}}
}

func (m *stubIdempotency) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		body, _ := readAndRestoreBody(r)
		p, _ := auth.PrincipalFrom(r.Context())
		scopedKey := p.UserID + ":" + key
		reqHash := hashBytes(body)

		m.mu.Lock()
		ent, found := m.store[scopedKey]
		m.mu.Unlock()

		if found {
			if ent.reqHash != reqHash {
				httpx.WriteError(w, r, apperr.Conflict("IDEMPOTENCY_KEY_REUSED"))
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(ent.status)
			_, _ = w.Write(ent.body)
			return
		}

		rec := &captureRW{ResponseWriter: w, status: http.StatusOK, buf: &bytes.Buffer{}}
		next.ServeHTTP(rec, r)
		if rec.status >= 200 && rec.status < 300 {
			m.mu.Lock()
			m.store[scopedKey] = idemEntry{reqHash: reqHash, status: rec.status, body: rec.buf.Bytes()}
			m.mu.Unlock()
		}
	})
}

func readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	_ = r.Body.Close()
	body := buf.Bytes()
	r.Body = newReadCloser(body)
	return body, err
}

func hashBytes(b []byte) string {
	// A cheap, stable digest; equality is all the replay contract needs.
	return strconv.Itoa(len(b)) + ":" + string(b)
}

type captureRW struct {
	http.ResponseWriter
	status int
	buf    *bytes.Buffer
}

func (c *captureRW) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureRW) Write(b []byte) (int, error) {
	c.buf.Write(b)
	return c.ResponseWriter.Write(b)
}

func newReadCloser(b []byte) *bodyReader { return &bodyReader{r: bytes.NewReader(b)} }

type bodyReader struct{ r *bytes.Reader }

func (b *bodyReader) Read(p []byte) (int, error) { return b.r.Read(p) }
func (b *bodyReader) Close() error               { return nil }

// ---------------------------------------------------------------------------
// fakeAttendanceRepo — in-memory svc.AttendanceRepository.
//
// Verify/Reject mutate the map and return the updated row + 1 (or zero rows for
// a terminal record → drives the 409 CONFLICT). ApplyCorrectionToAttendance
// applies the COALESCE whitelist + appends the CORRECTED flag.
// ---------------------------------------------------------------------------

type fakeAttendanceRepo struct {
	records   map[string]att.Attendance
	schedules map[string]fakeScheduleEntry // employeeID → schedule (shared across tests)
	seq       int
}

func newFakeAttendanceRepo() *fakeAttendanceRepo {
	return &fakeAttendanceRepo{
		records:   map[string]att.Attendance{},
		schedules: map[string]fakeScheduleEntry{},
	}
}

func (r *fakeAttendanceRepo) ListAttendance(_ context.Context, f svc.AttendanceFilter) ([]att.Attendance, error) {
	var out []att.Attendance
	for _, a := range r.records {
		if f.CompanyID != nil && a.CompanyID != *f.CompanyID {
			continue
		}
		if f.EmployeeID != nil && a.EmployeeID != *f.EmployeeID {
			continue
		}
		if f.SiteID != nil && a.SiteID != *f.SiteID {
			continue
		}
		if f.PositionID != nil && a.PositionID != *f.PositionID {
			continue
		}
		if len(f.VerificationStatus) > 0 && !contains(f.VerificationStatus, string(a.VerificationStatus)) {
			continue
		}
		out = append(out, a)
	}
	// (check_in_at DESC, id) keyset — newest first. check_in_at is nullable (ABSENT).
	sort.Slice(out, func(i, j int) bool {
		ci, cj := ciOf(out[i]), ciOf(out[j])
		if ci.Equal(cj) {
			return out[i].ID > out[j].ID
		}
		return ci.After(cj)
	})
	if f.CursorCheckInAt != nil && f.CursorID != nil {
		var trimmed []att.Attendance
		for _, a := range out {
			ci := ciOf(a)
			if ci.Before(*f.CursorCheckInAt) ||
				(ci.Equal(*f.CursorCheckInAt) && a.ID < *f.CursorID) {
				trimmed = append(trimmed, a)
			}
		}
		out = trimmed
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakeAttendanceRepo) GetAttendance(_ context.Context, id string) (att.Attendance, error) {
	a, ok := r.records[id]
	if !ok {
		return att.Attendance{}, domain.ErrNotFound
	}
	return a, nil
}

func (r *fakeAttendanceRepo) GetAttendanceForUpdate(_ context.Context, _ pgx.Tx, id string) (att.Attendance, error) {
	return r.GetAttendance(context.Background(), id)
}

// verifiable reports whether the record is in a state a verify/reject can touch
// (mirrors the real RETURNING guard WHERE verification_status IN PENDING/ESCALATED).
func verifiable(v att.VerificationStatus) bool {
	return v == att.VerificationPending || v == att.VerificationEscalated
}

func (r *fakeAttendanceRepo) VerifyAttendance(_ context.Context, _ pgx.Tx, id string, verifiedBy *string) (att.Attendance, int64, error) {
	a, ok := r.records[id]
	if !ok {
		return att.Attendance{}, 0, domain.ErrNotFound
	}
	if !verifiable(a.VerificationStatus) {
		return att.Attendance{}, 0, nil // zero rows → terminal-state 409
	}
	a.VerificationStatus = att.VerificationVerified
	a.VerifiedBy = verifiedBy
	vt := fixedNow
	a.VerifiedAt = &vt
	r.records[id] = a
	return a, 1, nil
}

func (r *fakeAttendanceRepo) VerifyAttendanceWithTimes(_ context.Context, _ pgx.Tx, id string, checkInAt time.Time, _ *time.Time, status string, isLate bool, lateMinutes int, verifiedBy *string) (att.Attendance, int64, error) {
	a, ok := r.records[id]
	if !ok {
		return att.Attendance{}, 0, domain.ErrNotFound
	}
	if !verifiable(a.VerificationStatus) {
		return att.Attendance{}, 0, nil
	}
	a.CheckInAt = &checkInAt
	a.VerificationStatus = att.VerificationVerified
	a.Status = att.AttendanceStatus(status)
	a.IsLate = isLate
	a.LateMinutes = lateMinutes
	a.VerifiedBy = verifiedBy
	vt := fixedNow
	a.VerifiedAt = &vt
	r.records[id] = a
	return a, 1, nil
}

func (r *fakeAttendanceRepo) RejectAttendance(_ context.Context, _ pgx.Tx, id string, rejectedBy *string, reason string) (att.Attendance, int64, error) {
	a, ok := r.records[id]
	if !ok {
		return att.Attendance{}, 0, domain.ErrNotFound
	}
	if !verifiable(a.VerificationStatus) {
		return att.Attendance{}, 0, nil
	}
	a.VerificationStatus = att.VerificationRejected
	a.RejectedBy = rejectedBy
	rt := fixedNow
	a.RejectedAt = &rt
	a.RejectReason = strp(reason)
	r.records[id] = a
	return a, 1, nil
}

func (r *fakeAttendanceRepo) ApplyCorrectionToAttendance(_ context.Context, _ pgx.Tx, p svc.ApplyCorrectionParams) (att.Attendance, error) {
	a, ok := r.records[p.ID]
	if !ok {
		return att.Attendance{}, domain.ErrNotFound
	}
	if p.CheckInAt != nil {
		a.CheckInAt = p.CheckInAt
	}
	if p.CheckOutAt != nil {
		a.CheckOutAt = p.CheckOutAt
	}
	if p.AttendanceCodeID != nil {
		a.AttendanceCodeID = p.AttendanceCodeID
	}
	// BR CR-9 re-eval outputs (nil = leave as-is).
	if p.Status != nil {
		a.Status = att.AttendanceStatus(*p.Status)
	}
	if p.IsLate != nil {
		a.IsLate = *p.IsLate
	}
	if p.LateMinutes != nil {
		a.LateMinutes = *p.LateMinutes
	}
	a.LastCorrectionID = p.LastCorrectionID
	if !hasFlag(a.Flags, att.FlagCorrected) {
		a.Flags = append(a.Flags, att.FlagCorrected)
	}
	r.records[p.ID] = a
	return a, nil
}

func (r *fakeAttendanceRepo) GetActivePlacement(_ context.Context, employeeID string) (svc.PlacementInfo, bool, error) {
	if employeeID == empNoPlacement {
		return svc.PlacementInfo{}, false, nil
	}
	return svc.PlacementInfo{
		PlacementID: "SWP-PL-9999",
		CompanyID:   cmpLed,
		SiteID:      "SWP-SITE-001",
		PositionID:  "SWP-POS-001",
		ServiceLine: att.ServiceLineFacilityServices,
	}, true, nil
}

func (r *fakeAttendanceRepo) GetTodaySchedule(_ context.Context, employeeID string, _ time.Time) (string, time.Time, time.Time, bool, error) {
	s, ok := r.schedules[employeeID]
	if !ok {
		return "", time.Time{}, time.Time{}, false, nil
	}
	return s.scheduleID, s.shiftStart, s.shiftEnd, true, nil
}

func (r *fakeAttendanceRepo) CreateManualAttendance(_ context.Context, _ pgx.Tx, p svc.CreateManualAttendanceParams) (att.Attendance, error) {
	r.seq++
	id := "SWP-ATT-M" + strconv.Itoa(r.seq)
	worked := p.WorkedMinutes
	latMin := int32(p.LateMinutes)
	flagsList := make([]att.Flag, len(p.Flags))
	for i, f := range p.Flags {
		flagsList[i] = att.Flag(f)
	}
	a := att.Attendance{
		ID:                 id,
		EmployeeID:         p.EmployeeID,
		PlacementID:        p.PlacementID,
		ScheduleID:         p.ScheduleID,
		CompanyID:          p.CompanyID,
		ServiceLine:        p.ServiceLine,
		SiteID:             p.SiteID,
		PositionID:         p.PositionID,
		AttendanceCodeID:   p.AttendanceCodeID,
		ShiftStartAt:       p.ShiftStartAt,
		ShiftEndAt:         p.ShiftEndAt,
		CheckInAt:          &p.CheckInAt,
		CheckOutAt:         p.CheckOutAt,
		WFO:                p.WFO,
		IsLate:             p.IsLate,
		LateMinutes:        int(latMin),
		WorkedMinutes:      worked,
		AutoClosed:         false,
		Status:             att.AttendanceStatus(p.Status),
		VerificationStatus: att.VerificationStatus(p.VerificationStatus),
		Flags:              flagsList,
		CreatedBy:          p.CreatedBy,
		CreatedAt:          p.CreatedAt,
		UpdatedAt:          p.UpdatedAt,
	}
	r.records[id] = a
	return a, nil
}

func (r *fakeAttendanceRepo) GetManualAutofillData(_ context.Context, employeeID string, refDate time.Time) (svc.ManualAutofillData, bool, error) {
	if employeeID == empNoPlacement {
		return svc.ManualAutofillData{}, false, nil
	}
	siteName := "Site of " + cmpLed
	posName := "Position of " + employeeID
	data := svc.ManualAutofillData{
		PlacementID:  "SWP-PL-9999",
		CompanyID:    cmpLed,
		ServiceLine:  att.ServiceLineFacilityServices,
		SiteID:       "SWP-SITE-001",
		PositionID:   "SWP-POS-001",
		EmployeeName: "Agent " + employeeID,
		CompanyName:  "Company " + cmpLed,
		SiteName:     &siteName,
		PositionName: &posName,
	}
	if s, ok := r.schedules[employeeID]; ok {
		ss := s.shiftStart
		se := s.shiftEnd
		data.ScheduleID = &s.scheduleID
		data.ShiftStartAt = &ss
		data.ShiftEndAt = &se
	}
	return data, true, nil
}

// seedSchedule plants a schedule entry for a given employee (bypasses fake store).
func (r *fakeAttendanceRepo) seedSchedule(employeeID string, ss, se time.Time) {
	r.schedules[employeeID] = fakeScheduleEntry{
		scheduleID: "SWP-SCH-9xxx",
		shiftStart: ss,
		shiftEnd:   se,
	}
}

var _ svc.AttendanceRepository = (*fakeAttendanceRepo)(nil)

// ---------------------------------------------------------------------------
// fakeCorrectionRepo — in-memory svc.CorrectionRepository.
//
// Approve/Reject guard the PENDING state (zero rows on a terminal correction →
// drives the 409 CONFLICT). pendingByAttendance backs the CORRECTION_ALREADY_PENDING
// pre-check seam.
// ---------------------------------------------------------------------------

type fakeCorrectionRepo struct {
	records map[string]att.Correction
}

func newFakeCorrectionRepo() *fakeCorrectionRepo {
	return &fakeCorrectionRepo{records: map[string]att.Correction{}}
}

func (r *fakeCorrectionRepo) ListCorrections(_ context.Context, f svc.CorrectionFilter) ([]att.Correction, error) {
	var out []att.Correction
	for _, c := range r.records {
		if f.CompanyID != nil && c.CompanyID != *f.CompanyID {
			continue
		}
		if len(f.Status) > 0 && !contains(f.Status, string(c.Status)) {
			continue
		}
		out = append(out, c)
	}
	// (created_at DESC, id) keyset.
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if f.CursorCreatedAt != nil && f.CursorID != nil {
		var trimmed []att.Correction
		for _, c := range out {
			if c.CreatedAt.Before(*f.CursorCreatedAt) ||
				(c.CreatedAt.Equal(*f.CursorCreatedAt) && c.ID < *f.CursorID) {
				trimmed = append(trimmed, c)
			}
		}
		out = trimmed
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakeCorrectionRepo) GetCorrection(_ context.Context, id string) (att.Correction, error) {
	c, ok := r.records[id]
	if !ok {
		return att.Correction{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakeCorrectionRepo) GetCorrectionForUpdate(_ context.Context, _ pgx.Tx, id string) (att.Correction, error) {
	return r.GetCorrection(context.Background(), id)
}

func (r *fakeCorrectionRepo) ApproveCorrection(_ context.Context, _ pgx.Tx, id string, decidedBy *string) (att.Correction, int64, error) {
	c, ok := r.records[id]
	if !ok {
		return att.Correction{}, 0, domain.ErrNotFound
	}
	if c.Status != att.CorrectionStatusPending {
		return att.Correction{}, 0, nil
	}
	c.Status = att.CorrectionStatusApplied
	c.DecidedBy = decidedBy
	dt := fixedNow
	c.DecidedAt = &dt
	r.records[id] = c
	return c, 1, nil
}

func (r *fakeCorrectionRepo) RejectCorrection(_ context.Context, _ pgx.Tx, id string, decidedBy *string, reason string) (att.Correction, int64, error) {
	c, ok := r.records[id]
	if !ok {
		return att.Correction{}, 0, domain.ErrNotFound
	}
	if c.Status != att.CorrectionStatusPending {
		return att.Correction{}, 0, nil
	}
	c.Status = att.CorrectionStatusRejected
	c.DecidedBy = decidedBy
	dt := fixedNow
	c.DecidedAt = &dt
	c.RejectReason = strp(reason)
	r.records[id] = c
	return c, 1, nil
}

func (r *fakeCorrectionRepo) CreateCorrection(_ context.Context, _ pgx.Tx, p svc.CreateCorrectionParams) (string, error) {
	id := "SWP-COR-" + strconv.Itoa(len(r.records)+1)
	r.records[id] = att.Correction{
		ID:                       id,
		AttendanceID:             p.AttendanceID,
		RequesterID:              p.RequesterID,
		CompanyID:                p.CompanyID,
		Type:                     att.CorrectionType(p.Type),
		ProposedCheckInAt:        p.ProposedCheckInAt,
		ProposedCheckOutAt:       p.ProposedCheckOutAt,
		ProposedAttendanceCodeID: p.ProposedAttendanceCodeID,
		Reason:                   p.Reason,
		EvidenceFileID:           p.EvidenceFileID,
		Status:                   att.CorrectionStatusPending,
		AttendanceShiftDate:      p.AttendanceShiftDate,
		CreatedAt:                fixedNow,
		UpdatedAt:                fixedNow,
	}
	return id, nil
}

func (r *fakeCorrectionRepo) GetPendingCorrectionForAttendance(_ context.Context, attendanceID string) (string, bool, error) {
	id, n := r.countPending(attendanceID)
	return id, n > 0, nil
}

// countPending counts PENDING corrections on one attendance (the
// CORRECTION_ALREADY_PENDING pre-check seam: the production create endpoint is
// out of web scope, so this contract test drives the guard shape directly).
func (r *fakeCorrectionRepo) countPending(attendanceID string) (string, int) {
	n := 0
	first := ""
	for _, c := range r.records {
		if c.AttendanceID == attendanceID && c.Status == att.CorrectionStatusPending {
			if first == "" || c.ID < first {
				first = c.ID
			}
			n++
		}
	}
	return first, n
}

var _ svc.CorrectionRepository = (*fakeCorrectionRepo)(nil)

// ---------------------------------------------------------------------------
// harness — mounts the real services + handler over the fakes.
// ---------------------------------------------------------------------------

type harness struct {
	router     *chi.Mux
	attendance *fakeAttendanceRepo
	correction *fakeCorrectionRepo
	idem       *stubIdempotency
	principal  auth.Principal
}

// newHarness builds the attendance slice over the fakes. principalRole is the
// caller's role; leaderCompanyID + leaderEmployeeID populate a shift_leader's
// scope + own-record identity (ignored for staff roles).
func newHarness(t *testing.T, principalRole auth.Role, leaderCompanyID, leaderEmployeeID string) *harness {
	t.Helper()
	arepo := newFakeAttendanceRepo()
	crepo := newFakeCorrectionRepo()

	asvc := svc.NewAttendanceService(arepo, &fakeTxRunner{})
	asvc.SetClock(func() time.Time { return fixedNow })
	csvc := svc.NewCorrectionService(crepo, arepo, &fakeTxRunner{})
	csvc.SetClock(func() time.Time { return fixedNow })

	handler := attendancehandler.NewHandler(asvc, csvc)
	idem := newStubIdempotency()

	h := &harness{
		attendance: arepo,
		correction: crepo,
		idem:       idem,
		principal: auth.Principal{
			UserID:     "SWP-USR-0001",
			Role:       principalRole,
			CompanyID:  leaderCompanyID,
			EmployeeID: leaderEmployeeID,
		},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), h.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// Mirror server.go: reads + all actions under RequireRole(super/hr/leader);
	// the 6 action routes wrap the idempotency middleware.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.Get("/attendance", handler.ListAttendance)
		r.Get("/attendance/{id}", handler.GetAttendance)
		r.Get("/corrections", handler.ListCorrections)
		r.Get("/corrections/{id}", handler.GetCorrection)
		r.With(idem.Handler).Post("/attendance/{id}:verify", handler.VerifyAttendance)
		r.With(idem.Handler).Post("/attendance/{id}:reject", handler.RejectAttendance)
		r.With(idem.Handler).Post("/attendance:bulk-verify", handler.BulkVerify)
		r.With(idem.Handler).Post("/attendance:bulk-reject", handler.BulkReject)
		r.With(idem.Handler).Post("/corrections/{id}:approve", handler.ApproveCorrection)
		r.With(idem.Handler).Post("/corrections/{id}:reject", handler.RejectCorrection)
	// F5.6 Manual attendance (HR-only).
	r.Get("/attendance:manual-autofill", handler.ManualAutofill)
	r.With(idem.Handler).Post("/attendance:manual-create", handler.ManualCreate)
})
// Correction CREATE: agent-inclusive group (mirrors server.go).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleShiftLeader, auth.RoleHRAdmin, auth.RoleSuperAdmin))
		r.With(idem.Handler).Post("/corrections", handler.CreateCorrection)
	})

	h.router = r
	return h
}

func (h *harness) do(method, path string, body any) *httptest.ResponseRecorder {
	return h.doWithHeaders(method, path, body, nil)
}

func (h *harness) doWithHeaders(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// seed helpers
// ---------------------------------------------------------------------------

// seedAttendance plants an attendance record directly (bypasses any create path)
// so tests can pin id/company/employee/verification-status/flags.
func (h *harness) seedAttendance(id, company, employee string, vstatus att.VerificationStatus, checkIn time.Time, flags ...att.Flag) att.Attendance {
	ci := checkIn
	a := att.Attendance{
		ID:                 id,
		EmployeeID:         employee,
		PlacementID:        "SWP-PL-5001",
		CompanyID:          company,
		ServiceLine:        att.ServiceLineFacilityServices,
		SiteID:             "SWP-SITE-001",
		PositionID:         "SWP-POS-001",
		CheckInAt:          &ci,
		WFO:                true,
		Status:             att.StatusLate,
		VerificationStatus: vstatus,
		Flags:              flags,
		CreatedAt:          checkIn,
		UpdatedAt:          checkIn,
		EmployeeName:       strp("Agent " + employee),
		CompanyName:        strp("Company " + company),
		SiteName:           strp("Site of " + company),
		PositionName:       strp("Position of " + employee),
	}
	h.attendance.records[id] = a
	return a
}

// ciOf derefs the nullable check_in_at (zero time for a true ABSENT record).
func ciOf(a att.Attendance) time.Time {
	if a.CheckInAt == nil {
		return time.Time{}
	}
	return *a.CheckInAt
}

// seedCorrection plants a correction record directly. shiftDate is the
// OUTSIDE_CORRECTION_WINDOW basis; corrType drives the diff[] field.
func (h *harness) seedCorrection(id, attID, company string, status att.CorrectionStatus, shiftDate time.Time, corrType att.CorrectionType) att.Correction {
	c := att.Correction{
		ID:                  id,
		AttendanceID:        attID,
		RequesterID:         "SWP-EMP-1042",
		CompanyID:           company,
		Type:                corrType,
		Reason:              "Lupa clock-out, sudah pulang.",
		Status:              status,
		AttendanceShiftDate: shiftDate,
		OriginalSnapshot:    map[string]any{},
		CreatedAt:           shiftDate,
		UpdatedAt:           shiftDate,
		RequesterName:       strp("Agent SWP-EMP-1042"),
		CompanyName:         strp("Company " + company),
	}
	h.correction.records[id] = c
	return c
}

// --- small helpers ---

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func hasFlag(fs []att.Flag, want att.Flag) bool {
	for _, f := range fs {
		if f == want {
			return true
		}
	}
	return false
}

const empNoPlacement = "SWP-EMP-NOPL" // employee with no active placement (422 test)

// silence unused in case a helper is dropped during edits.
var _ = itoa
