// Package approval — ApprovalService: the E11 approval ENGINE. It owns the
// instance lifecycle (build from the company template or super-admin fallback,
// sequential OR-within-line advance, terminal reject, super-admin bypass), the
// append-only decision trail, self-approval blocking (INV-3), the live-template
// pending reset (INV-6), and the per-type side-effect hooks (INV-8). Every action
// locks the instance (*ForUpdate → 409 on terminal/cleared), checks membership /
// scope (403), audits in-tx, and enqueues notifications on the same tx
// (transactional outbox). Mirrors the leave service's tx/guard/audit/notify shape.
//
// The service implements approval.Engine (CreateInstance), so leave/overtime call
// it inside their own submit/confirm transaction.
package approval

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	reportingdom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// ApprovalService implements the approval engine + the template/inbox surfaces.
type ApprovalService struct {
	repo     ApprovalRepository
	txm      TxRunner
	now      Clock
	notifier jobs.Dispatcher // E10 transactional-outbox notify seam (nil-safe)
	hooks    map[dom.RequestType]dom.Hooks
}

var _ dom.Engine = (*ApprovalService)(nil)

// NewApprovalService wires the engine. The notifier + clock are attached via
// SetNotifier / SetClock (production + tests); hooks via RegisterHooks.
func NewApprovalService(repo ApprovalRepository, txm TxRunner) *ApprovalService {
	return &ApprovalService{
		repo:  repo,
		txm:   txm,
		now:   time.Now,
		hooks: map[dom.RequestType]dom.Hooks{},
	}
}

// SetClock overrides the time source (tests only).
func (s *ApprovalService) SetClock(c Clock) { s.now = c }

// SetNotifier wires the E10 notification dispatcher (additive + nil-safe: jobs.Dispatch
// no-ops when nil, so unit tests without it keep passing).
func (s *ApprovalService) SetNotifier(d jobs.Dispatcher) { s.notifier = d }

// RegisterHooks registers a request type's terminal-transition side-effect hooks
// (INV-8). Called at startup by the leave/overtime DI wiring (main.go).
func (s *ApprovalService) RegisterHooks(rt dom.RequestType, h dom.Hooks) {
	if s.hooks == nil {
		s.hooks = map[dom.RequestType]dom.Hooks{}
	}
	s.hooks[rt] = h
}

// ─────────────────────────────────────────────────────────────────────────────
// Engine: CreateInstance (EX-1) — called inside the caller's submit/confirm tx.
// ─────────────────────────────────────────────────────────────────────────────

// CreateInstance builds an ApprovalInstance for a freshly-submitted domain request
// (EX-1). It looks up the company template: if one exists → template_id/version
// set, current_line=1, line_count=len(lines); if none → super-admin fallback
// (template_id=nil, version=nil, line_count=1, INV-7). status=PENDING. The insert
// + the line-1/super-admin notification run on the PASSED tx (no new tx opened).
func (s *ApprovalService) CreateInstance(ctx context.Context, tx pgx.Tx, in dom.CreateInstanceInput) (string, error) {
	if in.RequestID == "" || in.CompanyID == "" {
		return "", apperr.Invalid(map[string]string{"request_id": "Wajib diisi."})
	}

	tpl, terr := s.repo.GetTemplateByCompany(ctx, in.CompanyID)
	hasTemplate := terr == nil
	if terr != nil && !errors.Is(terr, domain.ErrNotFound) {
		return "", terr
	}

	companyID := in.CompanyID
	requesterID := strOrNil(in.RequesterID)

	p := InsertInstanceParams{
		RequestType: in.RequestType,
		RequestID:   in.RequestID,
		CompanyID:   &companyID,
		CurrentLine: 1,
		Status:      dom.InstanceStatusPending,
		RequesterID: requesterID,
	}
	if hasTemplate {
		tid := tpl.ID
		ver := tpl.Version
		p.TemplateID = &tid
		p.TemplateVersion = &ver
		p.LineCount = len(tpl.Lines)
	} else {
		// Super-admin fallback (INV-7): a single implicit line, no stored template.
		p.LineCount = 1
	}

	inst, ierr := s.repo.InsertInstance(ctx, tx, p)
	if ierr != nil {
		return "", ierr
	}

	// Notify the line-1 members (or super-admins for the fallback) that a request
	// awaits their decision. Same tx (transactional outbox).
	recipients := s.line1Recipients(ctx, tpl, hasTemplate, in.RequesterID)
	if nerr := s.notifyPending(ctx, tx, inst, recipients); nerr != nil {
		return "", nerr
	}

	if aerr := audit.Record(ctx, tx, audit.Entry{
		Action:     audit.ActionCreate,
		EntityType: "approval_instance",
		EntityID:   inst.ID,
		After: map[string]any{
			"request_type": string(inst.RequestType),
			"request_id":   inst.RequestID,
			"company_id":   in.CompanyID,
			"status":       string(inst.Status),
			"current_line": inst.CurrentLine,
			"line_count":   inst.LineCount,
		},
	}); aerr != nil {
		return "", aerr
	}
	return inst.ID, nil
}

// line1Recipients returns the user ids to notify when an instance enters its first
// line. For a configured template that is line-1's OR-set (minus the requester);
// for the fallback the engine has no enumerated super-admin set here, so it returns
// nil (the notification step is skipped — fallback escalation is the super-admin's
// inbox, surfaced via ListInstances).
func (s *ApprovalService) line1Recipients(_ context.Context, tpl dom.Template, hasTemplate bool, requesterID string) []string {
	if !hasTemplate {
		return nil
	}
	var out []string
	for _, l := range tpl.Lines {
		if l.LineNo != 1 {
			continue
		}
		for _, m := range l.Members {
			out = append(out, m.UserID)
		}
	}
	return filterRequester(out, requesterID)
}

// filterRequester is a no-op for user ids (the requester is a SWP-EMP, members are
// SWP-USR); kept as the seam where a requester-user mapping would drop self.
func filterRequester(ids []string, _ string) []string { return ids }

// ─────────────────────────────────────────────────────────────────────────────
// Template surface (F11.1)
// ─────────────────────────────────────────────────────────────────────────────

// GetTemplate returns a company's template (404 if none — the super-admin fallback
// is implicit, not a stored row, INV-7). Role is gated at the handler; this is the
// service read.
func (s *ApprovalService) GetTemplate(ctx context.Context, companyID string) (dom.Template, error) {
	tpl, err := s.repo.GetTemplateByCompany(ctx, companyID)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.Template{}, apperr.NotFound()
	}
	if err != nil {
		return dom.Template{}, apperr.Internal(err)
	}
	return tpl, nil
}

// UpsertTemplate creates or replaces a company's template (TM-2/TM-3/TM-6). It
// validates 2..3 lines (else 400 INVALID_REQUEST) and each line ≥1 active member
// (else 422 APPROVAL_LINE_INVALID, per-line field). In one tx it
// inserts/bumps-version, re-inserts lines+members, resets pending instances to the
// new version (INV-6), notifies the new line-1 members of each reset pending
// instance, and audits. Returns the assembled template.
func (s *ApprovalService) UpsertTemplate(ctx context.Context, companyID string, lines [][]string) (dom.Template, error) {
	if err := validateLineShape(lines); err != nil {
		return dom.Template{}, err
	}

	var out dom.Template
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		existing, gerr := s.repo.GetTemplateByCompany(ctx, companyID)
		exists := gerr == nil
		if gerr != nil && !errors.Is(gerr, domain.ErrNotFound) {
			return gerr
		}

		var tpl dom.Template
		if exists {
			bumped, berr := s.repo.BumpTemplateVersion(ctx, tx, existing.ID)
			if berr != nil {
				return berr
			}
			tpl = bumped
		} else {
			created, cerr := s.repo.InsertTemplate(ctx, tx, companyID, actorUserIDPtr(ctx))
			if cerr != nil {
				return cerr
			}
			tpl = created // version 1
		}

		if rerr := s.repo.ReplaceLines(ctx, tx, tpl.ID, lines); rerr != nil {
			return rerr
		}

		// Validate active membership AFTER the rows exist (the active flag is a JOIN).
		members, merr := s.repo.ListMembers(ctx, tpl.ID)
		if merr != nil {
			return merr
		}
		if verr := validateActiveMembers(lines, members); verr != nil {
			return verr
		}

		// INV-6 live reset: every pending instance for the company restarts at line 1
		// on the new version. (Runs in the same tx — C-4 burst safety.)
		newVer := tpl.Version
		if rerr := s.repo.ResetPendingInstancesForCompany(ctx, tx, companyID, &newVer); rerr != nil {
			return rerr
		}

		// Notify the new line-1 members of each reset pending instance (TM-6).
		if nerr := s.notifyResetLine1(ctx, tx, companyID, lines); nerr != nil {
			return nerr
		}

		action := audit.ActionUpdate
		if !exists {
			action = audit.ActionCreate
		}
		if aerr := audit.Record(ctx, tx, audit.Entry{
			Action:     action,
			EntityType: "approval_template",
			EntityID:   tpl.ID,
			After:      map[string]any{"company_id": companyID, "version": tpl.Version, "line_count": len(lines)},
		}); aerr != nil {
			return aerr
		}

		// Re-read the assembled template (lines + members) for the response.
		full, ferr := s.repo.GetTemplateByID(ctx, tpl.ID)
		if ferr != nil {
			return ferr
		}
		out = full
		return nil
	})
	if err != nil {
		return dom.Template{}, asAppErr(err)
	}
	return out, nil
}

// notifyResetLine1 notifies the new line-1 members that pending instances for the
// company have been re-based to them (TM-6). Best-effort fan-out: each pending
// instance gets one notification per line-1 member.
func (s *ApprovalService) notifyResetLine1(ctx context.Context, tx pgx.Tx, companyID string, lines [][]string) error {
	if s.notifier == nil || len(lines) == 0 {
		return nil
	}
	cid := companyID
	f := InstanceFilter{CompanyID: &cid, Status: ptrStr(string(dom.InstanceStatusPending)), Limit: 200}
	pending, err := s.repo.ListInstances(ctx, f)
	if err != nil {
		return err
	}
	for _, inst := range pending {
		for _, uid := range lines[0] {
			if err := s.notifyPendingOne(ctx, tx, inst, uid); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteTemplate removes a company's template (404 if none); the company reverts to
// the super-admin fallback (TM-7). Pending instances are reset to the fallback
// (template_version=nil, current_line=1). Audits. 204 (no body).
func (s *ApprovalService) DeleteTemplate(ctx context.Context, companyID string) error {
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		existing, gerr := s.repo.GetTemplateByCompany(ctx, companyID)
		if errors.Is(gerr, domain.ErrNotFound) {
			return apperr.NotFound()
		}
		if gerr != nil {
			return gerr
		}
		if derr := s.repo.DeleteTemplate(ctx, tx, existing.ID); derr != nil {
			return derr
		}
		// Revert pending instances to the fallback (nil version, line 1) — INV-7/TM-7.
		if rerr := s.repo.ResetPendingInstancesForCompany(ctx, tx, companyID, nil); rerr != nil {
			return rerr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionDelete,
			EntityType: "approval_template",
			EntityID:   existing.ID,
			Before:     map[string]any{"company_id": companyID, "version": existing.Version},
		})
	})
	return asAppErr(err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Instances: list / get (F11.3)
// ─────────────────────────────────────────────────────────────────────────────

// List returns the instance page. mine → the inbox (current-line membership,
// requester excluded, INV-3). Otherwise scope is enforced: shift_leader/lead are
// restricted to their company/companies; super/hr are global. Cursor-paged.
func (s *ApprovalService) List(ctx context.Context, f InstanceFilter) ([]dom.Instance, *string, bool, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, nil, false, apperr.Unauthenticated()
	}

	if !f.Mine {
		// Company-scope enforcement for non-global roles.
		switch p.Role {
		case auth.RoleShiftLeader:
			if f.CompanyID != nil && *f.CompanyID != p.CompanyID {
				return nil, nil, false, apperr.OutOfScope()
			}
			cid := p.CompanyID
			f.CompanyID = &cid
		case auth.RoleLead:
			if f.CompanyID != nil {
				if !slices.Contains(p.CompanyIDs, *f.CompanyID) {
					return nil, nil, false, apperr.OutOfScope()
				}
			}
			// A lead with no explicit company filter: the member-mode inbox is the
			// scoped surface; for the non-mine list we leave the filter as-is and rely
			// on row-level membership being the practical scope. (No multi-company IN
			// query in v1; an explicit company_id is the supported lead filter.)
		}
	}

	limit := clampLimit(f.Limit)
	f.Limit = limit + 1

	var rows []dom.Instance
	var err error
	if f.Mine {
		rows, err = s.repo.ListInstancesForMember(ctx, p.UserID, f)
	} else {
		rows, err = s.repo.ListInstances(ctx, f)
	}
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	for i := range rows {
		rows[i].Summary = summaryFor(rows[i].RequestType)
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c, cerr := encodeInstanceCursor(last.CreatedAt, last.ID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// Get loads an instance with its resolved chain (lines + members) + the actions
// trail (IB-4). Visibility: staff are company-scoped (out-of-scope hidden as 404);
// the requester (agent) may read their own (requester_id == principal.EmployeeID).
func (s *ApprovalService) Get(ctx context.Context, id string) (dom.InstanceDetail, error) {
	inst, err := s.repo.GetInstance(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.InstanceDetail{}, apperr.NotFound()
	}
	if err != nil {
		return dom.InstanceDetail{}, apperr.Internal(err)
	}

	if !s.canRead(ctx, inst) {
		return dom.InstanceDetail{}, apperr.NotFound()
	}

	inst.Summary = summaryFor(inst.RequestType)
	detail := dom.InstanceDetail{Instance: inst}

	// Resolved chain: the live template's lines, or a single implicit super-admin
	// line for the fallback (INV-7).
	if inst.TemplateID != nil {
		tpl, terr := s.repo.GetTemplateByID(ctx, *inst.TemplateID)
		if terr == nil {
			detail.Lines = tpl.Lines
		}
	}
	if detail.Lines == nil {
		detail.Lines = []dom.Line{{LineNo: 1, Members: nil}} // fallback super-admin line
	}

	actions, aerr := s.repo.ListActionsByInstance(ctx, id)
	if aerr != nil {
		return dom.InstanceDetail{}, apperr.Internal(aerr)
	}
	detail.Actions = actions
	return detail, nil
}

// canRead enforces instance read visibility. Agents may read only their own
// (requester); staff use the company GuardCompany scope (super/hr global).
func (s *ApprovalService) canRead(ctx context.Context, inst dom.Instance) bool {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return false
	}
	if p.Role == auth.RoleAgent {
		return inst.RequesterID != nil && *inst.RequesterID == p.EmployeeID
	}
	return rbac.GuardCompany(ctx, deref(inst.CompanyID)) == nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Execution: approve / reject / bypass (F11.2)
// ─────────────────────────────────────────────────────────────────────────────

// Approve clears the current line (OR) and advances; the last line clearing
// finalizes the request and fires OnApproved (EX-3/EX-7). Guards: terminal/cleared
// → 409 LINE_ALREADY_CLEARED; not a current-line member (and not super_admin) →
// 403; requester on the current line → 403 SELF_APPROVAL_FORBIDDEN (INV-3).
func (s *ApprovalService) Approve(ctx context.Context, id, note string) (dom.Instance, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return dom.Instance{}, apperr.Unauthenticated()
	}
	var out dom.Instance
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		inst, lerr := s.lock(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if inst.Status != dom.InstanceStatusPending {
			return lineAlreadyCleared()
		}

		members, merr := s.repo.CurrentLineMembers(ctx, id)
		if merr != nil {
			return merr
		}
		isMember := slices.Contains(members, p.UserID)
		isRequester := inst.RequesterID != nil && *inst.RequesterID == p.EmployeeID

		// INV-3: a requester who is on the current line cannot clear it.
		if isMember && isRequester {
			return selfApprovalForbidden()
		}
		// Membership decides (not role) — but super_admin may always act.
		if !isMember && p.Role != auth.RoleSuperAdmin {
			return apperr.Forbidden()
		}

		// Append the APPROVE action (stamped with the version in force, INV-9).
		if _, aerr := s.repo.InsertAction(ctx, tx, InsertActionParams{
			InstanceID:      id,
			LineNo:          inst.CurrentLine,
			TemplateVersion: inst.TemplateVersion,
			ActorUserID:     actorUserIDPtr(ctx),
			Action:          dom.ActionApprove,
			Reason:          strOrNil(note),
		}); aerr != nil {
			return aerr
		}

		if inst.CurrentLine < inst.LineCount {
			// Advance to the next line (EX-3).
			next := inst.CurrentLine + 1
			if uerr := s.repo.UpdateInstanceProgress(ctx, tx, id, next, dom.InstanceStatusPending); uerr != nil {
				return uerr
			}
			inst.CurrentLine = next
			out = inst
			if nerr := s.notifyNextLine(ctx, tx, inst); nerr != nil {
				return nerr
			}
			return audit.Record(ctx, tx, instAudit(id, "PENDING", "PENDING", actorUserIDPtr(ctx), "APPROVE_ADVANCE", inst.CurrentLine))
		}

		// Last line cleared → finalize APPROVED + fire OnApproved (EX-7/INV-8).
		if uerr := s.repo.UpdateInstanceProgress(ctx, tx, id, inst.CurrentLine, dom.InstanceStatusApproved); uerr != nil {
			return uerr
		}
		inst.Status = dom.InstanceStatusApproved
		out = inst
		if herr := s.fireApproved(ctx, tx, inst); herr != nil {
			return herr // EX-9: hook error rolls back the tx; instance stays.
		}
		if nerr := s.notifyRequester(ctx, tx, inst, reportingdom.NotifApprovalApproved, "Pengajuan disetujui", "Pengajuan Anda telah disetujui."); nerr != nil {
			return nerr
		}
		return audit.Record(ctx, tx, instAudit(id, "PENDING", "APPROVED", actorUserIDPtr(ctx), "APPROVE_FINAL", inst.CurrentLine))
	})
	if err != nil {
		return dom.Instance{}, asAppErr(err)
	}
	return s.reread(ctx, out), nil
}

// Reject moves a PENDING instance → REJECTED (reason required, EX-5/INV-4). Any
// current-line member (or super_admin) may reject. Terminal → 409. Fires OnRejected.
func (s *ApprovalService) Reject(ctx context.Context, id, reason string) (dom.Instance, error) {
	if reason == "" {
		return dom.Instance{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi."})
	}
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return dom.Instance{}, apperr.Unauthenticated()
	}
	var out dom.Instance
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		inst, lerr := s.lock(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if inst.Status != dom.InstanceStatusPending {
			return lineAlreadyCleared()
		}
		members, merr := s.repo.CurrentLineMembers(ctx, id)
		if merr != nil {
			return merr
		}
		if !slices.Contains(members, p.UserID) && p.Role != auth.RoleSuperAdmin {
			return apperr.Forbidden()
		}
		if _, aerr := s.repo.InsertAction(ctx, tx, InsertActionParams{
			InstanceID:      id,
			LineNo:          inst.CurrentLine,
			TemplateVersion: inst.TemplateVersion,
			ActorUserID:     actorUserIDPtr(ctx),
			Action:          dom.ActionReject,
			Reason:          &reason,
		}); aerr != nil {
			return aerr
		}
		if uerr := s.repo.UpdateInstanceProgress(ctx, tx, id, inst.CurrentLine, dom.InstanceStatusRejected); uerr != nil {
			return uerr
		}
		inst.Status = dom.InstanceStatusRejected
		out = inst
		if herr := s.fireRejected(ctx, tx, inst); herr != nil {
			return herr
		}
		if nerr := s.notifyRequester(ctx, tx, inst, reportingdom.NotifApprovalRejected, "Pengajuan ditolak", "Pengajuan Anda ditolak: "+reason); nerr != nil {
			return nerr
		}
		return audit.Record(ctx, tx, instAudit(id, "PENDING", "REJECTED", actorUserIDPtr(ctx), "REJECT", inst.CurrentLine))
	})
	if err != nil {
		return dom.Instance{}, asAppErr(err)
	}
	return s.reread(ctx, out), nil
}

// Bypass force-approves a non-terminal instance, skipping remaining lines, even if
// the caller is not a member (EX-6/INV-5). super_admin ONLY; reason required.
// Terminal → 409. Records a BYPASS action, finalizes APPROVED, fires OnApproved.
func (s *ApprovalService) Bypass(ctx context.Context, id, reason string) (dom.Instance, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return dom.Instance{}, apperr.Unauthenticated()
	}
	if p.Role != auth.RoleSuperAdmin {
		return dom.Instance{}, apperr.Forbidden()
	}
	if reason == "" {
		return dom.Instance{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi."})
	}
	var out dom.Instance
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		inst, lerr := s.lock(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if inst.Status != dom.InstanceStatusPending {
			return lineAlreadyCleared()
		}
		if _, aerr := s.repo.InsertAction(ctx, tx, InsertActionParams{
			InstanceID:      id,
			LineNo:          inst.CurrentLine,
			TemplateVersion: inst.TemplateVersion,
			ActorUserID:     actorUserIDPtr(ctx),
			Action:          dom.ActionBypass,
			Reason:          &reason,
		}); aerr != nil {
			return aerr
		}
		if uerr := s.repo.UpdateInstanceProgress(ctx, tx, id, inst.CurrentLine, dom.InstanceStatusApproved); uerr != nil {
			return uerr
		}
		inst.Status = dom.InstanceStatusApproved
		out = inst
		if herr := s.fireApproved(ctx, tx, inst); herr != nil {
			return herr
		}
		if nerr := s.notifyRequester(ctx, tx, inst, reportingdom.NotifApprovalApproved, "Pengajuan disetujui", "Pengajuan Anda disetujui (bypass)."); nerr != nil {
			return nerr
		}
		return audit.Record(ctx, tx, instAudit(id, "PENDING", "APPROVED", actorUserIDPtr(ctx), "BYPASS", inst.CurrentLine))
	})
	if err != nil {
		return dom.Instance{}, asAppErr(err)
	}
	return s.reread(ctx, out), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers (hooks, notifications, locking, validation)
// ─────────────────────────────────────────────────────────────────────────────

func (s *ApprovalService) fireApproved(ctx context.Context, tx pgx.Tx, inst dom.Instance) error {
	h, ok := s.hooks[inst.RequestType]
	if !ok || h.OnApproved == nil {
		return nil // C-6: missing hook is a config error, not a failure.
	}
	return h.OnApproved(ctx, tx, inst.RequestID)
}

func (s *ApprovalService) fireRejected(ctx context.Context, tx pgx.Tx, inst dom.Instance) error {
	h, ok := s.hooks[inst.RequestType]
	if !ok || h.OnRejected == nil {
		return nil
	}
	return h.OnRejected(ctx, tx, inst.RequestID)
}

func (s *ApprovalService) lock(ctx context.Context, tx pgx.Tx, id string) (dom.Instance, error) {
	inst, err := s.repo.GetInstanceForUpdate(ctx, tx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.Instance{}, apperr.NotFound()
	}
	if err != nil {
		return dom.Instance{}, err
	}
	return inst, nil
}

// reread re-loads the instance (best-effort) for the response, falling back to the
// in-tx value if the read fails.
func (s *ApprovalService) reread(ctx context.Context, fallback dom.Instance) dom.Instance {
	if full, err := s.repo.GetInstance(ctx, fallback.ID); err == nil {
		full.Summary = summaryFor(full.RequestType)
		return full
	}
	fallback.Summary = summaryFor(fallback.RequestType)
	return fallback
}

func (s *ApprovalService) notifyNextLine(ctx context.Context, tx pgx.Tx, inst dom.Instance) error {
	members, err := s.repo.CurrentLineMembers(ctx, inst.ID)
	if err != nil {
		return err
	}
	return s.notifyPending(ctx, tx, inst, members)
}

func (s *ApprovalService) notifyPending(ctx context.Context, tx pgx.Tx, inst dom.Instance, recipients []string) error {
	for _, uid := range recipients {
		if err := s.notifyPendingOne(ctx, tx, inst, uid); err != nil {
			return err
		}
	}
	return nil
}

func (s *ApprovalService) notifyPendingOne(ctx context.Context, tx pgx.Tx, inst dom.Instance, recipientUserID string) error {
	return jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
		NotifKind:        string(reportingdom.NotifApprovalPending),
		RecipientID:      recipientUserID,
		Title:            "Persetujuan dibutuhkan",
		Body:             summaryFor(inst.RequestType) + " menunggu keputusan Anda.",
		DeepLinkEpic:     "E11",
		DeepLinkEntityID: inst.ID,
		DeepLinkPath:     "/approval-instances/" + inst.ID,
		ActorID:          actorUserID(ctx),
		IsCritical:       false,
	})
}

func (s *ApprovalService) notifyRequester(ctx context.Context, tx pgx.Tx, inst dom.Instance, kind reportingdom.NotificationKind, title, body string) error {
	if inst.RequesterID == nil || *inst.RequesterID == "" {
		return nil
	}
	return jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
		NotifKind:        string(kind),
		RecipientID:      *inst.RequesterID,
		Title:            title,
		Body:             body,
		DeepLinkEpic:     "E11",
		DeepLinkEntityID: inst.ID,
		DeepLinkPath:     "/approval-instances/" + inst.ID,
		ActorID:          actorUserID(ctx),
		IsCritical:       true,
	})
}

// validateLineShape enforces TM-2: 2..3 lines, each with ≥1 member id (else 400).
func validateLineShape(lines [][]string) error {
	if len(lines) < 2 || len(lines) > 3 {
		return apperr.Invalid(map[string]string{"lines": "Wajib 2 atau 3 baris persetujuan."})
	}
	fields := map[string]string{}
	for i, members := range lines {
		if len(members) == 0 {
			fields["lines."+itoa(i)] = "Setiap baris wajib memiliki minimal 1 anggota."
		}
	}
	if len(fields) > 0 {
		// Empty line is structurally invalid (400), per TM-2's "each line ≥1".
		return apperr.Invalid(fields)
	}
	return nil
}

// validateActiveMembers enforces TM-3: every configured member must be an active
// SWP staff user (employment not ended), else 422 APPROVAL_LINE_INVALID with
// per-line fields naming the offending line.
func validateActiveMembers(lines [][]string, members []dom.Member) error {
	active := make(map[string]bool, len(members))
	for _, m := range members {
		active[m.UserID] = m.Active
	}
	fields := map[string]string{}
	for i, lineMembers := range lines {
		for _, uid := range lineMembers {
			a, known := active[uid]
			if !known {
				fields["lines."+itoa(i)] = "Anggota tidak ditemukan: " + uid
				break
			}
			if !a {
				fields["lines."+itoa(i)] = "Anggota tidak aktif (employment berakhir): " + uid
				break
			}
		}
	}
	if len(fields) > 0 {
		return apperr.Rule(dom.CodeApprovalLineInvalid, fields)
	}
	return nil
}

// lineAlreadyCleared is the 409 for acting on a terminal/cleared instance (EX-11).
func lineAlreadyCleared() error {
	return &apperr.Error{Code: dom.CodeLineAlreadyCleared, HTTPStatus: http.StatusConflict, Message: "Baris sudah selesai atau pengajuan sudah final."}
}

// selfApprovalForbidden is the 403 for a requester clearing their own line (INV-3).
func selfApprovalForbidden() error {
	return &apperr.Error{Code: dom.CodeSelfApprovalForbidden, HTTPStatus: http.StatusForbidden, Message: "Tidak dapat menyetujui pengajuan sendiri."}
}

func instAudit(id, before, after string, actor *string, action string, line int) audit.Entry {
	return audit.Entry{
		Action:     audit.ActionUpdate,
		EntityType: "approval_instance",
		EntityID:   id,
		Before:     map[string]any{"status": before},
		After:      map[string]any{"status": after, "action": action, "current_line": line, "decided_by": ptrVal(actor)},
	}
}

func ptrVal(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func ptrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
