// Package overtime (handler) — hand-written chi handlers for the 13 FE-used E7
// endpoints (9 overtime + 4 holidays). Decode → validate → service → httpx.WriteJSON;
// apperr envelopes flow through httpx.WriteError. List handlers write
// httpx.PageResponse directly (FE reads query.data.data); single-object GETs wrap in
// the {data} envelope (FE detail unwraps {data}). Mirrors the Phase-7 attendance +
// Phase-8 leave handlers.
package overtime

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/overtime"
)

// Handler holds the two E7 services (overtime workflow + holiday calendar).
type Handler struct {
	overtime *svc.OvertimeService
	holiday  *svc.HolidayService
}

// NewHandler wires the handler to its services.
func NewHandler(o *svc.OvertimeService, h *svc.HolidayService) *Handler {
	return &Handler{overtime: o, holiday: h}
}

// --- shared helpers ---

func decodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return apperr.Invalid(nil).WithCause(err)
	}
	return nil
}

// decodeOptionalJSON decodes a body that may be empty (optional note / is_override).
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

func boolPtrParam(s string) *bool {
	switch s {
	case "true":
		v := true
		return &v
	case "false":
		v := false
		return &v
	default:
		return nil
	}
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
