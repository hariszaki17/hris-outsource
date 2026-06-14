// Package domain — foundations types for the E1 foundations slice.
// These dependency-free structs are shared between the foundations service
// and repository without importing either. They sit alongside identity.go.
package domain

import "time"

// AuditEntry is a single immutable audit-log row (CONVENTIONS §16.1).
// Before and After are the JSON snapshots of the entity before and after the
// mutation; nil when not applicable (before on create, after on delete).
type AuditEntry struct {
	ID          string
	ActorUserID *string
	ActorRole   *string
	Action      string
	EntityType  string
	EntityID    string
	Before      map[string]any
	After       map[string]any
	RequestID   *string
	CreatedAt   time.Time
}

// PlatformSetting is one row from the platform_settings table (7 v1 keys).
type PlatformSetting struct {
	Key    string
	Value  string
	Label  string
	Locked bool
}

// UserFilter is the decoded set of query parameters for GET /users.
// All fields are optional (nil = no constraint). The cursor fields are set
// when paginating past the first page.
type UserFilter struct {
	Q               *string
	Role            *string
	Status          *string
	CompanyID       *string
	CursorCreatedAt *time.Time
	CursorID        *string
	Limit           int
}

// AuditFilter is the decoded set of query parameters for GET /audit-log.
type AuditFilter struct {
	Q               *string
	ActorUserID     *string
	Action          *string
	EntityType      *string
	EntityID        *string
	CreatedGTE      *time.Time
	CreatedLTE      *time.Time
	CursorCreatedAt *time.Time
	CursorID        *string
	Limit           int
}
