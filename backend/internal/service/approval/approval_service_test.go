// Package approval — unit tests for the E11 approvals ENGINE. They exercise the
// invariants INV-1..9 over in-memory fakes + a fakeTx (so the audit-in-tx writes
// run without Postgres):
//
//   - UpsertTemplate shape/active-member validation + version bump + INV-6 pending reset,
//   - CreateInstance: template (current_line=1, line_count=N) vs super-admin fallback (INV-7),
//   - Approve OR-within-line / AND-across-lines advance, terminal finalize + OnApproved (INV-2/8),
//     non-member 403, self-approval 403 (INV-3), terminal 409,
//   - Reject (INV-4) + OnRejected + reason-required,
//   - Bypass (INV-5) super-only + OnApproved + BYPASS action + terminal 409,
//   - hook-error rollback (EX-9: instance stays non-terminal),
//   - List mine=true inbox (current-line member, requester excluded).
package approval

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// ---------------------------------------------------------------------------
// fakeTx — only Exec is needed (audit.Record); every other method panics.
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

type fakeRunner struct{}

func (fakeRunner) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error { return fn(fakeTx{}) }

// rollbackRunner models real transaction rollback over the in-memory repo: it
// snapshots the instance map before fn and restores it if fn errors (so an
// EX-9 hook error leaves the persisted instance at its pre-tx state). Used by the
// hook-error rollback test; the plain fakeRunner is enough elsewhere.
type rollbackRunner struct{ repo *fakeApprovalRepo }

func (rr rollbackRunner) InTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	snap := make(map[string]dom.Instance, len(rr.repo.instances))
	for id, inst := range rr.repo.instances {
		snap[id] = *inst
	}
	actSnap := make(map[string][]dom.Action, len(rr.repo.actions))
	for id, a := range rr.repo.actions {
		actSnap[id] = append([]dom.Action(nil), a...)
	}
	if err := fn(fakeTx{}); err != nil {
		for id := range rr.repo.instances {
			delete(rr.repo.instances, id)
		}
		for id, inst := range snap {
			cp := inst
			rr.repo.instances[id] = &cp
		}
		rr.repo.actions = actSnap
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// fakeApprovalRepo — in-memory ApprovalRepository over shared maps. Instances +
// actions mutate in place so the *ForUpdate re-read + list/get observe the new
// state. Members are stored as the active map; templates carry their resolved
// lines so GetTemplateByID/ByCompany return the OR-sets.
// ---------------------------------------------------------------------------

type fakeApprovalRepo struct {
	templates map[string]dom.Template // id -> template
	byCompany map[string]string       // companyID -> templateID
	active    map[string]bool         // userID -> active (drives TM-3 validation)
	names     map[string]string       // userID -> display name

	instances map[string]*dom.Instance // id -> instance
	actions   map[string][]dom.Action  // instanceID -> trail
	userToEmp map[string]string        // userID -> employeeID (drives inbox requester exclusion)

	tplSeq  int
	instSeq int
	actSeq  int

	// lastLines records the most recent ReplaceLines payload so ListMembers can
	// reflect it for the post-replace active-member validation.
	lastLines [][]string
}

func newFakeRepo() *fakeApprovalRepo {
	return &fakeApprovalRepo{
		templates: map[string]dom.Template{},
		byCompany: map[string]string{},
		active:    map[string]bool{},
		names:     map[string]string{},
		instances: map[string]*dom.Instance{},
		actions:   map[string][]dom.Action{},
		userToEmp: map[string]string{},
	}
}

func (r *fakeApprovalRepo) GetTemplateByCompany(_ context.Context, companyID string) (dom.Template, error) {
	id, ok := r.byCompany[companyID]
	if !ok {
		return dom.Template{}, domain.ErrNotFound
	}
	return r.templates[id], nil
}

func (r *fakeApprovalRepo) GetTemplateByID(_ context.Context, id string) (dom.Template, error) {
	tpl, ok := r.templates[id]
	if !ok {
		return dom.Template{}, domain.ErrNotFound
	}
	return tpl, nil
}

func (r *fakeApprovalRepo) InsertTemplate(_ context.Context, _ pgx.Tx, companyID string, createdBy *string) (dom.Template, error) {
	r.tplSeq++
	id := "SWP-APT-" + itoa(r.tplSeq)
	tpl := dom.Template{
		ID:        id,
		CompanyID: companyID,
		Version:   1,
		CreatedBy: createdBy,
		CreatedAt: fixedNow,
		UpdatedAt: fixedNow,
	}
	r.templates[id] = tpl
	r.byCompany[companyID] = id
	return tpl, nil
}

func (r *fakeApprovalRepo) BumpTemplateVersion(_ context.Context, _ pgx.Tx, id string) (dom.Template, error) {
	tpl, ok := r.templates[id]
	if !ok {
		return dom.Template{}, domain.ErrNotFound
	}
	tpl.Version++
	tpl.UpdatedAt = fixedNow
	r.templates[id] = tpl
	return tpl, nil
}

func (r *fakeApprovalRepo) DeleteTemplate(_ context.Context, _ pgx.Tx, id string) error {
	tpl, ok := r.templates[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(r.byCompany, tpl.CompanyID)
	delete(r.templates, id)
	return nil
}

// ReplaceLines stores the ordered OR-sets onto the template (line_no i+1) and
// records the raw lines for the post-replace ListMembers validation.
func (r *fakeApprovalRepo) ReplaceLines(_ context.Context, _ pgx.Tx, templateID string, lines [][]string) error {
	tpl, ok := r.templates[templateID]
	if !ok {
		return domain.ErrNotFound
	}
	r.lastLines = lines
	out := make([]dom.Line, 0, len(lines))
	for i, members := range lines {
		l := dom.Line{ID: templateID + "-L" + itoa(i+1), LineNo: i + 1}
		for _, uid := range members {
			l.Members = append(l.Members, dom.Member{UserID: uid, DisplayName: r.names[uid], Active: r.active[uid]})
		}
		out = append(out, l)
	}
	tpl.Lines = out
	r.templates[templateID] = tpl
	return nil
}

// ListMembers returns every member across the most recent ReplaceLines payload
// with the active flag joined (TM-3).
func (r *fakeApprovalRepo) ListMembers(_ context.Context, _ string) ([]dom.Member, error) {
	var out []dom.Member
	seen := map[string]bool{}
	for _, members := range r.lastLines {
		for _, uid := range members {
			if seen[uid] {
				continue
			}
			seen[uid] = true
			out = append(out, dom.Member{UserID: uid, DisplayName: r.names[uid], Active: r.active[uid]})
		}
	}
	return out, nil
}

func (r *fakeApprovalRepo) ResetPendingInstancesForCompany(_ context.Context, _ pgx.Tx, companyID string, newVersion *int) error {
	for _, inst := range r.instances {
		if inst.Status != dom.InstanceStatusPending {
			continue
		}
		if inst.CompanyID == nil || *inst.CompanyID != companyID {
			continue
		}
		inst.CurrentLine = 1
		inst.TemplateVersion = newVersion
		if newVersion == nil {
			inst.TemplateID = nil
			inst.LineCount = 1
		} else if tid, ok := r.byCompany[companyID]; ok {
			t := tid
			inst.TemplateID = &t
			inst.LineCount = len(r.templates[tid].Lines)
		}
		inst.UpdatedAt = fixedNow
	}
	return nil
}

func (r *fakeApprovalRepo) InsertInstance(_ context.Context, _ pgx.Tx, p InsertInstanceParams) (dom.Instance, error) {
	r.instSeq++
	id := "SWP-APV-" + itoa(r.instSeq)
	inst := dom.Instance{
		ID:              id,
		RequestType:     p.RequestType,
		RequestID:       p.RequestID,
		CompanyID:       p.CompanyID,
		TemplateID:      p.TemplateID,
		TemplateVersion: p.TemplateVersion,
		CurrentLine:     p.CurrentLine,
		LineCount:       p.LineCount,
		Status:          p.Status,
		RequesterID:     p.RequesterID,
		CreatedAt:       fixedNow,
		UpdatedAt:       fixedNow,
	}
	r.instances[id] = &inst
	return inst, nil
}

func (r *fakeApprovalRepo) GetInstance(_ context.Context, id string) (dom.Instance, error) {
	inst, ok := r.instances[id]
	if !ok {
		return dom.Instance{}, domain.ErrNotFound
	}
	return *inst, nil
}

func (r *fakeApprovalRepo) GetInstanceForUpdate(_ context.Context, _ pgx.Tx, id string) (dom.Instance, error) {
	return r.GetInstance(context.Background(), id)
}

func (r *fakeApprovalRepo) ListInstances(_ context.Context, f InstanceFilter) ([]dom.Instance, error) {
	var out []dom.Instance
	for _, inst := range r.instances {
		if f.CompanyID != nil && (inst.CompanyID == nil || *inst.CompanyID != *f.CompanyID) {
			continue
		}
		if f.Status != nil && string(inst.Status) != *f.Status {
			continue
		}
		if f.RequestType != nil && string(inst.RequestType) != *f.RequestType {
			continue
		}
		out = append(out, *inst)
	}
	return out, nil
}

// ListInstancesForMember is the inbox query: PENDING instances whose current line
// includes memberUserID and whose requester is not the caller (INV-3).
func (r *fakeApprovalRepo) ListInstancesForMember(_ context.Context, memberUserID string, f InstanceFilter) ([]dom.Instance, error) {
	var out []dom.Instance
	for _, inst := range r.instances {
		if inst.Status != dom.InstanceStatusPending {
			continue
		}
		members := r.currentLineMembers(inst)
		if !containsStr(members, memberUserID) {
			continue
		}
		// INV-3 inbox: exclude instances the caller requested (requester is an
		// employee id; map the caller's user id to their employee id).
		if emp, ok := r.userToEmp[memberUserID]; ok && inst.RequesterID != nil && *inst.RequesterID == emp {
			continue
		}
		if f.CompanyID != nil && (inst.CompanyID == nil || *inst.CompanyID != *f.CompanyID) {
			continue
		}
		out = append(out, *inst)
	}
	return out, nil
}

func (r *fakeApprovalRepo) UpdateInstanceProgress(_ context.Context, _ pgx.Tx, id string, currentLine int, status dom.InstanceStatus) error {
	inst, ok := r.instances[id]
	if !ok {
		return domain.ErrNotFound
	}
	inst.CurrentLine = currentLine
	inst.Status = status
	inst.UpdatedAt = fixedNow
	return nil
}

func (r *fakeApprovalRepo) currentLineMembers(inst *dom.Instance) []string {
	if inst.TemplateID == nil {
		return nil // super-admin fallback line has no enumerated members
	}
	tpl := r.templates[*inst.TemplateID]
	for _, l := range tpl.Lines {
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

func (r *fakeApprovalRepo) CurrentLineMembers(_ context.Context, instanceID string) ([]string, error) {
	inst, ok := r.instances[instanceID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return r.currentLineMembers(inst), nil
}

func (r *fakeApprovalRepo) InsertAction(_ context.Context, _ pgx.Tx, p InsertActionParams) (dom.Action, error) {
	r.actSeq++
	a := dom.Action{
		ID:              "SWP-APA-" + itoa(r.actSeq),
		InstanceID:      p.InstanceID,
		LineNo:          p.LineNo,
		TemplateVersion: p.TemplateVersion,
		ActorUserID:     p.ActorUserID,
		Action:          p.Action,
		Reason:          p.Reason,
		CreatedAt:       fixedNow,
	}
	r.actions[p.InstanceID] = append(r.actions[p.InstanceID], a)
	return a, nil
}

func (r *fakeApprovalRepo) ListActionsByInstance(_ context.Context, instanceID string) ([]dom.Action, error) {
	return r.actions[instanceID], nil
}

var _ ApprovalRepository = (*fakeApprovalRepo)(nil)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

var fixedNow = time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)

func newEngine(r *fakeApprovalRepo) *ApprovalService {
	s := NewApprovalService(r, fakeRunner{})
	s.SetClock(func() time.Time { return fixedNow })
	return s
}

func superCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-SUP", EmployeeID: "SWP-EMP-SUP", Role: auth.RoleSuperAdmin})
}
func hrCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-HR", EmployeeID: "SWP-EMP-HR", Role: auth.RoleHRAdmin})
}

// memberCtx builds a leader principal that is a line member (UserID drives
// membership; EmployeeID drives the self-approval guard).
func memberCtx(userID, employeeID, company string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: userID, EmployeeID: employeeID, Role: auth.RoleShiftLeader, CompanyID: company})
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected *apperr.Error, got %v", err)
	}
	return ae.Code
}

func statusOf(t *testing.T, err error) int {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected *apperr.Error, got %v", err)
	}
	return ae.Status()
}

func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// recordingHooks is a fake Hooks pair that counts calls + can be made to error.
type recordingHooks struct {
	approved   int
	rejected   int
	approvedID string
	rejectedID string
	approveErr error
	rejectErr  error
}

func (h *recordingHooks) hooks() dom.Hooks {
	return dom.Hooks{
		OnApproved: func(_ context.Context, _ pgx.Tx, requestID string) error {
			h.approved++
			h.approvedID = requestID
			return h.approveErr
		},
		OnRejected: func(_ context.Context, _ pgx.Tx, requestID string) error {
			h.rejected++
			h.rejectedID = requestID
			return h.rejectErr
		},
	}
}

// seedActiveUsers registers user ids as active SWP staff so TM-3 passes.
func (r *fakeApprovalRepo) seedActiveUsers(ids ...string) {
	for _, id := range ids {
		r.active[id] = true
		r.names[id] = "User " + id
	}
}

// seedTemplate plants a versioned template with resolved lines for a company.
func (r *fakeApprovalRepo) seedTemplate(id, companyID string, version int, lines [][]string) dom.Template {
	tpl := dom.Template{ID: id, CompanyID: companyID, Version: version, CreatedAt: fixedNow, UpdatedAt: fixedNow}
	for i, members := range lines {
		l := dom.Line{ID: id + "-L" + itoa(i+1), LineNo: i + 1}
		for _, uid := range members {
			r.active[uid] = true
			l.Members = append(l.Members, dom.Member{UserID: uid, Active: true})
		}
		tpl.Lines = append(tpl.Lines, l)
	}
	r.templates[id] = tpl
	r.byCompany[companyID] = id
	return tpl
}

// seedInstance plants a live instance bound to a template (line/count derived).
func (r *fakeApprovalRepo) seedInstance(id, companyID, tplID, requesterEmp string, currentLine int, status dom.InstanceStatus) *dom.Instance {
	c := companyID
	t := tplID
	ver := r.templates[tplID].Version
	inst := &dom.Instance{
		ID:              id,
		RequestType:     dom.RequestTypeLeave,
		RequestID:       "SWP-LR-" + id,
		CompanyID:       &c,
		TemplateID:      &t,
		TemplateVersion: &ver,
		CurrentLine:     currentLine,
		LineCount:       len(r.templates[tplID].Lines),
		Status:          status,
		RequesterID:     &requesterEmp,
		CreatedAt:       fixedNow,
		UpdatedAt:       fixedNow,
	}
	r.instances[id] = inst
	return inst
}

// ===========================================================================
// UpsertTemplate (F11.1 / TM-2 / TM-3 / INV-6)
// ===========================================================================

func TestUpsertTemplate_TooFewLines400(t *testing.T) {
	r := newFakeRepo()
	r.seedActiveUsers("SWP-USR-A")
	s := newEngine(r)
	_, err := s.UpsertTemplate(hrCtx(), "SWP-CMP-1", [][]string{{"SWP-USR-A"}})
	if got := codeOf(t, err); got != dom.CodeInvalidRequest {
		t.Fatalf("code = %s, want %s", got, dom.CodeInvalidRequest)
	}
	if got := statusOf(t, err); got != 400 {
		t.Fatalf("status = %d, want 400", got)
	}
}

func TestUpsertTemplate_EmptyMemberLine400(t *testing.T) {
	r := newFakeRepo()
	r.seedActiveUsers("SWP-USR-A")
	s := newEngine(r)
	// 2 lines but the second is empty → structurally invalid (TM-2, 400).
	_, err := s.UpsertTemplate(hrCtx(), "SWP-CMP-1", [][]string{{"SWP-USR-A"}, {}})
	if got := codeOf(t, err); got != dom.CodeInvalidRequest {
		t.Fatalf("code = %s, want %s", got, dom.CodeInvalidRequest)
	}
}

func TestUpsertTemplate_InactiveMember422(t *testing.T) {
	r := newFakeRepo()
	r.seedActiveUsers("SWP-USR-A")
	r.active["SWP-USR-DEAD"] = false // known but inactive (employment ended)
	r.names["SWP-USR-DEAD"] = "Former Staff"
	s := newEngine(r)
	_, err := s.UpsertTemplate(hrCtx(), "SWP-CMP-1", [][]string{{"SWP-USR-A"}, {"SWP-USR-DEAD"}})
	if got := codeOf(t, err); got != dom.CodeApprovalLineInvalid {
		t.Fatalf("code = %s, want %s", got, dom.CodeApprovalLineInvalid)
	}
	if got := statusOf(t, err); got != 422 {
		t.Fatalf("status = %d, want 422", got)
	}
}

func TestUpsertTemplate_ValidCreateVersion1(t *testing.T) {
	r := newFakeRepo()
	r.seedActiveUsers("SWP-USR-A", "SWP-USR-B")
	s := newEngine(r)
	tpl, err := s.UpsertTemplate(hrCtx(), "SWP-CMP-1", [][]string{{"SWP-USR-A"}, {"SWP-USR-B"}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tpl.Version != 1 {
		t.Fatalf("version = %d, want 1", tpl.Version)
	}
	if len(tpl.Lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(tpl.Lines))
	}
}

// INV-6: editing an existing template bumps the version AND resets every PENDING
// instance for the company back to current_line=1 on the new version.
func TestUpsertTemplate_EditBumpsVersionAndResetsPending(t *testing.T) {
	r := newFakeRepo()
	r.seedActiveUsers("SWP-USR-A", "SWP-USR-B", "SWP-USR-C")
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-A"}, {"SWP-USR-B"}})
	// A pending instance already advanced to line 2.
	inst := r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 2, dom.InstanceStatusPending)

	s := newEngine(r)
	out, err := s.UpsertTemplate(hrCtx(), "SWP-CMP-1", [][]string{{"SWP-USR-A"}, {"SWP-USR-C"}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Version != 2 {
		t.Fatalf("version = %d, want 2 (bumped)", out.Version)
	}
	if inst.CurrentLine != 1 {
		t.Fatalf("pending current_line = %d, want 1 (INV-6 reset)", inst.CurrentLine)
	}
	if inst.TemplateVersion == nil || *inst.TemplateVersion != 2 {
		t.Fatalf("pending template_version = %v, want 2", inst.TemplateVersion)
	}
}

// ===========================================================================
// CreateInstance (EX-1 / INV-7)
// ===========================================================================

func TestCreateInstance_WithTemplate(t *testing.T) {
	r := newFakeRepo()
	r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 3, [][]string{{"SWP-USR-A"}, {"SWP-USR-B"}, {"SWP-USR-C"}})
	s := newEngine(r)

	id, err := s.CreateInstance(superCtx(), fakeTx{}, dom.CreateInstanceInput{
		RequestType: dom.RequestTypeLeave, RequestID: "SWP-LR-1", CompanyID: "SWP-CMP-1", RequesterID: "SWP-EMP-9",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	inst, _ := r.GetInstance(context.Background(), id)
	if inst.CurrentLine != 1 {
		t.Fatalf("current_line = %d, want 1", inst.CurrentLine)
	}
	if inst.LineCount != 3 {
		t.Fatalf("line_count = %d, want 3", inst.LineCount)
	}
	if inst.TemplateID == nil || *inst.TemplateID != "SWP-APT-1" {
		t.Fatalf("template_id = %v, want SWP-APT-1", inst.TemplateID)
	}
}

// INV-7: no company template → super-admin fallback (template_id nil, line_count 1).
func TestCreateInstance_SuperAdminFallback(t *testing.T) {
	r := newFakeRepo()
	s := newEngine(r)
	id, err := s.CreateInstance(superCtx(), fakeTx{}, dom.CreateInstanceInput{
		RequestType: dom.RequestTypeOvertime, RequestID: "SWP-OT-1", CompanyID: "SWP-CMP-NOPE", RequesterID: "SWP-EMP-9",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	inst, _ := r.GetInstance(context.Background(), id)
	if inst.TemplateID != nil {
		t.Fatalf("template_id = %v, want nil (fallback)", inst.TemplateID)
	}
	if inst.LineCount != 1 {
		t.Fatalf("line_count = %d, want 1 (fallback)", inst.LineCount)
	}
	if inst.Status != dom.InstanceStatusPending {
		t.Fatalf("status = %s, want PENDING", inst.Status)
	}
}

// ===========================================================================
// Approve (F11.2 / INV-2 / INV-3 / INV-8)
// ===========================================================================

// A current-line member's approve on a non-final line advances current_line (OR
// within line, AND across lines).
func TestApprove_AdvancesLine(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	s := newEngine(r)

	out, err := s.Approve(memberCtx("SWP-USR-L1", "SWP-EMP-L1", "SWP-CMP-1"), "SWP-APV-1", "ok")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.CurrentLine != 2 {
		t.Fatalf("current_line = %d, want 2 (advanced)", out.CurrentLine)
	}
	if out.Status != dom.InstanceStatusPending {
		t.Fatalf("status = %s, want PENDING", out.Status)
	}
}

// Clearing the last line finalizes APPROVED + fires OnApproved EXACTLY ONCE (INV-8).
func TestApprove_LastLineFinalizesAndFiresHook(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 2, dom.InstanceStatusPending)
	s := newEngine(r)
	rh := &recordingHooks{}
	s.RegisterHooks(dom.RequestTypeLeave, rh.hooks())

	out, err := s.Approve(memberCtx("SWP-USR-L2", "SWP-EMP-L2", "SWP-CMP-1"), "SWP-APV-1", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.InstanceStatusApproved {
		t.Fatalf("status = %s, want APPROVED", out.Status)
	}
	if rh.approved != 1 {
		t.Fatalf("OnApproved fired %d times, want exactly 1", rh.approved)
	}
	if rh.approvedID != "SWP-LR-SWP-APV-1" {
		t.Fatalf("OnApproved requestID = %q, want the instance request id", rh.approvedID)
	}
}

// INV-2 OR-within-line: any ONE of the line's members clears it.
func TestApprove_OrWithinLine(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-A", "SWP-USR-B"}, {"SWP-USR-C"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	s := newEngine(r)
	// User B (the second member of the OR-set) clears line 1.
	out, err := s.Approve(memberCtx("SWP-USR-B", "SWP-EMP-B", "SWP-CMP-1"), "SWP-APV-1", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.CurrentLine != 2 {
		t.Fatalf("current_line = %d, want 2", out.CurrentLine)
	}
}

// A non-member, non-super caller → 403 FORBIDDEN.
func TestApprove_NonMemberForbidden(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	s := newEngine(r)
	_, err := s.Approve(memberCtx("SWP-USR-STRANGER", "SWP-EMP-X", "SWP-CMP-1"), "SWP-APV-1", "")
	if got := codeOf(t, err); got != "FORBIDDEN" {
		t.Fatalf("code = %s, want FORBIDDEN", got)
	}
}

// INV-3: the requester who is also a current-line member → 403 SELF_APPROVAL_FORBIDDEN.
func TestApprove_SelfApprovalForbidden(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	// requester employee == the caller's employee, and the caller is line-1 member.
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-L1", 1, dom.InstanceStatusPending)
	s := newEngine(r)
	_, err := s.Approve(memberCtx("SWP-USR-L1", "SWP-EMP-L1", "SWP-CMP-1"), "SWP-APV-1", "")
	if got := codeOf(t, err); got != dom.CodeSelfApprovalForbidden {
		t.Fatalf("code = %s, want %s", got, dom.CodeSelfApprovalForbidden)
	}
	if got := statusOf(t, err); got != 403 {
		t.Fatalf("status = %d, want 403", got)
	}
}

// Approving an already-terminal (APPROVED) instance → 409 LINE_ALREADY_CLEARED.
func TestApprove_TerminalConflict(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 2, dom.InstanceStatusApproved)
	s := newEngine(r)
	_, err := s.Approve(memberCtx("SWP-USR-L2", "SWP-EMP-L2", "SWP-CMP-1"), "SWP-APV-1", "")
	if got := codeOf(t, err); got != dom.CodeLineAlreadyCleared {
		t.Fatalf("code = %s, want %s", got, dom.CodeLineAlreadyCleared)
	}
	if got := statusOf(t, err); got != 409 {
		t.Fatalf("status = %d, want 409", got)
	}
}

// ===========================================================================
// Reject (F11.2 / INV-4)
// ===========================================================================

func TestReject_FinalizesAndFiresHook(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	s := newEngine(r)
	rh := &recordingHooks{}
	s.RegisterHooks(dom.RequestTypeLeave, rh.hooks())

	out, err := s.Reject(memberCtx("SWP-USR-L1", "SWP-EMP-L1", "SWP-CMP-1"), "SWP-APV-1", "Tidak lengkap")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.InstanceStatusRejected {
		t.Fatalf("status = %s, want REJECTED", out.Status)
	}
	if rh.rejected != 1 {
		t.Fatalf("OnRejected fired %d times, want 1", rh.rejected)
	}
}

func TestReject_ReasonRequired(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	s := newEngine(r)
	_, err := s.Reject(memberCtx("SWP-USR-L1", "SWP-EMP-L1", "SWP-CMP-1"), "SWP-APV-1", "")
	if got := codeOf(t, err); got != "INVALID_REQUEST" {
		t.Fatalf("code = %s, want INVALID_REQUEST", got)
	}
}

// ===========================================================================
// Bypass (F11.2 / INV-5)
// ===========================================================================

func TestBypass_NonSuperForbidden(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	s := newEngine(r)
	_, err := s.Bypass(hrCtx(), "SWP-APV-1", "darurat")
	if got := codeOf(t, err); got != "FORBIDDEN" {
		t.Fatalf("code = %s, want FORBIDDEN", got)
	}
}

func TestBypass_SuperApprovesAndRecordsBypassAction(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	s := newEngine(r)
	rh := &recordingHooks{}
	s.RegisterHooks(dom.RequestTypeLeave, rh.hooks())

	out, err := s.Bypass(superCtx(), "SWP-APV-1", "Eskalasi manajemen")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.InstanceStatusApproved {
		t.Fatalf("status = %s, want APPROVED", out.Status)
	}
	if rh.approved != 1 {
		t.Fatalf("OnApproved fired %d times, want 1", rh.approved)
	}
	acts := r.actions["SWP-APV-1"]
	if len(acts) != 1 || acts[0].Action != dom.ActionBypass {
		t.Fatalf("trail = %+v, want exactly one BYPASS action", acts)
	}
}

func TestBypass_ReasonRequired(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	s := newEngine(r)
	_, err := s.Bypass(superCtx(), "SWP-APV-1", "")
	if got := codeOf(t, err); got != "INVALID_REQUEST" {
		t.Fatalf("code = %s, want INVALID_REQUEST", got)
	}
}

func TestBypass_TerminalConflict(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusRejected)
	s := newEngine(r)
	_, err := s.Bypass(superCtx(), "SWP-APV-1", "telat")
	if got := codeOf(t, err); got != dom.CodeLineAlreadyCleared {
		t.Fatalf("code = %s, want %s", got, dom.CodeLineAlreadyCleared)
	}
}

// ===========================================================================
// Hook-error rollback (EX-9): a hook returning an error leaves the instance
// non-terminal (the fakeRunner returns the error from InTx without committing the
// in-tx mutations — here we assert the engine surfaces the error AND the persisted
// status was not left APPROVED by a partial path).
// ===========================================================================

func TestApprove_HookErrorRollsBack(t *testing.T) {
	r := newFakeRepo()
	// Single-line template so the approve hits the final-line path and fires OnApproved.
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}})
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	// Use the rollback-modeling runner so an in-tx error truly discards the writes.
	s := NewApprovalService(r, rollbackRunner{repo: r})
	s.SetClock(func() time.Time { return fixedNow })
	rh := &recordingHooks{approveErr: errors.New("downstream commit failed")}
	s.RegisterHooks(dom.RequestTypeLeave, rh.hooks())

	_, err := s.Approve(memberCtx("SWP-USR-L1", "SWP-EMP-L1", "SWP-CMP-1"), "SWP-APV-1", "")
	if err == nil {
		t.Fatalf("expected the hook error to surface, got nil")
	}
	if rh.approved != 1 {
		t.Fatalf("OnApproved should have been invoked once (then errored), got %d", rh.approved)
	}
	// EX-9: the tx rolled back — the persisted instance stays PENDING (non-terminal)
	// and no action row survived.
	got, _ := r.GetInstance(context.Background(), "SWP-APV-1")
	if got.Status != dom.InstanceStatusPending {
		t.Fatalf("status = %s after hook error, want PENDING (rolled back)", got.Status)
	}
	if len(r.actions["SWP-APV-1"]) != 0 {
		t.Fatalf("trail should be empty after rollback, got %d actions", len(r.actions["SWP-APV-1"]))
	}
}

// ===========================================================================
// List mine=true (F11.3 inbox / INV-3)
// ===========================================================================

func TestList_MineReturnsCurrentLineMembership(t *testing.T) {
	r := newFakeRepo()
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	// inst1: pending at line 1 → L1 is a current-line member, requester is someone else.
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	// inst2: pending at line 2 → L1 is NOT a current-line member.
	r.seedInstance("SWP-APV-2", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 2, dom.InstanceStatusPending)
	// inst3: approved → not in inbox.
	r.seedInstance("SWP-APV-3", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusApproved)
	s := newEngine(r)

	rows, _, _, err := s.List(memberCtx("SWP-USR-L1", "SWP-EMP-L1", "SWP-CMP-1"), InstanceFilter{Mine: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("mine returned %d rows, want 1 (only the line-1 pending instance)", len(rows))
	}
	if rows[0].ID != "SWP-APV-1" {
		t.Fatalf("mine[0] = %s, want SWP-APV-1", rows[0].ID)
	}
}

// INV-3: List (mine) excludes instances the caller requested even when they are a
// current-line member of those instances.
func TestList_MineExcludesRequester(t *testing.T) {
	r := newFakeRepo()
	r.userToEmp["SWP-USR-L1"] = "SWP-EMP-L1" // caller's user→employee mapping
	tpl := r.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	// inst1: caller (L1) is the requester AND a line-1 member → excluded.
	r.seedInstance("SWP-APV-1", "SWP-CMP-1", tpl.ID, "SWP-EMP-L1", 1, dom.InstanceStatusPending)
	// inst2: someone else requested; caller is a line-1 member → included.
	r.seedInstance("SWP-APV-2", "SWP-CMP-1", tpl.ID, "SWP-EMP-9", 1, dom.InstanceStatusPending)
	s := newEngine(r)

	rows, _, _, err := s.List(memberCtx("SWP-USR-L1", "SWP-EMP-L1", "SWP-CMP-1"), InstanceFilter{Mine: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "SWP-APV-2" {
		t.Fatalf("mine = %v, want only SWP-APV-2 (own request excluded, INV-3)", idsOf(rows))
	}
}

func idsOf(rows []dom.Instance) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}
