// Package ids formats and parses the SWP-<ENTITY>-<NUMERIC> identifiers
// (CONVENTIONS §4). IDs are OPAQUE to clients; only the server constructs them.
// The numeric portion is allocated per-prefix by the `swp_next_id()` Postgres
// function (see migrations) — repositories call the NextID query inside the
// same transaction as the insert, then Format the result.
package ids

import (
	"fmt"
	"strconv"
	"strings"
)

// Prefix is the per-entity ID prefix (CONVENTIONS §4 entity table).
type Prefix string

const (
	User                Prefix = "USR"
	AuditLog            Prefix = "AL"
	Employee            Prefix = "EMP"
	EmploymentAgreement Prefix = "AG"
	ClientCompany       Prefix = "CMP"
	ClientSite          Prefix = "SITE"
	ServiceLine         Prefix = "SVC"
	Position            Prefix = "POS"
	LeaveType           Prefix = "LT"
	AttendanceCode      Prefix = "AC"
	OvertimeRule        Prefix = "OTR"
	ChangeRequest       Prefix = "CHG"
	Placement           Prefix = "PL"
	ShiftLeaderAssign   Prefix = "SLA"
	ShiftMaster         Prefix = "SHF"
	ScheduleEntry       Prefix = "SCH"
	Attendance          Prefix = "ATT"
	AttendanceCorrect   Prefix = "COR"
	LeaveRequest        Prefix = "LR"
	LeaveQuota          Prefix = "LQ"
	Overtime            Prefix = "OT"
	PublicHoliday       Prefix = "HOL"
	Payslip             Prefix = "PS"
	Notification        Prefix = "NTF"
	ExportJob           Prefix = "EXP"
)

const namespace = "SWP"

// Format builds an opaque ID, e.g. Format(Employee, 1042) -> "SWP-EMP-1042".
func Format(p Prefix, n int64) string {
	return fmt.Sprintf("%s-%s-%d", namespace, p, n)
}

// Parse splits an ID back into prefix + numeric. Used internally only (e.g. to
// pick the right sequence); never expose the numeric to clients.
func Parse(id string) (Prefix, int64, error) {
	parts := strings.SplitN(id, "-", 3)
	if len(parts) != 3 || parts[0] != namespace {
		return "", 0, fmt.Errorf("malformed id %q", id)
	}
	n, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("malformed id numeric %q: %w", id, err)
	}
	return Prefix(parts[1]), n, nil
}
