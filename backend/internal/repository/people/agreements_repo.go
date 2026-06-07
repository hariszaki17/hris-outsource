// Package people (repository) — AgreementRepo implements the people service's
// AgreementRepository interface over sqlc-generated queries.
// Reads on the pool; writes via q.WithTx(tx). pgx.ErrNoRows → domain.ErrNotFound.
// BPJS terms are marshalled/unmarshalled as JSON in the mapping layer.
package people

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// AgreementRepo is the sqlc-backed implementation of svc.AgreementRepository.
type AgreementRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check: AgreementRepo satisfies the service port.
var _ svc.AgreementRepository = (*AgreementRepo)(nil)

// NewAgreementRepo returns a new AgreementRepo backed by pool.
func NewAgreementRepo(pool *db.Pool) *AgreementRepo {
	return &AgreementRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// ListAgreements returns a cursor-paginated page of agreements matching the filter.
func (r *AgreementRepo) ListAgreements(ctx context.Context, f domain.AgreementFilter) ([]domain.Agreement, error) {
	var endDateLte pgtype.Date
	if f.EndDateLTE != nil {
		endDateLte = pgtype.Date{Time: *f.EndDateLTE, Valid: true}
	}

	rows, err := r.q.ListAgreements(ctx, sqlcgen.ListAgreementsParams{
		EmployeeID:      f.EmployeeID,
		Status:          f.Status,
		Type:            f.Type,
		EndDateLte:      endDateLte,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]domain.Agreement, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAgreementFromList(row))
	}
	return out, nil
}

// GetAgreementByID fetches a single agreement by SWP-AG id.
func (r *AgreementRepo) GetAgreementByID(ctx context.Context, id string) (domain.Agreement, error) {
	row, err := r.q.GetAgreementByID(ctx, id)
	if err != nil {
		return domain.Agreement{}, mapAgreementErr(err)
	}
	return mapAgreementFromGetByID(row), nil
}

// GetActiveAgreementForEmployee returns the single active agreement for an employee.
// Returns domain.ErrNotFound if no active agreement exists (EA-2 pre-check).
func (r *AgreementRepo) GetActiveAgreementForEmployee(ctx context.Context, employeeID string) (domain.Agreement, error) {
	row, err := r.q.GetActiveAgreementForEmployee(ctx, employeeID)
	if err != nil {
		return domain.Agreement{}, mapAgreementErr(err)
	}
	return mapAgreementFromActive(row), nil
}

// GetEmployeeByID is a read method to verify the employee exists before creating
// an agreement. Reuses the Repository from the employees slice via a shared sqlc.Queries.
func (r *AgreementRepo) GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error) {
	row, err := r.q.GetEmployeeByID(ctx, id)
	if err != nil {
		return domain.Employee{}, mapAgreementErr(err)
	}
	// Map using the same helpers from employees_repo.go (same package).
	return mapEmployeeFromGetByID(row), nil
}

// CreateAgreement inserts a new agreement in the given transaction.
func (r *AgreementRepo) CreateAgreement(ctx context.Context, tx pgx.Tx, p svc.CreateAgreementParams) (domain.Agreement, error) {
	bpjsJSON, err := json.Marshal(p.BpjsTerms)
	if err != nil {
		return domain.Agreement{}, err
	}

	var salary pgtype.Numeric
	if p.BaseSalaryIDR != nil {
		_ = salary.Scan(p.BaseSalaryIDR)
	}

	row, err := r.q.WithTx(tx).CreateAgreement(ctx, sqlcgen.CreateAgreementParams{
		EmployeeID:                 p.EmployeeID,
		Type:                       p.Type,
		AgreementNo:                p.AgreementNo,
		StartDate:                  dateToPgtype(p.StartDate),
		EndDate:                    ptrTimeToPgDate(p.EndDate),
		PredecessorID:              p.PredecessorID,
		BaseSalaryIdr:              salary,
		AnnualLeaveEntitlementDays: p.AnnualLeaveEntitlementDays,
		BpjsTerms:                  bpjsJSON,
		TaxProfile:                 p.TaxProfile,
		CompEffectiveDate:          ptrTimeToPgDate(p.CompEffectiveDate),
		CreatedBy:                  p.CreatedBy,
	})
	if err != nil {
		return domain.Agreement{}, mapAgreementErr(err)
	}
	return mapAgreementFromCreate(row), nil
}

// SetAgreementStatus updates the status (and optional close/supersede fields) in tx.
func (r *AgreementRepo) SetAgreementStatus(ctx context.Context, tx pgx.Tx, p svc.SetAgreementStatusParams) (domain.Agreement, error) {
	row, err := r.q.WithTx(tx).SetAgreementStatus(ctx, sqlcgen.SetAgreementStatusParams{
		ID:           p.ID,
		Status:       p.Status,
		ClosedReason: p.ClosedReason,
		ClosedAt:     p.ClosedAt,
		SuccessorID:  p.SuccessorID,
	})
	if err != nil {
		return domain.Agreement{}, mapAgreementErr(err)
	}
	return mapAgreementFromSetStatus(row), nil
}

// CreateAttachment inserts a new attachment row (with blob) in the given transaction.
func (r *AgreementRepo) CreateAttachment(ctx context.Context, tx pgx.Tx, p svc.CreateAttachmentParams) (domain.Attachment, error) {
	row, err := r.q.WithTx(tx).CreateAttachment(ctx, sqlcgen.CreateAttachmentParams{
		AgreementID: p.AgreementID,
		Category:    p.Category,
		Caption:     p.Caption,
		FileName:    p.FileName,
		Mime:        p.MIME,
		SizeBytes:   p.SizeBytes,
		Blob:        p.Blob,
		UploadedBy:  p.UploadedBy,
	})
	if err != nil {
		return domain.Attachment{}, mapAgreementErr(err)
	}
	return domain.Attachment{
		ID:          row.ID,
		AgreementID: row.AgreementID,
		Category:    row.Category,
		Caption:     row.Caption,
		FileName:    row.FileName,
		MIME:        row.Mime,
		SizeBytes:   row.SizeBytes,
		UploadedBy:  p.UploadedBy,
		CreatedAt:   row.CreatedAt,
	}, nil
}

// GetAttachmentByID returns attachment metadata + blob for the download handler.
func (r *AgreementRepo) GetAttachmentByID(ctx context.Context, id string) (domain.Attachment, error) {
	row, err := r.q.GetAttachmentByID(ctx, id)
	if err != nil {
		return domain.Attachment{}, mapAgreementErr(err)
	}
	return domain.Attachment{
		ID:        row.ID,
		FileName:  row.FileName,
		MIME:      row.Mime,
		SizeBytes: row.SizeBytes,
		Blob:      row.Blob,
	}, nil
}

// --- mapping helpers ---

func mapAgreementFromList(row sqlcgen.ListAgreementsRow) domain.Agreement {
	return domain.Agreement{
		ID:            row.ID,
		EmployeeID:    row.EmployeeID,
		Type:          row.Type,
		AgreementNo:   row.AgreementNo,
		StartDate:     pgtypeToTime(row.StartDate),
		EndDate:       pgtypeDateToPtr(row.EndDate),
		Status:        row.Status,
		PredecessorID: row.PredecessorID,
		SuccessorID:   row.SuccessorID,
		ClosedReason:  row.ClosedReason,
		ClosedAt:      row.ClosedAt,
		Compensation:  unmarshalComp(row.BaseSalaryIdr, row.AnnualLeaveEntitlementDays, row.BpjsTerms, row.TaxProfile, row.CompEffectiveDate),
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func mapAgreementFromGetByID(row sqlcgen.GetAgreementByIDRow) domain.Agreement {
	return domain.Agreement{
		ID:            row.ID,
		EmployeeID:    row.EmployeeID,
		Type:          row.Type,
		AgreementNo:   row.AgreementNo,
		StartDate:     pgtypeToTime(row.StartDate),
		EndDate:       pgtypeDateToPtr(row.EndDate),
		Status:        row.Status,
		PredecessorID: row.PredecessorID,
		SuccessorID:   row.SuccessorID,
		ClosedReason:  row.ClosedReason,
		ClosedAt:      row.ClosedAt,
		Compensation:  unmarshalComp(row.BaseSalaryIdr, row.AnnualLeaveEntitlementDays, row.BpjsTerms, row.TaxProfile, row.CompEffectiveDate),
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func mapAgreementFromActive(row sqlcgen.GetActiveAgreementForEmployeeRow) domain.Agreement {
	return domain.Agreement{
		ID:            row.ID,
		EmployeeID:    row.EmployeeID,
		Type:          row.Type,
		AgreementNo:   row.AgreementNo,
		StartDate:     pgtypeToTime(row.StartDate),
		EndDate:       pgtypeDateToPtr(row.EndDate),
		Status:        row.Status,
		PredecessorID: row.PredecessorID,
		SuccessorID:   row.SuccessorID,
		ClosedReason:  row.ClosedReason,
		ClosedAt:      row.ClosedAt,
		Compensation:  unmarshalComp(row.BaseSalaryIdr, row.AnnualLeaveEntitlementDays, row.BpjsTerms, row.TaxProfile, row.CompEffectiveDate),
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func mapAgreementFromCreate(row sqlcgen.CreateAgreementRow) domain.Agreement {
	return domain.Agreement{
		ID:            row.ID,
		EmployeeID:    row.EmployeeID,
		Type:          row.Type,
		AgreementNo:   row.AgreementNo,
		StartDate:     pgtypeToTime(row.StartDate),
		EndDate:       pgtypeDateToPtr(row.EndDate),
		Status:        row.Status,
		PredecessorID: row.PredecessorID,
		SuccessorID:   row.SuccessorID,
		ClosedReason:  row.ClosedReason,
		ClosedAt:      row.ClosedAt,
		Compensation:  unmarshalComp(row.BaseSalaryIdr, row.AnnualLeaveEntitlementDays, row.BpjsTerms, row.TaxProfile, row.CompEffectiveDate),
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func mapAgreementFromSetStatus(row sqlcgen.SetAgreementStatusRow) domain.Agreement {
	return domain.Agreement{
		ID:            row.ID,
		EmployeeID:    row.EmployeeID,
		Type:          row.Type,
		AgreementNo:   row.AgreementNo,
		StartDate:     pgtypeToTime(row.StartDate),
		EndDate:       pgtypeDateToPtr(row.EndDate),
		Status:        row.Status,
		PredecessorID: row.PredecessorID,
		SuccessorID:   row.SuccessorID,
		ClosedReason:  row.ClosedReason,
		ClosedAt:      row.ClosedAt,
		Compensation:  unmarshalComp(row.BaseSalaryIdr, row.AnnualLeaveEntitlementDays, row.BpjsTerms, row.TaxProfile, row.CompEffectiveDate),
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

// unmarshalComp converts sqlc compensation columns to domain.CompensationTerms.
// bpjsJSON is the raw JSONB bytes from Postgres.
func unmarshalComp(salary pgtype.Numeric, annualLeave *int32, bpjsJSON []byte, taxProfile *string, effDate pgtype.Date) domain.CompensationTerms {
	var comp domain.CompensationTerms

	comp.AnnualLeaveEntitlementDays = annualLeave

	// base_salary_idr: pgtype.Numeric → *float64
	if salary.Valid {
		f, _ := new(big.Float).SetInt(salary.Int).Float64()
		if salary.Exp != 0 {
			scale := new(big.Float).SetPrec(64)
			exp := salary.Exp
			if exp > 0 {
				base := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exp)), nil)
				f2, _ := new(big.Float).SetInt(base).Float64()
				f *= f2
			} else {
				base := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-exp)), nil)
				_ = scale
				f2, _ := new(big.Float).SetInt(base).Float64()
				f /= f2
			}
		}
		comp.BaseSalaryIDR = &f
	}

	// bpjs_terms: JSONB → BpjsTerms struct
	if len(bpjsJSON) > 0 {
		_ = json.Unmarshal(bpjsJSON, &comp.BpjsTerms)
	}

	comp.TaxProfile = taxProfile

	if effDate.Valid {
		t := effDate.Time
		comp.EffectiveDate = &t
	}

	return comp
}

// ptrTimeToPgDate converts a *time.Time to pgtype.Date (invalid if nil).
func ptrTimeToPgDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// mapAgreementErr maps pgx.ErrNoRows to domain.ErrNotFound.
func mapAgreementErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}
