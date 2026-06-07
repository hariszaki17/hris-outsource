// Package people (handler) is the HTTP boundary for E2 employees.
// Hand-written chi handlers — no server codegen (oapi-codegen cannot parse OpenAPI 3.1).
// RBAC is enforced in server.go route groups; scope guards are in the service layer.
package people

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// Handler holds the people service and handles all E2 employee endpoints.
type Handler struct {
	svc *svc.Service
}

// NewHandler returns a Handler wired to the given service.
func NewHandler(s *svc.Service) *Handler {
	return &Handler{svc: s}
}

// ListEmployees handles GET /employees.
// RBAC: super_admin, hr_admin, shift_leader (enforced in server.go).
func (h *Handler) ListEmployees(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.EmployeeFilter{
		Q:      queryStringPtr(q.Get("q")),
		Status: queryStringPtr(q.Get("status")),
		Limit:  parseLimit(q.Get("limit")),
	}

	if cursor := q.Get("cursor"); cursor != "" {
		var p pageCursor
		if err := httpx.DecodeCursor(cursor, &p); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorCreatedAt = &p.CreatedAt
		filter.CursorID = &p.ID
	}

	employees, nextCursor, err := h.svc.ListEmployees(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]employeeResponse, 0, len(employees))
	for _, e := range employees {
		items = append(items, toEmployeeResponse(e))
	}

	resp := httpx.PageResponse[employeeResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// GetEmployee handles GET /employees/{employee_id}.
// RBAC: super_admin, hr_admin, shift_leader (enforced in server.go).
// NOTE: The OpenAPI spec x-rbac also lists agent for GET detail, but the FE
// web app never calls it as an agent (agent self-service is mobile). Web roles
// only here. See SUMMARY.md for the rationale.
func (h *Handler) GetEmployee(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "employee_id")

	emp, err := h.svc.GetEmployee(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// CreateEmployee handles POST /employees.
// Returns 201 + Location header. D1: every employee auto-provisions a login at
// create — the response carries the one-time temp_password (show-once). The login
// identifier is the phone (required); login_email is optional (D2).
func (h *Handler) CreateEmployee(w http.ResponseWriter, r *http.Request) {
	var req employeeWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	joinAt, joinAtErr := parseDate(derefString(req.JoinAt))
	if joinAtErr != nil && derefString(req.JoinAt) != "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"join_at": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}

	var birthDate *time.Time
	if derefString(req.BirthDate) != "" {
		bd, err := parseDate(derefString(req.BirthDate))
		if err != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"birth_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
			return
		}
		birthDate = &bd
	}

	bankName, bankAccNum, bankHolder := "", "", ""
	if req.BankAccount != nil {
		bankName = derefString(req.BankAccount.BankName)
		bankAccNum = derefString(req.BankAccount.AccountNumber)
		bankHolder = derefString(req.BankAccount.AccountHolderName)
	}

	params := svc.CreateEmployeeParams{
		FullName:              derefString(req.FullName),
		NIK:                   derefString(req.NIK),
		NIP:                   derefString(req.NIP),
		JoinAt:                joinAt,
		Gender:                derefString(req.Gender),
		BirthDate:             birthDate,
		BirthPlace:            derefString(req.BirthPlace),
		Phone:                 derefString(req.Phone),
		EmailPersonal:         derefString(req.EmailPersonal),
		Address:               derefString(req.Address),
		NPWP:                  derefString(req.NPWP),
		BPJSKesehatan:         derefString(req.BPJSKesehatan),
		BPJSKetenagakerjaan:   derefString(req.BPJSKetenagakerjaan),
		BankName:              bankName,
		BankAccountNumber:     bankAccNum,
		BankAccountHolderName: bankHolder,
		LoginEmail:            derefString(req.LoginEmail),
	}

	emp, tempPw, err := h.svc.CreateEmployee(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/employees/"+emp.ID)
	resp := toEmployeeResponse(emp)
	resp.TempPassword = tempPw // show-once: login is always provisioned at create (D1)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// UpdateEmployee handles PATCH /employees/{employee_id}.
func (h *Handler) UpdateEmployee(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "employee_id")

	var req employeeWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Load current employee to carry forward unchanged fields (partial update).
	current, err := h.svc.GetEmployee(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Resolve joinAt: use request value if provided, else keep current.
	joinAt := current.JoinAt
	if derefString(req.JoinAt) != "" {
		parsed, parseErr := parseDate(derefString(req.JoinAt))
		if parseErr != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"join_at": "Format tanggal tidak valid (YYYY-MM-DD)."}))
			return
		}
		joinAt = parsed
	}

	// Resolve birthDate: use request value if provided, else keep current.
	birthDate := current.BirthDate
	if derefString(req.BirthDate) != "" {
		parsed, parseErr := parseDate(derefString(req.BirthDate))
		if parseErr != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"birth_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
			return
		}
		birthDate = &parsed
	}

	// Resolve bank account fields.
	bankName := current.BankAccount.BankName
	bankAccNum := current.BankAccount.AccountNumber
	bankHolder := current.BankAccount.AccountHolderName
	if req.BankAccount != nil {
		if req.BankAccount.BankName != nil {
			bankName = *req.BankAccount.BankName
		}
		if req.BankAccount.AccountNumber != nil {
			bankAccNum = *req.BankAccount.AccountNumber
		}
		if req.BankAccount.AccountHolderName != nil {
			bankHolder = *req.BankAccount.AccountHolderName
		}
	}

	params := svc.UpdateEmployeeParams{
		ID:                    id,
		FullName:              coalesce(req.FullName, current.FullName),
		NIK:                   coalesce(req.NIK, current.NIK),
		NIP:                   coalesce(req.NIP, current.NIP),
		JoinAt:                joinAt,
		Gender:                coalesce(req.Gender, ptrStr(current.Gender)),
		BirthDate:             birthDate,
		BirthPlace:            coalesce(req.BirthPlace, ptrStr(current.BirthPlace)),
		Phone:                 coalesce(req.Phone, ptrStr(current.Phone)),
		EmailPersonal:         coalesce(req.EmailPersonal, ptrStr(current.EmailPersonal)),
		Address:               coalesce(req.Address, ptrStr(current.Address)),
		NPWP:                  coalesce(req.NPWP, ptrStr(current.NPWP)),
		BPJSKesehatan:         coalesce(req.BPJSKesehatan, ptrStr(current.BPJSKesehatan)),
		BPJSKetenagakerjaan:   coalesce(req.BPJSKetenagakerjaan, ptrStr(current.BPJSKetenagakerjaan)),
		BankName:              bankName,
		BankAccountNumber:     bankAccNum,
		BankAccountHolderName: bankHolder,
	}

	emp, err := h.svc.UpdateEmployee(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// DeactivateEmployee handles POST /employees/{employee_id}:deactivate (offboard,
// F2.7 OB-1). reason is REQUIRED and enum-validated; note is optional. The reason
// drives a traceable cascade (close agreement + end placements) in the service,
// which returns 400 with field "reason" when missing/invalid.
func (h *Handler) DeactivateEmployee(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "employee_id")

	var req reasonRequest
	// Body is optional to decode (a missing/empty body yields an empty reason, which
	// the service rejects as INVALID_REQUEST on "reason").
	_ = decodeJSON(r, &req)

	reason := derefString(req.Reason)
	note := derefString(req.Note)

	emp, err := h.svc.DeactivateEmployee(r.Context(), id, reason, note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// ReactivateEmployee handles POST /employees/{employee_id}:reactivate.
func (h *Handler) ReactivateEmployee(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "employee_id")

	emp, err := h.svc.ReactivateEmployee(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// RegenerateTempPassword handles POST /employees/{employee_id}:regenerate-password.
// Re-issues the temporary password (show-once) and forces a rotation on next login.
func (h *Handler) RegenerateTempPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "employee_id")

	tempPw, err := h.svc.RegenerateTempPassword(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"temp_password": tempPw})
}

// --- private helpers ---

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.Invalid(nil).WithCause(err)
	}
	return nil
}

// parseDate parses a "YYYY-MM-DD" string into a time.Time (midnight UTC).
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// coalesce returns the dereferenced pointer value, or fallback if nil/empty.
func coalesce(ptr *string, fallback string) string {
	if ptr != nil && *ptr != "" {
		return *ptr
	}
	return fallback
}

// ptrStr dereferences a *string; empty string if nil.
func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
