// Package payroll_test — E8 payroll contract tests (the drift gate replacing
// server codegen). This testkit mirrors the Phase-9 overtime harness
// (internal/handler/overtime/overtime_testkit_test.go) EXACTLY:
//
//   - fakeTx (Exec no-op so audit.Record + InsertAuditNote work inside InTx) +
//     fakeTxRunner,
//   - in-memory fake repos implementing the 10-02 service ports
//     (svc.PayslipRepository + svc.ExportRepository) over shared mutable maps,
//   - a recording fakeJobs (svc.Jobs) whose EnqueueTx captures the args so the
//     export tests assert a PayslipExportArgs was enqueued in the export tx,
//   - a REAL *crypto.Cipher built from a fixed 32-byte test key, used to BOTH seed
//     valid ciphertext (seedFinal) AND injected into the service — so FINAL rows
//     decrypt and garbage rows (seedDecryptFail) fail through the REAL Decrypt
//     boundary (we assert the DECRYPT_FAIL row status, not a stub flag),
//   - newHarness(role) that builds the REAL PayslipService + ExportService +
//     handler.Handler and mounts them on a chi.Router under the SAME
//     RequireRole(super_admin, hr_admin) group + stubIdempotency at the server.go
//     router position, with a mutable-principal closure middleware (swap role per
//     case).
//
// decodeBody snapshots rr.Body.Bytes() so a single response re-decodes for
// errCode + errFields (decision [08-03]).
package payroll_test

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
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	payrollhandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
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
func intp(n int) *int       { return &n }

// fixedNow is the deterministic clock for the payroll tests (12:00 WIB on
// 2026-06-05 — mirrors the leave/attendance/scheduling/overtime test clock).
var fixedNow = time.Date(2026, 6, 5, 5, 0, 0, 0, time.UTC)

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
// fakeJobs — records EnqueueTx args so the export tests assert a
// PayslipExportArgs was enqueued in the export tx (transactional outbox).
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
// fakePayslipRepo — in-memory svc.PayslipRepository over shared maps.
//
// Each payslip row carries RAW *_enc ciphertext (the repo NEVER decrypts; the
// service does). seedFinal Encrypts the money with the harness cipher so it
// opens; seedDecryptFail stores RANDOM garbage bytes so the REAL Decrypt returns
// ErrDecrypt — the DECRYPT_FAIL signal is produced honestly through crypto, not
// a flag. Components/benefits are LineRows per id; audit notes accumulate per id.
// ---------------------------------------------------------------------------

type fakePayslipRepo struct {
	rows       map[string]svc.PayslipRow
	components map[string][]svc.LineRow
	benefits   map[string][]svc.LineRow
	notes      map[string][]dom.PayslipAuditNote
	noteSeq    map[string]int
}

func newFakePayslipRepo() *fakePayslipRepo {
	return &fakePayslipRepo{
		rows:       map[string]svc.PayslipRow{},
		components: map[string][]svc.LineRow{},
		benefits:   map[string][]svc.LineRow{},
		notes:      map[string][]dom.PayslipAuditNote{},
		noteSeq:    map[string]int{},
	}
}

func (r *fakePayslipRepo) ListPayslips(_ context.Context, f svc.PayslipFilter) ([]svc.PayslipRow, error) {
	var out []svc.PayslipRow
	for _, row := range r.rows {
		if f.EmployeeID != nil && row.EmployeeID != *f.EmployeeID {
			continue
		}
		if f.Year != nil && row.Year != *f.Year {
			continue
		}
		if f.Month != nil && row.Month != *f.Month {
			continue
		}
		if f.Status != nil && row.Status != *f.Status {
			continue
		}
		out = append(out, row)
	}
	// keyset (paid_on DESC, id DESC) — newest first; NULL paid_on sorts last.
	sort.Slice(out, func(i, j int) bool {
		pi, pj := out[i].PaidOn, out[j].PaidOn
		switch {
		case pi == nil && pj == nil:
			return out[i].ID > out[j].ID
		case pi == nil:
			return false
		case pj == nil:
			return true
		}
		if pi.Equal(*pj) {
			return out[i].ID > out[j].ID
		}
		return pi.After(*pj)
	})
	if f.CursorID != nil {
		var trimmed []svc.PayslipRow
		for _, row := range out {
			if payslipAfterCursor(row, f.CursorPaidOn, *f.CursorID) {
				trimmed = append(trimmed, row)
			}
		}
		out = trimmed
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// payslipAfterCursor reports whether row sorts strictly after the (paid_on, id)
// cursor under the (paid_on DESC, id DESC) keyset.
func payslipAfterCursor(row svc.PayslipRow, curPaidOn *time.Time, curID string) bool {
	pi := row.PaidOn
	switch {
	case pi == nil && curPaidOn == nil:
		return row.ID < curID
	case pi == nil:
		return true // NULL sorts after any non-null under DESC NULLS LAST
	case curPaidOn == nil:
		return false
	}
	if pi.Equal(*curPaidOn) {
		return row.ID < curID
	}
	return pi.Before(*curPaidOn)
}

func (r *fakePayslipRepo) GetPayslip(_ context.Context, id string) (svc.PayslipRow, error) {
	row, ok := r.rows[id]
	if !ok {
		return svc.PayslipRow{}, domain.ErrNotFound
	}
	return row, nil
}

func (r *fakePayslipRepo) ListComponents(_ context.Context, payslipID string) ([]svc.LineRow, error) {
	return r.components[payslipID], nil
}

func (r *fakePayslipRepo) ListBenefits(_ context.Context, payslipID string) ([]svc.LineRow, error) {
	return r.benefits[payslipID], nil
}

func (r *fakePayslipRepo) PayslipExists(_ context.Context, id string) (bool, error) {
	_, ok := r.rows[id]
	return ok, nil
}

func (r *fakePayslipRepo) ListAuditNotes(_ context.Context, payslipID string, cursorSeq *int, cursorCreatedAt *time.Time, limit int) ([]dom.PayslipAuditNote, error) {
	all := append([]dom.PayslipAuditNote(nil), r.notes[payslipID]...)
	// oldest-first keyset (created_at ASC, seq ASC).
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return seqOf(all[i].ID) < seqOf(all[j].ID)
		}
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	if cursorCreatedAt != nil && cursorSeq != nil {
		var trimmed []dom.PayslipAuditNote
		for _, n := range all {
			if n.CreatedAt.After(*cursorCreatedAt) ||
				(n.CreatedAt.Equal(*cursorCreatedAt) && seqOf(n.ID) > *cursorSeq) {
				trimmed = append(trimmed, n)
			}
		}
		all = trimmed
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func (r *fakePayslipRepo) CountAuditNotes(_ context.Context, payslipID string) (int, error) {
	return len(r.notes[payslipID]), nil
}

func (r *fakePayslipRepo) InsertAuditNote(_ context.Context, _ pgx.Tx, p svc.AuditNoteRow) (dom.PayslipAuditNote, error) {
	note := dom.PayslipAuditNote{
		ID:         p.ID,
		PayslipID:  p.PayslipID,
		Text:       p.Text,
		AuthorID:   p.AuthorID,
		AuthorName: p.AuthorName,
		CreatedAt:  fixedNow,
	}
	r.notes[p.PayslipID] = append(r.notes[p.PayslipID], note)
	return note, nil
}

var _ svc.PayslipRepository = (*fakePayslipRepo)(nil)

// seqOf extracts the trailing {seq} from a composite "{payslip_id}-NOTE-{seq}".
func seqOf(id string) int {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '-' {
			return atoi(id[i+1:])
		}
	}
	return 0
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ---------------------------------------------------------------------------
// fakeExportRepo — in-memory svc.ExportRepository.
//
// countInScope drives the EXPORT_TOO_LARGE guard; InsertExportJob returns a
// QUEUED job (stamping a server-allocated SWP-EXP id) so the 202 response shape
// + the enqueued job id can be asserted; GetExportJob serves the stored job.
// ---------------------------------------------------------------------------

type fakeExportRepo struct {
	countInScope int
	jobs         map[string]dom.ExportJob
	seq          int
}

func newFakeExportRepo() *fakeExportRepo {
	return &fakeExportRepo{jobs: map[string]dom.ExportJob{}}
}

func (r *fakeExportRepo) CountPayslipsInScope(_ context.Context, _, _ *int, _ []string) (int, error) {
	return r.countInScope, nil
}

func (r *fakeExportRepo) InsertExportJob(_ context.Context, _ pgx.Tx, p svc.ExportJobParams) (dom.ExportJob, error) {
	r.seq++
	id := "SWP-EXP-" + itoa(8820+r.seq)
	job := dom.ExportJob{
		ID:               id,
		Status:           dom.ExportJobStatusQueued,
		Format:           "XLSX",
		Confidential:     true,
		RequestedByID:    p.RequestedByID,
		RequestedByName:  p.RequestedByName,
		ScopePeriod:      p.ScopePeriod,
		ScopeYear:        p.ScopeYear,
		ScopeEmployeeIDs: p.ScopeEmployeeIDs,
		RequestedAt:      fixedNow,
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

var _ svc.ExportRepository = (*fakeExportRepo)(nil)

// ---------------------------------------------------------------------------
// testCipher — a REAL *crypto.Cipher over a fixed 32-byte key. The SAME cipher
// seeds valid ciphertext (seedFinal) AND is injected into the service, so FINAL
// rows decrypt and garbage rows fail through the genuine AES-GCM Open.
// ---------------------------------------------------------------------------

func newTestCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1) // deterministic non-zero 32-byte key
	}
	c, err := crypto.New(key)
	if err != nil {
		t.Fatalf("build test cipher: %v", err)
	}
	return c
}

// ---------------------------------------------------------------------------
// harness — mounts the REAL services + handler over the fakes + cipher.
// ---------------------------------------------------------------------------

type harness struct {
	router    *chi.Mux
	payslips  *fakePayslipRepo
	exports   *fakeExportRepo
	jobs      *fakeJobs
	cipher    *crypto.Cipher
	idem      *stubIdempotency
	principal auth.Principal
}

// newHarness builds the E8 payroll slice over the fakes. principalRole is the
// caller's role; the HR/super principal carries an employee id so audit-note
// authorship + export requested_by resolve.
func newHarness(t *testing.T, principalRole auth.Role) *harness {
	t.Helper()
	prepo := newFakePayslipRepo()
	erepo := newFakeExportRepo()
	fjobs := &fakeJobs{}
	cipher := newTestCipher(t)

	psvc := svc.NewPayslipService(prepo, &fakeTxRunner{}, cipher, fjobs)
	esvc := svc.NewExportService(erepo, &fakeTxRunner{}, fjobs)
	handler := payrollhandler.NewHandler(psvc, esvc)
	idem := newStubIdempotency()

	h := &harness{
		payslips: prepo,
		exports:  erepo,
		jobs:     fjobs,
		cipher:   cipher,
		idem:     idem,
		principal: auth.Principal{
			UserID:     "SWP-USR-9001",
			Role:       principalRole,
			CompanyID:  "",
			EmployeeID: "SWP-EMP-9001",
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

	// Mirror server.go: payslip READS are self-or-global (agent + hr/super);
	// audit-notes (read + append) and async export stay HR/Super-Admin ONLY. The
	// two POSTs wrap the idempotency middleware.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleHRAdmin, auth.RoleSuperAdmin))
		r.Get("/payslips", handler.ListPayslips)
		r.Get("/payslips/{id}", handler.GetPayslip)
	})
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Get("/payslips/{id}/audit-notes", handler.ListAuditNotes)
		r.With(idem.Handler).Post("/payslips/{id}/audit-notes", handler.CreateAuditNote)
		r.With(idem.Handler).Post("/payslips:export", handler.ExportPayslips)
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

// moneyFields carries the three summary money strings + the working_days for a
// FINAL payslip fixture.
type moneyFields struct {
	gross    string
	deduct   string
	takeHome string
	workDays int
}

// seedFinal plants a FINAL payslip whose three summary money columns are
// ENCRYPTED with the harness cipher (so the REAL Decrypt opens them). paidOn
// drives the cursor ordering.
func (h *harness) seedFinal(id, employee, name string, year, month int, paidOn time.Time, m moneyFields) svc.PayslipRow {
	row := svc.PayslipRow{
		ID:                 id,
		EmployeeID:         employee,
		EmployeeName:       strp(name),
		Year:               year,
		Month:              month,
		PaidOn:             &paidOn,
		WorkingDays:        intp(m.workDays),
		GrossEarningsEnc:   h.enc(id, m.gross),
		GrossDeductionsEnc: h.enc(id, m.deduct),
		TakeHomePayEnc:     h.enc(id, m.takeHome),
		Status:             string(dom.PayslipStatusFinal),
		SourceSystem:       dom.SourceSystemLumenSwp,
		SourceID:           "44218",
		CreatedAt:          fixedNow,
	}
	h.payslips.rows[id] = row
	return row
}

// enc encrypts a money string with the harness cipher (so the service decrypts
// it). t.Fatalf is not reachable from a method without *testing.T; the cipher is
// deterministic and Encrypt only fails on a broken RNG, so we panic on error.
func (h *harness) enc(_ string, plaintext string) []byte {
	ct, err := h.cipher.Encrypt(plaintext)
	if err != nil {
		panic("seed encrypt: " + err.Error())
	}
	return ct
}

// seedFinalBreakdown attaches earning/deduction components + benefits (all
// ENCRYPTED) to a FINAL payslip so the detail breakdown decrypts.
func (h *harness) seedFinalBreakdown(id string) {
	h.payslips.components[id] = []svc.LineRow{
		{Name: "Gaji Pokok", Kind: "EARNING", ValueEnc: h.enc(id, "6500000.00"), ForBPJS: true},
		{Name: "Tunjangan Transport", Kind: "EARNING", ValueEnc: h.enc(id, "1200000.00"), ForBPJS: false},
		{Name: "BPJS Kesehatan (1%)", Kind: "DEDUCTION", ValueEnc: h.enc(id, "65000.00"), ForBPJS: true},
		{Name: "PPh 21", Kind: "DEDUCTION", ValueEnc: h.enc(id, "915000.00"), ForBPJS: false},
	}
	h.payslips.benefits[id] = []svc.LineRow{
		{Name: "BPJS Kesehatan (employer 4%)", ValueEnc: h.enc(id, "260000.00")},
		{Name: "BPJS JKK", ValueEnc: h.enc(id, "16900.00")},
	}
}

// seedDecryptFail plants a payslip whose money columns hold RANDOM GARBAGE bytes
// (not produced by the cipher) so the REAL Decrypt returns ErrDecrypt → the
// service surfaces a 200 OK DECRYPT_FAIL row. WorkingDays nil (nulled at read).
func (h *harness) seedDecryptFail(id, employee, name string, year, month int, paidOn time.Time) svc.PayslipRow {
	garbage := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}
	row := svc.PayslipRow{
		ID:                 id,
		EmployeeID:         employee,
		EmployeeName:       strp(name),
		Year:               year,
		Month:              month,
		PaidOn:             &paidOn,
		WorkingDays:        nil,
		GrossEarningsEnc:   garbage,
		GrossDeductionsEnc: garbage,
		TakeHomePayEnc:     garbage,
		Status:             string(dom.PayslipStatusDecryptFail),
		SourceSystem:       dom.SourceSystemLumenSwp,
		SourceID:           "44216",
		CreatedAt:          fixedNow,
	}
	h.payslips.rows[id] = row
	return row
}

// seedNote appends a pre-existing audit note (e.g. the 2 seeded notes on the
// DECRYPT_FAIL payslip).
func (h *harness) seedNote(payslipID, text, authorID, authorName string, createdAt time.Time) {
	h.payslips.noteSeq[payslipID]++
	seq := h.payslips.noteSeq[payslipID]
	id := payslipID + "-NOTE-" + itoa(seq)
	h.payslips.notes[payslipID] = append(h.payslips.notes[payslipID], dom.PayslipAuditNote{
		ID:         id,
		PayslipID:  payslipID,
		Text:       text,
		AuthorID:   authorID,
		AuthorName: strp(authorName),
		CreatedAt:  createdAt,
	})
}

// assert the PayslipExportArgs type is reachable for the export tests.
var _ = jobs.PayslipExportArgs{}

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
