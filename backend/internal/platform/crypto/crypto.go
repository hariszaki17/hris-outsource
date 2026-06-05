// Package crypto is the payroll encryption-at-rest helper (INV-2). Monetary
// payslip fields (gross_earnings, gross_deductions, take_home_pay, component +
// benefit line values) are stored as AES-256-GCM ciphertext in `*_enc bytea`
// columns — never plaintext — and decrypted at the E8 service boundary (10-02)
// for authorized roles.
//
// The key is a 32-byte AES-256 key supplied via config (base64 std-encoded).
// This is a milestone-scoped env constant, NOT a production KMS — the seed
// (10-04) encrypts with the SAME key the API decrypts with.
//
// The typed ErrDecrypt is the DECRYPT_FAIL source: a row whose ciphertext fails
// to open (garbage / legacy / wrong-key) returns ErrDecrypt at the boundary,
// which the service maps to a 200 OK payslip with status DECRYPT_FAIL and the
// monetary fields nulled (openapi PayslipStatus.DECRYPT_FAIL) — NOT an error.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrDecrypt is returned by Decrypt/DecryptPtr when a ciphertext cannot be
// authenticated/opened (too short, tampered, or encrypted under a different
// key). It is the typed signal the service maps to the DECRYPT_FAIL row status.
var ErrDecrypt = errors.New("crypto: decrypt failed")

// Cipher wraps an AES-256-GCM AEAD. Construct via New or NewFromBase64.
type Cipher struct {
	aead cipher.AEAD
}

// New builds an AES-256-GCM Cipher from a raw 32-byte key. Returns an error if
// the key is not exactly 32 bytes (AES-256).
func New(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes for AES-256, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// NewFromBase64 decodes a base64 (std encoding) 32-byte key and builds a Cipher.
// Config stores the payroll key base64-encoded, like the Ed25519 keys in
// config.Auth.
func NewFromBase64(b64 string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode base64 key: %w", err)
	}
	return New(key)
}

// Encrypt seals plaintext under AES-256-GCM. A fresh random nonce is generated
// per call and PREPENDED to the ciphertext (the standard self-describing layout
// Decrypt expects). The plaintext for payslip money is the decimal Money string
// e.g. "8500000.00".
func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: read nonce: %w", err)
	}
	// Seal appends the ciphertext to nonce (the dst), so the result is
	// nonce || ciphertext — exactly what Decrypt splits back apart.
	return c.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt opens a nonce-prepended AES-256-GCM ciphertext produced by Encrypt.
// On any failure (too short, or the AEAD tag does not authenticate — i.e.
// garbage / legacy / wrong-key bytes) it returns ErrDecrypt and NEVER panics.
// This is what makes a seeded corrupt row surface as DECRYPT_FAIL.
func (c *Cipher) Decrypt(ciphertext []byte) (string, error) {
	ns := c.aead.NonceSize()
	if len(ciphertext) < ns {
		return "", ErrDecrypt
	}
	nonce, ct := ciphertext[:ns], ciphertext[ns:]
	plaintext, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", ErrDecrypt
	}
	return string(plaintext), nil
}

// DecryptPtr is the convenience seam the 10-02 service uses to set the payslip
// row status. It distinguishes THREE cases:
//
//   - nil/empty ciphertext  → (nil, true):  no value was stored (a NULL *_enc
//     column). This is a normal absent value, NOT a decrypt failure.
//   - valid ciphertext      → (&plaintext, true): decrypted successfully.
//   - non-empty but garbage  → (nil, false): the ciphertext could not be opened
//     (ErrDecrypt) — the DECRYPT_FAIL signal. The caller nulls every monetary
//     field and sets status DECRYPT_FAIL / locked_reason decrypt_fail.
//
// The boolean is "ok": true for the first two cases, false only for a genuine
// decrypt failure on present ciphertext.
func (c *Cipher) DecryptPtr(ciphertext []byte) (*string, bool) {
	if len(ciphertext) == 0 {
		return nil, true
	}
	plaintext, err := c.Decrypt(ciphertext)
	if err != nil {
		return nil, false
	}
	return &plaintext, true
}
