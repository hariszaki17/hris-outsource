package org

import (
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- Request structs ---

// createServiceLineRequest is the POST /service-lines body (ServiceLineWriteRequest).
type createServiceLineRequest struct {
	Name *string `json:"name"`
}

// updateServiceLineRequest is the PATCH /service-lines/{id} body (ServiceLineWriteRequest).
type updateServiceLineRequest struct {
	Name *string `json:"name"`
}

// createPositionRequest is the POST /service-lines/{id}/positions body (PositionWriteRequest).
type createPositionRequest struct {
	Name  *string `json:"name"`
	Alias *string `json:"alias"`
}

// updatePositionRequest is the PATCH /positions/{id} body (PositionWriteRequest).
type updatePositionRequest struct {
	Name  *string `json:"name"`
	Alias *string `json:"alias"`
}

// --- Response structs ---

// serviceLineResponse is the ServiceLine object per the E2 OpenAPI spec (F2.4).
// Status is uppercased (ACTIVE/INACTIVE) at this boundary (DB stores lowercase).
type serviceLineResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Status        string `json:"status"` // ACTIVE | INACTIVE
	PositionCount int    `json:"position_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// positionResponse is the Position object per the E2 OpenAPI spec (F2.4).
// Status is uppercased at this boundary.
type positionResponse struct {
	ID            string `json:"id"`
	ServiceLineID string `json:"service_line_id"`
	Name          string `json:"name"`
	Alias         string `json:"alias"`
	Status        string `json:"status"` // ACTIVE | INACTIVE
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// --- mapper functions ---

func toServiceLineResponse(sl domain.ServiceLine) serviceLineResponse {
	return serviceLineResponse{
		ID:            sl.ID,
		Name:          sl.Name,
		Status:        strings.ToUpper(sl.Status), // "active" -> "ACTIVE"
		PositionCount: sl.PositionCount,
		CreatedAt:     sl.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     sl.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toPositionResponse(p domain.Position) positionResponse {
	return positionResponse{
		ID:            p.ID,
		ServiceLineID: p.ServiceLineID,
		Name:          p.Name,
		Alias:         p.Alias,
		Status:        strings.ToUpper(p.Status), // "active" -> "ACTIVE"
		CreatedAt:     p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
