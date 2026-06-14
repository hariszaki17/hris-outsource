// Package approval holds the dependency-free domain types for the E11 approvals
// engine (F11.1/F11.2/F11.3 / SWP-APT-* / SWP-APL-* / SWP-APV-* / SWP-APA-*).
// These structs are shared between the approval service and repository and map
// 1:1 onto the E11-approvals/openapi.yaml component schemas (the handler maps
// sqlc rows → these → DTOs).
//
// The PINNED CROSS-AGENT CONTRACT (Engine / Hooks / CreateInstanceInput /
// RequestType) lives here verbatim so the leave/overtime epics can import it to
// route their submit/confirm through the engine.
//
// Convention (mirrors internal/domain/leave + internal/domain/reporting):
// nullable columns are pointers; denormalized read-time fields (display_name,
// actor_name, …) are plain strings/pointers filled via JOINs.
package approval

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// ─────────────────────────────────────────────────────────────────────────────
// PINNED CROSS-AGENT CONTRACT — define EXACTLY this (the leave/overtime rip-out
// agent imports it; signatures must match verbatim).
// ─────────────────────────────────────────────────────────────────────────────

// RequestType is the domain request kind routed through the engine (openapi
// schemas.RequestType). Generic — new types opt in via a hook registration.
type RequestType string

const (
	RequestTypeLeave    RequestType = "LEAVE"
	RequestTypeOvertime RequestType = "OVERTIME"
)

// CreateInstanceInput is what a domain epic passes to Engine.CreateInstance when
// it submits a request (EX-1). RequestID is the SWP-LR-* / SWP-OT-* id;
// RequesterID is the SWP-EMP-* of the submitter; CompanyID selects the template.
type CreateInstanceInput struct {
	RequestType RequestType
	RequestID   string
	CompanyID   string
	RequesterID string
}

// Engine is the port leave/overtime call DURING their own submit/confirm
// transaction (EX-1). The instance is created on the SAME tx (so a rolled-back
// submit creates no instance).
type Engine interface {
	CreateInstance(ctx context.Context, tx pgx.Tx, in CreateInstanceInput) (instanceID string, err error)
}

// Hooks fire on a terminal transition, inside the engine's transaction (INV-8).
// A domain epic registers its hooks per RequestType; the engine calls them when
// the last line clears / on bypass (OnApproved) or on reject (OnRejected). A nil
// hook is a no-op (config-error tolerant, C-6). If a hook returns an error the
// engine's transaction rolls back and the instance stays at its current state
// (EX-9 / C-2).
type Hooks struct {
	OnApproved func(ctx context.Context, tx pgx.Tx, requestID string) error
	OnRejected func(ctx context.Context, tx pgx.Tx, requestID string) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Domain types
// ─────────────────────────────────────────────────────────────────────────────

// InstanceStatus is the persisted instance lifecycle state (openapi
// schemas.InstanceStatus). Byte-for-byte with the wire enum.
type InstanceStatus string

const (
	InstanceStatusPending  InstanceStatus = "PENDING"
	InstanceStatusApproved InstanceStatus = "APPROVED"
	InstanceStatusRejected InstanceStatus = "REJECTED"
)

// ActionType is the kind of decision recorded on the trail (openapi
// ApprovalAction.action).
type ActionType string

const (
	ActionApprove ActionType = "APPROVE"
	ActionReject  ActionType = "REJECT"
	ActionBypass  ActionType = "BYPASS"
)

// Member is one user on a line's OR-set (openapi LineMember). DisplayName +
// Active are denormalized read-time (employee full_name; active = employment not
// ended, TM-3).
type Member struct {
	UserID      string
	DisplayName string
	Active      bool
}

// Line is one ordered line of a template (openapi ApprovalLine). Members is the
// OR-set (any one clears it, INV-2).
type Line struct {
	ID      string
	LineNo  int
	Members []Member
}

// Template is a company's approval chain (openapi ApprovalTemplate). 2..3 ordered
// lines (INV-2); version bumped on every edit (INV-6).
type Template struct {
	ID        string
	CompanyID string
	Version   int
	Lines     []Line
	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Instance is one live run of the engine for a domain request (openapi
// ApprovalInstance). TemplateID/TemplateVersion are nil on the super-admin
// fallback (INV-7). Summary is a denormalized inbox-rendering string.
type Instance struct {
	ID              string
	RequestType     RequestType
	RequestID       string
	CompanyID       *string
	TemplateID      *string
	TemplateVersion *int
	CurrentLine     int
	LineCount       int
	Status          InstanceStatus
	RequesterID     *string
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Summary is a server-derived short label for inbox rows (openapi
	// ApprovalInstance.summary). Derived from request_type when no richer domain
	// summary is wired.
	Summary string
}

// Action is one immutable decision-trail row (openapi ApprovalAction). Append-only
// (INV-9), stamped with the template_version in force. ActorName is denormalized.
type Action struct {
	ID              string
	InstanceID      string
	LineNo          int
	TemplateVersion *int
	ActorUserID     *string
	ActorName       string
	Action          ActionType
	Reason          *string
	CreatedAt       time.Time
}

// InstanceDetail is an Instance plus its resolved chain (lines + members from the
// current template, or the implicit single super-admin fallback line) and its
// append-only actions trail (openapi ApprovalInstanceDetail).
type InstanceDetail struct {
	Instance
	Lines   []Line
	Actions []Action
}
