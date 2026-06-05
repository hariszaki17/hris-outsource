// Package leave — unit tests for the two-level approval state machine, the quota
// guard, and the INV-3 loop-closer. Uses in-memory fake repos + a fakeTx so the
// audit-in-tx + side-effect writes run without Postgres. Mirrors the Phase-7
// attendance testkit shape (fakeTx Exec no-op, dynamic principal via auth.WithPrincipal).
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
func (fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("Query unused")
}
func (fakeTx) QueryRow(context.Context, string, ...any) pgx.Row { panic("QueryRow unused") }
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

func (fakeRunner) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return fn(fakeTx{})
}

// --- fake leave repo ---

type fakeLeaveRepo struct {
	req       dom.LeaveRequest
	leaveType LeaveTypeInfo
	approvals []dom.LeaveApproval
	updated   *UpdateStatusParams
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
func (f *fakeLeaveRepo) InsertLeaveApproval(_ context.Context, _ pgx.Tx, p ApprovalRow) (dom.LeaveApproval, error) {
	a := dom.LeaveApproval{LeaveRequestID: p.LeaveRequestID, Stage: p.Stage, Decision: p.Decision, IsOverride: p.IsOverride, OccurredAt: time.Now()}
	f.approvals = append(f.approvals, a)
	return a, nil
}
func (f *fakeLeaveRepo) ListLeaveApprovalsForRequest(context.Context, string) ([]dom.LeaveApproval, error) {
	return f.approvals, nil
}
func (f *fakeLeaveRepo) GetLeaveType(context.Context, string) (LeaveTypeInfo, error) {
	return f.leaveType, nil
}
func (f *fakeLeaveRepo) ListCalendarEntries(context.Context, CalendarFilter, []string, time.Time, time.Time) ([]dom.LeaveCalendarEntry, error) {
	return nil, nil
}

// --- fake quota repo ---

type fakeQuotaRepo struct {
	quota       dom.LeaveQuota
	pending     int
	deducted    int
	overrideSet bool
}

func (f *fakeQuotaRepo) ListLeaveQuotas(context.Context, QuotaFilter) ([]dom.LeaveQuota, error) {
	return []dom.LeaveQuota{f.quota}, nil
}
func (f *fakeQuotaRepo) GetLeaveQuota(context.Context, string) (dom.LeaveQuota, error) {
	return f.quota, nil
}
func (f *fakeQuotaRepo) GetLeaveQuotaForUpdate(context.Context, pgx.Tx, string) (dom.LeaveQuota, error) {
	return f.quota, nil
}
func (f *fakeQuotaRepo) FindQuotaForEmployeeTypePeriod(context.Context, string, string, int) (dom.LeaveQuota, error) {
	if f.quota.ID == "" {
		return dom.LeaveQuota{}, domain.ErrNotFound
	}
	return f.quota, nil
}
func (f *fakeQuotaRepo) UpsertLeaveQuota(_ context.Context, _ pgx.Tx, p UpsertQuotaParams) (dom.LeaveQuota, error) {
	return dom.LeaveQuota{ID: "SWP-LQ-X", EmployeeID: p.EmployeeID, Total: p.Total, Used: f.quota.Used, IsProrated: p.IsProrated, ProrateMonths: p.ProrateMonths}, nil
}
func (f *fakeQuotaRepo) AdjustLeaveQuotaTotal(_ context.Context, _ pgx.Tx, id string, delta int, _ dom.LeaveQuotaAdjustment) (dom.LeaveQuota, error) {
	f.quota.Total += delta
	return f.quota, nil
}
func (f *fakeQuotaRepo) DeductLeaveQuota(_ context.Context, _ pgx.Tx, _ string, delta int) (dom.LeaveQuota, error) {
	f.deducted += delta
	f.quota.Used += delta
	return f.quota, nil
}
func (f *fakeQuotaRepo) RestoreLeaveQuota(_ context.Context, _ pgx.Tx, _ string, delta int) (dom.LeaveQuota, error) {
	f.quota.Used -= delta
	return f.quota, nil
}
func (f *fakeQuotaRepo) SetLeaveQuotaOverride(_ context.Context, _ pgx.Tx, _ string, _ dom.LeaveQuotaOverride) (dom.LeaveQuota, error) {
	f.overrideSet = true
	return f.quota, nil
}
func (f *fakeQuotaRepo) CountPendingLeaveDaysForQuota(context.Context, string, string, time.Time, time.Time) (int, error) {
	return f.pending, nil
}
func (f *fakeQuotaRepo) ListActivePlacedEmployeesForGrant(context.Context, time.Time, time.Time) ([]GrantCandidate, error) {
	return []GrantCandidate{{EmployeeID: "SWP-EMP-3001", PlacementStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}, nil
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

// --- helpers ---

func hrCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-HR", EmployeeID: "SWP-EMP-HR", Role: auth.RoleHRAdmin})
}
func leaderCtx(company, emp string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-LD", EmployeeID: emp, Role: auth.RoleShiftLeader, CompanyID: company})
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

func newSvc(lr *fakeLeaveRepo, qr *fakeQuotaRepo, sp *fakeSchedule) *LeaveService {
	return NewLeaveService(lr, qr, sp, fakeRunner{})
}

func annualQuota(total, used int) dom.LeaveQuota {
	return dom.LeaveQuota{ID: "SWP-LQ-1", EmployeeID: "SWP-EMP-3001", LeaveTypeID: "SWP-LT-001", Period: 2026, Total: total, Used: used,
		PeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected *apperr.Error, got %v", err)
	}
	return ae.Code
}

// --- approve-l1 ---

func TestApproveL1_LeaderForwardsToHR(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingL1, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: LeaveTypeInfo{ID: "SWP-LT-001", IsAnnual: true}}
	qr := &fakeQuotaRepo{quota: annualQuota(12, 4)}
	s := newSvc(lr, qr, &fakeSchedule{})
	out, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusPendingHR {
		t.Fatalf("status = %s, want PENDING_HR", out.Status)
	}
	if qr.deducted != 0 {
		t.Fatalf("quota deducted at L1: %d", qr.deducted)
	}
	if len(lr.approvals) != 1 || lr.approvals[0].Stage != dom.StageL1 {
		t.Fatalf("expected one L1 approval row")
	}
}

func TestApproveL1_WrongState409(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 1)}
	s := newSvc(lr, &fakeQuotaRepo{}, &fakeSchedule{})
	_, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "CONFLICT" {
		t.Fatalf("code = %s, want CONFLICT", got)
	}
}

func TestApproveL1_CrossCompany403(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingL1, "SWP-CMP-0022", "SWP-EMP-3001", 1)}
	s := newSvc(lr, &fakeQuotaRepo{}, &fakeSchedule{})
	_, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "OUT_OF_SCOPE" {
		t.Fatalf("code = %s, want OUT_OF_SCOPE", got)
	}
}

func TestApproveL1_SelfApprove403(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingL1, "SWP-CMP-0021", "SWP-EMP-2003", 1)}
	s := newSvc(lr, &fakeQuotaRepo{}, &fakeSchedule{})
	_, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "FORBIDDEN" {
		t.Fatalf("code = %s, want FORBIDDEN", got)
	}
}

// --- approve-final ---

func TestApproveFinal_DeductsAndFiresINV3(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: LeaveTypeInfo{ID: "SWP-LT-001", Code: "ANNUAL", IsAnnual: true}}
	qr := &fakeQuotaRepo{quota: annualQuota(12, 4)}
	sp := &fakeSchedule{}
	s := newSvc(lr, qr, sp)
	out, err := s.ApproveFinal(hrCtx(), "SWP-LR-8001", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusApproved {
		t.Fatalf("status = %s, want APPROVED", out.Status)
	}
	if qr.deducted != 1 {
		t.Fatalf("deducted = %d, want 1", qr.deducted)
	}
	if sp.inserted != 1 {
		t.Fatalf("approved_leave_days inserts = %d, want 1", sp.inserted)
	}
	if len(out.ScheduleImpact) != 1 || out.ScheduleImpact[0].NewStatus != "LEAVE" {
		t.Fatalf("schedule_impact new_status not mapped to LEAVE: %+v", out.ScheduleImpact)
	}
}

func TestApproveFinal_OverBalance422(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0022", "SWP-EMP-2891", 3), leaveType: LeaveTypeInfo{ID: "SWP-LT-001", IsAnnual: true}}
	qr := &fakeQuotaRepo{quota: annualQuota(12, 11)} // remaining 1 < 3
	s := newSvc(lr, qr, &fakeSchedule{})
	_, err := s.ApproveFinal(hrCtx(), "SWP-LR-8001", "")
	ae, ok := apperr.As(err)
	if !ok || ae.Code != "BALANCE_RECHECK_FAILED" {
		t.Fatalf("code = %v, want BALANCE_RECHECK_FAILED", err)
	}
	if ae.HTTPStatus != 422 {
		t.Fatalf("status = %d, want 422", ae.HTTPStatus)
	}
	if qr.deducted != 0 {
		t.Fatalf("quota deducted on a blocked approval")
	}
}

func TestApproveOverride_ForceApproves(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0022", "SWP-EMP-2891", 3), leaveType: LeaveTypeInfo{ID: "SWP-LT-001", IsAnnual: true}}
	qr := &fakeQuotaRepo{quota: annualQuota(12, 11)}
	sp := &fakeSchedule{}
	s := newSvc(lr, qr, sp)
	out, err := s.ApproveOverride(hrCtx(), "SWP-LR-8001", "Disetujui direktur operasional retroaktif.")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusApproved {
		t.Fatalf("status = %s, want APPROVED", out.Status)
	}
	if qr.deducted != 3 {
		t.Fatalf("deducted = %d, want 3", qr.deducted)
	}
	if !qr.overrideSet {
		t.Fatalf("last_override not recorded")
	}
	if sp.inserted != 1 {
		t.Fatalf("INV-3 not fired on override")
	}
}

// --- reject ---

func TestReject_Terminal(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 1)}
	s := newSvc(lr, &fakeQuotaRepo{}, &fakeSchedule{})
	out, err := s.Reject(hrCtx(), "SWP-LR-8001", "Coverage tidak mencukupi.")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusRejected {
		t.Fatalf("status = %s, want REJECTED", out.Status)
	}
}

func TestReject_AlreadyTerminal409(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusApproved, "SWP-CMP-0021", "SWP-EMP-3001", 1)}
	s := newSvc(lr, &fakeQuotaRepo{}, &fakeSchedule{})
	_, err := s.Reject(hrCtx(), "SWP-LR-8001", "terlambat")
	if got := codeOf(t, err); got != "CONFLICT" {
		t.Fatalf("code = %s, want CONFLICT", got)
	}
}
