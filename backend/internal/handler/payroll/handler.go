// Package payroll (handler) — hand-written chi handlers for the 5 FE-used E8
// endpoints: GET /payslips, GET /payslips/{id}, GET/POST /payslips/{id}/audit-notes,
// POST /payslips:export. Decode → service → httpx.WriteJSON; apperr envelopes flow
// through httpx.WriteError. List handlers write the cursor envelope at the top level
// (FE reads query.data.data); the single-object GET wraps in {data} (FE unwraps
// .data). Only the 5 FE ops are routed — no 405-immutable / PDF / forward-export
// handlers (out of scope). Mirrors the Phase-2 foundations + Phase-9 overtime
// handlers.
package payroll

import (
	"encoding/json"
	"net/http"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// Handler holds the two E8 services (payslip read/notes + async export).
type Handler struct {
	payslip *svc.PayslipService
	export  *svc.ExportService
}

// NewHandler wires the handler to its services.
func NewHandler(p *svc.PayslipService, e *svc.ExportService) *Handler {
	return &Handler{payslip: p, export: e}
}

// --- shared helpers ---

func decodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
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

func intPtrParam(s string) *int {
	if s == "" {
		return nil
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return nil
		}
		n = n*10 + int(c-'0')
	}
	return &n
}

func intParam(s string) int {
	if p := intPtrParam(s); p != nil {
		return *p
	}
	return 0
}

// splitPeriod parses a YYYY-MM period filter into (year, month) pointers; returns
// (nil, nil) for an empty or malformed value (the malformed case simply yields no
// filter, matching the cursor-style leniency of the other epics).
func splitPeriod(period string) (year, month *int) {
	if len(period) != 7 || period[4] != '-' {
		return nil, nil
	}
	y := intPtrParam(period[0:4])
	m := intPtrParam(period[5:7])
	return y, m
}
