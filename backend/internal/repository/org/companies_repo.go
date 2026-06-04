// Package org (repository) implements the org service's CompanyRepository over
// sqlc-generated queries. Mirrors the foundations repository pattern exactly:
// reads on the pool, writes via r.q.WithTx(tx), pgx.ErrNoRows → domain.ErrNotFound.
package org

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
)

// Repository is the sqlc-backed implementation of svc.CompanyRepository.
type Repository struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check: Repository satisfies the service port.
var _ svc.CompanyRepository = (*Repository)(nil)

// New returns a new Repository backed by pool.
func New(pool *db.Pool) *Repository {
	return &Repository{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- Client Companies ---

// ListClientCompanies returns a page of client companies matching the filter.
// SiteCount is populated via a secondary CountActiveSitesForCompany query per row.
// HasLeader and ActivePlacementCount are hardcoded Phase-3 stubs (TODO: Phase-5).
func (r *Repository) ListClientCompanies(ctx context.Context, f domain.CompanyFilter) ([]domain.ClientCompany, error) {
	rows, err := r.q.ListClientCompanies(ctx, sqlcgen.ListClientCompaniesParams{
		Status:          f.Status,
		Q:               f.Q,
		ServiceLine:     f.ServiceLine,
		HasLeader:       f.HasLeader,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]domain.ClientCompany, 0, len(rows))
	for _, row := range rows {
		siteCount, err := r.q.CountActiveSitesForCompany(ctx, row.ID)
		if err != nil {
			siteCount = 0
		}
		out = append(out, domain.ClientCompany{
			ID:                   row.ID,
			Name:                 row.Name,
			Address:              row.Address,
			LeaderScope:          row.LeaderScope,
			NPWP:                 row.Npwp,
			PICName:              row.PicName,
			Phone:                row.Phone,
			Email:                row.Email,
			Status:               row.Status,
			HasLeader:            false,         // TODO(Phase-5): wire to shift_leader assignment
			SiteCount:            int(siteCount),
			ActivePlacementCount: 0,             // TODO(Phase-5): wire to placements table
			CreatedAt:            row.CreatedAt,
			UpdatedAt:            row.UpdatedAt,
		})
	}
	return out, nil
}

// GetCompanyByID fetches a single client company by SWP-CMP id.
func (r *Repository) GetCompanyByID(ctx context.Context, id string) (domain.ClientCompany, error) {
	row, err := r.q.GetClientCompanyByID(ctx, id)
	if err != nil {
		return domain.ClientCompany{}, mapErr(err)
	}
	siteCount, err := r.q.CountActiveSitesForCompany(ctx, id)
	if err != nil {
		siteCount = 0
	}
	return domain.ClientCompany{
		ID:                   row.ID,
		Name:                 row.Name,
		Address:              row.Address,
		LeaderScope:          row.LeaderScope,
		NPWP:                 row.Npwp,
		PICName:              row.PicName,
		Phone:                row.Phone,
		Email:                row.Email,
		Status:               row.Status,
		HasLeader:            false,         // TODO(Phase-5): wire to shift_leader assignment
		SiteCount:            int(siteCount),
		ActivePlacementCount: 0,             // TODO(Phase-5): wire to placements table
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}, nil
}

// CreateCompany inserts a new client company in the given transaction.
func (r *Repository) CreateCompany(ctx context.Context, tx pgx.Tx, p svc.CreateCompanyParams) (domain.ClientCompany, error) {
	row, err := r.q.WithTx(tx).CreateClientCompany(ctx, sqlcgen.CreateClientCompanyParams{
		Name:        p.Name,
		Address:     p.Address,
		LeaderScope: p.LeaderScope,
		Npwp:        nullStr(p.NPWP),
		PicName:     nullStr(p.PICName),
		Phone:       nullStr(p.Phone),
		Email:       nullStr(p.Email),
	})
	if err != nil {
		return domain.ClientCompany{}, mapErr(err)
	}
	return domain.ClientCompany{
		ID:                   row.ID,
		Name:                 row.Name,
		Address:              row.Address,
		LeaderScope:          row.LeaderScope,
		NPWP:                 row.Npwp,
		PICName:              row.PicName,
		Phone:                row.Phone,
		Email:                row.Email,
		Status:               row.Status,
		HasLeader:            false, // TODO(Phase-5)
		SiteCount:            0,    // caller increments after CreateSite
		ActivePlacementCount: 0,    // TODO(Phase-5)
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}, nil
}

// UpdateCompany patches a client company's mutable fields.
func (r *Repository) UpdateCompany(ctx context.Context, tx pgx.Tx, p svc.UpdateCompanyParams) (domain.ClientCompany, error) {
	row, err := r.q.WithTx(tx).UpdateClientCompany(ctx, sqlcgen.UpdateClientCompanyParams{
		ID:          p.ID,
		Name:        p.Name,
		Address:     p.Address,
		LeaderScope: p.LeaderScope,
		Npwp:        nullStr(p.NPWP),
		PicName:     nullStr(p.PICName),
		Phone:       nullStr(p.Phone),
		Email:       nullStr(p.Email),
	})
	if err != nil {
		return domain.ClientCompany{}, mapErr(err)
	}
	siteCount, err := r.q.CountActiveSitesForCompany(ctx, p.ID)
	if err != nil {
		siteCount = 0
	}
	return domain.ClientCompany{
		ID:                   row.ID,
		Name:                 row.Name,
		Address:              row.Address,
		LeaderScope:          row.LeaderScope,
		NPWP:                 row.Npwp,
		PICName:              row.PicName,
		Phone:                row.Phone,
		Email:                row.Email,
		Status:               row.Status,
		HasLeader:            false,         // TODO(Phase-5)
		SiteCount:            int(siteCount),
		ActivePlacementCount: 0,             // TODO(Phase-5)
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}, nil
}

// SetCompanyStatus updates the status of a client company (active/inactive).
func (r *Repository) SetCompanyStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.ClientCompany, error) {
	row, err := r.q.WithTx(tx).SetClientCompanyStatus(ctx, sqlcgen.SetClientCompanyStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return domain.ClientCompany{}, mapErr(err)
	}
	siteCount, err := r.q.CountActiveSitesForCompany(ctx, id)
	if err != nil {
		siteCount = 0
	}
	return domain.ClientCompany{
		ID:                   row.ID,
		Name:                 row.Name,
		Address:              row.Address,
		LeaderScope:          row.LeaderScope,
		NPWP:                 row.Npwp,
		PICName:              row.PicName,
		Phone:                row.Phone,
		Email:                row.Email,
		Status:               row.Status,
		HasLeader:            false,         // TODO(Phase-5)
		SiteCount:            int(siteCount),
		ActivePlacementCount: 0,             // TODO(Phase-5)
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}, nil
}

// CountActiveSitesForCompany returns the number of active sites for a company.
func (r *Repository) CountActiveSitesForCompany(ctx context.Context, companyID string) (int64, error) {
	return r.q.CountActiveSitesForCompany(ctx, companyID)
}

// --- Client Sites ---

// ListSitesForCompany returns a page of sites for a company (primary first).
func (r *Repository) ListSitesForCompany(ctx context.Context, companyID string, f domain.SiteFilter) ([]domain.Site, error) {
	rows, err := r.q.ListSitesForCompany(ctx, sqlcgen.ListSitesForCompanyParams{
		ClientCompanyID: companyID,
		Status:          f.Status,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Site, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapSiteRow(row.ID, row.ClientCompanyID, row.Name, row.Code,
			row.Address, row.GeoLat, row.GeoLng, int(row.GeofenceRadiusM),
			row.IsPrimary, row.PicName, row.Phone, row.Status, row.CreatedAt, row.UpdatedAt))
	}
	return out, nil
}

// GetSiteByID fetches a single site by SWP-SITE id.
func (r *Repository) GetSiteByID(ctx context.Context, id string) (domain.Site, error) {
	row, err := r.q.GetSiteByID(ctx, id)
	if err != nil {
		return domain.Site{}, mapErr(err)
	}
	return mapSiteRow(row.ID, row.ClientCompanyID, row.Name, row.Code,
		row.Address, row.GeoLat, row.GeoLng, int(row.GeofenceRadiusM),
		row.IsPrimary, row.PicName, row.Phone, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// CreateSite inserts a new site in the given transaction.
func (r *Repository) CreateSite(ctx context.Context, tx pgx.Tx, p svc.CreateSiteParams) (domain.Site, error) {
	row, err := r.q.WithTx(tx).CreateSite(ctx, sqlcgen.CreateSiteParams{
		ClientCompanyID: p.ClientCompanyID,
		Name:            p.Name,
		Code:            nullStr(p.Code),
		Address:         p.Address,
		GeoLat:          p.GeoLat,
		GeoLng:          p.GeoLng,
		GeofenceRadiusM: int32(p.GeofenceRadiusM),
		IsPrimary:       p.IsPrimary,
		PicName:         nullStr(p.PICName),
		Phone:           nullStr(p.Phone),
	})
	if err != nil {
		return domain.Site{}, mapErr(err)
	}
	return mapSiteRow(row.ID, row.ClientCompanyID, row.Name, row.Code,
		row.Address, row.GeoLat, row.GeoLng, int(row.GeofenceRadiusM),
		row.IsPrimary, row.PicName, row.Phone, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// UpdateSite patches a site's mutable fields (not is_primary — use SetSitePrimary).
func (r *Repository) UpdateSite(ctx context.Context, tx pgx.Tx, p svc.UpdateSiteParams) (domain.Site, error) {
	row, err := r.q.WithTx(tx).UpdateSite(ctx, sqlcgen.UpdateSiteParams{
		ID:              p.ID,
		Name:            p.Name,
		Code:            nullStr(p.Code),
		Address:         p.Address,
		GeoLat:          p.GeoLat,
		GeoLng:          p.GeoLng,
		GeofenceRadiusM: int32(p.GeofenceRadiusM),
		PicName:         nullStr(p.PICName),
		Phone:           nullStr(p.Phone),
	})
	if err != nil {
		return domain.Site{}, mapErr(err)
	}
	return mapSiteRow(row.ID, row.ClientCompanyID, row.Name, row.Code,
		row.Address, row.GeoLat, row.GeoLng, int(row.GeofenceRadiusM),
		row.IsPrimary, row.PicName, row.Phone, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// DemoteOtherPrimaries clears is_primary on all other sites of the company.
// Must be called inside the same tx before SetSitePrimary (INV-5).
func (r *Repository) DemoteOtherPrimaries(ctx context.Context, tx pgx.Tx, companyID, exceptSiteID string) error {
	return r.q.WithTx(tx).DemoteOtherPrimaries(ctx, sqlcgen.DemoteOtherPrimariesParams{
		ClientCompanyID: companyID,
		ID:              exceptSiteID,
	})
}

// SetSitePrimary sets is_primary=true on the given site.
func (r *Repository) SetSitePrimary(ctx context.Context, tx pgx.Tx, id string) (domain.Site, error) {
	row, err := r.q.WithTx(tx).SetSitePrimary(ctx, id)
	if err != nil {
		return domain.Site{}, mapErr(err)
	}
	return mapSiteRow(row.ID, row.ClientCompanyID, row.Name, row.Code,
		row.Address, row.GeoLat, row.GeoLng, int(row.GeofenceRadiusM),
		row.IsPrimary, row.PicName, row.Phone, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// SetSiteStatus sets the site status (active/inactive).
func (r *Repository) SetSiteStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.Site, error) {
	row, err := r.q.WithTx(tx).SetSiteStatus(ctx, sqlcgen.SetSiteStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return domain.Site{}, mapErr(err)
	}
	return mapSiteRow(row.ID, row.ClientCompanyID, row.Name, row.Code,
		row.Address, row.GeoLat, row.GeoLng, int(row.GeofenceRadiusM),
		row.IsPrimary, row.PicName, row.Phone, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// --- mapping helpers ---

func mapSiteRow(
	id, clientCompanyID, name string,
	code *string, address string,
	geoLat, geoLng *float64,
	geofenceRadiusM int, isPrimary bool,
	picName, phone *string, status string,
	createdAt, updatedAt time.Time,
) domain.Site {
	return domain.Site{
		ID:                   id,
		ClientCompanyID:      clientCompanyID,
		Name:                 name,
		Code:                 code,
		Address:              address,
		GeoLat:               geoLat,
		GeoLng:               geoLng,
		GeofenceRadiusM:      geofenceRadiusM,
		IsPrimary:            isPrimary,
		PICName:              picName,
		Phone:                phone,
		Status:               status,
		ActivePlacementCount: 0, // TODO(Phase-5): wire to placements table
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
