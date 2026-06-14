package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"testing"
)

func newTestCipher(t *testing.T) *Cipher {
	t.Helper()
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("gen key: %v", err)
	}
	c, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestRoundTrip(t *testing.T) {
	c := newTestCipher(t)
	for _, plain := range []string{"8500000.00", "0.00", "-1175000.00", ""} {
		ct, err := c.Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plain, err)
		}
		got, err := c.Decrypt(ct)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if got != plain {
			t.Fatalf("round-trip mismatch: got %q want %q", got, plain)
		}
	}
}

func TestDecryptGarbageReturnsErrDecrypt(t *testing.T) {
	c := newTestCipher(t)
	// Deliberately-corrupt ciphertext (the seeded DECRYPT_FAIL row).
	if _, err := c.Decrypt([]byte("not-a-valid-ciphertext-at-all")); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("garbage: want ErrDecrypt, got %v", err)
	}
	// Too-short ciphertext (< nonce size) must also be ErrDecrypt, never panic.
	if _, err := c.Decrypt([]byte{0x01}); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("too-short: want ErrDecrypt, got %v", err)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	a := newTestCipher(t)
	b := newTestCipher(t)
	ct, err := a.Encrypt("8500000.00")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := b.Decrypt(ct); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("wrong key: want ErrDecrypt, got %v", err)
	}
}

func TestDecryptPtrThreeCases(t *testing.T) {
	c := newTestCipher(t)

	// 1. nil/empty ciphertext → (nil, true): no value stored, not a failure.
	if v, ok := c.DecryptPtr(nil); v != nil || !ok {
		t.Fatalf("nil ct: want (nil,true), got (%v,%v)", v, ok)
	}
	if v, ok := c.DecryptPtr([]byte{}); v != nil || !ok {
		t.Fatalf("empty ct: want (nil,true), got (%v,%v)", v, ok)
	}

	// 2. valid ciphertext → (&plaintext, true).
	ct, _ := c.Encrypt("7325000.00")
	if v, ok := c.DecryptPtr(ct); !ok || v == nil || *v != "7325000.00" {
		t.Fatalf("valid ct: want (&\"7325000.00\",true), got (%v,%v)", v, ok)
	}

	// 3. garbage ciphertext → (nil, false): the DECRYPT_FAIL signal.
	if v, ok := c.DecryptPtr([]byte("garbage-ciphertext-bytes-xxxxx")); v != nil || ok {
		t.Fatalf("garbage ct: want (nil,false), got (%v,%v)", v, ok)
	}
}

func TestNewKeyLength(t *testing.T) {
	if _, err := New(make([]byte, 16)); err == nil {
		t.Fatal("16-byte key: want error, got nil")
	}
	if _, err := New(make([]byte, 32)); err != nil {
		t.Fatalf("32-byte key: want nil, got %v", err)
	}
}

func TestNewFromBase64(t *testing.T) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("gen key: %v", err)
	}
	b64 := base64.StdEncoding.EncodeToString(key)
	c, err := NewFromBase64(b64)
	if err != nil {
		t.Fatalf("NewFromBase64: %v", err)
	}
	ct, _ := c.Encrypt("100.00")
	got, err := c.Decrypt(ct)
	if err != nil || got != "100.00" {
		t.Fatalf("round-trip via base64: got %q err %v", got, err)
	}
	if _, err := NewFromBase64("!!!not-base64!!!"); err == nil {
		t.Fatal("bad base64: want error, got nil")
	}
}
