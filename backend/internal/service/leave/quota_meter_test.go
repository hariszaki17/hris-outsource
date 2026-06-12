package leave

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
)

// --- fakes ---

type fakeReader struct {
	cap    dom.LeaveTypeCap
	emp    dom.EmployeeGateInfo
	annual *int
}

func (f fakeReader) GetLeaveTypeCap(context.Context, string) (dom.LeaveTypeCap, error) {
	return f.cap, nil
}
func (f fakeReader) GetEmployeeGateInfo(context.Context, string) (dom.EmployeeGateInfo, error) {
	return f.emp, nil
}
func (f fakeReader) GetAnnualEntitlement(context.Context, string) (*int, error) { return f.annual, nil }

type fakeStore struct {
	win        dom.LeaveQuota
	resolveErr error
	prior      int
	reserved   int
	opened     bool
}

func (f *fakeStore) ResolveQuotaWindow(context.Context, pgx.Tx, string, string, string) (dom.LeaveQuota, error) {
	if f.resolveErr != nil {
		return dom.LeaveQuota{}, f.resolveErr
	}
	return f.win, nil
}
func (f *fakeStore) OpenQuotaWindow(_ context.Context, _ pgx.Tx, s dom.QuotaWindowSpec) (dom.LeaveQuota, error) {
	f.opened = true
	f.win = dom.LeaveQuota{ID: "SWP-LQ-NEW", EntitledDays: s.EntitledDays}
	return f.win, nil
}
func (f *fakeStore) ReserveQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	f.reserved = d
	f.win.ID = id
	f.win.PendingDays += d
	return f.win, nil
}
func (f *fakeStore) CommitQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	return f.win, nil
}
func (f *fakeStore) ReleaseQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	return f.win, nil
}
func (f *fakeStore) ReverseCommittedQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	return f.win, nil
}
func (f *fakeStore) CountApprovedRequestsForType(context.Context, string, string, time.Time, time.Time) (int, error) {
	return f.prior, nil
}

// --- pure helpers ---

func TestWindowFor(t *testing.T) {
	start := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		basis dom.LeaveTypeCapBasis
		key   string
		exp   bool // expiresAt non-nil
	}{
		{dom.CapBasisAnnualPool, "2026", true},
		{dom.CapBasisPerYearCount, "2026", true},
		{dom.CapBasisPerMonth, "2026-06", true},
		{dom.CapBasisLifetimeOnce, "EMP", false},
		{dom.CapBasisServiceUnpaid, "EMP", false},
	}
	for _, c := range cases {
		key, _, _, _, exp := windowFor(c.basis, start)
		if key != c.key {
			t.Errorf("%s: key=%q want %q", c.basis, key, c.key)
		}
		if (exp != nil) != c.exp {
			t.Errorf("%s: expiresAt non-nil=%v want %v", c.basis, exp != nil, c.exp)
		}
	}
}

func TestChargeFor(t *testing.T) {
	if got := chargeFor(dom.LeaveTypeCap{CapUnit: "COUNT"}, 4); got != 1 {
		t.Errorf("COUNT charge=%d want 1", got)
	}
	if got := chargeFor(dom.LeaveTypeCap{CapUnit: "DAYS"}, 4); got != 4 {
		t.Errorf("DAYS charge=%d want 4", got)
	}
}

func TestEvaluateGates(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	female := "FEMALE"
	join := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // ~2.4y service

	// gender mismatch
	if e := evaluateGates(dom.LeaveTypeCap{Gender: "FEMALE"}, dom.EmployeeGateInfo{Gender: nil}, now, now); e == nil || e.Reason != GateGenderMismatch {
		t.Errorf("gender gate not raised: %v", e)
	}
	// gender ok
	if e := evaluateGates(dom.LeaveTypeCap{Gender: "FEMALE"}, dom.EmployeeGateInfo{Gender: &female}, now.AddDate(0, 0, 40), now); e != nil {
		t.Errorf("gender ok but errored: %v", e)
	}
	// notice insufficient (start in 10 days, need 30)
	if e := evaluateGates(dom.LeaveTypeCap{NoticeDays: 30}, dom.EmployeeGateInfo{}, now.AddDate(0, 0, 10), now); e == nil || e.Reason != GateInsufficientNotice {
		t.Errorf("notice gate not raised: %v", e)
	}
	// service insufficient (2.4y < 5)
	if e := evaluateGates(dom.LeaveTypeCap{MinServiceYears: 5}, dom.EmployeeGateInfo{JoinAt: join}, now, now); e == nil || e.Reason != GateInsufficientSvc {
		t.Errorf("service gate not raised: %v", e)
	}
}

// --- reserve flows ---

func reserveIn(typeID string, days int) ReserveInput {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return ReserveInput{EmployeeID: "E1", LeaveTypeID: typeID, Days: days, StartDate: now.AddDate(0, 1, 0), Now: now}
}

func TestReserve_PerEvent_OverCap(t *testing.T) {
	m := NewQuotaMeter(&fakeStore{}, fakeReader{cap: dom.LeaveTypeCap{CapBasis: dom.CapBasisPerEvent, CapValue: iptr(2), CapUnit: "DAYS"}})
	_, err := m.Reserve(context.Background(), nil, reserveIn("CKM", 3))
	if ge, ok := err.(*GateError); !ok || ge.Reason != GateOverEventCap {
		t.Fatalf("want OVER_EVENT_CAP, got %v", err)
	}
}

func TestReserve_PerEvent_NoQuota(t *testing.T) {
	st := &fakeStore{}
	m := NewQuotaMeter(st, fakeReader{cap: dom.LeaveTypeCap{CapBasis: dom.CapBasisPerEvent, CapValue: iptr(2), CapUnit: "DAYS", Paid: true}})
	res, err := m.Reserve(context.Background(), nil, reserveIn("CKM", 2))
	if err != nil {
		t.Fatal(err)
	}
	if res.QuotaID != nil {
		t.Errorf("per-event should not open a window")
	}
	if !res.Paid || res.Charge != 2 {
		t.Errorf("res=%+v", res)
	}
}

func TestReserve_Annual_OverRemaining(t *testing.T) {
	st := &fakeStore{win: dom.LeaveQuota{ID: "SWP-LQ-1", EntitledDays: 12, UsedDays: 11, PendingDays: 0}}
	m := NewQuotaMeter(st, fakeReader{cap: dom.LeaveTypeCap{CapBasis: dom.CapBasisAnnualPool, CapUnit: "DAYS"}, annual: iptr(12)})
	_, err := m.Reserve(context.Background(), nil, reserveIn("CT", 3))
	if ge, ok := err.(*GateError); !ok || ge.Reason != GateOverCap {
		t.Fatalf("want OVER_CAP, got %v", err)
	}
}

func TestReserve_Annual_Happy(t *testing.T) {
	st := &fakeStore{win: dom.LeaveQuota{ID: "SWP-LQ-1", EntitledDays: 12, UsedDays: 4, PendingDays: 0}}
	m := NewQuotaMeter(st, fakeReader{cap: dom.LeaveTypeCap{CapBasis: dom.CapBasisAnnualPool, CapUnit: "DAYS"}, annual: iptr(12)})
	res, err := m.Reserve(context.Background(), nil, reserveIn("CT", 3))
	if err != nil {
		t.Fatal(err)
	}
	if res.QuotaID == nil || *res.QuotaID != "SWP-LQ-1" {
		t.Errorf("quotaID=%v", res.QuotaID)
	}
	if st.reserved != 3 {
		t.Errorf("reserved=%d want 3", st.reserved)
	}
}

func TestReserve_Annual_AutoOpensWindow(t *testing.T) {
	st := &fakeStore{resolveErr: domain.ErrNotFound}
	m := NewQuotaMeter(st, fakeReader{cap: dom.LeaveTypeCap{CapBasis: dom.CapBasisAnnualPool, CapUnit: "DAYS"}, annual: iptr(12)})
	res, err := m.Reserve(context.Background(), nil, reserveIn("CT", 2))
	if err != nil {
		t.Fatal(err)
	}
	if !st.opened {
		t.Errorf("window should auto-open when not found")
	}
	if res.QuotaID == nil {
		t.Errorf("quotaID nil after open")
	}
}

func TestReserve_Lifetime_AlreadyUsed(t *testing.T) {
	st := &fakeStore{prior: 1}
	m := NewQuotaMeter(st, fakeReader{cap: dom.LeaveTypeCap{CapBasis: dom.CapBasisLifetimeOnce, CapValue: iptr(3), CapUnit: "DAYS"}})
	_, err := m.Reserve(context.Background(), nil, reserveIn("CM", 3))
	if ge, ok := err.(*GateError); !ok || ge.Reason != GateAlreadyUsed {
		t.Fatalf("want ALREADY_USED_LIFETIME, got %v", err)
	}
}

func iptr(i int) *int { return &i }
