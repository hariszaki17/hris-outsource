package people

import (
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- Request structs ---

// employeeWriteRequest is the POST /employees and PATCH /employees/{id} body.
// Matches the EmployeeWriteRequest schema in the E2 OpenAPI spec.
// provision_login and login_email are accepted but ignored in Phase 4 (EP-3 stub).
type employeeWriteRequest struct {
	FullName            *string         `json:"full_name"`
	NIK                 *string         `json:"nik"`
	NIP                 *string         `json:"nip"`
	JoinAt              *string         `json:"join_at"` // "YYYY-MM-DD"
	Gender              *string         `json:"gender"`
	BirthDate           *string         `json:"birth_date"` // "YYYY-MM-DD"
	BirthPlace          *string         `json:"birth_place"`
	Phone               *string         `json:"phone"`
	EmailPersonal       *string         `json:"email_personal"`
	Address             *string         `json:"address"`
	NPWP                *string         `json:"npwp"`
	BPJSKesehatan       *string         `json:"bpjs_kesehatan"`
	BPJSKetenagakerjaan *string         `json:"bpjs_ketenagakerjaan"`
	BankAccount         *bankAccountReq `json:"bank_account"`
	// LoginEmail is the optional secondary login identifier (D2). The primary
	// identifier is Phone (required); a login is always auto-provisioned (D1).
	LoginEmail *string `json:"login_email"`
}

// bankAccountReq is the nested bank_account object in write requests.
type bankAccountReq struct {
	BankName          *string `json:"bank_name"`
	AccountNumber     *string `json:"account_number"`
	AccountHolderName *string `json:"account_holder_name"`
}

// reasonRequest is the body for :deactivate (offboard, F2.7 OB-1). reason is
// REQUIRED and enum-validated (RESIGNED|TERMINATED|END_OF_TERM|OTHER); note is
// an optional free-text. The reason drives the traceable agreement+placement
// cascade in the service.
type reasonRequest struct {
	Reason *string `json:"reason"`
	Note   *string `json:"note"`
}

// --- Response structs ---

// bankAccountResp is the nested bank_account object in the Employee response.
type bankAccountResp struct {
	BankName          string `json:"bank_name"`
	AccountNumber     string `json:"account_number"`
	AccountHolderName string `json:"account_holder_name"`
}

// clientCompanyRef is the compact client-company reference in the Employee response.
type clientCompanyRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// emergencyContactResp is the flat emergency_contact object in the Employee response.
type emergencyContactResp struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// employeeResponse is the Employee object per the E2 OpenAPI spec.
// All JSON keys are snake_case. Status is uppercased (ACTIVE/INACTIVE) at this boundary.
// current_position (free-text label) and current_client_company come from the
// agent's current placement (E3); null/empty when unplaced.
type employeeResponse struct {
	ID                   string                `json:"id"`
	UserID               *string               `json:"user_id"`
	FullName             string                `json:"full_name"`
	NIK                  string                `json:"nik"`
	NIP                  string                `json:"nip"`
	JoinAt               string                `json:"join_at"` // "YYYY-MM-DD"
	Gender               *string               `json:"gender"`
	BirthDate            *string               `json:"birth_date"` // "YYYY-MM-DD" or omitted
	BirthPlace           *string               `json:"birth_place"`
	Phone                *string               `json:"phone"`
	EmailPersonal        *string               `json:"email_personal"`
	Address              *string               `json:"address"`
	NPWP                 *string               `json:"npwp"`
	BPJSKesehatan        *string               `json:"bpjs_kesehatan"`
	BPJSKetenagakerjaan  *string               `json:"bpjs_ketenagakerjaan"`
	BankAccount          *bankAccountResp      `json:"bank_account"`
	EmergencyContact     *emergencyContactResp `json:"emergency_contact"`
	Status               string                `json:"status"` // ACTIVE | INACTIVE
	HasLogin             bool                  `json:"has_login"`
	CurrentPosition      *string               `json:"current_position"` // free-text label; null when unplaced
	CurrentClientCompany *clientCompanyRef     `json:"current_client_company"`
	CreatedAt            string                `json:"created_at"` // RFC3339
	UpdatedAt            string                `json:"updated_at"` // RFC3339
	CreatedBy            *string               `json:"created_by"`
	// TempPassword is the one-time temporary password (EP-3 show-once). Present only
	// in the create / provision-login / regenerate responses; never on reads.
	TempPassword *string `json:"temp_password,omitempty"`
}

// toEmployeeResponse maps a domain.Employee to the wire-format response.
func toEmployeeResponse(e domain.Employee) employeeResponse {
	resp := employeeResponse{
		ID:                  e.ID,
		UserID:              e.UserID,
		FullName:            e.FullName,
		NIK:                 e.NIK,
		NIP:                 e.NIP,
		JoinAt:              e.JoinAt.Format("2006-01-02"),
		Gender:              e.Gender,
		BirthPlace:          e.BirthPlace,
		Phone:               e.Phone,
		EmailPersonal:       e.EmailPersonal,
		Address:             e.Address,
		NPWP:                e.NPWP,
		BPJSKesehatan:       e.BPJSKesehatan,
		BPJSKetenagakerjaan: e.BPJSKetenagakerjaan,
		Status:              strings.ToUpper(e.Status), // "active" → "ACTIVE"
		HasLogin:            e.HasLogin,
		CreatedAt:           e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           e.UpdatedAt.UTC().Format(time.RFC3339),
		CreatedBy:           e.CreatedBy,
	}

	// current_* — populated from the employee's non-terminal placement (list query);
	// null/empty when unplaced or for endpoints that don't resolve the placement.
	// current_position is a free-text label (no master / FK / ID).
	if e.CurrentPosition != "" {
		pos := e.CurrentPosition
		resp.CurrentPosition = &pos
	}
	if e.CurrentClientCompany != nil {
		resp.CurrentClientCompany = &clientCompanyRef{ID: e.CurrentClientCompany.ID, Name: e.CurrentClientCompany.Name}
	}

	// birth_date: omit (null) if zero.
	if e.BirthDate != nil && !e.BirthDate.IsZero() {
		s := e.BirthDate.Format("2006-01-02")
		resp.BirthDate = &s
	}

	// bank_account: always present; null fields become empty strings.
	resp.BankAccount = &bankAccountResp{
		BankName:          e.BankAccount.BankName,
		AccountNumber:     e.BankAccount.AccountNumber,
		AccountHolderName: e.BankAccount.AccountHolderName,
	}

	// emergency_contact: null when both fields are empty; otherwise populate.
	if e.EmergencyContact.Name != "" || e.EmergencyContact.Phone != "" {
		resp.EmergencyContact = &emergencyContactResp{
			Name:  e.EmergencyContact.Name,
			Phone: e.EmergencyContact.Phone,
		}
	}

	return resp
}

// --- shared helpers (local copies — not exported; no coupling to other packages) ---

// queryStringPtr returns a *string from a query param value, nil if empty.
func queryStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// queryBoolPtr returns a *bool from a query param: "true"/"false" → &v, anything
// else (incl. empty) → nil (filter not applied).
func queryBoolPtr(s string) *bool {
	switch s {
	case "true":
		v := true
		return &v
	case "false":
		v := false
		return &v
	default:
		return nil
	}
}

// parseLimit parses the limit query param, returning 0 (means "use default") on error.
func parseLimit(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return 0
	}
	return n
}

// pageCursor is the opaque cursor payload (must match the service's pageCursor).
type pageCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// derefString safely dereferences a *string, returning "" if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
