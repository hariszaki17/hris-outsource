// Package leave — QuotaService: list balances (remaining = total - used - pending,
// pending recomputed on read), HR per-employee :adjust (refuses total < used →
// 422 RULE_VIOLATION), and HR :bulk-grant (pro-rate, partial success, preview). The
// QUOTA_EXCEEDED guard (the annual submit/over-balance gate) lives here as an
// exported seam. Audit-in-tx + notify stub on every write.
package leave

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// QuotaService implements the quota balance + grant business logic.
type QuotaService struct {
	repo QuotaRepository
	txm  TxRunner
	now  Clock
}

// NewQuotaService wires the quota service.
func NewQuotaService(repo QuotaRepository, txm TxRunner) *QuotaService {
	return &QuotaService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *QuotaService) SetClock(c Clock) { s.now = c }

// --- list ---

// List returns the quota page with pending recomputed (remaining = total-used-pending).
// Leader scope is forced to their led company.
func (s *QuotaService) List(ctx context.Context, f QuotaFilter) ([]dom.LeaveQuota, *string, bool, error) {
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
	rows, err := s.repo.ListLeaveQuotas(ctx, f)
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	// Recompute pending on read (soft-reservation; computed-on-read per 08-01).
	for i := range rows {
		pend, perr := s.repo.CountPendingLeaveDaysForQuota(ctx, rows[i].EmployeeID, rows[i].LeaveTypeID, rows[i].PeriodStart, rows[i].PeriodEnd)
		if perr == nil {
			rows[i].Pending = pend
		}
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c, cerr := encodeQuotaCursor(last.CreatedAt, last.ID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// --- adjust ---

// Adjust applies a signed delta to a quota's total (reason required, min 5). Refuses
// total + delta < used → 422 RULE_VIOLATION(fields.delta). Audited.
func (s *QuotaService) Adjust(ctx context.Context, id string, delta int, reason string) (dom.LeaveQuota, error) {
	if len([]rune(reason)) < 5 {
		return dom.LeaveQuota{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."})
	}
	actor := actorEmployeeID(ctx)
	var out dom.LeaveQuota
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		q, lerr := s.repo.GetLeaveQuotaForUpdate(ctx, tx, id)
		if errors.Is(lerr, domain.ErrNotFound) {
			return apperr.NotFound()
		}
		if lerr != nil {
			return lerr
		}
		if q.Total+delta < q.Used {
			return apperr.Rule("RULE_VIOLATION", map[string]string{
				"delta": itoa(delta),
			})
		}
		adj := dom.LeaveQuotaAdjustment{
			Delta:      delta,
			Reason:     reason,
			AdjustedBy: deref(actor),
			AdjustedAt: s.now().UTC(),
		}
		updated, uerr := s.repo.AdjustLeaveQuotaTotal(ctx, tx, id, delta, adj)
		if uerr != nil {
			return uerr
		}
		out = updated
		// TODO(Phase-11): enqueue NotificationArgs ("quota adjusted").
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "leave_quota",
			EntityID:   id,
			Before:     map[string]any{"total": q.Total},
			After:      map[string]any{"total": q.Total + delta, "delta": delta, "reason": reason, "adjusted_by": ptrStr(actor)},
		})
	})
	if err != nil {
		return dom.LeaveQuota{}, asAppErr(err)
	}
	if full, gerr := s.repo.GetLeaveQuota(ctx, out.ID); gerr == nil {
		return full, nil
	}
	return out, nil
}

// --- bulk-grant ---

// BulkGrantResult is the per-employee partial-success envelope (openapi
// LeaveQuotaBulkGrantResponse).
type BulkGrantResult struct {
	Preview       bool
	TotalAffected int
	Succeeded     []BulkGrantRow
	Failed        []BulkGrantFailure
}

// BulkGrantRow is one granted employee (or projected, in preview).
type BulkGrantRow struct {
	EmployeeID    string
	EmployeeName  *string
	QuotaID       *string
	Total         int
	IsProrated    bool
	ProrateMonths *int
}

// BulkGrantFailure is one skipped employee (e.g. new total < used).
type BulkGrantFailure struct {
	EmployeeID string
	Code       string
	Message    string
}

// BulkGrant grants annual/per-type quotas for a period. pro_rate → entitlement ×
// remaining_months/12 (half-up) for mid-year joiners; does NOT overwrite used;
// rows where new total < existing used go to failed[]; preview writes nothing.
func (s *QuotaService) BulkGrant(ctx context.Context, p dom.LeaveQuotaBulkGrantParams) (BulkGrantResult, error) {
	// Resolve the target employee set.
	candidates, err := s.resolveGrantSet(ctx, p)
	if err != nil {
		return BulkGrantResult{}, asAppErr(err)
	}
	result := BulkGrantResult{Preview: p.Preview, TotalAffected: len(candidates)}

	for _, cand := range candidates {
		total := p.EntitlementDays
		months := 12
		isProrated := false
		if p.ProRate {
			m := proRateMonths(cand.PlacementStart, p.Period, s.now())
			if m < 12 {
				total = int(math.Round(float64(p.EntitlementDays) * float64(m) / 12.0))
				months = m
				isProrated = true
			}
		}

		// Refuse to set total below the employee's current used (partial success).
		existing, ferr := s.repo.FindQuotaForEmployeeTypePeriod(ctx, cand.EmployeeID, p.LeaveTypeID, p.Period)
		if ferr == nil && total < existing.Used {
			result.Failed = append(result.Failed, BulkGrantFailure{
				EmployeeID: cand.EmployeeID,
				Code:       "RULE_VIOLATION",
				Message:    "Total kuota baru lebih kecil dari hari yang sudah terpakai.",
			})
			continue
		}

		row := BulkGrantRow{
			EmployeeID:   cand.EmployeeID,
			EmployeeName: cand.EmployeeName,
			Total:        total,
			IsProrated:   isProrated,
		}
		if isProrated {
			pm := months
			row.ProrateMonths = &pm
		}

		if p.Preview {
			result.Succeeded = append(result.Succeeded, row)
			continue
		}

		// Write the upsert + audit in its own tx (per-employee atomicity; one
		// failure never rolls back successes — bulk partial-success shape).
		var quotaID string
		werr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
			q, uerr := s.repo.UpsertLeaveQuota(ctx, tx, UpsertQuotaParams{
				EmployeeID:    cand.EmployeeID,
				LeaveTypeID:   p.LeaveTypeID,
				Period:        p.Period,
				PeriodStart:   p.PeriodStart,
				PeriodEnd:     p.PeriodEnd,
				Total:         total,
				IsProrated:    isProrated,
				ProrateMonths: months,
			})
			if uerr != nil {
				return uerr
			}
			quotaID = q.ID
			return audit.Record(ctx, tx, audit.Entry{
				Action:     audit.ActionUpdate,
				EntityType: "leave_quota",
				EntityID:   q.ID,
				After:      map[string]any{"total": total, "is_prorated": isProrated, "prorate_months": months, "bulk_grant": true},
			})
		})
		if werr != nil {
			result.Failed = append(result.Failed, BulkGrantFailure{
				EmployeeID: cand.EmployeeID,
				Code:       "RULE_VIOLATION",
				Message:    "Gagal menerbitkan kuota untuk karyawan ini.",
			})
			continue
		}
		qid := quotaID
		row.QuotaID = &qid
		result.Succeeded = append(result.Succeeded, row)
	}
	// TODO(Phase-11): enqueue NotificationArgs ("annual quota granted") batch.
	return result, nil
}

// resolveGrantSet expands the ["all"] sentinel via ListActivePlacedEmployeesForGrant
// or maps an explicit id list onto the active set (join-date for pro-rate).
func (s *QuotaService) resolveGrantSet(ctx context.Context, p dom.LeaveQuotaBulkGrantParams) ([]GrantCandidate, error) {
	active, err := s.repo.ListActivePlacedEmployeesForGrant(ctx, p.PeriodStart, p.PeriodEnd)
	if err != nil {
		return nil, err
	}
	if isAllSentinel(p.EmployeeIDs) {
		return active, nil
	}
	want := make(map[string]bool, len(p.EmployeeIDs))
	for _, id := range p.EmployeeIDs {
		want[id] = true
	}
	out := make([]GrantCandidate, 0, len(p.EmployeeIDs))
	for _, c := range active {
		if want[c.EmployeeID] {
			out = append(out, c)
		}
	}
	return out, nil
}

func isAllSentinel(ids []string) bool {
	return len(ids) == 1 && ids[0] == "all"
}

// proRateMonths returns the remaining months of the period from the join date
// (mid-year joiners get a partial year). A join before the period start → 12.
func proRateMonths(placementStart time.Time, period int, now time.Time) int {
	periodStart := time.Date(period, 1, 1, 0, 0, 0, 0, time.UTC)
	if !placementStart.After(periodStart) {
		return 12
	}
	if placementStart.Year() != period {
		return 12
	}
	// Remaining months inclusive of the join month.
	return 12 - int(placementStart.Month()) + 1
}

// --- QUOTA_EXCEEDED guard (exported seam for the submit/over-balance path) ---

// CheckQuota is the INV-1 annual over-balance guard: for an annual (quota-tracked)
// type, a request exceeding remaining → 422 QUOTA_EXCEEDED with field errors.
func CheckQuota(q dom.LeaveQuota, requestedDays int, isAnnual bool) error {
	if !isAnnual {
		return nil
	}
	if requestedDays > q.Remaining() {
		return apperr.Rule("QUOTA_EXCEEDED", map[string]string{
			"requested_days": itoa(requestedDays),
			"remaining_days": itoa(q.Remaining()),
		})
	}
	return nil
}
