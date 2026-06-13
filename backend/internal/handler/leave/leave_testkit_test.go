// Package leave_test — E6 leave contract tests (the drift gate replacing server
// codegen). This testkit mirrors the Phase-7 attendance harness
// (internal/handler/attendance/attendance_testkit_test.go) EXACTLY:
//
//   - fakeTx (Exec no-op so audit.Record works inside InTx) + fakeTxRunner,
//   - in-memory fake repos implementing the 08-02 service ports
//     (svc.LeaveRepository + svc.QuotaRepository) over shared mutable maps so the
//     state-machine transitions + ForUpdate locks observe each other,
//   - a fakeScheduleRepo implementing the INV-3 svc.SchedulePort that RECORDS the
//     CancelScheduleEntriesForLeave + InsertApprovedLeaveDay calls (so the
//     loop-closer side-effects are asserted at the service-contract level),
//   - newHarness(role, companyID, employeeID) that builds the REAL LeaveService /
//     QuotaService / CalendarService + the real leavehandler.Handler and mounts them
//     on a chi.Router with a mutable-principal closure middleware (swap role/company/
//     employee per case), mirroring server.go's RequireRole + Idempotency positions.
//
// Assertions hit the real handler over the fakes and check the openapi response
// shape + status code + every contract error code.
package leave_test

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
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	leavehandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
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
// shared helpers
// ---------------------------------------------------------------------------

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	// Read from a snapshot of the buffer so a body can be decoded more than once
	// (e.g. errCode + errFields on the same response).
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

// fixedNow is the deterministic clock for all leave tests (12:00 WIB on
// 2026-06-04 — mirrors the attendance/scheduling test clock).
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
// fakeLeaveRepo — in-memory svc.LeaveRepository over shared maps.
//
// UpdateLeaveRequestStatus mutates the request map so the *ForUpdate re-read +
// list/get observe the new state. Approval rows accumulate per request to drive
// the timeline. Calendar entries are seeded directly (status-filtered on read).
// ---------------------------------------------------------------------------

type fakeLeaveRepo struct {
	requests   map[string]dom.LeaveRequest
	approvals  map[string][]dom.LeaveApproval
	leaveTypes map[string]svc.LeaveTypeInfo
	calendar   []dom.LeaveCalendarEntry
	createSeq  int
}

func newFakeLeaveRepo() *fakeLeaveRepo {
	return &fakeLeaveRepo{
		requests:   map[string]dom.LeaveRequest{},
		approvals:  map[string][]dom.LeaveApproval{},
		leaveTypes: map[string]svc.LeaveTypeInfo{},
	}
}

func (r *fakeLeaveRepo) ListLeaveRequests(_ context.Context, f svc.RequestFilter) ([]dom.LeaveRequest, error) {
	var out []dom.LeaveRequest
	for _, req := range r.requests {
		if f.CompanyID != nil && deref(req.CompanyID) != *f.CompanyID {
			continue
		}
		if f.EmployeeID != nil && req.EmployeeID != *f.EmployeeID {
			continue
		}
		if f.Status != nil && string(req.Status) != *f.Status {
			continue
		}
		if len(f.StatusIn) > 0 && !contains(f.StatusIn, string(req.Status)) {
			continue
		}
		out = append(out, req)
	}
	// (created_at DESC, id) keyset — newest first.
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if f.CursorCreated != nil && f.CursorID != nil {
		var trimmed []dom.LeaveRequest
		for _, req := range out {
			if req.CreatedAt.Before(*f.CursorCreated) ||
				(req.CreatedAt.Equal(*f.CursorCreated) && req.ID < *f.CursorID) {
				trimmed = append(trimmed, req)
			}
		}
		out = trimmed
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakeLeaveRepo) GetLeaveRequest(_ context.Context, id string) (dom.LeaveRequest, error) {
	req, ok := r.requests[id]
	if !ok {
		return dom.LeaveRequest{}, domain.ErrNotFound
	}
	return req, nil
}

func (r *fakeLeaveRepo) GetLeaveRequestForUpdate(_ context.Context, _ pgx.Tx, id string) (dom.LeaveRequest, error) {
	return r.GetLeaveRequest(context.Background(), id)
}

// createSeq drives a deterministic SWP-LR-* id for inserted DRAFT requests.
func (r *fakeLeaveRepo) CreateLeaveRequest(_ context.Context, _ pgx.Tx, p svc.CreateLeaveRequestParams) (dom.LeaveRequest, error) {
	r.createSeq++
	id := "SWP-LR-" + itoa(9000+r.createSeq)
	req := dom.LeaveRequest{
		ID:             id,
		EmployeeID:     p.EmployeeID,
		PlacementID:    p.PlacementID,
		CompanyID:      p.CompanyID,
		LeaveTypeID:    p.LeaveTypeID,
		StartDate:      p.StartDate,
		EndDate:        p.EndDate,
		DurationDays:   p.DurationDays,
		Reason:         p.Reason,
		Notes:          p.Notes,
		Status:         p.Status,
		DelegateID:     p.DelegateID,
		DocumentFileID: p.DocumentFileID,
		Backdated:      p.Backdated,
		Routing:        dom.LeaveRouting{NoLeader: p.NoLeader, AssignedLeaderID: p.AssignedLeaderID},
		CreatedBy:      p.CreatedBy,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		EmployeeName:   strp("Agent " + p.EmployeeID),
	}
	r.requests[id] = req
	return req, nil
}

func (r *fakeLeaveRepo) CheckOverlappingLeave(_ context.Context, employeeID string, start, end time.Time) (bool, error) {
	for _, req := range r.requests {
		if req.EmployeeID != employeeID {
			continue
		}
		if req.Status == dom.LeaveStatusRejected || req.Status == dom.LeaveStatusCancelled {
			continue
		}
		// overlap iff start <= other.end AND end >= other.start.
		if !start.After(req.EndDate) && !end.Before(req.StartDate) {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeLeaveRepo) UpdateLeaveRequestStatus(_ context.Context, _ pgx.Tx, p svc.UpdateStatusParams) (dom.LeaveRequest, error) {
	req, ok := r.requests[p.ID]
	if !ok {
		return dom.LeaveRequest{}, domain.ErrNotFound
	}
	req.Status = p.Status
	req.Routing.NoLeader = p.NoLeader
	req.Routing.AssignedLeaderID = p.AssignedLeaderID
	req.ClockInConflict = p.ClockInConflict
	req.BalanceCheck.RequestedDays = p.BalanceRequestedDays
	req.BalanceCheck.RemainingDaysAtCheck = p.BalanceRemainingAtCheck
	if p.BalanceRequiresOverride != nil {
		req.BalanceCheck.RequiresOverride = *p.BalanceRequiresOverride
	}
	req.UpdatedAt = fixedNow
	r.requests[p.ID] = req
	return req, nil
}

func (r *fakeLeaveRepo) UpdateLeaveRequestDates(_ context.Context, _ pgx.Tx, id string, start, end time.Time, days int) (dom.LeaveRequest, error) {
	req, ok := r.requests[id]
	if !ok {
		return dom.LeaveRequest{}, domain.ErrNotFound
	}
	req.StartDate, req.EndDate, req.DurationDays = start, end, days
	req.UpdatedAt = fixedNow
	r.requests[id] = req
	return req, nil
}

func (r *fakeLeaveRepo) SetBalanceSnapshot(_ context.Context, _ pgx.Tx, p svc.BalanceSnapshotParams) error {
	req, ok := r.requests[p.ID]
	if !ok {
		return domain.ErrNotFound
	}
	req.BalanceCheck.RequestedDays = p.RequestedDays
	req.BalanceCheck.RemainingDaysAtCheck = p.RemainingAtCheck
	if p.RequiresOverride != nil {
		req.BalanceCheck.RequiresOverride = *p.RequiresOverride
	}
	r.requests[p.ID] = req
	return nil
}

func (r *fakeLeaveRepo) InsertLeaveApproval(_ context.Context, _ pgx.Tx, p svc.ApprovalRow) (dom.LeaveApproval, error) {
	a := dom.LeaveApproval{
		ID:             int64(len(r.approvals[p.LeaveRequestID]) + 1),
		LeaveRequestID: p.LeaveRequestID,
		Stage:          p.Stage,
		Decision:       p.Decision,
		ActorID:        p.ActorID,
		ActorRole:      p.ActorRole,
		DecisionNote:   p.DecisionNote,
		RejectReason:   p.RejectReason,
		IsOverride:     p.IsOverride,
		OverrideReason: p.OverrideReason,
		OccurredAt:     fixedNow,
	}
	r.approvals[p.LeaveRequestID] = append(r.approvals[p.LeaveRequestID], a)
	return a, nil
}

func (r *fakeLeaveRepo) ListLeaveApprovalsForRequest(_ context.Context, id string) ([]dom.LeaveApproval, error) {
	return r.approvals[id], nil
}

func (r *fakeLeaveRepo) GetLeaveType(_ context.Context, id string) (svc.LeaveTypeInfo, error) {
	lt, ok := r.leaveTypes[id]
	if !ok {
		return svc.LeaveTypeInfo{}, domain.ErrNotFound
	}
	return lt, nil
}

func (r *fakeLeaveRepo) ListCalendarEntries(_ context.Context, f svc.CalendarFilter, statusIn []string, from, to time.Time) ([]dom.LeaveCalendarEntry, error) {
	var out []dom.LeaveCalendarEntry
	for _, e := range r.calendar {
		if !contains(statusIn, string(e.Status)) {
			continue
		}
		if f.CompanyID != nil && deref(e.CompanyID) != *f.CompanyID {
			continue
		}
		// overlap with [from,to].
		if e.EndDate.Before(from) || e.StartDate.After(to) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

var _ svc.LeaveRepository = (*fakeLeaveRepo)(nil)

// ---------------------------------------------------------------------------
// fakeQuotaRepo — in-memory svc.QuotaRepository (per-type balance read only).
// ---------------------------------------------------------------------------

type fakeQuotaRepo struct {
	balances map[string][]dom.TypeBalance // employee_id → per-type rows
}

func newFakeQuotaRepo() *fakeQuotaRepo {
	return &fakeQuotaRepo{balances: map[string][]dom.TypeBalance{}}
}

func (r *fakeQuotaRepo) ListEmployeeTypeBalances(_ context.Context, employeeID, _, _ string) ([]dom.TypeBalance, error) {
	return r.balances[employeeID], nil
}

var _ svc.QuotaRepository = (*fakeQuotaRepo)(nil)

// ---------------------------------------------------------------------------
// fakeScheduleRepo — in-memory svc.SchedulePort (the INV-3 loop-closer surface).
//
// Records every CancelScheduleEntriesForLeave + InsertApprovedLeaveDay call so a
// test can assert the loop-closer fired (and how many schedule entries it
// cancelled + approved-leave-days it inserted). cancelReturns drives the
// schedule_impact[] payload surfaced on the response.
// ---------------------------------------------------------------------------

type fakeScheduleRepo struct {
	cancelReturns map[string][]svc.ScheduleImpact // keyed by employee id
	cancelCalls   []string                        // employee ids passed to Cancel
	insertedDays  []insertedLeaveDay
	// duration overrides the F6.2 server-computed leave duration. When >0 it is
	// returned verbatim; when 0 the fake falls back to the inclusive calendar-day
	// count of [start,end] (the simplest deterministic stand-in for "rostered days
	// minus holidays" — the real query lives in the scheduling repo).
	duration int
}

type insertedLeaveDay struct {
	EmployeeID     string
	Date           time.Time
	LeaveRequestID string
	LeaveType      string
}

func newFakeScheduleRepo() *fakeScheduleRepo {
	return &fakeScheduleRepo{cancelReturns: map[string][]svc.ScheduleImpact{}}
}

func (r *fakeScheduleRepo) CancelScheduleEntriesForLeave(_ context.Context, _ pgx.Tx, employeeID string, _, _ time.Time) ([]svc.ScheduleImpact, error) {
	r.cancelCalls = append(r.cancelCalls, employeeID)
	return r.cancelReturns[employeeID], nil
}

func (r *fakeScheduleRepo) InsertApprovedLeaveDay(_ context.Context, _ pgx.Tx, employeeID string, date time.Time, leaveRequestID, leaveType string) error {
	r.insertedDays = append(r.insertedDays, insertedLeaveDay{employeeID, date, leaveRequestID, leaveType})
	return nil
}

func (r *fakeScheduleRepo) CountLeaveDuration(_ context.Context, _ string, start, end time.Time) (int, error) {
	if r.duration > 0 {
		return r.duration, nil
	}
	return int(end.Sub(start).Hours()/24) + 1, nil
}

var _ svc.SchedulePort = (*fakeScheduleRepo)(nil)


// ---------------------------------------------------------------------------
// harness — mounts the REAL services + handler over the fakes.
// ---------------------------------------------------------------------------

type harness struct {
	router      *chi.Mux
	leave       *fakeLeaveRepo
	quota       *fakeQuotaRepo
	meterStore  *fakeMeterStore
	meterReader *fakeMeterReader
	schedule    *fakeScheduleRepo
	idem        *stubIdempotency
	principal   auth.Principal
}

// newHarness builds the E6 leave slice over the fakes. principalRole is the
// caller's role; companyID + employeeID populate a shift_leader's scope + own-
// record identity (employeeID also drives the self-approve guard for staff).
func newHarness(t *testing.T, principalRole auth.Role, companyID, employeeID string) *harness {
	t.Helper()
	lrepo := newFakeLeaveRepo()
	qrepo := newFakeQuotaRepo()
	mstore := newFakeMeterStore()
	mreader := newFakeMeterReader()
	srepo := newFakeScheduleRepo()

	lsvc := svc.NewLeaveService(lrepo, srepo, &fakeTxRunner{})
	lsvc.SetClock(func() time.Time { return fixedNow })
	lsvc.SetMeter(svc.NewQuotaMeter(mstore, mreader)) // per-type ledger (2026-06-12)
	qsvc := svc.NewQuotaService(qrepo, &fakeTxRunner{})
	qsvc.SetClock(func() time.Time { return fixedNow })
	csvc := svc.NewCalendarService(lrepo)
	csvc.SetClock(func() time.Time { return fixedNow })

	handler := leavehandler.NewHandler(lsvc, qsvc, csvc)
	idem := newStubIdempotency()

	h := &harness{
		leave:       lrepo,
		quota:       qrepo,
		meterStore:  mstore,
		meterReader: mreader,
		schedule:    srepo,
		idem:        idem,
		principal: auth.Principal{
			UserID:     "SWP-USR-0001",
			Role:       principalRole,
			CompanyID:  companyID,
			EmployeeID: employeeID,
		},
	}
	// A lead's company scope lives in CompanyIDs (the middleware fills it from
	// lead_assignments). Mirror that so rbac.GuardCompany scopes the lead.
	if principalRole == auth.RoleLead && companyID != "" {
		h.principal.CompanyIDs = []string{companyID}
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), h.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// Mirror server.go: reads + L1 + reject + calendar + quota-list under
	// RequireRole(super/hr/leader); final/override + quota-writes under
	// RequireRole(super/hr); action routes wrap the idempotency middleware.
	// Reads: super/hr/leader/AGENT (agent self-scoped in service).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Get("/leave-requests", handler.ListLeaveRequests)
		r.Get("/leave-requests/{id}", handler.GetLeaveRequest)
		r.Get("/leave-balances/by-employee/{employee_id}/types", handler.GetEmployeeTypeBalances)
	})
	// Staff-only reads + L1/reject/cancel-approved: super/hr/leader.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.With(idem.Handler).Post("/leave-requests/{id}:approve-l1", handler.ApproveLeaveRequestL1)
		r.With(idem.Handler).Post("/leave-requests/{id}:reject", handler.RejectLeaveRequest)
		r.Get("/leave-calendar", handler.GetLeaveCalendar)
		r.With(idem.Handler).Post("/leave-requests/{id}:cancel-approved", handler.CancelApprovedLeaveRequest)
	})
	// Agent file-a-request + own-request actions: agent/hr/super.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleHRAdmin, auth.RoleSuperAdmin))
		r.With(idem.Handler).Post("/leave-requests", handler.CreateLeaveRequest)
		r.With(idem.Handler).Post("/leave-requests/{id}:submit", handler.SubmitLeaveRequest)
		r.With(idem.Handler).Post("/leave-requests/{id}:cancel", handler.CancelLeaveRequest)
	})
	// L2 final/override admits lead (scoped via GuardCompany in-service).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleLead))
		r.With(idem.Handler).Post("/leave-requests/{id}:approve-final", handler.ApproveLeaveRequestFinal)
		r.With(idem.Handler).Post("/leave-requests/{id}:approve-override", handler.ApproveLeaveRequestOverride)
	})
	// HR per-type quota adjust (LQ-6).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.With(idem.Handler).Post("/leave-quotas:adjust-entitled", handler.AdjustTypeQuota)
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

// seedRequest plants a leave_requests row directly (pin id/company/employee/
// status/dates/duration). Defaults annual leave type SWP-LT-001 + denorm names.
func (h *harness) seedRequest(id, company, employee string, status dom.LeaveStatus, start, end time.Time, days int) dom.LeaveRequest {
	c := company
	req := dom.LeaveRequest{
		ID:            id,
		EmployeeID:    employee,
		CompanyID:     &c,
		LeaveTypeID:   "SWP-LT-001",
		StartDate:     start,
		EndDate:       end,
		DurationDays:  days,
		Reason:        strp("Keperluan keluarga."),
		Status:        status,
		Routing:       dom.LeaveRouting{},
		CreatedAt:     start.Add(-24 * time.Hour),
		UpdatedAt:     start.Add(-24 * time.Hour),
		EmployeeName:  strp("Agent " + employee),
		CompanyName:   strp("Company " + company),
		LeaveTypeName: strp("Cuti Tahunan"),
		LeaveTypeCode: strp("ANNUAL"),
	}
	h.leave.requests[id] = req
	return req
}

// seedLeaveType registers an E2 leave-type (is_annual drives the quota gate).
func (h *harness) seedLeaveType(id, code string, isAnnual bool) {
	h.leave.leaveTypes[id] = svc.LeaveTypeInfo{ID: id, Code: code, Name: code, IsAnnual: isAnnual}
}

// seedLeaveTypeFull registers a leave-type with the F6.2 create-time gate flags
// (is_document_required / allows_backdated).
func (h *harness) seedLeaveTypeFull(id, code string, isAnnual, docRequired, allowsBackdated bool) {
	h.leave.leaveTypes[id] = svc.LeaveTypeInfo{
		ID: id, Code: code, Name: code, IsAnnual: isAnnual,
		IsDocumentRequired: docRequired, AllowsBackdated: allowsBackdated,
	}
}

// seedGrant seeds an employee's leave balance under the per-type ledger: it sets the
// annual entitlement (so Reserve auto-opens the ANNUAL_POOL window at `amount`), and —
// when consumed/pending are non-zero — pre-opens the current-year window pre-loaded
// with that usage. The `earmark`/`expires` params are retained for call-site
// compatibility but no longer meaningful (the grant-lot ledger is gone). All callers
// file against the annual type (leaveAnn).
func (h *harness) seedGrant(id, employee string, amount, consumed, pending int, _ string, _ time.Time) {
	h.meterReader.annual[employee] = amount
	if consumed == 0 && pending == 0 {
		return
	}
	h.meterStore.byID[id] = &dom.LeaveQuota{
		ID: id, EmployeeID: employee, LeaveTypeID: leaveAnn, PeriodKey: "2026",
		EntitledDays: amount, UsedDays: consumed, PendingDays: pending,
	}
}

// seedWindow pre-opens a per-type quota window (employee, leaveAnn, current year)
// with the given entitled/used/pending. Returns the window id for assertions.
func (h *harness) seedWindow(id, employee string, entitled, used, pending int) {
	h.meterReader.annual[employee] = entitled
	h.meterStore.byID[id] = &dom.LeaveQuota{
		ID: id, EmployeeID: employee, LeaveTypeID: leaveAnn, PeriodKey: "2026",
		EntitledDays: entitled, UsedDays: used, PendingDays: pending,
	}
}

// seedCalendarEntry plants a leave_calendar entry directly (status-filtered on read).
func (h *harness) seedCalendarEntry(id, company, employee string, status dom.LeaveStatus, start, end time.Time) {
	c := company
	h.leave.calendar = append(h.leave.calendar, dom.LeaveCalendarEntry{
		LeaveRequestID: id,
		EmployeeID:     employee,
		EmployeeName:   strp("Agent " + employee),
		CompanyID:      &c,
		CompanyName:    strp("Company " + company),
		LeaveTypeID:    "SWP-LT-001",
		LeaveTypeCode:  strp("ANNUAL"),
		StartDate:      start,
		EndDate:        end,
		Status:         status,
	})
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
