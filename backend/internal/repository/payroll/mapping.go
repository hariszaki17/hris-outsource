// Package payroll (repository) — implements the E8 payroll service ports over the
// 10-01 sqlc queries. The repo NEVER decrypts: it returns the RAW *_enc / value_enc
// ciphertext on the svc.PayslipRow / svc.LineRow intermediates, and the SERVICE
// decrypts at the boundary (decrypt-at-boundary; INV-2). Reads on the pool; writes
// via q.WithTx(tx). pgtype.Date ↔ *time.Time + int32 ↔ *int conversions mirror
// Phase-5/6/8/9; pgx.ErrNoRows → domain.ErrNotFound.
package payroll

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func timeToPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

// pgDateToTimePtr returns nil for a NULL date (so paid_on can serialize as JSON null).
func pgDateToTimePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func i32(n int) int32 { return int32(n) }

func i32ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

func intPtr(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

// --- payslip mappers (list + get share the same column set) ---

func mapPayslipFromList(r sqlcgen.ListPayslipsRow) svc.PayslipRow {
	return svc.PayslipRow{
		ID:                 r.ID,
		EmployeeID:         r.EmployeeID,
		EmployeeName:       r.EmployeeName,
		PlacementID:        r.PlacementID,
		Year:               int(r.Year),
		Month:              int(r.Month),
		PaidOn:             pgDateToTimePtr(r.PaidOn),
		WorkingDays:        intPtr(r.WorkingDays),
		GrossEarningsEnc:   r.GrossEarningsEnc,
		GrossDeductionsEnc: r.GrossDeductionsEnc,
		TakeHomePayEnc:     r.TakeHomePayEnc,
		Status:             r.Status,
		SourceSystem:       r.SourceSystem,
		SourceID:           r.SourceID,
		CreatedAt:          r.CreatedAt,
	}
}

func mapPayslipFromGet(r sqlcgen.GetPayslipRow) svc.PayslipRow {
	return svc.PayslipRow{
		ID:                 r.ID,
		EmployeeID:         r.EmployeeID,
		EmployeeName:       r.EmployeeName,
		PlacementID:        r.PlacementID,
		Year:               int(r.Year),
		Month:              int(r.Month),
		PaidOn:             pgDateToTimePtr(r.PaidOn),
		WorkingDays:        intPtr(r.WorkingDays),
		GrossEarningsEnc:   r.GrossEarningsEnc,
		GrossDeductionsEnc: r.GrossDeductionsEnc,
		TakeHomePayEnc:     r.TakeHomePayEnc,
		Status:             r.Status,
		SourceSystem:       r.SourceSystem,
		SourceID:           r.SourceID,
		CreatedAt:          r.CreatedAt,
	}
}

func mapComponent(r sqlcgen.PayslipComponent) svc.LineRow {
	return svc.LineRow{
		Name:     r.Name,
		Kind:     r.Kind,
		ValueEnc: r.ValueEnc,
		ForBPJS:  r.ForBpjs,
	}
}

func mapBenefit(r sqlcgen.PayslipBenefit) svc.LineRow {
	return svc.LineRow{
		Name:     r.Name,
		ValueEnc: r.ValueEnc,
	}
}

func mapAuditNote(r sqlcgen.PayslipAuditNote) dom.PayslipAuditNote {
	return dom.PayslipAuditNote{
		ID:         r.ID,
		PayslipID:  r.PayslipID,
		Text:       r.Text,
		AuthorID:   r.AuthorID,
		AuthorName: r.AuthorName,
		CreatedAt:  r.CreatedAt,
	}
}

func mapExportJob(r sqlcgen.ExportJob) dom.ExportJob {
	return dom.ExportJob{
		ID:               r.ID,
		Status:           dom.ExportJobStatus(r.Status),
		Format:           r.Format,
		Confidential:     r.Confidential,
		RequestedByID:    r.RequestedByID,
		RequestedByName:  r.RequestedByName,
		ScopePeriod:      r.ScopePeriod,
		ScopeYear:        intPtr(r.ScopeYear),
		ScopeEmployeeIDs: emptyIfNil(r.ScopeEmployeeIds),
		RowCount:         intPtr(r.RowCount),
		ArtifactRef:      r.ArtifactRef,
		ErrorMessage:     r.ErrorMessage,
		RequestedAt:      r.RequestedAt,
		StartedAt:        r.StartedAt,
		CompletedAt:      r.CompletedAt,
	}
}

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
