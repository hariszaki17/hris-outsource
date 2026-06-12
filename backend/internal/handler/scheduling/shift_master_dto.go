// Package scheduling (handler) — request/response DTOs + snake_case mappers for
// the E4 shift-master endpoints. Field names match docs/api/E4-shift-scheduling/
// openapi.yaml byte-for-shape AND the field set the built e4-scheduling FE
// components consume (break_minutes, status ACTIVE/INACTIVE, in_use_count,
// next_cursor/has_more). Nullable fields are pointers WITHOUT omitempty so the
// JSON carries explicit null (Phase-5 convention).
package scheduling

import (
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- request DTO ---

// shiftMasterWriteRequest is the POST body (create). The *Set pattern for PATCH
// lives in shiftMasterPatchRequest below.
type shiftMasterWriteRequest struct {
	Name       string  `json:"name"`
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	BreakStart *string `json:"break_start"`
	BreakEnd   *string `json:"break_end"`
	IsActive   *bool   `json:"is_active"`
}

// --- response DTO ---

// shiftMasterResponse is the openapi ShiftMaster object.
type shiftMasterResponse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	StartTime     string  `json:"start_time"`
	EndTime       string  `json:"end_time"`
	BreakStart    *string `json:"break_start"`
	BreakEnd      *string `json:"break_end"`
	BreakMinutes  *int    `json:"break_minutes"`
	CrossMidnight bool    `json:"cross_midnight"`
	IsActive      bool    `json:"is_active"`
	Status        string  `json:"status"`
	InUseCount    int64   `json:"in_use_count"`
	CreatedBy     *string `json:"created_by"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type shiftMasterListResponse struct {
	Data       []shiftMasterResponse `json:"data"`
	NextCursor *string               `json:"next_cursor"`
	HasMore    bool                  `json:"has_more"`
}

// --- mappers ---

func toShiftMasterResponse(m domain.ShiftMaster) shiftMasterResponse {
	status := "INACTIVE"
	if m.IsActive {
		status = "ACTIVE"
	}
	resp := shiftMasterResponse{
		ID:            m.ID,
		Name:          m.Name,
		StartTime:     m.StartTime,
		EndTime:       m.EndTime,
		BreakStart:    m.BreakStart,
		BreakEnd:      m.BreakEnd,
		BreakMinutes:  breakMinutes(m.BreakStart, m.BreakEnd),
		CrossMidnight: m.CrossMidnight,
		IsActive:      m.IsActive,
		Status:        status,
		InUseCount:    m.InUseCount,
		CreatedBy:     m.CreatedBy,
		CreatedAt:     m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     m.UpdatedAt.UTC().Format(time.RFC3339),
	}
	return resp
}

func toShiftMasterListResponse(rows []domain.ShiftMaster, next *string) shiftMasterListResponse {
	items := make([]shiftMasterResponse, 0, len(rows))
	for _, m := range rows {
		items = append(items, toShiftMasterResponse(m))
	}
	return shiftMasterListResponse{Data: items, NextCursor: next, HasMore: next != nil}
}

// breakMinutes derives the break length (break_end − break_start in minutes);
// nil when either bound is absent or unparseable.
func breakMinutes(start, end *string) *int {
	if start == nil || end == nil || *start == "" || *end == "" {
		return nil
	}
	sm := hhmmToMinutes(*start)
	em := hhmmToMinutes(*end)
	if sm < 0 || em < 0 || em < sm {
		return nil
	}
	v := em - sm
	return &v
}

func hhmmToMinutes(s string) int {
	if len(s) != 5 || s[2] != ':' {
		return -1
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}
