// Package leave (handler) — hand-written chi handlers for the 10 FE-used E6
// endpoints (F6.1/F6.2/F6.3). Decode → validate → service → httpx.WriteJSON; apperr
// envelopes flow through httpx.WriteError. Mirrors the Phase-7 attendance handler.
package leave

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// Handler holds the E6 services (one struct serves requests + grants/balances +
// deprecated quotas + calendar).
type Handler struct {
	leave    *svc.LeaveService
	quota    *svc.QuotaService // DEPRECATED 2026-06-08 — /leave-quotas* only
	grant    *svc.GrantService // F6.1 grant-lot ledger + balances (live path)
	calendar *svc.CalendarService
}

// NewHandler wires the handler to its services.
func NewHandler(l *svc.LeaveService, q *svc.QuotaService, g *svc.GrantService, c *svc.CalendarService) *Handler {
	return &Handler{leave: l, quota: q, grant: g, calendar: c}
}

// --- shared helpers ---

func decodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return apperr.Invalid(nil).WithCause(err)
	}
	return nil
}

// decodeOptionalJSON decodes a body that may be empty (optional note).
func decodeOptionalJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if err.Error() == "EOF" {
			return nil
		}
		return apperr.Invalid(nil).WithCause(err)
	}
	return nil
}

func strPtrParam(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intParam(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func intPtrParam(s string) *int {
	if s == "" {
		return nil
	}
	n := intParam(s)
	return &n
}

func csvParam(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	cur := ""
	for _, c := range s {
		if c == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func parseDateParam(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}
