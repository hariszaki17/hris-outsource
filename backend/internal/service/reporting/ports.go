// Package reporting — E10 services (F10.1 notifications + 11-02b dashboard /
// billable report / export framework). 11-02 owns the notification surface: the
// caller's in-app inbox (cursor list + read-state/kind filter), single mark-read,
// and bulk mark-all-read — all scope=self (a user only ever sees rows addressed
// to their SWP-USR-* or SWP-EMP-* id; the auto-dispatch in leave/OT/attendance
// targets whichever the principal carries).
//
// Mirrors the Phase-2 foundations audit-log list (keyset cursor on created_at,id)
// + the Phase-10 payroll slice shape (service → handler → routes → seed).
package reporting

import (
	"context"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
)

// --- filters ---

// NotificationFilter is the decoded GET /notifications query (cursor-paged,
// newest-first). ReadState is one of UNREAD/READ/ALL (default ALL); Kinds is the
// optional kind / kind__in set. RecipientIDs is filled by the service from the
// principal (scope=self) — never from the client.
type NotificationFilter struct {
	RecipientIDs  []string // [actorUserID, actorEmployeeID] — scope=self
	ReadState     *string  // UNREAD | READ | ALL (nil → ALL)
	Kinds         []string // kind or kind__in (empty → no kind filter)
	Limit         int
	CursorCreated *time.Time
	CursorID      *string
}

// --- repository port ---

// NotificationRepository is the data dependency for the notification service.
// It wraps the 11-01 sqlc notifications queries. List/Get/MarkRead/MarkAllRead
// are all recipient-scoped at the SQL level (scope=self defense-in-depth).
type NotificationRepository interface {
	// List returns up to limit rows for ANY of recipientIDs, newest-first. The
	// service passes limit+1 for the cursor probe. (sqlc's ListNotifications is
	// single-recipient; the repo fans out + merges for the user/employee pair.)
	List(ctx context.Context, f NotificationFilter, limit int) ([]dom.Notification, error)
	// MarkRead flips read_at null→now for (id ∈ recipientIDs); returns the row.
	// domain.ErrNotFound when no owned row matches (404 / scope=self).
	MarkRead(ctx context.Context, id string, recipientIDs []string) (dom.Notification, error)
	// MarkAllRead marks every unread row for recipientIDs (optional before cutoff)
	// and returns the affected count.
	MarkAllRead(ctx context.Context, recipientIDs []string, before *time.Time) (int, error)
}
