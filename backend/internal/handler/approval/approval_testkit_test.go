// Package approval_test — E11 approvals handler contract tests (the drift gate
// replacing server codegen). The testkit mirrors the leave/attendance harnesses:
//
//   - fakeTx (Exec no-op so audit.Record works inside InTx) + fakeTxRunner,
//   - an in-memory svc.ApprovalRepository over shared maps so the lock/re-read +
//     list/get observe each other,
//   - newHarness(role, company, employee) that mounts the REAL ApprovalService +
//     the real approvalhandler.Handler on a chi.Router, with the SAME RBAC role
//     groups as server.go (template manage = hr/super; act = +leader/lead/agent;
//     bypass = super only) and a mutable-principal middleware.
//
// Assertions hit the real handler over the fakes and check the openapi response
// shape + status code + every contract error code.
package approval_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	approvalhandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/approval"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/approval"
)

// ---------------------------------------------------------------------------
// fakeTx — only Exec needed (audit.Record); everything else panics.
// ---------------------------------------------------------------------------

type fakeTx struct{}

func (fakeTx) Begin(context.Context) (pgx.Tx, error) { return fakeTx{}, nil }
func (fakeTx) Commit(context.Context) error          { return nil }
func (fakeTx) Rollback(context.Context) error        { return nil }
func (fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("Query unused") }
func (fakeTx) QueryRow(context.Context, string, ...any) pgx.Row        { panic("QueryRow unused") }
func (fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("CopyFrom unused")
}
func (fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("SendBatch unused") }
func (fakeTx) LargeObjects() pgx.LargeObjects                         { panic("LargeObjects unused") }
func (fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("Prepare unused")
}
func (fakeTx) Conn() *pgx.Conn { panic("Conn unused") }

type fakeTxRunner struct{}

func (fakeTxRunner) InTx(_ context.Context, fn func(pgx.Tx) error) error { return fn(fakeTx{}) }

// ---------------------------------------------------------------------------
// fakeRepo — in-memory svc.ApprovalRepository over shared maps.
// ---------------------------------------------------------------------------

type fakeRepo struct {
	templates map[string]dom.Template
	byCompany map[string]string
	active    map[string]bool
	instances map[string]*dom.Instance
	actions   map[string][]dom.Action
	lastLines [][]string
	tplSeq    int
	instSeq   int
	actSeq    int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		templates: map[string]dom.Template{},
		byCompany: map[string]string{},
		active:    map[string]bool{},
		instances: map[string]*dom.Instance{},
		actions:   map[string][]dom.Action{},
	}
}

func (r *fakeRepo) GetTemplateByCompany(_ context.Context, companyID string) (dom.Template, error) {
	id, ok := r.byCompany[companyID]
	if !ok {
		return dom.Template{}, domain.ErrNotFound
	}
	return r.templates[id], nil
}

func (r *fakeRepo) GetTemplateByID(_ context.Context, id string) (dom.Template, error) {
	tpl, ok := r.templates[id]
	if !ok {
		return dom.Template{}, domain.ErrNotFound
	}
	return tpl, nil
}

func (r *fakeRepo) InsertTemplate(_ context.Context, _ pgx.Tx, companyID string, createdBy *string) (dom.Template, error) {
	r.tplSeq++
	id := "SWP-APT-" + itoa(r.tplSeq)
	tpl := dom.Template{ID: id, CompanyID: companyID, Version: 1, CreatedBy: createdBy, CreatedAt: fixedNow, UpdatedAt: fixedNow}
	r.templates[id] = tpl
	r.byCompany[companyID] = id
	return tpl, nil
}

func (r *fakeRepo) BumpTemplateVersion(_ context.Context, _ pgx.Tx, id string) (dom.Template, error) {
	tpl := r.templates[id]
	tpl.Version++
	tpl.UpdatedAt = fixedNow
	r.templates[id] = tpl
	return tpl, nil
}

func (r *fakeRepo) DeleteTemplate(_ context.Context, _ pgx.Tx, id string) error {
	tpl, ok := r.templates[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(r.byCompany, tpl.CompanyID)
	delete(r.templates, id)
	return nil
}

func (r *fakeRepo) ReplaceLines(_ context.Context, _ pgx.Tx, templateID string, lines [][]string) error {
	tpl := r.templates[templateID]
	r.lastLines = lines
	out := make([]dom.Line, 0, len(lines))
	for i, members := range lines {
		l := dom.Line{ID: templateID + "-L" + itoa(i+1), LineNo: i + 1}
		for _, uid := range members {
			l.Members = append(l.Members, dom.Member{UserID: uid, Active: r.active[uid]})
		}
		out = append(out, l)
	}
	tpl.Lines = out
	r.templates[templateID] = tpl
	return nil
}

func (r *fakeRepo) ListMembers(_ context.Context, _ string) ([]dom.Member, error) {
	var out []dom.Member
	seen := map[string]bool{}
	for _, members := range r.lastLines {
		for _, uid := range members {
			if seen[uid] {
				continue
			}
			seen[uid] = true
			out = append(out, dom.Member{UserID: uid, Active: r.active[uid]})
		}
	}
	return out, nil
}

func (r *fakeRepo) ResetPendingInstancesForCompany(_ context.Context, _ pgx.Tx, companyID string, newVersion *int) error {
	for _, inst := range r.instances {
		if inst.Status != dom.InstanceStatusPending || inst.CompanyID == nil || *inst.CompanyID != companyID {
			continue
		}
		inst.CurrentLine = 1
		inst.TemplateVersion = newVersion
	}
	return nil
}

func (r *fakeRepo) InsertInstance(_ context.Context, _ pgx.Tx, p svc.InsertInstanceParams) (dom.Instance, error) {
	r.instSeq++
	id := "SWP-APV-" + itoa(r.instSeq)
	inst := dom.Instance{
		ID: id, RequestType: p.RequestType, RequestID: p.RequestID, CompanyID: p.CompanyID,
		TemplateID: p.TemplateID, TemplateVersion: p.TemplateVersion, CurrentLine: p.CurrentLine,
		LineCount: p.LineCount, Status: p.Status, RequesterID: p.RequesterID, CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}
	r.instances[id] = &inst
	return inst, nil
}

func (r *fakeRepo) GetInstance(_ context.Context, id string) (dom.Instance, error) {
	inst, ok := r.instances[id]
	if !ok {
		return dom.Instance{}, domain.ErrNotFound
	}
	return *inst, nil
}

func (r *fakeRepo) GetInstanceForUpdate(_ context.Context, _ pgx.Tx, id string) (dom.Instance, error) {
	return r.GetInstance(context.Background(), id)
}

func (r *fakeRepo) ListInstances(_ context.Context, f svc.InstanceFilter) ([]dom.Instance, error) {
	var out []dom.Instance
	for _, inst := range r.instances {
		if f.CompanyID != nil && (inst.CompanyID == nil || *inst.CompanyID != *f.CompanyID) {
			continue
		}
		if f.Status != nil && string(inst.Status) != *f.Status {
			continue
		}
		out = append(out, *inst)
	}
	return out, nil
}

func (r *fakeRepo) ListInstancesForMember(_ context.Context, memberUserID string, _ svc.InstanceFilter) ([]dom.Instance, error) {
	var out []dom.Instance
	for _, inst := range r.instances {
		if inst.Status != dom.InstanceStatusPending {
			continue
		}
		if containsStr(r.curLine(inst), memberUserID) {
			out = append(out, *inst)
		}
	}
	return out, nil
}

func (r *fakeRepo) UpdateInstanceProgress(_ context.Context, _ pgx.Tx, id string, currentLine int, status dom.InstanceStatus) error {
	inst, ok := r.instances[id]
	if !ok {
		return domain.ErrNotFound
	}
	inst.CurrentLine = currentLine
	inst.Status = status
	inst.UpdatedAt = fixedNow
	return nil
}

func (r *fakeRepo) curLine(inst *dom.Instance) []string {
	if inst.TemplateID == nil {
		return nil
	}
	for _, l := range r.templates[*inst.TemplateID].Lines {
		if l.LineNo == inst.CurrentLine {
			out := make([]string, 0, len(l.Members))
			for _, m := range l.Members {
				out = append(out, m.UserID)
			}
			return out
		}
	}
	return nil
}

func (r *fakeRepo) CurrentLineMembers(_ context.Context, instanceID string) ([]string, error) {
	inst, ok := r.instances[instanceID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return r.curLine(inst), nil
}

func (r *fakeRepo) InsertAction(_ context.Context, _ pgx.Tx, p svc.InsertActionParams) (dom.Action, error) {
	r.actSeq++
	a := dom.Action{
		ID: "SWP-APA-" + itoa(r.actSeq), InstanceID: p.InstanceID, LineNo: p.LineNo,
		TemplateVersion: p.TemplateVersion, ActorUserID: p.ActorUserID, Action: p.Action,
		Reason: p.Reason, CreatedAt: fixedNow,
	}
	r.actions[p.InstanceID] = append(r.actions[p.InstanceID], a)
	return a, nil
}

func (r *fakeRepo) ListActionsByInstance(_ context.Context, instanceID string) ([]dom.Action, error) {
	return r.actions[instanceID], nil
}

var _ svc.ApprovalRepository = (*fakeRepo)(nil)

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

var fixedNow = time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)

type harness struct {
	router    *chi.Mux
	repo      *fakeRepo
	principal auth.Principal
}

// newHarness mounts the real ApprovalService + handler over the fake repo, with
// the same RBAC role groups as server.go.
func newHarness(t *testing.T, role auth.Role, company, employee, userID string) *harness {
	t.Helper()
	repo := newFakeRepo()
	asvc := svc.NewApprovalService(repo, fakeTxRunner{})
	asvc.SetClock(func() time.Time { return fixedNow })
	handler := approvalhandler.NewHandler(asvc)

	h := &harness{
		repo: repo,
		principal: auth.Principal{
			UserID: userID, EmployeeID: employee, Role: role, CompanyID: company,
		},
	}
	if role == auth.RoleLead && company != "" {
		h.principal.CompanyIDs = []string{company}
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), h.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// Template manage: hr_admin, super_admin.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Get("/client-companies/{companyId}/approval-template", handler.GetApprovalTemplate)
		r.Put("/client-companies/{companyId}/approval-template", handler.UpsertApprovalTemplate)
		r.Delete("/client-companies/{companyId}/approval-template", handler.DeleteApprovalTemplate)
	})
	// Instance list: staff approvers.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleLead))
		r.Get("/approval-instances", handler.ListApprovalInstances)
	})
	// Detail + act: + agent.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleLead, auth.RoleAgent))
		r.Get("/approval-instances/{id}", handler.GetApprovalInstance)
		r.Post("/approval-instances/{id}:approve", handler.ApproveApprovalInstance)
		r.Post("/approval-instances/{id}:reject", handler.RejectApprovalInstance)
	})
	// Bypass: super_admin ONLY.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin))
		r.Post("/approval-instances/{id}:bypass", handler.BypassApprovalInstance)
	})

	h.router = r
	return h
}

func (h *harness) do(method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// doRaw sends a literal body string (for malformed-JSON decode tests).
func (h *harness) doRaw(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// seed + decode helpers
// ---------------------------------------------------------------------------

func (h *harness) seedTemplate(id, company string, version int, lines [][]string) {
	tpl := dom.Template{ID: id, CompanyID: company, Version: version, CreatedAt: fixedNow, UpdatedAt: fixedNow}
	for i, members := range lines {
		l := dom.Line{ID: id + "-L" + itoa(i+1), LineNo: i + 1}
		for _, uid := range members {
			h.repo.active[uid] = true
			l.Members = append(l.Members, dom.Member{UserID: uid, Active: true})
		}
		tpl.Lines = append(tpl.Lines, l)
	}
	h.repo.templates[id] = tpl
	h.repo.byCompany[company] = id
}

func (h *harness) seedInstance(id, company, tplID, requesterEmp string, currentLine int, status dom.InstanceStatus) {
	c := company
	t := tplID
	ver := h.repo.templates[tplID].Version
	h.repo.instances[id] = &dom.Instance{
		ID: id, RequestType: dom.RequestTypeLeave, RequestID: "SWP-LR-" + id, CompanyID: &c,
		TemplateID: &t, TemplateVersion: &ver, CurrentLine: currentLine,
		LineCount: len(h.repo.templates[tplID].Lines), Status: status, RequesterID: &requesterEmp,
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}
}

func (h *harness) seedActive(ids ...string) {
	for _, id := range ids {
		h.repo.active[id] = true
	}
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&m); err != nil {
		t.Fatalf("decode body: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

// bodyObject decodes a single-item success body. The approval handler writes the
// object directly (httpx.WriteJSON with no {data} wrapper), so this returns the
// top-level map.
func bodyObject(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	return decodeBody(t, rr)
}

// pageData decodes a list success body's data array.
func pageData(t *testing.T, rr *httptest.ResponseRecorder) []any {
	t.Helper()
	d, ok := decodeBody(t, rr)["data"].([]any)
	if !ok {
		t.Fatalf("response has no data array: %s", rr.Body.String())
	}
	return d
}

func errCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	e, ok := decodeBody(t, rr)["error"].(map[string]any)
	if !ok {
		t.Fatalf("response has no error object: %s", rr.Body.String())
	}
	s, _ := e["code"].(string)
	return s
}

func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
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
