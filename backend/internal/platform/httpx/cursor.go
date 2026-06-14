package httpx

import (
	"encoding/base64"
	"encoding/json"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
)

// Cursor pagination is the only list pagination in this system (CONVENTIONS §8,
// ENGINEERING D1). The cursor is an opaque base64url(JSON) blob that encodes the
// sort key + the last seen value, so the server can detect sort/filter mismatch.
//
// Page wraps a typed cursor payload. Each list endpoint defines its own payload
// struct (e.g. {CreatedAt, ID}); these helpers handle the opaque encoding.

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// PageResponse is the standard list envelope (CONVENTIONS §8).
type PageResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

// ClampLimit applies the documented default/max.
func ClampLimit(limit int) int {
	switch {
	case limit <= 0:
		return DefaultLimit
	case limit > MaxLimit:
		return MaxLimit
	default:
		return limit
	}
}

// EncodeCursor serializes a typed cursor payload to the opaque string form.
func EncodeCursor[T any](payload T) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", apperr.Internal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DecodeCursor parses an opaque cursor; a malformed cursor is a 400 CURSOR_MISMATCH.
func DecodeCursor[T any](cursor string, out *T) error {
	if cursor == "" {
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return cursorMismatch()
	}
	if err := json.Unmarshal(b, out); err != nil {
		return cursorMismatch()
	}
	return nil
}

func cursorMismatch() error {
	return &apperr.Error{Code: "CURSOR_MISMATCH", HTTPStatus: 400}
}
