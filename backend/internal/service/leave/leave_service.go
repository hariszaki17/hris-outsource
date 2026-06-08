// Package leave — LeaveService: the two-level approval state machine
// (PENDING_L1 → PENDING_HR → APPROVED; reject → REJECTED), the balance re-check +
// quota deduct, and the INV-3 loop-closer (cancel overlapping schedule_entries +
// INSERT approved_leave_days in the same tx). Every action guards state
// (*ForUpdate → 409 on terminal/wrong-state), scope (GuardCompany → 403
// OUT_OF_SCOPE), and self-approval (403 FORBIDDEN), then audits in-tx + stubs the
// notification (TODO Phase-11). Mirrors the Phase-7 attendance state machine.
package leave

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	reportingdom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// LeaveService implements the leave-request approval business logic.
type LeaveService struct {
	repo     LeaveRepository
	grants   *GrantService
	gr       GrantRepository
	schedule SchedulePort
	txm      TxRunner
	now      Clock
	notifier jobs.Dispatcher // E10 (11-02): transactional-outbox notify seam (nil-safe in unit tests)
}

// NewLeaveService wires the leave service. grants is the grant-lot allocator (FIFO
// reserve/commit/release/reverse); the schedule port is the INV-3 loop-closer surface.
func NewLeaveService(repo LeaveRepository, grants *GrantService, schedule SchedulePort, txm TxRunner) *LeaveService {
	return &LeaveService{repo: repo, grants: grants, gr: grants.repo, schedule: schedule, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only). Propagates to the allocator so FIFO
// active-lot selection uses the same clock.
func (s *LeaveService) SetClock(c Clock) {
	s.now = c
	if s.grants != nil {
		s.grants.SetClock(c)
	}
}

// LeaveTypeInfo gained an Earmark field (the purpose code) so the allocator can filter
// to matching lots. earmarkForType maps a leave type to its earmark purpose: an
// earmarked type (MATERNITY / STATUTORY) draws ONLY matching lots (LQ-10); an ordinary
// type draws the flat pool (earmark nil).
func earmarkForType(lt LeaveTypeInfo) *string {
	switch lt.Earmark {
	case "":
		return nil
	default:
		e := lt.Earmark
		return &e
	}
}

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
	// Attach the timeline to each row for the queue (best-effort).
	for i := range rows {
		rows[i].Timeline = s.buildTimeline(ctx, rows[i])
	}
	return rows, next, hasMore, nil
}

// Get loads a single request with its timeline; cross-scope reads are hidden as 404.
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
	rec.Timeline = s.buildTimeline(ctx, rec)
	return rec, nil
}

// --- approve-l1 ---

// ApproveL1 forwards PENDING_L1 → PENDING_HR. Guards: state (409), scope (403
// OUT_OF_SCOPE), self-approve (403 FORBIDDEN). No quota deducted yet (LA-4).
func (s *LeaveService) ApproveL1(ctx context.Context, id, note string) (dom.LeaveRequest, error) {
	actor := actorEmployeeID(ctx)
	role := actorRole(ctx)
	var out dom.LeaveRequest
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lockRequest(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if rec.Status != dom.LeaveStatusPendingL1 {
			return stateConflict(rec.Status)
		}
		if serr := rbac.GuardCompany(ctx, deref(rec.CompanyID)); serr != nil {
			return serr
		}
		if serr := guardSelf(ctx, rec); serr != nil {
			return serr
		}
		updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
			ID:               id,
			Status:           dom.LeaveStatusPendingHR,
			NoLeader:         rec.Routing.NoLeader,
			AssignedLeaderID: rec.Routing.AssignedLeaderID,
			ClockInConflict:  rec.ClockInConflict,
		})
		if uerr != nil {
			return uerr
		}
		out = updated
		notePtr := strOrNil(note)
		if _, aerr := s.repo.InsertLeaveApproval(ctx, tx, ApprovalRow{
			LeaveRequestID: id, Stage: dom.StageL1, Decision: dom.DecisionApproved,
			ActorID: actor, ActorRole: role, DecisionNote: notePtr,
		}); aerr != nil {
			return aerr
		}
		// Phase-11 stub (documented): the L1→HR next-stage event targets the HR
		// approver QUEUE, not a single recipient — there is no clean per-user
		// recipient at this point (the HR pool is not enumerated here). Left as a
		// stub per the plan's OPTIONAL coverage; the submitter-facing
		// approve-final/reject points ARE wired below. See 11-02-SUMMARY.
		return audit.Record(ctx, tx, leaveAudit(id, "leave_request", string(rec.Status), "PENDING_HR", actor, "APPROVE_L1"))
	})
	if err != nil {
		return dom.LeaveRequest{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// --- approve-final ---

// ApproveFinal moves PENDING_HR → APPROVED. Re-checks balance (LA-5): an annual
// over-balance returns 422 BALANCE_RECHECK_FAILED(requires_override). On success it
// deducts the quota and fires the INV-3 side effects, all in one tx.
func (s *LeaveService) ApproveFinal(ctx context.Context, id, note string) (dom.LeaveRequest, error) {
	return s.finalize(ctx, id, note, false, "")
}

// ApproveOverride force-approves PENDING_HR even over-balance (LA-8). override_reason
// (min 10) is recorded on the timeline + the quota's last_override. Skips the
// balance block; still deducts + fires INV-3.
func (s *LeaveService) ApproveOverride(ctx context.Context, id, overrideReason string) (dom.LeaveRequest, error) {
	if len([]rune(overrideReason)) < 10 {
		return dom.LeaveRequest{}, apperr.Invalid(map[string]string{"override_reason": "Wajib diisi (minimum 10 karakter)."})
	}
	return s.finalize(ctx, id, "", true, overrideReason)
}

// finalize is the shared PENDING_HR → APPROVED path for approve-final / override.
func (s *LeaveService) finalize(ctx context.Context, id, note string, override bool, overrideReason string) (dom.LeaveRequest, error) {
	actor := actorEmployeeID(ctx)
	role := actorRole(ctx)
	var out dom.LeaveRequest
	var impact []dom.ScheduleImpactEntry
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lockRequest(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if rec.Status != dom.LeaveStatusPendingHR {
			return stateConflict(rec.Status)
		}
		// HR/super only — RequireRole already gates the route; defense-in-depth here.
		if serr := guardSelf(ctx, rec); serr != nil {
			return serr
		}

		// 1. resolve the leave type (quota-tracked gate + earmark purpose).
		lt, lterr := s.repo.GetLeaveType(ctx, rec.LeaveTypeID)
		if lterr != nil && !errors.Is(lterr, domain.ErrNotFound) {
			return lterr
		}
		quotaTracked := lt.IsAnnual
		earmark := earmarkForType(lt)

		// 2. grant-lot allocation: commit the SUBMIT-time reservation, or allocate
		// fresh at approve when no reservation was held (seeded / HR-on-behalf).
		var remainingAtCheck *int
		var committed []dom.AllocationLine
		if quotaTracked {
			alloc := rec.BalanceCheck.Allocation
			if len(alloc) > 0 {
				// Commit the reserved lots (pending→consumed + LeaveConsumption rows).
				if cerr := s.grants.commit(ctx, tx, id, alloc); cerr != nil {
					return cerr
				}
				committed = alloc
			} else {
				// No prior reservation — FIFO allocate fresh at approve time.
				fresh, available, aerr := s.grants.allocate(ctx, tx, rec.EmployeeID, earmark, rec.DurationDays)
				if aerr != nil {
					return aerr
				}
				rem := available
				remainingAtCheck = &rem
				if !override && rec.DurationDays > available {
					return balanceRecheckFailed(rec.DurationDays, available)
				}
				// Reserve then commit each freshly-allocated lot (keeps the lot's
				// pending/consumed bookkeeping consistent). Over-balance overrides commit
				// only the available split — lots never go negative (LQ-5); the shortfall
				// is HR's to pre-fund via POST /leave-grants.
				for _, a := range fresh {
					if rerr := s.gr.ReservePending(ctx, tx, a.GrantID, a.Days); rerr != nil {
						return rerr
					}
				}
				if cerr := s.grants.commit(ctx, tx, id, fresh); cerr != nil {
					return cerr
				}
				committed = fresh
			}
		}
		_ = remainingAtCheck

		// 3. INV-3 loop-closer: cancel overlapping schedule entries (DB
		// CANCELLED_BY_LEAVE) + INSERT approved_leave_days for each leave date.
		cancels, cerr := s.schedule.CancelScheduleEntriesForLeave(ctx, tx, rec.EmployeeID, rec.StartDate, rec.EndDate)
		if cerr != nil {
			return cerr
		}
		impact = make([]dom.ScheduleImpactEntry, 0, len(cancels))
		for _, c := range cancels {
			impact = append(impact, dom.ScheduleImpactEntry{
				ScheduleID:  c.ScheduleID,
				Date:        c.Date.Format("2006-01-02"),
				PriorStatus: "PUBLISHED",
				NewStatus:   dtoNewStatus(c.NewStatus), // CANCELLED_BY_LEAVE → LEAVE
			})
		}
		leaveTypeCode := ""
		if lt.Code != "" {
			leaveTypeCode = lt.Code
		}
		for d := rec.StartDate; !d.After(rec.EndDate); d = d.AddDate(0, 0, 1) {
			if ierr := s.schedule.InsertApprovedLeaveDay(ctx, tx, rec.EmployeeID, d, id, leaveTypeCode); ierr != nil {
				return ierr
			}
		}

		// 4. transition + commit the BalanceCheck snapshot (committed allocation).
		req := int(rec.DurationDays)
		updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
			ID:                      id,
			Status:                  dom.LeaveStatusApproved,
			NoLeader:                rec.Routing.NoLeader,
			AssignedLeaderID:        rec.Routing.AssignedLeaderID,
			ClockInConflict:         rec.ClockInConflict,
			BalanceRequestedDays:    &req,
			BalanceRemainingAtCheck: remainingAtCheck,
			BalanceRequiresOverride: boolPtr(override),
		})
		if uerr != nil {
			return uerr
		}
		out = updated
		if serr := s.writeSnapshot(ctx, tx, id, &req, remainingAtCheck, boolPtr(override), earmark, committed); serr != nil {
			return serr
		}

		// 5. decision row + audit.
		decision := dom.DecisionApproved
		var ovReason *string
		if override {
			decision = dom.DecisionOverrideApproved
			ovReason = &overrideReason
		}
		if _, aerr := s.repo.InsertLeaveApproval(ctx, tx, ApprovalRow{
			LeaveRequestID: id, Stage: dom.StageHR, Decision: decision,
			ActorID: actor, ActorRole: role, DecisionNote: strOrNil(note),
			IsOverride: override, OverrideReason: ovReason,
		}); aerr != nil {
			return aerr
		}
		action := "APPROVE_FINAL"
		if override {
			action = "APPROVE_OVERRIDE"
		}
		// E10 (11-02): notify the submitter their leave is approved (transactional
		// outbox — enqueued in THIS tx, so a rolled-back approval never notifies).
		if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
			NotifKind:        string(reportingdom.NotifLeaveApproved),
			RecipientID:      rec.EmployeeID,
			Title:            "Cuti disetujui",
			Body:             leaveDateBody("Pengajuan cuti Anda disetujui", rec.StartDate, rec.EndDate),
			DeepLinkEpic:     "E6",
			DeepLinkEntityID: id,
			DeepLinkPath:     "/leave-requests/" + id,
			ActorID:          actorUserID(ctx),
			IsCritical:       true,
		}); derr != nil {
			return derr
		}
		return audit.Record(ctx, tx, leaveAudit(id, "leave_request", string(rec.Status), "APPROVED", actor, action))
	})
	if err != nil {
		return dom.LeaveRequest{}, asAppErr(err)
	}
	full, rerr := s.reread(ctx, out)
	if rerr != nil {
		return dom.LeaveRequest{}, rerr
	}
	full.ScheduleImpact = impact
	return full, nil
}

// --- reject ---

// Reject moves PENDING_L1 / PENDING_HR → REJECTED (reason required, min 5). The
// recorded stage matches the rejector's level. Terminal → 409; self-reject → 403.
func (s *LeaveService) Reject(ctx context.Context, id, reason string) (dom.LeaveRequest, error) {
	if len([]rune(reason)) < 5 {
		return dom.LeaveRequest{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."})
	}
	actor := actorEmployeeID(ctx)
	role := actorRole(ctx)
	var out dom.LeaveRequest
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lockRequest(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if rec.Status != dom.LeaveStatusPendingL1 && rec.Status != dom.LeaveStatusPendingHR {
			return stateConflict(rec.Status)
		}
		if serr := rbac.GuardCompany(ctx, deref(rec.CompanyID)); serr != nil {
			return serr
		}
		if serr := guardSelf(ctx, rec); serr != nil {
			return serr
		}
		stage := dom.StageHR
		if rec.Status == dom.LeaveStatusPendingL1 {
			stage = dom.StageL1
		}
		// Release the SUBMIT-time pending reservation (the request never consumed).
		if rerr := s.grants.release(ctx, tx, rec.BalanceCheck.Allocation); rerr != nil {
			return rerr
		}
		updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
			ID:               id,
			Status:           dom.LeaveStatusRejected,
			NoLeader:         rec.Routing.NoLeader,
			AssignedLeaderID: rec.Routing.AssignedLeaderID,
			ClockInConflict:  rec.ClockInConflict,
		})
		if uerr != nil {
			return uerr
		}
		out = updated
		if serr := s.writeSnapshot(ctx, tx, id, nil, nil, nil, nil, nil); serr != nil {
			return serr
		}
		rsn := reason
		if _, aerr := s.repo.InsertLeaveApproval(ctx, tx, ApprovalRow{
			LeaveRequestID: id, Stage: stage, Decision: dom.DecisionRejected,
			ActorID: actor, ActorRole: role, RejectReason: &rsn,
		}); aerr != nil {
			return aerr
		}
		// E10 (11-02): notify the submitter their leave is rejected.
		if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
			NotifKind:        string(reportingdom.NotifLeaveRejected),
			RecipientID:      rec.EmployeeID,
			Title:            "Cuti ditolak",
			Body:             "Pengajuan cuti Anda ditolak: " + reason,
			DeepLinkEpic:     "E6",
			DeepLinkEntityID: id,
			DeepLinkPath:     "/leave-requests/" + id,
			ActorID:          actorUserID(ctx),
			IsCritical:       true,
		}); derr != nil {
			return derr
		}
		return audit.Record(ctx, tx, leaveAudit(id, "leave_request", string(rec.Status), "REJECTED", actor, "REJECT"))
	})
	if err != nil {
		return dom.LeaveRequest{}, asAppErr(err)
	}
	return s.reread(ctx, out)
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
// (FIFO reservation; QUOTA_EXCEEDED surfaces here). Returns the full request.
// TODO: attachment upload + edit-draft + document-required leave types — the
// attachment endpoint is deferred; document_file_id stays optional on the wire but
// MISSING_REQUIRED_DOCUMENT is still enforced for document-required types.
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

		// 8. submit (default true): reserve → transition → QUOTA_EXCEEDED surfaces here.
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

// --- submit (reserve) ---

// Submit transitions DRAFT → PENDING_L1 (or PENDING_HR when no leader) and FIFO-
// reserves the duration across the employee's active matching-earmark lots, persisting
// the BalanceCheck allocation snapshot. Insufficient balance → QUOTA_EXCEEDED (HR
// pre-funds a lot instead of going negative). Quota-untracked types skip reservation.
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

// submitTx is the shared DRAFT → PENDING_L1/PENDING_HR reservation core, called from
// both the public Submit (after lock + self-guard) and Create (with submit=true, on
// the freshly-inserted DRAFT). It state-checks DRAFT, FIFO-reserves the duration for
// quota-tracked types (QUOTA_EXCEEDED on insufficient balance), writes the snapshot,
// transitions, and audits — all inside the caller's tx. Returns the updated row.
func (s *LeaveService) submitTx(ctx context.Context, tx pgx.Tx, rec dom.LeaveRequest) (dom.LeaveRequest, error) {
	actor := actorEmployeeID(ctx)
	id := rec.ID
	if rec.Status != dom.LeaveStatusDraft {
		return dom.LeaveRequest{}, stateConflict(rec.Status)
	}
	lt, lterr := s.repo.GetLeaveType(ctx, rec.LeaveTypeID)
	if lterr != nil && !errors.Is(lterr, domain.ErrNotFound) {
		return dom.LeaveRequest{}, lterr
	}
	earmark := earmarkForType(lt)

	var alloc []dom.AllocationLine
	var availPtr *int
	if lt.IsAnnual {
		a, available, rerr := s.grants.reserve(ctx, tx, rec.EmployeeID, earmark, rec.DurationDays)
		if rerr != nil {
			return dom.LeaveRequest{}, rerr
		}
		alloc = a
		availPtr = &available
	}

	next := dom.LeaveStatusPendingL1
	if rec.Routing.NoLeader {
		next = dom.LeaveStatusPendingHR
	}
	updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
		ID:               id,
		Status:           next,
		NoLeader:         rec.Routing.NoLeader,
		AssignedLeaderID: rec.Routing.AssignedLeaderID,
		ClockInConflict:  rec.ClockInConflict,
	})
	if uerr != nil {
		return dom.LeaveRequest{}, uerr
	}
	req := rec.DurationDays
	if serr := s.writeSnapshot(ctx, tx, id, &req, availPtr, boolPtr(false), earmark, alloc); serr != nil {
		return dom.LeaveRequest{}, serr
	}
	if aerr := audit.Record(ctx, tx, leaveAudit(id, "leave_request", string(rec.Status), string(next), actor, "SUBMIT")); aerr != nil {
		return dom.LeaveRequest{}, aerr
	}
	return updated, nil
}

// --- cancel (withdraw a not-yet-approved request) ---

// Cancel withdraws a DRAFT/PENDING_* request (LR-7). Releases any pending reservation;
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
		case dom.LeaveStatusDraft, dom.LeaveStatusPendingL1, dom.LeaveStatusPendingHR:
		default:
			return stateConflict(rec.Status)
		}
		if serr := guardSelfOwn(ctx, rec); serr != nil {
			return serr
		}
		if rerr := s.grants.release(ctx, tx, rec.BalanceCheck.Allocation); rerr != nil {
			return rerr
		}
		updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
			ID:               id,
			Status:           dom.LeaveStatusCancelled,
			NoLeader:         rec.Routing.NoLeader,
			AssignedLeaderID: rec.Routing.AssignedLeaderID,
			ClockInConflict:  rec.ClockInConflict,
		})
		if uerr != nil {
			return uerr
		}
		out = updated
		if serr := s.writeSnapshot(ctx, tx, id, nil, nil, nil, nil, nil); serr != nil {
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
		if _, rerr := s.grants.reverseConsumptions(ctx, tx, id); rerr != nil {
			return rerr
		}
		updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
			ID:               id,
			Status:           dom.LeaveStatusCancelled,
			NoLeader:         rec.Routing.NoLeader,
			AssignedLeaderID: rec.Routing.AssignedLeaderID,
			ClockInConflict:  rec.ClockInConflict,
		})
		if uerr != nil {
			return uerr
		}
		out = updated
		if serr := s.writeSnapshot(ctx, tx, id, nil, nil, nil, nil, nil); serr != nil {
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
		// Reverse the full consumption, then re-commit newDays FIFO from the same lots.
		if _, rerr := s.grants.reverseConsumptions(ctx, tx, id); rerr != nil {
			return rerr
		}
		lt, lterr := s.repo.GetLeaveType(ctx, rec.LeaveTypeID)
		if lterr != nil && !errors.Is(lterr, domain.ErrNotFound) {
			return lterr
		}
		earmark := earmarkForType(lt)
		var committed []dom.AllocationLine
		if lt.IsAnnual && newDays > 0 {
			fresh, _, aerr := s.grants.allocate(ctx, tx, rec.EmployeeID, earmark, newDays)
			if aerr != nil {
				return aerr
			}
			for _, a := range fresh {
				if perr := s.gr.ReservePending(ctx, tx, a.GrantID, a.Days); perr != nil {
					return perr
				}
			}
			if cerr := s.grants.commit(ctx, tx, id, fresh); cerr != nil {
				return cerr
			}
			committed = fresh
		}
		updated, uerr := s.repo.UpdateLeaveRequestDates(ctx, tx, id, rec.StartDate, newEnd, newDays)
		if uerr != nil {
			return uerr
		}
		out = updated
		req := newDays
		if serr := s.writeSnapshot(ctx, tx, id, &req, nil, boolPtr(false), earmark, committed); serr != nil {
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

// buildTimeline assembles the FE timeline[] from the leave_approvals rows plus an
// implicit leading "pending" marker. No-leader requests have NO L1 entry (the
// collapsed-timeline variant); the first marker is then HR-stage.
func (s *LeaveService) buildTimeline(ctx context.Context, rec dom.LeaveRequest) []dom.LeaveTimelineEntry {
	rows, err := s.repo.ListLeaveApprovalsForRequest(ctx, rec.ID)
	if err != nil {
		return nil
	}
	out := make([]dom.LeaveTimelineEntry, 0, len(rows)+1)
	// implicit submitted/pending marker at the first stage (HR if no-leader, else L1).
	firstStage := dom.StageL1
	if rec.Routing.NoLeader {
		firstStage = dom.StageHR
	}
	hasFirst := false
	for _, r := range rows {
		if r.Stage == firstStage {
			hasFirst = true
		}
	}
	if !hasFirst && (rec.Status == dom.LeaveStatusPendingL1 || rec.Status == dom.LeaveStatusPendingHR) {
		out = append(out, dom.LeaveTimelineEntry{
			Stage:      firstStage,
			Status:     dom.TimelineStatusPending,
			OccurredAt: rec.CreatedAt,
		})
	}
	for _, r := range rows {
		out = append(out, dom.LeaveTimelineEntry{
			Stage:          r.Stage,
			Status:         timelineStatus(r.Decision),
			ActorID:        r.ActorID,
			ActorRole:      r.ActorRole,
			Decision:       decisionPtr(r.Decision),
			DecisionNote:   r.DecisionNote,
			RejectReason:   r.RejectReason,
			Override:       r.IsOverride,
			OverrideReason: r.OverrideReason,
			OccurredAt:     r.OccurredAt,
		})
	}
	// If still PENDING_HR after an L1 approval, append the implicit HR pending marker.
	if rec.Status == dom.LeaveStatusPendingHR && !rec.Routing.NoLeader {
		hasHR := false
		for _, r := range rows {
			if r.Stage == dom.StageHR {
				hasHR = true
			}
		}
		if !hasHR {
			out = append(out, dom.LeaveTimelineEntry{Stage: dom.StageHR, Status: dom.TimelineStatusPending, OccurredAt: rec.UpdatedAt})
		}
	}
	return out
}

// reread re-loads the request with denormalized names + timeline for the DTO.
func (s *LeaveService) reread(ctx context.Context, fallback dom.LeaveRequest) (dom.LeaveRequest, error) {
	if full, err := s.repo.GetLeaveRequest(ctx, fallback.ID); err == nil {
		full.Timeline = s.buildTimeline(ctx, full)
		return full, nil
	}
	fallback.Timeline = s.buildTimeline(ctx, fallback)
	return fallback, nil
}

// guardSelf enforces LA-6: an approver cannot decide their own request.
func guardSelf(ctx context.Context, rec dom.LeaveRequest) error {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.EmployeeID != "" && p.EmployeeID == rec.EmployeeID {
		return apperr.Forbidden()
	}
	return nil
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

// balanceRecheckFailed is the 422 for LA-5 (offers the override CTA).
func balanceRecheckFailed(requested, remaining int) error {
	return &apperr.Error{
		Code:       "BALANCE_RECHECK_FAILED",
		HTTPStatus: 422,
		Message:    "Saldo cuti tidak mencukupi saat persetujuan akhir.",
		Fields: map[string]string{
			"requires_override": "true",
			"requested_days":    itoa(requested),
			"remaining_days":    itoa(remaining),
		},
	}
}

// dtoNewStatus maps the DB schedule status to the openapi ScheduleImpactEntry
// new_status enum: 'CANCELLED_BY_LEAVE' → 'LEAVE'. 'LEAVE' lives ONLY at the DTO.
func dtoNewStatus(dbStatus string) string {
	if dbStatus == "CANCELLED_BY_LEAVE" {
		return "LEAVE"
	}
	return dbStatus
}

func timelineStatus(d dom.LeaveDecision) dom.TimelineStatus {
	switch d {
	case dom.DecisionApproved:
		return dom.TimelineStatusApproved
	case dom.DecisionRejected:
		return dom.TimelineStatusRejected
	case dom.DecisionOverrideApproved:
		return dom.TimelineStatusOverrideApproved
	default:
		return dom.TimelineStatusPending
	}
}

func decisionPtr(d dom.LeaveDecision) *dom.LeaveDecision {
	if d == "" {
		return nil
	}
	return &d
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

func actorRole(ctx context.Context) *string {
	if p, ok := auth.PrincipalFrom(ctx); ok {
		r := string(p.Role)
		return &r
	}
	return nil
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
// requires_override/earmark + the marshalled FIFO allocation) on the request.
func (s *LeaveService) writeSnapshot(ctx context.Context, tx pgx.Tx, id string, requested, remaining *int, requiresOverride *bool, earmark *string, alloc []dom.AllocationLine) error {
	raw, merr := marshalAllocation(alloc)
	if merr != nil {
		return merr
	}
	return s.repo.SetBalanceSnapshot(ctx, tx, BalanceSnapshotParams{
		ID:               id,
		RequestedDays:    requested,
		RemainingAtCheck: remaining,
		RequiresOverride: requiresOverride,
		Earmark:          earmark,
		Allocation:       raw,
	})
}
