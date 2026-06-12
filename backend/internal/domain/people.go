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

// EmergencyContact holds the emergency-contact fields stored on an employee.
// JSON tags mirror BankAccount's rationale: when embedded as `any` in the change
// request diff (changeRequestFieldDiffResp.New / .Old) the serialized keys must
// match what the FE formatDiffValue() expects (snake_case). Editing emergency
// contact is approval-tier (routed via a change request).
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
	// (empty strings = not set). Edited via an approval-tier change request.
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

// --- Change Requests (EP-5 HR approval queue) ---

// ChangeRequestChanges holds the whitelisted fields that can be proposed via
// a change request: phone, emergency_contact, bank_account. All are optional.
// (address moved to instant-tier self apply; emergency_contact is the new
// approval-tier field — 2026-06-11 redesign.)
type ChangeRequestChanges struct {
	Phone            *string           `json:"phone,omitempty"`
	EmergencyContact *EmergencyContact `json:"emergency_contact,omitempty"`
	BankAccount      *BankAccount      `json:"bank_account,omitempty"`
}

// FieldResolution records how one field of a change request was resolved during
// the shift-leader bank-split flow: who applied it (or escalated it) and when.
// Stored per field in change_requests.field_resolutions (jsonb).
type FieldResolution struct {
	Status     string     `json:"status"`                // "applied" | "escalated_to_hr" | "rejected"
	ResolvedBy string     `json:"resolved_by,omitempty"` // SWP-EMP-<N> of the resolver
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// ChangeRequest is the domain entity for a change_requests row.
// Status is the DB lowercase value ("pending"|"approved"|"rejected"|"partially_approved")
// — uppercased only at DTO boundary.
// RequestType is stored uppercase as a DB CHECK: PHONE|EMERGENCY_CONTACT|BANK_ACCOUNT|MULTIPLE.
type ChangeRequest struct {
	ID              string
	EmployeeID      string
	Status          string // "pending" | "approved" | "rejected" | "partially_approved" (DB lowercase)
	SubmittedAt     time.Time
	ResolvedAt      *time.Time
	ResolvedBy      *string // SWP-EMP-<N> of resolving HR user
	RejectionReason *string
	Note            *string
	Changes         ChangeRequestChanges // deserialized from jsonb
	RequestType     string               // PHONE | EMERGENCY_CONTACT | BANK_ACCOUNT | MULTIPLE
	// FieldResolutions records per-field resolution in the SL bank-split flow,
	// keyed by field name ("phone","emergency_contact","bank_account").
	// Empty until a shift leader partially applies a mixed request.
	FieldResolutions map[string]FieldResolution
	// BankPending is the denormalized flag (DB column) backing the HR
	// bank-escalation queue: true when an SL applied the non-bank fields and a
	// bank change still awaits HR (status = partially_approved).
	BankPending bool
}

// ChangeRequestDetail is a value object combining a ChangeRequest with the employee
// summary and a per-field old→new diff (for the GET /change-requests/{id} detail view).
type ChangeRequestDetail struct {
	ChangeRequest
	Employee EmployeeRef
	Diff     map[string]ChangeRequestFieldDiff // keyed by field name: "phone", "emergency_contact", "bank_account"
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
