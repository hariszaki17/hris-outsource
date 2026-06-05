// Package reporting (handler) — request/response DTOs + snake_case mappers
// matching docs/api/E10-reporting/openapi.yaml byte-for-shape. The Notification
// wire shape: read_at is a pointer WITHOUT omitempty (serializes JSON null when
// unread); deep_link + actor are ALWAYS objects (the openapi marks both required).
package reporting

import (
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
)

// --- generic envelopes ---

type dataResponse[T any] struct {
	Data T `json:"data"`
}

// notificationPageResponse is the GET /notifications cursor envelope
// (CONVENTIONS §8 / openapi CursorPage allOf).
type notificationPageResponse struct {
	Data       []notificationResponse `json:"data"`
	NextCursor *string                `json:"next_cursor"`
	HasMore    bool                   `json:"has_more"`
}

// --- request bodies ---

type markAllReadRequest struct {
	BeforeTimestamp *string `json:"before_timestamp"`
}

// --- response: Notification (openapi schemas.Notification) ---

type deepLinkResponse struct {
	Epic     string  `json:"epic"`
	EntityID *string `json:"entity_id"` // null when no entity (no omitempty)
	Path     string  `json:"path"`
}

type actorResponse struct {
	ID    *string `json:"id"` // null = system actor (no omitempty)
	Label string  `json:"label"`
}

type notificationResponse struct {
	ID         string           `json:"id"`
	Kind       string           `json:"kind"`
	Title      string           `json:"title"`
	Body       string           `json:"body"`
	ReadAt     *string          `json:"read_at"` // null = unread (no omitempty)
	CreatedAt  string           `json:"created_at"`
	DeepLink   deepLinkResponse `json:"deep_link"`
	Actor      actorResponse    `json:"actor"`
	IsCritical bool             `json:"is_critical"`
}

// --- response: mark-all-read ---

type markAllReadResponse struct {
	MarkedCount int `json:"marked_count"`
}

// --- mappers ---

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func rfc3339Ptr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func toNotification(n dom.Notification) notificationResponse {
	return notificationResponse{
		ID:        n.ID,
		Kind:      string(n.Kind),
		Title:     n.Title,
		Body:      n.Body,
		ReadAt:    rfc3339Ptr(n.ReadAt),
		CreatedAt: rfc3339(n.CreatedAt),
		DeepLink: deepLinkResponse{
			Epic:     n.DeepLink.Epic,
			EntityID: n.DeepLink.EntityID,
			Path:     n.DeepLink.Path,
		},
		Actor: actorResponse{
			ID:    n.Actor.ID,
			Label: n.Actor.Label,
		},
		IsCritical: n.IsCritical,
	}
}
