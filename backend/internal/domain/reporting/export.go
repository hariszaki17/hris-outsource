package reporting

import "time"

// ReportType identifies the report being exported (openapi schemas.ReportType),
// byte-for-byte. Each value selects a downstream query + filter schema (11-02b).
type ReportType string

const (
	ReportAttendanceBillable ReportType = "ATTENDANCE_BILLABLE"
	ReportAttendanceRaw      ReportType = "ATTENDANCE_RAW"
	ReportLeaveHistory       ReportType = "LEAVE_HISTORY"
	ReportOvertimeRecords    ReportType = "OVERTIME_RECORDS"
	ReportPayslips           ReportType = "PAYSLIPS"
	ReportAuditLog           ReportType = "AUDIT_LOG"
)

// ExportFormat is the output format (openapi schemas.ExportFormat). EXCEL only in
// v1; PDF/CSV → EXPORT_FORMAT_UNSUPPORTED (enforced in 11-02b). The DB also still
// accepts the Phase-10 XLSX value for the payslip path.
type ExportFormat string

const (
	FormatExcel ExportFormat = "EXCEL"
	FormatPDF   ExportFormat = "PDF"
	FormatCSV   ExportFormat = "CSV"
	// FormatXLSX is the legacy Phase-10 payslip-export DB value (kept valid by the
	// 00036 format CHECK). The generic export path uses FormatExcel.
	FormatXLSX ExportFormat = "XLSX"
)

// ExportStatus values here are the DB-STORED lifecycle states (00036 CHECK):
// QUEUED/RUNNING/DONE/FAILED/CANCELLED. The 11-02b service maps these to the WIRE
// openapi schemas.ExportStatus (QUEUED/PROCESSING/COMPLETED/FAILED/CANCELLED):
// RUNNING<->PROCESSING and DONE<->COMPLETED at the DTO boundary. Keep the DB values
// here; do NOT pre-map.
type ExportStatus string

const (
	StatusQueued    ExportStatus = "QUEUED"
	StatusRunning   ExportStatus = "RUNNING" // wire: PROCESSING
	StatusDone      ExportStatus = "DONE"    // wire: COMPLETED
	StatusFailed    ExportStatus = "FAILED"
	StatusCancelled ExportStatus = "CANCELLED"
)

// ExportJob is the generalized async export-job entity (openapi schemas.ExportJob).
// Status holds the DB value (mapped to the wire enum by the service). ErrCode /
// ErrMessage populate the openapi error object only when Status == StatusFailed.
// Filters echoes the request filter set. ProgressPercent / Filename / SizeBytes /
// FileURL / CompletedAt / ExpiresAt are nil until the worker sets them.
type ExportJob struct {
	ID              string
	ReportType      ReportType
	Status          ExportStatus // DB value (RUNNING/DONE); wire-mapped by 11-02b
	Format          ExportFormat
	Confidential    bool
	ProgressPercent *int
	Filename        *string
	SizeBytes       *int64
	FileURL         *string
	ErrCode         *string
	ErrMessage      *string
	Filters         map[string]any
	AuditLogEntryID *string
	RequesterID     string
	RequesterName   *string
	RowCount        *int
	ArtifactRef     *string
	RequestedAt     time.Time
	StartedAt       *time.Time
	CompletedAt     *time.Time
	ExpiresAt       *time.Time
}
