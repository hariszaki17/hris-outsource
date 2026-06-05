// Package reporting_test — E10 reporting contract tests (the drift gate replacing
// server codegen). This testkit mirrors the Phase-10 payroll harness
// (internal/handler/payroll/payroll_testkit_test.go) EXACTLY:
//
//   - fakeTx (Exec no-op so audit.Record / audit.RecordReturningID work inside
//     InTx) + fakeTxRunner,
//   - in-memory fake repos implementing the reporting service ports
//     (svc.NotificationRepository + svc.DashboardRepository + svc.BillableRepository
//     + svc.ExportRepository) over shared mutable maps/counters,
//   - a recording fakeJobs (svc.Jobs) whose EnqueueTx captures the args so the
//     export tests assert exactly one ReportExportArgs was enqueued in the export
//     tx (transactional outbox),
//   - newHarness(role, company, employee) that builds the REAL reporting services
//     + handler.Handler and mounts them on a chi.Router at the SAME router
//     positions as server.go (notifications: all 4 roles; dashboard: all 4;
//     billable + exports: super/hr/leader) with stubIdempotency on the action
//     endpoints + a mutable-principal closure middleware (swap role/company/
//     employee per case).
//
// decodeBody snapshots rr.Body.Bytes() so a single response re-decodes for
// errCode + errFields (decision [08-03]).
package reporting_test

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
	"github.com/riverqueue/river"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	reportinghandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
)

// ---------------------------------------------------------------------------
// fakeTx — only Exec is needed (audit.Record / audit.RecordReturningID INSERTs).
// Every other method panics.
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
// QueryRow serves audit.RecordReturningID (the export tx INSERT ... RETURNING id):
// it returns a fake row that scans a deterministic SWP-AL id into the single
// *string destination, so the export-job audit_log_entry_id is captured honestly
// through the real service path.
func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &fakeRow{id: "SWP-AL-1204520"}
}

// fakeRow scans the RETURNING id (audit.RecordReturningID) into a *string dest.
type fakeRow struct{ id string }

func (r *fakeRow) Scan(dest ...any) error {
	if len(dest) == 1 {
		if p, ok := dest[0].(*string); ok {
			*p = r.id
		}
	}
	return nil
}

var _ pgx.Row = (*fakeRow)(nil)
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

// fixedNow is the deterministic clock for the reporting test fixtures (created_at
// stamps + the export requested_at). Mirrors the payroll test clock.
var fixedNow = time.Date(2026, 6, 5, 5, 0, 0, 0, time.UTC)

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
// fakeJobs — records EnqueueTx args so the export tests assert exactly one
// ReportExportArgs was enqueued in the export tx (transactional outbox).
// ---------------------------------------------------------------------------

type fakeJobs struct {
	enqueued []river.JobArgs
}

func (f *fakeJobs) EnqueueTx(_ context.Context, _ pgx.Tx, args river.JobArgs) error {
	f.enqueued = append(f.enqueued, args)
	return nil
}

var _ svc.Jobs = (*fakeJobs)(nil)

// ---------------------------------------------------------------------------
// fakeNotificationRepo — in-memory svc.NotificationRepository over a seeded set
// keyed by recipient. List honors the read_state + kind filters + the
// (created_at, id) keyset cursor; MarkRead/MarkAllRead are recipient-scoped.
// ---------------------------------------------------------------------------

type fakeNotificationRepo struct {
	rows []dom.Notification
}

func newFakeNotificationRepo() *fakeNotificationRepo {
	return &fakeNotificationRepo{}
}

func (r *fakeNotificationRepo) seed(n dom.Notification) {
	r.rows = append(r.rows, n)
}

func containsStr(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

func (r *fakeNotificationRepo) List(_ context.Context, f svc.NotificationFilter, limit int) ([]dom.Notification, error) {
	var out []dom.Notification
	for _, row := range r.rows {
		if !containsStr(f.RecipientIDs, row.Recipient) {
			continue // scope=self
		}
		if f.ReadState != nil {
			switch *f.ReadState {
			case "UNREAD":
				if row.ReadAt != nil {
					continue
				}
			case "READ":
				if row.ReadAt == nil {
					continue
				}
			}
		}
		if len(f.Kinds) > 0 && !containsStr(f.Kinds, string(row.Kind)) {
			continue
		}
		out = append(out, row)
	}
	// keyset (created_at DESC, id DESC) — newest first.
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if f.CursorCreated != nil && f.CursorID != nil {
		var trimmed []dom.Notification
		for _, row := range out {
			if row.CreatedAt.Before(*f.CursorCreated) ||
				(row.CreatedAt.Equal(*f.CursorCreated) && row.ID < *f.CursorID) {
				trimmed = append(trimmed, row)
			}
		}
		out = trimmed
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeNotificationRepo) MarkRead(_ context.Context, id string, recipientIDs []string) (dom.Notification, error) {
	for i := range r.rows {
		if r.rows[i].ID == id && containsStr(recipientIDs, r.rows[i].Recipient) {
			if r.rows[i].ReadAt == nil {
				now := fixedNow
				r.rows[i].ReadAt = &now // COALESCE: first read_at wins (no-op if already read)
			}
			return r.rows[i], nil
		}
	}
	return dom.Notification{}, domain.ErrNotFound
}

func (r *fakeNotificationRepo) MarkAllRead(_ context.Context, recipientIDs []string, before *time.Time) (int, error) {
	n := 0
	for i := range r.rows {
		if !containsStr(recipientIDs, r.rows[i].Recipient) {
			continue
		}
		if r.rows[i].ReadAt != nil {
			continue
		}
		if before != nil && r.rows[i].CreatedAt.After(*before) {
			continue
		}
		now := fixedNow
		r.rows[i].ReadAt = &now
		n++
	}
	return n, nil
}

var _ svc.NotificationRepository = (*fakeNotificationRepo)(nil)

// ---------------------------------------------------------------------------
// fakeDashboardRepo — in-memory svc.DashboardRepository returning configurable
// counts. Each method reads a field the test sets before the call.
// ---------------------------------------------------------------------------

type fakeDashboardRepo struct {
	hrCounts    svc.DashboardCounts
	leaderToday svc.LeaderTodayRow
	leaderAV    int
	leaderLA    int
	leaderOT    int
	companyName string
	agentRecent svc.AgentRecentRow
	agentLeave  int
	agentOT     int
	unread      int
}

func newFakeDashboardRepo() *fakeDashboardRepo {
	return &fakeDashboardRepo{companyName: "Plaza Senayan"}
}

func (r *fakeDashboardRepo) HrCounts(_ context.Context, _ time.Time, _ *string) (svc.DashboardCounts, error) {
	return r.hrCounts, nil
}

func (r *fakeDashboardRepo) LeaderToday(_ context.Context, _ time.Time, _ string) (svc.LeaderTodayRow, error) {
	return r.leaderToday, nil
}

func (r *fakeDashboardRepo) LeaderPending(_ context.Context, _ string) (int, int, int, error) {
	return r.leaderAV, r.leaderLA, r.leaderOT, nil
}

func (r *fakeDashboardRepo) CompanyName(_ context.Context, _ string) (string, error) {
	return r.companyName, nil
}

func (r *fakeDashboardRepo) AgentRecent(_ context.Context, _ string, _ time.Time) (svc.AgentRecentRow, error) {
	return r.agentRecent, nil
}

func (r *fakeDashboardRepo) AgentPending(_ context.Context, _ string) (svc.AgentPendingRow, error) {
	return svc.AgentPendingRow{Leave: r.agentLeave, OT: r.agentOT}, nil
}

func (r *fakeDashboardRepo) CountUnread(_ context.Context, _ []string) (int, error) {
	return r.unread, nil
}

var _ svc.DashboardRepository = (*fakeDashboardRepo)(nil)

// ---------------------------------------------------------------------------
// fakeBillableRepo — in-memory svc.BillableRepository returning configurable
// aggregate rows + summary/pending. countInScope drives the export size guard.
// ---------------------------------------------------------------------------

type fakeBillableRepo struct {
	rows         []dom.BillableReportRow
	summary      dom.BillableSummary
	pending      dom.BillablePendingSummary
	countInScope int
}

func newFakeBillableRepo() *fakeBillableRepo {
	return &fakeBillableRepo{}
}

func (r *fakeBillableRepo) Aggregate(_ context.Context, _ svc.BillableQuery) ([]dom.BillableReportRow, error) {
	return r.rows, nil
}

func (r *fakeBillableRepo) Summary(_ context.Context, _ svc.BillableQuery) (dom.BillableSummary, error) {
	return r.summary, nil
}

func (r *fakeBillableRepo) PendingSummary(_ context.Context, _ svc.BillableQuery) (dom.BillablePendingSummary, error) {
	return r.pending, nil
}

func (r *fakeBillableRepo) CountInScope(_ context.Context, _ svc.BillableQuery) (int, error) {
	return r.countInScope, nil
}

var _ svc.BillableRepository = (*fakeBillableRepo)(nil)

// ---------------------------------------------------------------------------
// fakeExportRepo — in-memory svc.ExportRepository.
//
// InsertExportJob stamps a server-allocated SWP-EXP id + the QUEUED status so
// the 202 body + the enqueued job id can be asserted; GetExportJob serves the
// stored job (seedJob plants RUNNING/DONE rows for the status-mapping tests);
// CancelExportJob flips QUEUED/RUNNING → CANCELLED (no-op if terminal);
// countRecent backs the throttle guard.
// ---------------------------------------------------------------------------

type fakeExportRepo struct {
	jobs        map[string]dom.ExportJob
	seq         int
	countRecent int
}

func newFakeExportRepo() *fakeExportRepo {
	return &fakeExportRepo{jobs: map[string]dom.ExportJob{}}
}

func (r *fakeExportRepo) InsertExportJob(_ context.Context, _ pgx.Tx, p svc.ExportInsert) (dom.ExportJob, error) {
	r.seq++
	id := "SWP-EXP-" + itoa(1040+r.seq)
	var filters map[string]any
	if len(p.Filters) > 0 {
		_ = json.Unmarshal(p.Filters, &filters)
	}
	if filters == nil {
		filters = map[string]any{}
	}
	job := dom.ExportJob{
		ID:              id,
		ReportType:      dom.ReportType(p.ReportType),
		Status:          dom.StatusQueued,
		Format:          dom.ExportFormat(p.Format),
		Confidential:    p.Confidential,
		Filters:         filters,
		AuditLogEntryID: p.AuditLogEntryID,
		RequesterID:     p.RequestedByID,
		RequesterName:   p.RequestedByName,
		RequestedAt:     fixedNow,
		ExpiresAt:       p.ExpiresAt,
	}
	r.jobs[id] = job
	return job, nil
}

func (r *fakeExportRepo) GetExportJob(_ context.Context, id string) (dom.ExportJob, error) {
	job, ok := r.jobs[id]
	if !ok {
		return dom.ExportJob{}, domain.ErrNotFound
	}
	return job, nil
}

func (r *fakeExportRepo) CancelExportJob(_ context.Context, id string) (dom.ExportJob, error) {
	job, ok := r.jobs[id]
	if !ok {
		return dom.ExportJob{}, domain.ErrNotFound
	}
	// QUEUED/RUNNING → CANCELLED; terminal states are a no-op (re-read).
	if job.Status == dom.StatusQueued || job.Status == dom.StatusRunning {
		job.Status = dom.StatusCancelled
		completed := fixedNow
		job.CompletedAt = &completed
		r.jobs[id] = job
	}
	return job, nil
}

func (r *fakeExportRepo) CountRecentExports(_ context.Context, _ string, _ time.Time) (int, error) {
	return r.countRecent, nil
}

var _ svc.ExportRepository = (*fakeExportRepo)(nil)

// seedJob plants a pre-existing export job (e.g. a RUNNING/DONE row for the
// DB→wire status-mapping tests or a foreign-owner row for the scope-404 test).
func (r *fakeExportRepo) seedJob(j dom.ExportJob) {
	r.jobs[j.ID] = j
}

// ---------------------------------------------------------------------------
// harness — mounts the REAL services + handler over the fakes.
// ---------------------------------------------------------------------------

type harness struct {
	router    *chi.Mux
	notifs    *fakeNotificationRepo
	dash      *fakeDashboardRepo
	billable  *fakeBillableRepo
	exports   *fakeExportRepo
	jobs      *fakeJobs
	idem      *stubIdempotency
	principal auth.Principal
}

// newHarness builds the E10 reporting slice over the fakes. role is the caller's
// role; companyID/employeeID populate the principal (leader scope + agent self +
// the notification recipient pair).
func newHarness(t *testing.T, role auth.Role, companyID, employeeID string) *harness {
	t.Helper()
	nrepo := newFakeNotificationRepo()
	drepo := newFakeDashboardRepo()
	brepo := newFakeBillableRepo()
	erepo := newFakeExportRepo()
	fjobs := &fakeJobs{}

	nsvc := svc.NewNotificationService(nrepo)
	dsvc := svc.NewDashboardService(drepo)
	bsvc := svc.NewBillableService(brepo)
	esvc := svc.NewExportService(erepo, brepo, &fakeTxRunner{}, fjobs)
	handler := reportinghandler.NewHandler(nsvc, dsvc, bsvc, esvc)
	idem := newStubIdempotency()

	h := &harness{
		notifs:   nrepo,
		dash:     drepo,
		billable: brepo,
		exports:  erepo,
		jobs:     fjobs,
		idem:     idem,
		principal: auth.Principal{
			UserID:     "SWP-USR-9001",
			Role:       role,
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

	// Mirror server.go router positions exactly.
	// NOTIFICATIONS — all 4 roles; the two action POSTs wrap idempotency.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Get("/notifications", handler.ListNotifications)
		r.With(idem.Handler).Post("/notifications/{notification_id}:mark-read", handler.MarkNotificationRead)
		r.With(idem.Handler).Post("/notifications:mark-all-read", handler.MarkAllNotificationsRead)
	})
	// DASHBOARD — all 4 roles.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Get("/dashboards/me", handler.GetMyDashboard)
	})
	// BILLABLE REPORT — super/hr/leader.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.Get("/reports/attendance-billable", handler.GetBillableReport)
	})
	// EXPORTS — super/hr/leader; create + cancel wrap idempotency.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.With(idem.Handler).Post("/exports", handler.CreateExport)
		r.Get("/exports/{export_id}", handler.GetExport)
		r.With(idem.Handler).Post("/exports/{export_id}:cancel", handler.CancelExport)
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

// assert the ReportExportArgs type is reachable for the export tests.
var _ = jobs.ReportExportArgs{}

// --- small helpers ---

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
