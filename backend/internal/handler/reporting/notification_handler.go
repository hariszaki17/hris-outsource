// Package reporting (handler) — hand-written chi handlers for the E10 notification
// surface: GET /notifications (cursor + read_state/kind filters), POST
// /notifications/{notification_id}:mark-read, POST /notifications:mark-all-read.
// Decode → service → httpx.WriteJSON; apperr envelopes flow through
// httpx.WriteError. The list writes the cursor envelope at the top level (FE reads
// query.data.data); mark-read wraps in {data}; mark-all-read returns
// {marked_count} (the openapi key). Mirrors the Phase-10 payroll handler.
//
// 11-02b EXTENDS this same Handler with dashboard/report/export methods — its
// routes append AFTER the notifications route block in server.go.
package reporting

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
)

// Handler holds the E10 reporting services. 11-02 wires the notification service;
// 11-02b adds dashboard/billable/export services to the same struct.
type Handler struct {
	notifications *svc.NotificationService
}

// NewHandler wires the handler to the notification service. (11-02b will extend
// the constructor signature with its services.)
func NewHandler(n *svc.NotificationService) *Handler {
	return &Handler{notifications: n}
}

// ListNotifications handles GET /notifications — cursor-paged, newest-first,
// scope=self. read_state (UNREAD/READ/ALL, default ALL) + kind / kind__in filters.
func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := svc.NotificationFilter{
		Limit: intParam(q.Get("limit")),
		Kinds: parseKinds(q.Get("kind"), q.Get("kind__in")),
	}
	if rs := q.Get("read_state"); rs != "" {
		f.ReadState = &rs
	}
	if cursor := q.Get("cursor"); cursor != "" {
		created, id, err := svc.DecodeNotificationCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		f.CursorCreated = created
		f.CursorID = id
	}

	rows, next, hasMore, err := h.notifications.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]notificationResponse, 0, len(rows))
	for _, n := range rows {
		items = append(items, toNotification(n))
	}
	httpx.WriteJSON(w, http.StatusOK, notificationPageResponse{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// MarkNotificationRead handles POST /notifications/{notification_id}:mark-read —
// flips read_at null→now (no-op if already read) and returns the updated
// Notification in a {data} envelope. 404 if not owned (scope=self).
func (h *Handler) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "notification_id")
	n, err := h.notifications.MarkRead(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[notificationResponse]{Data: toNotification(n)})
}

// MarkAllNotificationsRead handles POST /notifications:mark-all-read — marks every
// unread notification for the caller (optional before_timestamp cutoff) and
// returns {marked_count} (the openapi key).
func (h *Handler) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	var before *time.Time
	// Body is optional; tolerate empty / absent.
	if r.Body != nil {
		var req markAllReadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.BeforeTimestamp != nil {
			t, perr := time.Parse(time.RFC3339, *req.BeforeTimestamp)
			if perr != nil {
				httpx.WriteError(w, r, apperr.Invalid(map[string]string{
					"before_timestamp": "Format waktu tidak valid (RFC3339).",
				}))
				return
			}
			before = &t
		}
	}

	count, err := h.notifications.MarkAllRead(r.Context(), before)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, markAllReadResponse{MarkedCount: count})
}

// --- helpers ---

func intParam(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// parseKinds merges the single `kind` param and the comma-separated `kind__in`
// param into a deduped set (empty → no kind filter).
func parseKinds(kind, kindIn string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	add := func(k string) {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		out = append(out, k)
	}
	add(kind)
	for _, k := range strings.Split(kindIn, ",") {
		add(k)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
