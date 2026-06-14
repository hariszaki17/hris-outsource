// Package approval (handler) — hand-written chi handlers for the 8 E11 endpoints
// (F11.1/F11.2/F11.3). Decode → validate → service → httpx.WriteJSON; apperr
// envelopes flow through httpx.WriteError. Mirrors the Phase-7/E6 handlers.
package approval

import (
	"encoding/json"
	"net/http"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/approval"
)

// Handler holds the approval service (one struct serves templates + instances +
// execution actions).
type Handler struct {
	svc *svc.ApprovalService
}

// NewHandler wires the handler to the approval service.
func NewHandler(s *svc.ApprovalService) *Handler {
	return &Handler{svc: s}
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
