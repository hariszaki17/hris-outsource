package people

import (
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
)

// --- request structs ---

// selfProfileUpdateBody is the PATCH /me/profile body (SelfProfileUpdate schema).
// All fields optional (minProperties:1 enforced in the handler). E11
// (2026-06-14): phone / emergency_contact / bank_account are now INSTANT
// self-edits too — the change-request approval queue was removed, so there are no
// longer any approval-tier fields.
type selfProfileUpdateBody struct {
	Address          *string                     `json:"address"`
	AppLanguage      *string                     `json:"app_language"`
	PhotoObjectKey   *string                     `json:"photo_object_key"`
	Phone            *string                     `json:"phone"`
	EmergencyContact *selfEmergencyContactBody   `json:"emergency_contact"`
	BankAccount      *selfBankAccountBody        `json:"bank_account"`
}

// selfEmergencyContactBody is the emergency_contact object on the update body.
type selfEmergencyContactBody struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// selfBankAccountBody is the bank_account object on the update body.
type selfBankAccountBody struct {
	BankName          string `json:"bank_name"`
	AccountNumber     string `json:"account_number"`
	AccountHolderName string `json:"account_holder_name"`
}

// photoUploadInitBody is the POST /me/profile/photo-upload-init body.
type photoUploadInitBody struct {
	ContentType   string `json:"content_type"`
	ContentLength int64  `json:"content_length"`
}

// --- response structs ---

// selfEmergencyContactResp is the emergency_contact object on the self profile.
type selfEmergencyContactResp struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// selfProfileResponse is the Employee object returned by PATCH /me/profile —
// the staff employeeResponse plus the self-service fields (emergency_contact,
// app_language) and the presigned photo_url (readOnly).
type selfProfileResponse struct {
	ID                  string                    `json:"id"`
	FullName            string                    `json:"full_name"`
	NIK                 string                    `json:"nik"`
	NIP                 string                    `json:"nip"`
	JoinAt              string                    `json:"join_at"`
	Gender              *string                   `json:"gender"`
	BirthDate           *string                   `json:"birth_date"`
	BirthPlace          *string                   `json:"birth_place"`
	Phone               *string                   `json:"phone"`
	EmailPersonal       *string                   `json:"email_personal"`
	Address             *string                   `json:"address"`
	NPWP                *string                   `json:"npwp"`
	BPJSKesehatan       *string                   `json:"bpjs_kesehatan"`
	BPJSKetenagakerjaan *string                   `json:"bpjs_ketenagakerjaan"`
	BankAccount         *bankAccountResp          `json:"bank_account"`
	EmergencyContact    *selfEmergencyContactResp `json:"emergency_contact"`
	AppLanguage         string                    `json:"app_language"`
	PhotoURL            *string                   `json:"photo_url"`
	Status              string                    `json:"status"`
	HasLogin            bool                      `json:"has_login"`
	CreatedAt           string                    `json:"created_at"`
	UpdatedAt           string                    `json:"updated_at"`
}

// toSelfProfileResponse maps the domain employee + a presigned photo URL to the
// self-profile wire response. photoURL is "" when the employee has no photo.
func toSelfProfileResponse(e domain.Employee, photoURL string) selfProfileResponse {
	resp := selfProfileResponse{
		ID:                  e.ID,
		FullName:            e.FullName,
		NIK:                 e.NIK,
		NIP:                 e.NIP,
		JoinAt:              e.JoinAt.Format("2006-01-02"),
		Gender:              e.Gender,
		BirthPlace:          e.BirthPlace,
		Phone:               e.Phone,
		EmailPersonal:       e.EmailPersonal,
		Address:             e.Address,
		NPWP:                e.NPWP,
		BPJSKesehatan:       e.BPJSKesehatan,
		BPJSKetenagakerjaan: e.BPJSKetenagakerjaan,
		AppLanguage:         e.AppLanguage,
		Status:              strings.ToUpper(e.Status),
		HasLogin:            e.HasLogin,
		CreatedAt:           e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           e.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if e.BirthDate != nil && !e.BirthDate.IsZero() {
		s := e.BirthDate.Format("2006-01-02")
		resp.BirthDate = &s
	}
	resp.BankAccount = &bankAccountResp{
		BankName:          e.BankAccount.BankName,
		AccountNumber:     e.BankAccount.AccountNumber,
		AccountHolderName: e.BankAccount.AccountHolderName,
	}
	resp.EmergencyContact = &selfEmergencyContactResp{
		Name:  e.EmergencyContact.Name,
		Phone: e.EmergencyContact.Phone,
	}
	if photoURL != "" {
		resp.PhotoURL = &photoURL
	}
	return resp
}

// uploadTicketResponse is the POST /me/profile/photo-upload-init response
// (UploadTicket schema). expires_at is RFC3339.
type uploadTicketResponse struct {
	UploadURL   string `json:"upload_url"`
	ObjectKey   string `json:"object_key"`
	ContentType string `json:"content_type"`
	MaxBytes    int64  `json:"max_bytes"`
	ExpiresAt   string `json:"expires_at"`
}
