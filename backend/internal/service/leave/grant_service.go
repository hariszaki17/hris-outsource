// Package leave — GrantService: the per-employee grant-lot ledger (F6.1, resolved
// 2026-06-08). Owns the FIFO allocator (earmark-filtered, soonest expires_at first),
// the pending→consumed reservation lifecycle (reserve at SUBMIT, commit + write
// LeaveConsumption rows at APPROVE, release at REJECT/CANCEL, reverse exact rows at
// CANCEL-APPROVED/SHORTEN), the computed balance read model, and HR grant create /
// patch. Never drives a lot negative (LQ-5): a request reserves only available days;
// insufficient → caller blocks with QUOTA_EXCEEDED / BALANCE_RECHECK_FAILED. Earmarked
// lots are invisible to ordinary FIFO (LQ-10 earmark isolation). All writes are in-tx
// and audited via internal/platform/audit.
package leave

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// earmarkPoolSentinel is the GetActiveLotsForAllocation match value for the flat
// (unearmarked) pool — ordinary requests draw ONLY these lots.
const earmarkPoolSentinel = "__null"

// GrantService implements the grant-lot ledger + FIFO allocator.
type GrantService struct {
	repo GrantRepository
	txm  TxRunner
	now  Clock
}

// NewGrantService wires the grant service.
func NewGrantService(repo GrantRepository, txm TxRunner) *GrantService {
	return &GrantService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *GrantService) SetClock(c Clock) { s.now = c }

// --- HR grant create (POST /leave-grants) ---

// Create inserts one grant-lot. remark is required (LQ-6 audit); amount_days < 0 is
// refused; source must be a valid LeaveGrantSource. Audited in-tx.
func (s *GrantService) Create(ctx context.Context, p CreateGrantParams) (dom.LeaveGrant, error) {
	if p.Amount < 0 {
		return dom.LeaveGrant{}, apperr.Invalid(map[string]string{"amount_days": "Tidak boleh negatif."})
	}
	if !dom.ValidGrantSource(string(p.Source)) {
		return dom.LeaveGrant{}, apperr.Invalid(map[string]string{"source": "Sumber hibah tidak valid."})
	}
	if p.Remark == nil || len([]rune(*p.Remark)) < 5 {
		return dom.LeaveGrant{}, apperr.Invalid(map[string]string{"remark": "Wajib diisi (minimum 5 karakter)."})
	}
	if p.EffectiveFrom.IsZero() {
		p.EffectiveFrom = s.now().UTC()
	}
	if !p.ExpiresAt.After(p.EffectiveFrom) {
		return dom.LeaveGrant{}, apperr.Invalid(map[string]string{"expires_at": "Harus setelah tanggal berlaku."})
	}
	actor := actorEmployeeID(ctx)
	p.CreatedBy = actorUserIDPtr(ctx)
	var out dom.LeaveGrant
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		g, cerr := s.repo.CreateLeaveGrant(ctx, tx, p)
		if cerr != nil {
			return cerr
		}
		out = g
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "leave_grant",
			EntityID:   g.ID,
			After: map[string]any{
				"employee_id": g.EmployeeID, "amount_days": g.Amount, "source": string(g.Source),
				"earmark": ptrStr(g.Earmark), "expires_at": g.ExpiresAt.Format("2006-01-02"),
				"remark": ptrStr(g.Remark), "granted_by": ptrStr(actor),
			},
		})
	})
	if err != nil {
		return dom.LeaveGrant{}, asAppErr(err)
	}
	if full, gerr := s.repo.GetLeaveGrant(ctx, out.ID); gerr == nil {
		return full, nil
	}
	return out, nil
}

// --- HR grant patch (PATCH /leave-grants/{id}) ---

// Patch adjusts a lot's amount/expires_at/earmark with a required remark. Refuses an
// amount below the lot's current consumed+pending (422 RULE_VIOLATION). Audited.
func (s *GrantService) Patch(ctx context.Context, p PatchGrantParams) (dom.LeaveGrant, error) {
	if len([]rune(p.Remark)) < 5 {
		return dom.LeaveGrant{}, apperr.Invalid(map[string]string{"remark": "Wajib diisi (minimum 5 karakter)."})
	}
	actor := actorEmployeeID(ctx)
	var out dom.LeaveGrant
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cur, gerr := s.repo.GetLeaveGrantForUpdate(ctx, tx, p.ID)
		if errors.Is(gerr, domain.ErrNotFound) {
			return apperr.NotFound()
		}
		if gerr != nil {
			return gerr
		}
		if p.Amount != nil {
			if *p.Amount < cur.Consumed+cur.Pending {
				return apperr.Rule("RULE_VIOLATION", map[string]string{"amount_days": itoa(*p.Amount)})
			}
			if *p.Amount < 0 {
				return apperr.Invalid(map[string]string{"amount_days": "Tidak boleh negatif."})
			}
		}
		updated, uerr := s.repo.PatchLeaveGrant(ctx, tx, p)
		if uerr != nil {
			return uerr
		}
		out = updated
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "leave_grant",
			EntityID:   p.ID,
			Before:     map[string]any{"amount_days": cur.Amount, "expires_at": cur.ExpiresAt.Format("2006-01-02"), "earmark": ptrStr(cur.Earmark)},
			After:      map[string]any{"amount_days": updated.Amount, "expires_at": updated.ExpiresAt.Format("2006-01-02"), "earmark": ptrStr(updated.Earmark), "remark": p.Remark, "adjusted_by": ptrStr(actor)},
		})
	})
	if err != nil {
		return dom.LeaveGrant{}, asAppErr(err)
	}
	if full, gerr := s.repo.GetLeaveGrant(ctx, out.ID); gerr == nil {
		return full, nil
	}
	return out, nil
}

// --- ledger list (GET /leave-grants) ---

// List returns the grant-lot page (FIFO-ordered). Leader scope is forced to their led
// company; an explicit company_id outside scope yields 403 OUT_OF_SCOPE.
func (s *GrantService) List(ctx context.Context, f GrantFilter) ([]dom.LeaveGrant, *string, bool, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, nil, false, apperr.Unauthenticated()
	}
	if p.Role == auth.RoleShiftLeader {
		if f.CompanyID != nil && *f.CompanyID != p.CompanyID {
			return nil, nil, false, apperr.OutOfScope()
		}
		cid := p.CompanyID
		f.CompanyID = &cid
	}
	limit := clampLimit(f.Limit)
	f.Limit = limit + 1
	rows, err := s.repo.ListLeaveGrants(ctx, f, s.now().UTC())
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c, cerr := encodeGrantCursor(last.ExpiresAt, last.ID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// --- aggregate balance list (GET /leave-balances) ---

// ListBalances returns the per-employee aggregate balance page for the /leave/quotas
// screen — ONE ROW PER EMPLOYEE, rolling up all their ACTIVE lots. Cursor-paged on
// (full_name, employee_id) ascending; fetches limit+1 to compute has_more. RBAC mirrors
// the sibling listLeaveGrants (hr/super/leader); the underlying GetLeaveBalanceByEmployee
// applies no leader company-narrowing, so neither does this aggregate list.
func (s *GrantService) ListBalances(ctx context.Context, f BalanceListFilter) ([]dom.EmployeeLeaveBalance, *string, bool, error) {
	if _, ok := auth.PrincipalFrom(ctx); !ok {
		return nil, nil, false, apperr.Unauthenticated()
	}
	limit := clampLimit(f.Limit)
	f.Limit = limit + 1
	rows, err := s.repo.ListLeaveBalances(ctx, f, s.now().UTC())
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c, cerr := encodeBalanceCursor(last.FullName, last.EmployeeID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// Get loads one lot with its consumptions (GET /leave-grants/{id}).
func (s *GrantService) Get(ctx context.Context, id string, includeConsumptions bool) (dom.LeaveGrant, error) {
	g, err := s.repo.GetLeaveGrant(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.LeaveGrant{}, apperr.NotFound()
	}
	if err != nil {
		return dom.LeaveGrant{}, apperr.Internal(err)
	}
	if includeConsumptions {
		cons, cerr := s.repo.ListConsumptionsForGrant(ctx, id)
		if cerr == nil {
			g.Consumptions = cons
		}
	}
	return g, nil
}

// --- computed balance (GET /leave-balances/by-employee/{id}) ---

// Balance computes the employee's balance over active lots: the flat pool plus one
// line per active earmarked lot. include_expired_lots adds zeroed/expired lots to
// all_lots[] for the history view.
func (s *GrantService) Balance(ctx context.Context, employeeID string, includeExpired bool) (dom.LeaveBalance, error) {
	// SELF scope: an agent may read ONLY their own balance.
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleAgent && employeeID != p.EmployeeID {
		return dom.LeaveBalance{}, apperr.Forbidden()
	}
	now := s.now().UTC()
	groups, err := s.repo.SumActiveBalanceByEarmark(ctx, employeeID, now)
	if err != nil {
		return dom.LeaveBalance{}, apperr.Internal(err)
	}
	bal := dom.LeaveBalance{EmployeeID: employeeID}
	for _, g := range groups {
		bal.PendingTotal += g.Pending
		if g.Earmark == nil {
			bal.PoolRemaining += g.Remaining
		}
		if g.Remaining > 0 && g.NextExpiry != nil {
			if bal.NextExpiry == nil || g.NextExpiry.Before(*bal.NextExpiry) {
				ne := *g.NextExpiry
				bal.NextExpiry = &ne
			}
		}
	}
	// Per-lot earmark lines (and all_lots) need the lot rows: list active (or all) lots.
	f := GrantFilter{EmployeeID: &employeeID, IncludeExpired: includeExpired, Limit: 200}
	lots, lerr := s.repo.ListLeaveGrants(ctx, f, now)
	if lerr != nil {
		return dom.LeaveBalance{}, apperr.Internal(lerr)
	}
	for _, lot := range lots {
		if lot.EmployeeName != nil && bal.EmployeeName == nil {
			bal.EmployeeName = lot.EmployeeName
		}
		if lot.Earmark != nil && lot.IsActive(now) {
			bal.Earmarked = append(bal.Earmarked, dom.LeaveBalanceEarmarkLine{
				GrantID:   lot.ID,
				Earmark:   *lot.Earmark,
				Source:    lot.Source,
				Remaining: lot.Remaining(),
				ExpiresAt: lot.ExpiresAt,
			})
		}
	}
	if includeExpired {
		bal.AllLots = lots
	}
	return bal, nil
}

// --- FIFO allocator (called by LeaveService inside the approval tx) ---

// allocate computes the FIFO per-lot split of want days over the employee's active,
// matching-earmark lots (soonest expires_at first). It does NOT write — it returns the
// split and the total available so the caller can block when insufficient. earmark==nil
// ⇒ the flat pool (ordinary FIFO); a non-nil earmark draws ONLY matching lots (LQ-10).
func (s *GrantService) allocate(ctx context.Context, tx pgx.Tx, employeeID string, earmark *string, want int) (alloc []dom.AllocationLine, available int, err error) {
	match := earmarkPoolSentinel
	if earmark != nil {
		match = *earmark
	}
	lots, lerr := s.repo.GetActiveLotsForAllocation(ctx, tx, employeeID, match, s.now().UTC())
	if lerr != nil {
		return nil, 0, lerr
	}
	for _, lot := range lots {
		available += lot.Remaining()
	}
	remaining := want
	for _, lot := range lots {
		if remaining <= 0 {
			break
		}
		take := lot.Remaining()
		if take <= 0 {
			continue
		}
		if take > remaining {
			take = remaining
		}
		alloc = append(alloc, dom.AllocationLine{GrantID: lot.ID, Days: take, ExpiresAt: lot.ExpiresAt})
		remaining -= take
	}
	return alloc, available, nil
}

// reserve FIFO-reserves want days into pending across lots at SUBMIT. Returns the
// allocation snapshot. Blocks (QUOTA_EXCEEDED) when available < want — never partial,
// never negative (LQ-5).
func (s *GrantService) reserve(ctx context.Context, tx pgx.Tx, employeeID string, earmark *string, want int) ([]dom.AllocationLine, int, error) {
	alloc, available, err := s.allocate(ctx, tx, employeeID, earmark, want)
	if err != nil {
		return nil, 0, err
	}
	if available < want {
		return nil, available, apperr.Rule("QUOTA_EXCEEDED", map[string]string{
			"requested_days": itoa(want),
			"remaining_days": itoa(available),
		})
	}
	for _, a := range alloc {
		if rerr := s.repo.ReservePending(ctx, tx, a.GrantID, a.Days); rerr != nil {
			return nil, available, rerr
		}
	}
	return alloc, available, nil
}

// commit converts a reservation to consumption at APPROVE: pending→consumed on each
// lot in alloc + one LeaveConsumption row per lot. The allocation is the exact split
// produced at submit-time (re-derived by the caller from a fresh FIFO at approve time
// when no prior reservation exists, e.g. override).
func (s *GrantService) commit(ctx context.Context, tx pgx.Tx, requestID string, alloc []dom.AllocationLine) error {
	for _, a := range alloc {
		if cerr := s.repo.CommitReservation(ctx, tx, a.GrantID, a.Days); cerr != nil {
			return cerr
		}
		if _, aerr := s.repo.ApplyConsumption(ctx, tx, requestID, a.GrantID, a.Days); aerr != nil {
			return aerr
		}
	}
	return nil
}

// release returns pending days to the lots at REJECT/CANCEL of a still-pending request.
func (s *GrantService) release(ctx context.Context, tx pgx.Tx, alloc []dom.AllocationLine) error {
	for _, a := range alloc {
		if rerr := s.repo.ReleasePending(ctx, tx, a.GrantID, a.Days); rerr != nil {
			return rerr
		}
	}
	return nil
}

// reverseConsumptions reverses the EXACT committed consumption rows of a request at
// CANCEL-APPROVED / full SHORTEN: consumed -= days on each lot + delete the rows.
func (s *GrantService) reverseConsumptions(ctx context.Context, tx pgx.Tx, requestID string) (int, error) {
	cons, err := s.repo.ListConsumptionsForRequest(ctx, requestID)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, c := range cons {
		if rerr := s.repo.ReverseConsumption(ctx, tx, c.GrantID, c.Days); rerr != nil {
			return 0, rerr
		}
		total += c.Days
	}
	if len(cons) > 0 {
		if derr := s.repo.DeleteConsumptionsForRequest(ctx, tx, requestID); derr != nil {
			return 0, derr
		}
	}
	return total, nil
}
