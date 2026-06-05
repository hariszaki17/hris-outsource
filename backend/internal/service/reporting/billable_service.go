// Package reporting — BillableService: the F10.3 attendance & billable-hours report
// (GET /reports/attendance-billable). Aggregates VERIFIED attendance (INV-4 / BR-1)
// on billable codes into hours by group_by. Scope-aware: a shift_leader is forced
// to their own company (E3 INV-3); a leader requesting another company → 403
// OUT_OF_SCOPE. Validates the period (end >= start; <= 1 year else 422
// REPORT_PERIOD_TOO_WIDE). Hours only — no rates/amounts (EPICS §8).
package reporting

import (
	"context"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// maxReportSpan caps the date range (BR / openapi REPORT_PERIOD_TOO_WIDE).
const maxReportSpan = 366 * 24 * time.Hour

// BillableParams is the decoded request (pre-scope).
type BillableParams struct {
	CompanyID     *string
	ServiceLineID *string
	PeriodStart   string // ISO date (required)
	PeriodEnd     string // ISO date (required)
	GroupBy       dom.BillableGroupBy
}

// BillableService implements GET /reports/attendance-billable.
type BillableService struct {
	repo BillableRepository
}

// NewBillableService wires the billable service.
func NewBillableService(repo BillableRepository) *BillableService {
	return &BillableService{repo: repo}
}

// GetBillableReport validates + scopes the request, runs the aggregation, and
// assembles the BillableReport. Leader scope is enforced here (own company only).
func (s *BillableService) GetBillableReport(ctx context.Context, in BillableParams) (dom.BillableReport, error) {
	q, err := resolveBillableScope(ctx, in)
	if err != nil {
		return dom.BillableReport{}, err
	}

	rows, err := s.repo.Aggregate(ctx, q)
	if err != nil {
		return dom.BillableReport{}, apperr.Internal(err)
	}
	summary, err := s.repo.Summary(ctx, q)
	if err != nil {
		return dom.BillableReport{}, apperr.Internal(err)
	}
	pending, err := s.repo.PendingSummary(ctx, q)
	if err != nil {
		return dom.BillableReport{}, apperr.Internal(err)
	}

	// verification_rate_pct = verified / (verified + pending); null when no records.
	denom := summary.TotalVerifiedRecords + pending.PendingRecords
	if denom > 0 {
		pct := float64(summary.TotalVerifiedRecords) / float64(denom) * 100.0
		summary.VerificationRatePct = &pct
	}

	// pending_summary note (BR-6 callout copy); empty when nothing pending.
	if pending.PendingRecords > 0 {
		pending.Note = "Belum dapat ditagih hingga diverifikasi."
	}

	// Echo the company/service-line display names from the first row when present.
	var companyName, serviceLineName *string
	for _, r := range rows {
		if q.CompanyID != nil && r.CompanyName != nil {
			companyName = r.CompanyName
		}
		if q.ServiceLineID != nil && r.ServiceLineName != nil {
			serviceLineName = r.ServiceLineName
		}
	}

	return dom.BillableReport{
		GeneratedAt: time.Now().UTC(),
		Filters: dom.BillableFilters{
			CompanyID:       q.CompanyID,
			CompanyName:     companyName,
			ServiceLineID:   q.ServiceLineID,
			ServiceLineName: serviceLineName,
			PeriodStart:     q.PeriodStart,
			PeriodEnd:       q.PeriodEnd,
			GroupBy:         q.GroupBy,
		},
		Summary:        summary,
		PendingSummary: pending,
		Rows:           rows,
	}, nil
}

// resolveBillableScope validates the period + applies leader scope, returning the
// repo query.
func resolveBillableScope(ctx context.Context, in BillableParams) (BillableQuery, error) {
	start, serr := time.Parse("2006-01-02", in.PeriodStart)
	end, eerr := time.Parse("2006-01-02", in.PeriodEnd)
	if in.PeriodStart == "" || in.PeriodEnd == "" || serr != nil || eerr != nil {
		return BillableQuery{}, apperr.Invalid(map[string]string{
			"period_start": "period_start & period_end wajib (YYYY-MM-DD).",
		})
	}
	if end.Before(start) {
		return BillableQuery{}, apperr.Invalid(map[string]string{
			"period_end": "period_end tidak boleh sebelum period_start.",
		})
	}
	if end.Sub(start) > maxReportSpan {
		return BillableQuery{}, apperr.Rule("REPORT_PERIOD_TOO_WIDE", map[string]string{
			"period_end": "Maksimum 1 tahun dari period_start.",
		})
	}

	groupBy := in.GroupBy
	if groupBy == "" {
		groupBy = dom.GroupByEmployee
	}

	q := BillableQuery{
		CompanyID:     in.CompanyID,
		ServiceLineID: in.ServiceLineID,
		PeriodStart:   in.PeriodStart,
		PeriodEnd:     in.PeriodEnd,
		GroupBy:       groupBy,
	}

	// Scope: a shift_leader is locked to their own company (E3 INV-3).
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return BillableQuery{}, apperr.Unauthenticated()
	}
	if p.Role == auth.RoleShiftLeader {
		if p.CompanyID == "" {
			return BillableQuery{}, apperr.OutOfScope()
		}
		if q.CompanyID != nil && *q.CompanyID != p.CompanyID {
			return BillableQuery{}, apperr.OutOfScope()
		}
		cid := p.CompanyID
		q.CompanyID = &cid
	}
	return q, nil
}
