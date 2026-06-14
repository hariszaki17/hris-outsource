// Package approval (service) — the E11 approval ENGINE. ports.go declares the
// repository data dependency + the TxRunner/Clock seams the service needs.
// Mirrors internal/service/leave/ports.go (the port lives in the service package
// so the repository depends on the service, not the other way around).
package approval

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
)

// TxRunner runs a closure inside a DB transaction (db.TxManager satisfies it).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock supplies the current time (overridable in tests).
type Clock func() time.Time

// InstanceFilter is the decoded GET /approval-instances query (cursor-paged).
// Mine switches to the inbox query (current-line membership, requester excluded).
type InstanceFilter struct {
	Mine          bool
	CompanyID     *string
	RequestType   *string
	Status        *string
	Limit         int
	CursorCreated *time.Time
	CursorID      *string
}

// ApprovalRepository is the data dependency for the approval engine. Reads run on
// the pool; locked re-checks + writes run via methods that take a pgx.Tx.
type ApprovalRepository interface {
	// --- templates (F11.1) ---

	// GetTemplateByCompany assembles the company's full template (lines + members),
	// or domain.ErrNotFound when none is configured.
	GetTemplateByCompany(ctx context.Context, companyID string) (dom.Template, error)
	// GetTemplateByID assembles a template (lines + members) by its id.
	GetTemplateByID(ctx context.Context, id string) (dom.Template, error)

	InsertTemplate(ctx context.Context, tx pgx.Tx, companyID string, createdBy *string) (dom.Template, error)
	BumpTemplateVersion(ctx context.Context, tx pgx.Tx, id string) (dom.Template, error)
	DeleteTemplate(ctx context.Context, tx pgx.Tx, id string) error
	// ReplaceLines clears a template's lines (members cascade) then re-inserts the
	// ordered lines + their OR-set members. lines[i] is line_no i+1.
	ReplaceLines(ctx context.Context, tx pgx.Tx, templateID string, lines [][]string) error
	// ListMembers returns every member across a template's lines (active flag joined).
	ListMembers(ctx context.Context, templateID string) ([]dom.Member, error)

	// ResetPendingInstancesForCompany re-bases all PENDING instances for the company
	// to line 1 on newVersion (INV-6). newVersion is nil on a delete (revert to
	// fallback).
	ResetPendingInstancesForCompany(ctx context.Context, tx pgx.Tx, companyID string, newVersion *int) error

	// --- instances (F11.2/F11.3) ---

	InsertInstance(ctx context.Context, tx pgx.Tx, p InsertInstanceParams) (dom.Instance, error)
	GetInstance(ctx context.Context, id string) (dom.Instance, error)
	GetInstanceForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.Instance, error)
	ListInstances(ctx context.Context, f InstanceFilter) ([]dom.Instance, error)
	ListInstancesForMember(ctx context.Context, memberUserID string, f InstanceFilter) ([]dom.Instance, error)
	UpdateInstanceProgress(ctx context.Context, tx pgx.Tx, id string, currentLine int, status dom.InstanceStatus) error
	// CurrentLineMembers returns the user_ids on the instance's current line (INV-2/3).
	CurrentLineMembers(ctx context.Context, instanceID string) ([]string, error)

	// --- actions (decision trail, F11.2) ---

	InsertAction(ctx context.Context, tx pgx.Tx, p InsertActionParams) (dom.Action, error)
	ListActionsByInstance(ctx context.Context, instanceID string) ([]dom.Action, error)
}

// InsertInstanceParams carries one approval_instances insert (EX-1). Nullable
// columns are pointers; TemplateID/TemplateVersion are nil on the super-admin
// fallback (INV-7).
type InsertInstanceParams struct {
	RequestType     dom.RequestType
	RequestID       string
	CompanyID       *string
	TemplateID      *string
	TemplateVersion *int
	CurrentLine     int
	LineCount       int
	Status          dom.InstanceStatus
	RequesterID     *string
}

// InsertActionParams carries one append-only approval_actions insert (INV-9).
type InsertActionParams struct {
	InstanceID      string
	LineNo          int
	TemplateVersion *int
	ActorUserID     *string
	Action          dom.ActionType
	Reason          *string
}
