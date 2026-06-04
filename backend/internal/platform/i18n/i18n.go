// Package i18n resolves error-code -> human message. Bahasa Indonesia is the
// default (CONVENTIONS §11); en-US is selected via the Accept-Language header.
// User-facing API copy lives on the frontend; this catalog only covers the
// error-envelope messages the backend must emit.
package i18n

import "strings"

type Lang string

const (
	ID Lang = "id" // Bahasa Indonesia (default)
	EN Lang = "en"
)

// LangFrom parses an Accept-Language header; defaults to Bahasa.
func LangFrom(acceptLanguage string) Lang {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(acceptLanguage)), "en") {
		return EN
	}
	return ID
}

// messages[lang][code] -> message. Per-epic codes are added alongside their epic.
var messages = map[Lang]map[string]string{
	ID: {
		"INVALID_REQUEST": "Permintaan tidak valid.",
		"UNAUTHENTICATED": "Sesi tidak valid atau telah berakhir.",
		"FORBIDDEN":       "Anda tidak memiliki izin untuk tindakan ini.",
		"OUT_OF_SCOPE":    "Tindakan di luar cakupan perusahaan Anda.",
		"NOT_FOUND":       "Data tidak ditemukan.",
		"CONFLICT":        "Terjadi konflik dengan kondisi data saat ini.",
		"RULE_VIOLATION":  "Permintaan melanggar aturan bisnis.",
		"QUOTA_EXCEEDED":  "Pengajuan melebihi kuota tersisa.",
		"RATE_LIMITED":    "Terlalu banyak permintaan. Coba lagi nanti.",
		"INTERNAL":        "Terjadi kesalahan pada sistem.",
		"MAINTENANCE":     "Sistem sedang dalam pemeliharaan.",

		// E1 authentication (AU-1..AU-4)
		"INVALID_CREDENTIALS":  "Email atau kata sandi salah.",
		"ACCOUNT_DISABLED":     "Akun Anda telah dinonaktifkan.",
		"INVALID_REFRESH":      "Sesi tidak valid. Silakan masuk kembali.",
		"RESET_TOKEN_EXPIRED":  "Tautan reset sudah kedaluwarsa. Mohon minta tautan baru.",
		"WEAK_PASSWORD":        "Kata sandi tidak memenuhi kebijakan.",
		"FORGOT_PASSWORD_SENT": "Jika email terdaftar, tautan reset telah dikirim.",

		// E1 foundations — user management
		"ROLE_NOT_ALLOWED": "Perubahan peran tidak diizinkan.",
		"CURSOR_MISMATCH":  "Kursor tidak valid.",

		// E2 people — employees (EP-2)
		"DUPLICATE_NIK": "NIK sudah terdaftar untuk karyawan lain.",

		// E2 people — agreements (EA-1..EA-5, CONVENTIONS §15)
		"PKWT_PERIOD_EXCEEDS_MAX": "Periode PKWT melebihi batas 5 tahun yang diizinkan UU Ketenagakerjaan.",
		"ACTIVE_AGREEMENT_EXISTS": "Karyawan sudah memiliki perjanjian aktif. Gunakan endpoint :renew untuk perpanjangan.",
		"FILE_TOO_LARGE":          "Ukuran file melebihi batas maksimum 10 MB.",
	},
	EN: {
		"INVALID_REQUEST": "The request is invalid.",
		"UNAUTHENTICATED": "Session is invalid or has expired.",
		"FORBIDDEN":       "You do not have permission for this action.",
		"OUT_OF_SCOPE":    "Action is outside your company scope.",
		"NOT_FOUND":       "Resource not found.",
		"CONFLICT":        "The request conflicts with the current state.",
		"RULE_VIOLATION":  "The request violates a business rule.",
		"QUOTA_EXCEEDED":  "The request exceeds the remaining quota.",
		"RATE_LIMITED":    "Too many requests. Try again later.",
		"INTERNAL":        "An internal error occurred.",
		"MAINTENANCE":     "The system is under maintenance.",

		// E1 authentication (AU-1..AU-4)
		"INVALID_CREDENTIALS":  "Incorrect email or password.",
		"ACCOUNT_DISABLED":     "Your account has been disabled.",
		"INVALID_REFRESH":      "Invalid session. Please sign in again.",
		"RESET_TOKEN_EXPIRED":  "The reset link has expired. Please request a new one.",
		"WEAK_PASSWORD":        "The password does not meet the policy requirements.",
		"FORGOT_PASSWORD_SENT": "If the email is registered, a reset link has been sent.",

		// E1 foundations — user management
		"ROLE_NOT_ALLOWED": "Role change not allowed.",
		"CURSOR_MISMATCH":  "Invalid cursor.",

		// E2 people — employees (EP-2)
		"DUPLICATE_NIK": "NIK is already registered to another employee.",

		// E2 people — agreements (EA-1..EA-5, CONVENTIONS §15)
		"PKWT_PERIOD_EXCEEDS_MAX": "PKWT period exceeds the 5-year maximum allowed by Indonesian labor law.",
		"ACTIVE_AGREEMENT_EXISTS": "The employee already has an active agreement. Use :renew to extend.",
		"FILE_TOO_LARGE":          "File size exceeds the 10 MB limit.",
	},
}

// Message returns the localized message for a code, falling back to ID then code.
func Message(lang Lang, code string) string {
	if m, ok := messages[lang][code]; ok {
		return m
	}
	if m, ok := messages[ID][code]; ok {
		return m
	}
	return code
}
