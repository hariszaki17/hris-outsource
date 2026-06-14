// Package leave_test — in-memory per-type QuotaMeter fakes (EPICS §8 2026-06-12).
// Replaces the retired grant-lot fakes: the harness wires these THROUGH the real
// LeaveService so the contract tests exercise the meter path the production server
// runs. Mirrors the service-package memStore/memReader.
package leave_test

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// fakeMeterStore is an in-memory svc.QuotaMeterStore (the window-mutating side).
type fakeMeterStore struct {
	byID map[string]*dom.LeaveQuota
	seq  int
}

func newFakeMeterStore() *fakeMeterStore { return &fakeMeterStore{byID: map[string]*dom.LeaveQuota{}} }

func (s *fakeMeterStore) windowFor(emp, lt, pk string) *dom.LeaveQuota {
	for _, q := range s.byID {
		if q.EmployeeID == emp && q.LeaveTypeID == lt && q.PeriodKey == pk {
			return q
		}
	}
	return nil
}

func (s *fakeMeterStore) ResolveQuotaWindow(_ context.Context, _ pgx.Tx, emp, lt, pk string) (dom.LeaveQuota, error) {
	if q := s.windowFor(emp, lt, pk); q != nil {
		return *q, nil
	}
	return dom.LeaveQuota{}, domain.ErrNotFound
}

func (s *fakeMeterStore) OpenQuotaWindow(_ context.Context, _ pgx.Tx, spec dom.QuotaWindowSpec) (dom.LeaveQuota, error) {
	if q := s.windowFor(spec.EmployeeID, spec.LeaveTypeID, spec.PeriodKey); q != nil {
		q.EntitledDays = spec.EntitledDays
		return *q, nil
	}
	s.seq++
	id := "SWP-LQ-" + itoa(9500+s.seq)
	q := &dom.LeaveQuota{
		ID: id, EmployeeID: spec.EmployeeID, LeaveTypeID: spec.LeaveTypeID,
		PeriodKey: spec.PeriodKey, EntitledDays: spec.EntitledDays, Source: spec.Source,
	}
	s.byID[id] = q
	return *q, nil
}

func (s *fakeMeterStore) ReserveQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	q := s.byID[id]
	q.PendingDays += d
	return *q, nil
}

func (s *fakeMeterStore) CommitQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	q := s.byID[id]
	if q.PendingDays -= d; q.PendingDays < 0 {
		q.PendingDays = 0
	}
	q.UsedDays += d
	return *q, nil
}

func (s *fakeMeterStore) ReleaseQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	q := s.byID[id]
	if q.PendingDays -= d; q.PendingDays < 0 {
		q.PendingDays = 0
	}
	return *q, nil
}

func (s *fakeMeterStore) ReverseCommittedQuotaDays(_ context.Context, _ pgx.Tx, id string, d int) (dom.LeaveQuota, error) {
	q := s.byID[id]
	if q.UsedDays -= d; q.UsedDays < 0 {
		q.UsedDays = 0
	}
	return *q, nil
}

func (s *fakeMeterStore) AdjustQuotaEntitled(_ context.Context, _ pgx.Tx, id string, d int, remark string, adj dom.LeaveQuotaAdjustment) (dom.LeaveQuota, error) {
	q := s.byID[id]
	q.EntitledDays += d
	q.Remark = remark
	a := adj
	q.LastAdjustment = &a
	return *q, nil
}

func (s *fakeMeterStore) CountApprovedRequestsForType(context.Context, string, string, time.Time, time.Time) (int, error) {
	return 0, nil
}

var _ svc.QuotaMeterStore = (*fakeMeterStore)(nil)

// fakeMeterReader is an in-memory svc.QuotaMeterReader (cap mechanics + gate inputs).
type fakeMeterReader struct {
	annual map[string]int              // employee_id → annual entitlement (days)
	caps   map[string]dom.LeaveTypeCap // leave_type_id → cap (default: ANNUAL_POOL)
	gender map[string]*string
}

func newFakeMeterReader() *fakeMeterReader {
	return &fakeMeterReader{annual: map[string]int{}, caps: map[string]dom.LeaveTypeCap{}, gender: map[string]*string{}}
}

func (r *fakeMeterReader) GetLeaveTypeCap(_ context.Context, lt string) (dom.LeaveTypeCap, error) {
	if c, ok := r.caps[lt]; ok {
		return c, nil
	}
	return dom.LeaveTypeCap{ID: lt, CapBasis: dom.CapBasisAnnualPool, CapUnit: "DAYS"}, nil
}

func (r *fakeMeterReader) GetEmployeeGateInfo(_ context.Context, emp string) (dom.EmployeeGateInfo, error) {
	return dom.EmployeeGateInfo{Gender: r.gender[emp], JoinAt: time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
}

func (r *fakeMeterReader) GetAnnualEntitlement(_ context.Context, emp string) (*int, error) {
	if v, ok := r.annual[emp]; ok {
		vv := v
		return &vv, nil
	}
	return nil, nil
}

var _ svc.QuotaMeterReader = (*fakeMeterReader)(nil)
