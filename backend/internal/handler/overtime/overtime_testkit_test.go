// Package overtime_test — E7 overtime + holiday contract tests (the drift gate
// replacing server codegen). This testkit mirrors the Phase-8 leave harness
// (internal/handler/leave/leave_testkit_test.go) EXACTLY:
//
//   - fakeTx (Exec no-op so audit.Record + InsertOvertimeApproval work inside
//     InTx) + fakeTxRunner,
//   - in-memory fake repos implementing the 09-02 service ports
//     (svc.OvertimeRepository + svc.RuleRepository + svc.HolidayRepository +
//     svc.SchedulePort) over shared mutable maps so the state-machine transitions
//   - ForUpdate locks observe each other,
//   - newHarness(role, companyID, employeeID) that builds the REAL OvertimeService
//   - HolidayService + the real overtime handler.Handler and mounts them on a
//     chi.Router with a mutable-principal closure middleware (swap role/company/
//     employee per case) + a stubIdempotency at the server.go router position.
//
// Assertions hit the real handler over the fakes and check the openapi response
// shape + status code + every contract error code.
package overtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	overtimehandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/overtime"
	schedulingsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// ---------------------------------------------------------------------------
// fakeTx — only Exec is needed (audit.Record); every other method panics.
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

// ---------------------------------------------------------------------------
// shared decode helpers — decodeBody snapshots rr.Body.Bytes() so a single
// response re-decodes for errCode + errFields (decision [08-03]).
// ---------------------------------------------------------------------------

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

func dataObject(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	body := decodeBody(t, rr)
	d, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("response has no data object: %v", body)
	}
	return d
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

func errFields(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	e := errObject(t, decodeBody(t, rr))
	f, _ := e["fields"].(map[string]any)
	return f
}

func strOf(v any) string {
	s, _ := v.(string)
	return s
}

func strp(s string) *string { return &s }

// fixedNow is the deterministic clock for all overtime tests (12:00 WIB on
// 2026-06-04 — mirrors the leave/attendance/scheduling test clock).
var fixedNow = time.Date(2026, 6, 4, 5, 0, 0, 0, time.UTC)

// ymd is a YYYY-MM-DD date in UTC (matches the seed fixture convention).
func ymd(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// in-memory idempotency middleware — mirrors the real Postgres-backed contract
// (CONVENTIONS §13) at the router boundary, scoped by principal user id. The
// action routes wrap it exactly as server.go does.
// ---------------------------------------------------------------------------

type stubIdempotency struct {
	mu    sync.Mutex
	store map[string]idemEntry
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

func hashBytes(b []byte) string { return string(b) }

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
// fakeOvertimeRepo — in-memory svc.OvertimeRepository + svc.RuleRepository over
// shared maps.
//
// UpdateOvertimeStatus mutates the request map so the *ForUpdate re-read +
// list/get observe the new state. Approval rows accumulate per OT to drive the
// timeline. FindOvertimeRule serves the OT_BELOW_MIN + reference-multiplier
// read-through (line-scoped wins over the global default).
// ---------------------------------------------------------------------------

type fakeOvertimeRepo struct {
	records   map[string]dom.Overtime
	approvals map[string][]dom.OvertimeApproval
	rules     map[string]svc.OvertimeRule // keyed by service_line id; "" = global default
}

func newFakeOvertimeRepo() *fakeOvertimeRepo {
	return &fakeOvertimeRepo{
		records:   map[string]dom.Overtime{},
		approvals: map[string][]dom.OvertimeApproval{},
		rules:     map[string]svc.OvertimeRule{},
	}
}

func (r *fakeOvertimeRepo) ListOvertime(_ context.Context, f svc.OvertimeFilter) ([]dom.Overtime, error) {
	var out []dom.Overtime
	for _, rec := range r.records {
		if f.CompanyID != nil && deref(rec.CompanyID) != *f.CompanyID {
			continue
		}
		if f.EmployeeID != nil && rec.EmployeeID != *f.EmployeeID {
			continue
		}
		if f.Status != nil && string(rec.Status) != *f.Status {
			continue
		}
		if len(f.StatusIn) > 0 && !contains(f.StatusIn, string(rec.Status)) {
			continue
		}
		if f.Tier != nil && string(rec.DayType) != *f.Tier {
			continue
		}
		if f.Source != nil && string(rec.Source) != *f.Source {
			continue
		}
		out = append(out, rec)
	}
	// (created_at DESC, id) keyset — newest first.
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if f.CursorCreated != nil && f.CursorID != nil {
		var trimmed []dom.Overtime
		for _, rec := range out {
			if rec.CreatedAt.Before(*f.CursorCreated) ||
				(rec.CreatedAt.Equal(*f.CursorCreated) && rec.ID < *f.CursorID) {
				trimmed = append(trimmed, rec)
			}
		}
		out = trimmed
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakeOvertimeRepo) GetOvertime(_ context.Context, id string) (dom.Overtime, error) {
	rec, ok := r.records[id]
	if !ok {
		return dom.Overtime{}, domain.ErrNotFound
	}
	return rec, nil
}

func (r *fakeOvertimeRepo) GetOvertimeForUpdate(_ context.Context, _ pgx.Tx, id string) (dom.Overtime, error) {
	return r.GetOvertime(context.Background(), id)
}

func (r *fakeOvertimeRepo) UpdateOvertimeStatus(_ context.Context, _ pgx.Tx, id string, status dom.OvertimeStatus) (dom.Overtime, error) {
	rec, ok := r.records[id]
	if !ok {
		return dom.Overtime{}, domain.ErrNotFound
	}
	rec.Status = status
	rec.UpdatedAt = fixedNow
	r.records[id] = rec
	return rec, nil
}

func (r *fakeOvertimeRepo) InsertOvertimeApproval(_ context.Context, _ pgx.Tx, p svc.ApprovalRow) (dom.OvertimeApproval, error) {
	a := dom.OvertimeApproval{
		Level:        p.Level,
		Decision:     p.Decision,
		ApproverID:   p.ApproverID,
		ApproverName: p.ApproverName,
		Reason:       p.Reason,
		DecidedAt:    fixedNow,
	}
	r.approvals[p.OvertimeID] = append(r.approvals[p.OvertimeID], a)
	return a, nil
}

func (r *fakeOvertimeRepo) ListOvertimeApprovals(_ context.Context, overtimeID string) ([]dom.OvertimeApproval, error) {
	return r.approvals[overtimeID], nil
}

// FindOvertimeRule serves svc.RuleRepository: the line-scoped active rule wins
// over the NULL-line global default (OR-2), else the global default.
func (r *fakeOvertimeRepo) FindOvertimeRule(_ context.Context, serviceLineID *string) (svc.OvertimeRule, error) {
	if serviceLineID != nil {
		if rule, ok := r.rules[*serviceLineID]; ok {
			return rule, nil
		}
	}
	if rule, ok := r.rules[""]; ok {
		return rule, nil
	}
	return svc.OvertimeRule{}, domain.ErrNotFound
}

var (
	_ svc.OvertimeRepository = (*fakeOvertimeRepo)(nil)
	_ svc.RuleRepository     = (*fakeOvertimeRepo)(nil)
)

// ---------------------------------------------------------------------------
// fakeHolidayRepo — in-memory svc.HolidayRepository over shared maps.
//
// inUse is configurable per holiday id so the HOLIDAY_IN_USE delete guard +
// in_use_by_overtime flag are driven directly. Insert/Update/SoftDelete mutate
// in place so List/Get observe the new state. byDateCategory drives the
// HOLIDAY_DATE_CLASH pre-check.
// ---------------------------------------------------------------------------

type fakeHolidayRepo struct {
	byID  map[string]dom.Holiday
	inUse map[string]int64 // keyed by holiday id (HOLIDAY_IN_USE source)
	seq   int
}

func newFakeHolidayRepo() *fakeHolidayRepo {
	return &fakeHolidayRepo{byID: map[string]dom.Holiday{}, inUse: map[string]int64{}}
}

func (r *fakeHolidayRepo) ListHolidays(_ context.Context, f svc.HolidayFilter) ([]dom.Holiday, error) {
	var out []dom.Holiday
	for _, h := range r.byID {
		if f.Category != nil && string(h.Category) != *f.Category {
			continue
		}
		if f.Year != nil && h.Date.Year() != *f.Year {
			continue
		}
		out = append(out, h)
	}
	// (holiday_date ASC, id) keyset.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date.Equal(out[j].Date) {
			return out[i].ID < out[j].ID
		}
		return out[i].Date.Before(out[j].Date)
	})
	if f.CursorDate != nil && f.CursorID != nil {
		var trimmed []dom.Holiday
		for _, h := range out {
			if h.Date.After(*f.CursorDate) ||
				(h.Date.Equal(*f.CursorDate) && h.ID > *f.CursorID) {
				trimmed = append(trimmed, h)
			}
		}
		out = trimmed
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakeHolidayRepo) GetHoliday(_ context.Context, id string) (dom.Holiday, error) {
	h, ok := r.byID[id]
	if !ok {
		return dom.Holiday{}, domain.ErrNotFound
	}
	return h, nil
}

func (r *fakeHolidayRepo) GetHolidayByDateCategory(_ context.Context, date time.Time, category string) (dom.Holiday, error) {
	for _, h := range r.byID {
		if h.Date.Equal(date) && string(h.Category) == category {
			return h, nil
		}
	}
	return dom.Holiday{}, domain.ErrNotFound
}

func (r *fakeHolidayRepo) GetHolidayForDate(_ context.Context, date time.Time) (dom.Holiday, error) {
	for _, h := range r.byID {
		if h.Date.Equal(date) {
			return h, nil
		}
	}
	return dom.Holiday{}, domain.ErrNotFound
}

func (r *fakeHolidayRepo) InsertHoliday(_ context.Context, _ pgx.Tx, p svc.HolidayWriteParams) (dom.Holiday, error) {
	id := ""
	if p.ID != nil {
		id = *p.ID
	} else {
		r.seq++
		id = "SWP-HOL-" + itoa(9000+r.seq)
	}
	h := dom.Holiday{
		ID:                     id,
		Name:                   p.Name,
		Date:                   p.Date,
		Category:               dom.HolidayCategory(p.Category),
		Recurring:              p.Recurring,
		ApplicableServiceLines: p.ApplicableServiceLines,
		CreatedAt:              fixedNow,
		UpdatedAt:              fixedNow,
	}
	r.byID[id] = h
	return h, nil
}

func (r *fakeHolidayRepo) UpdateHoliday(_ context.Context, _ pgx.Tx, id string, p svc.HolidayUpdateParams) (dom.Holiday, error) {
	h, ok := r.byID[id]
	if !ok {
		return dom.Holiday{}, domain.ErrNotFound
	}
	if p.Name != nil {
		h.Name = *p.Name
	}
	if p.Date != nil {
		h.Date = *p.Date
	}
	if p.Category != nil {
		h.Category = dom.HolidayCategory(*p.Category)
	}
	if p.Recurring != nil {
		h.Recurring = *p.Recurring
	}
	if p.ApplicableServiceLines != nil {
		h.ApplicableServiceLines = p.ApplicableServiceLines
	}
	h.UpdatedAt = fixedNow
	r.byID[id] = h
	return h, nil
}

func (r *fakeHolidayRepo) SoftDeleteHoliday(_ context.Context, _ pgx.Tx, id string) (string, error) {
	if _, ok := r.byID[id]; !ok {
		return "", domain.ErrNotFound
	}
	delete(r.byID, id)
	return id, nil
}

func (r *fakeHolidayRepo) CountOvertimeUsingHoliday(_ context.Context, holidayID string) (int64, error) {
	return r.inUse[holidayID], nil
}

var _ svc.HolidayRepository = (*fakeHolidayRepo)(nil)

// ---------------------------------------------------------------------------
// fakeScheduleRepo — in-memory svc.SchedulePort (the WORKDAY/RESTDAY day_type
// classification surface). live[employeeID|date] returns a live schedule entry
// so a scheduled shift classifies WORKDAY; absent → RESTDAY.
// ---------------------------------------------------------------------------

type fakeScheduleRepo struct {
	live map[string]schedulingsvc.LiveEntry
}

func newFakeScheduleRepo() *fakeScheduleRepo {
	return &fakeScheduleRepo{live: map[string]schedulingsvc.LiveEntry{}}
}

func liveKey(employeeID string, date time.Time) string {
	return employeeID + "|" + date.Format("2006-01-02")
}

func (r *fakeScheduleRepo) FindLiveEntryForAgentDate(_ context.Context, employeeID string, date time.Time) (schedulingsvc.LiveEntry, error) {
	if e, ok := r.live[liveKey(employeeID, date)]; ok {
		return e, nil
	}
	return schedulingsvc.LiveEntry{}, domain.ErrNotFound
}

var _ svc.SchedulePort = (*fakeScheduleRepo)(nil)

// ---------------------------------------------------------------------------
// harness — mounts the REAL services + handler over the fakes.
// ---------------------------------------------------------------------------

type harness struct {
	router    *chi.Mux
	overtime  *fakeOvertimeRepo
	holiday   *fakeHolidayRepo
	schedule  *fakeScheduleRepo
	idem      *stubIdempotency
	otSvc     *svc.OvertimeService // the REAL service, exposed for the EnforceMinMinutes/ClassifyDayType seams
	principal auth.Principal
}

// newHarness builds the E7 overtime slice over the fakes. principalRole is the
// caller's role; companyID + employeeID populate a shift_leader's scope + own-
// record identity (employeeID also drives the SELF_APPROVAL_FORBIDDEN guard).
func newHarness(t *testing.T, principalRole auth.Role, companyID, employeeID string) *harness {
	t.Helper()
	orepo := newFakeOvertimeRepo()
	hrepo := newFakeHolidayRepo()
	srepo := newFakeScheduleRepo()

	osvc := svc.NewOvertimeService(orepo, orepo, hrepo, srepo, &fakeTxRunner{})
	osvc.SetClock(func() time.Time { return fixedNow })
	hsvc := svc.NewHolidayService(hrepo, &fakeTxRunner{})
	hsvc.SetClock(func() time.Time { return fixedNow })

	handler := overtimehandler.NewHandler(osvc, hsvc)
	idem := newStubIdempotency()

	h := &harness{
		overtime: orepo,
		holiday:  hrepo,
		schedule: srepo,
		idem:     idem,
		otSvc:    osvc,
		principal: auth.Principal{
			UserID:     "SWP-USR-0001",
			Role:       principalRole,
			CompanyID:  companyID,
			EmployeeID: employeeID,
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

	// Mirror server.go: reads + L1 + reject + confirm + withdraw + bulk + holiday
	// reads under RequireRole(super/hr/leader); final + holiday writes under
	// RequireRole(super/hr); action routes wrap the idempotency middleware.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.Get("/overtime", handler.ListOvertime)
		r.Get("/overtime/{id}", handler.GetOvertime)
		r.Get("/holidays", handler.ListHolidays)
		r.With(idem.Handler).Post("/overtime/{id}:confirm", handler.Confirm)
		r.With(idem.Handler).Post("/overtime/{id}:approve-l1", handler.ApproveL1)
		r.With(idem.Handler).Post("/overtime/{id}:reject", handler.Reject)
		r.With(idem.Handler).Post("/overtime/{id}:withdraw", handler.Withdraw)
		r.With(idem.Handler).Post("/overtime:bulk-approve", handler.BulkApprove)
		r.With(idem.Handler).Post("/overtime:bulk-reject", handler.BulkReject)
	})
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.With(idem.Handler).Post("/overtime/{id}:approve-final", handler.ApproveFinal)
		r.With(idem.Handler).Post("/holidays", handler.CreateHoliday)
		r.With(idem.Handler).Patch("/holidays/{id}", handler.UpdateHoliday)
		r.Delete("/holidays/{id}", handler.DeleteHoliday)
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

// seedOvertime plants an overtime row directly (pin id/company/employee/status/
// day_type/source). Defaults a 210-minute REQUESTED record with denorm names so
// the calculation + employee/company refs serialize.
func (h *harness) seedOvertime(id, company, employee string, status dom.OvertimeStatus, dayType dom.OvertimeTier) dom.Overtime {
	c := company
	line := "SWP-SVC-001"
	rec := dom.Overtime{
		ID:                   id,
		EmployeeID:           employee,
		EmployeeName:         strp("Agent " + employee),
		CompanyID:            &c,
		CompanyName:          strp("Company " + company),
		PlacementID:          "SWP-PL-5001",
		ServiceLineID:        &line,
		WorkDate:             ymd(2026, time.June, 2),
		ActualStartTime:      strp("15:00"),
		ActualEndTime:        strp("18:30"),
		Source:               dom.OvertimeSourceRequested,
		Status:               status,
		DayType:              dayType,
		WorkedMinutes:        210,
		CountedMinutes:       210,
		MinMinutesThreshold:  30,
		FlaggedNoPreapproval: false,
		Reason:               strp("Penyelesaian backlog."),
		CreatedAt:            ymd(2026, time.June, 1),
		UpdatedAt:            ymd(2026, time.June, 1),
	}
	h.overtime.records[id] = rec
	return rec
}

// seedRule plants the applicable E2 overtime_rule for OT_BELOW_MIN + the
// reference multiplier. lineID "" = the global default.
func (h *harness) seedRule(lineID string, minMinutes int) {
	h.overtime.rules[lineID] = svc.OvertimeRule{
		ID:          "SWP-OTR-001",
		WeekdayRate: 1.5,
		RestdayRate: 2.0,
		HolidayRate: 3.0,
		MinMinutes:  minMinutes,
	}
}

// seedHoliday plants a holiday row directly (HOLIDAY_IN_USE drives the delete
// guard + the in_use_by_overtime flag).
func (h *harness) seedHoliday(id, name string, date time.Time, category dom.HolidayCategory, inUse int64) dom.Holiday {
	hol := dom.Holiday{
		ID:                     id,
		Name:                   name,
		Date:                   date,
		Category:               category,
		ApplicableServiceLines: []string{},
		CreatedAt:              fixedNow,
		UpdatedAt:              fixedNow,
	}
	h.holiday.byID[id] = hol
	h.holiday.inUse[id] = inUse
	return hol
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

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
