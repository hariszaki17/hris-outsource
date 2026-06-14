// Package overtime — unit tests for the E11-engine seam on OvertimeService:
// Create/Confirm enter PENDING + drive engine.CreateInstance + link the instance,
// Withdraw collapses to CANCELLED, and the terminal hooks OnApproved/OnRejected
// (called by the approval engine inside its tx) flip the record + notify.
//
// These are package-internal (white-box) tests over minimal in-memory fakes —
// no Postgres, no River. They complement the handler-level contract tests in
// internal/handler/overtime.
package overtime

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	approval "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	schedulingsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

var svcNow = time.Date(2026, 6, 4, 5, 0, 0, 0, time.UTC)

func svcYMD(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func sp(s string) *string { return &s }

// --- fakeTx (Exec no-op for audit.Record) + runner ---

type svcFakeTx struct{}

func (svcFakeTx) Begin(context.Context) (pgx.Tx, error)  { return svcFakeTx{}, nil }
func (svcFakeTx) Commit(context.Context) error           { return nil }
func (svcFakeTx) Rollback(context.Context) error         { return nil }
func (svcFakeTx) Conn() *pgx.Conn                         { return nil }
func (svcFakeTx) LargeObjects() pgx.LargeObjects          { panic("n/a") }
func (svcFakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("n/a")
}
func (svcFakeTx) QueryRow(context.Context, string, ...any) pgx.Row { panic("n/a") }
func (svcFakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (svcFakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("n/a")
}
func (svcFakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("n/a") }
func (svcFakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("n/a")
}

type svcTxRunner struct{}

func (svcTxRunner) InTx(_ context.Context, fn func(pgx.Tx) error) error { return fn(svcFakeTx{}) }

// --- in-memory repo ---

type svcRepo struct {
	records map[string]dom.Overtime
	rule    *OvertimeRule
	seq     int
}

func newSvcRepo() *svcRepo { return &svcRepo{records: map[string]dom.Overtime{}} }

func (r *svcRepo) ListOvertime(context.Context, OvertimeFilter) ([]dom.Overtime, error) {
	out := make([]dom.Overtime, 0, len(r.records))
	for _, rec := range r.records {
		out = append(out, rec)
	}
	return out, nil
}

func (r *svcRepo) GetOvertime(_ context.Context, id string) (dom.Overtime, error) {
	rec, ok := r.records[id]
	if !ok {
		return dom.Overtime{}, domain.ErrNotFound
	}
	return rec, nil
}

func (r *svcRepo) GetOvertimeForUpdate(_ context.Context, _ pgx.Tx, id string) (dom.Overtime, error) {
	return r.GetOvertime(context.Background(), id)
}

func (r *svcRepo) UpdateOvertimeStatus(_ context.Context, _ pgx.Tx, id string, status dom.OvertimeStatus) (dom.Overtime, error) {
	rec, ok := r.records[id]
	if !ok {
		return dom.Overtime{}, domain.ErrNotFound
	}
	rec.Status = status
	rec.UpdatedAt = svcNow
	r.records[id] = rec
	return rec, nil
}

func (r *svcRepo) InsertOvertime(_ context.Context, _ pgx.Tx, p OvertimeInsertParams) (dom.Overtime, error) {
	r.seq++
	id := "SWP-OT-9" + string(rune('0'+r.seq)) // deterministic-ish, single-insert tests only
	rec := dom.Overtime{
		ID:               id,
		EmployeeID:       p.EmployeeID,
		CompanyID:        p.CompanyID,
		PlacementID:      p.PlacementID,
		WorkDate:         p.WorkDate,
		PlannedStartTime: p.PlannedStartTime,
		PlannedEndTime:   p.PlannedEndTime,
		CrossMidnight:    p.CrossMidnight,
		Source:           p.Source,
		Status:           p.Status,
		DayType:          p.DayType,
		HolidayID:        p.HolidayID,
		Reason:           p.Reason,
		CreatedBy:        p.CreatedBy,
		CreatedAt:        svcNow,
		UpdatedAt:        svcNow,
	}
	r.records[id] = rec
	return rec, nil
}

func (r *svcRepo) SetApprovalInstanceID(_ context.Context, _ pgx.Tx, id, instanceID string) error {
	rec, ok := r.records[id]
	if !ok {
		return domain.ErrNotFound
	}
	rec.ApprovalInstanceID = &instanceID
	r.records[id] = rec
	return nil
}

func (r *svcRepo) FindOvertimeRule(context.Context) (OvertimeRule, error) {
	if r.rule != nil {
		return *r.rule, nil
	}
	return OvertimeRule{}, domain.ErrNotFound
}

var (
	_ OvertimeRepository = (*svcRepo)(nil)
	_ RuleRepository     = (*svcRepo)(nil)
)

// --- holiday + schedule stubs (minimal; classification not exercised here) ---

type svcHolidayRepo struct{}

func (svcHolidayRepo) ListHolidays(context.Context, HolidayFilter) ([]dom.Holiday, error) {
	return nil, nil
}
func (svcHolidayRepo) GetHoliday(context.Context, string) (dom.Holiday, error) {
	return dom.Holiday{}, domain.ErrNotFound
}
func (svcHolidayRepo) GetHolidayByDateCategory(context.Context, time.Time, string) (dom.Holiday, error) {
	return dom.Holiday{}, domain.ErrNotFound
}
func (svcHolidayRepo) GetHolidayForDate(context.Context, time.Time) (dom.Holiday, error) {
	return dom.Holiday{}, domain.ErrNotFound
}
func (svcHolidayRepo) InsertHoliday(context.Context, pgx.Tx, HolidayWriteParams) (dom.Holiday, error) {
	return dom.Holiday{}, nil
}
func (svcHolidayRepo) UpdateHoliday(context.Context, pgx.Tx, string, HolidayUpdateParams) (dom.Holiday, error) {
	return dom.Holiday{}, nil
}
func (svcHolidayRepo) SoftDeleteHoliday(context.Context, pgx.Tx, string) (string, error) {
	return "", nil
}
func (svcHolidayRepo) CountOvertimeUsingHoliday(context.Context, string) (int64, error) { return 0, nil }

var _ HolidayRepository = svcHolidayRepo{}

type svcSchedule struct {
	placement *schedulingsvc.PlacementCover
	leave     *schedulingsvc.ApprovedLeave
	live      *schedulingsvc.LiveEntry
}

func (s svcSchedule) FindLiveEntryForAgentDate(context.Context, string, time.Time) (schedulingsvc.LiveEntry, error) {
	if s.live != nil {
		return *s.live, nil
	}
	return schedulingsvc.LiveEntry{}, domain.ErrNotFound
}
func (s svcSchedule) FindActivePlacementForAgentDate(context.Context, string, time.Time) (schedulingsvc.PlacementCover, error) {
	if s.placement != nil {
		return *s.placement, nil
	}
	return schedulingsvc.PlacementCover{}, domain.ErrNotFound
}
func (s svcSchedule) FindApprovedLeaveForAgentDate(context.Context, string, time.Time) (schedulingsvc.ApprovedLeave, error) {
	if s.leave != nil {
		return *s.leave, nil
	}
	return schedulingsvc.ApprovedLeave{}, domain.ErrNotFound
}

var _ SchedulePort = svcSchedule{}

// --- fake engine + notifier ---

type svcEngine struct {
	calls []approval.CreateInstanceInput
	seq   int
}

func (e *svcEngine) CreateInstance(_ context.Context, _ pgx.Tx, in approval.CreateInstanceInput) (string, error) {
	e.calls = append(e.calls, in)
	e.seq++
	return "SWP-API-000" + string(rune('0'+e.seq)), nil
}

var _ approval.Engine = (*svcEngine)(nil)

type svcNotifier struct{ sent []jobs.NotificationArgs }

func (n *svcNotifier) Dispatch(_ context.Context, _ pgx.Tx, args jobs.NotificationArgs) error {
	n.sent = append(n.sent, args)
	return nil
}

var _ jobs.Dispatcher = (*svcNotifier)(nil)

// --- harness ---

type svcKit struct {
	svc      *OvertimeService
	repo     *svcRepo
	engine   *svcEngine
	notifier *svcNotifier
	sched    svcSchedule
}

func newSvcKit(t *testing.T, sched svcSchedule) *svcKit {
	t.Helper()
	repo := newSvcRepo()
	engine := &svcEngine{}
	notifier := &svcNotifier{}
	s := NewOvertimeService(repo, repo, svcHolidayRepo{}, sched, svcTxRunner{})
	s.SetClock(func() time.Time { return svcNow })
	s.SetApprovalEngine(engine)
	s.SetNotifier(notifier)
	return &svcKit{svc: s, repo: repo, engine: engine, notifier: notifier, sched: sched}
}

func agentCtx(employeeID string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:     "SWP-USR-0001",
		Role:       auth.RoleAgent,
		EmployeeID: employeeID,
	})
}

// --- (a) Create enters PENDING + drives engine + links instance ---

func TestService_Create_EntersPendingAndLinksInstance(t *testing.T) {
	workDate := svcYMD(2026, time.June, 10)
	kit := newSvcKit(t, svcSchedule{
		placement: &schedulingsvc.PlacementCover{PlacementID: "SWP-PL-5001", CompanyID: "SWP-CMP-0021"},
		live:      &schedulingsvc.LiveEntry{ID: "SWP-SCH-1", Status: "PUBLISHED"},
	})

	rec, _, err := kit.svc.Create(agentCtx("SWP-EMP-3001"), CreateOvertimeInput{
		WorkDate:         workDate,
		PlannedStartTime: "15:00",
		PlannedEndTime:   "17:00",
		Reason:           "Cover for absent colleague.",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if rec.Status != dom.OvertimeStatusPending {
		t.Errorf("status = %s, want PENDING", rec.Status)
	}
	if len(kit.engine.calls) != 1 {
		t.Fatalf("engine.CreateInstance calls = %d, want 1", len(kit.engine.calls))
	}
	call := kit.engine.calls[0]
	if call.RequestType != approval.RequestTypeOvertime {
		t.Errorf("RequestType = %s, want OVERTIME", call.RequestType)
	}
	if call.RequesterID != "SWP-EMP-3001" {
		t.Errorf("RequesterID = %s, want SWP-EMP-3001", call.RequesterID)
	}
	if call.CompanyID != "SWP-CMP-0021" {
		t.Errorf("CompanyID = %s, want SWP-CMP-0021", call.CompanyID)
	}
	if rec.ApprovalInstanceID == nil || *rec.ApprovalInstanceID == "" {
		t.Errorf("instance id not linked on the returned record: %v", rec.ApprovalInstanceID)
	}
	if got := kit.repo.records[rec.ID].ApprovalInstanceID; got == nil || *got == "" {
		t.Errorf("instance id not persisted on the stored record: %v", got)
	}
}

// --- (b) Confirm PENDING_AGENT_CONFIRM → PENDING + engine instance ---

func TestService_Confirm_EntersApprovalChain(t *testing.T) {
	kit := newSvcKit(t, svcSchedule{})
	kit.repo.records["SWP-OT-1"] = dom.Overtime{
		ID:         "SWP-OT-1",
		EmployeeID: "SWP-EMP-3001",
		CompanyID:  sp("SWP-CMP-0021"),
		Status:     dom.OvertimeStatusPendingAgentConfirm,
		DayType:    dom.OvertimeTierWorkday,
		WorkDate:   svcYMD(2026, time.June, 2),
		CreatedAt:  svcNow,
		UpdatedAt:  svcNow,
	}

	rec, _, err := kit.svc.Confirm(agentCtx("SWP-EMP-3001"), "SWP-OT-1", "Konfirmasi.")
	if err != nil {
		t.Fatalf("Confirm returned error: %v", err)
	}
	if rec.Status != dom.OvertimeStatusPending {
		t.Errorf("status = %s, want PENDING", rec.Status)
	}
	if len(kit.engine.calls) != 1 {
		t.Fatalf("engine.CreateInstance calls = %d, want 1", len(kit.engine.calls))
	}
	if kit.engine.calls[0].RequestID != "SWP-OT-1" {
		t.Errorf("RequestID = %s, want SWP-OT-1", kit.engine.calls[0].RequestID)
	}
	if got := kit.repo.records["SWP-OT-1"].ApprovalInstanceID; got == nil || *got == "" {
		t.Errorf("instance id not linked after confirm: %v", got)
	}
}

// --- (c) Withdraw → CANCELLED ---

func TestService_Withdraw_Cancels(t *testing.T) {
	kit := newSvcKit(t, svcSchedule{})
	kit.repo.records["SWP-OT-2"] = dom.Overtime{
		ID:         "SWP-OT-2",
		EmployeeID: "SWP-EMP-3001",
		CompanyID:  sp("SWP-CMP-0021"),
		Status:     dom.OvertimeStatusPending,
		CreatedAt:  svcNow,
		UpdatedAt:  svcNow,
	}
	if err := kit.svc.Withdraw(agentCtx("SWP-EMP-3001"), "SWP-OT-2"); err != nil {
		t.Fatalf("Withdraw returned error: %v", err)
	}
	if got := kit.repo.records["SWP-OT-2"].Status; got != dom.OvertimeStatusCancelled {
		t.Errorf("status = %s, want CANCELLED", got)
	}
}

func TestService_Withdraw_TerminalConflict(t *testing.T) {
	kit := newSvcKit(t, svcSchedule{})
	kit.repo.records["SWP-OT-3"] = dom.Overtime{
		ID:         "SWP-OT-3",
		EmployeeID: "SWP-EMP-3001",
		CompanyID:  sp("SWP-CMP-0021"),
		Status:     dom.OvertimeStatusApproved,
		CreatedAt:  svcNow,
		UpdatedAt:  svcNow,
	}
	err := kit.svc.Withdraw(agentCtx("SWP-EMP-3001"), "SWP-OT-3")
	if err == nil {
		t.Fatalf("expected a conflict error withdrawing an APPROVED OT, got nil")
	}
	if got := kit.repo.records["SWP-OT-3"].Status; got != dom.OvertimeStatusApproved {
		t.Errorf("status changed to %s on a rejected withdraw, want APPROVED", got)
	}
}

// --- (d) OnApproved → APPROVED + notify; OnRejected → REJECTED + notify ---

func TestService_OnApproved_FinalizesAndNotifies(t *testing.T) {
	kit := newSvcKit(t, svcSchedule{})
	kit.repo.records["SWP-OT-4"] = dom.Overtime{
		ID:         "SWP-OT-4",
		EmployeeID: "SWP-EMP-3001",
		CompanyID:  sp("SWP-CMP-0021"),
		Status:     dom.OvertimeStatusPending,
		WorkDate:   svcYMD(2026, time.June, 2),
		CreatedAt:  svcNow,
		UpdatedAt:  svcNow,
	}
	if err := kit.svc.OnApproved(context.Background(), svcFakeTx{}, "SWP-OT-4"); err != nil {
		t.Fatalf("OnApproved returned error: %v", err)
	}
	if got := kit.repo.records["SWP-OT-4"].Status; got != dom.OvertimeStatusApproved {
		t.Errorf("status = %s, want APPROVED", got)
	}
	if len(kit.notifier.sent) != 1 {
		t.Fatalf("notifications = %d, want 1", len(kit.notifier.sent))
	}
	n := kit.notifier.sent[0]
	if n.NotifKind != "OT_APPROVED" {
		t.Errorf("notif kind = %s, want OT_APPROVED", n.NotifKind)
	}
	if n.RecipientID != "SWP-EMP-3001" {
		t.Errorf("recipient = %s, want SWP-EMP-3001 (submitter)", n.RecipientID)
	}

	// idempotent: a re-fire on an already-APPROVED record is a no-op (no extra notify).
	if err := kit.svc.OnApproved(context.Background(), svcFakeTx{}, "SWP-OT-4"); err != nil {
		t.Fatalf("idempotent OnApproved returned error: %v", err)
	}
	if len(kit.notifier.sent) != 1 {
		t.Errorf("idempotent re-fire sent %d notifications, want 1", len(kit.notifier.sent))
	}
}

func TestService_OnRejected_FinalizesAndNotifies(t *testing.T) {
	kit := newSvcKit(t, svcSchedule{})
	kit.repo.records["SWP-OT-5"] = dom.Overtime{
		ID:         "SWP-OT-5",
		EmployeeID: "SWP-EMP-3001",
		CompanyID:  sp("SWP-CMP-0021"),
		Status:     dom.OvertimeStatusPending,
		WorkDate:   svcYMD(2026, time.June, 2),
		CreatedAt:  svcNow,
		UpdatedAt:  svcNow,
	}
	if err := kit.svc.OnRejected(context.Background(), svcFakeTx{}, "SWP-OT-5"); err != nil {
		t.Fatalf("OnRejected returned error: %v", err)
	}
	if got := kit.repo.records["SWP-OT-5"].Status; got != dom.OvertimeStatusRejected {
		t.Errorf("status = %s, want REJECTED", got)
	}
	if len(kit.notifier.sent) != 1 {
		t.Fatalf("notifications = %d, want 1", len(kit.notifier.sent))
	}
	if kit.notifier.sent[0].NotifKind != "OT_REJECTED" {
		t.Errorf("notif kind = %s, want OT_REJECTED", kit.notifier.sent[0].NotifKind)
	}
}
