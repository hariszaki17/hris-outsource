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
	reportingdom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// Per-field resolution status values stored in change_requests.field_resolutions
// (domain.FieldResolution.Status, lowercase). The DTO uppercases them to the wire
// FieldResolution enum (APPLIED | ESCALATED_TO_HR | PENDING).
const (
	resolutionApplied   = "applied"
	resolutionEscalated = "escalated_to_hr"
)

// ChangeRequestRepository is the data dependency for the change-requests service.
// Defined by the consumer (Go interface inversion idiom).
type ChangeRequestRepository interface {
	// Reads on pool.
	ListChangeRequests(ctx context.Context, f domain.ChangeRequestFilter) ([]domain.ChangeRequest, error)
	GetChangeRequestByID(ctx context.Context, id string) (domain.ChangeRequest, error)
	GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error)
	// GetEmployeeCompanyID returns the employee's current (non-terminal) client
	// company id for shift-leader company-scope routing. Returns "" (not an
	// error) when the employee has no active placement.
	GetEmployeeCompanyID(ctx context.Context, employeeID string) (string, error)
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
// FieldResolutions/BankPending drive the SL bank-split partial-apply
// (status='partially_approved'); they are zero/nil for a plain approve or reject.
type ResolveChangeRequestParams struct {
	ID               string
	Status           string
	FieldResolutions map[string]domain.FieldResolution
	BankPending      bool
	ResolvedAt       *time.Time
	ResolvedBy       *string
	RejectionReason  *string
}

// ChangeRequestService implements the change-request HR-approval queue.
type ChangeRequestService struct {
	repo     ChangeRequestRepository
	txm      TxRunner        // reuse TxRunner defined in employees_service.go
	now      Clock           // reuse Clock defined in employees_service.go
	notifier jobs.Dispatcher // E10 notify seam (nil-safe in unit tests)
}

// NewChangeRequestService wires the service with its dependencies.
func NewChangeRequestService(repo ChangeRequestRepository, txm TxRunner) *ChangeRequestService {
	return &ChangeRequestService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *ChangeRequestService) SetClock(c Clock) { s.now = c }

// SetNotifier wires the E10 notification dispatcher (mirrors LeaveService).
// Additive + nil-safe: jobs.Dispatch no-ops when the notifier is nil, so unit
// tests constructed without it keep passing. main.go injects the River client.
func (s *ChangeRequestService) SetNotifier(d jobs.Dispatcher) { s.notifier = d }

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

// ApproveChangeRequest approves a pending (or partially-approved) change request.
//
// Approver routing (2026-06-11 redesign): shift leaders (company-scoped) and
// HR/super-admin may approve. The bank_account field additionally requires
// rbac.CanApproveBank (HR/super-admin only). When a shift leader approves a
// MIXED request (bank + non-bank), the non-bank fields are applied now and the
// bank field is escalated to HR (status → PARTIALLY_APPROVED, bank_pending=true,
// field_resolutions records who applied/escalated each field). HR then approves
// the partially-approved request to finalize the bank field → APPROVED.
//
//   - Loads CR; status must be "pending" or "partially_approved" (else 409).
//   - GuardCompany(emp.company) — a leader out of company scope → 403.
//   - InTx: applies the resolvable fields to the employee, resolves the CR,
//     audits, and dispatches the applied/partial approval notification.
//
// actorEmp is the resolver's SWP-EMP id (recorded in field_resolutions);
// actorUser is the SWP-USR id (audit resolved_by + notification actor).
func (s *ChangeRequestService) ApproveChangeRequest(ctx context.Context, id, actorUser, actorEmp string) (domain.ChangeRequest, error) {
	cr, err := s.repo.GetChangeRequestByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequest{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ChangeRequest{}, apperr.Internal(err)
	}

	if cr.Status != "pending" && cr.Status != "partially_approved" {
		return domain.ChangeRequest{}, apperr.Conflict("CONFLICT")
	}

	// Load the current employee (all fields for UpdateEmployeeParams + company scope).
	emp, err := s.repo.GetEmployeeByID(ctx, cr.EmployeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ChangeRequest{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ChangeRequest{}, apperr.Internal(err)
	}

	// Shift-leader company scope: derive the employee's current client company and
	// enforce GuardCompany (HR/super pass through). Hidden as 403 OUT_OF_SCOPE.
	companyID, cerr := s.repo.GetEmployeeCompanyID(ctx, cr.EmployeeID)
	if cerr != nil {
		return domain.ChangeRequest{}, apperr.Internal(cerr)
	}
	if serr := rbac.GuardCompany(ctx, companyID); serr != nil {
		return domain.ChangeRequest{}, serr
	}

	// Decide which fields this actor may apply now vs escalate. Only the bank
	// field gates on CanApproveBank; phone/emergency_contact always apply.
	canBank := rbac.CanApproveBank(ctx)
	bankRequested := cr.Changes.BankAccount != nil
	escalateBank := bankRequested && !canBank

	// Build the overlay of applicable fields. When escalating the bank field, it is
	// NOT written to the employee — only phone/emergency_contact are applied.
	apply := cr.Changes
	if escalateBank {
		apply.BankAccount = nil
	}

	// Carry forward any prior resolutions (HR finalizing a partial request).
	resolutions := map[string]domain.FieldResolution{}
	for k, v := range cr.FieldResolutions {
		resolutions[k] = v
	}
	now := s.now()
	stamp := func(field string, status string) {
		resolutions[field] = domain.FieldResolution{Status: status, ResolvedBy: actorEmp, ResolvedAt: &now}
	}

	// Determine the resolved status + bank_pending.
	status := "approved"
	bankPending := false
	if escalateBank {
		status = "partially_approved"
		bankPending = true
	}

	params := buildApproveParams(emp, apply)
	beforeSnap := buildBeforeSnap(apply, emp)

	var resolved domain.ChangeRequest
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// Apply the resolvable (non-escalated) fields to the employee. Skip the
		// write entirely when nothing is applied (a bank-only escalation).
		if hasApplicableField(apply) {
			if _, inErr := s.repo.UpdateEmployee(ctx, tx, params); inErr != nil {
				return inErr
			}
		}

		// Record per-field resolutions: applied fields → applied; the escalated
		// bank field → escalated_to_hr.
		if apply.Phone != nil {
			stamp("phone", resolutionApplied)
		}
		if apply.EmergencyContact != nil {
			stamp("emergency_contact", resolutionApplied)
		}
		if apply.BankAccount != nil {
			stamp("bank_account", resolutionApplied)
		}
		if escalateBank {
			stamp("bank_account", resolutionEscalated)
		}

		rp := ResolveChangeRequestParams{
			ID:               id,
			Status:           status,
			FieldResolutions: resolutions,
			BankPending:      bankPending,
		}
		// Terminal resolutions stamp resolved_at/resolved_by; a partial apply leaves
		// them nil (the request is not yet finally resolved).
		if status == "approved" {
			rp.ResolvedAt = &now
			rp.ResolvedBy = &actorUser
		}

		var inErr error
		resolved, inErr = s.repo.ResolveChangeRequest(ctx, tx, rp)
		if inErr != nil {
			return inErr
		}

		if err := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("change_request.approve"),
			EntityType: "change_request",
			EntityID:   id,
			Before:     beforeSnap,
			After:      buildAfterSnap(apply),
		}); err != nil {
			return err
		}

		// E10: notify the submitter. The body lists the applied fields and flags a
		// partial approval (bank escalated to HR).
		title := "Pengajuan perubahan disetujui"
		body := "Pengajuan perubahan data Anda disetujui."
		if escalateBank {
			title = "Pengajuan perubahan disetujui sebagian"
			body = "Sebagian data Anda telah diperbarui; perubahan rekening menunggu persetujuan HR."
		}
		if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
			NotifKind:        string(reportingdom.NotifChangeRequestApproved),
			RecipientID:      cr.EmployeeID,
			Title:            title,
			Body:             body,
			DeepLinkEpic:     "E2",
			DeepLinkEntityID: id,
			DeepLinkPath:     "/change-requests/" + id,
			ActorID:          actorUser,
			IsCritical:       false,
		}); derr != nil {
			return derr
		}
		return nil
	}); err != nil {
		return domain.ChangeRequest{}, asCRAppErr(err)
	}

	return resolved, nil
}

// RejectChangeRequest rejects a pending or partially-approved change request.
//   - Validates reason len 3..500 → 400 INVALID_REQUEST.
//   - Loads CR; if status is terminal → 409 CONFLICT.
//   - GuardCompany(emp.company) — a leader out of company scope → 403.
//   - InTx: ResolveChangeRequest(status=rejected, reason) + audit + notify.
//   - Employee is NOT modified.
//   - Returns the updated ChangeRequest (200). actorUser is the resolver SWP-USR id.
func (s *ChangeRequestService) RejectChangeRequest(ctx context.Context, id, reason, actorUser string) (domain.ChangeRequest, error) {
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

	if cr.Status != "pending" && cr.Status != "partially_approved" {
		return domain.ChangeRequest{}, apperr.Conflict("CONFLICT")
	}

	// Shift-leader company scope (mirrors approve): a leader may reject only their
	// own company's requests; HR/super pass through.
	companyID, cerr := s.repo.GetEmployeeCompanyID(ctx, cr.EmployeeID)
	if cerr != nil {
		return domain.ChangeRequest{}, apperr.Internal(cerr)
	}
	if serr := rbac.GuardCompany(ctx, companyID); serr != nil {
		return domain.ChangeRequest{}, serr
	}

	prevStatus := cr.Status
	now := s.now()
	var resolved domain.ChangeRequest
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		resolved, inErr = s.repo.ResolveChangeRequest(ctx, tx, ResolveChangeRequestParams{
			ID:               id,
			Status:           "rejected",
			FieldResolutions: cr.FieldResolutions, // preserve any prior partial resolutions
			BankPending:      false,               // rejecting clears the escalation flag
			ResolvedAt:       &now,
			ResolvedBy:       &actorUser,
			RejectionReason:  &reason,
		})
		if inErr != nil {
			return inErr
		}

		if err := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("change_request.reject"),
			EntityType: "change_request",
			EntityID:   id,
			Before:     map[string]any{"status": prevStatus, "employee_id": cr.EmployeeID},
			After:      map[string]any{"status": "rejected", "rejection_reason": reason, "resolved_by": actorUser},
		}); err != nil {
			return err
		}

		// E10: notify the submitter their request was rejected, carrying the reason.
		if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
			NotifKind:        string(reportingdom.NotifChangeRequestRejected),
			RecipientID:      cr.EmployeeID,
			Title:            "Pengajuan perubahan ditolak",
			Body:             "Pengajuan perubahan data Anda ditolak: " + reason,
			DeepLinkEpic:     "E2",
			DeepLinkEntityID: id,
			DeepLinkPath:     "/change-requests/" + id,
			ActorID:          actorUser,
			IsCritical:       false,
		}); derr != nil {
			return derr
		}
		return nil
	}); err != nil {
		return domain.ChangeRequest{}, asCRAppErr(err)
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
// request_type (PHONE|EMERGENCY_CONTACT|BANK_ACCOUNT|MULTIPLE). Returns a 422
// INVALID if no field is present (the DTO's additionalProperties:false already
// blocks statutory + instant-tier fields, so unknown fields never reach here).
func deriveRequestType(c domain.ChangeRequestChanges) (string, error) {
	var present []string
	if c.Phone != nil {
		present = append(present, "PHONE")
	}
	if c.EmergencyContact != nil {
		present = append(present, "EMERGENCY_CONTACT")
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

// hasApplicableField reports whether the overlay carries at least one field to
// write to the employee (so a bank-only escalation skips the employee update).
func hasApplicableField(c domain.ChangeRequestChanges) bool {
	return c.Phone != nil || c.EmergencyContact != nil || c.BankAccount != nil
}

// asCRAppErr returns err unchanged when it is already an *apperr.Error (so
// GuardCompany/repo app errors surface with their status), otherwise wraps it as
// a 500. Mirrors the leave service's asAppErr seam.
func asCRAppErr(err error) error {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae
	}
	return apperr.Internal(err)
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
	if changes.EmergencyContact != nil {
		var oldEC any
		if emp.EmergencyContact.Name != "" || emp.EmergencyContact.Phone != "" {
			oldEC = emp.EmergencyContact
		}
		diff["emergency_contact"] = domain.ChangeRequestFieldDiff{
			Old: oldEC,
			New: changes.EmergencyContact,
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
// applicable changed fields (phone/emergency_contact/bank_account only).
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
		EmergencyContactName:  emp.EmergencyContact.Name,
		EmergencyContactPhone: emp.EmergencyContact.Phone,
	}

	// Overlay applicable fields.
	if changes.Phone != nil {
		p.Phone = *changes.Phone
	}
	if changes.EmergencyContact != nil {
		p.EmergencyContactName = changes.EmergencyContact.Name
		p.EmergencyContactPhone = changes.EmergencyContact.Phone
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
	if changes.EmergencyContact != nil {
		snap["emergency_contact"] = map[string]any{
			"name":  emp.EmergencyContact.Name,
			"phone": emp.EmergencyContact.Phone,
		}
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
	if changes.EmergencyContact != nil {
		snap["emergency_contact"] = map[string]any{
			"name":  changes.EmergencyContact.Name,
			"phone": changes.EmergencyContact.Phone,
		}
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
