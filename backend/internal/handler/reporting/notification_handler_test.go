// Package reporting_test — E10 notification contract tests (F10.1), asserted
// byte-for-shape against docs/api/E10-reporting/openapi.yaml:
//
//	GET /notifications → 200 cursor envelope {data:[...], next_cursor, has_more};
//	    read_state=UNREAD returns only read_at==null rows; kind filter narrows;
//	    scope=self (a row for another recipient is NOT returned).
//	POST /notifications/{id}:mark-read → 200 {data:Notification} read_at non-null;
//	    second call (already read) preserves read_at; non-owned id → 404.
//	POST /notifications:mark-all-read → 200 {marked_count:N}; UNREAD then empty.
package reporting_test

import (
	"net/http"
	"testing"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// seedNotif plants one notification for a recipient (read=false → unread).
func seedNotif(h *harness, id, recipient string, kind dom.NotificationKind, created time.Time, read bool) {
	n := dom.Notification{
		ID:         id,
		Recipient:  recipient,
		Kind:       kind,
		Title:      "Judul",
		Body:       "Isi notifikasi.",
		DeepLink:   dom.DeepLink{Epic: "E6", EntityID: strp("SWP-LR-1042"), Path: "/leave-requests/SWP-LR-1042"},
		Actor:      dom.Actor{ID: strp("SWP-USR-3104"), Label: "Budi Santoso"},
		IsCritical: kind == dom.NotifLeaveRequestSubmitted,
		CreatedAt:  created,
	}
	if read {
		t := created.Add(time.Hour)
		n.ReadAt = &t
	}
	h.notifs.seed(n)
}

// hrPrincipal carries user id SWP-USR-9001 (the harness default). Notifications
// for that id resolve via scope=self.
const hrUserID = "SWP-USR-9001"

func TestListNotifications_CursorEnvelopeAndScopeSelf(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	base := time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC)
	// Two own rows (one unread, one read) + one for ANOTHER recipient.
	seedNotif(h, "SWP-NTF-880241", hrUserID, dom.NotifLeaveRequestSubmitted, base.Add(2*time.Minute), false)
	seedNotif(h, "SWP-NTF-880198", hrUserID, dom.NotifAttendanceVerifyNeeded, base, true)
	seedNotif(h, "SWP-NTF-999999", "SWP-USR-5555", dom.NotifLeaveApproved, base.Add(5*time.Minute), false)

	rr := h.do("GET", "/notifications", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Cursor envelope keys present.
	if _, ok := body["has_more"]; !ok {
		t.Errorf("missing has_more in envelope: %v", body)
	}
	if _, ok := body["next_cursor"]; !ok {
		t.Errorf("missing next_cursor key in envelope: %v", body)
	}
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %v", body["data"])
	}
	// scope=self: only the 2 own rows, NOT the foreign recipient's row.
	if len(data) != 2 {
		t.Fatalf("expected 2 own notifications, got %d: %v", len(data), data)
	}
	// Newest-first (the unread LEAVE_REQUEST_SUBMITTED at base+2m sorts first).
	first := data[0].(map[string]any)
	if first["id"] != "SWP-NTF-880241" {
		t.Errorf("first id = %v, want SWP-NTF-880241 (newest-first)", first["id"])
	}
	// Unread row: read_at is JSON null (present, no omitempty).
	if first["read_at"] != nil {
		t.Errorf("unread read_at = %v, want null", first["read_at"])
	}
	// deep_link + actor are always objects (openapi required).
	dl, ok := first["deep_link"].(map[string]any)
	if !ok {
		t.Fatalf("deep_link missing/not object: %v", first["deep_link"])
	}
	if dl["epic"] != "E6" || dl["path"] != "/leave-requests/SWP-LR-1042" || dl["entity_id"] != "SWP-LR-1042" {
		t.Errorf("deep_link = %v, want E6 / SWP-LR-1042 / path", dl)
	}
	actor, ok := first["actor"].(map[string]any)
	if !ok {
		t.Fatalf("actor missing/not object: %v", first["actor"])
	}
	if actor["label"] != "Budi Santoso" {
		t.Errorf("actor.label = %v, want Budi Santoso", actor["label"])
	}
	if first["is_critical"] != true {
		t.Errorf("is_critical = %v, want true", first["is_critical"])
	}

	// The READ row carries a non-null read_at string.
	second := data[1].(map[string]any)
	if second["read_at"] == nil {
		t.Errorf("read row read_at = nil, want a timestamp")
	}
}

func TestListNotifications_ReadStateUnreadFilter(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	base := time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC)
	seedNotif(h, "SWP-NTF-1", hrUserID, dom.NotifLeaveApproved, base.Add(2*time.Minute), false)
	seedNotif(h, "SWP-NTF-2", hrUserID, dom.NotifOTApproved, base, true)

	rr := h.do("GET", "/notifications?read_state=UNREAD", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("UNREAD filter: expected 1 row, got %d", len(data))
	}
	if data[0].(map[string]any)["id"] != "SWP-NTF-1" {
		t.Errorf("UNREAD row = %v, want SWP-NTF-1", data[0])
	}
}

func TestListNotifications_KindFilter(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	base := time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC)
	seedNotif(h, "SWP-NTF-1", hrUserID, dom.NotifLeaveApproved, base.Add(2*time.Minute), false)
	seedNotif(h, "SWP-NTF-2", hrUserID, dom.NotifOTApproved, base, false)

	rr := h.do("GET", "/notifications?kind=OT_APPROVED", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("kind filter: expected 1 row, got %d", len(data))
	}
	if data[0].(map[string]any)["kind"] != "OT_APPROVED" {
		t.Errorf("kind = %v, want OT_APPROVED", data[0])
	}
}

func TestListNotifications_CursorPaginates(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	base := time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC)
	// 3 rows, page size 2 → has_more true + a next_cursor.
	for i := 0; i < 3; i++ {
		seedNotif(h, "SWP-NTF-"+itoa(100+i), hrUserID, dom.NotifLeaveApproved, base.Add(time.Duration(i)*time.Minute), false)
	}
	rr := h.do("GET", "/notifications?limit=2", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := decodeBody(t, rr)
	if body["has_more"] != true {
		t.Errorf("has_more = %v, want true", body["has_more"])
	}
	cur, ok := body["next_cursor"].(string)
	if !ok || cur == "" {
		t.Fatalf("next_cursor missing on a paged response: %v", body["next_cursor"])
	}
	// Follow the cursor: the remaining row(s).
	rr2 := h.do("GET", "/notifications?limit=2&cursor="+cur, nil)
	body2 := decodeBody(t, rr2)
	if body2["has_more"] != false {
		t.Errorf("page 2 has_more = %v, want false", body2["has_more"])
	}
	if len(body2["data"].([]any)) != 1 {
		t.Errorf("page 2 rows = %d, want 1", len(body2["data"].([]any)))
	}
}

func TestMarkNotificationRead_FlipsAndNoOps(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	base := time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC)
	seedNotif(h, "SWP-NTF-880241", hrUserID, dom.NotifLeaveRequestSubmitted, base, false)

	rr := h.doWithHeaders("POST", "/notifications/SWP-NTF-880241:mark-read", nil,
		map[string]string{"Idempotency-Key": "11111111-1111-1111-1111-111111111111"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["id"] != "SWP-NTF-880241" {
		t.Errorf("id = %v, want SWP-NTF-880241", d["id"])
	}
	readAt1, ok := d["read_at"].(string)
	if !ok || readAt1 == "" {
		t.Fatalf("read_at = %v, want a timestamp after mark-read", d["read_at"])
	}

	// Second mark-read (already read) → 200, read_at preserved (no-op COALESCE).
	rr2 := h.do("POST", "/notifications/SWP-NTF-880241:mark-read", nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second mark-read expected 200, got %d", rr2.Code)
	}
	d2 := dataObject(t, rr2)
	if d2["read_at"] != readAt1 {
		t.Errorf("read_at changed on no-op: %v -> %v", readAt1, d2["read_at"])
	}
}

func TestMarkNotificationRead_NonOwned404(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	base := time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC)
	// Row belongs to ANOTHER recipient.
	seedNotif(h, "SWP-NTF-555", "SWP-USR-5555", dom.NotifLeaveApproved, base, false)

	rr := h.do("POST", "/notifications/SWP-NTF-555:mark-read", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-owned id, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "NOT_FOUND" {
		t.Errorf("code = %s, want NOT_FOUND", got)
	}
}

func TestMarkAllRead_CountAndUnreadEmpties(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	base := time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC)
	seedNotif(h, "SWP-NTF-1", hrUserID, dom.NotifLeaveApproved, base.Add(2*time.Minute), false)
	seedNotif(h, "SWP-NTF-2", hrUserID, dom.NotifOTApproved, base.Add(time.Minute), false)
	seedNotif(h, "SWP-NTF-3", hrUserID, dom.NotifAttendanceVerifyNeeded, base, true) // already read
	// Foreign recipient unread — must NOT be counted.
	seedNotif(h, "SWP-NTF-9", "SWP-USR-5555", dom.NotifLeaveApproved, base, false)

	rr := h.doWithHeaders("POST", "/notifications:mark-all-read", nil,
		map[string]string{"Idempotency-Key": "22222222-2222-2222-2222-222222222222"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	mc, ok := body["marked_count"].(float64)
	if !ok {
		t.Fatalf("marked_count missing/not a number: %v", body)
	}
	if int(mc) != 2 {
		t.Errorf("marked_count = %d, want 2 (the 2 own unread rows only)", int(mc))
	}

	// After mark-all-read, the UNREAD list is empty for the caller.
	rr2 := h.do("GET", "/notifications?read_state=UNREAD", nil)
	data := decodeBody(t, rr2)["data"].([]any)
	if len(data) != 0 {
		t.Errorf("UNREAD after mark-all-read = %d rows, want 0", len(data))
	}
}
