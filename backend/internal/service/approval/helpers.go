// Package approval — shared service helpers (limit clamp, principal extraction,
// error passthrough, cursor encode/decode, instance summary). Mirrors the leave
// service helpers.
package approval

import (
	"context"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// clampLimit applies the documented default(50)/max(200).
func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return 50
	case limit > 200:
		return 200
	default:
		return limit
	}
}

// actorUserIDPtr resolves the acting user id as a pointer (nil if absent) — the
// approval_actions.actor_user_id (SWP-USR-*).
func actorUserIDPtr(ctx context.Context) *string {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.UserID != "" {
		id := p.UserID
		return &id
	}
	return nil
}

// actorUserID resolves the acting user id (empty if absent) — the notification actor.
func actorUserID(ctx context.Context) string {
	if p, ok := auth.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return ""
}

// asAppErr passes *apperr.Error through, wrapping anything else as 500.
func asAppErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.Internal(err)
}

// summaryFor derives a Bahasa inbox-rendering summary string from the request
// type (the domain request CRUD lives in E6/E7; a richer per-request summary is a
// follow-up — the openapi marks summary readOnly/optional).
func summaryFor(rt dom.RequestType) string {
	switch rt {
	case dom.RequestTypeLeave:
		return "Pengajuan cuti"
	case dom.RequestTypeOvertime:
		return "Pengajuan lembur"
	default:
		return string(rt)
	}
}

// deref returns the pointed-to string, or "" for nil.
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// strOrNil maps "" → nil.
func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// --- cursor (keyset on (created_at DESC, id DESC), mirrors ListLeaveRequests) ---

type listCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

func encodeInstanceCursor(createdAt time.Time, id string) (string, error) {
	return httpx.EncodeCursor(listCursor{CreatedAt: createdAt, ID: id})
}

// DecodeInstanceCursor parses an opaque instance cursor into (created_at, id) pointers.
func DecodeInstanceCursor(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c listCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.CreatedAt, &c.ID, nil
}
