package reporting

import "time"

// BillableGroupBy is the report grouping dimension (openapi
// BillableReport.filters.group_by).
type BillableGroupBy string

const (
	GroupByEmployee    BillableGroupBy = "employee"
	GroupByPosition    BillableGroupBy = "position"
	GroupByDay         BillableGroupBy = "day"
	GroupByShiftMaster BillableGroupBy = "shift_master"
)

// BillableFilters echoes the filter set used for a billable-report run (openapi
// BillableReport.filters). CompanyID/CompanyName are nullable; Position is the
// optional free-text position filter echo (null when unfiltered).
type BillableFilters struct {
	CompanyID   *string
	CompanyName *string
	Position    *string // free-text position filter echo (nullable)
	PeriodStart string  // ISO date
	PeriodEnd   string  // ISO date
	GroupBy     BillableGroupBy
}

// BillableSummary is BillableReport.summary (verified-only totals, INV-4).
// VerificationRatePct is nil when there are zero records in the period.
type BillableSummary struct {
	TotalBillableHours   float64
	TotalWorkedHours     float64
	TotalPayableHours    float64
	TotalVerifiedRecords int
	VerificationRatePct  *float64
}

// BillablePendingSummary is BillableReport.pending_summary (records NOT yet
// billable because still unverified — BR-6 / C-1; excluded from billable totals).
type BillablePendingSummary struct {
	PendingRecords       int
	PendingHoursEstimate float64
	Note                 string
}

// BillableReportRow is one aggregated row (openapi schemas.BillableReportRow).
// group_key semantics depend on GroupBy: employee→SWP-EMP-*, position→free-text
// position, day→ISO date, shift_master→SWP-SHF-*. Position is the row's grouping
// value when GroupBy=position, otherwise the agent's position for context (nullable).
type BillableReportRow struct {
	GroupKey              string
	GroupLabel            string
	CompanyID             *string
	CompanyName           *string
	Position              *string
	WorkedHours           float64
	BillableHours         float64
	PayableHours          float64
	VerifiedRecordCount   int
	UnverifiedRecordCount int
}

// BillableReport is the full F10.3 report payload (openapi schemas.BillableReport).
type BillableReport struct {
	GeneratedAt    time.Time
	Filters        BillableFilters
	Summary        BillableSummary
	PendingSummary BillablePendingSummary
	Rows           []BillableReportRow
}
