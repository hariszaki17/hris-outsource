// Package payroll holds the dependency-free domain types for the E8 slice
// (F8.1/F8.2 / SWP-PS-* / SWP-EXP-*). Historical, read-only payroll: these
// structs are shared between the payroll service and repository and map 1:1 onto
// the openapi Payslip / EarningLine / DeductionLine / BenefitLine /
// PayslipAuditNote / PayslipExportJob shapes (10-02 maps sqlc rows → these →
// DTOs, decrypting the *_enc ciphertext at the boundary).
//
// Convention (mirrors internal/domain/overtime + internal/domain/leave):
// nullable columns / decrypt-failed money are pointers (nil = null on the wire).
//
// MONEY IS AN OPAQUE STRING (INV-2): GrossEarnings/etc. and line Values are
// decrypted decimal Money strings (e.g. "8500000.00"); they are NEVER parsed or
// computed server-side — there is no monetary method on any type here. A row
// whose ciphertext fails to decrypt surfaces with Status DECRYPT_FAIL,
// DecryptFail=true, all money nil, and LockedReason "decrypt_fail" (200 OK, NOT
// an error — openapi PayslipStatus.DECRYPT_FAIL).
package payroll

import "time"

// PayslipStatus is the payslip read status. Values are pinned to openapi
// schemas.PayslipStatus (AUTHORITATIVE) — byte-for-byte.
type PayslipStatus string

const (
	PayslipStatusFinal       PayslipStatus = "FINAL"
	PayslipStatusDecryptFail PayslipStatus = "DECRYPT_FAIL"
)

// ExportJobStatus is the async export-job lifecycle. Pinned to openapi
// PayslipExportJob.status — DONE is the terminal-success value.
type ExportJobStatus string

const (
	ExportJobStatusQueued  ExportJobStatus = "QUEUED"
	ExportJobStatusRunning ExportJobStatus = "RUNNING"
	ExportJobStatusDone    ExportJobStatus = "DONE"
	ExportJobStatusFailed  ExportJobStatus = "FAILED"
)

// LockedReasonDecryptFail is the openapi schemas.LockedReason value set on
// decrypt-failed payslips / lines (the only v1 value).
const LockedReasonDecryptFail = "decrypt_fail"

// SourceSystemLumenSwp is the only SourceRef.system value (E9 migration source).
const SourceSystemLumenSwp = "lumen_swp"

// SourceRef is migration traceability (openapi schemas.SourceRef). System is
// always "lumen_swp"; SourceID is the legacy employee_payslips.id (a STRING,
// never parsed).
type SourceRef struct {
	System   string
	SourceID string
}

// EarningLine is one earnings-breakdown row (openapi EarningLine). HR-only on
// read (INV-3). Value is the DECRYPTED Money string, nil on decrypt-fail (with
// LockedReason set). On a DECRYPT_FAIL payslip the parent array is [].
type EarningLine struct {
	Name         string
	Value        *string // decrypted Money string; nil on decrypt-fail
	ForBPJS      bool
	LockedReason *string // "decrypt_fail" when the line value failed to decrypt
}

// DeductionLine is one deductions-breakdown row (openapi DeductionLine). Same
// shape as EarningLine.
type DeductionLine struct {
	Name         string
	Value        *string
	ForBPJS      bool
	LockedReason *string
}

// BenefitLine is one employer-borne benefit (openapi BenefitLine). HR-only
// (INV-4). No for_bpjs field per the contract.
type BenefitLine struct {
	Name         string
	Value        *string
	LockedReason *string
}

// Payslip is the domain entity for one historical payslip (openapi Payslip).
// Money fields are DECRYPTED Money strings (nil on decrypt-fail). Earnings /
// Deductions / Benefits are HR-only (omitted from agent responses by 10-02) and
// [] on a DECRYPT_FAIL payslip. ReadOnly is always true (INV-1).
type Payslip struct {
	ID           string
	EmployeeID   string
	EmployeeName *string
	PlacementID  *string

	Year   int
	Month  int
	Period string // derived YYYY-MM (year + zero-padded month)

	PaidOn      *time.Time
	WorkingDays *int

	// Decrypted Money strings (e.g. "8500000.00"); nil on decrypt-fail. Opaque
	// — never parsed/computed (INV-2).
	GrossEarnings   *string
	GrossDeductions *string
	TakeHomePay     *string

	Status       PayslipStatus
	DecryptFail  bool
	ReadOnly     bool    // always true in v1 (INV-1)
	LockedReason *string // "decrypt_fail" when DecryptFail

	Earnings   []EarningLine
	Deductions []DeductionLine
	Benefits   []BenefitLine

	Source    SourceRef
	CreatedAt time.Time
}

// PayslipAuditNote is one append-only HR annotation (openapi PayslipAuditNote).
// ID is the composite "{payslip_id}-NOTE-{seq}".
type PayslipAuditNote struct {
	ID         string
	PayslipID  string
	Text       string
	AuthorID   string
	AuthorName *string
	CreatedAt  time.Time
}

// ExportJob is the async export-job stub (openapi PayslipExportJob). Confidential
// is server-enforced true. Scope* echo the request scope. RowCount / ArtifactRef
// are set by the worker on completion. PollURL is the E10 export resource URL.
type ExportJob struct {
	ID               string
	Status           ExportJobStatus
	Format           string
	Confidential     bool
	RequestedByID    string
	RequestedByName  *string
	ScopePeriod      *string
	ScopeYear        *int
	ScopeEmployeeIDs []string
	RowCount         *int
	ArtifactRef      *string
	ErrorMessage     *string
	RequestedAt      time.Time
	StartedAt        *time.Time
	CompletedAt      *time.Time
	PollURL          string
}
