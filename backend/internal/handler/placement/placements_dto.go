// Package placement (handler) — request/response DTOs + snake_case mappers for
// the E3 placement endpoints. Response field names match docs/api/E3-placement/
// openapi.yaml AND the field set the built e3-placement FE components destructure.
// lifecycle_status is derived at this boundary (Asia/Jakarta) — mirrors Phase-4
// toAgreementResponse EXPIRING derivation.
package placement

import (
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- request DTOs ---

type placementWriteRequest struct {
	EmployeeID      string  `json:"employee_id"`
	AgreementID     *string `json:"agreement_id,omitempty"` // optional: nil/"" → pending agreement
	ClientCompanyID string  `json:"client_company_id"`
	SiteID          string  `json:"site_id"`
	Position        string  `json:"position"` // free-text position label
	StartDate       string  `json:"start_date"`
	EndDate         *string `json:"end_date"`
	BackdateReason  *string `json:"backdate_reason"`
	Notes           *string `json:"notes"`
}

type placementPatchRequest struct {
	Position *string `json:"position"` // free-text position label
	EndDate  *string `json:"end_date"`
	Notes    *string `json:"notes"`
	// Read-only fields — rejected if present.
	EmployeeID      *string `json:"employee_id"`
	AgreementID     *string `json:"agreement_id"`
	ClientCompanyID *string `json:"client_company_id"`
	StartDate       *string `json:"start_date"`
	LifecycleStatus *string `json:"lifecycle_status"`
	PredecessorID   *string `json:"predecessor_id"`
	SuccessorID     *string `json:"successor_id"`
}

// setAgreementRequest is the body of POST /placements/{id}/agreement (backfill).
type setAgreementRequest struct {
	AgreementID string `json:"agreement_id"`
}

type transferRequest struct {
	NewClientCompanyID string  `json:"new_client_company_id"`
	NewPosition        string  `json:"new_position"` // free-text destination position label
	NewStartDate       string  `json:"new_start_date"`
	NewEndDate         *string `json:"new_end_date"`
	NewAgreementID     *string `json:"new_agreement_id"`
	TransferReason     string  `json:"transfer_reason"`
	TransferNote       *string `json:"transfer_note"`
}

type renewRequest struct {
	NewStartDate   string  `json:"new_start_date"`
	NewEndDate     *string `json:"new_end_date"`
	NewAgreementID *string `json:"new_agreement_id"`
	NewPosition    *string `json:"new_position"` // free-text; nil/"" keeps predecessor's position
	Notes          *string `json:"notes"`
}

type endRequest struct {
	Reason        string  `json:"reason"`
	EffectiveDate string  `json:"effective_date"`
	Notes         *string `json:"notes"`
}

type resignRequest struct {
	ResignAt          string  `json:"resign_at"`
	ResignationReason string  `json:"resignation_reason"`
	Notes             *string `json:"notes"`
}

type terminateRequest struct {
	TerminationReason   string  `json:"termination_reason"`
	EffectiveDate       *string `json:"effective_date"`
	TypeCompanyNameConf string  `json:"type_company_name_confirm"`
}

// --- response DTOs ---

// placementResponse is the openapi Placement object (snake_case, nullable
// handling). lifecycle_status is derived at the boundary.
type placementResponse struct {
	ID                string   `json:"id"`
	EmployeeID        string   `json:"employee_id"`
	EmployeeName      *string  `json:"employee_name"`
	AgreementID       *string  `json:"agreement_id"`
	AgreementType     *string  `json:"agreement_type"`
	AwaitingAgreement bool     `json:"awaiting_agreement"`
	ClientCompanyID   string   `json:"client_company_id"`
	ClientCompanyName *string  `json:"client_company_name"`
	SiteID            string   `json:"site_id"`
	SiteName          *string  `json:"site_name"`
	Position          string   `json:"position"`      // free-text position label
	PositionName      string   `json:"position_name"` // denormalized alias — same value as Position
	StartDate         string   `json:"start_date"`
	EndDate           *string  `json:"end_date"`
	Notes             *string  `json:"notes"`
	LifecycleStatus   string   `json:"lifecycle_status"`
	StatusChangedAt   string   `json:"status_changed_at"`
	EndedReason       *string  `json:"ended_reason"`
	EndedAt           *string  `json:"ended_at"`
	TerminationReason *string  `json:"termination_reason"`
	ResignAt          *string  `json:"resign_at"`
	PredecessorID     *string  `json:"predecessor_id"`
	SuccessorID       *string  `json:"successor_id"`
	BackdateReason    *string  `json:"backdate_reason"`
	CreatedBy         *string  `json:"created_by"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
	Warnings          []string `json:"warnings"`
}

// placementSummaryResponse is the openapi PlacementSummary (history_chain item).
type placementSummaryResponse struct {
	ID                string  `json:"id"`
	EmployeeID        string  `json:"employee_id"`
	ClientCompanyID   string  `json:"client_company_id"`
	ClientCompanyName *string `json:"client_company_name"`
	Position          string  `json:"position"` // free-text position label
	LifecycleStatus   string  `json:"lifecycle_status"`
	StartDate         string  `json:"start_date"`
	EndDate           *string `json:"end_date"`
}

type shiftLeaderSummaryResponse struct {
	ID                string  `json:"id"`
	ClientCompanyID   string  `json:"client_company_id"`
	ClientCompanyName *string `json:"client_company_name"`
	EmployeeID        string  `json:"employee_id"`
	EmployeeName      *string `json:"employee_name"`
	AssignedAt        string  `json:"assigned_at"`
	UnassignedAt      *string `json:"unassigned_at"`
}

type placementDetailResponse struct {
	Placement          placementResponse           `json:"placement"`
	HistoryChain       []placementSummaryResponse  `json:"history_chain"`
	CurrentShiftLeader *shiftLeaderSummaryResponse `json:"current_shift_leader"`
}

type placementListResponse struct {
	Data       []placementResponse `json:"data"`
	NextCursor *string             `json:"next_cursor"`
	HasMore    bool                `json:"has_more"`
}

// placementStatsResponse is the openapi PlacementStats object backing the
// /placements dashboard stat cards (F3.1 / C2SSLA).
type placementStatsResponse struct {
	ClientCompanyCount int64 `json:"client_company_count"`
	ActiveCount        int64 `json:"active_count"`
	ExpiringCount      int64 `json:"expiring_count"`
	PendingCount       int64 `json:"pending_count"`
}

type transferResponse struct {
	Predecessor       placementResponse          `json:"predecessor"`
	Successor         placementResponse          `json:"successor"`
	VacatedAssignment *shiftLeaderAssignmentResp `json:"vacated_assignment"`
	Warnings          []string                   `json:"warnings"`
}

type renewResponse struct {
	Predecessor placementResponse `json:"predecessor"`
	Successor   placementResponse `json:"successor"`
	Warnings    []string          `json:"warnings"`
}

// --- mappers ---

const expiringDeriveDays = 30

// toPlacementResponse maps a domain.Placement to the wire format, deriving
// lifecycle_status at the boundary (Asia/Jakarta): persisted ACTIVE with
// end_date within 30d → EXPIRING; PENDING_START with start_date<=today → ACTIVE.
func toPlacementResponse(p domain.Placement, today time.Time) placementResponse {
	status := p.LifecycleStatus
	switch p.LifecycleStatus {
	case "ACTIVE":
		if p.EndDate != nil && !p.EndDate.After(today.AddDate(0, 0, expiringDeriveDays)) {
			status = "EXPIRING"
		}
	case "PENDING_START":
		if !p.StartDate.After(today) {
			status = "ACTIVE"
		}
	}

	resp := placementResponse{
		ID:                p.ID,
		EmployeeID:        p.EmployeeID,
		EmployeeName:      p.EmployeeName,
		AgreementID:       p.AgreementID,
		AgreementType:     p.AgreementType,
		AwaitingAgreement: p.AwaitingAgreement,
		ClientCompanyID:   p.ClientCompanyID,
		ClientCompanyName: p.ClientCompanyName,
		SiteID:            p.SiteID,
		SiteName:          p.SiteName,
		Position:          p.Position,
		PositionName:      p.Position, // alias — same free-text value
		StartDate:         p.StartDate.Format("2006-01-02"),
		Notes:             p.Notes,
		LifecycleStatus:   status,
		StatusChangedAt:   p.StatusChangedAt.UTC().Format(time.RFC3339),
		EndedReason:       p.EndedReason,
		TerminationReason: p.TerminationReason,
		PredecessorID:     p.PredecessorID,
		SuccessorID:       p.SuccessorID,
		BackdateReason:    p.BackdateReason,
		CreatedBy:         p.CreatedBy,
		CreatedAt:         p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         p.UpdatedAt.UTC().Format(time.RFC3339),
		Warnings:          p.Warnings,
	}
	if resp.Warnings == nil {
		resp.Warnings = []string{}
	}
	if p.EndDate != nil {
		s := p.EndDate.Format("2006-01-02")
		resp.EndDate = &s
	}
	if p.EndedAt != nil {
		s := p.EndedAt.Format("2006-01-02")
		resp.EndedAt = &s
	}
	if p.ResignAt != nil {
		s := p.ResignAt.Format("2006-01-02")
		resp.ResignAt = &s
	}
	return resp
}

func toPlacementSummaryResponse(p domain.Placement, today time.Time) placementSummaryResponse {
	full := toPlacementResponse(p, today)
	return placementSummaryResponse{
		ID:                p.ID,
		EmployeeID:        p.EmployeeID,
		ClientCompanyID:   p.ClientCompanyID,
		ClientCompanyName: p.ClientCompanyName,
		Position:          p.Position,
		LifecycleStatus:   full.LifecycleStatus,
		StartDate:         full.StartDate,
		EndDate:           full.EndDate,
	}
}

func toShiftLeaderSummaryResponse(a domain.ShiftLeaderAssignment) *shiftLeaderSummaryResponse {
	resp := &shiftLeaderSummaryResponse{
		ID:                a.ID,
		ClientCompanyID:   a.ClientCompanyID,
		ClientCompanyName: a.ClientCompanyName,
		EmployeeID:        a.EmployeeID,
		EmployeeName:      a.EmployeeName,
		AssignedAt:        a.AssignedAt.UTC().Format(time.RFC3339),
	}
	if a.UnassignedAt != nil {
		u := a.UnassignedAt.UTC().Format(time.RFC3339)
		resp.UnassignedAt = &u
	}
	return resp
}
