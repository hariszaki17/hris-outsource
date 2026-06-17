// Package attendance (repository) — PhotoRepo implements the photo service port
// (F5.1 / CI-10 clock-in selfies) over the photos.sql queries. The INSERT runs via
// q.WithTx(tx) so it shares the audit row's transaction; reads run on the pool.
// Mirrors AgreementRepo's attachment methods (in-DB bytea blob, SWP-FILE id minted
// inline). pgx.ErrNoRows → domain.ErrNotFound.
package attendance

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// PhotoRepo is the sqlc-backed implementation of svc.PhotoRepository.
type PhotoRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.PhotoRepository = (*PhotoRepo)(nil)

// NewPhotoRepo returns a PhotoRepo backed by pool.
func NewPhotoRepo(pool *db.Pool) *PhotoRepo {
	return &PhotoRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// CreateAttendancePhoto inserts a new selfie row (with blob) in the given transaction.
func (r *PhotoRepo) CreateAttendancePhoto(ctx context.Context, tx pgx.Tx, p svc.CreateAttendancePhotoParams) (svc.UploadedPhoto, error) {
	row, err := r.q.WithTx(tx).CreateAttendancePhoto(ctx, sqlcgen.CreateAttendancePhotoParams{
		EmployeeID: p.EmployeeID,
		Caption:    p.Caption,
		FileName:   p.FileName,
		Mime:       p.MIME,
		SizeBytes:  p.SizeBytes,
		Blob:       p.Blob,
	})
	if err != nil {
		return svc.UploadedPhoto{}, err
	}
	return svc.UploadedPhoto{
		ID:         row.ID,
		FileName:   row.FileName,
		MIME:       row.Mime,
		SizeBytes:  row.SizeBytes,
		UploadedAt: row.CreatedAt,
	}, nil
}

// GetAttendancePhotoByID returns selfie metadata + blob for the download handler.
func (r *PhotoRepo) GetAttendancePhotoByID(ctx context.Context, id string) (domain.Attachment, error) {
	row, err := r.q.GetAttendancePhotoByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Attachment{}, domain.ErrNotFound
		}
		return domain.Attachment{}, err
	}
	return domain.Attachment{
		ID:        row.ID,
		FileName:  row.FileName,
		MIME:      row.Mime,
		SizeBytes: row.SizeBytes,
		Blob:      row.Blob,
		CreatedAt: row.CreatedAt,
	}, nil
}
