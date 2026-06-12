// Package leave — unit tests for the two-level approval state machine + the F6.1
// grant-lot ledger: FIFO allocation across lots, earmark isolation (LQ-10), the
// reserve-on-submit / commit-on-approve / release-on-reject lifecycle, exact-row
// reversal on cancel-approved, and the never-negative guard. Uses in-memory fake repos
// + a fakeTx so the audit-in-tx + side-effect writes run without Postgres.
package leave

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
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

// --- fake leave repo ---

type fakeLeaveRepo struct {
	req       dom.LeaveRequest
	leaveType LeaveTypeInfo
	approvals []dom.LeaveApproval
	updated   *UpdateStatusParams
	snapshot  *BalanceSnapshotParams
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
func (f *fakeLeaveRepo) InsertLeaveApproval(_ context.Context, _ pgx.Tx, p ApprovalRow) (dom.LeaveApproval, error) {
	a := dom.LeaveApproval{LeaveRequestID: p.LeaveRequestID, Stage: p.Stage, Decision: p.Decision, IsOverride: p.IsOverride, OccurredAt: time.Now()}
	f.approvals = append(f.approvals, a)
	return a, nil
}
func (f *fakeLeaveRepo) ListLeaveApprovalsForRequest(context.Context, string) ([]dom.LeaveApproval, error) {
	return f.approvals, nil
}
func (f *fakeLeaveRepo) CreateLeaveRequest(_ context.Context, _ pgx.Tx, p CreateLeaveRequestParams) (dom.LeaveRequest, error) {
	r := dom.LeaveRequest{
		ID: "SWP-LR-NEW", EmployeeID: p.EmployeeID, LeaveTypeID: p.LeaveTypeID,
		StartDate: p.StartDate, EndDate: p.EndDate, DurationDays: p.DurationDays,
		Reason: p.Reason, Status: p.Status, DelegateID: p.DelegateID, DocumentFileID: p.DocumentFileID,
		Backdated: p.Backdated, Routing: dom.LeaveRouting{NoLeader: p.NoLeader, AssignedLeaderID: p.AssignedLeaderID},
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
func leaderCtx(company, emp string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-LD", EmployeeID: emp, Role: auth.RoleShiftLeader, CompanyID: company})
}
func agentCtx(emp string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-AG", EmployeeID: emp, Role: auth.RoleAgent})
}

// leadCtx builds a stored `lead` principal whose company SET (CompanyIDs) the
// middleware would have resolved per-request from lead_assignments.
func leadCtx(emp string, companies ...string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-LEAD", EmployeeID: emp, Role: auth.RoleLead, CompanyIDs: companies})
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
// ledger, 2026-06-12). State-machine / auth tests use it; the metering specifics are
// covered in leave_service_meter_test.go.
func newSvc(lr *fakeLeaveRepo, sp *fakeSchedule) *LeaveService {
	return newMeterSvc(lr, sp, newMemStore(), memReader{cap: capAnnual(), annual: iptr(12)})
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

// --- approve-l1 (state machine) ---

func TestApproveL1_LeaderForwardsToHR(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingL1, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	s := newSvc(lr, &fakeSchedule{})
	out, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusPendingHR {
		t.Fatalf("status = %s, want PENDING_HR", out.Status)
	}
}

func TestApproveL1_CrossCompany403(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingL1, "SWP-CMP-0022", "SWP-EMP-3001", 1)}
	s := newSvc(lr, &fakeSchedule{})
	_, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "OUT_OF_SCOPE" {
		t.Fatalf("code = %s, want OUT_OF_SCOPE", got)
	}
}

func TestApproveL1_SelfApprove403(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingL1, "SWP-CMP-0021", "SWP-EMP-2003", 1)}
	s := newSvc(lr, &fakeSchedule{})
	_, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "FORBIDDEN" {
		t.Fatalf("code = %s, want FORBIDDEN", got)
	}
}

// --- lead as L2 (final) approver, scoped to the agent's company ---

// A lead finalizes a PENDING_HR leave for an agent at one of its assigned
// companies (in-scope → succeeds, window committed).
func TestApproveFinal_LeadInScope(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	s := newSvc(lr, &fakeSchedule{})
	out, err := s.ApproveFinal(leadCtx("SWP-EMP-3004", "SWP-CMP-0021", "SWP-CMP-0022"), "SWP-LR-8001", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusApproved {
		t.Fatalf("status = %s, want APPROVED", out.Status)
	}
}

// A lead cannot finalize a leave for an agent at a company OUTSIDE its set.
func TestApproveFinal_LeadOutOfScope(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0099", "SWP-EMP-3001", 1), leaveType: annualType()}
	s := newSvc(lr, &fakeSchedule{})
	_, err := s.ApproveFinal(leadCtx("SWP-EMP-3004", "SWP-CMP-0021", "SWP-CMP-0022"), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "OUT_OF_SCOPE" {
		t.Fatalf("code = %s, want OUT_OF_SCOPE", got)
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

// --- reject already-terminal 409 ---

func TestReject_AlreadyTerminal409(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusApproved, "SWP-CMP-0021", "SWP-EMP-3001", 1)}
	s := newSvc(lr, &fakeSchedule{})
	_, err := s.Reject(hrCtx(), "SWP-LR-8001", "terlambat")
	if got := codeOf(t, err); got != "CONFLICT" {
		t.Fatalf("code = %s, want CONFLICT", got)
	}
}
