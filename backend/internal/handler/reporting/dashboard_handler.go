// Package reporting (handler) — GET /dashboards/me. Returns the role-shaped
// dashboard payload in a {data} envelope (the FE unwraps query.data.data and
// discriminates on data.role). Sets Cache-Control: private, max-age=30 (DB-6).
package reporting

import (
	"net/http"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// GetMyDashboard handles GET /dashboards/me. The service returns the concrete
// role payload (HrDashboard | LeaderDashboard | AgentDashboard); the handler maps
// it to its wire DTO and wraps in {data}.
func (h *Handler) GetMyDashboard(w http.ResponseWriter, r *http.Request) {
	payload, err := h.dashboard.GetMyDashboard(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=30")
	httpx.WriteJSON(w, http.StatusOK, dataResponse[any]{Data: toDashboard(payload)})
}
