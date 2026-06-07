// Package domain — people types for the E2 employees slice (F2.1 / EP-*).
// These dependency-free structs are shared between the people service and repository.
package domain

import "time"

// BankAccount holds the flat bank_account fields stored on an employee.
// JSON tags are required so that when BankAccount is embedded as `any` in the
// diff response (changeRequestFieldDiffResp.New / .Old), the serialized keys
// match what the FE formatDiffValue() expects (snake_case).
type BankAccount struct {
	BankName          string `json:"bank_name"`
	AccountNumber     string `json:"account_number"`
	AccountHolderName string `json:"account_holder_name"`
}

// Employee is the domain entity for an SWP employee (F2.1 / EP-*).
//
// HasLogin is derived (UserID != nil), never stored.
// CurrentPosition, CurrentServiceLine, CurrentClientCompany are Phase-5 stubs
// (always nil until the placements table is wired in Phase 5).
type Employee struct {
	ID                  string
	UserID              *string // nullable — linked E1 user when provisioned (EP-3)
	FullName            string
	NIK                 string     // Indonesian KTP; unique among non-deleted (EP-2)
	NIP                 string     // SWP internal employee number (may be empty)
	JoinAt              time.Time  // date; stored as pgtype.Date in sqlc
	Gender              *string    // "MALE" | "FEMALE" | nil
	BirthDate           *time.Time // nullable date
	BirthPlace          *string
	Phone               *string
	EmailPersonal       *string
	Address             *string
	NPWP                *string
	BPJSKesehatan       *string
	BPJSKetenagakerjaan *string
	BankAccount         BankAccount // flat columns; empty strings = not set
	Status              string      // "active" | "inactive" (DB lowercase)
	HasLogin            bool        // derived: UserID != nil
	// Phase-5 stubs — always nil until placements table is wired.
	CurrentPosition      *PositionRef
	CurrentServiceLine   *ServiceLineRef
	CurrentClientCompany *ClientCompanyRef
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CreatedBy            *string
}

// PositionRef is the compact position reference embedded in Employee (Phase-5 stub).
type PositionRef struct {
	ID   string
	Name string
}

// ServiceLineRef is the compact service-line reference embedded in Employee (Phase-5 stub).
type ServiceLineRef struct {
	ID   string
	Name string
}

// ClientCompanyRef is the compact client-company reference embedded in Employee (Phase-5 stub).
type ClientCompanyRef struct {
	ID   string
	Name string
}

// EmployeeFilter holds the decoded query parameters for GET /employees.
// All fields optional; cursor fields are set when paginating past the first page.
type EmployeeFilter struct {
	Q               *string
	Status          *string
	Limit           int
	CursorCreatedAt *time.Time
	CursorID        *string
}

// --- Employment Agreements (EA-*) ---

// BpjsTerms holds the four BPJS percentage deduction fields stored as JSONB.
type BpjsTerms struct {
	KesehatanEmployerPct       *float64 `json:"kesehatan_employer_pct"`
	KesehatanEmployeePct       *float64 `json:"kesehatan_employee_pct"`
	KetenagakerjaanEmployerPct *float64 `json:"ketenagakerjaan_employer_pct"`
	KetenagakerjaanEmployeePct *float64 `json:"ketenagakerjaan_employee_pct"`
}

// CompensationTerms groups all compensation fields stored on an employment agreement.
// Stored plaintext this milestone; encryption at rest deferred (EA-4).
type CompensationTerms struct {
	BaseSalaryIDR              *float64   // base_salary_idr numeric
	AnnualLeaveEntitlementDays *int32     // annual_leave_entitlement_days integer (statutory leave term)
	BpjsTerms                  BpjsTerms  // bpjs_terms jsonb
	TaxProfile                 *string    // PTKP code e.g. PTKP_K0
	EffectiveDate              *time.Time // comp_effective_date date
}

// Agreement is the domain entity for an employment agreement (F2.2 / EA-*).
//
// Status is the DB value ("active"|"superseded"|"closed") — uppercased only at DTO boundary.
// Type is "PKWT" or "PKWTT" (stored uppercase, DB-checked).
// EndDate is nil for PKWTT agreements.
type Agreement struct {
	ID            string
	EmployeeID    string
	Type          string // "PKWT" | "PKWTT"
	AgreementNo   string
	StartDate     time.Time
	EndDate       *time.Time // nil for PKWTT
	Status        string     // "active" | "superseded" | "closed"
	PredecessorID *string
	SuccessorID   *string
	ClosedReason  *string
	ClosedAt      *time.Time
	Compensation  CompensationTerms
	EmployeeName  string // joined from employees.full_name (list view); empty if employee row missing
	CreatedBy     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AgreementFilter holds the decoded query parameters for GET /agreements.
type AgreementFilter struct {
	EmployeeID      *string
	Status          *string
	Type            *string
	Q               *string    // free-text ILIKE over employee name / employee id / agreement no
	EndDateLTE      *time.Time // for EXPIRING virtual status pre-filter
	Limit           int
	CursorCreatedAt *time.Time
	CursorID        *string
}

// Attachment is the domain entity for an agreement_attachments row.
// Blob holds the file bytes (read from bytea); only populated by GetAttachmentByID.
type Attachment struct {
	ID          string
	AgreementID string
	Category    string
	Caption     string
	FileName    string
	MIME        string
	SizeBytes   int64
	Blob        []byte
	UploadedBy  *string
	CreatedAt   time.Time
}

// --- Change Requests (EP-5 HR approval queue) ---

// ChangeRequestChanges holds the whitelisted fields that can be proposed via
// a change request: phone, address, bank_account. All are optional.
type ChangeRequestChanges struct {
	Phone       *string      `json:"phone,omitempty"`
	Address     *string      `json:"address,omitempty"`
	BankAccount *BankAccount `json:"bank_account,omitempty"`
}

// ChangeRequest is the domain entity for a change_requests row.
// Status is the DB lowercase value ("pending"|"approved"|"rejected") — uppercased only at DTO boundary.
// RequestType is stored uppercase as a DB CHECK: PHONE|ADDRESS|BANK_ACCOUNT|MULTIPLE.
type ChangeRequest struct {
	ID              string
	EmployeeID      string
	Status          string // "pending" | "approved" | "rejected" (DB lowercase)
	SubmittedAt     time.Time
	ResolvedAt      *time.Time
	ResolvedBy      *string // SWP-EMP-<N> of resolving HR user
	RejectionReason *string
	Note            *string
	Changes         ChangeRequestChanges // deserialized from jsonb
	RequestType     string               // PHONE | ADDRESS | BANK_ACCOUNT | MULTIPLE
}

// ChangeRequestDetail is a value object combining a ChangeRequest with the employee
// summary and a per-field old→new diff (for the GET /change-requests/{id} detail view).
type ChangeRequestDetail struct {
	ChangeRequest
	Employee EmployeeRef
	Diff     map[string]ChangeRequestFieldDiff // keyed by field name: "phone", "address", "bank_account"
}

// EmployeeRef is the compact employee reference embedded in ChangeRequestDetail.
type EmployeeRef struct {
	ID       string
	FullName string
	NIP      string
}

// ChangeRequestFieldDiff holds the before and after value for one changed field.
type ChangeRequestFieldDiff struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// ChangeRequestFilter holds the decoded query parameters for GET /change-requests.
// All fields optional; cursor fields are set when paginating past the first page.
type ChangeRequestFilter struct {
	Status            *string
	EmployeeID        *string
	RequestType       *string
	Q                 *string // reserved for FTS (currently unused in query, passed through)
	Limit             int
	CursorSubmittedAt *time.Time
	CursorID          *string
}
