// Package approval (repository) — ApprovalRepo implements svc.ApprovalRepository
// over the E11 sqlc templates / instances / actions queries. Reads run on the
// pool; locked re-checks + writes run via q.WithTx(tx). Row→domain mapping
// assembles a Template with its ordered lines + per-line OR-set members, and an
// InstanceDetail with its lines + actions. Mirrors internal/repository/leave.
package approval

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/approval"
)

// ApprovalRepo is the sqlc-backed implementation of svc.ApprovalRepository.
type ApprovalRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ApprovalRepository = (*ApprovalRepo)(nil)

// NewApprovalRepo returns an ApprovalRepo backed by pool.
func NewApprovalRepo(pool *db.Pool) *ApprovalRepo {
	return &ApprovalRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func strptr(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

func i32(n int) int32 { return int32(n) }

func i32ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

func intptr(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

// mapErr normalizes pgx.ErrNoRows to the shared domain.ErrNotFound (the service
// translates that to a 404).
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// ─────────────────────────────────────────────────────────────────────────────
// templates (F11.1)
// ─────────────────────────────────────────────────────────────────────────────

func (r *ApprovalRepo) GetTemplateByCompany(ctx context.Context, companyID string) (dom.Template, error) {
	t, err := r.q.GetApprovalTemplateByCompany(ctx, companyID)
	if err != nil {
		return dom.Template{}, mapErr(err)
	}
	return r.assembleTemplate(ctx, t)
}

func (r *ApprovalRepo) GetTemplateByID(ctx context.Context, id string) (dom.Template, error) {
	t, err := r.q.GetApprovalTemplateByID(ctx, id)
	if err != nil {
		return dom.Template{}, mapErr(err)
	}
	return r.assembleTemplate(ctx, t)
}

// assembleTemplate joins a template row with its ordered lines and per-line
// OR-set members (DisplayName/Active denormalized) into the domain Template.
func (r *ApprovalRepo) assembleTemplate(ctx context.Context, t sqlcgen.ApprovalTemplate) (dom.Template, error) {
	lines, err := r.q.ListApprovalLinesByTemplate(ctx, t.ID)
	if err != nil {
		return dom.Template{}, err
	}
	members, err := r.q.ListApprovalLineMembersByTemplate(ctx, t.ID)
	if err != nil {
		return dom.Template{}, err
	}
	byLine := make(map[string][]dom.Member, len(lines))
	for _, m := range members {
		byLine[m.LineID] = append(byLine[m.LineID], dom.Member{
			UserID:      m.UserID,
			DisplayName: m.DisplayName,
			Active:      m.Active,
		})
	}
	out := dom.Template{
		ID:        t.ID,
		CompanyID: t.CompanyID,
		Version:   int(t.Version),
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
		Lines:     make([]dom.Line, 0, len(lines)),
	}
	for _, l := range lines {
		out.Lines = append(out.Lines, dom.Line{
			ID:      l.ID,
			LineNo:  int(l.LineNo),
			Members: byLine[l.ID],
		})
	}
	return out, nil
}

func (r *ApprovalRepo) InsertTemplate(ctx context.Context, tx pgx.Tx, companyID string, createdBy *string) (dom.Template, error) {
	t, err := r.q.WithTx(tx).InsertApprovalTemplate(ctx, sqlcgen.InsertApprovalTemplateParams{
		CompanyID: companyID,
		CreatedBy: strptr(createdBy),
	})
	if err != nil {
		return dom.Template{}, mapErr(err)
	}
	return dom.Template{
		ID:        t.ID,
		CompanyID: t.CompanyID,
		Version:   int(t.Version),
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}, nil
}

func (r *ApprovalRepo) BumpTemplateVersion(ctx context.Context, tx pgx.Tx, id string) (dom.Template, error) {
	t, err := r.q.WithTx(tx).UpdateApprovalTemplateVersion(ctx, id)
	if err != nil {
		return dom.Template{}, mapErr(err)
	}
	return dom.Template{
		ID:        t.ID,
		CompanyID: t.CompanyID,
		Version:   int(t.Version),
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}, nil
}

func (r *ApprovalRepo) DeleteTemplate(ctx context.Context, tx pgx.Tx, id string) error {
	return r.q.WithTx(tx).DeleteApprovalTemplate(ctx, id)
}

// ReplaceLines clears a template's lines (members cascade) then re-inserts the
// ordered lines + their members. lines[i] is line_no i+1; each inner slice is the
// line's OR-set of user ids.
func (r *ApprovalRepo) ReplaceLines(ctx context.Context, tx pgx.Tx, templateID string, lines [][]string) error {
	qtx := r.q.WithTx(tx)
	if err := qtx.DeleteApprovalLinesByTemplate(ctx, templateID); err != nil {
		return err
	}
	for i, members := range lines {
		line, err := qtx.InsertApprovalLine(ctx, sqlcgen.InsertApprovalLineParams{
			TemplateID: templateID,
			LineNo:     i32(i + 1),
		})
		if err != nil {
			return err
		}
		for _, uid := range members {
			if err := qtx.InsertApprovalLineMember(ctx, sqlcgen.InsertApprovalLineMemberParams{
				LineID: line.ID,
				UserID: uid,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ApprovalRepo) ListMembers(ctx context.Context, templateID string) ([]dom.Member, error) {
	rows, err := r.q.ListApprovalLineMembersByTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	out := make([]dom.Member, 0, len(rows))
	for _, m := range rows {
		out = append(out, dom.Member{UserID: m.UserID, DisplayName: m.DisplayName, Active: m.Active})
	}
	return out, nil
}

func (r *ApprovalRepo) ResetPendingInstancesForCompany(ctx context.Context, tx pgx.Tx, companyID string, newVersion *int) error {
	cid := companyID
	return r.q.WithTx(tx).ResetPendingInstancesForCompany(ctx, sqlcgen.ResetPendingInstancesForCompanyParams{
		TemplateVersion: i32ptr(newVersion),
		CompanyID:       &cid,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// instances (F11.2/F11.3)
// ─────────────────────────────────────────────────────────────────────────────

func (r *ApprovalRepo) InsertInstance(ctx context.Context, tx pgx.Tx, p svc.InsertInstanceParams) (dom.Instance, error) {
	row, err := r.q.WithTx(tx).InsertApprovalInstance(ctx, sqlcgen.InsertApprovalInstanceParams{
		RequestType:     string(p.RequestType),
		RequestID:       p.RequestID,
		CompanyID:       strptr(p.CompanyID),
		TemplateID:      strptr(p.TemplateID),
		TemplateVersion: i32ptr(p.TemplateVersion),
		CurrentLine:     i32(p.CurrentLine),
		LineCount:       i32(p.LineCount),
		Status:          string(p.Status),
		RequesterID:     strptr(p.RequesterID),
	})
	if err != nil {
		return dom.Instance{}, mapErr(err)
	}
	return mapInstance(row), nil
}

func (r *ApprovalRepo) GetInstance(ctx context.Context, id string) (dom.Instance, error) {
	row, err := r.q.GetApprovalInstance(ctx, id)
	if err != nil {
		return dom.Instance{}, mapErr(err)
	}
	return mapInstance(row), nil
}

func (r *ApprovalRepo) GetInstanceForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.Instance, error) {
	row, err := r.q.WithTx(tx).GetApprovalInstanceForUpdate(ctx, id)
	if err != nil {
		return dom.Instance{}, mapErr(err)
	}
	return mapInstance(row), nil
}

func (r *ApprovalRepo) ListInstances(ctx context.Context, f svc.InstanceFilter) ([]dom.Instance, error) {
	rows, err := r.q.ListApprovalInstances(ctx, sqlcgen.ListApprovalInstancesParams{
		CompanyID:       strptr(f.CompanyID),
		RequestType:     strptr(f.RequestType),
		Status:          strptr(f.Status),
		CursorCreatedAt: f.CursorCreated,
		CursorID:        f.CursorID,
		Lim:             i32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]dom.Instance, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapInstance(row))
	}
	return out, nil
}

func (r *ApprovalRepo) ListInstancesForMember(ctx context.Context, memberUserID string, f svc.InstanceFilter) ([]dom.Instance, error) {
	rows, err := r.q.ListApprovalInstancesForMember(ctx, sqlcgen.ListApprovalInstancesForMemberParams{
		MemberUserID:    memberUserID,
		CompanyID:       strptr(f.CompanyID),
		RequestType:     strptr(f.RequestType),
		CursorCreatedAt: f.CursorCreated,
		CursorID:        f.CursorID,
		Lim:             i32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]dom.Instance, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapInstance(row))
	}
	return out, nil
}

func (r *ApprovalRepo) UpdateInstanceProgress(ctx context.Context, tx pgx.Tx, id string, currentLine int, status dom.InstanceStatus) error {
	return r.q.WithTx(tx).UpdateApprovalInstanceProgress(ctx, sqlcgen.UpdateApprovalInstanceProgressParams{
		CurrentLine: i32(currentLine),
		Status:      string(status),
		ID:          id,
	})
}

func (r *ApprovalRepo) CurrentLineMembers(ctx context.Context, instanceID string) ([]string, error) {
	return r.q.GetCurrentLineMembers(ctx, instanceID)
}

// ─────────────────────────────────────────────────────────────────────────────
// actions (decision trail, F11.2)
// ─────────────────────────────────────────────────────────────────────────────

func (r *ApprovalRepo) InsertAction(ctx context.Context, tx pgx.Tx, p svc.InsertActionParams) (dom.Action, error) {
	row, err := r.q.WithTx(tx).InsertApprovalAction(ctx, sqlcgen.InsertApprovalActionParams{
		InstanceID:      p.InstanceID,
		LineNo:          i32(p.LineNo),
		TemplateVersion: i32ptr(p.TemplateVersion),
		ActorUserID:     strptr(p.ActorUserID),
		Action:          string(p.Action),
		Reason:          strptr(p.Reason),
	})
	if err != nil {
		return dom.Action{}, mapErr(err)
	}
	return dom.Action{
		ID:              row.ID,
		InstanceID:      row.InstanceID,
		LineNo:          int(row.LineNo),
		TemplateVersion: intptr(row.TemplateVersion),
		ActorUserID:     row.ActorUserID,
		Action:          dom.ActionType(row.Action),
		Reason:          row.Reason,
		CreatedAt:       row.CreatedAt,
	}, nil
}

func (r *ApprovalRepo) ListActionsByInstance(ctx context.Context, instanceID string) ([]dom.Action, error) {
	rows, err := r.q.ListApprovalActionsByInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	out := make([]dom.Action, 0, len(rows))
	for _, row := range rows {
		out = append(out, dom.Action{
			ID:              row.ID,
			InstanceID:      row.InstanceID,
			LineNo:          int(row.LineNo),
			TemplateVersion: intptr(row.TemplateVersion),
			ActorUserID:     row.ActorUserID,
			ActorName:       row.ActorName,
			Action:          dom.ActionType(row.Action),
			Reason:          row.Reason,
			CreatedAt:       row.CreatedAt,
		})
	}
	return out, nil
}

// mapInstance maps a sqlc ApprovalInstance row to the domain Instance.
func mapInstance(row sqlcgen.ApprovalInstance) dom.Instance {
	return dom.Instance{
		ID:              row.ID,
		RequestType:     dom.RequestType(row.RequestType),
		RequestID:       row.RequestID,
		CompanyID:       row.CompanyID,
		TemplateID:      row.TemplateID,
		TemplateVersion: intptr(row.TemplateVersion),
		CurrentLine:     int(row.CurrentLine),
		LineCount:       int(row.LineCount),
		Status:          dom.InstanceStatus(row.Status),
		RequesterID:     row.RequesterID,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
