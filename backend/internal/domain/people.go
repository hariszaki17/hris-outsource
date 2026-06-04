// Package domain — people types for the E2 employees slice (F2.1 / EP-*).
// These dependency-free structs are shared between the people service and repository.
package domain

import "time"

// BankAccount holds the flat bank_account fields stored on an employee.
type BankAccount struct {
	BankName          string // nullable: empty string means not set
	AccountNumber     string
	AccountHolderName string
}

// Employee is the domain entity for an SWP employee (F2.1 / EP-*).
//
// HasLogin is derived (UserID != nil), never stored.
// CurrentPosition, CurrentServiceLine, CurrentClientCompany are Phase-5 stubs
// (always nil until the placements table is wired in Phase 5).
type Employee struct {
	ID                   string
	UserID               *string     // nullable — linked E1 user when provisioned (EP-3)
	FullName             string
	NIK                  string      // Indonesian KTP; unique among non-deleted (EP-2)
	NIP                  string      // SWP internal employee number (may be empty)
	JoinAt               time.Time   // date; stored as pgtype.Date in sqlc
	Gender               *string     // "MALE" | "FEMALE" | nil
	BirthDate            *time.Time  // nullable date
	BirthPlace           *string
	Phone                *string
	EmailPersonal        *string
	Address              *string
	NPWP                 *string
	BPJSKesehatan        *string
	BPJSKetenagakerjaan  *string
	BankAccount          BankAccount // flat columns; empty strings = not set
	Status               string      // "active" | "inactive" (DB lowercase)
	HasLogin             bool        // derived: UserID != nil
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
