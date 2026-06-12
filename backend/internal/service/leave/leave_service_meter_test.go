package leave

// Integration tests for the per-type QuotaMeter wired THROUGH LeaveService (the
// production path, EPICS §8 2026-06-12). Reuses the grant-era fakes (fakeLeaveRepo,
// fakeSchedule, fakeRunner, newReq, hrCtx, fixedNow, iptr) and adds an in-memory
// QuotaMeterStore/Reader. The legacy grant tests in leave_service_test.go cover the
// meter-nil fallback; these cover s.meter != nil.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
)

// --- in-memory meter store + reader ---

type memStore struct {
	byID  map[string]*dom.LeaveQuota
	seq   int
	prior int
}

func newMemStore() *memStore { return &memStore{byID: map[string]*dom.LeaveQuota{}} }

func (s *memStore) seed(emp, lt, pk string, entitled, used, pending int) string {
	s.seq++
	id := fmt.Sprintf("SWP-LQ-S%d", s.seq)
	s.byID[id] = &dom.LeaveQuota{ID: id, EmployeeID: emp, LeaveTypeID: lt, PeriodKey: pk, EntitledDays: entitled, UsedDays: used, PendingDays: pending}
	return id
}

func (s *memStore) windowFor(emp, lt, pk string) *dom.LeaveQuota {
	for _, q := range s.byID {
		if q.EmployeeID == emp && q.LeaveTypeID == lt && q.PeriodKey == pk {
			return q
		}
	}
	return nil
}

func (s *memStore) ResolveQuotaWindow(_ context.Context, _ pgx.Tx, emp, lt, pk string) (dom.LeaveQuota, error) {
	if q := s.windowFor(emp, lt, pk); q != nil {
		return *q, nil
	}
	return dom.LeaveQuota{}, domain.ErrNotFound
}
func (s *memStore) OpenQuotaWindow(_ context.Context, _ pgx.Tx, spec dom.QuotaWindowSpec) (dom.LeaveQuota, error) {
	s.seq++
	id := fmt.Sprintf("SWP-LQ-O%d", s.seq)
	q := &dom.LeaveQuota{ID: id, EmployeeID: spec.EmployeeID, LeaveTypeID: spec.LeaveTypeID, PeriodKey: spec.PeriodKey, EntitledDays: spec.EntitledDays}
	s.byID[id] = q
	return *q, nil
}
func (s *memStore) ReserveQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	q := s.byID[id]
	q.PendingDays += d
	return *q, nil
}
func (s *memStore) CommitQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	q := s.byID[id]
	q.PendingDays = maxInt(q.PendingDays-d, 0)
	q.UsedDays += d
	return *q, nil
}
func (s *memStore) ReleaseQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	q := s.byID[id]
	q.PendingDays = maxInt(q.PendingDays-d, 0)
	return *q, nil
}
func (s *memStore) ReverseCommittedQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	q := s.byID[id]
	q.UsedDays = maxInt(q.UsedDays-d, 0)
	return *q, nil
}
func (s *memStore) AdjustQuotaEntitled(_ context.Context, _ pgx.Tx, id string, d int, _ string, _ dom.LeaveQuotaAdjustment) (dom.LeaveQuota, error) {
	q := s.byID[id]
	q.EntitledDays += d
	return *q, nil
}
func (s *memStore) CountApprovedRequestsForType(context.Context, string, string, time.Time, time.Time) (int, error) {
	return s.prior, nil
}

type memReader struct {
	cap    dom.LeaveTypeCap
	gender *string
	annual *int
}

func (r memReader) GetLeaveTypeCap(context.Context, string) (dom.LeaveTypeCap, error) {
	return r.cap, nil
}
func (r memReader) GetEmployeeGateInfo(context.Context, string) (dom.EmployeeGateInfo, error) {
	return dom.EmployeeGateInfo{Gender: r.gender, JoinAt: time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
}
func (r memReader) GetAnnualEntitlement(context.Context, string) (*int, error) { return r.annual, nil }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func capAnnual() dom.LeaveTypeCap {
	return dom.LeaveTypeCap{ID: "SWP-LT-001", CapBasis: dom.CapBasisAnnualPool, CapUnit: "DAYS"}
}

func newMeterSvc(lr *fakeLeaveRepo, sp *fakeSchedule, store QuotaMeterStore, reader QuotaMeterReader) *LeaveService {
	gs := NewGrantService(newFakeGrantRepo(), fakeRunner{})
	gs.SetClock(func() time.Time { return fixedNow })
	s := NewLeaveService(lr, gs, sp, fakeRunner{})
	s.SetClock(func() time.Time { return fixedNow })
	s.SetMeter(NewQuotaMeter(store, reader))
	return s
}

// --- tests ---

func TestMeter_Submit_OpensAndReserves(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusDraft, "SWP-CMP-0021", "SWP-EMP-3001", 3)}
	store := newMemStore()
	s := newMeterSvc(lr, &fakeSchedule{}, store, memReader{cap: capAnnual(), annual: iptr(12)})

	if _, err := s.Submit(hrCtx(), "SWP-LR-8001"); err != nil {
		t.Fatal(err)
	}
	w := store.windowFor("SWP-EMP-3001", "SWP-LT-001", "2026")
	if w == nil || w.EntitledDays != 12 || w.PendingDays != 3 {
		t.Fatalf("window=%+v want entitled 12 pending 3", w)
	}
}

func TestMeter_ApproveFinal_Commits(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusPendingHR, "SWP-CMP-0021", "SWP-EMP-3001", 3)}
	store := newMemStore()
	store.seed("SWP-EMP-3001", "SWP-LT-001", "2026", 12, 0, 3)
	s := newMeterSvc(lr, &fakeSchedule{}, store, memReader{cap: capAnnual(), annual: iptr(12)})

	if _, err := s.ApproveFinal(hrCtx(), "SWP-LR-8001", ""); err != nil {
		t.Fatal(err)
	}
	w := store.windowFor("SWP-EMP-3001", "SWP-LT-001", "2026")
	if w.UsedDays != 3 || w.PendingDays != 0 {
		t.Fatalf("window=%+v want used 3 pending 0", w)
	}
}

func TestMeter_Submit_OverCap_Blocked(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusDraft, "SWP-CMP-0021", "SWP-EMP-3001", 5)}
	store := newMemStore()
	store.seed("SWP-EMP-3001", "SWP-LT-001", "2026", 12, 12, 0) // remaining 0
	s := newMeterSvc(lr, &fakeSchedule{}, store, memReader{cap: capAnnual(), annual: iptr(12)})

	if _, err := s.Submit(hrCtx(), "SWP-LR-8001"); err == nil {
		t.Fatal("expected over-cap block")
	}
}

func TestMeter_PerEvent_NoWindow(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusDraft, "SWP-CMP-0021", "SWP-EMP-3001", 2)}
	store := newMemStore()
	cap := dom.LeaveTypeCap{ID: "SWP-LT-001", CapBasis: dom.CapBasisPerEvent, CapValue: iptr(2), CapUnit: "DAYS"}
	s := newMeterSvc(lr, &fakeSchedule{}, store, memReader{cap: cap})

	if _, err := s.Submit(hrCtx(), "SWP-LR-8001"); err != nil {
		t.Fatal(err)
	}
	if len(store.byID) != 0 {
		t.Fatalf("per-event should open no window, got %d", len(store.byID))
	}
}

func TestMeter_GenderGate_Blocked(t *testing.T) {
	lr := &fakeLeaveRepo{req: newReq(dom.LeaveStatusDraft, "SWP-CMP-0021", "SWP-EMP-3001", 1)}
	store := newMemStore()
	cap := dom.LeaveTypeCap{ID: "SWP-LT-001", CapBasis: dom.CapBasisPerMonth, CapValue: iptr(2), CapUnit: "DAYS", Gender: "FEMALE"}
	s := newMeterSvc(lr, &fakeSchedule{}, store, memReader{cap: cap, gender: ptr("MALE")})

	if _, err := s.Submit(hrCtx(), "SWP-LR-8001"); err == nil {
		t.Fatal("expected gender-gate block")
	}
}
