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
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// LeaveService implements the leave-request approval business logic.
type LeaveService struct {
	repo     LeaveRepository
	quota    QuotaRepository
	schedule SchedulePort
	txm      TxRunner
	now      Clock
}

// NewLeaveService wires the leave service. The schedule port is the INV-3
// loop-closer surface (the scheduling repo).
func NewLeaveService(repo LeaveRepository, quota QuotaRepository, schedule SchedulePort, txm TxRunner) *LeaveService {
	return &LeaveService{repo: repo, quota: quota, schedule: schedule, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *LeaveService) SetClock(c Clock) { s.now = c }

// --- list / get ---

// List returns the leave-request page. Leader scope is forced to their led company;
// an explicit company_id outside scope yields 403 OUT_OF_SCOPE.
func (s *LeaveService) List(ctx context.Context, f RequestFilter) ([]dom.LeaveRequest, *string, bool, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, nil, false, apperr.Unauthenticated()
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
	if serr := rbac.GuardCompany(ctx, deref(rec.CompanyID)); serr != nil {
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
		// TODO(Phase-11): enqueue NotificationArgs ("leave L1-approved → HR queue").
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

		// 1. resolve the quota-tracked flag (real leave_types.is_annual).
		lt, lterr := s.repo.GetLeaveType(ctx, rec.LeaveTypeID)
		if lterr != nil && !errors.Is(lterr, domain.ErrNotFound) {
			return lterr
		}
		quotaTracked := lt.IsAnnual

		// 2. balance re-check + deduct (quota-tracked only).
		var quotaID *string
		var remainingAtCheck *int
		if quotaTracked {
			q, qerr := s.quota.FindQuotaForEmployeeTypePeriod(ctx, rec.EmployeeID, rec.LeaveTypeID, rec.StartDate.Year())
			if qerr != nil && !errors.Is(qerr, domain.ErrNotFound) {
				return qerr
			}
			if qerr == nil {
				qid := q.ID
				quotaID = &qid
				rem := q.Remaining()
				remainingAtCheck = &rem
				if !override && rec.DurationDays > rem {
					return balanceRecheckFailed(rec.DurationDays, rem)
				}
				if _, derr := s.quota.DeductLeaveQuota(ctx, tx, q.ID, rec.DurationDays); derr != nil {
					return derr
				}
				if override {
					ov := dom.LeaveQuotaOverride{
						LeaveRequestID: id,
						OverrideReason: overrideReason,
						OverriddenBy:   deref(actor),
						OverriddenAt:   s.now().UTC(),
					}
					if _, oerr := s.quota.SetLeaveQuotaOverride(ctx, tx, q.ID, ov); oerr != nil {
						return oerr
					}
				}
			}
		}

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

		// 4. transition + snapshot.
		req := int(rec.DurationDays)
		updated, uerr := s.repo.UpdateLeaveRequestStatus(ctx, tx, UpdateStatusParams{
			ID:                      id,
			Status:                  dom.LeaveStatusApproved,
			NoLeader:                rec.Routing.NoLeader,
			AssignedLeaderID:        rec.Routing.AssignedLeaderID,
			ClockInConflict:         rec.ClockInConflict,
			BalanceQuotaID:          quotaID,
			BalanceRequestedDays:    &req,
			BalanceRemainingAtCheck: remainingAtCheck,
			BalanceRequiresOverride: boolPtr(override),
		})
		if uerr != nil {
			return uerr
		}
		out = updated

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
		// TODO(Phase-11): enqueue NotificationArgs ("leave approved" + submitter notify).
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
		rsn := reason
		if _, aerr := s.repo.InsertLeaveApproval(ctx, tx, ApprovalRow{
			LeaveRequestID: id, Stage: stage, Decision: dom.DecisionRejected,
			ActorID: actor, ActorRole: role, RejectReason: &rsn,
		}); aerr != nil {
			return aerr
		}
		// TODO(Phase-11): enqueue NotificationArgs ("leave rejected").
		return audit.Record(ctx, tx, leaveAudit(id, "leave_request", string(rec.Status), "REJECTED", actor, "REJECT"))
	})
	if err != nil {
		return dom.LeaveRequest{}, asAppErr(err)
	}
	return s.reread(ctx, out)
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

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func boolPtr(b bool) *bool { return &b }
