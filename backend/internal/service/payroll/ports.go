// Package payroll — E8 historical, read-only payroll services (F8.1/F8.2 /
// PAY-01/PAY-02). The web surface is the HR/Super-Admin payroll archive: list
// payslips, open a payslip detail (full earnings/deductions/benefits breakdown),
// list + append immutable audit notes, and queue an async Excel export.
//
// The gnarly parts this package owns:
//   - decrypt-AT-THE-BOUNDARY: monetary fields are stored as AES-256-GCM
//     ciphertext (INV-2; 10-01 crypto.Cipher). The service decrypts each *_enc on
//     read; a row whose ciphertext fails to open surfaces as a 200 OK payslip with
//     status DECRYPT_FAIL + money nulled + empty breakdown (NOT a 4xx).
//   - async export: POST /payslips:export inserts an export_jobs QUEUED row AND
//     EnqueueTx's a River PayslipExportWorker in the SAME tx (transactional
//     outbox); the worker builds the artifact + marks the job DONE.
//
// Mirrors the Phase-2 foundations slice (read + list + simple write + audit) and
// the existing River NotificationWorker (async). There is NO two-level state
// machine here (simpler than E6/E7).
package payroll

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
)

// TxRunner runs a closure inside a DB transaction (db.TxManager satisfies it).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock supplies the current time (overridable in tests).
type Clock func() time.Time

// Jobs is the River enqueue seam (the real *jobs.Client satisfies it). Declaring
// it as an interface lets 10-03 unit-test the export service with a fake — no
// real River client / Postgres needed. EnqueueTx inserts a job in the SAME tx as
// the export_jobs QUEUED insert (transactional outbox).
type Jobs interface {
	EnqueueTx(ctx context.Context, tx pgx.Tx, args river.JobArgs) error
}

// --- filters ---

// PayslipFilter is the decoded GET /payslips query (cursor-paged, paid_on DESC).
// employee_id/period/year/status are all optional. RBAC is route-enforced
// (hr/super = global) so there is no agent-scope branch on the web archive.
type PayslipFilter struct {
	EmployeeID   *string
	Year         *int
	Month        *int // from the period YYYY-MM split
	Status       *string
	Limit        int
	CursorPaidOn *time.Time
	CursorID     *string
}

// --- raw repository rows (ciphertext NOT decrypted in the repo) ---

// PayslipRow is one payslip with the RAW *_enc ciphertext + plaintext metadata.
// The repo returns this; the SERVICE decrypts (decrypt-at-boundary). Keeping the
// ciphertext on an intermediate struct (not on the domain Payslip) means the
// domain type only ever carries DECRYPTED money.
type PayslipRow struct {
	ID                 string
	EmployeeID         string
	EmployeeName       *string
	PlacementID        *string
	Year               int
	Month              int
	PaidOn             *time.Time
	WorkingDays        *int
	GrossEarningsEnc   []byte
	GrossDeductionsEnc []byte
	TakeHomePayEnc     []byte
	Status             string
	SourceSystem       string
	SourceID           string
	CreatedAt          time.Time
}

// LineRow is one component/benefit line with the RAW value_enc ciphertext.
type LineRow struct {
	Name     string
	Kind     string // EARNING | DEDUCTION (empty for benefits)
	ValueEnc []byte
	ForBPJS  bool
}

// AuditNoteRow carries one payslip_audit_notes insert.
type AuditNoteRow struct {
	ID         string
	PayslipID  string
	Seq        int
	Text       string
	AuthorID   string
	AuthorName *string
}

// ExportJobParams carries one export_jobs QUEUED insert (transactional outbox).
type ExportJobParams struct {
	RequestedByID    string
	RequestedByName  *string
	ScopePeriod      *string
	ScopeYear        *int
	ScopeEmployeeIDs []string
}

// --- repository ports ---

// PayslipRepository is the read/note data dependency for the payslip service.
// Reads return RAW ciphertext rows; the service decrypts.
type PayslipRepository interface {
	ListPayslips(ctx context.Context, f PayslipFilter) ([]PayslipRow, error)
	GetPayslip(ctx context.Context, id string) (PayslipRow, error)
	ListComponents(ctx context.Context, payslipID string) ([]LineRow, error)
	ListBenefits(ctx context.Context, payslipID string) ([]LineRow, error)

	PayslipExists(ctx context.Context, id string) (bool, error)
	ListAuditNotes(ctx context.Context, payslipID string, cursorSeq *int, cursorCreatedAt *time.Time, limit int) ([]dom.PayslipAuditNote, error)
	CountAuditNotes(ctx context.Context, payslipID string) (int, error)
	InsertAuditNote(ctx context.Context, tx pgx.Tx, p AuditNoteRow) (dom.PayslipAuditNote, error)
}

// ExportRepository is the data dependency for the async export service + worker.
type ExportRepository interface {
	CountPayslipsInScope(ctx context.Context, year, month *int, employeeIDs []string) (int, error)
	InsertExportJob(ctx context.Context, tx pgx.Tx, p ExportJobParams) (dom.ExportJob, error)
	GetExportJob(ctx context.Context, id string) (dom.ExportJob, error)
}
