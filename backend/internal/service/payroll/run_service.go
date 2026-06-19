// Package payroll — RunService: the compute-assist payroll-run business logic
// (F8.3 / PR-*). HR opens a DRAFT run, the system assembles per-employee draft
// payslips from E2 (base salary), HR reviews/adjusts, then posts to generate
// immutable payslips. MVP: assembly uses only base salary — OT/leave/attendance
// integration is deferred.
//
// Mirrors the existing period_service.go pattern: TxRunner.InTx for guarded
// transitions, audit.Record inside the tx, apperr codes for business-rule
// rejections.
package payroll

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

const (
	ActionRunOpened audit.Action = "PAYROLL_RUN_OPENED"
	ActionRunPosted audit.Action = "PAYROLL_RUN_POSTED"
)

// RunService implements the F8.3 payroll-run business logic.
type RunService struct {
	repo   RunRepository
	txm    TxRunner
	cipher *crypto.Cipher
}

// NewRunService wires the payroll-run service. cipher encrypts the *_enc money at
// the write boundary.
func NewRunService(repo RunRepository, txm TxRunner, cipher *crypto.Cipher) *RunService {
	return &RunService{repo: repo, txm: txm, cipher: cipher}
}

// --- open run (PR-1) ---

// OpenRun creates a new DRAFT payroll run for (year, month) with the given cutoff
// date. Rejects when a run already exists for the period (409).
func (s *RunService) OpenRun(ctx context.Context, year, month int, cutoffDate time.Time) (dom.PayrollRun, error) {
	if err := validYearMonth(year, month); err != nil {
		return dom.PayrollRun{}, err
	}

	caller := deref(actorEmployeeID(ctx))
	if caller == "" {
		return dom.PayrollRun{}, apperr.Forbidden()
	}

	var out dom.PayrollRun
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		existing, err := s.repo.GetRunByPeriod(ctx, year, month)
		if err != nil && !isNotFound(err) {
			return err
		}
		if existing.ID != "" {
			return apperr.Conflict("PAYROLL_RUN_ALREADY_EXISTS")
		}

		run, err := s.repo.InsertRun(ctx, tx, year, month, cutoffDate, caller)
		if err != nil {
			return err
		}
		out = run

		return audit.Record(ctx, tx, audit.Entry{
			Action:     ActionRunOpened,
			EntityType: "payroll_run",
			EntityID:   run.ID,
			After: map[string]any{
				"year":        year,
				"month":       month,
				"cutoff_date": cutoffDate.Format("2006-01-02"),
			},
		})
	}); err != nil {
		return dom.PayrollRun{}, asAppErr(err)
	}
	return out, nil
}

// --- get / list runs ---

// GetRun returns a single payroll run by ID (404 if not found).
func (s *RunService) GetRun(ctx context.Context, id string) (dom.PayrollRun, error) {
	run, err := s.repo.GetRun(ctx, id)
	if isNotFound(err) {
		return dom.PayrollRun{}, apperr.NotFound()
	}
	if err != nil {
		return dom.PayrollRun{}, apperr.Internal(err)
	}
	return run, nil
}

// ListRuns returns a simple offset-paginated list of runs (newest first).
func (s *RunService) ListRuns(ctx context.Context, limit, offset int) ([]dom.PayrollRun, error) {
	limit = httpx.ClampLimit(limit)
	rows, err := s.repo.ListRuns(ctx, limit, offset)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return rows, nil
}

// --- assemble (PR-2) ---

// AssembleResult is the assembly outcome surfaced to the handler.
type AssembleResult struct {
	Run             dom.PayrollRun
	Lines           []dom.EmployeeRunLine
	EligibleCount   int
	WithAdjustments int
}

// Assemble builds the draft employee-line set for a DRAFT run. For each eligible
// employee (active agreement spanning the run period), it reads the base salary
// and any pending adjustments. The result is in-memory only — nothing is persisted
// until PostRun.
//
// MVP: base salary only. Full OT/leave/attendance proration is deferred.
func (s *RunService) Assemble(ctx context.Context, runID string) (AssembleResult, error) {
	run, err := s.repo.GetRun(ctx, runID)
	if isNotFound(err) {
		return AssembleResult{}, apperr.NotFound()
	}
	if err != nil {
		return AssembleResult{}, apperr.Internal(err)
	}
	if run.Status != dom.RunStatusDraft {
		return AssembleResult{}, apperr.Rule("RUN_NOT_DRAFT", nil)
	}

	periodStart := time.Date(run.Year, time.Month(run.Month), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, -1)

	eligible, err := s.repo.ListEligibleEmployees(ctx, periodStart, periodEnd)
	if err != nil {
		return AssembleResult{}, apperr.Internal(err)
	}

	lines := make([]dom.EmployeeRunLine, 0, len(eligible))
	withAdj := 0

	for _, emp := range eligible {
		line := dom.EmployeeRunLine{
			EmployeeID:   emp.ID,
			EmployeeName: emp.FullName,
			EmployeeType: emp.EmployeeType,
		}

		agreement, err := s.repo.GetActiveAgreementForEmployee(ctx, emp.ID)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return AssembleResult{}, apperr.Internal(err)
		}
		line.BaseSalaryIDR = agreement.BaseSalaryIDR

		adjustments, err := s.repo.ListPendingAdjustments(ctx, emp.ID)
		if err != nil {
			return AssembleResult{}, apperr.Internal(err)
		}
		if len(adjustments) > 0 {
			withAdj++
			for _, adj := range adjustments {
				line.AdjustmentIDs = append(line.AdjustmentIDs, adj.ID)
			}
		}

		base := float64(0)
		if agreement.BaseSalaryIDR != nil {
			base = *agreement.BaseSalaryIDR
		}
		line.GrossEarnings = fmt.Sprintf("%.2f", base)
		line.GrossDeductions = "0.00"
		line.TakeHomePay = fmt.Sprintf("%.2f", base)

		wd := 30
		line.WorkingDays = &wd

		lines = append(lines, line)
	}

	return AssembleResult{
		Run:             run,
		Lines:           lines,
		EligibleCount:   len(eligible),
		WithAdjustments: withAdj,
	}, nil
}

// --- post run (PR-3) ---

// PostResult is the post outcome surfaced to the handler.
type PostResult struct {
	Run           dom.PayrollRun
	PayslipCount  int
	AdjustmentsApplied int
}

// PostRun validates the DRAFT run and generates immutable payslips for every
// assembled employee line, marks pending adjustments applied, and flips the run
// to POSTED — all in one tx + audited.
//
// MVP: the handler passes the assembled lines back in; the service encrypts each
// line's totals and inserts the payslip rows. In a full implementation the lines
// would be read from an in-flight store.
func (s *RunService) PostRun(ctx context.Context, runID string, lines []dom.EmployeeRunLine) (PostResult, error) {
	run, err := s.repo.GetRun(ctx, runID)
	if isNotFound(err) {
		return PostResult{}, apperr.NotFound()
	}
	if err != nil {
		return PostResult{}, apperr.Internal(err)
	}
	if run.Status != dom.RunStatusDraft {
		return PostResult{}, apperr.Rule("RUN_NOT_DRAFT", nil)
	}
	if len(lines) == 0 {
		return PostResult{}, apperr.Rule("RUN_NO_PAYSLIPS", nil)
	}

	caller := deref(actorEmployeeID(ctx))
	if caller == "" {
		return PostResult{}, apperr.Forbidden()
	}

	var out PostResult
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		postedRun, err := s.repo.PostRun(ctx, tx, run.ID, caller)
		if err != nil {
			return err
		}
		out.Run = postedRun

		adjApplied := 0
		for _, line := range lines {
			ge, gerr := s.cipher.Encrypt(line.GrossEarnings)
			if gerr != nil {
				return gerr
			}
			gd, gerr := s.cipher.Encrypt(line.GrossDeductions)
			if gerr != nil {
				return gerr
			}
			th, gerr := s.cipher.Encrypt(line.TakeHomePay)
			if gerr != nil {
				return gerr
			}

			sourceID := fmt.Sprintf("%s-%s", run.ID, line.EmployeeID)
			_, ierr := s.repo.InsertGeneratedPayslip(ctx, tx, InsertPayslipParams{
				EmployeeID:         line.EmployeeID,
				EmployeeName:       line.EmployeeName,
				PlacementID:        nil,
				Year:               int32(run.Year),
				Month:              int32(run.Month),
				WorkingDays:        i32ptr(line.WorkingDays),
				GrossEarningsEnc:   ge,
				GrossDeductionsEnc: gd,
				TakeHomePayEnc:     th,
				Status:             "FINAL",
				SourceSystem:       dom.PayslipSourceSystemGenerated,
				SourceID:           sourceID,
				PayrollRunID:       run.ID,
				IsPosted:           true,
				SourceType:         string(dom.SourceTypeGenerated),
				PaymentStatus:      string(dom.PaymentStatusUnpaid),
			})
			if ierr != nil {
				return ierr
			}
			out.PayslipCount++

			if len(line.AdjustmentIDs) > 0 {
				if merr := s.repo.MarkAdjustmentsApplied(ctx, tx, line.AdjustmentIDs, run.ID); merr != nil {
					return merr
				}
				adjApplied += len(line.AdjustmentIDs)
			}
		}
		out.AdjustmentsApplied = adjApplied

		return audit.Record(ctx, tx, audit.Entry{
			Action:     ActionRunPosted,
			EntityType: "payroll_run",
			EntityID:   run.ID,
			Before:     map[string]any{"status": "DRAFT"},
			After: map[string]any{
				"status":             "POSTED",
				"payslip_count":      out.PayslipCount,
				"adjustments_applied": out.AdjustmentsApplied,
			},
		})
	}); err != nil {
		return PostResult{}, asAppErr(err)
	}
	return out, nil
}

// --- run payslips ---

// RunPayslipView is a lightweight decrypted view of one run payslip.
type RunPayslipView struct {
	ID             string
	EmployeeID     string
	EmployeeName   *string
	Year           int
	Month          int
	WorkingDays    *int
	GrossEarnings  *string
	GrossDeductions *string
	TakeHomePay    *string
	PaymentStatus  string
	IsPosted       bool
}

// ListRunPayslips returns the payslips belonging to a run, decrypting totals.
func (s *RunService) ListRunPayslips(ctx context.Context, runID string, limit int) ([]RunPayslipView, error) {
	exists, err := s.repo.RunExists(ctx, runID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if !exists {
		return nil, apperr.NotFound()
	}

	limit = httpx.ClampLimit(limit)
	rows, err := s.repo.ListRunPayslips(ctx, runID, limit)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	out := make([]RunPayslipView, 0, len(rows))
	for _, r := range rows {
		v := RunPayslipView{
			ID:           r.ID,
			EmployeeID:   r.EmployeeID,
			EmployeeName: r.EmployeeName,
			Year:         int(r.Year),
			Month:        int(r.Month),
			WorkingDays:  intPtr(r.WorkingDays),
			PaymentStatus: r.PaymentStatus,
			IsPosted:     r.IsPosted,
		}
		v.GrossEarnings, _ = decryptMoney(s.cipher, r.GrossEarningsEnc)
		v.GrossDeductions, _ = decryptMoney(s.cipher, r.GrossDeductionsEnc)
		v.TakeHomePay, _ = decryptMoney(s.cipher, r.TakeHomePayEnc)
		out = append(out, v)
	}
	return out, nil
}

// --- helpers ---

func isNotFound(err error) bool {
	return errors.Is(err, domain.ErrNotFound)
}

func i32ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

func intPtr(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}
