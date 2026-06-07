package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Password hashing uses argon2id (OWASP-preferred). Hashes are stored in the
// standard PHC string form so parameters travel with the hash and can be
// upgraded over time without a schema change.

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLen     uint32
	keyLen      uint32
}

// defaultParams are sensible 2024-era argon2id settings (~64MB, 3 passes).
var defaultParams = argon2Params{
	memory:      64 * 1024,
	iterations:  3,
	parallelism: 2,
	saltLen:     16,
	keyLen:      32,
}

var ErrPasswordMismatch = errors.New("password mismatch")

// GenerateTempPassword returns a random temporary password that satisfies the
// platform policy (>=10 chars; upper, lower, digit, symbol). EP-3 show-once: the
// caller returns this plaintext to the admin ONCE and persists only its argon2id
// hash — the temp password itself is never stored.
func GenerateTempPassword() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// "Swp-" guarantees upper+lower+symbol; the trailing "9" guarantees a digit;
	// the base64url body (A-Za-z0-9-_) keeps it unguessable. ~21 chars.
	return "Swp-" + base64.RawURLEncoding.EncodeToString(b) + "9", nil
}

// HashPassword returns a PHC-encoded argon2id hash.
func HashPassword(plain string) (string, error) {
	p := defaultParams
	salt := make([]byte, p.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, p.iterations, p.memory, p.parallelism, p.keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.iterations, p.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether plain matches the PHC-encoded hash. Returns
// ErrPasswordMismatch on a clean mismatch; other errors mean a malformed hash.
func VerifyPassword(plain, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return errors.New("invalid argon2 hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return err
	}
	var p argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return err
	}
	got := argon2.IDKey([]byte(plain), salt, p.iterations, p.memory, p.parallelism, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}
