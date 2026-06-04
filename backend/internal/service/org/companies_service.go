// Package org is the E2 org service: client company management (list, get, create,
// update, deactivate, reactivate) and client site management (list, create, update,
// deactivate). Business rules (CC-1..CC-5, ST-1..ST-8), geofence validation,
// auto-primary Main Site provisioning, atomic primary reassignment, and audit
// on every write. Mirrors the foundations service pattern exactly.
package org

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// CompanyRepository is the data dependency, defined by this consumer (Go idiom).
// The repository layer in internal/repository/org implements it over sqlc.
type CompanyRepository interface {
	// Client company reads run on the pool.
	ListClientCompanies(ctx context.Context, f domain.CompanyFilter) ([]domain.ClientCompany, error)
	GetCompanyByID(ctx context.Context, id string) (domain.ClientCompany, error)
	CountActiveSitesForCompany(ctx context.Context, companyID string) (int64, error)
	// Client company writes take the active transaction.
	CreateCompany(ctx context.Context, tx pgx.Tx, p CreateCompanyParams) (domain.ClientCompany, error)
	UpdateCompany(ctx context.Context, tx pgx.Tx, p UpdateCompanyParams) (domain.ClientCompany, error)
	SetCompanyStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.ClientCompany, error)
	// Site reads run on the pool.
	ListSitesForCompany(ctx context.Context, companyID string, f domain.SiteFilter) ([]domain.Site, error)
	GetSiteByID(ctx context.Context, id string) (domain.Site, error)
	// Site writes take the active transaction.
	CreateSite(ctx context.Context, tx pgx.Tx, p CreateSiteParams) (domain.Site, error)
	UpdateSite(ctx context.Context, tx pgx.Tx, p UpdateSiteParams) (domain.Site, error)
	DemoteOtherPrimaries(ctx context.Context, tx pgx.Tx, companyID, exceptSiteID string) error
	SetSitePrimary(ctx context.Context, tx pgx.Tx, id string) (domain.Site, error)
	SetSiteStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.Site, error)
}

// CreateCompanyParams carries the fields for inserting a new client company.
type CreateCompanyParams struct {
	Name        string
	Address     string
	LeaderScope string
	NPWP        string
	PICName     string
	Phone       string
	Email       string
}

// UpdateCompanyParams carries the fields for updating a client company.
type UpdateCompanyParams struct {
	ID          string
	Name        string
	Address     string
	LeaderScope string
	NPWP        string
	PICName     string
	Phone       string
	Email       string
}

// CreateSiteParams carries the fields for inserting a new site.
type CreateSiteParams struct {
	ClientCompanyID string
	Name            string
	Code            string
	Address         string
	GeoLat          *float64
	GeoLng          *float64
	GeofenceRadiusM int
	IsPrimary       bool
	PICName         string
	Phone           string
}

// UpdateSiteParams carries the fields for updating a site.
type UpdateSiteParams struct {
	ID              string
	Name            string
	Code            string
	Address         string
	GeoLat          *float64
	GeoLng          *float64
	GeofenceRadiusM int
	IsPrimary       bool
	PICName         string
	Phone           string
}

// TxRunner is a thin interface over db.TxManager (injectable for tests).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock is injectable for deterministic tests; defaults to time.Now.
type Clock func() time.Time

// Service implements the E2 org business logic.
type Service struct {
	repo CompanyRepository
	txm  TxRunner
	now  Clock
}

// NewService wires the service with its dependencies.
func NewService(repo CompanyRepository, txm TxRunner) *Service {
	return &Service{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *Service) SetClock(c Clock) { s.now = c }

// pageCursor is the opaque JSON payload encoded into the cursor string.
type pageCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// geofenceMinRadius and geofenceMaxRadius are the valid range per spec (ST-8 / GEOFENCE_RADIUS_INVALID).
const (
	geofenceMinRadius = 25
	geofenceMaxRadius = 1000
	geofenceDefault   = 100
)

// validateGeofenceRadius returns GEOFENCE_RADIUS_INVALID (400) if the radius is out of bounds.
func validateGeofenceRadius(radius int) error {
	if radius < geofenceMinRadius || radius > geofenceMaxRadius {
		return &apperr.Error{
			Code:       "GEOFENCE_RADIUS_INVALID",
			Message:    "Radius geofence harus 25–1000 meter.",
			Fields:     map[string]string{"geofence_radius_m": "Nilai di luar rentang yang diizinkan."},
			HTTPStatus: 400,
		}
	}
	return nil
}

// --- Client Companies ---

// ListClientCompanies returns a cursor-paginated page of client companies.
func (s *Service) ListClientCompanies(ctx context.Context, f domain.CompanyFilter) ([]domain.ClientCompany, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1 // fetch one extra to detect has_more

	// Lowercase the status filter before the query (DB stores lowercase).
	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListClientCompanies(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// GetClientCompany returns a single client company by id.
func (s *Service) GetClientCompany(ctx context.Context, id string) (domain.ClientCompany, error) {
	company, err := s.repo.GetCompanyByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ClientCompany{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ClientCompany{}, apperr.Internal(err)
	}
	return company, nil
}

// CreateClientCompany creates a new client company and auto-provisions a primary
// "Main Site" (CC-1c). Returns the company with site_count=1.
func (s *Service) CreateClientCompany(ctx context.Context, p CreateCompanyParams) (domain.ClientCompany, error) {
	// Default leader_scope to "company" if not specified (CC-1 default).
	if p.LeaderScope == "" {
		p.LeaderScope = "company"
	}

	var created domain.ClientCompany
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateCompany(ctx, tx, p)
		if inErr != nil {
			return inErr
		}

		if err := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "client_company",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"name":         created.Name,
				"leader_scope": created.LeaderScope,
				"status":       created.Status,
			},
		}); err != nil {
			return err
		}

		// CC-1c: auto-provision a primary "Main Site" for immediate placeability.
		_, inErr = s.repo.CreateSite(ctx, tx, CreateSiteParams{
			ClientCompanyID: created.ID,
			Name:            "Main Site",
			Address:         created.Address,
			GeofenceRadiusM: geofenceDefault,
			IsPrimary:       true,
		})
		if inErr != nil {
			return inErr
		}

		return nil
	}); err != nil {
		return domain.ClientCompany{}, mapConflict(err)
	}

	// SiteCount=1 after Main Site provisioning.
	created.SiteCount = 1
	return created, nil
}

// UpdateClientCompany patches a client company's statutory/billing fields.
func (s *Service) UpdateClientCompany(ctx context.Context, p UpdateCompanyParams) (domain.ClientCompany, error) {
	current, err := s.repo.GetCompanyByID(ctx, p.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ClientCompany{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ClientCompany{}, apperr.Internal(err)
	}

	var updated domain.ClientCompany
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdateCompany(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "client_company",
			EntityID:   p.ID,
			Before:     map[string]any{"name": current.Name, "leader_scope": current.LeaderScope},
			After:      map[string]any{"name": updated.Name, "leader_scope": updated.LeaderScope},
		})
	}); err != nil {
		return domain.ClientCompany{}, mapConflict(err)
	}

	return updated, nil
}

// DeactivateClientCompany sets a company to inactive.
// CC-5: active-placement guard — Phase 3 has no placements so active_placement_count=0,
// the guard never trips. TODO(Phase-5): return COMPANY_HAS_ACTIVE_PLACEMENTS when count>0 && !force.
func (s *Service) DeactivateClientCompany(ctx context.Context, id, reason string, force bool) (domain.ClientCompany, error) {
	current, err := s.repo.GetCompanyByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ClientCompany{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ClientCompany{}, apperr.Internal(err)
	}
	if current.Status == "inactive" {
		return domain.ClientCompany{}, apperr.Conflict("CONFLICT")
	}

	// TODO(Phase-5): check active_placement_count > 0; if !force return COMPANY_HAS_ACTIVE_PLACEMENTS.

	var updated domain.ClientCompany
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetCompanyStatus(ctx, tx, id, "inactive")
		if inErr != nil {
			return inErr
		}
		afterSnap := map[string]any{"status": "inactive"}
		if reason != "" {
			afterSnap["reason"] = reason
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("client_company.deactivate"),
			EntityType: "client_company",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status},
			After:      afterSnap,
		})
	}); err != nil {
		return domain.ClientCompany{}, apperr.Internal(err)
	}

	return updated, nil
}

// ReactivateClientCompany sets a company back to active.
func (s *Service) ReactivateClientCompany(ctx context.Context, id string) (domain.ClientCompany, error) {
	current, err := s.repo.GetCompanyByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ClientCompany{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ClientCompany{}, apperr.Internal(err)
	}
	if current.Status == "active" {
		return domain.ClientCompany{}, apperr.Conflict("CONFLICT")
	}

	var updated domain.ClientCompany
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetCompanyStatus(ctx, tx, id, "active")
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("client_company.reactivate"),
			EntityType: "client_company",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status},
			After:      map[string]any{"status": "active"},
		})
	}); err != nil {
		return domain.ClientCompany{}, apperr.Internal(err)
	}

	return updated, nil
}

// --- Sites ---

// ListSites returns a cursor-paginated page of sites for a company (primary first).
func (s *Service) ListSites(ctx context.Context, companyID string, f domain.SiteFilter) ([]domain.Site, *string, error) {
	// Verify company exists (404 if not).
	if _, err := s.repo.GetCompanyByID(ctx, companyID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil, apperr.NotFound()
		}
		return nil, nil, apperr.Internal(err)
	}

	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1

	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListSitesForCompany(ctx, companyID, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// GetSite returns a single site by id.
func (s *Service) GetSite(ctx context.Context, id string) (domain.Site, error) {
	site, err := s.repo.GetSiteByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Site{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Site{}, apperr.Internal(err)
	}
	return site, nil
}

// CreateSite creates a new site for a company.
// ST-2: unique name within company → 409 CONFLICT.
// ST-8: geofence_radius_m must be 25–1000 → 400 GEOFENCE_RADIUS_INVALID.
// INV-5: if is_primary, DemoteOtherPrimaries first in the same tx.
func (s *Service) CreateSite(ctx context.Context, companyID string, p CreateSiteParams) (domain.Site, error) {
	// Verify company exists.
	if _, err := s.repo.GetCompanyByID(ctx, companyID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Site{}, apperr.NotFound()
		}
		return domain.Site{}, apperr.Internal(err)
	}

	// Validate geofence radius (use default if zero).
	radius := p.GeofenceRadiusM
	if radius == 0 {
		radius = geofenceDefault
	}
	if err := validateGeofenceRadius(radius); err != nil {
		return domain.Site{}, err
	}
	p.GeofenceRadiusM = radius
	p.ClientCompanyID = companyID

	var created domain.Site
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// INV-5: demote existing primary before setting a new one.
		if p.IsPrimary {
			// We need the ID of the new site to exclude it from demotion.
			// Since we don't have it yet, pass an empty string — DemoteOtherPrimaries
			// excludes by ID so any existing sites get demoted, new one won't exist yet.
			if err := s.repo.DemoteOtherPrimaries(ctx, tx, companyID, ""); err != nil {
				return err
			}
		}

		var inErr error
		created, inErr = s.repo.CreateSite(ctx, tx, p)
		if inErr != nil {
			return inErr
		}

		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "client_site",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"name":              created.Name,
				"client_company_id": created.ClientCompanyID,
				"is_primary":        created.IsPrimary,
			},
		})
	}); err != nil {
		return domain.Site{}, mapConflict(err)
	}

	return created, nil
}

// UpdateSite patches a site's mutable fields.
// If is_primary=true, demotes the current primary and promotes this site atomically.
func (s *Service) UpdateSite(ctx context.Context, siteID string, p UpdateSiteParams) (domain.Site, error) {
	current, err := s.repo.GetSiteByID(ctx, siteID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Site{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Site{}, apperr.Internal(err)
	}

	// Validate geofence radius if provided (use current if zero).
	radius := p.GeofenceRadiusM
	if radius == 0 {
		radius = current.GeofenceRadiusM
	}
	if err := validateGeofenceRadius(radius); err != nil {
		return domain.Site{}, err
	}
	p.GeofenceRadiusM = radius
	p.ID = siteID

	// Carry forward fields not included in partial update.
	if p.Name == "" {
		p.Name = current.Name
	}
	if p.Address == "" {
		p.Address = current.Address
	}
	if p.GeoLat == nil {
		p.GeoLat = current.GeoLat
	}
	if p.GeoLng == nil {
		p.GeoLng = current.GeoLng
	}

	var updated domain.Site
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// INV-5: if promoting to primary, demote others first then promote.
		if p.IsPrimary && !current.IsPrimary {
			if err := s.repo.DemoteOtherPrimaries(ctx, tx, current.ClientCompanyID, siteID); err != nil {
				return err
			}
			if _, err := s.repo.SetSitePrimary(ctx, tx, siteID); err != nil {
				return err
			}
		}

		var inErr error
		updated, inErr = s.repo.UpdateSite(ctx, tx, p)
		if inErr != nil {
			return inErr
		}

		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "client_site",
			EntityID:   siteID,
			Before:     map[string]any{"name": current.Name, "is_primary": current.IsPrimary},
			After:      map[string]any{"name": updated.Name, "is_primary": updated.IsPrimary},
		})
	}); err != nil {
		return domain.Site{}, mapConflict(err)
	}

	return updated, nil
}

// DeactivateSite sets a site to inactive.
// ST-6: blocks if active placements exist (stubbed in Phase 3 — count=0 so guard never trips).
// TODO(Phase-5): add SITE_HAS_ACTIVE_PLACEMENTS and SITE_IS_LAST_ACTIVE guards.
func (s *Service) DeactivateSite(ctx context.Context, id string) (domain.Site, error) {
	current, err := s.repo.GetSiteByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Site{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Site{}, apperr.Internal(err)
	}
	if current.Status == "inactive" {
		return domain.Site{}, apperr.Conflict("CONFLICT")
	}

	// TODO(Phase-5): check active_placement_count > 0 → return SITE_HAS_ACTIVE_PLACEMENTS.
	// TODO(Phase-5): check last active site → return SITE_IS_LAST_ACTIVE.

	var updated domain.Site
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetSiteStatus(ctx, tx, id, "inactive")
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("client_site.deactivate"),
			EntityType: "client_site",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status},
			After:      map[string]any{"status": "inactive"},
		})
	}); err != nil {
		return domain.Site{}, apperr.Internal(err)
	}

	return updated, nil
}

// mapConflict translates a unique-index violation (Postgres error code 23505)
// into apperr.Conflict("CONFLICT"). Other errors are wrapped as internal.
func mapConflict(err error) error {
	if err == nil {
		return nil
	}
	// Check if the error is already an apperr.Error (e.g. GEOFENCE_RADIUS_INVALID).
	if _, ok := apperr.As(err); ok {
		return err
	}
	// Postgres unique violation code "23505".
	if isUniqueViolation(err) {
		return apperr.Conflict("CONFLICT")
	}
	return apperr.Internal(err)
}

// isUniqueViolation checks for Postgres error code 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pgx wraps PgError; check error message substring as a portable fallback.
	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "unique constraint") ||
		strings.Contains(err.Error(), "duplicate key")
}
