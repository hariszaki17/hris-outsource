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
	"strings"
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
	req.BalanceCheck.Earmark = p.Earmark
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
// fakeQuotaRepo — in-memory svc.QuotaRepository over shared maps.
//
// Keyed by id AND by (employee,type,period) so the approve-final balance
// re-check + bulk-grant FindQuotaForEmployeeTypePeriod resolve the same row.
// Deduct/Adjust/Upsert mutate in place so List observes the new totals.
// ---------------------------------------------------------------------------

type fakeQuotaRepo struct {
	byID     map[string]dom.LeaveQuota
	pending  map[string]int // keyed by quota id (recompute-on-read source)
	grantSet []svc.GrantCandidate
}

func newFakeQuotaRepo() *fakeQuotaRepo {
	return &fakeQuotaRepo{byID: map[string]dom.LeaveQuota{}, pending: map[string]int{}}
}

func quotaKey(emp, lt string, period int) string {
	return emp + "|" + lt + "|" + itoa(period)
}

func (r *fakeQuotaRepo) put(q dom.LeaveQuota) { r.byID[q.ID] = q }

func (r *fakeQuotaRepo) ListLeaveQuotas(_ context.Context, f svc.QuotaFilter) ([]dom.LeaveQuota, error) {
	var out []dom.LeaveQuota
	for _, q := range r.byID {
		if f.EmployeeID != nil && q.EmployeeID != *f.EmployeeID {
			continue
		}
		if f.Period != nil && q.Period != *f.Period {
			continue
		}
		out = append(out, q)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if f.CursorCreated != nil && f.CursorID != nil {
		var trimmed []dom.LeaveQuota
		for _, q := range out {
			if q.CreatedAt.Before(*f.CursorCreated) ||
				(q.CreatedAt.Equal(*f.CursorCreated) && q.ID < *f.CursorID) {
				trimmed = append(trimmed, q)
			}
		}
		out = trimmed
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakeQuotaRepo) GetLeaveQuota(_ context.Context, id string) (dom.LeaveQuota, error) {
	q, ok := r.byID[id]
	if !ok {
		return dom.LeaveQuota{}, domain.ErrNotFound
	}
	return q, nil
}

func (r *fakeQuotaRepo) GetLeaveQuotaForUpdate(_ context.Context, _ pgx.Tx, id string) (dom.LeaveQuota, error) {
	return r.GetLeaveQuota(context.Background(), id)
}

func (r *fakeQuotaRepo) FindQuotaForEmployeeTypePeriod(_ context.Context, emp, lt string, period int) (dom.LeaveQuota, error) {
	for _, q := range r.byID {
		if q.EmployeeID == emp && q.LeaveTypeID == lt && q.Period == period {
			return q, nil
		}
	}
	return dom.LeaveQuota{}, domain.ErrNotFound
}

func (r *fakeQuotaRepo) UpsertLeaveQuota(_ context.Context, _ pgx.Tx, p svc.UpsertQuotaParams) (dom.LeaveQuota, error) {
	// Find existing (employee,type,period); preserve used/pending.
	for id, q := range r.byID {
		if q.EmployeeID == p.EmployeeID && q.LeaveTypeID == p.LeaveTypeID && q.Period == p.Period {
			q.Total = p.Total
			q.IsProrated = p.IsProrated
			q.ProrateMonths = p.ProrateMonths
			r.byID[id] = q
			return q, nil
		}
	}
	q := dom.LeaveQuota{
		ID:            "SWP-LQ-" + p.EmployeeID,
		EmployeeID:    p.EmployeeID,
		LeaveTypeID:   p.LeaveTypeID,
		Period:        p.Period,
		PeriodStart:   p.PeriodStart,
		PeriodEnd:     p.PeriodEnd,
		Total:         p.Total,
		IsProrated:    p.IsProrated,
		ProrateMonths: p.ProrateMonths,
		CreatedAt:     fixedNow,
		UpdatedAt:     fixedNow,
	}
	r.byID[q.ID] = q
	return q, nil
}

func (r *fakeQuotaRepo) AdjustLeaveQuotaTotal(_ context.Context, _ pgx.Tx, id string, delta int, adj dom.LeaveQuotaAdjustment) (dom.LeaveQuota, error) {
	q := r.byID[id]
	q.Total += delta
	a := adj
	q.LastAdjustment = &a
	q.UpdatedAt = fixedNow
	r.byID[id] = q
	return q, nil
}

func (r *fakeQuotaRepo) DeductLeaveQuota(_ context.Context, _ pgx.Tx, id string, delta int) (dom.LeaveQuota, error) {
	q := r.byID[id]
	q.Used += delta
	r.byID[id] = q
	return q, nil
}

func (r *fakeQuotaRepo) RestoreLeaveQuota(_ context.Context, _ pgx.Tx, id string, delta int) (dom.LeaveQuota, error) {
	q := r.byID[id]
	q.Used -= delta
	r.byID[id] = q
	return q, nil
}

func (r *fakeQuotaRepo) SetLeaveQuotaOverride(_ context.Context, _ pgx.Tx, id string, ov dom.LeaveQuotaOverride) (dom.LeaveQuota, error) {
	q := r.byID[id]
	o := ov
	q.LastOverride = &o
	r.byID[id] = q
	return q, nil
}

func (r *fakeQuotaRepo) CountPendingLeaveDaysForQuota(_ context.Context, emp, lt string, _, _ time.Time) (int, error) {
	for id, q := range r.byID {
		if q.EmployeeID == emp && q.LeaveTypeID == lt {
			return r.pending[id], nil
		}
	}
	return 0, nil
}

func (r *fakeQuotaRepo) ListActivePlacedEmployeesForGrant(_ context.Context, _, _ time.Time) ([]svc.GrantCandidate, error) {
	return r.grantSet, nil
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

var _ svc.SchedulePort = (*fakeScheduleRepo)(nil)

// ---------------------------------------------------------------------------
// fakeGrantRepo — in-memory svc.GrantRepository (F6.1 grant-lot ledger).
// ---------------------------------------------------------------------------

type fakeEmp struct {
	fullName, nik, nip string
}

type fakeGrantRepo struct {
	lots map[string]*dom.LeaveGrant
	emps map[string]fakeEmp // employee_id → name/nik/nip for ListLeaveBalances
	cons []dom.LeaveConsumption
	seq  int
}

func newFakeGrantRepo() *fakeGrantRepo {
	return &fakeGrantRepo{lots: map[string]*dom.LeaveGrant{}, emps: map[string]fakeEmp{}}
}

func (r *fakeGrantRepo) put(g dom.LeaveGrant) { gg := g; r.lots[g.ID] = &gg }

func (r *fakeGrantRepo) CreateLeaveGrant(_ context.Context, _ pgx.Tx, p svc.CreateGrantParams) (dom.LeaveGrant, error) {
	r.seq++
	id := "SWP-LG-" + itoa(8000+r.seq)
	g := dom.LeaveGrant{ID: id, EmployeeID: p.EmployeeID, Amount: p.Amount, Source: p.Source, Earmark: p.Earmark, Remark: p.Remark, EffectiveFrom: p.EffectiveFrom, ExpiresAt: p.ExpiresAt, GrantedAt: fixedNow}
	r.lots[id] = &g
	return g, nil
}
func (r *fakeGrantRepo) GetLeaveGrant(_ context.Context, id string) (dom.LeaveGrant, error) {
	if l, ok := r.lots[id]; ok {
		return *l, nil
	}
	return dom.LeaveGrant{}, domain.ErrNotFound
}
func (r *fakeGrantRepo) GetLeaveGrantForUpdate(_ context.Context, _ pgx.Tx, id string) (dom.LeaveGrant, error) {
	return r.GetLeaveGrant(context.Background(), id)
}
func (r *fakeGrantRepo) ListLeaveGrants(_ context.Context, f svc.GrantFilter, now time.Time) ([]dom.LeaveGrant, error) {
	var out []dom.LeaveGrant
	for _, l := range r.lots {
		if f.EmployeeID != nil && l.EmployeeID != *f.EmployeeID {
			continue
		}
		if !f.IncludeExpired && !l.IsActive(now) {
			continue
		}
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].ExpiresAt.Equal(out[j].ExpiresAt) {
			return out[i].ExpiresAt.Before(out[j].ExpiresAt)
		}
		return out[i].ID < out[j].ID
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}
func (r *fakeGrantRepo) ListLeaveBalances(_ context.Context, f svc.BalanceListFilter, now time.Time) ([]dom.EmployeeLeaveBalance, error) {
	// Aggregate active lots per employee (mirrors the SQL). Only employees with >= 1
	// active (non-expired) lot appear.
	agg := map[string]*dom.EmployeeLeaveBalance{}
	for _, l := range r.lots {
		if !l.IsActive(now) {
			continue
		}
		emp, ok := r.emps[l.EmployeeID]
		if !ok {
			continue
		}
		if f.Q != nil {
			q := strings.ToLower(*f.Q)
			if !strings.Contains(strings.ToLower(emp.fullName), q) &&
				!strings.Contains(strings.ToLower(emp.nik), q) &&
				!strings.Contains(strings.ToLower(emp.nip), q) {
				continue
			}
		}
		b := agg[l.EmployeeID]
		if b == nil {
			b = &dom.EmployeeLeaveBalance{EmployeeID: l.EmployeeID, FullName: emp.fullName, NIK: emp.nik, NIP: emp.nip}
			agg[l.EmployeeID] = b
		}
		rem := l.Remaining()
		if l.Earmark == nil {
			b.PoolTotal += l.Amount
			b.PoolConsumed += l.Consumed
			b.PoolPending += l.Pending
			b.PoolRemaining += rem
		} else {
			b.EarmarkedRemaining += rem
		}
		b.LotCount++
		if rem > 0 {
			exp := l.ExpiresAt
			if b.NextExpiry == nil || exp.Before(*b.NextExpiry) {
				b.NextExpiry = &exp
			}
		}
	}
	out := make([]dom.EmployeeLeaveBalance, 0, len(agg))
	for _, b := range agg {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FullName != out[j].FullName {
			return out[i].FullName < out[j].FullName
		}
		return out[i].EmployeeID < out[j].EmployeeID
	})
	// keyset cursor
	if f.CursorFullName != nil && f.CursorID != nil {
		var trimmed []dom.EmployeeLeaveBalance
		for _, b := range out {
			if b.FullName > *f.CursorFullName || (b.FullName == *f.CursorFullName && b.EmployeeID > *f.CursorID) {
				trimmed = append(trimmed, b)
			}
		}
		out = trimmed
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakeGrantRepo) PatchLeaveGrant(_ context.Context, _ pgx.Tx, p svc.PatchGrantParams) (dom.LeaveGrant, error) {
	l := r.lots[p.ID]
	if l == nil {
		return dom.LeaveGrant{}, domain.ErrNotFound
	}
	if p.Amount != nil {
		l.Amount = *p.Amount
	}
	if p.ExpiresAt != nil {
		l.ExpiresAt = *p.ExpiresAt
	}
	if p.SetEarmark {
		l.Earmark = p.Earmark
	}
	return *l, nil
}
func (r *fakeGrantRepo) ListConsumptionsForGrant(_ context.Context, grantID string) ([]dom.LeaveConsumption, error) {
	var out []dom.LeaveConsumption
	for _, c := range r.cons {
		if c.GrantID == grantID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *fakeGrantRepo) ListConsumptionsForRequest(_ context.Context, requestID string) ([]dom.LeaveConsumption, error) {
	var out []dom.LeaveConsumption
	for _, c := range r.cons {
		if c.LeaveRequestID == requestID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *fakeGrantRepo) GetActiveLotsForAllocation(_ context.Context, _ pgx.Tx, employeeID, earmarkMatch string, now time.Time) ([]dom.LeaveGrant, error) {
	var out []dom.LeaveGrant
	for _, l := range r.lots {
		if l.EmployeeID != employeeID || !l.IsActive(now) {
			continue
		}
		if earmarkMatch == "__null" {
			if l.Earmark != nil {
				continue
			}
		} else if l.Earmark == nil || *l.Earmark != earmarkMatch {
			continue
		}
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].ExpiresAt.Equal(out[j].ExpiresAt) {
			return out[i].ExpiresAt.Before(out[j].ExpiresAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}
func (r *fakeGrantRepo) ReservePending(_ context.Context, _ pgx.Tx, id string, days int) error {
	r.lots[id].Pending += days
	return nil
}
func (r *fakeGrantRepo) CommitReservation(_ context.Context, _ pgx.Tx, id string, days int) error {
	l := r.lots[id]
	if l.Pending -= days; l.Pending < 0 {
		l.Pending = 0
	}
	l.Consumed += days
	return nil
}
func (r *fakeGrantRepo) ReleasePending(_ context.Context, _ pgx.Tx, id string, days int) error {
	l := r.lots[id]
	if l.Pending -= days; l.Pending < 0 {
		l.Pending = 0
	}
	return nil
}
func (r *fakeGrantRepo) ReverseConsumption(_ context.Context, _ pgx.Tx, id string, days int) error {
	l := r.lots[id]
	if l.Consumed -= days; l.Consumed < 0 {
		l.Consumed = 0
	}
	return nil
}
func (r *fakeGrantRepo) ApplyConsumption(_ context.Context, _ pgx.Tx, requestID, grantID string, days int) (dom.LeaveConsumption, error) {
	r.seq++
	c := dom.LeaveConsumption{ID: "SWP-LC-" + itoa(r.seq), LeaveRequestID: requestID, GrantID: grantID, Days: days, CreatedAt: fixedNow}
	r.cons = append(r.cons, c)
	return c, nil
}
func (r *fakeGrantRepo) DeleteConsumptionsForRequest(_ context.Context, _ pgx.Tx, requestID string) error {
	var keep []dom.LeaveConsumption
	for _, c := range r.cons {
		if c.LeaveRequestID != requestID {
			keep = append(keep, c)
		}
	}
	r.cons = keep
	return nil
}
func (r *fakeGrantRepo) SumActiveBalanceByEarmark(_ context.Context, employeeID string, now time.Time) ([]svc.EarmarkBalanceGroup, error) {
	groups := map[string]*svc.EarmarkBalanceGroup{}
	for _, l := range r.lots {
		if l.EmployeeID != employeeID || !l.IsActive(now) {
			continue
		}
		key := ""
		if l.Earmark != nil {
			key = *l.Earmark
		}
		g := groups[key]
		if g == nil {
			g = &svc.EarmarkBalanceGroup{}
			if l.Earmark != nil {
				e := *l.Earmark
				g.Earmark = &e
			}
			groups[key] = g
		}
		g.Remaining += l.Remaining()
		g.Pending += l.Pending
		exp := l.ExpiresAt
		if g.NextExpiry == nil || exp.Before(*g.NextExpiry) {
			g.NextExpiry = &exp
		}
	}
	var out []svc.EarmarkBalanceGroup
	for _, g := range groups {
		out = append(out, *g)
	}
	return out, nil
}
func (r *fakeGrantRepo) FindExpiredLotsWithPending(_ context.Context, today time.Time, _ int) ([]svc.ExpiredLot, error) {
	var out []svc.ExpiredLot
	for _, l := range r.lots {
		if l.ExpiresAt.Before(today) && l.Pending > 0 {
			out = append(out, svc.ExpiredLot{ID: l.ID, EmployeeID: l.EmployeeID, ExpiresAt: l.ExpiresAt, PendingDays: l.Pending})
		}
	}
	return out, nil
}
func (r *fakeGrantRepo) ZeroLotPending(_ context.Context, _ pgx.Tx, id string) error {
	r.lots[id].Pending = 0
	return nil
}

var _ svc.GrantRepository = (*fakeGrantRepo)(nil)

// ---------------------------------------------------------------------------
// harness — mounts the REAL services + handler over the fakes.
// ---------------------------------------------------------------------------

type harness struct {
	router    *chi.Mux
	leave     *fakeLeaveRepo
	quota     *fakeQuotaRepo
	grant     *fakeGrantRepo
	schedule  *fakeScheduleRepo
	idem      *stubIdempotency
	principal auth.Principal
}

// newHarness builds the E6 leave slice over the fakes. principalRole is the
// caller's role; companyID + employeeID populate a shift_leader's scope + own-
// record identity (employeeID also drives the self-approve guard for staff).
func newHarness(t *testing.T, principalRole auth.Role, companyID, employeeID string) *harness {
	t.Helper()
	lrepo := newFakeLeaveRepo()
	qrepo := newFakeQuotaRepo()
	grepo := newFakeGrantRepo()
	srepo := newFakeScheduleRepo()

	gsvc := svc.NewGrantService(grepo, &fakeTxRunner{})
	gsvc.SetClock(func() time.Time { return fixedNow })
	lsvc := svc.NewLeaveService(lrepo, gsvc, srepo, &fakeTxRunner{})
	lsvc.SetClock(func() time.Time { return fixedNow })
	qsvc := svc.NewQuotaService(qrepo, &fakeTxRunner{})
	qsvc.SetClock(func() time.Time { return fixedNow })
	csvc := svc.NewCalendarService(lrepo)
	csvc.SetClock(func() time.Time { return fixedNow })

	handler := leavehandler.NewHandler(lsvc, qsvc, gsvc, csvc)
	idem := newStubIdempotency()

	h := &harness{
		leave:    lrepo,
		quota:    qrepo,
		grant:    grepo,
		schedule: srepo,
		idem:     idem,
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

	// Mirror server.go: reads + L1 + reject + calendar + quota-list under
	// RequireRole(super/hr/leader); final/override + quota-writes under
	// RequireRole(super/hr); action routes wrap the idempotency middleware.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.Get("/leave-requests", handler.ListLeaveRequests)
		r.Get("/leave-requests/{id}", handler.GetLeaveRequest)
		r.With(idem.Handler).Post("/leave-requests/{id}:approve-l1", handler.ApproveLeaveRequestL1)
		r.With(idem.Handler).Post("/leave-requests/{id}:reject", handler.RejectLeaveRequest)
		r.Get("/leave-quotas", handler.ListLeaveQuotas)
		r.Get("/leave-calendar", handler.GetLeaveCalendar)
		r.Get("/leave-grants", handler.ListLeaveGrants)
		r.Get("/leave-grants/{id}", handler.GetLeaveGrant)
		r.Get("/leave-balances", handler.ListLeaveBalances)
		r.Get("/leave-balances/by-employee/{employee_id}", handler.GetLeaveBalanceByEmployee)
	})
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.With(idem.Handler).Post("/leave-requests/{id}:approve-final", handler.ApproveLeaveRequestFinal)
		r.With(idem.Handler).Post("/leave-requests/{id}:approve-override", handler.ApproveLeaveRequestOverride)
		r.With(idem.Handler).Post("/leave-grants", handler.CreateLeaveGrant)
		r.With(idem.Handler).Patch("/leave-grants/{id}", handler.PatchLeaveGrant)
		r.With(idem.Handler).Post("/leave-quotas/{id}:adjust", handler.AdjustLeaveQuota)
		r.With(idem.Handler).Post("/leave-quotas:bulk-grant", handler.BulkGrantLeaveQuotas)
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
	line := dom.ServiceLineParking
	req := dom.LeaveRequest{
		ID:            id,
		EmployeeID:    employee,
		CompanyID:     &c,
		ServiceLineID: &line,
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

// seedQuota plants a leave_quotas row (annual, calendar-year period).
func (h *harness) seedQuota(id, employee, leaveType string, period, total, used, pending int) dom.LeaveQuota {
	q := dom.LeaveQuota{
		ID:           id,
		EmployeeID:   employee,
		LeaveTypeID:  leaveType,
		Period:       period,
		PeriodStart:  ymd(period, time.January, 1),
		PeriodEnd:    ymd(period, time.December, 31),
		Total:        total,
		Used:         used,
		Pending:      pending,
		CreatedAt:    fixedNow,
		UpdatedAt:    fixedNow,
		EmployeeName: strp("Agent " + employee),
	}
	h.quota.byID[id] = q
	h.quota.pending[id] = pending
	return q
}

// seedGrant plants a leave_grants lot (F6.1) for the FIFO allocator. earmark "" = pool.
func (h *harness) seedGrant(id, employee string, amount, consumed, pending int, earmark string, expires time.Time) dom.LeaveGrant {
	var em *string
	if earmark != "" {
		e := earmark
		em = &e
	}
	g := dom.LeaveGrant{
		ID: id, EmployeeID: employee, Amount: amount, Consumed: consumed, Pending: pending,
		Source: dom.GrantSourceAnnual, Earmark: em, Remark: strp("seed lot"),
		GrantedAt: ymd(2026, time.January, 1), EffectiveFrom: ymd(2026, time.January, 1), ExpiresAt: expires,
		CreatedAt: fixedNow, UpdatedAt: fixedNow, EmployeeName: strp("Agent " + employee),
	}
	h.grant.put(g)
	if _, ok := h.grant.emps[employee]; !ok {
		h.grant.emps[employee] = fakeEmp{fullName: "Agent " + employee, nik: "NIK-" + employee, nip: "NIP-" + employee}
	}
	return g
}

// seedEmp registers an employee's name/nik/nip for the ListLeaveBalances aggregate
// (the JOIN employees source). Call before seedGrant to control the search fields.
func (h *harness) seedEmp(employee, fullName, nik, nip string) {
	h.grant.emps[employee] = fakeEmp{fullName: fullName, nik: nik, nip: nip}
}

// seedCalendarEntry plants a leave_calendar entry directly (status-filtered on read).
func (h *harness) seedCalendarEntry(id, company, employee string, status dom.LeaveStatus, start, end time.Time) {
	c := company
	line := dom.ServiceLineParking
	h.leave.calendar = append(h.leave.calendar, dom.LeaveCalendarEntry{
		LeaveRequestID: id,
		EmployeeID:     employee,
		EmployeeName:   strp("Agent " + employee),
		CompanyID:      &c,
		CompanyName:    strp("Company " + company),
		ServiceLine:    &line,
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
