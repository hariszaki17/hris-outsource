package leave

// QuotaMeter is the per-type leave metering engine (EPICS §8 "E6 — Leave"
// 2026-06-12, F6.1 LQ-13). It replaces the grant-lot GrantService: each request
// is metered against its leave type's own cap_basis window, never another type's,
// and the annual pool is never depleted by statutory/sick/religious leave.
//
// Lifecycle (called by LeaveService in Phase 4, inside the request's tx):
//   Reserve  — submit: gates + hold pending on the window (or per-event cap check)
//   Commit   — final approval: pending -> used
//   Release  — reject/withdraw: drop the held pending
//   Reverse  — cancel/shorten an APPROVED leave: return committed used
//
// PER_EVENT / UNCAPPED types hold no window row: Reserve returns a nil QuotaID and
// the cap/document checks happen at request time. Quota-bearing types return the
// window id, which LeaveService persists on leave_requests.quota_id.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
)

// QuotaMeterStore is the window-mutating side (implemented by repository QuotaRepo).
type QuotaMeterStore interface {
	ResolveQuotaWindow(ctx context.Context, tx pgx.Tx, employeeID, leaveTypeID, periodKey string) (dom.LeaveQuota, error)
	OpenQuotaWindow(ctx context.Context, tx pgx.Tx, s dom.QuotaWindowSpec) (dom.LeaveQuota, error)
	ReserveQuotaDays(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error)
	CommitQuotaDays(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error)
	ReleaseQuotaDays(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error)
	ReverseCommittedQuotaDays(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error)
	CountApprovedRequestsForType(ctx context.Context, employeeID, leaveTypeID string, from, to time.Time) (int, error)
}

// QuotaMeterReader is the read side (cap mechanics + gate inputs + annual entitlement).
type QuotaMeterReader interface {
	GetLeaveTypeCap(ctx context.Context, leaveTypeID string) (dom.LeaveTypeCap, error)
	GetEmployeeGateInfo(ctx context.Context, employeeID string) (dom.EmployeeGateInfo, error)
	GetAnnualEntitlement(ctx context.Context, employeeID string) (*int, error)
}

// QuotaMeter meters leave against per-type cap_basis windows.
type QuotaMeter struct {
	store  QuotaMeterStore
	reader QuotaMeterReader
}

// NewQuotaMeter builds a QuotaMeter.
func NewQuotaMeter(store QuotaMeterStore, reader QuotaMeterReader) *QuotaMeter {
	return &QuotaMeter{store: store, reader: reader}
}

// GateError is a request-time eligibility/cap rejection. LeaveService maps it to
// 422 RULE_VIOLATION (CONVENTIONS §error).
type GateError struct {
	Reason  string // machine code, e.g. GENDER_MISMATCH
	Message string // human (Bahasa) message
}

func (e *GateError) Error() string { return fmt.Sprintf("leave gate %s: %s", e.Reason, e.Message) }

// Gate reason codes.
const (
	GateGenderMismatch     = "GENDER_MISMATCH"
	GateInsufficientNotice = "INSUFFICIENT_NOTICE"
	GateInsufficientSvc    = "INSUFFICIENT_SERVICE"
	GateAlreadyUsed        = "ALREADY_USED_LIFETIME"
	GateOverCap            = "OVER_CAP"
	GateOverEventCap       = "OVER_EVENT_CAP"
)

// ReserveInput is one submit-time metering request.
type ReserveInput struct {
	EmployeeID  string
	LeaveTypeID string
	Days        int       // computed duration (working days)
	StartDate   time.Time // request start (window selection + notice gate)
	Now         time.Time // clock (Asia/Jakarta layer); notice/tenure reference
}

// ReserveResult is the outcome of a reserve. QuotaID is non-nil only for
// quota-bearing types; Charge is what was held (days, or 1 for COUNT types).
type ReserveResult struct {
	QuotaID *string
	Charge  int
	Paid    bool
}

// Reserve applies the eligibility gates and holds the reservation for a submit.
func (m *QuotaMeter) Reserve(ctx context.Context, tx pgx.Tx, in ReserveInput) (ReserveResult, error) {
	cap, err := m.reader.GetLeaveTypeCap(ctx, in.LeaveTypeID)
	if err != nil {
		return ReserveResult{}, err
	}
	emp, err := m.reader.GetEmployeeGateInfo(ctx, in.EmployeeID)
	if err != nil {
		return ReserveResult{}, err
	}
	if gerr := evaluateGates(cap, emp, in.StartDate, in.Now); gerr != nil {
		return ReserveResult{}, gerr
	}

	charge := chargeFor(cap, in.Days)

	// Lifetime/service-unpaid: enforce once per employment.
	if cap.CapBasis == dom.CapBasisLifetimeOnce || cap.CapBasis == dom.CapBasisServiceUnpaid {
		prior, err := m.store.CountApprovedRequestsForType(ctx, in.EmployeeID, in.LeaveTypeID, lifetimeFrom(emp), farFuture())
		if err != nil {
			return ReserveResult{}, err
		}
		if prior > 0 {
			return ReserveResult{}, &GateError{Reason: GateAlreadyUsed, Message: "Jenis cuti ini hanya dapat digunakan sekali selama masa kerja."}
		}
	}

	// Non-quota-bearing: no standing window.
	if !cap.CapBasis.QuotaBearing() {
		if cap.CapBasis == dom.CapBasisPerEvent && cap.CapValue != nil && in.Days > *cap.CapValue {
			return ReserveResult{}, &GateError{Reason: GateOverEventCap, Message: fmt.Sprintf("Melebihi batas %d hari per kejadian.", *cap.CapValue)}
		}
		return ReserveResult{QuotaID: nil, Charge: charge, Paid: cap.Paid}, nil
	}

	// Quota-bearing: resolve (row-lock) or auto-open the window.
	key, period, ps, pe, exp := windowFor(cap.CapBasis, in.StartDate)
	win, err := m.store.ResolveQuotaWindow(ctx, tx, in.EmployeeID, in.LeaveTypeID, key)
	if errors.Is(err, domain.ErrNotFound) {
		entitled, eerr := m.entitlementFor(ctx, cap, in.EmployeeID)
		if eerr != nil {
			return ReserveResult{}, eerr
		}
		win, err = m.store.OpenQuotaWindow(ctx, tx, dom.QuotaWindowSpec{
			EmployeeID:   in.EmployeeID,
			LeaveTypeID:  in.LeaveTypeID,
			PeriodKey:    key,
			Period:       period,
			PeriodStart:  ps,
			PeriodEnd:    pe,
			EntitledDays: entitled,
			Source:       dom.QuotaSourceAuto,
			Remark:       "auto-open " + string(cap.CapBasis),
			ExpiresAt:    exp,
		})
	}
	if err != nil {
		return ReserveResult{}, err
	}

	// Day cap applies to annual and to any quota-bearing type with an explicit
	// cap_value; nil-cap lifetime types (e.g. hajj) are bounded by the once gate
	// and the document, not a day count.
	if dayCapped(cap) && win.RemainingPerType() < charge {
		return ReserveResult{}, &GateError{Reason: GateOverCap, Message: "Sisa kuota jenis cuti ini tidak mencukupi."}
	}

	win, err = m.store.ReserveQuotaDays(ctx, tx, win.ID, charge)
	if err != nil {
		return ReserveResult{}, err
	}
	id := win.ID
	return ReserveResult{QuotaID: &id, Charge: charge, Paid: cap.Paid}, nil
}

// Commit moves a held reservation into used (final approval). No-op when quotaID
// is nil (PER_EVENT / UNCAPPED).
func (m *QuotaMeter) Commit(ctx context.Context, tx pgx.Tx, leaveTypeID string, quotaID *string, days int) error {
	return m.move(ctx, tx, leaveTypeID, quotaID, days, m.store.CommitQuotaDays)
}

// Release drops a held reservation (reject / withdraw).
func (m *QuotaMeter) Release(ctx context.Context, tx pgx.Tx, leaveTypeID string, quotaID *string, days int) error {
	return m.move(ctx, tx, leaveTypeID, quotaID, days, m.store.ReleaseQuotaDays)
}

// Reverse returns committed used to the balance (cancel / shorten an APPROVED leave).
func (m *QuotaMeter) Reverse(ctx context.Context, tx pgx.Tx, leaveTypeID string, quotaID *string, days int) error {
	return m.move(ctx, tx, leaveTypeID, quotaID, days, m.store.ReverseCommittedQuotaDays)
}

func (m *QuotaMeter) move(ctx context.Context, tx pgx.Tx, leaveTypeID string, quotaID *string, days int, fn func(context.Context, pgx.Tx, string, int) (dom.LeaveQuota, error)) error {
	if quotaID == nil {
		return nil
	}
	cap, err := m.reader.GetLeaveTypeCap(ctx, leaveTypeID)
	if err != nil {
		return err
	}
	_, err = fn(ctx, tx, *quotaID, chargeFor(cap, days))
	return err
}

func (m *QuotaMeter) entitlementFor(ctx context.Context, cap dom.LeaveTypeCap, employeeID string) (int, error) {
	if cap.CapBasis == dom.CapBasisAnnualPool {
		ent, err := m.reader.GetAnnualEntitlement(ctx, employeeID)
		if err != nil {
			return 0, err
		}
		if ent != nil {
			return *ent, nil
		}
		if cap.CapValue != nil {
			return *cap.CapValue, nil
		}
		return 0, nil
	}
	if cap.CapValue != nil {
		return *cap.CapValue, nil
	}
	// Quota-bearing with no day cap (e.g. LIFETIME_ONCE hajj): track the row but
	// don't day-limit; a high sentinel keeps remaining non-binding.
	return noDayCapEntitlement, nil
}

const noDayCapEntitlement = 36500 // ~100y; effectively uncapped on days

// --- pure helpers (unit-tested without IO) ---

// chargeFor is what a request charges its window: 1 occurrence for COUNT types,
// otherwise the day count.
func chargeFor(cap dom.LeaveTypeCap, days int) int {
	if cap.CapUnit == "COUNT" {
		return 1
	}
	return days
}

// dayCapped reports whether the window enforces a day/occurrence remaining check.
func dayCapped(cap dom.LeaveTypeCap) bool {
	if cap.CapBasis == dom.CapBasisAnnualPool {
		return true
	}
	return cap.CapValue != nil
}

// windowFor maps a cap_basis + request start to the quota window key and the
// legacy period bounds (still NOT NULL until Phase 8) and the per-window expiry.
func windowFor(basis dom.LeaveTypeCapBasis, start time.Time) (key string, period int, periodStart, periodEnd time.Time, expiresAt *time.Time) {
	y := start.Year()
	switch basis {
	case dom.CapBasisPerMonth:
		ps := time.Date(y, start.Month(), 1, 0, 0, 0, 0, time.UTC)
		pe := ps.AddDate(0, 1, -1)
		return fmt.Sprintf("%04d-%02d", y, int(start.Month())), y, ps, pe, &pe
	case dom.CapBasisAnnualPool, dom.CapBasisPerYearCount:
		ps := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		pe := time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC)
		return fmt.Sprintf("%04d", y), y, ps, pe, &pe
	default: // LIFETIME_ONCE, SERVICE_UNPAID — one window per employment, no expiry
		ps := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		pe := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
		return "EMP", 0, ps, pe, nil
	}
}

// evaluateGates applies gender / advance-notice / minimum-service gates (INV-7).
func evaluateGates(cap dom.LeaveTypeCap, emp dom.EmployeeGateInfo, start, now time.Time) *GateError {
	if cap.Gender != "" && cap.Gender != "ANY" {
		if emp.Gender == nil || *emp.Gender != cap.Gender {
			return &GateError{Reason: GateGenderMismatch, Message: "Jenis cuti ini tidak berlaku untuk gender Anda."}
		}
	}
	if cap.NoticeDays > 0 {
		if daysBetween(now, start) < cap.NoticeDays {
			return &GateError{Reason: GateInsufficientNotice, Message: fmt.Sprintf("Pengajuan minimal %d hari sebelum tanggal mulai.", cap.NoticeDays)}
		}
	}
	if cap.MinServiceYears > 0 {
		if fullYearsBetween(emp.JoinAt, now) < cap.MinServiceYears {
			return &GateError{Reason: GateInsufficientSvc, Message: fmt.Sprintf("Minimal %d tahun masa kerja.", cap.MinServiceYears)}
		}
	}
	return nil
}

func daysBetween(from, to time.Time) int {
	return int(truncDay(to).Sub(truncDay(from)).Hours() / 24)
}

func fullYearsBetween(from, to time.Time) int {
	y := to.Year() - from.Year()
	if to.Month() < from.Month() || (to.Month() == from.Month() && to.Day() < from.Day()) {
		y--
	}
	if y < 0 {
		y = 0
	}
	return y
}

func truncDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func lifetimeFrom(emp dom.EmployeeGateInfo) time.Time {
	if !emp.JoinAt.IsZero() {
		return emp.JoinAt
	}
	return time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
}

func farFuture() time.Time { return time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC) }
