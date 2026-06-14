// Package reporting (repository) — NotificationRepo implements
// svc.NotificationRepository over the 11-01 sqlc notifications queries. Maps sqlc
// rows → domain.Notification (re-nesting the flattened deep_link_*/actor_* columns
// into DeepLink/Actor; nullable read_at → *time.Time). Reads on the pool.
//
// sqlc's ListNotifications/MarkNotificationRead/MarkAllNotificationsRead are
// single-recipient; scope=self spans the principal's (user id, employee id) pair,
// so the repo fans out per recipient and merges. The set is at most 2 ids, so the
// fan-out is cheap and the keyset still holds after the in-memory merge-sort.
package reporting

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
)

// NotificationRepo is the sqlc-backed implementation of svc.NotificationRepository.
type NotificationRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.NotificationRepository = (*NotificationRepo)(nil)

// New returns a NotificationRepo backed by pool.
func New(pool *db.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// List fans out the cursor query over each recipient id, merges newest-first, and
// truncates to limit. read_state spans ALL/UNREAD/READ; the optional kind set is
// applied per-query (single kind) or in-memory (multi via kind__in).
func (r *NotificationRepo) List(ctx context.Context, f svc.NotificationFilter, limit int) ([]dom.Notification, error) {
	// A single kind pushes down to SQL; a multi-kind set is filtered in-memory
	// (the sqlc query takes one kind), still keyset-correct after the merge.
	var sqlKind *string
	if len(f.Kinds) == 1 {
		k := f.Kinds[0]
		sqlKind = &k
	}

	merged := make([]dom.Notification, 0, limit*len(f.RecipientIDs))
	for _, rid := range f.RecipientIDs {
		rows, err := r.q.ListNotifications(ctx, sqlcgen.ListNotificationsParams{
			RecipientID:     rid,
			ReadState:       f.ReadState,
			Kind:            sqlKind,
			CursorCreatedAt: f.CursorCreated,
			CursorID:        f.CursorID,
			RowLimit:        int32(limit),
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			n := mapNotification(row)
			if len(f.Kinds) > 1 && !kindIn(string(n.Kind), f.Kinds) {
				continue
			}
			merged = append(merged, n)
		}
	}

	// Merge-sort newest-first (created_at DESC, id DESC) and truncate to limit so
	// the multi-recipient page is a single keyset window.
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].CreatedAt.Equal(merged[j].CreatedAt) {
			return merged[i].ID > merged[j].ID
		}
		return merged[i].CreatedAt.After(merged[j].CreatedAt)
	})
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// MarkRead flips read_at for an id owned by ANY of recipientIDs. Tries each
// recipient (the id belongs to exactly one); pgx.ErrNoRows on all → ErrNotFound.
func (r *NotificationRepo) MarkRead(ctx context.Context, id string, recipientIDs []string) (dom.Notification, error) {
	for _, rid := range recipientIDs {
		row, err := r.q.MarkNotificationRead(ctx, sqlcgen.MarkNotificationReadParams{
			ID:          id,
			RecipientID: rid,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return dom.Notification{}, err
		}
		return mapNotification(row), nil
	}
	return dom.Notification{}, domain.ErrNotFound
}

// MarkAllRead marks unread rows for every recipient id and sums the counts.
func (r *NotificationRepo) MarkAllRead(ctx context.Context, recipientIDs []string, before *time.Time) (int, error) {
	total := 0
	for _, rid := range recipientIDs {
		n, err := r.q.MarkAllNotificationsRead(ctx, sqlcgen.MarkAllNotificationsReadParams{
			RecipientID: rid,
			Before:      before,
		})
		if err != nil {
			return 0, err
		}
		total += int(n)
	}
	return total, nil
}

// mapNotification re-nests a flat sqlc row into the domain Notification.
func mapNotification(row sqlcgen.Notification) dom.Notification {
	return dom.Notification{
		ID:        row.ID,
		Recipient: row.RecipientID,
		Kind:      dom.NotificationKind(row.Kind),
		Title:     row.Title,
		Body:      row.Body,
		DeepLink: dom.DeepLink{
			Epic:     deref(row.DeepLinkEpic),
			EntityID: row.DeepLinkEntityID,
			Path:     row.DeepLinkPath,
		},
		Actor: dom.Actor{
			ID:    row.ActorID,
			Label: row.ActorLabel,
		},
		IsCritical: row.IsCritical,
		ReadAt:     row.ReadAt,
		CreatedAt:  row.CreatedAt,
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func kindIn(k string, set []string) bool {
	for _, s := range set {
		if s == k {
			return true
		}
	}
	return false
}
