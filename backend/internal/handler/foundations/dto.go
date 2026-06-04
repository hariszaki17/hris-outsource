package foundations

import (
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- User DTOs ---

// userResponse is the User object in the E1 OpenAPI spec (snake_case, UPPER status).
type userResponse struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`      // ACTIVE | DISABLED (upper)
	EmployeeID  *string `json:"employee_id"` // null when unset
	FullName    string  `json:"full_name"`
	CompanyID   *string `json:"company_id"`    // null when unset
	CompanyName *string `json:"company_name"`  // null in Phase 2; resolved in E3+
	LastLoginAt *string `json:"last_login_at"` // RFC3339 or null
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// createUserRequest is the POST /users body.
type createUserRequest struct {
	Email               string `json:"email"`
	Role                string `json:"role"`
	EmployeeID          string `json:"employee_id"`
	SendInvitationEmail *bool  `json:"send_invitation_email"`
}

// updateUserRequest is the PATCH /users/{user_id} body.
type updateUserRequest struct {
	Email *string `json:"email"`
}

// changeRoleRequest is the POST /users/{user_id}:change-role body.
type changeRoleRequest struct {
	NewRole string  `json:"new_role"`
	Reason  *string `json:"reason"`
}

// reasonRequest is the body for :deactivate and :reactivate (optional reason).
type reasonRequest struct {
	Reason *string `json:"reason"`
}

// --- Audit-log DTOs ---

// auditSummaryResponse is one item in the GET /audit-log list.
type auditSummaryResponse struct {
	ID            string  `json:"id"`
	ActorUserID   *string `json:"actor_user_id"` // null for system actions
	ActorLabel    string  `json:"actor_label"`   // best-effort: role string or "system"
	Action        string  `json:"action"`
	EntityType    string  `json:"entity_type"`
	EntityID      string  `json:"entity_id"`
	ChangeSummary string  `json:"change_summary"`
	IP            *string `json:"ip"` // always null (column not in migration 00004)
	CreatedAt     string  `json:"created_at"`
}

// auditEntryResponse is the GET /audit-log/{id} detail response.
type auditEntryResponse struct {
	ID            string         `json:"id"`
	ActorUserID   *string        `json:"actor_user_id"`
	ActorLabel    string         `json:"actor_label"`
	Action        string         `json:"action"`
	EntityType    string         `json:"entity_type"`
	EntityID      string         `json:"entity_id"`
	ChangeSummary string         `json:"change_summary"`
	IP            *string        `json:"ip"`
	Before        map[string]any `json:"before"`
	After         map[string]any `json:"after"`
	RequestID     string         `json:"request_id"`
	CreatedAt     string         `json:"created_at"`
}

// --- Platform-settings DTO ---

// platformSettingEntry is one setting value (value + label + locked).
type platformSettingEntry struct {
	Value  string `json:"value"`
	Label  string `json:"label"`
	Locked bool   `json:"locked"`
}

// platformSettingsResponse maps the 7 keys to their setting entries.
type platformSettingsResponse struct {
	Locale           platformSettingEntry `json:"locale"`
	Timezone         platformSettingEntry `json:"timezone"`
	DateFormat       platformSettingEntry `json:"date_format"`
	Currency         platformSettingEntry `json:"currency"`
	Version          platformSettingEntry `json:"version"`
	Stack            platformSettingEntry `json:"stack"`
	LegacyDataSource platformSettingEntry `json:"legacy_data_source"`
}

// --- helpers ---

func toUserResponse(u domain.User) userResponse {
	var employeeID *string
	if u.EmployeeID != "" {
		s := u.EmployeeID
		employeeID = &s
	}
	var companyID *string
	if u.CompanyID != "" {
		s := u.CompanyID
		companyID = &s
	}
	var lastLoginAt *string
	if u.LastLoginAt != nil {
		s := u.LastLoginAt.UTC().Format(time.RFC3339)
		lastLoginAt = &s
	}
	return userResponse{
		ID:          u.ID,
		Email:       u.Email,
		Role:        string(u.Role),
		Status:      strings.ToUpper(u.Status), // "active" -> "ACTIVE"
		EmployeeID:  employeeID,
		FullName:    u.FullName,
		CompanyID:   companyID,
		CompanyName: nil, // TODO(Phase-3): resolved via companies endpoint
		LastLoginAt: lastLoginAt,
		CreatedAt:   u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// toAuditSummary converts a domain.AuditEntry to the list DTO.
// actor_label: if ActorUserID is nil -> "system", else "<actor_role>:<actor_user_id>".
// Full name resolution needs the E2 employee endpoint; documented in SUMMARY.
// change_summary: "action: <entity_type>/<entity_id>" best-effort one-liner.
// ip: always nil — the audit_log table has no ip column (migration 00004 omitted it).
func toAuditSummary(e domain.AuditEntry) auditSummaryResponse {
	return auditSummaryResponse{
		ID:            e.ID,
		ActorUserID:   e.ActorUserID,
		ActorLabel:    actorLabel(e.ActorUserID, e.ActorRole),
		Action:        e.Action,
		EntityType:    e.EntityType,
		EntityID:      e.EntityID,
		ChangeSummary: buildChangeSummary(e),
		IP:            nil,
		CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toAuditEntry(e domain.AuditEntry) auditEntryResponse {
	reqID := ""
	if e.RequestID != nil {
		reqID = *e.RequestID
	}
	return auditEntryResponse{
		ID:            e.ID,
		ActorUserID:   e.ActorUserID,
		ActorLabel:    actorLabel(e.ActorUserID, e.ActorRole),
		Action:        e.Action,
		EntityType:    e.EntityType,
		EntityID:      e.EntityID,
		ChangeSummary: buildChangeSummary(e),
		IP:            nil,
		Before:        e.Before,
		After:         e.After,
		RequestID:     reqID,
		CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func actorLabel(actorUserID, actorRole *string) string {
	if actorUserID == nil {
		return "system"
	}
	if actorRole != nil && *actorRole != "" {
		return *actorRole + ":" + *actorUserID
	}
	return *actorUserID
}

// buildChangeSummary derives a one-line description from the before/after maps.
// For rows that have richer maps, it iterates keys. Best-effort only.
func buildChangeSummary(e domain.AuditEntry) string {
	if len(e.After) == 0 && len(e.Before) == 0 {
		return e.Action + ": " + e.EntityType + "/" + e.EntityID
	}
	var parts []string
	for k, after := range e.After {
		if before, ok := e.Before[k]; ok {
			parts = append(parts, k+": "+stringify(before)+" → "+stringify(after))
		} else {
			parts = append(parts, k+": "+stringify(after))
		}
	}
	if len(parts) == 0 {
		return e.Action + ": " + e.EntityType + "/" + e.EntityID
	}
	return strings.Join(parts, "; ")
}

func stringify(v any) string {
	if v == nil {
		return "null"
	}
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return "…"
	}
}

// toPlatformSettingsResponse maps the flat []domain.PlatformSetting slice (sorted)
// into the keyed response object. Unknown keys are silently ignored.
func toPlatformSettingsResponse(settings []domain.PlatformSetting) platformSettingsResponse {
	m := make(map[string]platformSettingEntry, len(settings))
	for _, s := range settings {
		m[s.Key] = platformSettingEntry{Value: s.Value, Label: s.Label, Locked: s.Locked}
	}
	return platformSettingsResponse{
		Locale:           m["locale"],
		Timezone:         m["timezone"],
		DateFormat:       m["date_format"],
		Currency:         m["currency"],
		Version:          m["version"],
		Stack:            m["stack"],
		LegacyDataSource: m["legacy_data_source"],
	}
}
