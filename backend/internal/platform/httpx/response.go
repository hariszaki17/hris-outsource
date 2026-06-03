// Package httpx holds the HTTP boundary helpers shared by every handler:
// JSON encoding, the error-envelope renderer, request-id propagation, cursor
// codec, and middleware. Handlers never write the envelope themselves — they
// return data or an *apperr.Error and let WriteError do the mapping.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/i18n"
)

// envelope mirrors CONVENTIONS §11 exactly.
type envelope struct {
	Error errBody `json:"error"`
}

type errBody struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id"`
}

// WriteJSON encodes v with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

// WriteError maps any error to the shared ErrorEnvelope. Unknown errors become
// INTERNAL (and are logged with their cause); *apperr.Error is rendered as-is.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	ae, ok := apperr.As(err)
	if !ok {
		ae = apperr.Internal(err)
	}

	lang := i18n.LangFrom(r.Header.Get("Accept-Language"))
	msg := ae.Message
	if msg == "" {
		msg = i18n.Message(lang, ae.Code)
	}

	reqID := RequestID(r.Context())
	if ae.Status() >= 500 {
		// Log the cause; never leak it to the client.
		slog.ErrorContext(r.Context(), "request failed",
			"code", ae.Code, "request_id", reqID, "err", errors.Unwrap(ae))
	}

	WriteJSON(w, ae.Status(), envelope{Error: errBody{
		Code:      ae.Code,
		Message:   msg,
		Fields:    ae.Fields,
		RequestID: reqID,
	}})
}

// --- request id ---

type ctxKey int

const requestIDKey ctxKey = iota

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID returns the correlation id for this request ("" if unset).
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
