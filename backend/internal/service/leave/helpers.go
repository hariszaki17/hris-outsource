// Package leave — shared service helpers (limit clamp, principal extraction, error
// passthrough, cursor encode/decode). Mirrors the Phase-7 attendance helpers.
package leave

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// allocSnapshot is the jsonb shape persisted in leave_requests.balance_allocation
// (mirrors the repository allocLine + the openapi BalanceCheck.allocation item).
type allocSnapshot struct {
	GrantID   string `json:"grant_id"`
	Days      int    `json:"days"`
	ExpiresAt string `json:"expires_at"`
}

// marshalAllocation serializes the FIFO split for the balance_allocation column. A
// nil/empty allocation marshals to nil (clears the column).
func marshalAllocation(alloc []dom.AllocationLine) ([]byte, error) {
	if len(alloc) == 0 {
		return nil, nil
	}
	lines := make([]allocSnapshot, 0, len(alloc))
	for _, a := range alloc {
		lines = append(lines, allocSnapshot{GrantID: a.GrantID, Days: a.Days, ExpiresAt: a.ExpiresAt.Format("2006-01-02")})
	}
	return json.Marshal(lines)
}

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

// --- grant cursor (keyset on (expires_at ASC, id) — FIFO-aligned) ---

type grantCursor struct {
	ExpiresAt time.Time `json:"e"`
	ID        string    `json:"i"`
}

func encodeGrantCursor(expiresAt time.Time, id string) (string, error) {
	return httpx.EncodeCursor(grantCursor{ExpiresAt: expiresAt, ID: id})
}

// DecodeGrantCursor parses an opaque grant cursor into (expires_at, id) pointers.
func DecodeGrantCursor(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c grantCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.ExpiresAt, &c.ID, nil
}

// --- balance-list cursor (keyset on (full_name ASC, employee_id ASC)) ---

type balanceCursor struct {
	FullName string `json:"n"`
	ID       string `json:"i"`
}

func encodeBalanceCursor(fullName, id string) (string, error) {
	return httpx.EncodeCursor(balanceCursor{FullName: fullName, ID: id})
}

// DecodeBalanceCursor parses an opaque balance-list cursor into (full_name, id) pointers.
func DecodeBalanceCursor(cursor string) (*string, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c balanceCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.FullName, &c.ID, nil
}

// actorUserIDPtr resolves the acting user id as a pointer (nil if absent) — the
// grant's created_by.
func actorUserIDPtr(ctx context.Context) *string {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.UserID != "" {
		id := p.UserID
		return &id
	}
	return nil
}
