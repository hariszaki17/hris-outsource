// Package domain — service line and position types for E2 F2.4 (ORG-03).
// Separate file to keep domain/org.go (client companies + sites) isolated.
package domain

import "time"

// ServiceLine is the domain entity for a service line (F2.4 / ORG-03).
// PositionCount is derived from CountActivePositionsForLine at the repository layer.
type ServiceLine struct {
	ID            string
	Name          string
	Status        string // "active" | "inactive" (DB lowercase)
	PositionCount int    // derived from CountActivePositionsForLine
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Position is the domain entity for a position scoped to one service line (F2.4 / ORG-03).
// SP-3: (service_line_id, name) is unique.
type Position struct {
	ID            string
	ServiceLineID string
	Name          string
	Alias         string // optional English label
	Status        string // "active" | "inactive" (DB lowercase)
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ServiceLineFilter holds the decoded query parameters for GET /service-lines.
// Cursor fields support pagination (consistent with other filters).
type ServiceLineFilter struct {
	Status          *string
	CursorCreatedAt *time.Time
	CursorID        *string
	Limit           int
}

// PositionFilter holds the decoded query parameters for GET /service-lines/{id}/positions.
type PositionFilter struct {
	Status          *string
	CursorCreatedAt *time.Time
	CursorID        *string
	Limit           int
}
