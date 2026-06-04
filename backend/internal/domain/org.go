// Package domain — org types for the E2 org slice (client companies + sites).
// These dependency-free structs are shared between the org service and repository.
package domain

import "time"

// ClientCompany is the domain entity for a client company (F2.3 / ORG-01).
// HasLeader and ActivePlacementCount are Phase-3 stubs (always false/0 until Phase 5).
// SiteCount is wired to CountActiveSitesForCompany.
type ClientCompany struct {
	ID                   string
	Name                 string
	Address              string
	LeaderScope          string  // "company" | "site"
	NPWP                 *string // optional Indonesian tax ID (unique when set, CC-2)
	PICName              *string
	Phone                *string
	Email                *string
	Status               string // "active" | "inactive" (DB lowercase)
	HasLeader            bool   // TODO(Phase-5): wire to shift_leader assignment
	SiteCount            int    // derived from CountActiveSitesForCompany
	ActivePlacementCount int    // TODO(Phase-5): wire to placements table
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Site is the domain entity for a physical placement location (F2.6 / ORG-02).
// ActivePlacementCount is a Phase-3 stub (always 0 until Phase 5).
type Site struct {
	ID                   string
	ClientCompanyID      string
	Name                 string
	Code                 *string // optional short code (unique within company when set)
	Address              string
	GeoLat               *float64 // nullable; geofence_active = GeoLat != nil && GeoLng != nil
	GeoLng               *float64
	GeofenceRadiusM      int  // valid range 25–1000 (GEOFENCE_RADIUS_INVALID)
	IsPrimary            bool // exactly one per company (INV-5)
	PICName              *string
	Phone                *string
	Status               string // "active" | "inactive"
	ActivePlacementCount int    // TODO(Phase-5): wire to placements table
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// CompanyFilter holds the decoded query parameters for GET /client-companies.
// All fields optional; cursor fields are set when paginating past the first page.
type CompanyFilter struct {
	Q               *string
	Status          *string
	ServiceLine     *string
	HasLeader       *bool
	CursorCreatedAt *time.Time
	CursorID        *string
	Limit           int
}

// SiteFilter holds the decoded query parameters for GET /client-companies/{id}/sites.
type SiteFilter struct {
	Status          *string
	CursorCreatedAt *time.Time
	CursorID        *string
	Limit           int
}
