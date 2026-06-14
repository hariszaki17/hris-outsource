// Package leave — unit tests for the leave-request lifecycle wired to the E11
// approval engine (the two-level state machine was ripped out — E11 owns the chain).
// Covers Submit (reserve → PENDING → engine.CreateInstance + link the instance id),
// the OnApproved / OnRejected engine hooks (commit/release the per-type meter, INV-3
// schedule loop-closer, status transition, notify), the cancel-approved actor guard,
// and the collapsed status enum. Uses in-memory fake repos + a fakeTx so the
// audit-in-tx + side-effect writes run without Postgres.
package leave

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	approval "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// --- fakeTx (only Exec needed for audit.Record) ---

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

// --- fake approval engine (E11) ---

// fakeEngine is an in-memory approval.Engine: CreateInstance returns a stub instance
// id and records the call so a test can assert the leave service routed its submit
// through the engine.
type fakeEngine struct {
	instanceID string
	calls      []approval.CreateInstanceInput
	err        error
}

func newFakeEngine() *fakeEngine { return &fakeEngine{instanceID: "SWP-API-9001"} }

func (e *fakeEngine) CreateInstance(_ context.Context, _ pgx.Tx, in approval.CreateInstanceInput) (string, error) {
	if e.err != nil {
		return "", e.err
	}
	e.calls = append(e.calls, in)
	return e.instanceID, nil
}

var _ approval.Engine = (*fakeEngine)(nil)

// --- fake leave repo ---

type fakeLeaveRepo struct {
	req        dom.LeaveRequest
	leaveType  LeaveTypeInfo
	updated    *UpdateStatusParams
	snapshot   *BalanceSnapshotParams
	instanceID string // last value passed to SetApprovalInstanceID
}

func (f *fakeLeaveRepo) ListLeaveRequests(context.Context, RequestFilter) ([]dom.LeaveRequest, error) {
	return []dom.LeaveRequest{f.req}, nil
}
func (f *fakeLeaveRepo) GetLeaveRequest(_ context.Context, id string) (dom.LeaveRequest, error) {
	if f.req.ID == id {
		return f.req, nil
	}
	return dom.LeaveRequest{}, domain.ErrNotFound
}
func (f *fakeLeaveRepo) GetLeaveRequestForUpdate(_ context.Context, _ pgx.Tx, id string) (dom.LeaveRequest, error) {
	if f.req.ID == id {
		return f.req, nil
	}
	return dom.LeaveRequest{}, domain.ErrNotFound
}
func (f *fakeLeaveRepo) UpdateLeaveRequestStatus(_ context.Context, _ pgx.Tx, p UpdateStatusParams) (dom.LeaveRequest, error) {
	f.updated = &p
	f.req.Status = p.Status
	return f.req, nil
}
func (f *fakeLeaveRepo) UpdateLeaveRequestDates(_ context.Context, _ pgx.Tx, id string, start, end time.Time, days int) (dom.LeaveRequest, error) {
	f.req.StartDate, f.req.EndDate, f.req.DurationDays = start, end, days
	return f.req, nil
}
func (f *fakeLeaveRepo) SetApprovalInstanceID(_ context.Context, _ pgx.Tx, id, instanceID string) error {
	f.instanceID = instanceID
	if f.req.ID == id {
		f.req.ApprovalInstanceID = &instanceID
	}
	return nil
}
func (f *fakeLeaveRepo) CreateLeaveRequest(_ context.Context, _ pgx.Tx, p CreateLeaveRequestParams) (dom.LeaveRequest, error) {
	r := dom.LeaveRequest{
		ID: "SWP-LR-NEW", EmployeeID: p.EmployeeID, LeaveTypeID: p.LeaveTypeID,
		StartDate: p.StartDate, EndDate: p.EndDate, DurationDays: p.DurationDays,
		Reason: p.Reason, Status: p.Status, DelegateID: p.DelegateID, DocumentFileID: p.DocumentFileID,
		Backdated: p.Backdated, NoLeader: p.NoLeader, AssignedLeaderID: p.AssignedLeaderID,
	}
	f.req = r
	return r, nil
}
func (f *fakeLeaveRepo) CheckOverlappingLeave(context.Context, string, time.Time, time.Time) (bool, error) {
	return false, nil
}
func (f *fakeLeaveRepo) GetLeaveType(context.Context, string) (LeaveTypeInfo, error) {
	return f.leaveType, nil
}
func (f *fakeLeaveRepo) SetBalanceSnapshot(_ context.Context, _ pgx.Tx, p BalanceSnapshotParams) error {
	f.snapshot = &p
	return nil
}
func (f *fakeLeaveRepo) ListCalendarEntries(context.Context, CalendarFilter, []string, time.Time, time.Time) ([]dom.LeaveCalendarEntry, error) {
	return nil, nil
}

var _ LeaveRepository = (*fakeLeaveRepo)(nil)

// --- fake schedule port ---

type fakeSchedule struct {
	cancelled []ScheduleImpact
	inserted  int
}

func (f *fakeSchedule) CancelScheduleEntriesForLeave(_ context.Context, _ pgx.Tx, _ string, start, _ time.Time) ([]ScheduleImpact, error) {
	imp := ScheduleImpact{ScheduleID: "SWP-SCH-6002", Date: start, NewStatus: "CANCELLED_BY_LEAVE"}
	f.cancelled = append(f.cancelled, imp)
	return f.cancelled, nil
}
func (f *fakeSchedule) InsertApprovedLeaveDay(context.Context, pgx.Tx, string, time.Time, string, string) error {
	f.inserted++
	return nil
}
func (f *fakeSchedule) CountLeaveDuration(_ context.Context, _ string, start, end time.Time) (int, error) {
	return int(end.Sub(start).Hours()/24) + 1, nil
}

// --- helpers ---

var fixedNow = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

func hrCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-HR", EmployeeID: "SWP-EMP-HR", Role: auth.RoleHRAdmin})
}
func agentCtx(emp string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-AG", EmployeeID: emp, Role: auth.RoleAgent})
}

func newReq(status dom.LeaveStatus, company, employee string, days int) dom.LeaveRequest {
	c := company
	return dom.LeaveRequest{
		ID:           "SWP-LR-8001",
		EmployeeID:   employee,
		CompanyID:    &c,
		LeaveTypeID:  "SWP-LT-001",
		StartDate:    time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		EndDate:      time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		DurationDays: days,
		Status:       status,
	}
}

func ptr(s string) *string { return &s }

// newSvc builds a meter-backed LeaveService over an annual-pool window (per-type
// ledger, 2026-06-12) plus a fake approval engine. State-machine / auth tests use it;
// the metering specifics are covered in leave_service_meter_test.go.
func newSvc(lr *fakeLeaveRepo, sp *fakeSchedule) *LeaveService {
	s := newMeterSvc(lr, sp, newMemStore(), memReader{cap: capAnnual(), annual: iptr(12)})
	s.SetApprovalEngine(newFakeEngine())
	return s
}

func annualType() LeaveTypeInfo {
	return LeaveTypeInfo{ID: "SWP-LT-001", Code: "ANNUAL", IsAnnual: true}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected *apperr.Error, got %v", err)
	}
	return ae.Code
}

// --- Submit: reserve → PENDING → engine.CreateInstance + link the instance id ---

func TestSubmit_ReservesPendingAndCreatesInstance(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusDraft, "SWP-CMP-0021", "SWP-EMP-3001", 3), leaveType: annualType()}
	store := newMemStore()
	s := newMeterSvc(lr, &fakeSchedule{}, store, memReader{cap: capAnnual(), annual: iptr(12)})
	eng := newFakeEngine()
	s.SetApprovalEngine(eng)

	out, err := s.Submit(hrCtx(), "SWP-LR-8001")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// reserved against the per-type window.
	w := store.windowFor("SWP-EMP-3001", "SWP-LT-001", "2026")
	if w == nil || w.PendingDays != 3 {
		t.Fatalf("window=%+v want pending 3", w)
	}
	// status PENDING (collapsed; no PENDING_L1/PENDING_HR).
	if out.Status != dom.LeaveStatusPending {
		t.Fatalf("status = %s, want PENDING", out.Status)
	}
	// the engine was called once with the leave request type + ids.
	if len(eng.calls) != 1 {
		t.Fatalf("engine.CreateInstance calls = %d, want 1", len(eng.calls))
	}
	if got := eng.calls[0]; got.RequestType != approval.RequestTypeLeave || got.RequestID != "SWP-LR-8001" || got.RequesterID != "SWP-EMP-3001" {
		t.Errorf("CreateInstance input = %+v, want {LEAVE, SWP-LR-8001, requester SWP-EMP-3001}", got)
	}
	// the instance id was linked on the request.
	if lr.instanceID != eng.instanceID {
		t.Errorf("SetApprovalInstanceID got %q, want %q", lr.instanceID, eng.instanceID)
	}
	if out.ApprovalInstanceID == nil || *out.ApprovalInstanceID != eng.instanceID {
		t.Errorf("request.ApprovalInstanceID = %v, want %q", out.ApprovalInstanceID, eng.instanceID)
	}
}

func TestSubmit_NoEngine_RuleViolation(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusDraft, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	// build a meter-backed service WITHOUT an engine (do not use newMeterSvc, which
	// always wires one).
	s := NewLeaveService(lr, &fakeSchedule{}, fakeRunner{})
	s.SetClock(func() time.Time { return fixedNow })
	s.SetMeter(NewQuotaMeter(newMemStore(), memReader{cap: capAnnual(), annual: iptr(12)}))
	_, err := s.Submit(hrCtx(), "SWP-LR-8001")
	if got := codeOf(t, err); got != "RULE_VIOLATION" {
		t.Fatalf("code = %s, want RULE_VIOLATION (engine not wired)", got)
	}
}

func TestSubmit_NotDraft409(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPending, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	s := newSvc(lr, &fakeSchedule{})
	_, err := s.Submit(hrCtx(), "SWP-LR-8001")
	if got := codeOf(t, err); got != "CONFLICT" {
		t.Fatalf("code = %s, want CONFLICT (not DRAFT)", got)
	}
}

// --- OnApproved hook (engine terminal APPROVED): commit meter + APPROVED + INV-3 ---

func TestOnApproved_CommitsMeterAndFiresINV3(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPending, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	store := newMemStore()
	store.seed("SWP-EMP-3001", "SWP-LT-001", "2026", 12, 0, 1) // 1 day reserved at submit
	sched := &fakeSchedule{}
	s := newMeterSvc(lr, sched, store, memReader{cap: capAnnual(), annual: iptr(12)})

	if err := s.OnApproved(hrCtx(), fakeTx{}, "SWP-LR-8001"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if lr.req.Status != dom.LeaveStatusApproved {
		t.Fatalf("status = %s, want APPROVED", lr.req.Status)
	}
	// meter committed: pending → used.
	w := store.windowFor("SWP-EMP-3001", "SWP-LT-001", "2026")
	if w.UsedDays != 1 || w.PendingDays != 0 {
		t.Errorf("window=%+v want used 1 pending 0", w)
	}
	// INV-3 loop-closer fired: cancel + one approved_leave_day inserted (1-day leave).
	if len(sched.cancelled) != 1 {
		t.Errorf("CancelScheduleEntriesForLeave calls = %d, want 1", len(sched.cancelled))
	}
	if sched.inserted != 1 {
		t.Errorf("approved_leave_days inserts = %d, want 1", sched.inserted)
	}
}

func TestOnApproved_AlreadyApprovedIdempotent(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusApproved, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	sched := &fakeSchedule{}
	s := newMeterSvc(lr, sched, newMemStore(), memReader{cap: capAnnual(), annual: iptr(12)})
	if err := s.OnApproved(hrCtx(), fakeTx{}, "SWP-LR-8001"); err != nil {
		t.Fatalf("idempotent re-fire should be a no-op, got %v", err)
	}
	if sched.inserted != 0 || len(sched.cancelled) != 0 {
		t.Errorf("idempotent OnApproved fired side-effects: cancelled=%d inserted=%d", len(sched.cancelled), sched.inserted)
	}
}

func TestOnApproved_NotFound(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPending, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	s := newSvc(lr, &fakeSchedule{})
	err := s.OnApproved(hrCtx(), fakeTx{}, "SWP-LR-NOPE")
	if got := codeOf(t, err); got != "NOT_FOUND" {
		t.Fatalf("code = %s, want NOT_FOUND", got)
	}
}

// --- OnRejected hook (engine terminal REJECTED): release reservation + REJECTED ---

func TestOnRejected_ReleasesAndSetsRejected(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPending, "SWP-CMP-0021", "SWP-EMP-3001", 2), leaveType: annualType()}
	store := newMemStore()
	store.seed("SWP-EMP-3001", "SWP-LT-001", "2026", 12, 0, 2) // 2 days reserved at submit
	s := newMeterSvc(lr, &fakeSchedule{}, store, memReader{cap: capAnnual(), annual: iptr(12)})

	if err := s.OnRejected(hrCtx(), fakeTx{}, "SWP-LR-8001"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if lr.req.Status != dom.LeaveStatusRejected {
		t.Fatalf("status = %s, want REJECTED", lr.req.Status)
	}
	// the reservation was released (pending back to 0, nothing used).
	w := store.windowFor("SWP-EMP-3001", "SWP-LT-001", "2026")
	if w.PendingDays != 0 || w.UsedDays != 0 {
		t.Errorf("window=%+v want pending 0 used 0 (released)", w)
	}
}

func TestOnRejected_AlreadyRejectedIdempotent(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusRejected, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	s := newSvc(lr, &fakeSchedule{})
	if err := s.OnRejected(hrCtx(), fakeTx{}, "SWP-LR-8001"); err != nil {
		t.Fatalf("idempotent re-fire should be a no-op, got %v", err)
	}
}

// --- cancel-approved actor guard ---

func TestCancelApproved_AgentPastLeaveBlocked(t *testing.T) {
	req := newReq(dom.LeaveStatusApproved, "SWP-CMP-0021", "SWP-EMP-3001", 1)
	req.StartDate = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) // past (before fixedNow)
	lr := &fakeLeaveRepo{req: req}
	s := newSvc(lr, &fakeSchedule{})
	_, err := s.CancelApproved(agentCtx("SWP-EMP-3001"), "SWP-LR-8001", "Berubah pikiran.")
	if got := codeOf(t, err); got != "RULE_VIOLATION" {
		t.Fatalf("code = %s, want RULE_VIOLATION (agent cannot cancel started leave)", got)
	}
}

// --- cancel (withdraw a PENDING request) releases the reservation ---

func TestCancel_PendingReleasesReservation(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPending, "SWP-CMP-0021", "SWP-EMP-3001", 2), leaveType: annualType()}
	store := newMemStore()
	store.seed("SWP-EMP-3001", "SWP-LT-001", "2026", 12, 0, 2)
	s := newMeterSvc(lr, &fakeSchedule{}, store, memReader{cap: capAnnual(), annual: iptr(12)})

	out, err := s.Cancel(agentCtx("SWP-EMP-3001"), "SWP-LR-8001", "Berubah pikiran.")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusCancelled {
		t.Fatalf("status = %s, want CANCELLED", out.Status)
	}
	if w := store.windowFor("SWP-EMP-3001", "SWP-LT-001", "2026"); w.PendingDays != 0 {
		t.Errorf("window pending = %d, want 0 (released)", w.PendingDays)
	}
}

func TestCancel_TerminalRejectedConflict(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusRejected, "SWP-CMP-0021", "SWP-EMP-3001", 1)}
	s := newSvc(lr, &fakeSchedule{})
	_, err := s.Cancel(hrCtx(), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "CONFLICT" {
		t.Fatalf("code = %s, want CONFLICT", got)
	}
}
