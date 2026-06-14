// Package approval (handler) — request/response DTOs + snake_case mappers for the
// 8 E11 endpoints. Required-nullable openapi fields use pointers; denormalized
// display fields use omitempty. Timestamps are UTC RFC3339. The DTOs match
// docs/api/E11-approvals/openapi.yaml.
package approval

import (
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
)

// --- request bodies ---

// approvalTemplateUpsert is the PUT body (openapi ApprovalTemplateUpsert).
type approvalTemplateUpsert struct {
	Lines []upsertLine `json:"lines"`
}

type upsertLine struct {
	Members []string `json:"members"`
}

// approveBody is the optional POST :approve body.
type approveBody struct {
	Note string `json:"note"`
}

// decisionReason is the POST :reject / :bypass body (openapi DecisionReason).
type decisionReason struct {
	Reason string `json:"reason"`
}

// --- response: template ---

type lineMemberResponse struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name,omitempty"`
	Active      bool   `json:"active"`
}

type lineResponse struct {
	ID      string               `json:"id"`
	LineNo  int                  `json:"line_no"`
	Members []lineMemberResponse `json:"members"`
}

type templateResponse struct {
	ID        string         `json:"id"`
	CompanyID string         `json:"company_id"`
	Version   int            `json:"version"`
	Lines     []lineResponse `json:"lines"`
	CreatedBy *string        `json:"created_by,omitempty"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

// --- response: instance ---

type instanceResponse struct {
	ID              string  `json:"id"`
	RequestType     string  `json:"request_type"`
	RequestID       string  `json:"request_id"`
	CompanyID       *string `json:"company_id"`
	TemplateID      *string `json:"template_id"`
	TemplateVersion *int    `json:"template_version,omitempty"`
	CurrentLine     int     `json:"current_line"`
	LineCount       int     `json:"line_count"`
	Status          string  `json:"status"`
	RequesterID     *string `json:"requester_id,omitempty"`
	Summary         string  `json:"summary,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type actionResponse struct {
	ID              string  `json:"id"`
	LineNo          int     `json:"line_no"`
	TemplateVersion *int    `json:"template_version,omitempty"`
	ActorUserID     *string `json:"actor_user_id"`
	ActorName       string  `json:"actor_name,omitempty"`
	Action          string  `json:"action"`
	Reason          *string `json:"reason"`
	CreatedAt       string  `json:"created_at"`
}

type instanceDetailResponse struct {
	instanceResponse
	Lines   []lineResponse   `json:"lines"`
	Actions []actionResponse `json:"actions"`
}

// --- mappers ---

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func toLineResponse(l dom.Line) lineResponse {
	out := lineResponse{
		ID:      l.ID,
		LineNo:  l.LineNo,
		Members: make([]lineMemberResponse, 0, len(l.Members)),
	}
	for _, m := range l.Members {
		out.Members = append(out.Members, lineMemberResponse{
			UserID:      m.UserID,
			DisplayName: m.DisplayName,
			Active:      m.Active,
		})
	}
	return out
}

func toTemplateResponse(t dom.Template) templateResponse {
	out := templateResponse{
		ID:        t.ID,
		CompanyID: t.CompanyID,
		Version:   t.Version,
		Lines:     make([]lineResponse, 0, len(t.Lines)),
		CreatedBy: t.CreatedBy,
		CreatedAt: rfc3339(t.CreatedAt),
		UpdatedAt: rfc3339(t.UpdatedAt),
	}
	for _, l := range t.Lines {
		out.Lines = append(out.Lines, toLineResponse(l))
	}
	return out
}

func toInstanceResponse(i dom.Instance) instanceResponse {
	return instanceResponse{
		ID:              i.ID,
		RequestType:     string(i.RequestType),
		RequestID:       i.RequestID,
		CompanyID:       i.CompanyID,
		TemplateID:      i.TemplateID,
		TemplateVersion: i.TemplateVersion,
		CurrentLine:     i.CurrentLine,
		LineCount:       i.LineCount,
		Status:          string(i.Status),
		RequesterID:     i.RequesterID,
		Summary:         i.Summary,
		CreatedAt:       rfc3339(i.CreatedAt),
		UpdatedAt:       rfc3339(i.UpdatedAt),
	}
}

func toActionResponse(a dom.Action) actionResponse {
	return actionResponse{
		ID:              a.ID,
		LineNo:          a.LineNo,
		TemplateVersion: a.TemplateVersion,
		ActorUserID:     a.ActorUserID,
		ActorName:       a.ActorName,
		Action:          string(a.Action),
		Reason:          a.Reason,
		CreatedAt:       rfc3339(a.CreatedAt),
	}
}

func toInstanceDetailResponse(d dom.InstanceDetail) instanceDetailResponse {
	out := instanceDetailResponse{
		instanceResponse: toInstanceResponse(d.Instance),
		Lines:            make([]lineResponse, 0, len(d.Lines)),
		Actions:          make([]actionResponse, 0, len(d.Actions)),
	}
	for _, l := range d.Lines {
		out.Lines = append(out.Lines, toLineResponse(l))
	}
	for _, a := range d.Actions {
		out.Actions = append(out.Actions, toActionResponse(a))
	}
	return out
}
