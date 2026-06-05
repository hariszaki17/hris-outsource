// Package leave — shared service helpers (limit clamp, principal extraction, error
// passthrough, cursor encode/decode). Mirrors the Phase-7 attendance helpers.
package leave

import (
	"strconv"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"

	"context"
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

// actorEmployeeID resolves the acting employee id (nil if absent).
func actorEmployeeID(ctx context.Context) *string {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.EmployeeID != "" {
		id := p.EmployeeID
		return &id
	}
	return nil
}

// actorUserID resolves the acting user id (empty if absent) — the notification
// actor (who approved/rejected). Returns the SWP-USR-* of the principal.
func actorUserID(ctx context.Context) string {
	if p, ok := auth.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return ""
}

func ptrStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func itoa(n int) string { return strconv.Itoa(n) }

// leaveDateBody builds a single-line notification body summarizing the leave
// date range (Asia/Jakarta-neutral YYYY-MM-DD; the FE renders TZ-aware copy).
func leaveDateBody(prefix string, start, end time.Time) string {
	s := start.Format("2006-01-02")
	e := end.Format("2006-01-02")
	if s == e {
		return prefix + " (" + s + ")."
	}
	return prefix + " (" + s + " s/d " + e + ")."
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

// --- cursors (keyset on (created_at DESC, id) for both requests and quotas) ---

type listCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

func encodeRequestCursor(createdAt time.Time, id string) (string, error) {
	return httpx.EncodeCursor(listCursor{CreatedAt: createdAt, ID: id})
}

// DecodeRequestCursor parses an opaque request cursor into (created_at, id) pointers.
func DecodeRequestCursor(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c listCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.CreatedAt, &c.ID, nil
}

func encodeQuotaCursor(createdAt time.Time, id string) (string, error) {
	return httpx.EncodeCursor(listCursor{CreatedAt: createdAt, ID: id})
}

// DecodeQuotaCursor parses an opaque quota cursor into (created_at, id) pointers.
func DecodeQuotaCursor(cursor string) (*time.Time, *string, error) {
	return DecodeRequestCursor(cursor)
}
