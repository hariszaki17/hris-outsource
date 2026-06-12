// Package leave — unit tests for the two-level approval state machine + the F6.1
// grant-lot ledger: FIFO allocation across lots, earmark isolation (LQ-10), the
// reserve-on-submit / commit-on-approve / release-on-reject lifecycle, exact-row
// reversal on cancel-approved, and the never-negative guard. Uses in-memory fake repos
// + a fakeTx so the audit-in-tx + side-effect writes run without Postgres.
package leave

import (
	"context"
	"sort"
	"strings"
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

// --- fake grant repo (in-memory lot ledger) ---

type fakeGrantEmp struct{ fullName, nik, nip string }

type fakeGrantRepo struct {
	lots      map[string]*dom.LeaveGrant
	emps      map[string]fakeGrantEmp // employee_id → name/nik/nip for ListLeaveBalances
	cons      []dom.LeaveConsumption
	consSeq   int
	deletedFR string
}

func newFakeGrantRepo(lots ...dom.LeaveGrant) *fakeGrantRepo {
	m := map[string]*dom.LeaveGrant{}
	for i := range lots {
		l := lots[i]
		m[l.ID] = &l
	}
	return &fakeGrantRepo{lots: m, emps: map[string]fakeGrantEmp{}}
}

func (f *fakeGrantRepo) activeMatching(employeeID, earmark string, now time.Time) []dom.LeaveGrant {
	var out []dom.LeaveGrant
	for _, l := range f.lots {
		if l.EmployeeID != employeeID || !l.IsActive(now) {
			continue
		}
		if earmark == earmarkPoolSentinel {
			if l.Earmark != nil {
				continue
			}
		} else {
			if l.Earmark == nil || *l.Earmark != earmark {
				continue
			}
		}
		out = append(out, *l)
	}
	// FIFO: soonest expires_at, then granted_at, then id.
	sort.Slice(out, func(i, j int) bool {
		if !out[i].ExpiresAt.Equal(out[j].ExpiresAt) {
			return out[i].ExpiresAt.Before(out[j].ExpiresAt)
		}
		if !out[i].GrantedAt.Equal(out[j].GrantedAt) {
			return out[i].GrantedAt.Before(out[j].GrantedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (f *fakeGrantRepo) CreateLeaveGrant(_ context.Context, _ pgx.Tx, p CreateGrantParams) (dom.LeaveGrant, error) {
	f.consSeq++
	id := "SWP-LG-NEW-" + itoa(f.consSeq)
	g := dom.LeaveGrant{ID: id, EmployeeID: p.EmployeeID, Amount: p.Amount, Source: p.Source, Earmark: p.Earmark, Remark: p.Remark, EffectiveFrom: p.EffectiveFrom, ExpiresAt: p.ExpiresAt}
	f.lots[id] = &g
	return g, nil
}
func (f *fakeGrantRepo) GetLeaveGrant(_ context.Context, id string) (dom.LeaveGrant, error) {
	if l, ok := f.lots[id]; ok {
		return *l, nil
	}
	return dom.LeaveGrant{}, domain.ErrNotFound
}
func (f *fakeGrantRepo) GetLeaveGrantForUpdate(_ context.Context, _ pgx.Tx, id string) (dom.LeaveGrant, error) {
	return f.GetLeaveGrant(context.Background(), id)
}
func (f *fakeGrantRepo) ListLeaveGrants(_ context.Context, fr GrantFilter, now time.Time) ([]dom.LeaveGrant, error) {
	var out []dom.LeaveGrant
	for _, l := range f.lots {
		if fr.EmployeeID != nil && l.EmployeeID != *fr.EmployeeID {
			continue
		}
		if !fr.IncludeExpired && !l.IsActive(now) {
			continue
		}
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt.Before(out[j].ExpiresAt) })
	return out, nil
}
func (f *fakeGrantRepo) ListLeaveBalances(_ context.Context, fl BalanceListFilter, now time.Time) ([]dom.EmployeeLeaveBalance, error) {
	agg := map[string]*dom.EmployeeLeaveBalance{}
	for _, l := range f.lots {
		if !l.IsActive(now) {
			continue
		}
		emp, ok := f.emps[l.EmployeeID]
		if !ok {
			continue
		}
		if fl.Q != nil {
			q := strings.ToLower(*fl.Q)
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
	if fl.CursorFullName != nil && fl.CursorID != nil {
		var trimmed []dom.EmployeeLeaveBalance
		for _, b := range out {
			if b.FullName > *fl.CursorFullName || (b.FullName == *fl.CursorFullName && b.EmployeeID > *fl.CursorID) {
				trimmed = append(trimmed, b)
			}
		}
		out = trimmed
	}
	if fl.Limit > 0 && len(out) > fl.Limit {
		out = out[:fl.Limit]
	}
	return out, nil
}

func (f *fakeGrantRepo) PatchLeaveGrant(_ context.Context, _ pgx.Tx, p PatchGrantParams) (dom.LeaveGrant, error) {
	l := f.lots[p.ID]
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
func (f *fakeGrantRepo) ListConsumptionsForGrant(_ context.Context, grantID string) ([]dom.LeaveConsumption, error) {
	var out []dom.LeaveConsumption
	for _, c := range f.cons {
		if c.GrantID == grantID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeGrantRepo) ListConsumptionsForRequest(_ context.Context, requestID string) ([]dom.LeaveConsumption, error) {
	var out []dom.LeaveConsumption
	for _, c := range f.cons {
		if c.LeaveRequestID == requestID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeGrantRepo) GetActiveLotsForAllocation(_ context.Context, _ pgx.Tx, employeeID, earmarkMatch string, now time.Time) ([]dom.LeaveGrant, error) {
	return f.activeMatching(employeeID, earmarkMatch, now), nil
}
func (f *fakeGrantRepo) ReservePending(_ context.Context, _ pgx.Tx, grantID string, days int) error {
	f.lots[grantID].Pending += days
	return nil
}
func (f *fakeGrantRepo) CommitReservation(_ context.Context, _ pgx.Tx, grantID string, days int) error {
	l := f.lots[grantID]
	l.Pending -= days
	if l.Pending < 0 {
		l.Pending = 0
	}
	l.Consumed += days
	return nil
}
func (f *fakeGrantRepo) ReleasePending(_ context.Context, _ pgx.Tx, grantID string, days int) error {
	l := f.lots[grantID]
	l.Pending -= days
	if l.Pending < 0 {
		l.Pending = 0
	}
	return nil
}
func (f *fakeGrantRepo) ReverseConsumption(_ context.Context, _ pgx.Tx, grantID string, days int) error {
	l := f.lots[grantID]
	l.Consumed -= days
	if l.Consumed < 0 {
		l.Consumed = 0
	}
	return nil
}
func (f *fakeGrantRepo) ApplyConsumption(_ context.Context, _ pgx.Tx, requestID, grantID string, days int) (dom.LeaveConsumption, error) {
	f.consSeq++
	c := dom.LeaveConsumption{ID: "SWP-LC-" + itoa(f.consSeq), LeaveRequestID: requestID, GrantID: grantID, Days: days, CreatedAt: time.Now()}
	f.cons = append(f.cons, c)
	return c, nil
}
func (f *fakeGrantRepo) DeleteConsumptionsForRequest(_ context.Context, _ pgx.Tx, requestID string) error {
	f.deletedFR = requestID
	var keep []dom.LeaveConsumption
	for _, c := range f.cons {
		if c.LeaveRequestID != requestID {
			keep = append(keep, c)
		}
	}
	f.cons = keep
	return nil
}
func (f *fakeGrantRepo) SumActiveBalanceByEarmark(_ context.Context, employeeID string, now time.Time) ([]EarmarkBalanceGroup, error) {
	groups := map[string]*EarmarkBalanceGroup{}
	for _, l := range f.lots {
		if l.EmployeeID != employeeID || !l.IsActive(now) {
			continue
		}
		key := ""
		if l.Earmark != nil {
			key = *l.Earmark
		}
		g := groups[key]
		if g == nil {
			g = &EarmarkBalanceGroup{}
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
	var out []EarmarkBalanceGroup
	for _, g := range groups {
		out = append(out, *g)
	}
	return out, nil
}
func (f *fakeGrantRepo) FindExpiredLotsWithPending(_ context.Context, today time.Time, _ int) ([]ExpiredLot, error) {
	var out []ExpiredLot
	for _, l := range f.lots {
		if l.ExpiresAt.Before(today) && l.Pending > 0 {
			out = append(out, ExpiredLot{ID: l.ID, EmployeeID: l.EmployeeID, ExpiresAt: l.ExpiresAt, PendingDays: l.Pending})
		}
	}
	return out, nil
}
func (f *fakeGrantRepo) ZeroLotPending(_ context.Context, _ pgx.Tx, grantID string) error {
	f.lots[grantID].Pending = 0
	return nil
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

func lot(id, emp string, amount int, expires time.Time, earmark *string) dom.LeaveGrant {
	return dom.LeaveGrant{
		ID: id, EmployeeID: emp, Amount: amount, Source: dom.GrantSourceAnnual,
		Earmark: earmark, GrantedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: expires,
	}
}

func ptr(s string) *string { return &s }

func newSvc(lr *fakeLeaveRepo, gr *fakeGrantRepo, sp *fakeSchedule) *LeaveService {
	gs := NewGrantService(gr, fakeRunner{})
	gs.SetClock(func() time.Time { return fixedNow })
	s := NewLeaveService(lr, gs, sp, fakeRunner{})
	s.SetClock(func() time.Time { return fixedNow })
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

// --- approve-l1 (state machine unchanged) ---

func TestApproveL1_LeaderForwardsToHR(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingL1, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	gr := newFakeGrantRepo(lot("SWP-LG-1", "SWP-EMP-3001", 12, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), nil))
	s := newSvc(lr, gr, &fakeSchedule{})
	out, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusPendingHR {
		t.Fatalf("status = %s, want PENDING_HR", out.Status)
	}
	if gr.lots["SWP-LG-1"].Consumed != 0 {
		t.Fatalf("lot consumed at L1: %d", gr.lots["SWP-LG-1"].Consumed)
	}
}

func TestApproveL1_CrossCompany403(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingL1, "SWP-CMP-0022", "SWP-EMP-3001", 1)}
	s := newSvc(lr, newFakeGrantRepo(), &fakeSchedule{})
	_, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "OUT_OF_SCOPE" {
		t.Fatalf("code = %s, want OUT_OF_SCOPE", got)
	}
}

func TestApproveL1_SelfApprove403(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingL1, "SWP-CMP-0021", "SWP-EMP-2003", 1)}
	s := newSvc(lr, newFakeGrantRepo(), &fakeSchedule{})
	_, err := s.ApproveL1(leaderCtx("SWP-CMP-0021", "SWP-EMP-2003"), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "FORBIDDEN" {
		t.Fatalf("code = %s, want FORBIDDEN", got)
	}
}

// --- lead as L2 (final) approver, scoped to the agent's company ---

// A lead finalizes a PENDING_HR leave for an agent at one of its assigned
// companies (in-scope → succeeds, lot consumed).
func TestApproveFinal_LeadInScope(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 1), leaveType: annualType()}
	gr := newFakeGrantRepo(lot("SWP-LG-1", "SWP-EMP-3001", 12, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), nil))
	s := newSvc(lr, gr, &fakeSchedule{})
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
	gr := newFakeGrantRepo(lot("SWP-LG-1", "SWP-EMP-3001", 12, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), nil))
	s := newSvc(lr, gr, &fakeSchedule{})
	_, err := s.ApproveFinal(leadCtx("SWP-EMP-3004", "SWP-CMP-0021", "SWP-CMP-0022"), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "OUT_OF_SCOPE" {
		t.Fatalf("code = %s, want OUT_OF_SCOPE", got)
	}
}

// --- FIFO allocation across two lots (soonest expiry first) ---

func TestApproveFinal_FIFOAcrossTwoLots(t *testing.T) {
	soon := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 5), leaveType: annualType()}
	gr := newFakeGrantRepo(
		lot("SWP-LG-LATER", "SWP-EMP-3001", 10, later, nil),
		lot("SWP-LG-SOON", "SWP-EMP-3001", 3, soon, nil), // 3 days, expires first
	)
	s := newSvc(lr, gr, &fakeSchedule{})
	out, err := s.ApproveFinal(hrCtx(), "SWP-LR-8001", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusApproved {
		t.Fatalf("status = %s, want APPROVED", out.Status)
	}
	// 5 days: 3 from SOON (exhausted), 2 from LATER.
	if gr.lots["SWP-LG-SOON"].Consumed != 3 {
		t.Fatalf("SOON lot consumed = %d, want 3 (FIFO drains soonest expiry first)", gr.lots["SWP-LG-SOON"].Consumed)
	}
	if gr.lots["SWP-LG-LATER"].Consumed != 2 {
		t.Fatalf("LATER lot consumed = %d, want 2", gr.lots["SWP-LG-LATER"].Consumed)
	}
	cons, _ := gr.ListConsumptionsForRequest(context.Background(), "SWP-LR-8001")
	if len(cons) != 2 {
		t.Fatalf("consumption rows = %d, want 2 (one per lot)", len(cons))
	}
}

// --- earmark isolation (LQ-10) ---

func TestEarmarkIsolation_OrdinaryCannotDrawMaternityLot(t *testing.T) {
	// Only a MATERNITY-earmarked lot exists; an ordinary (pool) annual request must
	// NOT see it → insufficient → BALANCE_RECHECK_FAILED.
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 2), leaveType: annualType()}
	gr := newFakeGrantRepo(lot("SWP-LG-MAT", "SWP-EMP-3001", 90, time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC), ptr("MATERNITY")))
	s := newSvc(lr, gr, &fakeSchedule{})
	_, err := s.ApproveFinal(hrCtx(), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "BALANCE_RECHECK_FAILED" {
		t.Fatalf("code = %s, want BALANCE_RECHECK_FAILED (ordinary request hidden from earmarked lot)", got)
	}
	if gr.lots["SWP-LG-MAT"].Consumed != 0 {
		t.Fatalf("earmarked lot drawn by ordinary request: consumed=%d", gr.lots["SWP-LG-MAT"].Consumed)
	}
}

func TestEarmarkIsolation_MaternityRequestDrawsMaternityLot(t *testing.T) {
	mat := lr_maternity()
	lr := &fakeLeaveRepo{req: mat, leaveType: LeaveTypeInfo{ID: "SWP-LT-MAT", Code: "MATERNITY", IsAnnual: true, Earmark: "MATERNITY"}}
	gr := newFakeGrantRepo(
		lot("SWP-LG-POOL", "SWP-EMP-3001", 12, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), nil),
		lot("SWP-LG-MAT", "SWP-EMP-3001", 90, time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC), ptr("MATERNITY")),
	)
	s := newSvc(lr, gr, &fakeSchedule{})
	if _, err := s.ApproveFinal(hrCtx(), "SWP-LR-MAT", ""); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gr.lots["SWP-LG-MAT"].Consumed != 30 {
		t.Fatalf("maternity lot consumed = %d, want 30", gr.lots["SWP-LG-MAT"].Consumed)
	}
	if gr.lots["SWP-LG-POOL"].Consumed != 0 {
		t.Fatalf("pool lot drawn by maternity request: %d", gr.lots["SWP-LG-POOL"].Consumed)
	}
}

func lr_maternity() dom.LeaveRequest {
	c := "SWP-CMP-0021"
	return dom.LeaveRequest{ID: "SWP-LR-MAT", EmployeeID: "SWP-EMP-3001", CompanyID: &c, LeaveTypeID: "SWP-LT-MAT",
		StartDate: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
		DurationDays: 30, Status: dom.LeaveStatusPendingHR}
}

// --- reserve-on-submit / commit-on-approve / release-on-reject ---

func TestSubmit_ReservesPending(t *testing.T) {
	req := newReq(dom.LeaveStatusDraft, "SWP-CMP-0021", "SWP-EMP-3001", 3)
	lr := &fakeLeaveRepo{req: req, leaveType: annualType()}
	gr := newFakeGrantRepo(lot("SWP-LG-1", "SWP-EMP-3001", 12, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), nil))
	s := newSvc(lr, gr, &fakeSchedule{})
	out, err := s.Submit(hrCtx(), "SWP-LR-8001")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusPendingL1 {
		t.Fatalf("status = %s, want PENDING_L1", out.Status)
	}
	if gr.lots["SWP-LG-1"].Pending != 3 {
		t.Fatalf("pending = %d, want 3 (reserved at submit)", gr.lots["SWP-LG-1"].Pending)
	}
	if lr.snapshot == nil || len(lr.snapshot.Allocation) == 0 {
		t.Fatalf("allocation snapshot not persisted at submit")
	}
}

func TestReject_ReleasesPending(t *testing.T) {
	req := newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 3)
	req.BalanceCheck.Allocation = []dom.AllocationLine{{GrantID: "SWP-LG-1", Days: 3}}
	lr := &fakeLeaveRepo{req: req}
	g := lot("SWP-LG-1", "SWP-EMP-3001", 12, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), nil)
	g.Pending = 3
	gr := newFakeGrantRepo(g)
	s := newSvc(lr, gr, &fakeSchedule{})
	if _, err := s.Reject(hrCtx(), "SWP-LR-8001", "Coverage tidak cukup."); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gr.lots["SWP-LG-1"].Pending != 0 {
		t.Fatalf("pending = %d, want 0 (released on reject)", gr.lots["SWP-LG-1"].Pending)
	}
	if gr.lots["SWP-LG-1"].Consumed != 0 {
		t.Fatalf("consumed grew on reject: %d", gr.lots["SWP-LG-1"].Consumed)
	}
}

func TestApproveFinal_CommitsReservedAllocation(t *testing.T) {
	req := newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 3)
	req.BalanceCheck.Allocation = []dom.AllocationLine{{GrantID: "SWP-LG-1", Days: 3}}
	lr := &fakeLeaveRepo{req: req, leaveType: annualType()}
	g := lot("SWP-LG-1", "SWP-EMP-3001", 12, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), nil)
	g.Pending = 3
	gr := newFakeGrantRepo(g)
	sp := &fakeSchedule{}
	s := newSvc(lr, gr, sp)
	if _, err := s.ApproveFinal(hrCtx(), "SWP-LR-8001", ""); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gr.lots["SWP-LG-1"].Pending != 0 || gr.lots["SWP-LG-1"].Consumed != 3 {
		t.Fatalf("commit pending=%d consumed=%d, want 0/3", gr.lots["SWP-LG-1"].Pending, gr.lots["SWP-LG-1"].Consumed)
	}
	if sp.inserted != 1 {
		t.Fatalf("INV-3 not fired: inserts=%d", sp.inserted)
	}
}

// --- no-negative guard ---

func TestApproveFinal_InsufficientBlocks(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 5), leaveType: annualType()}
	gr := newFakeGrantRepo(lot("SWP-LG-1", "SWP-EMP-3001", 3, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), nil)) // only 3
	s := newSvc(lr, gr, &fakeSchedule{})
	_, err := s.ApproveFinal(hrCtx(), "SWP-LR-8001", "")
	if got := codeOf(t, err); got != "BALANCE_RECHECK_FAILED" {
		t.Fatalf("code = %s, want BALANCE_RECHECK_FAILED", got)
	}
	if gr.lots["SWP-LG-1"].Consumed != 0 {
		t.Fatalf("lot consumed on a blocked approval: %d (never negative)", gr.lots["SWP-LG-1"].Consumed)
	}
}

// --- cancel-approved reverses the exact consumption rows ---

func TestCancelApproved_ReversesExactConsumptions(t *testing.T) {
	req := newReq(dom.LeaveStatusApproved, "SWP-CMP-0021", "SWP-EMP-3001", 3)
	req.StartDate = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) // future
	req.EndDate = time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	lr := &fakeLeaveRepo{req: req}
	g := lot("SWP-LG-1", "SWP-EMP-3001", 12, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), nil)
	g.Consumed = 3
	gr := newFakeGrantRepo(g)
	gr.cons = []dom.LeaveConsumption{{ID: "SWP-LC-1", LeaveRequestID: "SWP-LR-8001", GrantID: "SWP-LG-1", Days: 3}}
	s := newSvc(lr, gr, &fakeSchedule{})
	out, err := s.CancelApproved(hrCtx(), "SWP-LR-8001", "Klien minta agen kembali.")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Status != dom.LeaveStatusCancelled {
		t.Fatalf("status = %s, want CANCELLED", out.Status)
	}
	if gr.lots["SWP-LG-1"].Consumed != 0 {
		t.Fatalf("consumed = %d, want 0 (reversed)", gr.lots["SWP-LG-1"].Consumed)
	}
	if gr.deletedFR != "SWP-LR-8001" {
		t.Fatalf("consumption rows not deleted for the request")
	}
}

func TestCancelApproved_AgentPastLeaveBlocked(t *testing.T) {
	req := newReq(dom.LeaveStatusApproved, "SWP-CMP-0021", "SWP-EMP-3001", 1)
	req.StartDate = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) // past (before fixedNow)
	lr := &fakeLeaveRepo{req: req}
	gr := newFakeGrantRepo()
	s := newSvc(lr, gr, &fakeSchedule{})
	_, err := s.CancelApproved(agentCtx("SWP-EMP-3001"), "SWP-LR-8001", "Berubah pikiran.")
	if got := codeOf(t, err); got != "RULE_VIOLATION" {
		t.Fatalf("code = %s, want RULE_VIOLATION (agent cannot cancel started leave)", got)
	}
}

// --- reject already-terminal 409 ---

func TestReject_AlreadyTerminal409(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusApproved, "SWP-CMP-0021", "SWP-EMP-3001", 1)}
	s := newSvc(lr, newFakeGrantRepo(), &fakeSchedule{})
	_, err := s.Reject(hrCtx(), "SWP-LR-8001", "terlambat")
	if got := codeOf(t, err); got != "CONFLICT" {
		t.Fatalf("code = %s, want CONFLICT", got)
	}
}

// --- expiry sweep ---

func TestExpirySweep_ReleasesDanglingPending(t *testing.T) {
	g := lot("SWP-LG-EXP", "SWP-EMP-3001", 12, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil) // expired (before fixedNow 2026-06-01)
	g.Pending = 2                                                                                // dangling
	gr := newFakeGrantRepo(g)
	sweep := NewLeaveExpirySweepService(gr, fakeRunner{}, 0)
	sweep.SetClock(func() time.Time { return fixedNow })
	n, err := sweep.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if n != 1 {
		t.Fatalf("swept = %d, want 1", n)
	}
	if gr.lots["SWP-LG-EXP"].Pending != 0 {
		t.Fatalf("dangling pending not released: %d", gr.lots["SWP-LG-EXP"].Pending)
	}
}

// --- HR grant create + balance shape ---

func TestCreateGrant_AndBalance(t *testing.T) {
	gr := newFakeGrantRepo()
	gs := NewGrantService(gr, fakeRunner{})
	gs.SetClock(func() time.Time { return fixedNow })
	g, err := gs.Create(hrCtx(), CreateGrantParams{
		EmployeeID: "SWP-EMP-3001", Amount: 12, Source: dom.GrantSourceAnnual,
		Remark: ptr("Hibah kuota tahunan 2026."), ExpiresAt: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if g.Amount != 12 || g.Remaining() != 12 {
		t.Fatalf("grant amount/remaining = %d/%d, want 12/12", g.Amount, g.Remaining())
	}
	// add a maternity earmarked lot then check balance shape.
	if _, err := gs.Create(hrCtx(), CreateGrantParams{
		EmployeeID: "SWP-EMP-3001", Amount: 90, Source: dom.GrantSourceMaternity, Earmark: ptr("MATERNITY"),
		Remark: ptr("Pre-fund cuti melahirkan."), ExpiresAt: time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("unexpected err on maternity grant: %v", err)
	}
	bal, err := gs.Balance(hrCtx(), "SWP-EMP-3001", false)
	if err != nil {
		t.Fatalf("balance err: %v", err)
	}
	if bal.PoolRemaining != 12 {
		t.Fatalf("pool_remaining = %d, want 12 (earmarked lot excluded)", bal.PoolRemaining)
	}
	if len(bal.Earmarked) != 1 || bal.Earmarked[0].Earmark != "MATERNITY" || bal.Earmarked[0].Remaining != 90 {
		t.Fatalf("earmarked line wrong: %+v", bal.Earmarked)
	}
}

// --- aggregate per-employee balance LIST (GET /leave-balances) ---

func TestListBalances_AggregateAndPaginate(t *testing.T) {
	gr := newFakeGrantRepo()
	gr.emps["SWP-EMP-1001"] = fakeGrantEmp{fullName: "Andi", nik: "NIK-1001", nip: "NIP-1001"}
	gr.emps["SWP-EMP-1002"] = fakeGrantEmp{fullName: "Budi", nik: "NIK-1002", nip: "NIP-1002"}
	gr.emps["SWP-EMP-1003"] = fakeGrantEmp{fullName: "Citra", nik: "NIK-1003", nip: "NIP-1003"}
	put := func(id, emp string, amount, consumed, pending int, earmark string, exp time.Time) {
		var em *string
		if earmark != "" {
			e := earmark
			em = &e
		}
		gr.lots[id] = &dom.LeaveGrant{ID: id, EmployeeID: emp, Amount: amount, Consumed: consumed, Pending: pending, Earmark: em, ExpiresAt: exp}
	}
	// Andi: pool 12-4-0=8 + earmarked 90; soonest active w/ remaining = 2026-09-30.
	put("L1", "SWP-EMP-1001", 12, 4, 0, "", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	put("L2", "SWP-EMP-1001", 5, 1, 1, "", time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))
	put("L3", "SWP-EMP-1001", 90, 0, 0, "MATERNITY", time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC))
	put("L4", "SWP-EMP-1002", 6, 0, 0, "", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	put("L5", "SWP-EMP-1003", 7, 0, 0, "", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	// expired-only employee — excluded.
	gr.emps["SWP-EMP-1099"] = fakeGrantEmp{fullName: "Zaki", nik: "NIK-1099", nip: "NIP-1099"}
	put("L9", "SWP-EMP-1099", 5, 0, 0, "", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	gs := NewGrantService(gr, fakeRunner{})
	gs.SetClock(func() time.Time { return fixedNow }) // 2026-06-01

	rows, next, hasMore, err := gs.ListBalances(hrCtx(), BalanceListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rows) != 2 || !hasMore || next == nil {
		t.Fatalf("page1: rows=%d hasMore=%v next=%v, want 2/true/non-nil", len(rows), hasMore, next)
	}
	andi := rows[0]
	if andi.FullName != "Andi" || andi.PoolTotal != 17 || andi.PoolRemaining != 11 || andi.EarmarkedRemaining != 90 || andi.LotCount != 3 {
		t.Fatalf("andi aggregate wrong: %+v", andi)
	}
	if andi.NextExpiry == nil || !andi.NextExpiry.Equal(time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("andi next_expiry = %v, want 2026-09-30", andi.NextExpiry)
	}

	// page 2 via cursor: Citra remains (expired-only Zaki excluded).
	fn, id, derr := DecodeBalanceCursor(*next)
	if derr != nil {
		t.Fatalf("decode cursor: %v", derr)
	}
	rows2, _, hasMore2, err := gs.ListBalances(hrCtx(), BalanceListFilter{Limit: 2, CursorFullName: fn, CursorID: id})
	if err != nil {
		t.Fatalf("page2 err: %v", err)
	}
	if len(rows2) != 1 || hasMore2 || rows2[0].FullName != "Citra" {
		t.Fatalf("page2 = %+v hasMore=%v, want [Citra] / false", rows2, hasMore2)
	}
}

func TestCreateGrant_NegativeAmountRejected(t *testing.T) {
	gs := NewGrantService(newFakeGrantRepo(), fakeRunner{})
	gs.SetClock(func() time.Time { return fixedNow })
	_, err := gs.Create(hrCtx(), CreateGrantParams{EmployeeID: "SWP-EMP-3001", Amount: -1, Source: dom.GrantSourceAdjustment, Remark: ptr("salah"), ExpiresAt: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)})
	if got := codeOf(t, err); got != "INVALID_REQUEST" {
		t.Fatalf("code = %s, want INVALID_REQUEST", got)
	}
}
