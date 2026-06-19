// Package payroll (handler) — F8.3 Payroll Run request/response DTOs matching
// openapi shapes. Money is the 2-decimal decrypted string. Dates are RFC3339
// strings. Mirrors the existing dto.go convention.
package payroll

import (
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// --- request bodies ---

type openRunRequest struct {
	Year       int    `json:"year"`
	Month      int    `json:"month"`
	CutoffDate string `json:"cutoff_date"`
}

// --- response: PayrollRun ---

type runResponse struct {
	ID         string  `json:"id"`
	Year       int     `json:"year"`
	Month      int     `json:"month"`
	Status     string  `json:"status"`
	CutoffDate string  `json:"cutoff_date"`
	CreatedBy  string  `json:"created_by"`
	PostedBy   *string `json:"posted_by"`
	PostedAt   *string `json:"posted_at"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// --- response: AssembleResult ---

type employeeRunLineResponse struct {
	EmployeeID      string   `json:"employee_id"`
	EmployeeName    string   `json:"employee_name"`
	EmployeeType    string   `json:"employee_type"`
	BaseSalaryIDR   *float64 `json:"base_salary_idr"`
	WorkingDays     *int     `json:"working_days"`
	GrossEarnings   string   `json:"gross_earnings"`
	GrossDeductions string   `json:"gross_deductions"`
	TakeHomePay     string   `json:"take_home_pay"`
	AdjustmentIDs   []string `json:"adjustment_ids"`
}

type assembleRunResponse struct {
	Run              runResponse               `json:"run"`
	Lines            []employeeRunLineResponse `json:"lines"`
	EligibleCount    int                       `json:"eligible_count"`
	WithAdjustments  int                       `json:"with_adjustments"`
}

// --- response: PostResult ---

type postRunResponse struct {
	Run                runResponse `json:"run"`
	PayslipCount       int         `json:"payslip_count"`
	AdjustmentsApplied int         `json:"adjustments_applied"`
}

// --- response: Run Payslip ---

type runPayslipResponse struct {
	ID              string  `json:"id"`
	EmployeeID      string  `json:"employee_id"`
	EmployeeName    *string `json:"employee_name,omitempty"`
	Year            int     `json:"year"`
	Month           int     `json:"month"`
	WorkingDays     *int    `json:"working_days"`
	GrossEarnings   *string `json:"gross_earnings"`
	GrossDeductions *string `json:"gross_deductions"`
	TakeHomePay     *string `json:"take_home_pay"`
	PaymentStatus   string  `json:"payment_status"`
	IsPosted        bool    `json:"is_posted"`
}

// --- post body for posting lines ---

type postRunBody struct {
	Lines []postRunLineBody `json:"lines"`
}

type postRunLineBody struct {
	EmployeeID      string   `json:"employee_id"`
	EmployeeName    string   `json:"employee_name"`
	EmployeeType    string   `json:"employee_type"`
	BaseSalaryIDR   *float64 `json:"base_salary_idr"`
	WorkingDays     *int     `json:"working_days"`
	GrossEarnings   string   `json:"gross_earnings"`
	GrossDeductions string   `json:"gross_deductions"`
	TakeHomePay     string   `json:"take_home_pay"`
	AdjustmentIDs   []string `json:"adjustment_ids"`
}

// --- mappers ---

func toRunResponse(r dom.PayrollRun) runResponse {
	resp := runResponse{
		ID:         r.ID,
		Year:       r.Year,
		Month:      r.Month,
		Status:     string(r.Status),
		CutoffDate: r.CutoffDate.Format("2006-01-02"),
		CreatedBy:  r.CreatedBy,
		CreatedAt:  rfc3339(r.CreatedAt),
		UpdatedAt:  rfc3339(r.UpdatedAt),
	}
	if r.PostedBy != nil {
		resp.PostedBy = r.PostedBy
	}
	if r.PostedAt != nil {
		s := r.PostedAt.UTC().Format(time.RFC3339)
		resp.PostedAt = &s
	}
	return resp
}

func toEmployeeRunLineResponse(l dom.EmployeeRunLine) employeeRunLineResponse {
	adj := l.AdjustmentIDs
	if adj == nil {
		adj = []string{}
	}
	return employeeRunLineResponse{
		EmployeeID:      l.EmployeeID,
		EmployeeName:    l.EmployeeName,
		EmployeeType:    l.EmployeeType,
		BaseSalaryIDR:   l.BaseSalaryIDR,
		WorkingDays:     l.WorkingDays,
		GrossEarnings:   l.GrossEarnings,
		GrossDeductions: l.GrossDeductions,
		TakeHomePay:     l.TakeHomePay,
		AdjustmentIDs:   adj,
	}
}

func toRunPayslipResponse(v svc.RunPayslipView) runPayslipResponse {
	return runPayslipResponse{
		ID:              v.ID,
		EmployeeID:      v.EmployeeID,
		EmployeeName:    v.EmployeeName,
		Year:            v.Year,
		Month:           v.Month,
		WorkingDays:     v.WorkingDays,
		GrossEarnings:   v.GrossEarnings,
		GrossDeductions: v.GrossDeductions,
		TakeHomePay:     v.TakeHomePay,
		PaymentStatus:   v.PaymentStatus,
		IsPosted:        v.IsPosted,
	}
}
