// Package leave — LeaveService: leave-request lifecycle (create/submit/cancel/
// shorten) + the per-type quota meter (reserve/commit/release/reverse), wired to the
// E11 approval engine. Submit reserves the duration, sets status=PENDING, and creates
// an E11 ApprovalInstance; the engine drives the chain and calls the OnApproved /
// OnRejected hooks on terminal transition (in the engine's tx) to apply the leave-
// domain side-effects (quota commit/release, INV-3 schedule loop-closer, notify).
package leave

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	approval "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	reportingdom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// LeaveService implements the leave-request business logic.
type LeaveService struct {
	repo     LeaveRepository
	meter    *QuotaMeter // per-type cap_basis ledger (2026-06-12; the sole balance model)
	schedule SchedulePort
	txm      TxRunner
	now      Clock
	notifier jobs.Dispatcher  // E10 (11-02): transactional-outbox notify seam (nil-safe in unit tests)
	engine   approval.Engine  // E11: creates the approval instance at :submit
}

// SetMeter wires the per-type QuotaMeter (EPICS §8 2026-06-12): reserve/commit/
// release/reverse meter against per-type cap_basis windows. Production (cmd/api)
// sets it; tests wire an in-memory meter via the same hook.
func (s *LeaveService) SetMeter(m *QuotaMeter) { s.meter = m }

// SetApprovalEngine wires the E11 approval engine. REQUIRED for Submit (the
// integrator in cmd/api/main.go injects the real engine after constructing it). A
// nil engine fails submit with a clear RULE_VIOLATION rather than panicking.
func (s *LeaveService) SetApprovalEngine(e approval.Engine) { s.engine = e }

// AdjustTypeQuota applies an HR per-type quota adjustment (LQ-6 / "Sesuaikan Kuota"):
// signed delta on the (employee, type, window) entitlement, audited. start selects
// the window by the type's cap_basis. Requires the per-type meter (production).
func (s *LeaveService) AdjustTypeQuota(ctx context.Context, employeeID, leaveTypeID string, start time.Time, delta int, reason string) (dom.LeaveQuota, error) {
	if s.meter == nil {
		return dom.LeaveQuota{}, apperr.Rule("RULE_VIOLATION", map[string]string{"meter": "Per-type ledger tidak aktif."})
	}
	actor := actorEmployeeID(ctx)
	actorStr := ""
	if actor != nil {
		actorStr = *actor
	}
	var out dom.LeaveQuota
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		q, aerr := s.meter.AdjustEntitled(ctx, tx, AdjustEntitledInput{
			EmployeeID: employeeID, LeaveTypeID: leaveTypeID, StartDate: start,
			Delta: delta, Actor: actorStr, Reason: reason, Now: s.now(),
		})
		if aerr != nil {
			return aerr
		}
		out = q
		return audit.Record(ctx, tx, leaveAudit(q.ID, "leave_quota", "", "", actor, "ADJUST_QUOTA"))
	})
	if err != nil {
		return dom.LeaveQuota{}, asAppErr(mapMeterErr(err))
	}
	return out, nil
}

// mapMeterErr maps a QuotaMeter GateError to the API error contract.
func mapMeterErr(err error) error {
	var ge *GateError
	if errors.As(err, &ge) {
		code := "RULE_VIOLATION"
		if ge.Reason == GateOverCap || ge.Reason == GateOverEventCap {
			code = "QUOTA_EXCEEDED"
		}
		return apperr.Rule(code, map[string]string{"leave_type_id": ge.Message})
	}
	return err
}

// NewLeaveService wires the leave service. The per-type QuotaMeter is attached via
// SetMeter (production + tests); the approval engine via SetApprovalEngine; the
// schedule port is the INV-3 loop-closer surface.
func NewLeaveService(repo LeaveRepository, schedule SchedulePort, txm TxRunner) *LeaveService {
	return &LeaveService{repo: repo, schedule: schedule, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *LeaveService) SetClock(c Clock) { s.now = c }

// SetNotifier wires the E10 notification dispatcher (11-02). Additive + nil-safe:
// notify.Dispatch no-ops when the notifier is nil, so existing unit tests that
// construct the service without it keep passing. main.go injects the River client.
func (s *LeaveService) SetNotifier(d jobs.Dispatcher) { s.notifier = d }

// --- list / get ---

// List returns the leave-request page. Leader scope is forced to their led company;
// an explicit company_id outside scope yields 403 OUT_OF_SCOPE.
func (s *LeaveService) List(ctx context.Context, f RequestFilter) ([]dom.LeaveRequest, *string, bool, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, nil, false, apperr.Unauthenticated()
	}
	// Agents see ONLY their own requests (SELF scope) — force the employee filter
	// regardless of any client-supplied employee_id.
	if p.Role == auth.RoleAgent {
		eid := p.EmployeeID
		f.EmployeeID = &eid
	}
	if p.Role == auth.RoleShiftLeader {
		if f.CompanyID != nil && *f.CompanyID != p.CompanyID {
			return nil, nil, false, apperr.OutOfScope()
		}
		cid := p.CompanyID
		f.CompanyID = &cid
	}
	limit := clampLimit(f.Limit)
	f.Limit = limit + 1
	rows, err := s.repo.ListLeaveRequests(ctx, f)
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c, cerr := encodeRequestCursor(last.CreatedAt, last.ID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// Get loads a single request; cross-scope reads are hidden as 404. The approval
// chain is read by the client from approval_instance_id (E11), not assembled here.
func (s *LeaveService) Get(ctx context.Context, id string) (dom.LeaveRequest, error) {
	rec, err := s.repo.GetLeaveRequest(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.LeaveRequest{}, apperr.NotFound()
	}
	if err != nil {
		return dom.LeaveRequest{}, apperr.Internal(err)
	}
	// Agent self-scope: another employee's request is hidden as 404 (no existence
	// leak). Staff keep the company GuardCompany scope.
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleAgent {
		if rec.EmployeeID != p.EmployeeID {
			return dom.LeaveRequest{}, apperr.NotFound()
		}
	} else if serr := rbac.GuardCompany(ctx, deref(rec.CompanyID)); serr != nil {
		return dom.LeaveRequest{}, apperr.NotFound()
	}
	return rec, nil
}

// --- E11 hooks (called by the approval engine on terminal transition, in its tx) ---

// OnApproved applies the leave-domain side-effects when the E11 instance reaches a
// terminal APPROVED/BYPASS_APPROVED decision. Runs INSIDE the engine's tx: it locks
// the request (FOR UPDATE), commits the per-type reservation (pending→used; over-cap
// recheck fails the tx → EX-9), fires the INV-3 loop-closer (cancel overlapping
// schedule entries + INSERT approved_leave_days), sets status=APPROVED + the balance
// snapshot, and dispatches NotifLeaveApproved on the transactional outbox.
//
// The engine owns the instance/line state — this hook only does leave-domain effects
// + sets the leave status. Returning an error rolls the engine's tx back.
func (s *LeaveService) OnApproved(ctx context.Context, tx pgx.Tx, requestID string) error {
	rec, lerr := s.repo.GetLeaveRequestForUpdate(ctx, tx, requestID)
	if errors.Is(lerr, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if lerr != nil {
		return lerr
	}
	// Idempotency: a re-fired hook on an already-terminal request is a no-op.
	if rec.Status == dom.LeaveStatusApproved {
		return nil
	}
	if rec.Status != dom.LeaveStatusPending {
		return stateConflict(rec.Status)
	}

	// 1. resolve the leave type (for the approved_leave_days code below).
	lt, lterr := s.repo.GetLeaveType(ctx, rec.LeaveTypeID)
	if lterr != nil && !errors.Is(lterr, domain.ErrNotFound) {
		return lterr
	}

	// 2. per-type meter: commit the SUBMIT-time reservation (pending→used) against the
	// cap_basis window. Over-cap recheck fails here (no override on the hook path —
	// a super-admin bypass is an E11 concern; the leave domain still rechecks quota).
	if cerr := s.meter.Commit(ctx, tx, CommitInput{
		EmployeeID: rec.EmployeeID, LeaveTypeID: rec.LeaveTypeID,
		StartDate: rec.StartDate, Days: rec.DurationDays, Override: false,
	}); cerr != nil {
		return mapMeterErr(cerr)
	}

	// 3. INV-3 loop-closer: cancel overlapping schedule entries (DB CANCELLED_BY_LEAVE)
	// + INSERT approved_leave_days for each leave date.
	cancels, cerr := s.schedule.CancelScheduleEntriesForLeave(ctx, tx, rec.EmployeeID, rec.StartDate, rec.EndDate)
	if cerr != nil {
		return cerr
	}
	leaveTypeCode := ""
	if lt.Code != "" {
		leaveTypeCode = lt.Code
	}
	for d := rec.StartDate; !d.After(rec.EndDate); d = d.AddDate(0, 0, 1) {
		if ierr := s.schedule.InsertApprovedLeaveDay(ctx, tx, rec.EmployeeID, d, requestID, leaveTypeCode); ierr != nil {
			return ierr
		}
	}
	_ = cancels // schedule_impact is recomputed at read time; not persisted here.

	// 4. transition + commit the BalanceCheck snapshot.
	req := rec.DurationDays
	if _, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
		ID:                   requestID,
		Status:               dom.LeaveStatusApproved,
		NoLeader:             rec.NoLeader,
		AssignedLeaderID:     rec.AssignedLeaderID,
		ClockInConflict:      rec.ClockInConflict,
		BalanceRequestedDays: &req,
	}); uerr != nil {
		return uerr
	}
	if serr := s.writeSnapshot(ctx, tx, requestID, &req, nil, boolPtr(false)); serr != nil {
		return serr
	}

	// 5. notify the submitter (transactional outbox — enqueued in THIS tx, so a
	// rolled-back approval never notifies).
	if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
		NotifKind:        string(reportingdom.NotifLeaveApproved),
		RecipientID:      rec.EmployeeID,
		Title:            "Cuti disetujui",
		Body:             leaveDateBody("Pengajuan cuti Anda disetujui", rec.StartDate, rec.EndDate),
		DeepLinkEpic:     "E6",
		DeepLinkEntityID: requestID,
		DeepLinkPath:     "/leave-requests/" + requestID,
		ActorID:          actorUserID(ctx),
		IsCritical:       true,
	}); derr != nil {
		return derr
	}
	return audit.Record(ctx, tx, leaveAudit(requestID, "leave_request", string(rec.Status), "APPROVED", actorEmployeeID(ctx), "APPROVE"))
}

// OnRejected applies the leave-domain side-effects when the E11 instance reaches a
// terminal REJECTED decision. Runs INSIDE the engine's tx: releases the SUBMIT-time
// pending reservation, sets status=REJECTED, and dispatches NotifLeaveRejected.
func (s *LeaveService) OnRejected(ctx context.Context, tx pgx.Tx, requestID string) error {
	rec, lerr := s.repo.GetLeaveRequestForUpdate(ctx, tx, requestID)
	if errors.Is(lerr, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if lerr != nil {
		return lerr
	}
	if rec.Status == dom.LeaveStatusRejected {
		return nil // idempotent
	}
	if rec.Status != dom.LeaveStatusPending {
		return stateConflict(rec.Status)
	}
	// Release the SUBMIT-time pending reservation (the request never consumed).
	if rerr := s.meter.Release(ctx, tx, WindowOp{
		EmployeeID: rec.EmployeeID, LeaveTypeID: rec.LeaveTypeID,
		StartDate: rec.StartDate, Days: rec.DurationDays,
	}); rerr != nil {
		return rerr
	}
	if _, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
		ID:               requestID,
		Status:           dom.LeaveStatusRejected,
		NoLeader:         rec.NoLeader,
		AssignedLeaderID: rec.AssignedLeaderID,
		ClockInConflict:  rec.ClockInConflict,
	}); uerr != nil {
		return uerr
	}
	if serr := s.writeSnapshot(ctx, tx, requestID, nil, nil, nil); serr != nil {
		return serr
	}
	if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
		NotifKind:        string(reportingdom.NotifLeaveRejected),
		RecipientID:      rec.EmployeeID,
		Title:            "Cuti ditolak",
		Body:             "Pengajuan cuti Anda ditolak.",
		DeepLinkEpic:     "E6",
		DeepLinkEntityID: requestID,
		DeepLinkPath:     "/leave-requests/" + requestID,
		ActorID:          actorUserID(ctx),
		IsCritical:       true,
	}); derr != nil {
		return derr
	}
	return audit.Record(ctx, tx, leaveAudit(requestID, "leave_request", string(rec.Status), "REJECTED", actorEmployeeID(ctx), "REJECT"))
}

// --- create (F6.2 agent file-a-request) ---

// CreateLeaveInput is the validated POST /leave-requests body (LeaveRequestWriteRequest).
// EmployeeID is optional on the wire: an agent omits it (server fills from the token);
// staff may target an employee. Submit defaults true (nil ⇒ submit) — the create-and-
// submit single-call path. DurationDays is NOT carried: it is always server-computed.
type CreateLeaveInput struct {
	LeaveTypeID    string
	StartDate      time.Time
	EndDate        time.Time
	Reason         string
	EmployeeID     *string
	DelegateID     *string
	DocumentFileID *string
	Submit         *bool
}

// Create files a leave request (F6.2). It resolves the principal (an agent may only
// file for themselves — 403 if EmployeeID is set to anyone else), loads the leave type,
// then in ONE tx validates in the contract order
// (INVALID_DATE_RANGE → MISSING_REQUIRED_DOCUMENT → OVERLAPPING_LEAVE → BACKDATED_LEAVE),
// computes duration_days via the server-authoritative calculator (rostered days minus
// public holidays), inserts the DRAFT, and — when submit (default true) — runs submitTx
// (reserve + create the E11 instance; QUOTA_EXCEEDED surfaces here). Returns the request.
func (s *LeaveService) Create(ctx context.Context, in CreateLeaveInput) (dom.LeaveRequest, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return dom.LeaveRequest{}, apperr.Unauthenticated()
	}

	// Resolve the target employee. An agent files only for themselves.
	employeeID := deref(in.EmployeeID)
	if p.Role == auth.RoleAgent {
		if in.EmployeeID != nil && *in.EmployeeID != "" && *in.EmployeeID != p.EmployeeID {
			return dom.LeaveRequest{}, apperr.Forbidden()
		}
		employeeID = p.EmployeeID
	}
	if employeeID == "" {
		return dom.LeaveRequest{}, apperr.Invalid(map[string]string{"employee_id": "Wajib diisi."})
	}

	lt, lterr := s.repo.GetLeaveType(ctx, in.LeaveTypeID)
	if errors.Is(lterr, domain.ErrNotFound) {
		return dom.LeaveRequest{}, apperr.Invalid(map[string]string{"leave_type_id": "Jenis cuti tidak ditemukan."})
	}
	if lterr != nil {
		return dom.LeaveRequest{}, apperr.Internal(lterr)
	}

	submit := true
	if in.Submit != nil {
		submit = *in.Submit
	}

	var out dom.LeaveRequest
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// 1. INVALID_DATE_RANGE (422) — end before start.
		if in.EndDate.Before(in.StartDate) {
			return apperr.Rule("INVALID_DATE_RANGE", map[string]string{"end_date": "Harus >= tanggal mulai."})
		}
		// 2. MISSING_REQUIRED_DOCUMENT (422) — document-required type without a doc.
		if lt.IsDocumentRequired && deref(in.DocumentFileID) == "" {
			return apperr.Rule("MISSING_REQUIRED_DOCUMENT", map[string]string{"document_file_id": "Wajib dilampirkan untuk jenis cuti ini."})
		}
		// 3. OVERLAPPING_LEAVE (409, LR-5) — a live leave overlaps the range.
		overlaps, oerr := s.repo.CheckOverlappingLeave(ctx, employeeID, in.StartDate, in.EndDate)
		if oerr != nil {
			return oerr
		}
		if overlaps {
			return apperr.Conflict("OVERLAPPING_LEAVE")
		}
		// 4. BACKDATED_LEAVE (422) — start before today and the type does not allow it.
		today := s.now().UTC().Truncate(24 * time.Hour)
		backdated := in.StartDate.Before(today)
		if backdated && !lt.AllowsBackdated {
			return apperr.Rule("BACKDATED_LEAVE", map[string]string{"requires_hr_override": "true"})
		}

		// 5. server-authoritative duration (rostered days minus public holidays).
		duration, derr := s.schedule.CountLeaveDuration(ctx, employeeID, in.StartDate, in.EndDate)
		if derr != nil {
			return derr
		}

		// 6. insert the DRAFT.
		created, cerr := s.repo.CreateLeaveRequest(ctx, tx, CreateLeaveRequestParams{
			EmployeeID:     employeeID,
			LeaveTypeID:    in.LeaveTypeID,
			StartDate:      in.StartDate,
			EndDate:        in.EndDate,
			DurationDays:   duration,
			Reason:         strOrNil(in.Reason),
			Status:         dom.LeaveStatusDraft,
			DelegateID:     strptrNil(in.DelegateID),
			DocumentFileID: strptrNil(in.DocumentFileID),
			Backdated:      backdated,
			CreatedBy:      strOrNil(actorUserID(ctx)),
		})
		if cerr != nil {
			return cerr
		}
		out = created

		// 7. audit the create.
		if aerr := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "leave_request",
			EntityID:   created.ID,
			After: map[string]any{
				"employee_id": created.EmployeeID, "leave_type_id": created.LeaveTypeID,
				"start_date": created.StartDate.Format("2006-01-02"), "end_date": created.EndDate.Format("2006-01-02"),
				"duration_days": created.DurationDays, "status": string(created.Status),
			},
		}); aerr != nil {
			return aerr
		}

		// 8. submit (default true): reserve → PENDING → create the E11 instance.
		if submit {
			submitted, serr := s.submitTx(ctx, tx, created)
			if serr != nil {
				return serr
			}
			out = submitted
		}
		return nil
	})
	if err != nil {
		return dom.LeaveRequest{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// --- submit (reserve + create the E11 instance) ---

// Submit transitions DRAFT → PENDING: it reserves the duration against the per-type
// window, sets status=PENDING, and creates the E11 ApprovalInstance (the engine then
// drives the chain). Insufficient balance → QUOTA_EXCEEDED.
func (s *LeaveService) Submit(ctx context.Context, id string) (dom.LeaveRequest, error) {
	var out dom.LeaveRequest
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lockRequest(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if serr := guardSelfOwn(ctx, rec); serr != nil {
			return serr
		}
		updated, serr := s.submitTx(ctx, tx, rec)
		if serr != nil {
			return serr
		}
		out = updated
		return nil
	})
	if err != nil {
		return dom.LeaveRequest{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// submitTx is the shared DRAFT → PENDING reservation + instance-creation core, called
// from both the public Submit (after lock + self-guard) and Create (with submit=true,
// on the freshly-inserted DRAFT). It state-checks DRAFT, reserves the duration for
// quota-tracked types (QUOTA_EXCEEDED on insufficient balance), transitions to
// PENDING, creates the E11 ApprovalInstance, links it on the request, and audits —
// all inside the caller's tx. Returns the updated row.
func (s *LeaveService) submitTx(ctx context.Context, tx pgx.Tx, rec dom.LeaveRequest) (dom.LeaveRequest, error) {
	actor := actorEmployeeID(ctx)
	id := rec.ID
	if rec.Status != dom.LeaveStatusDraft {
		return dom.LeaveRequest{}, stateConflict(rec.Status)
	}
	if s.engine == nil {
		return dom.LeaveRequest{}, apperr.Rule("RULE_VIOLATION", map[string]string{"approval": "Approval engine tidak aktif."})
	}
	// Per-type meter: reserve the duration against the type's cap_basis window
	// (auto-opens a quota-bearing window if absent). QUOTA_EXCEEDED / gate failures
	// surface here. PER_EVENT / UNCAPPED types reserve no window.
	if _, merr := s.meter.Reserve(ctx, tx, ReserveInput{
		EmployeeID: rec.EmployeeID, LeaveTypeID: rec.LeaveTypeID,
		Days: rec.DurationDays, StartDate: rec.StartDate, Now: s.now(),
	}); merr != nil {
		return dom.LeaveRequest{}, mapMeterErr(merr)
	}

	updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
		ID:               id,
		Status:           dom.LeaveStatusPending,
		NoLeader:         rec.NoLeader,
		AssignedLeaderID: rec.AssignedLeaderID,
		ClockInConflict:  rec.ClockInConflict,
	})
	if uerr != nil {
		return dom.LeaveRequest{}, uerr
	}
	req := rec.DurationDays
	if serr := s.writeSnapshot(ctx, tx, id, &req, nil, boolPtr(false)); serr != nil {
		return dom.LeaveRequest{}, serr
	}

	// Create the E11 ApprovalInstance for this request and link it.
	instanceID, ierr := s.engine.CreateInstance(ctx, tx, approval.CreateInstanceInput{
		RequestType: approval.RequestTypeLeave,
		RequestID:   id,
		CompanyID:   deref(rec.CompanyID),
		RequesterID: rec.EmployeeID,
	})
	if ierr != nil {
		return dom.LeaveRequest{}, ierr
	}
	if lerr := s.repo.SetApprovalInstanceID(ctx, tx, id, instanceID); lerr != nil {
		return dom.LeaveRequest{}, lerr
	}
	updated.ApprovalInstanceID = &instanceID

	if aerr := audit.Record(ctx, tx, leaveAudit(id, "leave_request", string(rec.Status), string(dom.LeaveStatusPending), actor, "SUBMIT")); aerr != nil {
		return dom.LeaveRequest{}, aerr
	}
	return updated, nil
}

// --- cancel (withdraw a not-yet-approved request) ---

// Cancel withdraws a DRAFT/PENDING request (LR-7). Releases any pending reservation;
// no schedule effect (the request never reached APPROVED).
func (s *LeaveService) Cancel(ctx context.Context, id, reason string) (dom.LeaveRequest, error) {
	actor := actorEmployeeID(ctx)
	var out dom.LeaveRequest
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lockRequest(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		switch rec.Status {
		case dom.LeaveStatusDraft, dom.LeaveStatusPending:
		default:
			return stateConflict(rec.Status)
		}
		if serr := guardSelfOwn(ctx, rec); serr != nil {
			return serr
		}
		if rerr := s.meter.Release(ctx, tx, WindowOp{
			EmployeeID: rec.EmployeeID, LeaveTypeID: rec.LeaveTypeID,
			StartDate: rec.StartDate, Days: rec.DurationDays,
		}); rerr != nil {
			return rerr
		}
		updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
			ID:               id,
			Status:           dom.LeaveStatusCancelled,
			NoLeader:         rec.NoLeader,
			AssignedLeaderID: rec.AssignedLeaderID,
			ClockInConflict:  rec.ClockInConflict,
		})
		if uerr != nil {
			return uerr
		}
		out = updated
		if serr := s.writeSnapshot(ctx, tx, id, nil, nil, nil); serr != nil {
			return serr
		}
		return audit.Record(ctx, tx, leaveAudit(id, "leave_request", string(rec.Status), "CANCELLED", actor, "CANCEL"))
	})
	if err != nil {
		return dom.LeaveRequest{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// --- cancel-approved (LI-4 full restore) ---

// CancelApproved cancels an APPROVED leave and reverses the EXACT consumption rows it
// drew (consumed -= days on each lot + delete the rows). Agents may cancel only their
// own future leave (start_date > today); HR may cancel any. Schedule restore is the
// leader's E4 re-roster (out of v1 scope here).
func (s *LeaveService) CancelApproved(ctx context.Context, id, reason string) (dom.LeaveRequest, error) {
	if len([]rune(reason)) < 5 {
		return dom.LeaveRequest{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."})
	}
	actor := actorEmployeeID(ctx)
	var out dom.LeaveRequest
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lockRequest(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if rec.Status != dom.LeaveStatusApproved {
			return stateConflict(rec.Status)
		}
		if perr := s.guardCancelApprovedActor(ctx, rec); perr != nil {
			return perr
		}
		if rerr := s.meter.Reverse(ctx, tx, WindowOp{
			EmployeeID: rec.EmployeeID, LeaveTypeID: rec.LeaveTypeID,
			StartDate: rec.StartDate, Days: rec.DurationDays,
		}); rerr != nil {
			return rerr
		}
		updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
			ID:               id,
			Status:           dom.LeaveStatusCancelled,
			NoLeader:         rec.NoLeader,
			AssignedLeaderID: rec.AssignedLeaderID,
			ClockInConflict:  rec.ClockInConflict,
		})
		if uerr != nil {
			return uerr
		}
		out = updated
		if serr := s.writeSnapshot(ctx, tx, id, nil, nil, nil); serr != nil {
			return serr
		}
		return audit.Record(ctx, tx, leaveAudit(id, "leave_request", "APPROVED", "CANCELLED", actor, "CANCEL_APPROVED"))
	})
	if err != nil {
		return dom.LeaveRequest{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// --- shorten (LI-4 partial restore) ---

// Shorten sets a new (earlier) end_date on an APPROVED leave and reverses the days
// between new_end_date+1 and the original end_date. Implemented as a reverse-then-
// re-commit of the shortened duration against the original lots (FIFO), so the lot
// consumed bookkeeping stays exact. Cannot shorten past start_date (use CancelApproved).
func (s *LeaveService) Shorten(ctx context.Context, id string, newEnd time.Time, reason string) (dom.LeaveRequest, error) {
	if len([]rune(reason)) < 5 {
		return dom.LeaveRequest{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."})
	}
	actor := actorEmployeeID(ctx)
	var out dom.LeaveRequest
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lockRequest(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if rec.Status != dom.LeaveStatusApproved {
			return stateConflict(rec.Status)
		}
		if newEnd.Before(rec.StartDate) || !newEnd.Before(rec.EndDate) {
			return apperr.Rule("RULE_VIOLATION", map[string]string{"new_end_date": "Harus >= tanggal mulai dan < tanggal selesai."})
		}
		// New duration = whole-day count [start, newEnd] inclusive (server-authoritative
		// roster-minus-holiday recomputation is the E4/E7 concern; here we proportionally
		// reduce by calendar days as the v1 approximation).
		newDays := int(newEnd.Sub(rec.StartDate).Hours()/24) + 1
		if newDays < 1 {
			newDays = 1
		}
		// Reverse the full window charge, then re-commit newDays (override: a shorten
		// never over-caps relative to the already-approved charge).
		if rerr := s.meter.Reverse(ctx, tx, WindowOp{
			EmployeeID: rec.EmployeeID, LeaveTypeID: rec.LeaveTypeID,
			StartDate: rec.StartDate, Days: rec.DurationDays,
		}); rerr != nil {
			return rerr
		}
		if cerr := s.meter.Commit(ctx, tx, CommitInput{
			EmployeeID: rec.EmployeeID, LeaveTypeID: rec.LeaveTypeID,
			StartDate: rec.StartDate, Days: newDays, Override: true,
		}); cerr != nil {
			return mapMeterErr(cerr)
		}
		updated, uerr := s.repo.UpdateLeaveRequestDates(ctx, tx, id, rec.StartDate, newEnd, newDays)
		if uerr != nil {
			return uerr
		}
		out = updated
		req := newDays
		if serr := s.writeSnapshot(ctx, tx, id, &req, nil, boolPtr(false)); serr != nil {
			return serr
		}
		return audit.Record(ctx, tx, leaveAudit(id, "leave_request", "APPROVED", "APPROVED", actor, "SHORTEN"))
	})
	if err != nil {
		return dom.LeaveRequest{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// guardCancelApprovedActor enforces the cancel-approved permission split (openapi):
// an agent may cancel ONLY own future leave (start_date > today); HR/super may cancel
// any. Other roles are blocked.
func (s *LeaveService) guardCancelApprovedActor(ctx context.Context, rec dom.LeaveRequest) error {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apperr.Unauthenticated()
	}
	switch p.Role {
	case auth.RoleHRAdmin, auth.RoleSuperAdmin:
		return nil
	case auth.RoleAgent:
		if p.EmployeeID != rec.EmployeeID {
			return apperr.Forbidden()
		}
		today := s.now().UTC().Truncate(24 * time.Hour)
		if !rec.StartDate.After(today) {
			return apperr.Rule("RULE_VIOLATION", map[string]string{"start_date": "Cuti sudah dimulai; hubungi HR."})
		}
		return nil
	default:
		return apperr.Forbidden()
	}
}

// guardSelfOwn enforces that an agent acts only on their own request (cancel/withdraw).
func guardSelfOwn(ctx context.Context, rec dom.LeaveRequest) error {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleAgent && p.EmployeeID != rec.EmployeeID {
		return apperr.Forbidden()
	}
	return nil
}

// --- helpers ---

func (s *LeaveService) lockRequest(ctx context.Context, tx pgx.Tx, id string) (dom.LeaveRequest, error) {
	rec, err := s.repo.GetLeaveRequestForUpdate(ctx, tx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.LeaveRequest{}, apperr.NotFound()
	}
	if err != nil {
		return dom.LeaveRequest{}, err
	}
	return rec, nil
}

// reread re-loads the request with denormalized names for the DTO.
func (s *LeaveService) reread(ctx context.Context, fallback dom.LeaveRequest) (dom.LeaveRequest, error) {
	if full, err := s.repo.GetLeaveRequest(ctx, fallback.ID); err == nil {
		return full, nil
	}
	return fallback, nil
}

// stateConflict is the 409 for a wrong/terminal-state transition (carries status).
func stateConflict(cur dom.LeaveStatus) error {
	return &apperr.Error{
		Code:       "CONFLICT",
		HTTPStatus: 409,
		Message:    "Pengajuan cuti sudah pada status lain.",
		Fields:     map[string]string{"status": string(cur)},
	}
}

func leaveAudit(id, entity, before, after string, actor *string, action string) audit.Entry {
	return audit.Entry{
		Action:     audit.ActionUpdate,
		EntityType: entity,
		EntityID:   id,
		Before:     map[string]any{"status": before},
		After:      map[string]any{"status": after, "decided_by": ptrStr(actor), "decision": action},
	}
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// strptrNil normalizes an optional string pointer: nil or empty → nil.
func strptrNil(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func boolPtr(b bool) *bool { return &b }

// writeSnapshot persists the openapi BalanceCheck snapshot (requested/remaining/
// requires_override) on the request. Under the per-type ledger there is no per-lot
// allocation or earmark split, so those snapshot columns are left null.
func (s *LeaveService) writeSnapshot(ctx context.Context, tx pgx.Tx, id string, requested, remaining *int, requiresOverride *bool) error {
	return s.repo.SetBalanceSnapshot(ctx, tx, BalanceSnapshotParams{
		ID:               id,
		RequestedDays:    requested,
		RemainingAtCheck: remaining,
		RequiresOverride: requiresOverride,
	})
}
