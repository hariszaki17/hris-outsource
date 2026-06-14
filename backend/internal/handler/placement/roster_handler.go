package placement

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// GetCompanyRoster handles GET /client-companies/{company_id}/roster.
func (h *Handler) GetCompanyRoster(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "company_id")
	q := r.URL.Query()

	filter := domain.PlacementFilter{
		Position:       strPtrParam(q.Get("position")),
		Status:         strPtrParam(q.Get("status")),
		StatusIn:       csvParam(q.Get("status__in")),
		Q:              strPtrParam(q.Get("q")),
		IncludeHistory: boolParam(q.Get("include_history")),
		Limit:          intParam(q.Get("limit")),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		var c struct {
			StatusChangedAt time.Time `json:"c"`
			ID              string    `json:"i"`
		}
		if err := httpx.DecodeCursor(cursor, &c); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorStatusChangedAt = &c.StatusChangedAt
		filter.CursorID = &c.ID
	}

	roster, err := h.leaders.GetCompanyRoster(r.Context(), companyID, filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	today := jakartaToday()
	resp := companyRosterResponse{
		CompanyID:   roster.CompanyID,
		CompanyName: roster.CompanyName,
		Placements:  make([]placementResponse, 0, len(roster.Placements)),
		NextCursor:  roster.NextCursor,
		HasMore:     roster.HasMore,
		Summary:     toRosterSummaryResponse(roster.Summary),
	}
	for _, p := range roster.Placements {
		resp.Placements = append(resp.Placements, toPlacementResponse(p, today))
	}
	if roster.CurrentLeader != nil {
		resp.CurrentShiftLeader = toShiftLeaderSummaryResponse(*roster.CurrentLeader)
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}
