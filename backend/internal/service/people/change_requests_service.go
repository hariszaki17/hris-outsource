// Package people — ChangeRequestService implements the HR-approval queue for
// employee-submitted change requests (E2 EP-5). Approve applies the whitelisted
// phone/address/bank_account change to the employee in the same tx (never
// statutory fields like NIK, NIP, join_at). Reject requires a reason (min 3 chars).
// Both are audited. Notification dispatch on resolve is DEFERRED — no notification
// epic is in scope for Phase 4; a stub comment marks the integration point.
//
// Separate struct in the same package — mirrors AgreementService / Service pattern.
package people

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// ChangeRequestRepository is the data dependency for the change-requests service.
// Defined by the consumer (Go interface inversion idiom).
type ChangeRequestRepository interface {
	// Reads on pool.
	ListChangeRequests(ctx context.Context, f domain.ChangeRequestFilter) ([]domain.ChangeRequest, error)
	GetChangeRequestByID(ctx context.Context, id string) (domain.ChangeRequest, error)
	GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error)
	// Writes in tx.
	UpdateEmployee(ctx context.Context, tx pgx.Tx, p UpdateEmployeeParams) (domain.Employee, error)
	ResolveChangeRequest(ctx context.Context, tx pgx.Tx, p ResolveChangeRequestParams) (domain.ChangeRequest, error)
	CreateChangeRequest(ctx context.Context, tx pgx.Tx, p CreateChangeRequestParams) (domain.ChangeRequest, error)
}

// CreateChangeRequestParams carries fields for the CreateChangeRequest repo call.
// Changes is the already-validated, whitelisted set of proposed field changes;
// RequestType is derived (PHONE|ADDRESS|BANK_ACCOUNT|MULTIPLE).
type CreateChangeRequestParams struct {
	EmployeeID  string
	Changes     domain.ChangeRequestChanges
	RequestType string
	Note        *string
}

// ResolveChangeRequestParams carries fields for the ResolveChangeRequest repo call.
type ResolveChangeRequestParams struct {
	ID              string
	Status          string
	ResolvedAt      *time.Time
	ResolvedBy      *string
	RejectionReason *string
}

// ChangeRequestService implements the change-request HR-approval queue.
type ChangeRequestService struct {
	repo ChangeRequestRepository
	txm  TxRunner // reuse TxRunner defined in employees_service.go
	now  Clock    // reuse Clock defined in employees_service.go
}

// NewChangeRequestService wires the service with its dependencies.
func NewChangeRequestService(repo ChangeRequestRepository, txm TxRunner) *ChangeRequestService {
	return &ChangeRequestService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *ChangeRequestService) SetClock(c Clock) { s.now = c }

// crPageCursor is the opaque JSON payload encoded into the cursor string.
// Uses (submitted_at, id) to match the change_requests index.
type crPageCursor struct {
	SubmittedAt time.Time `json:"s"`
	ID          string    `json:"i"`
}

// --- Change Requests ---

// ListChangeRequests returns a cursor-paginated page of change requests.
// The default queue view passes status=PENDING from the FE; the endpoint supports all statuses.
func (s *ChangeRequestService) ListChangeRequests(ctx context.Context, f domain.ChangeRequestFilter) ([]domain.ChangeRequest, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1 // fetch one extra to detect has_more

	// Lowercase the status filter before the query (DB stores lowercase).
	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListChangeRequests(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(crPageCursor{SubmittedAt: last.SubmittedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// GetChangeRequestDetail loads a change request and builds the per-field diff:
// for each field present in CR.Changes, old = current employee value, new = requested value.
// Returns domain.ChangeRequestDetail (CR + employee ref + diff map).
func (s *ChangeRequestService) GetChangeRequestDetail(ctx context.Context, id string) (domain.ChangeRequestDetail, error) {
	cr, err := s.repo.GetChangeRequestByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequestDetail{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ChangeRequestDetail{}, apperr.Internal(err)
	}

	emp, err := s.repo.GetEmployeeByID(ctx, cr.EmployeeID)
	if errors.Is(err, domain.ErrNotFound) {
		// Employee deleted after CR submitted — rare but return 404.
		return domain.ChangeRequestDetail{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ChangeRequestDetail{}, apperr.Internal(err)
	}

	diff := buildDiff(cr.Changes, emp)

	return domain.ChangeRequestDetail{
		ChangeRequest: cr,
		Employee: domain.EmployeeRef{
			ID:       emp.ID,
			FullName: emp.FullName,
			NIP:      emp.NIP,
		},
		Diff: diff,
	}, nil
}

// ApproveChangeRequest approves a pending change request.
//   - Loads CR; if status != "pending" → 409 CONFLICT.
//   - InTx: builds UpdateEmployeeParams from current employee overlaid with the
//     whitelisted changed fields (phone/address/bank_account ONLY — statutory
//     fields NIK/NIP/join_at are never touched).
//   - Calls repo.UpdateEmployee + repo.ResolveChangeRequest + audit.
//   - Returns the updated ChangeRequest (200).
func (s *ChangeRequestService) ApproveChangeRequest(ctx context.Context, id, actor string) (domain.ChangeRequest, error) {
	cr, err := s.repo.GetChangeRequestByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequest{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ChangeRequest{}, apperr.Internal(err)
	}

	if cr.Status != "pending" {
		return domain.ChangeRequest{}, apperr.Conflict("CONFLICT")
	}

	// Load the current employee to get all fields for UpdateEmployeeParams.
	emp, err := s.repo.GetEmployeeByID(ctx, cr.EmployeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequest{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ChangeRequest{}, apperr.Internal(err)
	}

	// Build UpdateEmployeeParams: start with current employee values, overlay whitelisted changes.
	params := buildApproveParams(emp, cr.Changes)

	// Record before-state for the audit diff.
	beforeSnap := buildBeforeSnap(cr.Changes, emp)

	now := s.now()
	var resolved domain.ChangeRequest
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// Apply the whitelisted fields to the employee.
		if _, inErr := s.repo.UpdateEmployee(ctx, tx, params); inErr != nil {
			return inErr
		}

		// Resolve the change request.
		var inErr error
		resolved, inErr = s.repo.ResolveChangeRequest(ctx, tx, ResolveChangeRequestParams{
			ID:         id,
			Status:     "approved",
			ResolvedAt: &now,
			ResolvedBy: &actor,
		})
		if inErr != nil {
			return inErr
		}

		// Audit: capture what changed on the employee.
		afterSnap := buildAfterSnap(cr.Changes)
		if err := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("change_request.approve"),
			EntityType: "change_request",
			EntityID:   id,
			Before:     beforeSnap,
			After:      afterSnap,
		}); err != nil {
			return err
		}

		// STUB: notification dispatch on CR resolution is DEFERRED.
		// When E11 Notifications is implemented, enqueue a NotificationArgs job here:
		//   jobs.Client.EnqueueTx(tx, &NotificationArgs{EmployeeID: cr.EmployeeID, Type: "change_request.approved"})
		return nil
	}); err != nil {
		return domain.ChangeRequest{}, apperr.Internal(err)
	}

	return resolved, nil
}

// RejectChangeRequest rejects a pending change request.
//   - Validates reason len 3..500 → 400 INVALID_REQUEST.
//   - Loads CR; if status != "pending" → 409 CONFLICT.
//   - InTx: ResolveChangeRequest(status=rejected, reason) + audit.
//   - Employee is NOT modified.
//   - Returns the updated ChangeRequest (200).
func (s *ChangeRequestService) RejectChangeRequest(ctx context.Context, id, reason, actor string) (domain.ChangeRequest, error) {
	// Validate reason: required, min 3 chars, max 500.
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 {
		return domain.ChangeRequest{}, apperr.Invalid(map[string]string{
			"reason": "Alasan penolakan wajib diisi minimal 3 karakter.",
		})
	}
	if len(reason) > 500 {
		return domain.ChangeRequest{}, apperr.Invalid(map[string]string{
			"reason": "Alasan penolakan maksimal 500 karakter.",
		})
	}

	cr, err := s.repo.GetChangeRequestByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequest{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ChangeRequest{}, apperr.Internal(err)
	}

	if cr.Status != "pending" {
		return domain.ChangeRequest{}, apperr.Conflict("CONFLICT")
	}

	now := s.now()
	var resolved domain.ChangeRequest
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		resolved, inErr = s.repo.ResolveChangeRequest(ctx, tx, ResolveChangeRequestParams{
			ID:              id,
			Status:          "rejected",
			ResolvedAt:      &now,
			ResolvedBy:      &actor,
			RejectionReason: &reason,
		})
		if inErr != nil {
			return inErr
		}

		if err := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("change_request.reject"),
			EntityType: "change_request",
			EntityID:   id,
			Before:     map[string]any{"status": "pending", "employee_id": cr.EmployeeID},
			After:      map[string]any{"status": "rejected", "rejection_reason": reason, "resolved_by": actor},
		}); err != nil {
			return err
		}

		// STUB: notification dispatch on CR resolution is DEFERRED.
		// When E11 Notifications is implemented, enqueue a NotificationArgs job here.
		return nil
	}); err != nil {
		return domain.ChangeRequest{}, apperr.Internal(err)
	}

	return resolved, nil
}

// CreateChangeRequest files a new PENDING change request for an employee's
// non-statutory profile fields (phone/address/bank_account — E2 §8 lock).
//
// Self-scope (E2 F2.1): an agent may file ONLY for their own employee_id. If the
// path employee_id differs from the caller's, the existence of the employee is
// hidden as 404 (no leak), matching GetEmployee. Staff (hr_admin/super_admin) may
// file on behalf of any employee — the route admits them but they pass the path id
// directly.
//
//   - Requires at least one whitelisted field in changes (else 422 EMPTY_CHANGES).
//   - Derives request_type: a single field → PHONE|ADDRESS|BANK_ACCOUNT; 2+ → MULTIPLE.
//   - InTx: inserts the PENDING request + audit.
//   - Returns the created ChangeRequest (status=pending).
func (s *ChangeRequestService) CreateChangeRequest(ctx context.Context, employeeID string, changes domain.ChangeRequestChanges, note *string) (domain.ChangeRequest, error) {
	// Self-scope: an agent may only file for their own record.
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleAgent {
		if p.EmployeeID == "" || p.EmployeeID != employeeID {
			return domain.ChangeRequest{}, apperr.NotFound()
		}
	}

	// Derive request_type from the present fields; reject an empty change set.
	requestType, err := deriveRequestType(changes)
	if err != nil {
		return domain.ChangeRequest{}, err
	}

	// Normalize the note (trim; empty → nil; enforce max length).
	if note != nil {
		trimmed := strings.TrimSpace(*note)
		if len(trimmed) > 500 {
			return domain.ChangeRequest{}, apperr.Invalid(map[string]string{
				"note": "Catatan maksimal 500 karakter.",
			})
		}
		if trimmed == "" {
			note = nil
		} else {
			note = &trimmed
		}
	}

	var created domain.ChangeRequest
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateChangeRequest(ctx, tx, CreateChangeRequestParams{
			EmployeeID:  employeeID,
			Changes:     changes,
			RequestType: requestType,
			Note:        note,
		})
		if inErr != nil {
			return inErr
		}

		if err := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("change_request.create"),
			EntityType: "change_request",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"employee_id":  created.EmployeeID,
				"request_type": created.RequestType,
				"status":       created.Status,
			},
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return domain.ChangeRequest{}, apperr.Internal(err)
	}

	return created, nil
}

// deriveRequestType inspects the whitelisted change fields and returns the DB
// request_type (PHONE|ADDRESS|BANK_ACCOUNT|MULTIPLE). Returns a 422 INVALID if
// no field is present (the DTO's additionalProperties:false already blocks
// statutory fields, so unknown fields never reach here).
func deriveRequestType(c domain.ChangeRequestChanges) (string, error) {
	var present []string
	if c.Phone != nil {
		present = append(present, "PHONE")
	}
	if c.Address != nil {
		present = append(present, "ADDRESS")
	}
	if c.BankAccount != nil {
		present = append(present, "BANK_ACCOUNT")
	}
	switch len(present) {
	case 0:
		return "", apperr.Invalid(map[string]string{
			"changes": "Minimal satu field perubahan wajib diisi.",
		})
	case 1:
		return present[0], nil
	default:
		return "MULTIPLE", nil
	}
}

// --- private helpers ---

// buildDiff constructs the per-field old→new diff map for the detail response.
// For each field present in the changes, old = current employee value, new = requested.
func buildDiff(changes domain.ChangeRequestChanges, emp domain.Employee) map[string]domain.ChangeRequestFieldDiff {
	diff := make(map[string]domain.ChangeRequestFieldDiff)

	if changes.Phone != nil {
		diff["phone"] = domain.ChangeRequestFieldDiff{
			Old: emp.Phone, // *string (nil if not set)
			New: changes.Phone,
		}
	}
	if changes.Address != nil {
		diff["address"] = domain.ChangeRequestFieldDiff{
			Old: emp.Address,
			New: changes.Address,
		}
	}
	if changes.BankAccount != nil {
		var oldBank any
		if emp.BankAccount.BankName != "" || emp.BankAccount.AccountNumber != "" {
			oldBank = emp.BankAccount
		}
		diff["bank_account"] = domain.ChangeRequestFieldDiff{
			Old: oldBank,
			New: changes.BankAccount,
		}
	}

	return diff
}

// buildApproveParams starts with the current employee fields and overlays the
// whitelisted changed fields (phone/address/bank_account only).
func buildApproveParams(emp domain.Employee, changes domain.ChangeRequestChanges) UpdateEmployeeParams {
	p := UpdateEmployeeParams{
		ID:                    emp.ID,
		FullName:              emp.FullName,
		NIK:                   emp.NIK,
		NIP:                   emp.NIP,
		JoinAt:                emp.JoinAt,
		Gender:                derefPtrStr(emp.Gender),
		BirthDate:             emp.BirthDate,
		BirthPlace:            derefPtrStr(emp.BirthPlace),
		Phone:                 derefPtrStr(emp.Phone),
		EmailPersonal:         derefPtrStr(emp.EmailPersonal),
		Address:               derefPtrStr(emp.Address),
		NPWP:                  derefPtrStr(emp.NPWP),
		BPJSKesehatan:         derefPtrStr(emp.BPJSKesehatan),
		BPJSKetenagakerjaan:   derefPtrStr(emp.BPJSKetenagakerjaan),
		BankName:              emp.BankAccount.BankName,
		BankAccountNumber:     emp.BankAccount.AccountNumber,
		BankAccountHolderName: emp.BankAccount.AccountHolderName,
	}

	// Overlay whitelisted fields.
	if changes.Phone != nil {
		p.Phone = *changes.Phone
	}
	if changes.Address != nil {
		p.Address = *changes.Address
	}
	if changes.BankAccount != nil {
		p.BankName = changes.BankAccount.BankName
		p.BankAccountNumber = changes.BankAccount.AccountNumber
		p.BankAccountHolderName = changes.BankAccount.AccountHolderName
	}

	return p
}

// buildBeforeSnap builds the audit before-snapshot for the changed fields.
func buildBeforeSnap(changes domain.ChangeRequestChanges, emp domain.Employee) map[string]any {
	snap := map[string]any{}
	if changes.Phone != nil {
		snap["phone"] = emp.Phone
	}
	if changes.Address != nil {
		snap["address"] = emp.Address
	}
	if changes.BankAccount != nil {
		snap["bank_account"] = map[string]any{
			"bank_name":      emp.BankAccount.BankName,
			"account_number": emp.BankAccount.AccountNumber,
			"account_holder": emp.BankAccount.AccountHolderName,
		}
	}
	return snap
}

// buildAfterSnap builds the audit after-snapshot for the changed fields.
func buildAfterSnap(changes domain.ChangeRequestChanges) map[string]any {
	snap := map[string]any{}
	if changes.Phone != nil {
		snap["phone"] = *changes.Phone
	}
	if changes.Address != nil {
		snap["address"] = *changes.Address
	}
	if changes.BankAccount != nil {
		snap["bank_account"] = map[string]any{
			"bank_name":      changes.BankAccount.BankName,
			"account_number": changes.BankAccount.AccountNumber,
			"account_holder": changes.BankAccount.AccountHolderName,
		}
	}
	return snap
}

// derefPtrStr safely dereferences a *string (returns "" if nil).
func derefPtrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
