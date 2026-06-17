// Package payroll (repository) — ClarificationRepo implements
// svc.ClarificationRepository over the F8.5 clarification + adjustment queries.
// Reads on the pool; the guarded answer/status transitions + the inserts via
// q.WithTx(tx). pgx.ErrNoRows on a guarded UPDATE -> found=false.
package payroll

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// ClarificationRepo is the sqlc-backed implementation of svc.ClarificationRepository.
type ClarificationRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ClarificationRepository = (*ClarificationRepo)(nil)

// NewClarificationRepo returns a ClarificationRepo backed by pool.
func NewClarificationRepo(pool *db.Pool) *ClarificationRepo {
	return &ClarificationRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func (r *ClarificationRepo) CreateClarification(ctx context.Context, tx pgx.Tx, p svc.CreateClarificationParams) (dom.ClarificationRequest, error) {
	row, err := r.q.WithTx(tx).CreateClarificationRequest(ctx, sqlcgen.CreateClarificationRequestParams{
		SourceType:       p.SourceType,
		SourceID:         p.SourceID,
		PeriodID:         p.PeriodID,
		RaisedBy:         p.RaisedBy,
		TargetEmployeeID: p.TargetEmployeeID,
		Question:         p.Question,
	})
	if err != nil {
		return dom.ClarificationRequest{}, err
	}
	return mapCreateClarification(row), nil
}

func (r *ClarificationRepo) GetClarification(ctx context.Context, id string) (dom.ClarificationRequest, error) {
	row, err := r.q.GetClarificationRequest(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return dom.ClarificationRequest{}, domain.ErrNotFound
	}
	if err != nil {
		return dom.ClarificationRequest{}, err
	}
	return mapGetClarification(row), nil
}

func (r *ClarificationRepo) GetClarificationForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.ClarificationRequest, error) {
	row, err := r.q.WithTx(tx).GetClarificationRequestForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return dom.ClarificationRequest{}, domain.ErrNotFound
	}
	if err != nil {
		return dom.ClarificationRequest{}, err
	}
	return mapGetClarificationForUpdate(row), nil
}

func (r *ClarificationRepo) Answer(ctx context.Context, tx pgx.Tx, id, answer string, answeredBy *string) (dom.ClarificationRequest, bool, error) {
	row, err := r.q.WithTx(tx).AnswerClarificationRequest(ctx, sqlcgen.AnswerClarificationRequestParams{
		ID:         id,
		Answer:     &answer,
		AnsweredBy: answeredBy,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dom.ClarificationRequest{}, false, nil
	}
	if err != nil {
		return dom.ClarificationRequest{}, false, err
	}
	return mapAnswerClarification(row), true, nil
}

func (r *ClarificationRepo) SetStatus(ctx context.Context, tx pgx.Tx, id, status string) (dom.ClarificationRequest, bool, error) {
	row, err := r.q.WithTx(tx).SetClarificationStatus(ctx, sqlcgen.SetClarificationStatusParams{
		ID:     id,
		Status: status,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dom.ClarificationRequest{}, false, nil
	}
	if err != nil {
		return dom.ClarificationRequest{}, false, err
	}
	return mapSetClarificationStatus(row), true, nil
}

func (r *ClarificationRepo) CreateAdjustment(ctx context.Context, tx pgx.Tx, p svc.CreateAdjustmentParams) (dom.PayrollAdjustment, error) {
	row, err := r.q.WithTx(tx).CreatePayrollAdjustment(ctx, sqlcgen.CreatePayrollAdjustmentParams{
		EmployeeID:  p.EmployeeID,
		SourceType:  p.SourceType,
		SourceID:    p.SourceID,
		OriginYear:  int32(p.OriginYear),
		OriginMonth: int32(p.OriginMonth),
		Note:        p.Note,
	})
	if err != nil {
		return dom.PayrollAdjustment{}, err
	}
	return mapCreateAdjustment(row), nil
}
