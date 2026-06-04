// Package attendance (handler) — correction request/response DTOs + mappers.
// The Correction response matches the openapi byte-for-shape: required-nullable
// fields emit as `null`; diff[] is present only on detail/approve/reject;
// original_snapshot passes through as a map.
package attendance

import (
	"time"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
)

// --- request DTOs ---

type approveRequest struct {
	Note string `json:"note"`
}

// (reject reuses rejectRequest from attendance_dto.go)

// --- response DTOs ---

type diffRowResponse struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// correctionResponse is the openapi Correction object.
type correctionResponse struct {
	ID                       string            `json:"id"`
	AttendanceID             string            `json:"attendance_id"`
	RequesterID              string            `json:"requester_id"`
	RequesterName            *string           `json:"requester_name,omitempty"`
	CompanyID                string            `json:"company_id,omitempty"`
	CompanyName              *string           `json:"company_name,omitempty"`
	Type                     string            `json:"type"`
	ProposedCheckInAt        *string           `json:"proposed_check_in_at"`
	ProposedCheckOutAt       *string           `json:"proposed_check_out_at"`
	ProposedAttendanceCodeID *string           `json:"proposed_attendance_code_id"`
	Reason                   string            `json:"reason"`
	EvidenceFileID           *string           `json:"evidence_file_id"`
	Status                   string            `json:"status"`
	DecidedBy                *string           `json:"decided_by"`
	DecidedAt                *string           `json:"decided_at"`
	RejectReason             *string           `json:"reject_reason"`
	OriginalSnapshot         map[string]any    `json:"original_snapshot"`
	Diff                     []diffRowResponse `json:"diff,omitempty"`
	CreatedAt                string            `json:"created_at"`
	UpdatedAt                string            `json:"updated_at"`
}

// approveCorrectionResponse is the openapi approve 200 body `{ data, attendance }`.
type approveCorrectionResponse struct {
	Data       correctionResponse `json:"data"`
	Attendance attendanceResponse `json:"attendance"`
}

// --- mappers ---

func toCorrectionResponse(c att.Correction) correctionResponse {
	snap := c.OriginalSnapshot
	if snap == nil {
		snap = map[string]any{}
	}
	var diff []diffRowResponse
	if len(c.Diff) > 0 {
		diff = make([]diffRowResponse, 0, len(c.Diff))
		for _, d := range c.Diff {
			diff = append(diff, diffRowResponse{Field: d.Field, Before: d.Before, After: d.After})
		}
	}
	return correctionResponse{
		ID:                       c.ID,
		AttendanceID:             c.AttendanceID,
		RequesterID:              c.RequesterID,
		RequesterName:            c.RequesterName,
		CompanyID:                c.CompanyID,
		CompanyName:              c.CompanyName,
		Type:                     string(c.Type),
		ProposedCheckInAt:        rfc3339Ptr(c.ProposedCheckInAt),
		ProposedCheckOutAt:       rfc3339Ptr(c.ProposedCheckOutAt),
		ProposedAttendanceCodeID: c.ProposedAttendanceCodeID,
		Reason:                   c.Reason,
		EvidenceFileID:           c.EvidenceFileID,
		Status:                   string(c.Status),
		DecidedBy:                c.DecidedBy,
		DecidedAt:                rfc3339Ptr(c.DecidedAt),
		RejectReason:             c.RejectReason,
		OriginalSnapshot:         snap,
		Diff:                     diff,
		CreatedAt:                c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
