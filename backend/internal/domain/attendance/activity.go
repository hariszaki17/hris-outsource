// Package attendance — F5.8 attendance activity log domain type (SWP-ACT-*).
// An AttendanceActivity is one free-text note an agent logs onto their own OPEN
// attendance record during a shift; one Attendance has many activities. recorded_at
// is server-set (Asia/Jakarta) — the capture time (AA-5). Agent clock-out is gated on
// >=1 non-deleted activity (INV-7 / AA-7).
package attendance

import "time"

// AttendanceActivity is the domain entity for one activity note (openapi
// AttendanceActivity). Soft-delete (deleted_at) is enforced in the repo/query layer;
// the domain type only carries the live-row projection.
type AttendanceActivity struct {
	ID           string
	AttendanceID string
	EmployeeID   string // creator (the agent); set from the JWT principal (AA-2)
	Note         string
	RecordedAt   time.Time // server-set capture time (AA-5)
	CreatedAt    time.Time
}
