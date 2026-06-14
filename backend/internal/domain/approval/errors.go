// Package approval — typed domain errors for the E11 engine, mapped to the
// apperr contract codes (E11-approvals/openapi.yaml §Error):
//   - APPROVAL_LINE_INVALID  422 — empty line / inactive member (TM-3).
//   - LINE_ALREADY_CLEARED   409 — instance terminal / line already cleared (EX-11).
//   - SELF_APPROVAL_FORBIDDEN 403 — requester is on the current line (INV-3).
//   - INVALID_REQUEST        400 — <2 or >3 lines / malformed body (TM-2).
//
// These are constructed via the apperr helpers in the service layer; this file
// names the codes once so the service/handler never hard-code the strings.
package approval

// Error codes (mirror E11-approvals/openapi.yaml §components.schemas.Error).
const (
	CodeApprovalLineInvalid   = "APPROVAL_LINE_INVALID"
	CodeLineAlreadyCleared    = "LINE_ALREADY_CLEARED"
	CodeSelfApprovalForbidden = "SELF_APPROVAL_FORBIDDEN"
	CodeInvalidRequest        = "INVALID_REQUEST"
)
