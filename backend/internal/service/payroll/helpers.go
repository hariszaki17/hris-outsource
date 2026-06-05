// Package payroll — shared service helpers: limit clamp, principal extraction,
// cursor codecs, error passthrough, and the SINGLE decrypt seam (decryptMoney)
// that produces the DECRYPT_FAIL signal. Payslip cursors keyset on
// (paid_on DESC, id DESC); audit-note cursors keyset on (created_at ASC, seq ASC).
package payroll

import (
	"context"
	"fmt"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// clampLimit applies the documented default(50)/max(200).
func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return 50
	case limit > 200:
		return 200
	default:
		return limit
	}
}

// actorEmployeeID resolves the acting employee id (nil if absent).
func actorEmployeeID(ctx context.Context) *string {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.EmployeeID != "" {
		id := p.EmployeeID
		return &id
	}
	return nil
}

// actorUserID resolves the acting user id (nil if absent).
func actorUserID(ctx context.Context) *string {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.UserID != "" {
		id := p.UserID
		return &id
	}
	return nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// asAppErr passes *apperr.Error through, wrapping anything else as 500.
func asAppErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.Internal(err)
}

// periodString builds the YYYY-MM convenience field from year + month.
func periodString(year, month int) string {
	return fmt.Sprintf("%04d-%02d", year, month)
}

// decryptMoney is the single seam that produces the DECRYPT_FAIL signal. It wraps
// crypto.Cipher.DecryptPtr's three-case contract:
//   - nil/empty ciphertext → (nil, false): no value was stored (a NULL *_enc
//     column). NOT a decrypt failure.
//   - valid ciphertext     → (&plaintext, false): decrypted successfully.
//   - present but garbage  → (nil, TRUE): the ciphertext could not be opened
//     (ErrDecrypt) — the DECRYPT_FAIL signal.
//
// The returned bool is "decryptFailed": true ONLY for the third case. The caller
// ORs this across every money field on the payslip to decide the row status.
func decryptMoney(c *crypto.Cipher, ciphertext []byte) (value *string, decryptFailed bool) {
	v, ok := c.DecryptPtr(ciphertext)
	if !ok {
		return nil, true // present ciphertext that failed to open
	}
	return v, false // nil (absent) or a valid plaintext — neither is a failure
}

// --- payslip cursor (keyset on (paid_on DESC, id DESC)) ---

type payslipCursor struct {
	PaidOn *time.Time `json:"p"`
	ID     string     `json:"i"`
}

func encodePayslipCursor(paidOn *time.Time, id string) (string, error) {
	return httpx.EncodeCursor(payslipCursor{PaidOn: paidOn, ID: id})
}

// DecodePayslipCursor parses an opaque payslip cursor into (paid_on, id) pointers.
func DecodePayslipCursor(cursor string) (*time.Time, *string, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c payslipCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return c.PaidOn, &c.ID, nil
}

// --- audit-note cursor (keyset on (created_at ASC, seq ASC)) ---

type auditNoteCursor struct {
	CreatedAt time.Time `json:"c"`
	Seq       int       `json:"s"`
}

func encodeAuditNoteCursor(createdAt time.Time, seq int) (string, error) {
	return httpx.EncodeCursor(auditNoteCursor{CreatedAt: createdAt, Seq: seq})
}

// DecodeAuditNoteCursor parses an opaque audit-note cursor into (created_at, seq).
func DecodeAuditNoteCursor(cursor string) (*time.Time, *int, error) {
	if cursor == "" {
		return nil, nil, nil
	}
	var c auditNoteCursor
	if err := httpx.DecodeCursor(cursor, &c); err != nil {
		return nil, nil, err
	}
	return &c.CreatedAt, &c.Seq, nil
}
