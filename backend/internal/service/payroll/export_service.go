// Package payroll — ExportService: the async Excel export (POST /payslips:export).
// Validates the period-or-year rule, FORCES confidential=true (Wave 2.8 lock),
// guards EXPORT_TOO_LARGE (422), then in ONE tx inserts an export_jobs QUEUED row
// AND EnqueueTx's a River PayslipExportWorker (transactional outbox). Returns the
// job stub; the handler responds 202. The worker builds the artifact + marks the
// job DONE.
package payroll

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
)

// exportRowThreshold is the EXPORT_TOO_LARGE cap (default; configurable later).
const exportRowThreshold = 50000

// ExportService implements the async payslip-export business logic.
type ExportService struct {
	repo ExportRepository
	txm  TxRunner
	jobs Jobs
}

// NewExportService wires the export service. jobs is the River enqueue seam (the
// real *jobs.Client; a fake in unit tests).
func NewExportService(repo ExportRepository, txm TxRunner, j Jobs) *ExportService {
	return &ExportService{repo: repo, txm: txm, jobs: j}
}

// ExportRequest is the validated POST /payslips:export body. Period wins over Year
// when both are present (openapi). Confidential is accepted but server-forced true.
type ExportRequest struct {
	Period      *string
	Year        *int
	EmployeeIDs []string
}

// Export queues an async export. RULE_VIOLATION (422) when neither period nor year
// is given; EXPORT_TOO_LARGE (422) when the scope exceeds the threshold. The
// QUEUED insert + the River enqueue run in ONE tx. Returns the job stub (status
// QUEUED) for the 202 response.
func (s *ExportService) Export(ctx context.Context, req ExportRequest) (dom.ExportJob, error) {
	if req.Period == nil && req.Year == nil {
		return dom.ExportJob{}, apperr.Rule("RULE_VIOLATION", map[string]string{
			"period": "Wajib mengisi period (YYYY-MM) atau year.",
		})
	}

	year, month := scopeYearMonth(req)

	// EXPORT_TOO_LARGE guard (count the rows the export would emit).
	n, err := s.repo.CountPayslipsInScope(ctx, year, month, req.EmployeeIDs)
	if err != nil {
		return dom.ExportJob{}, apperr.Internal(err)
	}
	if n > exportRowThreshold {
		return dom.ExportJob{}, apperr.Rule("EXPORT_TOO_LARGE", map[string]string{
			"period": fmt.Sprintf("Cakupan menghasilkan %d baris (maks %d).", n, exportRowThreshold),
		})
	}

	requestedBy := deref(actorEmployeeID(ctx))
	requestedByName := actorUserID(ctx) // best-effort denormalized name; FE falls back to id

	employeeIDs := req.EmployeeIDs
	if employeeIDs == nil {
		employeeIDs = []string{}
	}

	var job dom.ExportJob
	err = s.txm.InTx(ctx, func(tx pgx.Tx) error {
		j, ierr := s.repo.InsertExportJob(ctx, tx, ExportJobParams{
			RequestedByID:    requestedBy,
			RequestedByName:  requestedByName,
			ScopePeriod:      req.Period,
			ScopeYear:        req.Year,
			ScopeEmployeeIDs: employeeIDs,
		})
		if ierr != nil {
			return ierr
		}
		job = j

		// Transactional outbox: enqueue the River worker in the SAME tx as the
		// QUEUED insert. If the tx rolls back the job is never enqueued; if it
		// commits the worker is guaranteed to run.
		if eerr := s.jobs.EnqueueTx(ctx, tx, jobs.PayslipExportArgs{
			JobID:       j.ID,
			Period:      req.Period,
			Year:        req.Year,
			EmployeeIDs: employeeIDs,
		}); eerr != nil {
			return eerr
		}

		return audit.Record(ctx, tx, audit.Entry{
			Action:     "CREATE",
			EntityType: "payslip_export",
			EntityID:   j.ID,
			After: map[string]any{
				"confidential": true, // PA-7 confidentiality marking
				"scope_period": deref(req.Period),
			},
		})
	})
	if err != nil {
		return dom.ExportJob{}, asAppErr(err)
	}

	// Confidentiality lock: the DB DEFAULT already sets true, but assert it on the
	// returned stub regardless of the client's input (Wave 2.8).
	job.Confidential = true
	job.PollURL = "/api/v1/exports/" + job.ID
	return job, nil
}

// scopeYearMonth resolves (year, month) for the CountPayslipsInScope guard. Period
// (YYYY-MM) wins over Year; a bare Year leaves month nil (full-year scope).
func scopeYearMonth(req ExportRequest) (year, month *int) {
	if req.Year != nil {
		y := *req.Year
		year = &y
	}
	if req.Period != nil && len(*req.Period) == 7 {
		p := *req.Period
		yi := parseIntPrefix(p[0:4])
		mi := parseIntPrefix(p[5:7])
		if yi > 0 {
			year = &yi
		}
		if mi > 0 {
			month = &mi
		}
	}
	return year, month
}

func parseIntPrefix(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
