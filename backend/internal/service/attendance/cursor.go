// Package attendance — opaque cursor encode/decode for the attendance + corrections
// keyset lists. Attendance keys on (check_in_at DESC, id); corrections on
// (created_at DESC, id). The opaque base64(JSON) form mirrors httpx.EncodeCursor.
package attendance

import (
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

type attendanceCursor struct {
	CheckInAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

type correctionCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// encodeAttendanceCursor serializes the tail (check_in_at, id) to an opaque cursor.
func encodeAttendanceCursor(checkInAt time.Time, id string) (string, error) {
	return httpx.EncodeCursor(attendanceCursor{CheckInAt: checkInAt, ID: id})
}

// DecodeAttendanceCursor parses an opaque attendance cursor into (check_in_at, id)
// pointers (both nil on the first page / empty cursor).
func DecodeAttendanceCursor(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c attendanceCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.CheckInAt, &c.ID, nil
}

// encodeCorrectionCursor serializes the tail (created_at, id) to an opaque cursor.
func encodeCorrectionCursor(createdAt time.Time, id string) (string, error) {
	return httpx.EncodeCursor(correctionCursor{CreatedAt: createdAt, ID: id})
}

// DecodeCorrectionCursor parses an opaque correction cursor into (created_at, id)
// pointers (both nil on the first page / empty cursor).
func DecodeCorrectionCursor(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c correctionCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.CreatedAt, &c.ID, nil
}
