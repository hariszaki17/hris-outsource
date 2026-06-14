// Package reporting (repository) — ExportRepo implements svc.ExportRepository over
// the 11-01 GENERIC export queries (InsertExportJobGeneric / GetExportJobGeneric /
// CancelExportJob). The QUEUED insert runs via q.WithTx(tx) (same tx as the River
// EnqueueTx — transactional outbox). Maps the generic sqlc rows → domain.ExportJob
// (filters jsonb → map[string]any; DB status kept raw, wire-mapped at the DTO).
// The Phase-10 payroll export repo is untouched (separate queries).
package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
)

// ExportRepo is the sqlc-backed implementation of svc.ExportRepository.
type ExportRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ExportRepository = (*ExportRepo)(nil)

// NewExportRepo returns an ExportRepo backed by pool.
func NewExportRepo(pool *db.Pool) *ExportRepo {
	return &ExportRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// InsertExportJob writes the QUEUED row inside the export tx (transactional outbox).
func (r *ExportRepo) InsertExportJob(ctx context.Context, tx pgx.Tx, p svc.ExportInsert) (dom.ExportJob, error) {
	row, err := r.q.WithTx(tx).InsertExportJobGeneric(ctx, sqlcgen.InsertExportJobGenericParams{
		ReportType:      p.ReportType,
		Format:          p.Format,
		Confidential:    p.Confidential,
		Filters:         p.Filters,
		RequestedByID:   p.RequestedByID,
		RequestedByName: p.RequestedByName,
		AuditLogEntryID: p.AuditLogEntryID,
		ExpiresAt:       p.ExpiresAt,
	})
	if err != nil {
		return dom.ExportJob{}, err
	}
	return mapGenericExportJob(genericRow(row)), nil
}

// GetExportJob reads one export job (requester scope enforced in the service).
func (r *ExportRepo) GetExportJob(ctx context.Context, id string) (dom.ExportJob, error) {
	row, err := r.q.GetExportJobGeneric(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return dom.ExportJob{}, domain.ErrNotFound
	}
	if err != nil {
		return dom.ExportJob{}, err
	}
	return mapGenericExportJob(genericRow(row)), nil
}

// CancelExportJob flips a QUEUED/RUNNING job to CANCELLED; 0 rows (already terminal)
// → re-read via GetExportJobGeneric (no-op-safe).
func (r *ExportRepo) CancelExportJob(ctx context.Context, id string) (dom.ExportJob, error) {
	row, err := r.q.CancelExportJob(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		// Already terminal — return current state.
		return r.GetExportJob(ctx, id)
	}
	if err != nil {
		return dom.ExportJob{}, err
	}
	return mapGenericExportJob(genericRow(row)), nil
}

// CountRecentExports counts this requester's export_jobs created since `since`
// (per-user throttle, RATE_LIMITED_EXPORTS). Raw count — no dedicated sqlc query.
func (r *ExportRepo) CountRecentExports(ctx context.Context, requesterID string, since time.Time) (int, error) {
	var n int
	err := r.pool.Pool.QueryRow(ctx,
		`SELECT count(*) FROM export_jobs WHERE requested_by_id = $1 AND requested_at >= $2`,
		requesterID, since).Scan(&n)
	return n, err
}

// genericExportRow is the field-identical shape shared by the three generic export
// sqlc Row structs (Insert/Get/Cancel/Update all RETURN the same columns). A small
// adapter avoids four near-identical mappers.
type genericExportRow struct {
	ID              string
	ReportType      string
	Status          string
	Format          string
	Confidential    bool
	Filters         []byte
	ProgressPercent *int32
	RowCount        *int32
	ArtifactRef     *string
	ErrorMessage    *string
	AuditLogEntryID *string
	ExpiresAt       *time.Time
	RequestedByID   string
	RequestedByName *string
	RequestedAt     time.Time
	StartedAt       *time.Time
	CompletedAt     *time.Time
}

// genericRow normalizes any of the generic sqlc Row structs into genericExportRow.
func genericRow[T sqlcgen.InsertExportJobGenericRow | sqlcgen.GetExportJobGenericRow | sqlcgen.CancelExportJobRow](r T) genericExportRow {
	switch v := any(r).(type) {
	case sqlcgen.InsertExportJobGenericRow:
		return genericExportRow(v)
	case sqlcgen.GetExportJobGenericRow:
		return genericExportRow(v)
	case sqlcgen.CancelExportJobRow:
		return genericExportRow(v)
	}
	return genericExportRow{}
}

func mapGenericExportJob(row genericExportRow) dom.ExportJob {
	var filters map[string]any
	if len(row.Filters) > 0 {
		_ = json.Unmarshal(row.Filters, &filters)
	}
	if filters == nil {
		filters = map[string]any{}
	}
	return dom.ExportJob{
		ID:              row.ID,
		ReportType:      dom.ReportType(row.ReportType),
		Status:          dom.ExportStatus(row.Status),
		Format:          dom.ExportFormat(row.Format),
		Confidential:    row.Confidential,
		ProgressPercent: i32ptrToInt(row.ProgressPercent),
		RowCount:        i32ptrToInt(row.RowCount),
		ArtifactRef:     row.ArtifactRef,
		Filename:        row.ArtifactRef,
		ErrMessage:      row.ErrorMessage,
		Filters:         filters,
		AuditLogEntryID: row.AuditLogEntryID,
		RequesterID:     row.RequestedByID,
		RequesterName:   row.RequestedByName,
		RequestedAt:     row.RequestedAt,
		StartedAt:       row.StartedAt,
		CompletedAt:     row.CompletedAt,
		ExpiresAt:       row.ExpiresAt,
	}
}

func i32ptrToInt(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}
