// Package overtime — shared service helpers (limit clamp, principal extraction,
// error passthrough, cursor encode/decode, stateConflict). Mirrors the Phase-8
// leave / Phase-7 attendance helpers. Overtime cursors keyset on (created_at DESC,
// id); holiday cursors keyset on (holiday_date ASC, id).
package overtime

import (
	"context"
	"strconv"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
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

// actorEmployeeID resolves the acting employee id (nil if absent).
func actorEmployeeID(ctx context.Context) *string {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.EmployeeID != "" {
		id := p.EmployeeID
		return &id
	}
	return nil
}

// actorName resolves a display name for the approver from the principal user id
// (best-effort; the approver_name column is denormalized for the timeline DTO).
func actorName(ctx context.Context) *string {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.UserID != "" {
		id := p.UserID
		return &id
	}
	return nil
}

func ptrStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func itoa(n int) string { return strconv.Itoa(n) }

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

// stateConflict is the 409 for a wrong/terminal-state transition (carries the
// current status). The openapi maps wrong-state actions to 409 CONFLICT.
func stateConflict(cur dom.OvertimeStatus) error {
	return &apperr.Error{
		Code:       "CONFLICT",
		HTTPStatus: 409,
		Message:    "Lembur sudah pada status lain.",
		Fields:     map[string]string{"status": string(cur)},
	}
}

// --- overtime cursor (keyset on (created_at DESC, id)) ---

type overtimeCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

func encodeOvertimeCursor(createdAt time.Time, id string) (string, error) {
	return httpx.EncodeCursor(overtimeCursor{CreatedAt: createdAt, ID: id})
}

// DecodeOvertimeCursor parses an opaque overtime cursor into (created_at, id) pointers.
func DecodeOvertimeCursor(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c overtimeCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.CreatedAt, &c.ID, nil
}

// --- holiday cursor (keyset on (holiday_date ASC, id)) ---

type holidayCursor struct {
	Date time.Time `json:"d"`
	ID   string    `json:"i"`
}

func encodeHolidayCursor(date time.Time, id string) (string, error) {
	return httpx.EncodeCursor(holidayCursor{Date: date, ID: id})
}

// DecodeHolidayCursor parses an opaque holiday cursor into (date, id) pointers.
func DecodeHolidayCursor(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c holidayCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.Date, &c.ID, nil
}
