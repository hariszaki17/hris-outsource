// Package payroll (repository) — PayslipRepo implements svc.PayslipRepository over
// the 10-01 sqlc payslips/audit_notes queries. Returns RAW ciphertext rows (no
// decryption in the repo); the service decrypts at the boundary. Reads on the pool;
// the note insert runs via q.WithTx(tx).
package payroll

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// PayslipRepo is the sqlc-backed implementation of svc.PayslipRepository.
type PayslipRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.PayslipRepository = (*PayslipRepo)(nil)

// New returns a PayslipRepo backed by pool.
func New(pool *db.Pool) *PayslipRepo {
	return &PayslipRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- payslip reads ---

func (r *PayslipRepo) ListPayslips(ctx context.Context, f svc.PayslipFilter) ([]svc.PayslipRow, error) {
	p := sqlcgen.ListPayslipsParams{
		EmployeeID: f.EmployeeID,
		Year:       i32ptr(f.Year),
		Month:      i32ptr(f.Month),
		Status:     f.Status,
		CursorID:   f.CursorID,
		Lim:        i32(f.Limit),
	}
	if f.CursorPaidOn != nil {
		p.CursorPaidOn = timeToPgDate(*f.CursorPaidOn)
	}
	rows, err := r.q.ListPayslips(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]svc.PayslipRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPayslipFromList(row))
	}
	return out, nil
}

func (r *PayslipRepo) GetPayslip(ctx context.Context, id string) (svc.PayslipRow, error) {
	row, err := r.q.GetPayslip(ctx, id)
	if err != nil {
		return svc.PayslipRow{}, mapErr(err)
	}
	return mapPayslipFromGet(row), nil
}

func (r *PayslipRepo) ListComponents(ctx context.Context, payslipID string) ([]svc.LineRow, error) {
	rows, err := r.q.ListPayslipComponents(ctx, payslipID)
	if err != nil {
		return nil, err
	}
	out := make([]svc.LineRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapComponent(row))
	}
	return out, nil
}

func (r *PayslipRepo) ListBenefits(ctx context.Context, payslipID string) ([]svc.LineRow, error) {
	rows, err := r.q.ListPayslipBenefits(ctx, payslipID)
	if err != nil {
		return nil, err
	}
	out := make([]svc.LineRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapBenefit(row))
	}
	return out, nil
}

// --- audit notes ---

func (r *PayslipRepo) PayslipExists(ctx context.Context, id string) (bool, error) {
	return r.q.PayslipExists(ctx, id)
}

func (r *PayslipRepo) ListAuditNotes(ctx context.Context, payslipID string, cursorSeq *int, cursorCreatedAt *time.Time, limit int) ([]dom.PayslipAuditNote, error) {
	rows, err := r.q.ListPayslipAuditNotes(ctx, sqlcgen.ListPayslipAuditNotesParams{
		PayslipID:       payslipID,
		CursorSeq:       i32ptr(cursorSeq),
		CursorCreatedAt: cursorCreatedAt,
		Lim:             i32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]dom.PayslipAuditNote, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAuditNote(row))
	}
	return out, nil
}

func (r *PayslipRepo) CountAuditNotes(ctx context.Context, payslipID string) (int, error) {
	n, err := r.q.CountPayslipAuditNotes(ctx, payslipID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *PayslipRepo) InsertAuditNote(ctx context.Context, tx pgx.Tx, p svc.AuditNoteRow) (dom.PayslipAuditNote, error) {
	row, err := r.q.WithTx(tx).InsertPayslipAuditNote(ctx, sqlcgen.InsertPayslipAuditNoteParams{
		ID:         p.ID,
		PayslipID:  p.PayslipID,
		Seq:        i32(p.Seq),
		Text:       p.Text,
		AuthorID:   p.AuthorID,
		AuthorName: p.AuthorName,
	})
	if err != nil {
		return dom.PayslipAuditNote{}, mapErr(err)
	}
	return mapAuditNote(row), nil
}
