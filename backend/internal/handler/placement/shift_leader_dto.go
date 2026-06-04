package placement

import (
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- request DTOs ---

type shiftLeaderAssignRequest struct {
	ClientCompanyID string  `json:"client_company_id"`
	EmployeeID      string  `json:"employee_id"`
	StartDate       string  `json:"start_date"`
	Replace         bool    `json:"replace"`
	ReplaceReason   *string `json:"replace_reason"`
	Notes           *string `json:"notes"`
}

type shiftLeaderReplaceRequest struct {
	NewEmployeeID string  `json:"new_employee_id"`
	StartDate     string  `json:"start_date"`
	ReplaceReason string  `json:"replace_reason"`
	Notes         *string `json:"notes"`
}

type shiftLeaderEndRequest struct {
	Reason      *string `json:"reason"`
	EffectiveAt *string `json:"effective_at"`
}

// --- response DTOs ---

// shiftLeaderAssignmentResp is the openapi ShiftLeaderAssignment object.
type shiftLeaderAssignmentResp struct {
	ID                string  `json:"id"`
	ClientCompanyID   string  `json:"client_company_id"`
	ClientCompanyName *string `json:"client_company_name"`
	EmployeeID        string  `json:"employee_id"`
	EmployeeName      *string `json:"employee_name"`
	AssignedAt        string  `json:"assigned_at"`
	UnassignedAt      *string `json:"unassigned_at"`
	AssignedBy        *string `json:"assigned_by"`
	VacatedReason     *string `json:"vacated_reason"`
	Active            bool    `json:"active"`
	Notes             *string `json:"notes"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

type shiftLeaderCreateResponse struct {
	Assignment         shiftLeaderAssignmentResp  `json:"assignment"`
	ReplacedAssignment *shiftLeaderAssignmentResp `json:"replaced_assignment"`
}

// --- roster response DTOs ---

type rosterServiceLineCount struct {
	ServiceLineID   string `json:"service_line_id"`
	ServiceLineName string `json:"service_line_name"`
	Count           int    `json:"count"`
}

type rosterStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type companyRosterSummaryResponse struct {
	TotalActive    int                      `json:"total_active"`
	TotalScheduled int                      `json:"total_scheduled"`
	TotalExpiring  int                      `json:"total_expiring"`
	ByServiceLine  []rosterServiceLineCount `json:"by_service_line"`
	ByStatus       []rosterStatusCount      `json:"by_status"`
}

type companyRosterResponse struct {
	CompanyID          string                       `json:"company_id"`
	CompanyName        string                       `json:"company_name"`
	CurrentShiftLeader *shiftLeaderSummaryResponse  `json:"current_shift_leader"`
	Placements         []placementResponse          `json:"placements"`
	NextCursor         *string                      `json:"next_cursor"`
	HasMore            bool                         `json:"has_more"`
	Summary            companyRosterSummaryResponse `json:"summary"`
}

// --- mappers ---

func toShiftLeaderAssignmentResponse(a domain.ShiftLeaderAssignment) shiftLeaderAssignmentResp {
	resp := shiftLeaderAssignmentResp{
		ID:                a.ID,
		ClientCompanyID:   a.ClientCompanyID,
		ClientCompanyName: a.ClientCompanyName,
		EmployeeID:        a.EmployeeID,
		EmployeeName:      a.EmployeeName,
		AssignedAt:        a.AssignedAt.UTC().Format(time.RFC3339),
		AssignedBy:        a.AssignedBy,
		VacatedReason:     a.VacatedReason,
		Active:            a.Active(),
		Notes:             a.Notes,
		CreatedAt:         a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         a.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if a.UnassignedAt != nil {
		u := a.UnassignedAt.UTC().Format(time.RFC3339)
		resp.UnassignedAt = &u
	}
	return resp
}

func toRosterSummaryResponse(s domain.CompanyRosterSummary) companyRosterSummaryResponse {
	out := companyRosterSummaryResponse{
		TotalActive:    s.TotalActive,
		TotalScheduled: s.TotalScheduled,
		TotalExpiring:  s.TotalExpiring,
		ByServiceLine:  make([]rosterServiceLineCount, 0, len(s.ByServiceLine)),
		ByStatus:       make([]rosterStatusCount, 0, len(s.ByStatus)),
	}
	for _, l := range s.ByServiceLine {
		out.ByServiceLine = append(out.ByServiceLine, rosterServiceLineCount{
			ServiceLineID:   l.ServiceLineID,
			ServiceLineName: l.ServiceLineName,
			Count:           l.Count,
		})
	}
	for _, st := range s.ByStatus {
		out.ByStatus = append(out.ByStatus, rosterStatusCount{Status: st.Status, Count: st.Count})
	}
	return out
}
