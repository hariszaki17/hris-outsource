package people

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// AgreementHandler holds the agreement service and handles all E2 agreement endpoints.
type AgreementHandler struct {
	svc *svc.AgreementService
}

// NewAgreementHandler returns an AgreementHandler wired to the given service.
func NewAgreementHandler(s *svc.AgreementService) *AgreementHandler {
	return &AgreementHandler{svc: s}
}

// ListAgreements handles GET /agreements.
// RBAC: super_admin, hr_admin (enforced in server.go).
func (h *AgreementHandler) ListAgreements(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.AgreementFilter{
		EmployeeID: queryStringPtr(q.Get("employee_id")),
		Status:     queryStringPtr(q.Get("status")),
		Type:       queryStringPtr(q.Get("type")),
		Q:          queryStringPtr(q.Get("q")),
		Limit:      parseLimit(q.Get("limit")),
	}

	if edlte := q.Get("end_date__lte"); edlte != "" {
		t, err := parseDate(edlte)
		if err == nil {
			filter.EndDateLTE = &t
		}
	}

	if cursor := q.Get("cursor"); cursor != "" {
		var p agreementPageCursor
		if err := httpx.DecodeCursor(cursor, &p); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorCreatedAt = &p.CreatedAt
		filter.CursorID = &p.ID
	}

	agreements, nextCursor, err := h.svc.ListAgreements(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	now := time.Now()
	items := make([]agreementResponse, 0, len(agreements))
	for _, ag := range agreements {
		items = append(items, toAgreementResponse(ag, now))
	}

	resp := httpx.PageResponse[agreementResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// GetAgreement handles GET /agreements/{agreement_id}.
// RBAC: super_admin, hr_admin (enforced in server.go).
func (h *AgreementHandler) GetAgreement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "agreement_id")

	ag, err := h.svc.GetAgreement(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toAgreementResponse(ag, time.Now()))
}

// CreateAgreement handles POST /agreements.
// Returns 201 + Location header.
func (h *AgreementHandler) CreateAgreement(w http.ResponseWriter, r *http.Request) {
	var req agreementWriteRequest
	if err := decodeAgreementJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	startDate, err := parseDate(req.StartDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}

	var endDate *time.Time
	if req.EndDate != nil && *req.EndDate != "" {
		ed, err := parseDate(*req.EndDate)
		if err != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"end_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
			return
		}
		endDate = &ed
	}

	baseSalary, annualLeave, bpjsTerms, taxProfile, effDate := toCompensationParams(req.Compensation)

	principal, _ := auth.PrincipalFrom(r.Context())
	createdBy := principal.UserID

	params := svc.CreateAgreementParams{
		EmployeeID:                 req.EmployeeID,
		Type:                       req.Type,
		AgreementNo:                req.AgreementNo,
		StartDate:                  startDate,
		EndDate:                    endDate,
		BaseSalaryIDR:              baseSalary,
		AnnualLeaveEntitlementDays: annualLeave,
		BpjsTerms:                  bpjsTerms,
		TaxProfile:                 taxProfile,
		CompEffectiveDate:          effDate,
		CreatedBy:                  &createdBy,
	}

	ag, err := h.svc.CreateAgreement(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/agreements/"+ag.ID)
	httpx.WriteJSON(w, http.StatusCreated, toAgreementResponse(ag, time.Now()))
}

// RenewAgreement handles POST /agreements/{agreement_id}:renew.
// Returns 201 + Location pointing at the new (successor) agreement.
func (h *AgreementHandler) RenewAgreement(w http.ResponseWriter, r *http.Request) {
	predecessorID := chi.URLParam(r, "agreement_id")

	var req renewRequest
	if err := decodeAgreementJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	startDate, err := parseDate(req.StartDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}

	var endDate *time.Time
	if req.EndDate != nil && *req.EndDate != "" {
		ed, err := parseDate(*req.EndDate)
		if err != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"end_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
			return
		}
		endDate = &ed
	}

	baseSalary, annualLeave, bpjsTerms, taxProfile, effDate := toCompensationParams(req.Compensation)

	principal, _ := auth.PrincipalFrom(r.Context())
	createdBy := principal.UserID

	params := svc.CreateAgreementParams{
		Type:                       req.Type,
		AgreementNo:                req.AgreementNo,
		StartDate:                  startDate,
		EndDate:                    endDate,
		BaseSalaryIDR:              baseSalary,
		AnnualLeaveEntitlementDays: annualLeave,
		BpjsTerms:                  bpjsTerms,
		TaxProfile:                 taxProfile,
		CompEffectiveDate:          effDate,
		CreatedBy:                  &createdBy,
	}

	ag, err := h.svc.RenewAgreement(r.Context(), predecessorID, params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/agreements/"+ag.ID)
	httpx.WriteJSON(w, http.StatusCreated, toAgreementResponse(ag, time.Now()))
}

// CloseAgreement handles POST /agreements/{agreement_id}:close.
// Returns 200 with the updated agreement.
func (h *AgreementHandler) CloseAgreement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "agreement_id")

	var req closeRequest
	if err := decodeAgreementJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	effectiveDate, err := parseDate(req.EffectiveDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"effective_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}

	ag, err := h.svc.CloseAgreement(r.Context(), id, req.Reason, effectiveDate, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toAgreementResponse(ag, time.Now()))
}

// UploadAttachment handles POST /agreements/{agreement_id}/attachments.
// Multipart form upload (CONVENTIONS §15): field "file" required, "category" and
// "caption" optional. Max 10MB; allowed types: application/pdf, image/jpeg, image/png.
func (h *AgreementHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	agreementID := chi.URLParam(r, "agreement_id")

	// Parse the multipart form with a 10MB memory limit.
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"file": "Gagal mem-parsing multipart form."}))
		return
	}

	fileHeader, fileHeaders, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"file": "Field 'file' wajib diisi."}))
		return
	}
	defer fileHeader.Close()

	// Read file bytes into memory.
	blob, err := io.ReadAll(fileHeader)
	if err != nil {
		httpx.WriteError(w, r, apperr.Internal(fmt.Errorf("read file: %w", err)))
		return
	}

	// Category — default to "signed_agreement" per spec.
	category := r.FormValue("category")
	if category == "" {
		category = "signed_agreement"
	}

	caption := r.FormValue("caption")

	// Detect MIME: prefer Content-Type from the part header, fall back to detection.
	mime := fileHeaders.Header.Get("Content-Type")
	if mime == "" {
		// Detect from first 512 bytes.
		mime = http.DetectContentType(blob)
	}

	// Get uploader identity.
	principal, _ := auth.PrincipalFrom(r.Context())
	uploaderID := principal.UserID

	params := svc.CreateAttachmentParams{
		Category:   category,
		Caption:    caption,
		FileName:   fileHeaders.Filename,
		MIME:       mime,
		SizeBytes:  int64(len(blob)),
		Blob:       blob,
		UploadedBy: &uploaderID,
	}

	att, err := h.svc.UploadAttachment(r.Context(), agreementID, params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, toFileRefResponse(att))
}

// DownloadFile handles GET /files/{file_id}.
// Requires auth (route is under the authenticated group in server.go).
// Returns the file bytes with the correct Content-Type header.
func (h *AgreementHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")

	att, err := h.svc.GetAttachment(r.Context(), fileID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", att.MIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, att.FileName))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(att.Blob)
}

// --- private helpers ---

// decodeAgreementJSON decodes a JSON request body; unlike decodeJSON in employees_handler.go,
// we allow unknown fields for forward-compatibility on agreement bodies (the spec has a
// note field that is passed through to audit only).
func decodeAgreementJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return apperr.Invalid(nil).WithCause(err)
	}
	return nil
}

// agreementPageCursor is the local handler-side cursor — the handler decodes it
// from the request cursor param. Must match agreementPageCursor in service.
type agreementPageCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}
