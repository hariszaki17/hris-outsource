// Package domain — people types for the E2 employees slice (F2.1 / EP-*).
// These dependency-free structs are shared between the people service and repository.
package domain

import "time"

// BankAccount holds the flat bank_account fields stored on an employee.
// JSON tags are snake_case to match the wire contract (employee + self-profile
// responses) the FE expects.
type BankAccount struct {
	BankName          string `json:"bank_name"`
	AccountNumber     string `json:"account_number"`
	AccountHolderName string `json:"account_holder_name"`
}

// EmergencyContact holds the emergency-contact fields stored on an employee.
// JSON tags are snake_case to match the wire contract. Editing emergency contact
// is instant self-apply via PATCH /me/profile (E11, 2026-06-14).
type EmergencyContact struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// Employee is the domain entity for an SWP employee (F2.1 / EP-*).
//
// HasLogin is derived (UserID != nil), never stored.
// CurrentPosition (free-text label) and CurrentClientCompany come from the
// agent's current placement (E3 read); nil/empty until a placement exists.
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
	// EmergencyContact holds the flat emergency_contact_{name,phone} columns
	// (empty strings = not set). Instant self-edit via PATCH /me/profile (E11).
	EmergencyContact EmergencyContact
	// AppLanguage is the agent's UI language preference ("id" default | "en");
	// instant-tier self edit (PATCH /me/profile).
	AppLanguage string
	// PhotoObjectKey is the server-built key into the MinIO private bucket
	// (profile-photos/{employee_id}/{ulid}.{ext}); nil = no photo. The presigned
	// GET photo_url is derived at the DTO boundary, never stored.
	PhotoObjectKey *string
	Status         string // "active" | "inactive" (DB lowercase)
	HasLogin       bool   // derived: UserID != nil
	// current_* come from the agent's current placement (E3 read); empty/nil unplaced.
	// CurrentPosition is a free-text label (no master / FK / ID).
	CurrentPosition      string
	CurrentClientCompany *ClientCompanyRef
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CreatedBy            *string
}

// ClientCompanyRef is the compact client-company reference embedded in Employee.
type ClientCompanyRef struct {
	ID   string
	Name string
}

// EmployeeFilter holds the decoded query parameters for GET /employees.
// All fields optional; cursor fields are set when paginating past the first page.
type EmployeeFilter struct {
	Q               *string
	Status          *string
	Role            *string // filter by linked User role (E1): agent|shift_leader|hr_admin|super_admin
	Assigned        *bool   // true/false against an active shift-leader assignment
	ClientCompanyID *string // filter by current placement's client company ID
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

// E11 (2026-06-14, EPICS §8 decision A): the profile change-request approval
// queue was hard-deleted. Agent profile edits (phone / emergency_contact /
// bank_account, alongside address / app_language / photo) are now INSTANT
// self-apply via PATCH /me/profile. The ChangeRequest* domain types,
// change_requests table, and approval surface no longer exist.
