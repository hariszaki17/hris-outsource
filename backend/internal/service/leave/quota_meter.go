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
	AdjustQuotaEntitled(ctx context.Context, tx pgx.Tx, id string, delta int, remark string, adj dom.LeaveQuotaAdjustment) (dom.LeaveQuota, error)
	CountApprovedRequestsForType(ctx context.Context, employeeID, leaveTypeID string, from, to time.Time) (int, error)
}

// QuotaMeterReader is the read side (cap mechanics + the HR-assigned entitlement +
// annual entitlement transition fallback).
type QuotaMeterReader interface {
	GetLeaveTypeCap(ctx context.Context, leaveTypeID string) (dom.LeaveTypeCap, error)
	GetEmployeeGateInfo(ctx context.Context, employeeID string) (dom.EmployeeGateInfo, error)
	GetAnnualEntitlement(ctx context.Context, employeeID string) (*int, error)
	// GetEmployeeEntitlement returns the HR-assigned per-type entitlement (ELE-2), or
	// nil (no error) when the type is not assigned — the window auto-opens from this
	// row's entitled_days; nil falls back to cap_value (transition safety).
	GetEmployeeEntitlement(ctx context.Context, employeeID, leaveTypeID string) (*dom.LeaveEntitlement, error)
}

// EntitlementSetter writes the HR base entitlement. The meter uses it to open
// enforcement on an otherwise "sesuai ketentuan" type (PER_EVENT / UNCAPPED) when HR
// adjusts it straight from Kuota Cuti — the "Both places" rule (2026-06-15): a base can
// be defined on the Hak Cuti tab OR by adjusting the quota here.
type EntitlementSetter interface {
	UpsertEntitlement(ctx context.Context, tx pgx.Tx, p EntitlementWrite) (dom.LeaveEntitlement, error)
}

// QuotaMeter meters leave against per-type cap_basis windows.
type QuotaMeter struct {
	store  QuotaMeterStore
	reader QuotaMeterReader
	ent    EntitlementSetter
}

// NewQuotaMeter builds a QuotaMeter.
func NewQuotaMeter(store QuotaMeterStore, reader QuotaMeterReader) *QuotaMeter {
	return &QuotaMeter{store: store, reader: reader}
}

// SetEntitlementSetter wires the entitlement writer (production). Without it the meter
// refuses to open enforcement on a non-windowed type from Kuota Cuti (HR must set the
// base on Hak Cuti instead).
func (m *QuotaMeter) SetEntitlementSetter(e EntitlementSetter) { m.ent = e }

// GateError is a request-time eligibility/cap rejection. LeaveService maps it to
// 422 RULE_VIOLATION (CONVENTIONS §error).
type GateError struct {
	Reason  string // machine code, e.g. GENDER_MISMATCH
	Message string // human (Bahasa) message
}

func (e *GateError) Error() string { return fmt.Sprintf("leave gate %s: %s", e.Reason, e.Message) }

// Gate reason codes. The eligibility gates (gender / notice / min-service /
// lifetime-once) were DROPPED with the HR-assigned entitlement model (ELE-8); only
// the quota/per-event cap rejections remain.
const (
	GateOverCap      = "OVER_CAP"
	GateOverEventCap = "OVER_EVENT_CAP"
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

// Reserve holds the reservation for a submit. Eligibility gates (gender / notice /
// min-service / lifetime-once) are DROPPED (PRD leave-entitlement-assignment, ELE-8):
// HR's per-employee assignment is the eligibility control. "Once per employment" now
// emerges naturally from a LIFETIME_ONCE window (opens once, never resets → exhausts).
// Retained request-time checks: per-event cap + remaining quota (QUOTA_EXCEEDED);
// date-range / overlap / backdate are enforced upstream in LeaveService.
func (m *QuotaMeter) Reserve(ctx context.Context, tx pgx.Tx, in ReserveInput) (ReserveResult, error) {
	cap, err := m.reader.GetLeaveTypeCap(ctx, in.LeaveTypeID)
	if err != nil {
		return ReserveResult{}, err
	}

	charge := chargeFor(cap, in.Days)

	isWin, err := m.windowed(ctx, cap, in.EmployeeID)
	if err != nil {
		return ReserveResult{}, err
	}

	// Not windowed: no standing window (truly "sesuai ketentuan" / per-event).
	if !isWin {
		if cap.CapBasis == dom.CapBasisPerEvent && cap.CapValue != nil && in.Days > *cap.CapValue {
			return ReserveResult{}, &GateError{Reason: GateOverEventCap, Message: fmt.Sprintf("Melebihi batas %d hari per kejadian.", *cap.CapValue)}
		}
		return ReserveResult{QuotaID: nil, Charge: charge, Paid: cap.Paid}, nil
	}

	// Windowed: resolve (row-lock) or auto-open the window.
	win, err := m.resolveOrOpen(ctx, tx, cap, in.EmployeeID, in.StartDate)
	if err != nil {
		return ReserveResult{}, err
	}

	// Enforce the day cap when the window carries a finite entitled (annual pool, a
	// type cap_value, or an HR-defined quota). The non-binding sentinel (nil-cap
	// lifetime types, e.g. hajj) is bounded by the document, not a day count.
	if win.EntitledDays < noDayCapEntitlement && win.RemainingPerType() < charge {
		return ReserveResult{}, &GateError{Reason: GateOverCap, Message: "Sisa kuota jenis cuti ini tidak mencukupi."}
	}

	win, err = m.store.ReserveQuotaDays(ctx, tx, win.ID, charge)
	if err != nil {
		return ReserveResult{}, err
	}
	id := win.ID
	return ReserveResult{QuotaID: &id, Charge: charge, Paid: cap.Paid}, nil
}

// CommitInput finalizes a request's window charge (approve). The window is
// re-resolved from (employee, type, start) — no quota_id needs to be persisted.
type CommitInput struct {
	EmployeeID  string
	LeaveTypeID string
	StartDate   time.Time
	Days        int
	Override    bool // HR force-approve past remaining (LA-8)
}

// WindowOp targets a request's window for release/reverse (no override).
type WindowOp struct {
	EmployeeID  string
	LeaveTypeID string
	StartDate   time.Time
	Days        int
}

// Commit moves a held reservation into used (final approval). Re-resolves (or
// opens) the window; for the not-pre-reserved portion it applies the remaining
// recheck unless Override. No-op for PER_EVENT / UNCAPPED.
func (m *QuotaMeter) Commit(ctx context.Context, tx pgx.Tx, in CommitInput) error {
	cap, err := m.reader.GetLeaveTypeCap(ctx, in.LeaveTypeID)
	if err != nil {
		return err
	}
	isWin, err := m.windowed(ctx, cap, in.EmployeeID)
	if err != nil {
		return err
	}
	if !isWin {
		return nil
	}
	win, err := m.resolveOrOpen(ctx, tx, cap, in.EmployeeID, in.StartDate)
	if err != nil {
		return err
	}
	charge := chargeFor(cap, in.Days)
	if win.PendingDays < charge && win.EntitledDays < noDayCapEntitlement {
		shortfall := charge - win.PendingDays
		if !in.Override && win.RemainingPerType() < shortfall {
			return &GateError{Reason: GateOverCap, Message: "Sisa kuota jenis cuti ini tidak mencukupi."}
		}
	}
	_, err = m.store.CommitQuotaDays(ctx, tx, win.ID, charge)
	return err
}

// Release drops a held reservation (reject / withdraw). No-op when no window.
func (m *QuotaMeter) Release(ctx context.Context, tx pgx.Tx, in WindowOp) error {
	return m.adjust(ctx, tx, in, m.store.ReleaseQuotaDays)
}

// Reverse returns committed used to the balance (cancel / shorten an APPROVED leave).
func (m *QuotaMeter) Reverse(ctx context.Context, tx pgx.Tx, in WindowOp) error {
	return m.adjust(ctx, tx, in, m.store.ReverseCommittedQuotaDays)
}

func (m *QuotaMeter) adjust(ctx context.Context, tx pgx.Tx, in WindowOp, fn func(context.Context, pgx.Tx, string, int) (dom.LeaveQuota, error)) error {
	cap, err := m.reader.GetLeaveTypeCap(ctx, in.LeaveTypeID)
	if err != nil {
		return err
	}
	isWin, err := m.windowed(ctx, cap, in.EmployeeID)
	if err != nil {
		return err
	}
	if !isWin {
		return nil
	}
	key, _, _, _, _ := windowFor(cap.CapBasis, in.StartDate)
	win, err := m.store.ResolveQuotaWindow(ctx, tx, in.EmployeeID, in.LeaveTypeID, key)
	if errors.Is(err, domain.ErrNotFound) {
		return nil // nothing held; nothing to do
	}
	if err != nil {
		return err
	}
	_, err = fn(ctx, tx, win.ID, chargeFor(cap, in.Days))
	return err
}

// AdjustEntitledInput is an HR per-type quota adjustment (LQ-6).
type AdjustEntitledInput struct {
	EmployeeID  string
	LeaveTypeID string
	StartDate   time.Time // selects the window (cap_basis)
	Delta       int
	Actor       string
	Reason      string
	Now         time.Time
}

// AdjustEntitled applies an audited signed delta to a per-type window's entitled
// days (HR "Sesuaikan Kuota"). Opens the window if absent; refuses dropping entitled
// below used+pending (INV-6 / no-negative).
func (m *QuotaMeter) AdjustEntitled(ctx context.Context, tx pgx.Tx, in AdjustEntitledInput) (dom.LeaveQuota, error) {
	cap, err := m.reader.GetLeaveTypeCap(ctx, in.LeaveTypeID)
	if err != nil {
		return dom.LeaveQuota{}, err
	}
	isWin, err := m.windowed(ctx, cap, in.EmployeeID)
	if err != nil {
		return dom.LeaveQuota{}, err
	}
	if !isWin {
		// Non-windowed "sesuai ketentuan" type (PER_EVENT / UNCAPPED) with no HR-defined
		// quota. Adjusting it here DEFINES the base: set entitled_days (= the displayed
		// cap_value fallback + delta) on the entitlement so the type becomes enforced +
		// adjustable, then open the lifetime window at that base. "Both places" (2026-06-15).
		return m.openFromBase(ctx, tx, cap, in)
	}
	win, err := m.resolveOrOpen(ctx, tx, cap, in.EmployeeID, in.StartDate)
	if err != nil {
		return dom.LeaveQuota{}, err
	}
	if win.EntitledDays+in.Delta < win.UsedDays+win.PendingDays {
		return dom.LeaveQuota{}, &GateError{Reason: GateOverCap, Message: "Kuota tidak boleh di bawah hari terpakai + tertahan."}
	}
	adj := dom.LeaveQuotaAdjustment{Delta: in.Delta, Reason: in.Reason, AdjustedBy: in.Actor, AdjustedAt: in.Now}
	return m.store.AdjustQuotaEntitled(ctx, tx, win.ID, in.Delta, in.Reason, adj)
}

// openFromBase defines + opens a quota for an otherwise non-windowed type when HR adjusts
// it from Kuota Cuti. The new base is the displayed entitled (cap_value, else 0) + delta;
// it is written to the entitlement (flipping the type to windowed/enforced) and the
// lifetime "EMP" window is opened at that base so the balance reflects it immediately.
func (m *QuotaMeter) openFromBase(ctx context.Context, tx pgx.Tx, cap dom.LeaveTypeCap, in AdjustEntitledInput) (dom.LeaveQuota, error) {
	if m.ent == nil {
		return dom.LeaveQuota{}, &GateError{Reason: GateOverCap, Message: "Tetapkan kuota dasar di Hak Cuti karyawan sebelum menyesuaikan."}
	}
	base := 0
	if cap.CapValue != nil {
		base = *cap.CapValue
	}
	newBase := base + in.Delta
	if newBase < 0 {
		newBase = 0
	}
	actor := &in.Actor
	if in.Actor == "" {
		actor = nil
	}
	if _, werr := m.ent.UpsertEntitlement(ctx, tx, EntitlementWrite{
		EmployeeID: in.EmployeeID, LeaveTypeID: in.LeaveTypeID,
		EntitledDays: &newBase, Note: in.Reason, AssignedBy: actor,
	}); werr != nil {
		return dom.LeaveQuota{}, werr
	}
	key, _, _, _, exp := windowFor(cap.CapBasis, in.StartDate)
	win, rerr := m.store.ResolveQuotaWindow(ctx, tx, in.EmployeeID, in.LeaveTypeID, key)
	if rerr == nil {
		// A window already exists (rare for these types): re-base it via a delta to newBase.
		if d := newBase - win.EntitledDays; d != 0 {
			adj := dom.LeaveQuotaAdjustment{Delta: d, Reason: in.Reason, AdjustedBy: in.Actor, AdjustedAt: in.Now}
			return m.store.AdjustQuotaEntitled(ctx, tx, win.ID, d, in.Reason, adj)
		}
		return win, nil
	}
	if !errors.Is(rerr, domain.ErrNotFound) {
		return dom.LeaveQuota{}, rerr
	}
	return m.store.OpenQuotaWindow(ctx, tx, dom.QuotaWindowSpec{
		EmployeeID: in.EmployeeID, LeaveTypeID: in.LeaveTypeID, PeriodKey: key,
		EntitledDays: newBase, Source: dom.QuotaSourceAdjustment,
		Remark: "HR set base: " + in.Reason, ExpiresAt: exp,
	})
}

// windowed reports whether the type meters against a standing window FOR THIS EMPLOYEE.
// A type is windowed when its cap_basis is inherently quota-bearing, OR HR assigned the
// employee a numeric entitled_days for it (set on the Hak Cuti tab) — an HR-defined quota
// turns even an "sesuai ketentuan" (UNCAPPED / PER_EVENT) type into an enforced, adjustable
// quota (2026-06-15). UNCAPPED / PER_EVENT use the existing "EMP" lifetime window (no reset).
func (m *QuotaMeter) windowed(ctx context.Context, cap dom.LeaveTypeCap, employeeID string) (bool, error) {
	if cap.CapBasis.QuotaBearing() {
		return true, nil
	}
	ent, err := m.reader.GetEmployeeEntitlement(ctx, employeeID, cap.ID)
	if err != nil {
		return false, err
	}
	return ent != nil && ent.EntitledDays != nil, nil
}

// resolveOrOpen row-locks the window, auto-opening it at its entitlement if absent.
func (m *QuotaMeter) resolveOrOpen(ctx context.Context, tx pgx.Tx, cap dom.LeaveTypeCap, employeeID string, start time.Time) (dom.LeaveQuota, error) {
	key, _, _, _, exp := windowFor(cap.CapBasis, start)
	win, err := m.store.ResolveQuotaWindow(ctx, tx, employeeID, cap.ID, key)
	if !errors.Is(err, domain.ErrNotFound) {
		return win, err
	}
	entitled, eerr := m.entitlementFor(ctx, cap, employeeID)
	if eerr != nil {
		return dom.LeaveQuota{}, eerr
	}
	return m.store.OpenQuotaWindow(ctx, tx, dom.QuotaWindowSpec{
		EmployeeID: employeeID, LeaveTypeID: cap.ID, PeriodKey: key,
		EntitledDays: entitled, Source: dom.QuotaSourceAuto,
		Remark: "auto-open " + string(cap.CapBasis), ExpiresAt: exp,
	})
}

// entitlementFor sizes a fresh window's entitled_days (ELE-2). The HR-assigned
// employee_leave_entitlement is the source of truth; when no row exists it falls back
// to the legacy sources (annual agreement for ANNUAL_POOL, then cap_value) so the
// system keeps working through the transition / before backfill (PRD §9).
func (m *QuotaMeter) entitlementFor(ctx context.Context, cap dom.LeaveTypeCap, employeeID string) (int, error) {
	ent, err := m.reader.GetEmployeeEntitlement(ctx, employeeID, cap.ID)
	if err != nil {
		return 0, err
	}
	if ent != nil {
		// Assigned: a fixed quota opens the window at that number; a NULL quota
		// (event/uncapped toggle on a quota-bearing type) falls to the type's
		// cap_value, else the non-binding sentinel.
		if ent.EntitledDays != nil {
			return *ent.EntitledDays, nil
		}
		if cap.CapValue != nil {
			return *cap.CapValue, nil
		}
		return noDayCapEntitlement, nil
	}

	// --- no entitlement row: transition fallback (pre-backfill safety) ---
	if cap.CapBasis == dom.CapBasisAnnualPool {
		annual, aerr := m.reader.GetAnnualEntitlement(ctx, employeeID)
		if aerr != nil {
			return 0, aerr
		}
		days := 0
		if annual != nil {
			days = *annual
		} else if cap.CapValue != nil {
			days = *cap.CapValue
		}
		if days > 0 {
			info, gerr := m.reader.GetEmployeeGateInfo(ctx, employeeID)
			if gerr != nil {
				return 0, gerr
			}
			return proRateAnnualPool(days, info.JoinAt), nil
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

// proRateAnnualPool applies mid-year-joiner pro-rating to the annual pool
// entitlement (2026-06-19). If joined in January, returns the full amount;
// otherwise returns floor(remaining_months * annualDays / 12).
// remaining_months = 12 - join_month + 1.
func proRateAnnualPool(annualDays int, joinAt time.Time) int {
	if joinAt.Month() == time.January {
		return annualDays
	}
	remaining := 12 - int(joinAt.Month()) + 1
	return (remaining * annualDays) / 12
}

// --- pure helpers (unit-tested without IO) ---

// chargeFor is what a request charges its window: 1 occurrence for COUNT types,
// otherwise the day count.
func chargeFor(cap dom.LeaveTypeCap, days int) int {
	if cap.CapUnit == "COUNT" {
		return 1
	}
	return days
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

// Eligibility gates (evaluateGates + the lifetime CountApprovedRequestsForType
// pre-check) were removed 2026-06-15 — HR's per-employee entitlement assignment is
// the eligibility control (PRD leave-entitlement-assignment §6, ELE-8). The gate
// columns (gender / notice_days / min_service_years) remain on leave_types as
// unenforced metadata.
