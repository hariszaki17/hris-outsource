// Package reporting — NotificationService: the caller's in-app inbox. Every
// method is scope=self — the recipient set is derived from the principal (their
// SWP-USR-* user id AND their SWP-EMP-* employee id, since auto-dispatched
// notifications from prior phases target the submitter's employee id while
// system/HR notifications target the user id). A caller NEVER sees another
// recipient's rows, and mark-read on a non-owned id → 404.
package reporting

import (
	"context"
	"errors"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// NotificationService implements the F10.1 notification surface.
type NotificationService struct {
	repo NotificationRepository
}

// NewNotificationService wires the notification service.
func NewNotificationService(repo NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

// List returns one cursor page of the caller's notifications (newest-first),
// honoring the read_state + kind filters. scope=self: the recipient set is the
// principal's (user id, employee id) pair — the SQL filters on it, so no other
// recipient's rows can leak.
func (s *NotificationService) List(ctx context.Context, f NotificationFilter) ([]dom.Notification, *string, bool, error) {
	recipients, err := selfRecipients(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	f.RecipientIDs = recipients

	limit := httpx.ClampLimit(f.Limit)
	rows, err := s.repo.List(ctx, f, limit+1)
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
		c, cerr := encodeNotificationCursor(last.CreatedAt, last.ID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	return rows, next, hasMore, nil
}

// MarkRead flips a single notification read_at null→now (no-op 200 if already
// read — the COALESCE in SQL preserves the first read_at) and returns the updated
// row. 404 if the id is not owned by the caller (scope=self).
func (s *NotificationService) MarkRead(ctx context.Context, id string) (dom.Notification, error) {
	recipients, err := selfRecipients(ctx)
	if err != nil {
		return dom.Notification{}, err
	}
	row, err := s.repo.MarkRead(ctx, id, recipients)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.Notification{}, apperr.NotFound()
	}
	if err != nil {
		return dom.Notification{}, apperr.Internal(err)
	}
	return row, nil
}

// MarkAllRead marks every unread notification for the caller as read (optional
// before cutoff) and returns the count transitioned.
func (s *NotificationService) MarkAllRead(ctx context.Context, before *time.Time) (int, error) {
	recipients, err := selfRecipients(ctx)
	if err != nil {
		return 0, err
	}
	n, err := s.repo.MarkAllRead(ctx, recipients, before)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	return n, nil
}

// selfRecipients resolves the caller's recipient-id set: their user id plus their
// employee id (when present). This is the scope=self key — a notification
// dispatched to the submitter's SWP-EMP-* is visible to that logged-in user,
// while system/HR notifications target SWP-USR-*.
func selfRecipients(ctx context.Context) ([]string, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}
	ids := make([]string, 0, 2)
	if p.UserID != "" {
		ids = append(ids, p.UserID)
	}
	if p.EmployeeID != "" {
		ids = append(ids, p.EmployeeID)
	}
	if len(ids) == 0 {
		return nil, apperr.Unauthenticated()
	}
	return ids, nil
}

// --- cursor (keyset on (created_at DESC, id DESC), mirrors the audit-log list) ---

type notificationCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

func encodeNotificationCursor(createdAt time.Time, id string) (string, error) {
	return httpx.EncodeCursor(notificationCursor{CreatedAt: createdAt, ID: id})
}

// DecodeNotificationCursor parses an opaque cursor into (created_at, id) pointers.
func DecodeNotificationCursor(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c notificationCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.CreatedAt, &c.ID, nil
}
