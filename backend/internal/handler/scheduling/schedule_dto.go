// Package scheduling (handler) — schedule-entry request/response DTOs +
// snake_case mappers. ScheduleEntry response matches the openapi byte-for-shape;
// work_date is "YYYY-MM-DD" (Asia/Jakarta date). The bulkApplyResult failed item
// nests error:{code,message,details} (the FE reads failed[].error.code).
package scheduling

import (
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// --- request DTOs ---

// scheduleEntryWriteRequest is the POST /schedule body (kind=single).
type scheduleEntryWriteRequest struct {
	Kind          string  `json:"kind"`
	EmployeeID    string  `json:"employee_id"`
	ShiftMasterID *string `json:"shift_master_id"`
	Date          string  `json:"date"`
	IsDayOff      *bool   `json:"is_day_off"`
	ForceReplace  *bool   `json:"force_replace"`
}

// PATCH /schedule/{id} is decoded directly from raw JSON in the handler so that
// an explicit `shift_master_id: null` (convert to OFF) is distinguishable from
// an absent field — see UpdateScheduleEntry.

// bulkApplyRequest is the POST /schedule:bulk-apply body (kind=bulk).
type bulkApplyRequest struct {
	Kind             string   `json:"kind"`
	ShiftMasterID    string   `json:"shift_master_id"`
	StartDate        string   `json:"start_date"`
	EndDate          string   `json:"end_date"`
	EmployeeIDs      []string `json:"employee_ids"`
	WeekdaysMask     []int    `json:"weekdays_mask"`
	OverrideExisting *bool    `json:"override_existing"`
}

// --- response DTOs ---

// scheduleEntryResponse is the openapi ScheduleEntry object.
type scheduleEntryResponse struct {
	ID              string  `json:"id"`
	EmployeeID      string  `json:"employee_id"`
	EmployeeName    *string `json:"employee_name"`
	CompanyID       string  `json:"company_id"`
	CompanyName     *string `json:"company_name"`
	PlacementID     string  `json:"placement_id"`
	ServiceLineID   *string `json:"service_line_id"`
	ShiftMasterID   *string `json:"shift_master_id"`
	ShiftMasterName *string `json:"shift_master_name"`
	StartTime       *string `json:"start_time"`
	EndTime         *string `json:"end_time"`
	CrossMidnight   bool    `json:"cross_midnight"`
	WorkDate        string  `json:"work_date"`
	Status          string  `json:"status"`
	IsDayOff        bool    `json:"is_day_off"`
	ReplacedEntryID *string `json:"replaced_entry_id"`
	CreatedBy       *string `json:"created_by"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// scheduleEntryCreateResponse is the 201 body (ScheduleEntry + warnings).
type scheduleEntryCreateResponse struct {
	scheduleEntryResponse
	Warnings []warningResponse `json:"warnings"`
}

type scheduleListResponse struct {
	Data     []scheduleEntryResponse `json:"data"`
	Warnings []warningResponse       `json:"warnings"`
}

type warningResponse struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// bulkApplyResult is the openapi BulkApplyResult (shared by :check + :bulk-apply).
type bulkApplyResult struct {
	Succeeded []bulkSucceeded   `json:"succeeded"`
	Failed    []bulkFailed      `json:"failed"`
	Warnings  []warningResponse `json:"warnings"`
}

type bulkSucceeded struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employee_id"`
	Date       string `json:"date"`
	Status     string `json:"status,omitempty"`
}

type bulkFailed struct {
	EmployeeID string          `json:"employee_id"`
	Date       string          `json:"date"`
	Error      bulkFailedError `json:"error"`
}

type bulkFailedError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// --- mappers ---

// jakartaDate formats a work_date as YYYY-MM-DD (date-only; stored at midnight).
func jakartaDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func toScheduleEntryResponse(e domain.ScheduleEntry) scheduleEntryResponse {
	return scheduleEntryResponse{
		ID:              e.ID,
		EmployeeID:      e.EmployeeID,
		EmployeeName:    e.EmployeeName,
		CompanyID:       e.CompanyID,
		CompanyName:     e.CompanyName,
		PlacementID:     e.PlacementID,
		ServiceLineID:   e.ServiceLineID,
		ShiftMasterID:   e.ShiftMasterID,
		ShiftMasterName: e.ShiftMasterName,
		StartTime:       e.StartTime,
		EndTime:         e.EndTime,
		CrossMidnight:   e.CrossMidnight,
		WorkDate:        jakartaDate(e.WorkDate),
		Status:          e.Status,
		IsDayOff:        e.IsDayOff,
		ReplacedEntryID: e.ReplacedEntryID,
		CreatedBy:       e.CreatedBy,
		CreatedAt:       e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       e.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toScheduleListResponse(rows []domain.ScheduleEntry) scheduleListResponse {
	items := make([]scheduleEntryResponse, 0, len(rows))
	for _, e := range rows {
		items = append(items, toScheduleEntryResponse(e))
	}
	return scheduleListResponse{Data: items, Warnings: []warningResponse{}}
}

// toBulkApplyResult maps the service BulkResult to the openapi envelope.
func toBulkApplyResult(r svc.BulkResult) bulkApplyResult {
	out := bulkApplyResult{
		Succeeded: make([]bulkSucceeded, 0, len(r.Succeeded)),
		Failed:    make([]bulkFailed, 0, len(r.Failed)),
		Warnings:  []warningResponse{},
	}
	for _, s := range r.Succeeded {
		out.Succeeded = append(out.Succeeded, bulkSucceeded{
			ID:         s.ID,
			EmployeeID: s.EmployeeID,
			Date:       jakartaDate(s.Date),
			Status:     s.Status,
		})
	}
	for _, f := range r.Failed {
		out.Failed = append(out.Failed, bulkFailed{
			EmployeeID: f.EmployeeID,
			Date:       jakartaDate(f.Date),
			Error: bulkFailedError{
				Code:    f.Code,
				Message: f.Message,
				Details: f.Details,
			},
		})
	}
	return out
}
