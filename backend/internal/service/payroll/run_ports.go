// Package payroll — F8.3 Payroll Run service ports (SWP-PRR-* / SWP-PS-*).
// Consumer-defined repository interfaces + param structs the RunService and
// PaymentService depend on. Reads on the pool; guarded transitions run under
// pgx.Tx. Mirrors the existing period_ports.go convention.
package payroll

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
)

// --- run repository port ---

// RunRow is the raw sqlc row for a payroll_run (no decryption needed — runs
// carry no monetary ciphertext).
type RunRow struct {
	ID         string
	Year       int32
	Month      int32
	Status     string
	CutoffDate time.Time
	CreatedBy  string
	PostedBy   *string
	PostedAt   *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// GenPayslipRow is the raw sqlc row returned by InsertGeneratedPayslip + ListRunPayslips.
type GenPayslipRow struct {
	ID                 string
	EmployeeID         string
	EmployeeName       *string
	PlacementID        *string
	Year               int32
	Month              int32
	PaidOn             *time.Time
	WorkingDays        *int32
	GrossEarningsEnc   []byte
	GrossDeductionsEnc []byte
	TakeHomePayEnc     []byte
	Status             string
	SourceSystem       string
	SourceID           string
	PayrollRunID       *string
	IsPosted           bool
	SourceType         string
	PaymentStatus      string
	CreatedAt          time.Time
}

// EligibleEmployeeRow is a lightweight eligibility row from the DB.
type EligibleEmployeeRow struct {
	ID           string
	FullName     string
	EmployeeType string
}

// ActiveAgreementRow is the active employment agreement for one employee.
type ActiveAgreementRow struct {
	ID            string
	EmployeeID    string
	BaseSalaryIDR *float64
	StartDate     time.Time
	EndDate       *time.Time
	Status        string
}

// RunRepository is the data dependency for the payroll-run service.
type RunRepository interface {
	InsertRun(ctx context.Context, tx pgx.Tx, year, month int, cutoffDate time.Time, createdBy string) (dom.PayrollRun, error)
	GetRun(ctx context.Context, id string) (dom.PayrollRun, error)
	GetRunByPeriod(ctx context.Context, year, month int) (dom.PayrollRun, error)
	ListRuns(ctx context.Context, limit, offset int) ([]dom.PayrollRun, error)
	PostRun(ctx context.Context, tx pgx.Tx, id string, postedBy string) (dom.PayrollRun, error)

	ListEligibleEmployees(ctx context.Context, periodStart, periodEnd time.Time) ([]EligibleEmployeeRow, error)
	GetActiveAgreementForEmployee(ctx context.Context, employeeID string) (ActiveAgreementRow, error)
	ListPendingAdjustments(ctx context.Context, employeeID string) ([]dom.PayrollAdjustment, error)
	MarkAdjustmentsApplied(ctx context.Context, tx pgx.Tx, adjustmentIDs []string, runID string) error

	InsertGeneratedPayslip(ctx context.Context, tx pgx.Tx, p InsertPayslipParams) (GenPayslipRow, error)
	ListRunPayslips(ctx context.Context, runID string, limit int) ([]GenPayslipRow, error)

	RunExists(ctx context.Context, id string) (bool, error)
	CountRunPayslips(ctx context.Context, runID string) (int, error)
}

// InsertPayslipParams is the payload for inserting a generated payslip.
type InsertPayslipParams struct {
	EmployeeID         string
	EmployeeName       string
	PlacementID        *string
	Year               int32
	Month              int32
	WorkingDays        *int32
	GrossEarningsEnc   []byte
	GrossDeductionsEnc []byte
	TakeHomePayEnc     []byte
	Status             string
	SourceSystem       string
	SourceID           string
	PayrollRunID       string
	IsPosted           bool
	SourceType         string
	PaymentStatus      string
}
