// Package payroll (handler) — request/response DTOs + snake_case mappers matching
// docs/api/E8-payroll/openapi.yaml byte-for-shape. Required-nullable fields
// (paid_on, working_days, gross_*, take_home_pay, money line value, requested_by
// name, scope.period/year) are pointers WITHOUT omitempty → JSON null on
// DECRYPT_FAIL / absent (decision [07-02]). locked_reason / employee_name use
// omitempty (present only when set). Money is the 2-decimal decrypted string for
// FINAL, JSON null for DECRYPT_FAIL.
package payroll

import (
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
)

// --- request bodies ---

type createAuditNoteRequest struct {
	Text string `json:"text"`
}

type exportRequest struct {
	Period       *string  `json:"period"`
	Year         *int     `json:"year"`
	EmployeeIDs  []string `json:"employee_ids"`
	Format       *string  `json:"format"`
	Confidential *bool    `json:"confidential"`
}

// --- generic envelopes ---

type dataResponse[T any] struct {
	Data T `json:"data"`
}

// payslipPageResponse is the GET /payslips envelope. It mirrors httpx.PageResponse
// but adds the optional meta block (meta.code MISSING_PAYROLL_HISTORY on empty).
type payslipPageResponse struct {
	Data       []payslipResponse `json:"data"`
	NextCursor *string           `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
	Meta       *pageMeta         `json:"meta,omitempty"`
}

type pageMeta struct {
	Code string `json:"code"`
}

// --- response: Payslip ---

type sourceRefResponse struct {
	System   string `json:"system"`
	SourceID string `json:"source_id"`
}

type earningLineResponse struct {
	Name         string  `json:"name"`
	Value        *string `json:"value"` // null on decrypt-fail (no omitempty)
	ForBPJS      bool    `json:"for_bpjs"`
	LockedReason *string `json:"locked_reason,omitempty"`
}

type deductionLineResponse struct {
	Name         string  `json:"name"`
	Value        *string `json:"value"`
	ForBPJS      bool    `json:"for_bpjs"`
	LockedReason *string `json:"locked_reason,omitempty"`
}

type benefitLineResponse struct {
	Name         string  `json:"name"`
	Value        *string `json:"value"`
	LockedReason *string `json:"locked_reason,omitempty"`
}

// payslipResponse is the openapi Payslip. Breakdown arrays are pointers so they are
// OMITTED on the list shape and PRESENT (possibly []) on the detail shape.
type payslipResponse struct {
	ID              string                   `json:"id"`
	EmployeeID      string                   `json:"employee_id"`
	EmployeeName    *string                  `json:"employee_name,omitempty"`
	Year            int                      `json:"year"`
	Month           int                      `json:"month"`
	Period          string                   `json:"period"`
	PaidOn          *string                  `json:"paid_on"`        // date or null
	WorkingDays     *int                     `json:"working_days"`   // int or null
	GrossEarnings   *string                  `json:"gross_earnings"` // Money or null
	GrossDeductions *string                  `json:"gross_deductions"`
	TakeHomePay     *string                  `json:"take_home_pay"`
	Status          string                   `json:"status"`
	DecryptFail     bool                     `json:"decrypt_fail"`
	ReadOnly        bool                     `json:"read_only"`
	LockedReason    *string                  `json:"locked_reason,omitempty"`
	Earnings        *[]earningLineResponse   `json:"earnings,omitempty"`
	Deductions      *[]deductionLineResponse `json:"deductions,omitempty"`
	Benefits        *[]benefitLineResponse   `json:"benefits,omitempty"`
	Source          sourceRefResponse        `json:"source"`
	CreatedAt       string                   `json:"created_at"`
}

// --- response: PayslipAuditNote ---

type auditNoteResponse struct {
	ID         string  `json:"id"`
	PayslipID  string  `json:"payslip_id"`
	Text       string  `json:"text"`
	AuthorID   string  `json:"author_id"`
	AuthorName *string `json:"author_name"` // null when unresolved (no omitempty)
	CreatedAt  string  `json:"created_at"`
}

// --- response: PayslipExportJob ---

type requestedByResponse struct {
	ID   string  `json:"id"`
	Name *string `json:"name"` // null when unresolved (no omitempty)
}

type exportScopeResponse struct {
	Period      *string  `json:"period"` // null (no omitempty)
	Year        *int     `json:"year"`   // null (no omitempty)
	EmployeeIDs []string `json:"employee_ids"`
}

type exportJobResponse struct {
	ID           string              `json:"id"`
	Status       string              `json:"status"`
	Format       string              `json:"format"`
	Confidential bool                `json:"confidential"`
	RequestedAt  string              `json:"requested_at"`
	RequestedBy  requestedByResponse `json:"requested_by"`
	Scope        exportScopeResponse `json:"scope"`
	PollURL      string              `json:"poll_url"`
}

// --- mappers ---

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func dateStrPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02")
	return &s
}

// toPayslipSummary builds the list-row shape (no breakdown arrays).
func toPayslipSummary(p dom.Payslip) payslipResponse {
	return payslipResponse{
		ID:              p.ID,
		EmployeeID:      p.EmployeeID,
		EmployeeName:    p.EmployeeName,
		Year:            p.Year,
		Month:           p.Month,
		Period:          p.Period,
		PaidOn:          dateStrPtr(p.PaidOn),
		WorkingDays:     p.WorkingDays,
		GrossEarnings:   p.GrossEarnings,
		GrossDeductions: p.GrossDeductions,
		TakeHomePay:     p.TakeHomePay,
		Status:          string(p.Status),
		DecryptFail:     p.DecryptFail,
		ReadOnly:        p.ReadOnly,
		LockedReason:    p.LockedReason,
		Source:          sourceRefResponse{System: p.Source.System, SourceID: p.Source.SourceID},
		CreatedAt:       rfc3339(p.CreatedAt),
	}
}

// toPayslipDetail builds the full breakdown shape (earnings/deductions/benefits
// always present; [] on DECRYPT_FAIL per the openapi nulling table).
func toPayslipDetail(p dom.Payslip) payslipResponse {
	out := toPayslipSummary(p)

	earnings := make([]earningLineResponse, 0, len(p.Earnings))
	for _, e := range p.Earnings {
		earnings = append(earnings, earningLineResponse{
			Name: e.Name, Value: e.Value, ForBPJS: e.ForBPJS, LockedReason: e.LockedReason,
		})
	}
	deductions := make([]deductionLineResponse, 0, len(p.Deductions))
	for _, d := range p.Deductions {
		deductions = append(deductions, deductionLineResponse{
			Name: d.Name, Value: d.Value, ForBPJS: d.ForBPJS, LockedReason: d.LockedReason,
		})
	}
	benefits := make([]benefitLineResponse, 0, len(p.Benefits))
	for _, b := range p.Benefits {
		benefits = append(benefits, benefitLineResponse{
			Name: b.Name, Value: b.Value, LockedReason: b.LockedReason,
		})
	}

	out.Earnings = &earnings
	out.Deductions = &deductions
	out.Benefits = &benefits
	return out
}

func toAuditNote(n dom.PayslipAuditNote) auditNoteResponse {
	return auditNoteResponse{
		ID:         n.ID,
		PayslipID:  n.PayslipID,
		Text:       n.Text,
		AuthorID:   n.AuthorID,
		AuthorName: n.AuthorName,
		CreatedAt:  rfc3339(n.CreatedAt),
	}
}

func toExportJob(j dom.ExportJob) exportJobResponse {
	scope := exportScopeResponse{
		Period:      j.ScopePeriod,
		Year:        j.ScopeYear,
		EmployeeIDs: j.ScopeEmployeeIDs,
	}
	if scope.EmployeeIDs == nil {
		scope.EmployeeIDs = []string{}
	}
	pollURL := j.PollURL
	if pollURL == "" {
		pollURL = "/api/v1/exports/" + j.ID
	}
	return exportJobResponse{
		ID:           j.ID,
		Status:       string(j.Status),
		Format:       j.Format,
		Confidential: true, // server-enforced (Wave 2.8)
		RequestedAt:  rfc3339(j.RequestedAt),
		RequestedBy:  requestedByResponse{ID: j.RequestedByID, Name: j.RequestedByName},
		Scope:        scope,
		PollURL:      pollURL,
	}
}
