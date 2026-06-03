package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// Refresh tokens are long-lived, opaque, and revocable. The plaintext is shown
// to the client exactly once (cookie for web, secure storage for mobile); only
// a SHA-256 hash is persisted, so a DB leak can't be replayed. Each /auth/refresh
// rotates the token (see service layer) and a reused token revokes the family —
// the standard refresh-token reuse-detection pattern.

// NewRefreshToken returns (plaintext, hashHex). Store the hash; return the
// plaintext to the client once.
func NewRefreshToken() (plaintext, hash string) {
	b := make([]byte, 32) // 256 bits
	_, _ = rand.Read(b)
	plaintext = base64.RawURLEncoding.EncodeToString(b)
	return plaintext, HashRefreshToken(plaintext)
}

// HashRefreshToken hashes a plaintext refresh token for storage/lookup.
func HashRefreshToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
