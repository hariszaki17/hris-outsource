// Package apperr is the single domain-error type used across all layers.
// Services return *Error; one HTTP middleware (httpx) renders it into the shared
// ErrorEnvelope (CONVENTIONS §11) and picks the status code (CONVENTIONS §7).
//
// The code<->status mapping is encoded here once so the rules
// "409 for INV_* violations, 422 for rule/quota, 403 for RBAC/scope" live in
// exactly one place.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a structured, client-safe error. Message defaults to a Bahasa string
// resolved from i18n at the boundary; Fields populates error.fields (400/422).
type Error struct {
	Code       string            // UPPER_SNAKE machine code, stable for clients
	Message    string            // optional override; usually filled from i18n by code
	Fields     map[string]string // field -> message, only for 400/422
	Details    any               // optional structured payload (e.g. INVViolationDetails); serialized as error.details
	HTTPStatus int               // resolved from Code unless explicitly set
	cause      error             // wrapped internal error (never serialized)
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.cause)
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.cause }

// Status returns the HTTP status, deriving it from Code when not set explicitly.
func (e *Error) Status() int {
	if e.HTTPStatus != 0 {
		return e.HTTPStatus
	}
	return statusForCode(e.Code)
}

// WithCause attaches an internal error for logging (not serialized to clients).
func (e *Error) WithCause(err error) *Error { e.cause = err; return e }

// As extracts an *Error from any error chain (false if none).
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// --- constructors keyed to CONVENTIONS §11 standard codes ---

func Invalid(fields map[string]string) *Error {
	return &Error{Code: "INVALID_REQUEST", Fields: fields, HTTPStatus: http.StatusBadRequest}
}

func Unauthenticated() *Error {
	return &Error{Code: "UNAUTHENTICATED", HTTPStatus: http.StatusUnauthorized}
}

func Forbidden() *Error {
	return &Error{Code: "FORBIDDEN", HTTPStatus: http.StatusForbidden}
}

// OutOfScope — leader acting on a non-own-company resource (CONVENTIONS §17).
func OutOfScope() *Error {
	return &Error{Code: "OUT_OF_SCOPE", HTTPStatus: http.StatusForbidden}
}

func NotFound() *Error {
	return &Error{Code: "NOT_FOUND", HTTPStatus: http.StatusNotFound}
}

// Conflict — 409, for INV_<N>_VIOLATION / DOUBLE_SHIFT / etc.
func Conflict(code string) *Error {
	return &Error{Code: code, HTTPStatus: http.StatusConflict}
}

// ConflictWithDetails — 409 carrying a structured error.details payload
// (e.g. INVViolationDetails for INV-1..4) plus optional field-level errors.
func ConflictWithDetails(code string, fields map[string]string, details any) *Error {
	return &Error{Code: code, Fields: fields, Details: details, HTTPStatus: http.StatusConflict}
}

// Rule — 422, semantic business-rule failure (quota, geofence, period).
// fields is optional (may be nil).
func Rule(code string, fields map[string]string) *Error {
	return &Error{Code: code, Fields: fields, HTTPStatus: http.StatusUnprocessableEntity}
}

// Internal — 500. The cause is logged; clients only ever see code INTERNAL.
func Internal(cause error) *Error {
	return (&Error{Code: "INTERNAL", HTTPStatus: http.StatusInternalServerError}).WithCause(cause)
}

// statusForCode maps the cross-cutting codes to HTTP statuses (CONVENTIONS §7, §11).
func statusForCode(code string) int {
	switch code {
	case "INVALID_REQUEST", "CURSOR_MISMATCH", "UNKNOWN_FILTER":
		return http.StatusBadRequest
	case "UNAUTHENTICATED":
		return http.StatusUnauthorized
	case "FORBIDDEN", "OUT_OF_SCOPE":
		return http.StatusForbidden
	case "NOT_FOUND":
		return http.StatusNotFound
	case "RATE_LIMITED":
		return http.StatusTooManyRequests
	case "MAINTENANCE":
		return http.StatusServiceUnavailable
	case "INTERNAL":
		return http.StatusInternalServerError
	}
	// Convention: INV_*_VIOLATION and *_SHIFT conflicts are 409; everything else
	// semantic (QUOTA_EXCEEDED, OUT_OF_GEOFENCE, RULE_VIOLATION, *_EXCEEDS_*) is 422.
	switch {
	case isConflictCode(code):
		return http.StatusConflict
	default:
		return http.StatusUnprocessableEntity
	}
}

func isConflictCode(code string) bool {
	// INV_<N>_VIOLATION, DOUBLE_SHIFT, SHIFT_OVER_LEAVE, CONFLICT, IDEMPOTENCY_KEY_REUSED
	switch code {
	case "CONFLICT", "DOUBLE_SHIFT", "SHIFT_OVER_LEAVE", "IDEMPOTENCY_KEY_REUSED":
		return true
	}
	const inv = "INV_"
	return len(code) > len(inv) && code[:len(inv)] == inv
}
