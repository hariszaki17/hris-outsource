// Package people — AgreementService implements E2 employment-agreement business
// logic: list, get, create (EA-1/EA-2 one-active guard, PKWT/PKWTT cross-field
// rules), renew (EA-3 successor chain), close (EA-5), and multipart attachment
// upload (CONVENTIONS §15). Mirrors the pattern of Service in employees_service.go:
// separate struct in the same package, consumer-defined interface, TxRunner, Clock.
package people

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// AgreementRepository is the data dependency for the agreements service.
// Defined by the consumer (Go interface inversion idiom).
type AgreementRepository interface {
	// Reads on pool.
	ListAgreements(ctx context.Context, f domain.AgreementFilter) ([]domain.Agreement, error)
	GetAgreementByID(ctx context.Context, id string) (domain.Agreement, error)
	GetActiveAgreementForEmployee(ctx context.Context, employeeID string) (domain.Agreement, error)
	GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error)
	// Writes in tx.
	CreateAgreement(ctx context.Context, tx pgx.Tx, p CreateAgreementParams) (domain.Agreement, error)
	SetAgreementStatus(ctx context.Context, tx pgx.Tx, p SetAgreementStatusParams) (domain.Agreement, error)
	CreateAttachment(ctx context.Context, tx pgx.Tx, p CreateAttachmentParams) (domain.Attachment, error)
	GetAttachmentByID(ctx context.Context, id string) (domain.Attachment, error)
}

// CreateAgreementParams carries fields for inserting a new employment agreement.
type CreateAgreementParams struct {
	EmployeeID                 string
	Type                       string // "PKWT" | "PKWTT"
	AgreementNo                string
	StartDate                  time.Time
	EndDate                    *time.Time // nil for PKWTT
	PredecessorID              *string
	BaseSalaryIDR              *float64
	AnnualLeaveEntitlementDays *int32
	BpjsTerms                  domain.BpjsTerms
	TaxProfile                 *string
	CompEffectiveDate          *time.Time
	CreatedBy                  *string
}

// SetAgreementStatusParams carries fields for SetAgreementStatus (close / supersede).
type SetAgreementStatusParams struct {
	ID           string
	Status       string
	ClosedReason *string
	ClosedAt     *time.Time
	SuccessorID  *string
}

// CreateAttachmentParams carries fields for inserting an attachment row.
type CreateAttachmentParams struct {
	AgreementID string
	Category    string
	Caption     string
	FileName    string
	MIME        string
	SizeBytes   int64
	Blob        []byte
	UploadedBy  *string
}

// AgreementService implements the employment-agreement business logic.
type AgreementService struct {
	repo AgreementRepository
	txm  TxRunner // reuse TxRunner defined in employees_service.go
	now  Clock    // reuse Clock defined in employees_service.go
}

// NewAgreementService wires the service with its dependencies.
func NewAgreementService(repo AgreementRepository, txm TxRunner) *AgreementService {
	return &AgreementService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *AgreementService) SetClock(c Clock) { s.now = c }

// agreementPageCursor is the opaque JSON payload encoded into the cursor string.
type agreementPageCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// --- Agreements ---

// ListAgreements returns a cursor-paginated page of agreements.
// The EXPIRING virtual status is handled here: if status filter == "EXPIRING",
// convert to an active+end_date__lte query (the DTO boundary also re-derives it
// per record).
func (s *AgreementService) ListAgreements(ctx context.Context, f domain.AgreementFilter) ([]domain.Agreement, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1

	// Lower status for DB query (DB stores lowercase).
	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		// "expiring" is a virtual status: translate to active + end_date__lte filter.
		if lower == "expiring" {
			activeStr := "active"
			f.Status = &activeStr
			if f.EndDateLTE == nil {
				cutoff := s.now().Add(30 * 24 * time.Hour)
				f.EndDateLTE = &cutoff
			}
		} else {
			f.Status = &lower
		}
	}

	// Type lowercased for DB.
	if f.Type != nil {
		upper := strings.ToUpper(*f.Type)
		f.Type = &upper
	}

	rows, err := s.repo.ListAgreements(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(agreementPageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// GetAgreement returns a single agreement by id.
func (s *AgreementService) GetAgreement(ctx context.Context, id string) (domain.Agreement, error) {
	ag, err := s.repo.GetAgreementByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Agreement{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Agreement{}, apperr.Internal(err)
	}
	return ag, nil
}

// CreateAgreement creates a new employment agreement.
// Validates:
//  1. Employee must exist → 404.
//  2. PKWT requires end_date; PKWTT must not have end_date → 400 INVALID_REQUEST.
//  3. end_date < start_date → 400 INVALID_REQUEST.
//  4. PKWT period > 5 years → 422 PKWT_PERIOD_EXCEEDS_MAX.
//  5. Employee already has an active agreement → 409 ACTIVE_AGREEMENT_EXISTS.
func (s *AgreementService) CreateAgreement(ctx context.Context, p CreateAgreementParams) (domain.Agreement, error) {
	// 1. Employee must exist.
	if _, err := s.repo.GetEmployeeByID(ctx, p.EmployeeID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Agreement{}, apperr.NotFound()
		}
		return domain.Agreement{}, apperr.Internal(err)
	}

	// 2 & 3. Cross-field PKWT/PKWTT date validation.
	if err := validateAgreementDates(p.Type, p.StartDate, p.EndDate); err != nil {
		return domain.Agreement{}, err
	}

	// 5. EA-2: exactly one active agreement per employee.
	_, activeErr := s.repo.GetActiveAgreementForEmployee(ctx, p.EmployeeID)
	if activeErr == nil {
		// Active agreement found → conflict.
		return domain.Agreement{}, apperr.Conflict("ACTIVE_AGREEMENT_EXISTS")
	}
	if !errors.Is(activeErr, domain.ErrNotFound) {
		return domain.Agreement{}, apperr.Internal(activeErr)
	}

	var created domain.Agreement
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateAgreement(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("agreement.create"),
			EntityType: "employment_agreement",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"employee_id":  created.EmployeeID,
				"type":         created.Type,
				"agreement_no": created.AgreementNo,
				"status":       created.Status,
			},
		})
	}); err != nil {
		return domain.Agreement{}, apperr.Internal(err)
	}

	return created, nil
}

// RenewAgreement creates a successor agreement (EA-3).
// Predecessor must be active → 409 CONFLICT otherwise.
// Same PKWT/PKWTT date rules apply to the successor.
func (s *AgreementService) RenewAgreement(ctx context.Context, predecessorID string, p CreateAgreementParams) (domain.Agreement, error) {
	// Load predecessor.
	predecessor, err := s.repo.GetAgreementByID(ctx, predecessorID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Agreement{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Agreement{}, apperr.Internal(err)
	}

	// Predecessor must be active.
	if predecessor.Status != "active" {
		return domain.Agreement{}, apperr.Conflict("CONFLICT")
	}

	// Set predecessor_id on the new agreement.
	p.PredecessorID = &predecessorID
	p.EmployeeID = predecessor.EmployeeID

	// Validate new period PKWT rules.
	if err := validateAgreementDates(p.Type, p.StartDate, p.EndDate); err != nil {
		return domain.Agreement{}, err
	}

	var newAgreement domain.Agreement
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// Supersede predecessor FIRST so the partial unique index
		// (employment_agreements_active_employee_uq) is released before inserting
		// the new active agreement for the same employee.
		_, inErr := s.repo.SetAgreementStatus(ctx, tx, SetAgreementStatusParams{
			ID:     predecessorID,
			Status: "superseded",
			// SuccessorID is backfilled after we know the new ID.
		})
		if inErr != nil {
			return inErr
		}

		// Now create the successor (unique constraint no longer blocks it).
		newAgreement, inErr = s.repo.CreateAgreement(ctx, tx, p)
		if inErr != nil {
			return inErr
		}

		// Backfill successor_id on the superseded predecessor.
		succID := newAgreement.ID
		_, inErr = s.repo.SetAgreementStatus(ctx, tx, SetAgreementStatusParams{
			ID:          predecessorID,
			Status:      "superseded",
			SuccessorID: &succID,
		})
		if inErr != nil {
			return inErr
		}

		// Set predecessor_id on new agreement (link back).
		predID := predecessorID
		newAgreement.PredecessorID = &predID

		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("agreement.renew"),
			EntityType: "employment_agreement",
			EntityID:   newAgreement.ID,
			Before:     map[string]any{"predecessor_id": predecessorID, "predecessor_status": "active"},
			After: map[string]any{
				"successor_id":    newAgreement.ID,
				"type":            newAgreement.Type,
				"agreement_no":    newAgreement.AgreementNo,
				"predecessor_now": "superseded",
			},
		})
	}); err != nil {
		return domain.Agreement{}, apperr.Internal(err)
	}

	return newAgreement, nil
}

// CloseAgreement terminates an active agreement (EA-5).
// Agreement must be active → 409 CONFLICT otherwise.
// reason must be one of RESIGNED/TERMINATED/END_OF_TERM/OTHER.
func (s *AgreementService) CloseAgreement(ctx context.Context, id, reason string, effectiveDate time.Time, note string) (domain.Agreement, error) {
	ag, err := s.repo.GetAgreementByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Agreement{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Agreement{}, apperr.Internal(err)
	}

	if ag.Status != "active" {
		return domain.Agreement{}, apperr.Conflict("CONFLICT")
	}

	// Validate reason enum.
	validReasons := map[string]bool{
		"RESIGNED": true, "TERMINATED": true, "END_OF_TERM": true,
		"DECEASED": true, "RETIRED": true, "ABSCONDED": true, "OTHER": true,
	}
	if !validReasons[reason] {
		return domain.Agreement{}, apperr.Invalid(map[string]string{"reason": "Alasan tidak valid. Pilih: RESIGNED, TERMINATED, END_OF_TERM, OTHER."})
	}
	if effectiveDate.IsZero() {
		return domain.Agreement{}, apperr.Invalid(map[string]string{"effective_date": "Wajib diisi."})
	}

	now := s.now()
	var updated domain.Agreement
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetAgreementStatus(ctx, tx, SetAgreementStatusParams{
			ID:           id,
			Status:       "closed",
			ClosedReason: &reason,
			ClosedAt:     &now,
		})
		if inErr != nil {
			return inErr
		}
		afterSnap := map[string]any{"status": "closed", "closed_reason": reason, "closed_at": now.Format(time.RFC3339)}
		if note != "" {
			afterSnap["note"] = note
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("agreement.close"),
			EntityType: "employment_agreement",
			EntityID:   id,
			Before:     map[string]any{"status": "active"},
			After:      afterSnap,
		})
	}); err != nil {
		return domain.Agreement{}, apperr.Internal(err)
	}

	return updated, nil
}

// UploadAttachment stores a file attachment for an agreement.
// Validates: size ≤ 10MB (→ 413 FILE_TOO_LARGE), mime ∈ allowed set (→ 400).
func (s *AgreementService) UploadAttachment(ctx context.Context, agreementID string, p CreateAttachmentParams) (domain.Attachment, error) {
	// Agreement must exist.
	_, err := s.repo.GetAgreementByID(ctx, agreementID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Attachment{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Attachment{}, apperr.Internal(err)
	}

	// File size ≤ 10MB.
	const maxSize = 10 * 1024 * 1024
	if p.SizeBytes > maxSize {
		return domain.Attachment{}, &apperr.Error{
			Code:       "FILE_TOO_LARGE",
			HTTPStatus: http.StatusRequestEntityTooLarge,
		}
	}

	// MIME type must be one of: application/pdf, image/jpeg, image/png.
	allowedMIME := map[string]bool{
		"application/pdf": true,
		"image/jpeg":      true,
		"image/png":       true,
	}
	if !allowedMIME[p.MIME] {
		return domain.Attachment{}, apperr.Invalid(map[string]string{"file": "Tipe file tidak didukung. Gunakan PDF, JPEG, atau PNG."})
	}

	p.AgreementID = agreementID

	var created domain.Attachment
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateAttachment(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("agreement.attach"),
			EntityType: "agreement_attachment",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"agreement_id": agreementID,
				"file_name":    p.FileName,
				"mime":         p.MIME,
				"size_bytes":   p.SizeBytes,
				"category":     p.Category,
			},
		})
	}); err != nil {
		return domain.Attachment{}, apperr.Internal(err)
	}

	return created, nil
}

// GetAttachment returns an attachment by file id (for authenticated download).
func (s *AgreementService) GetAttachment(ctx context.Context, fileID string) (domain.Attachment, error) {
	att, err := s.repo.GetAttachmentByID(ctx, fileID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Attachment{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Attachment{}, apperr.Internal(err)
	}
	return att, nil
}

// --- private validation helpers ---

// validateAgreementDates enforces PKWT/PKWTT cross-field date rules:
//   - PKWT requires end_date; PKWTT must not have end_date.
//   - end_date must be >= start_date.
//   - PKWT period (end - start) must not exceed 5 years.
func validateAgreementDates(agType string, start time.Time, end *time.Time) error {
	upper := strings.ToUpper(agType)
	if upper == "PKWT" {
		if end == nil {
			return apperr.Invalid(map[string]string{"end_date": "PKWT wajib memiliki tanggal berakhir."})
		}
		if end.Before(start) || end.Equal(start) {
			return apperr.Invalid(map[string]string{"end_date": "Tanggal berakhir harus setelah tanggal mulai."})
		}
		// 5-year limit (Indonesian labor law).
		fiveYears := start.AddDate(5, 0, 0)
		if end.After(fiveYears) {
			return apperr.Rule("PKWT_PERIOD_EXCEEDS_MAX", map[string]string{
				"end_date": "Periode PKWT melebihi maksimum 5 tahun.",
			})
		}
	} else if upper == "PKWTT" {
		if end != nil {
			return apperr.Invalid(map[string]string{"end_date": "PKWTT tidak boleh memiliki tanggal berakhir."})
		}
	}
	return nil
}
