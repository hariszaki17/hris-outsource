package people

import (
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- Request structs ---

// agreementWriteRequest is the POST /agreements body.
// Matches AgreementWriteRequest in the E2 OpenAPI spec.
type agreementWriteRequest struct {
	EmployeeID   string                `json:"employee_id"`
	Type         string                `json:"type"` // PKWT | PKWTT
	AgreementNo  string                `json:"agreement_no"`
	StartDate    string                `json:"start_date"` // YYYY-MM-DD
	EndDate      *string               `json:"end_date"`   // YYYY-MM-DD, required for PKWT
	Compensation *compensationTermsReq `json:"compensation"`
}

// renewRequest is the POST /agreements/{id}:renew body.
type renewRequest struct {
	Type         string                `json:"type"`
	AgreementNo  string                `json:"agreement_no"`
	StartDate    string                `json:"start_date"`
	EndDate      *string               `json:"end_date"`
	Compensation *compensationTermsReq `json:"compensation"`
	Note         string                `json:"note"`
}

// closeRequest is the POST /agreements/{id}:close body.
type closeRequest struct {
	Reason        string `json:"reason"`         // RESIGNED|TERMINATED|END_OF_TERM|OTHER
	EffectiveDate string `json:"effective_date"` // YYYY-MM-DD
	Note          string `json:"note"`
}

// compensationTermsReq is the nested compensation object in agreement write requests.
type compensationTermsReq struct {
	BaseSalaryIDR              *float64      `json:"base_salary_idr"`
	AnnualLeaveEntitlementDays *int32        `json:"annual_leave_entitlement_days"`
	BpjsTerms                  *bpjsTermsReq `json:"bpjs_terms"`
	TaxProfile                 *string       `json:"tax_profile"`
	EffectiveDate              *string       `json:"effective_date"` // YYYY-MM-DD
}

// bpjsTermsReq is the nested bpjs_terms object in compensation.
type bpjsTermsReq struct {
	KesehatanEmployerPct       *float64 `json:"kesehatan_employer_pct"`
	KesehatanEmployeePct       *float64 `json:"kesehatan_employee_pct"`
	KetenagakerjaanEmployerPct *float64 `json:"ketenagakerjaan_employer_pct"`
	KetenagakerjaanEmployeePct *float64 `json:"ketenagakerjaan_employee_pct"`
}

// --- Response structs ---

// bpjsTermsResp is the BPJS terms object in Agreement responses.
type bpjsTermsResp struct {
	KesehatanEmployerPct       *float64 `json:"kesehatan_employer_pct"`
	KesehatanEmployeePct       *float64 `json:"kesehatan_employee_pct"`
	KetenagakerjaanEmployerPct *float64 `json:"ketenagakerjaan_employer_pct"`
	KetenagakerjaanEmployeePct *float64 `json:"ketenagakerjaan_employee_pct"`
}

// compensationTermsResp is the nested compensation object in Agreement responses.
type compensationTermsResp struct {
	BaseSalaryIDR              *float64       `json:"base_salary_idr"`
	AnnualLeaveEntitlementDays *int32         `json:"annual_leave_entitlement_days"`
	BpjsTerms                  *bpjsTermsResp `json:"bpjs_terms"`
	TaxProfile                 *string        `json:"tax_profile"`
	EffectiveDate              *string        `json:"effective_date"` // YYYY-MM-DD or null
}

// agreementResponse is the Agreement object per the E2 OpenAPI spec.
// Status is emitted uppercase (ACTIVE/SUPERSEDED/CLOSED) with the EXPIRING
// virtual-status computed at this DTO boundary:
//
//	status="active" AND type="PKWT" AND end_date within 30 days of now → "EXPIRING"
//
// Persisted status stays "active" in the DB.
type agreementResponse struct {
	ID            string                 `json:"id"`
	EmployeeID    string                 `json:"employee_id"`
	EmployeeName  string                 `json:"employee_name,omitempty"`
	Type          string                 `json:"type"`
	AgreementNo   string                 `json:"agreement_no"`
	StartDate     string                 `json:"start_date"` // YYYY-MM-DD
	EndDate       *string                `json:"end_date"`   // YYYY-MM-DD or null
	Status        string                 `json:"status"`     // ACTIVE | SUPERSEDED | CLOSED | EXPIRING
	PredecessorID *string                `json:"predecessor_id"`
	SuccessorID   *string                `json:"successor_id"`
	ClosedReason  *string                `json:"closed_reason"`
	ClosedAt      *string                `json:"closed_at"` // RFC3339 or null
	Compensation  *compensationTermsResp `json:"compensation"`
	CreatedBy     *string                `json:"created_by"`
	CreatedAt     string                 `json:"created_at"` // RFC3339
	UpdatedAt     string                 `json:"updated_at"` // RFC3339
}

// fileRefResponse matches the §15 FileRef schema:
// {id, url, name, size_bytes, mime, uploaded_at}.
// url points at the authenticated download route /api/v1/files/{id}.
type fileRefResponse struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Name       string `json:"name"`
	SizeBytes  int64  `json:"size_bytes"`
	MIME       string `json:"mime"`
	UploadedAt string `json:"uploaded_at"` // RFC3339
}

// toAgreementResponse maps a domain.Agreement to the wire-format response.
// now is used to compute the EXPIRING virtual status.
func toAgreementResponse(ag domain.Agreement, now time.Time) agreementResponse {
	// Compute displayed status.
	status := strings.ToUpper(ag.Status)
	if ag.Status == "active" && strings.ToUpper(ag.Type) == "PKWT" && ag.EndDate != nil {
		if ag.EndDate.Before(now.Add(30 * 24 * time.Hour)) {
			status = "EXPIRING"
		}
	}

	resp := agreementResponse{
		ID:            ag.ID,
		EmployeeID:    ag.EmployeeID,
		EmployeeName:  ag.EmployeeName,
		Type:          strings.ToUpper(ag.Type),
		AgreementNo:   ag.AgreementNo,
		StartDate:     ag.StartDate.Format("2006-01-02"),
		Status:        status,
		PredecessorID: ag.PredecessorID,
		SuccessorID:   ag.SuccessorID,
		ClosedReason:  ag.ClosedReason,
		CreatedBy:     ag.CreatedBy,
		CreatedAt:     ag.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     ag.UpdatedAt.UTC().Format(time.RFC3339),
	}

	if ag.EndDate != nil {
		s := ag.EndDate.Format("2006-01-02")
		resp.EndDate = &s
	}

	if ag.ClosedAt != nil {
		s := ag.ClosedAt.UTC().Format(time.RFC3339)
		resp.ClosedAt = &s
	}

	// Compensation — always present (may be zero-value).
	comp := &compensationTermsResp{
		BaseSalaryIDR:              ag.Compensation.BaseSalaryIDR,
		AnnualLeaveEntitlementDays: ag.Compensation.AnnualLeaveEntitlementDays,
		TaxProfile:                 ag.Compensation.TaxProfile,
	}
	bpjs := ag.Compensation.BpjsTerms
	comp.BpjsTerms = &bpjsTermsResp{
		KesehatanEmployerPct:       bpjs.KesehatanEmployerPct,
		KesehatanEmployeePct:       bpjs.KesehatanEmployeePct,
		KetenagakerjaanEmployerPct: bpjs.KetenagakerjaanEmployerPct,
		KetenagakerjaanEmployeePct: bpjs.KetenagakerjaanEmployeePct,
	}
	if ag.Compensation.EffectiveDate != nil {
		s := ag.Compensation.EffectiveDate.Format("2006-01-02")
		comp.EffectiveDate = &s
	}
	resp.Compensation = comp

	return resp
}

// toFileRefResponse maps a domain.Attachment (metadata only) to the §15 FileRef response.
func toFileRefResponse(att domain.Attachment) fileRefResponse {
	return fileRefResponse{
		ID:         att.ID,
		URL:        "/api/v1/files/" + att.ID,
		Name:       att.FileName,
		SizeBytes:  att.SizeBytes,
		MIME:       att.MIME,
		UploadedAt: att.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// toCompensationParams extracts service params from the compensation request.
func toCompensationParams(c *compensationTermsReq) (baseSalary *float64, annualLeave *int32, bpjsTerms domain.BpjsTerms, taxProfile *string, effDate *time.Time) {
	if c == nil {
		return nil, nil, domain.BpjsTerms{}, nil, nil
	}
	baseSalary = c.BaseSalaryIDR
	annualLeave = c.AnnualLeaveEntitlementDays
	taxProfile = c.TaxProfile
	if c.BpjsTerms != nil {
		bpjsTerms = domain.BpjsTerms{
			KesehatanEmployerPct:       c.BpjsTerms.KesehatanEmployerPct,
			KesehatanEmployeePct:       c.BpjsTerms.KesehatanEmployeePct,
			KetenagakerjaanEmployerPct: c.BpjsTerms.KetenagakerjaanEmployerPct,
			KetenagakerjaanEmployeePct: c.BpjsTerms.KetenagakerjaanEmployeePct,
		}
	}
	if c.EffectiveDate != nil && *c.EffectiveDate != "" {
		t, err := parseDate(*c.EffectiveDate)
		if err == nil {
			effDate = &t
		}
	}
	return
}
