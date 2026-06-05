// Package reporting holds the dependency-free domain types for the E10 slice
// (F10.1 notifications, F10.2 dashboards, F10.3 billable report, F10.4 export
// framework / SWP-NTF-* / SWP-EXP-*). These structs are shared between the
// reporting service and repository and map 1:1 onto the openapi
// E10-reporting/openapi.yaml component schemas (11-02 maps sqlc rows → these →
// DTOs).
//
// Convention (mirrors internal/domain/payroll + internal/domain/overtime):
// nullable columns are pointers (nil = null on the wire).
package reporting

import "time"

// NotificationKind is the notification event kind, pinned BYTE-FOR-BYTE to openapi
// schemas.NotificationKind (the app owns the enum; the notifications.kind column
// has NO CHECK so new kinds need no migration). All 19 v1 kinds below.
type NotificationKind string

const (
	NotifSchedulePublished             NotificationKind = "SCHEDULE_PUBLISHED"
	NotifScheduleChanged               NotificationKind = "SCHEDULE_CHANGED"
	NotifShiftReminder                 NotificationKind = "SHIFT_REMINDER"
	NotifLeaveRequestSubmitted         NotificationKind = "LEAVE_REQUEST_SUBMITTED"
	NotifLeaveApproved                 NotificationKind = "LEAVE_APPROVED"
	NotifLeaveRejected                 NotificationKind = "LEAVE_REJECTED"
	NotifOTRequestSubmitted            NotificationKind = "OT_REQUEST_SUBMITTED"
	NotifOTAutoDetected                NotificationKind = "OT_AUTO_DETECTED"
	NotifOTApproved                    NotificationKind = "OT_APPROVED"
	NotifOTRejected                    NotificationKind = "OT_REJECTED"
	NotifAttendanceVerifyNeeded        NotificationKind = "ATTENDANCE_VERIFY_NEEDED"
	NotifAttendanceCorrectionSubmitted NotificationKind = "ATTENDANCE_CORRECTION_SUBMITTED"
	NotifAttendanceAutoClosed          NotificationKind = "ATTENDANCE_AUTO_CLOSED"
	NotifHRChangeRequestSubmitted      NotificationKind = "HR_CHANGE_REQUEST_SUBMITTED"
	NotifAgreementExpiring             NotificationKind = "AGREEMENT_EXPIRING"
	NotifPlacementExpiring             NotificationKind = "PLACEMENT_EXPIRING"
	NotifPlacementLeaderChanged        NotificationKind = "PLACEMENT_LEADER_CHANGED"
	NotifExportReady                   NotificationKind = "EXPORT_READY"
	NotifExportFailed                  NotificationKind = "EXPORT_FAILED"
)

// DeepLink references a target screen in another epic (openapi schemas.DeepLink).
// The client routes on Epic + Path; EntityID is the prefixed id for deep-link copy
// (nullable when the notification has no entity). Path is always present (defaults
// to "" at the DB level).
type DeepLink struct {
	Epic     string  // 'E2'..'E8'
	EntityID *string // SWP-LR-* etc; nil = no entity
	Path     string  // client route path
}

// Actor is who triggered the underlying event (openapi Notification.actor). ID nil
// = system actor (cron / auto-detection); Label is the display name (or "system").
type Actor struct {
	ID    *string
	Label string
}

// Notification is one durable in-app notification row (openapi schemas.Notification).
// ReadAt nil = unread.
type Notification struct {
	ID         string
	Recipient  string // recipient_id (SWP-USR-* or SWP-EMP-*); not serialized
	Kind       NotificationKind
	Title      string
	Body       string
	DeepLink   DeepLink
	Actor      Actor
	IsCritical bool
	ReadAt     *time.Time // nil = unread
	CreatedAt  time.Time
}
