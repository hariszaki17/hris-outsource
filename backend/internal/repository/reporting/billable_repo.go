// Package reporting (repository) — BillableRepo implements svc.BillableRepository
// over the 11-01 billable aggregation queries (verified-only, INV-4). It picks the
// group_by-specific aggregate query, maps *_minutes → *_hours (/60.0) floats, and
// runs the two summary queries. CountInScope backs the export size guard. Dates are
// converted ISO string → pgtype.Date at the boundary.
package reporting

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
)

// BillableRepo is the sqlc-backed implementation of svc.BillableRepository.
type BillableRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.BillableRepository = (*BillableRepo)(nil)

// NewBillableRepo returns a BillableRepo backed by pool.
func NewBillableRepo(pool *db.Pool) *BillableRepo {
	return &BillableRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func isoDate(s string) pgtype.Date {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func hours(min int64) float64 { return float64(min) / 60.0 }

// strPtr returns a non-nil pointer only for a non-empty company/service-line name.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Aggregate runs the group_by-specific aggregate and maps rows → BillableReportRow.
// payable_hours = worked_hours (v1 has no separate payable column — faithful
// stand-in derived from real worked_minutes). unverified_record_count is 0 per row
// (the per-group pending split is out of scope; report-level pending_summary covers
// it — matches the FE which only colors the badge when > 0).
func (r *BillableRepo) Aggregate(ctx context.Context, q svc.BillableQuery) ([]dom.BillableReportRow, error) {
	ps, pe := isoDate(q.PeriodStart), isoDate(q.PeriodEnd)

	out := make([]dom.BillableReportRow, 0, 16)
	switch q.GroupBy {
	case dom.GroupByDay:
		rows, err := r.q.BillableAggregateByDay(ctx, sqlcgen.BillableAggregateByDayParams{
			PeriodStart: ps, PeriodEnd: pe, CompanyID: q.CompanyID, ServiceLineID: q.ServiceLineID,
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, dom.BillableReportRow{
				GroupKey:            row.GroupKey,
				GroupLabel:          row.GroupLabel,
				CompanyID:           strPtr(row.CompanyID),
				CompanyName:         strPtr(row.CompanyName),
				ServiceLineID:       strPtr(row.ServiceLineID),
				ServiceLineName:     strPtr(row.ServiceLineName),
				WorkedHours:         hours(row.WorkedMinutes),
				BillableHours:       hours(row.BillableMinutes),
				PayableHours:        hours(row.WorkedMinutes),
				VerifiedRecordCount: int(row.VerifiedRecordCount),
			})
		}
	case dom.GroupByShiftMaster:
		rows, err := r.q.BillableAggregateByShiftMaster(ctx, sqlcgen.BillableAggregateByShiftMasterParams{
			PeriodStart: ps, PeriodEnd: pe, CompanyID: q.CompanyID, ServiceLineID: q.ServiceLineID,
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, dom.BillableReportRow{
				GroupKey:            row.GroupKey,
				GroupLabel:          row.GroupLabel,
				CompanyID:           strPtr(row.CompanyID),
				CompanyName:         strPtr(row.CompanyName),
				ServiceLineID:       strPtr(row.ServiceLineID),
				ServiceLineName:     strPtr(row.ServiceLineName),
				WorkedHours:         hours(row.WorkedMinutes),
				BillableHours:       hours(row.BillableMinutes),
				PayableHours:        hours(row.WorkedMinutes),
				VerifiedRecordCount: int(row.VerifiedRecordCount),
			})
		}
	default: // employee
		rows, err := r.q.BillableAggregateByEmployee(ctx, sqlcgen.BillableAggregateByEmployeeParams{
			PeriodStart: ps, PeriodEnd: pe, CompanyID: q.CompanyID, ServiceLineID: q.ServiceLineID,
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, dom.BillableReportRow{
				GroupKey:            row.GroupKey,
				GroupLabel:          row.GroupLabel,
				CompanyID:           strPtr(row.CompanyID),
				CompanyName:         strPtr(row.CompanyName),
				ServiceLineID:       strPtr(row.ServiceLineID),
				ServiceLineName:     strPtr(row.ServiceLineName),
				WorkedHours:         hours(row.WorkedMinutes),
				BillableHours:       hours(row.BillableMinutes),
				PayableHours:        hours(row.WorkedMinutes),
				VerifiedRecordCount: int(row.VerifiedRecordCount),
			})
		}
	}
	return out, nil
}

// Summary runs the verified-only totals. VerificationRatePct is computed by the
// service (it needs the pending count too); the repo leaves it nil.
func (r *BillableRepo) Summary(ctx context.Context, q svc.BillableQuery) (dom.BillableSummary, error) {
	row, err := r.q.BillableSummary(ctx, sqlcgen.BillableSummaryParams{
		PeriodStart: isoDate(q.PeriodStart), PeriodEnd: isoDate(q.PeriodEnd),
		CompanyID: q.CompanyID, ServiceLineID: q.ServiceLineID,
	})
	if err != nil {
		return dom.BillableSummary{}, err
	}
	return dom.BillableSummary{
		TotalBillableHours:   hours(row.TotalBillableMinutes),
		TotalWorkedHours:     hours(row.TotalWorkedMinutes),
		TotalPayableHours:    hours(row.TotalWorkedMinutes),
		TotalVerifiedRecords: int(row.TotalVerifiedRecords),
	}, nil
}

// PendingSummary runs the not-yet-verified record count + estimate (BR-6 / C-1).
func (r *BillableRepo) PendingSummary(ctx context.Context, q svc.BillableQuery) (dom.BillablePendingSummary, error) {
	row, err := r.q.BillablePendingSummary(ctx, sqlcgen.BillablePendingSummaryParams{
		PeriodStart: isoDate(q.PeriodStart), PeriodEnd: isoDate(q.PeriodEnd),
		CompanyID: q.CompanyID, ServiceLineID: q.ServiceLineID,
	})
	if err != nil {
		return dom.BillablePendingSummary{}, err
	}
	return dom.BillablePendingSummary{
		PendingRecords:       int(row.PendingRecords),
		PendingHoursEstimate: hours(row.PendingMinutesEstimate),
	}, nil
}

// CountInScope = verified + pending record count (backs the export size guard).
func (r *BillableRepo) CountInScope(ctx context.Context, q svc.BillableQuery) (int, error) {
	s, err := r.q.BillableSummary(ctx, sqlcgen.BillableSummaryParams{
		PeriodStart: isoDate(q.PeriodStart), PeriodEnd: isoDate(q.PeriodEnd),
		CompanyID: q.CompanyID, ServiceLineID: q.ServiceLineID,
	})
	if err != nil {
		return 0, err
	}
	p, err := r.q.BillablePendingSummary(ctx, sqlcgen.BillablePendingSummaryParams{
		PeriodStart: isoDate(q.PeriodStart), PeriodEnd: isoDate(q.PeriodEnd),
		CompanyID: q.CompanyID, ServiceLineID: q.ServiceLineID,
	})
	if err != nil {
		return 0, err
	}
	return int(s.TotalVerifiedRecords + p.PendingRecords), nil
}
