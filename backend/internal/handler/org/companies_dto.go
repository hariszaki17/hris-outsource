package org

import (
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- Request structs ---

// createCompanyRequest is the POST /client-companies body (ClientCompanyWriteRequest).
type createCompanyRequest struct {
	Name        *string `json:"name"`
	Address     *string `json:"address"`
	LeaderScope *string `json:"leader_scope"`
	NPWP        *string `json:"npwp"`
	PICName     *string `json:"pic_name"`
	Phone       *string `json:"phone"`
	Email       *string `json:"email"`
}

// updateCompanyRequest is the PATCH /client-companies/{id} body.
type updateCompanyRequest struct {
	Name        *string `json:"name"`
	Address     *string `json:"address"`
	LeaderScope *string `json:"leader_scope"`
	NPWP        *string `json:"npwp"`
	PICName     *string `json:"pic_name"`
	Phone       *string `json:"phone"`
	Email       *string `json:"email"`
}

// reasonRequest is the body for :deactivate (optional reason).
type reasonRequest struct {
	Reason *string `json:"reason"`
}

// createSiteRequest is the POST /client-companies/{id}/sites body (SiteWriteRequest).
type createSiteRequest struct {
	Name            *string  `json:"name"`
	Code            *string  `json:"code"`
	Address         *string  `json:"address"`
	Geo             *geoReq  `json:"geo"`
	GeofenceRadiusM *int     `json:"geofence_radius_m"`
	IsPrimary       *bool    `json:"is_primary"`
	PICName         *string  `json:"pic_name"`
	Phone           *string  `json:"phone"`
}

// updateSiteRequest is the PATCH /sites/{site_id} body (SiteWriteRequest).
type updateSiteRequest struct {
	Name            *string  `json:"name"`
	Code            *string  `json:"code"`
	Address         *string  `json:"address"`
	Geo             *geoReq  `json:"geo"`
	GeofenceRadiusM *int     `json:"geofence_radius_m"`
	IsPrimary       *bool    `json:"is_primary"`
	PICName         *string  `json:"pic_name"`
	Phone           *string  `json:"phone"`
}

// geoReq is the nested geo object in the site write request.
type geoReq struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// --- Response structs ---

// clientCompanyResponse is the ClientCompany object per the E2 OpenAPI spec.
// Status is uppercased (ACTIVE/INACTIVE) at this boundary (DB stores lowercase).
type clientCompanyResponse struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Address              string  `json:"address"`
	LeaderScope          string  `json:"leader_scope"`
	NPWP                 *string `json:"npwp"`
	PICName              *string `json:"pic_name"`
	Phone                *string `json:"phone"`
	Email                *string `json:"email"`
	Status               string  `json:"status"`               // ACTIVE | INACTIVE
	HasLeader            bool    `json:"has_leader"`
	SiteCount            int     `json:"site_count"`
	ActivePlacementCount int     `json:"active_placement_count"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

// geoResp is the nested geo object in the site response (null when no coordinates).
type geoResp struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// siteResponse is the Site object per the E2 OpenAPI spec.
// geofence_active is server-derived: true iff geo != nil (ST-8 / D11).
// Status is uppercased at this boundary.
type siteResponse struct {
	ID                   string   `json:"id"`
	ClientCompanyID      string   `json:"client_company_id"`
	Name                 string   `json:"name"`
	Code                 *string  `json:"code"`
	Address              string   `json:"address"`
	Geo                  *geoResp `json:"geo"`               // null when no coordinates
	GeofenceRadiusM      int      `json:"geofence_radius_m"`
	GeofenceActive       bool     `json:"geofence_active"`   // derived: geo != nil
	IsPrimary            bool     `json:"is_primary"`
	PICName              *string  `json:"pic_name"`
	Phone                *string  `json:"phone"`
	Status               string   `json:"status"` // ACTIVE | INACTIVE
	ActivePlacementCount int      `json:"active_placement_count"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
}

// --- mapper functions ---

func toClientCompanyResponse(c domain.ClientCompany) clientCompanyResponse {
	return clientCompanyResponse{
		ID:                   c.ID,
		Name:                 c.Name,
		Address:              c.Address,
		LeaderScope:          c.LeaderScope,
		NPWP:                 c.NPWP,
		PICName:              c.PICName,
		Phone:                c.Phone,
		Email:                c.Email,
		Status:               strings.ToUpper(c.Status), // "active" -> "ACTIVE"
		HasLeader:            c.HasLeader,
		SiteCount:            c.SiteCount,
		ActivePlacementCount: c.ActivePlacementCount,
		CreatedAt:            c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toSiteResponse(s domain.Site) siteResponse {
	// geofence_active is derived: true iff both GeoLat and GeoLng are non-nil (ST-8).
	var geo *geoResp
	geofenceActive := false
	if s.GeoLat != nil && s.GeoLng != nil {
		geo = &geoResp{Lat: *s.GeoLat, Lng: *s.GeoLng}
		geofenceActive = true
	}

	return siteResponse{
		ID:                   s.ID,
		ClientCompanyID:      s.ClientCompanyID,
		Name:                 s.Name,
		Code:                 s.Code,
		Address:              s.Address,
		Geo:                  geo,
		GeofenceRadiusM:      s.GeofenceRadiusM,
		GeofenceActive:       geofenceActive,
		IsPrimary:            s.IsPrimary,
		PICName:              s.PICName,
		Phone:                s.Phone,
		Status:               strings.ToUpper(s.Status), // "active" -> "ACTIVE"
		ActivePlacementCount: s.ActivePlacementCount,
		CreatedAt:            s.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            s.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// derefString safely dereferences a *string, returning "" if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
