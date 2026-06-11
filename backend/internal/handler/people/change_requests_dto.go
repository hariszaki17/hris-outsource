package people

import (
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- Request structs ---

// rejectRequest is the POST /change-requests/{id}:reject body.
// reason is required, minLength 3, maxLength 500 (per OpenAPI spec lines 770-782).
type rejectRequest struct {
	Reason string `json:"reason"`
}

// createChangeRequestBody is the POST /employees/{employee_id}/change-requests
// body (ChangeRequestCreate schema). Only the approval-tier fields are accepted
// (phone, emergency_contact, bank_account); instant-tier (address/app_language/
// photo) and statutory fields are absent from the schema and ignored.
type createChangeRequestBody struct {
	Changes struct {
		Phone            *string                           `json:"phone"`
		EmergencyContact *changeRequestEmergencyContactDTO `json:"emergency_contact"`
		BankAccount      *changeRequestBankAccountResp     `json:"bank_account"`
	} `json:"changes"`
	Note *string `json:"note"`
}

// toDomainChanges converts the create body's changes into the domain value object.
func (b createChangeRequestBody) toDomainChanges() domain.ChangeRequestChanges {
	c := domain.ChangeRequestChanges{
		Phone: b.Changes.Phone,
	}
	if b.Changes.EmergencyContact != nil {
		c.EmergencyContact = &domain.EmergencyContact{
			Name:  b.Changes.EmergencyContact.Name,
			Phone: b.Changes.EmergencyContact.Phone,
		}
	}
	if b.Changes.BankAccount != nil {
		c.BankAccount = &domain.BankAccount{
			BankName:          b.Changes.BankAccount.BankName,
			AccountNumber:     b.Changes.BankAccount.AccountNumber,
			AccountHolderName: b.Changes.BankAccount.AccountHolderName,
		}
	}
	return c
}

// --- Response structs ---

// changeRequestBankAccountResp is the bank_account object in changeRequestChangesResp.
type changeRequestBankAccountResp struct {
	BankName          string `json:"bank_name"`
	AccountNumber     string `json:"account_number"`
	AccountHolderName string `json:"account_holder_name"`
}

// changeRequestEmergencyContactDTO is the emergency_contact object on the wire
// (request + response), matching the EmergencyContact schema {name, phone}.
type changeRequestEmergencyContactDTO struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// changeRequestChangesResp is the changes object in changeRequestResponse.
type changeRequestChangesResp struct {
	Phone            *string                           `json:"phone,omitempty"`
	EmergencyContact *changeRequestEmergencyContactDTO `json:"emergency_contact,omitempty"`
	BankAccount      *changeRequestBankAccountResp     `json:"bank_account,omitempty"`
}

// fieldResolutionResp is one entry in the field_resolutions map: which field a
// resolver applied or escalated. resolution is the uppercase wire enum
// (APPLIED | ESCALATED_TO_HR | PENDING).
type fieldResolutionResp struct {
	Resolution string  `json:"resolution"`
	ResolvedBy *string `json:"resolved_by"`
	ResolvedAt *string `json:"resolved_at"`
}

// changeRequestResponse is the ChangeRequest object per the E2 OpenAPI spec (lines 2893-2926).
// Status is emitted uppercase (PENDING|APPROVED|REJECTED).
type changeRequestResponse struct {
	ID               string                         `json:"id"`
	EmployeeID       string                         `json:"employee_id"`
	Status           string                         `json:"status"`       // PENDING | APPROVED | REJECTED | PARTIALLY_APPROVED
	RequestType      string                         `json:"request_type"` // PHONE | EMERGENCY_CONTACT | BANK_ACCOUNT | MULTIPLE
	Note             *string                        `json:"note"`
	Changes          changeRequestChangesResp       `json:"changes"`
	FieldResolutions map[string]fieldResolutionResp `json:"field_resolutions"`
	BankPending      bool                           `json:"bank_pending"`
	SubmittedAt      string                         `json:"submitted_at"` // RFC3339
	ResolvedAt       *string                        `json:"resolved_at"`  // RFC3339 or null
	ResolvedBy       *string                        `json:"resolved_by"`
	RejectionReason  *string                        `json:"rejection_reason"`
}

// employeeRefResp is the compact employee object in changeRequestDetailResponse.
type employeeRefResp struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
	NIP      string `json:"nip"`
}

// changeRequestFieldDiffResp is the per-field {old, new} diff entry.
type changeRequestFieldDiffResp struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// changeRequestDetailResponse is the ChangeRequestDetail object per the E2 OpenAPI spec
// (lines 2943+): includes employee{id,full_name,nip} and a diff map.
type changeRequestDetailResponse struct {
	ID               string                                `json:"id"`
	Employee         employeeRefResp                       `json:"employee"`
	Status           string                                `json:"status"`
	RequestType      string                                `json:"request_type"`
	Note             *string                               `json:"note"`
	Changes          changeRequestChangesResp              `json:"changes"`
	Diff             map[string]changeRequestFieldDiffResp `json:"diff"`
	FieldResolutions map[string]fieldResolutionResp        `json:"field_resolutions"`
	BankPending      bool                                  `json:"bank_pending"`
	SubmittedAt      string                                `json:"submitted_at"`
	ResolvedAt       *string                               `json:"resolved_at"`
	ResolvedBy       *string                               `json:"resolved_by"`
	RejectionReason  *string                               `json:"rejection_reason"`
}

// toChangeRequestResponse maps a domain.ChangeRequest to the wire-format response.
func toChangeRequestResponse(cr domain.ChangeRequest) changeRequestResponse {
	resp := changeRequestResponse{
		ID:               cr.ID,
		EmployeeID:       cr.EmployeeID,
		Status:           strings.ToUpper(cr.Status),
		RequestType:      cr.RequestType,
		Note:             cr.Note,
		Changes:          toChangesResp(cr.Changes),
		FieldResolutions: toFieldResolutionsResp(cr.FieldResolutions),
		BankPending:      cr.BankPending,
		SubmittedAt:      cr.SubmittedAt.UTC().Format(time.RFC3339),
		ResolvedBy:       cr.ResolvedBy,
		RejectionReason:  cr.RejectionReason,
	}

	if cr.ResolvedAt != nil {
		s := cr.ResolvedAt.UTC().Format(time.RFC3339)
		resp.ResolvedAt = &s
	}

	return resp
}

// toChangeRequestDetailResponse maps a domain.ChangeRequestDetail to the detail wire response.
func toChangeRequestDetailResponse(d domain.ChangeRequestDetail) changeRequestDetailResponse {
	resp := changeRequestDetailResponse{
		ID: d.ID,
		Employee: employeeRefResp{
			ID:       d.Employee.ID,
			FullName: d.Employee.FullName,
			NIP:      d.Employee.NIP,
		},
		Status:           strings.ToUpper(d.Status),
		RequestType:      d.RequestType,
		Note:             d.Note,
		Changes:          toChangesResp(d.Changes),
		Diff:             toDiffResp(d.Diff),
		FieldResolutions: toFieldResolutionsResp(d.FieldResolutions),
		BankPending:      d.BankPending,
		SubmittedAt:      d.SubmittedAt.UTC().Format(time.RFC3339),
		ResolvedBy:       d.ResolvedBy,
		RejectionReason:  d.RejectionReason,
	}

	if d.ResolvedAt != nil {
		s := d.ResolvedAt.UTC().Format(time.RFC3339)
		resp.ResolvedAt = &s
	}

	return resp
}

// toChangesResp converts domain.ChangeRequestChanges to the response shape.
func toChangesResp(c domain.ChangeRequestChanges) changeRequestChangesResp {
	resp := changeRequestChangesResp{
		Phone: c.Phone,
	}
	if c.EmergencyContact != nil {
		resp.EmergencyContact = &changeRequestEmergencyContactDTO{
			Name:  c.EmergencyContact.Name,
			Phone: c.EmergencyContact.Phone,
		}
	}
	if c.BankAccount != nil {
		resp.BankAccount = &changeRequestBankAccountResp{
			BankName:          c.BankAccount.BankName,
			AccountNumber:     c.BankAccount.AccountNumber,
			AccountHolderName: c.BankAccount.AccountHolderName,
		}
	}
	return resp
}

// toFieldResolutionsResp maps the domain per-field resolution map to the wire
// shape: the lowercase internal status becomes the uppercase FieldResolution
// enum, and resolved_at is RFC3339. Returns an empty (non-nil) map when none.
func toFieldResolutionsResp(m map[string]domain.FieldResolution) map[string]fieldResolutionResp {
	out := make(map[string]fieldResolutionResp, len(m))
	for k, v := range m {
		entry := fieldResolutionResp{Resolution: strings.ToUpper(v.Status)}
		if v.ResolvedBy != "" {
			rb := v.ResolvedBy
			entry.ResolvedBy = &rb
		}
		if v.ResolvedAt != nil {
			ra := v.ResolvedAt.UTC().Format(time.RFC3339)
			entry.ResolvedAt = &ra
		}
		out[k] = entry
	}
	return out
}

// toDiffResp converts the domain diff map to the response shape.
func toDiffResp(diff map[string]domain.ChangeRequestFieldDiff) map[string]changeRequestFieldDiffResp {
	if len(diff) == 0 {
		return map[string]changeRequestFieldDiffResp{}
	}
	out := make(map[string]changeRequestFieldDiffResp, len(diff))
	for k, v := range diff {
		out[k] = changeRequestFieldDiffResp{Old: v.Old, New: v.New}
	}
	return out
}
