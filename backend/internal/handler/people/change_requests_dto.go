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

// --- Response structs ---

// changeRequestBankAccountResp is the bank_account object in changeRequestChangesResp.
type changeRequestBankAccountResp struct {
	BankName          string `json:"bank_name"`
	AccountNumber     string `json:"account_number"`
	AccountHolderName string `json:"account_holder_name"`
}

// changeRequestChangesResp is the changes object in changeRequestResponse.
type changeRequestChangesResp struct {
	Phone       *string                        `json:"phone,omitempty"`
	Address     *string                        `json:"address,omitempty"`
	BankAccount *changeRequestBankAccountResp  `json:"bank_account,omitempty"`
}

// changeRequestResponse is the ChangeRequest object per the E2 OpenAPI spec (lines 2893-2926).
// Status is emitted uppercase (PENDING|APPROVED|REJECTED).
type changeRequestResponse struct {
	ID              string                    `json:"id"`
	EmployeeID      string                    `json:"employee_id"`
	Status          string                    `json:"status"`          // PENDING | APPROVED | REJECTED
	RequestType     string                    `json:"request_type"`    // PHONE | ADDRESS | BANK_ACCOUNT | MULTIPLE
	Note            *string                   `json:"note"`
	Changes         changeRequestChangesResp  `json:"changes"`
	SubmittedAt     string                    `json:"submitted_at"`    // RFC3339
	ResolvedAt      *string                   `json:"resolved_at"`     // RFC3339 or null
	ResolvedBy      *string                   `json:"resolved_by"`
	RejectionReason *string                   `json:"rejection_reason"`
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
	ID              string                                `json:"id"`
	Employee        employeeRefResp                       `json:"employee"`
	Status          string                                `json:"status"`
	RequestType     string                                `json:"request_type"`
	Note            *string                               `json:"note"`
	Changes         changeRequestChangesResp              `json:"changes"`
	Diff            map[string]changeRequestFieldDiffResp `json:"diff"`
	SubmittedAt     string                                `json:"submitted_at"`
	ResolvedAt      *string                               `json:"resolved_at"`
	ResolvedBy      *string                               `json:"resolved_by"`
	RejectionReason *string                               `json:"rejection_reason"`
}

// toChangeRequestResponse maps a domain.ChangeRequest to the wire-format response.
func toChangeRequestResponse(cr domain.ChangeRequest) changeRequestResponse {
	resp := changeRequestResponse{
		ID:              cr.ID,
		EmployeeID:      cr.EmployeeID,
		Status:          strings.ToUpper(cr.Status),
		RequestType:     cr.RequestType,
		Note:            cr.Note,
		Changes:         toChangesResp(cr.Changes),
		SubmittedAt:     cr.SubmittedAt.UTC().Format(time.RFC3339),
		ResolvedBy:      cr.ResolvedBy,
		RejectionReason: cr.RejectionReason,
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
		Status:          strings.ToUpper(d.Status),
		RequestType:     d.RequestType,
		Note:            d.Note,
		Changes:         toChangesResp(d.Changes),
		Diff:            toDiffResp(d.Diff),
		SubmittedAt:     d.SubmittedAt.UTC().Format(time.RFC3339),
		ResolvedBy:      d.ResolvedBy,
		RejectionReason: d.RejectionReason,
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
		Phone:   c.Phone,
		Address: c.Address,
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
