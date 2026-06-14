package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims are the access-token body. Kept small: the access token is stateless
// (no DB hit per request), so it carries exactly what RBAC needs.
type Claims struct {
	jwt.RegisteredClaims
	Role       Role   `json:"role"`
	EmployeeID string `json:"emp,omitempty"`
	CompanyID  string `json:"cmp,omitempty"`
}

// Issuer signs and verifies access tokens with Ed25519 (EdDSA). Asymmetric so
// other services can verify tokens with only the public key.
type Issuer struct {
	priv      ed25519.PrivateKey
	pub       ed25519.PublicKey
	accessTTL time.Duration
}

// NewIssuer builds an Issuer from base64 (std) encoded raw Ed25519 keys.
func NewIssuer(privB64, pubB64 string, accessTTL time.Duration) (*Issuer, error) {
	priv, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil || len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid AUTH_JWT_PRIVATE_KEY (need base64 of %d bytes)", ed25519.PrivateKeySize)
	}
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid AUTH_JWT_PUBLIC_KEY (need base64 of %d bytes)", ed25519.PublicKeySize)
	}
	return &Issuer{priv: priv, pub: pub, accessTTL: accessTTL}, nil
}

// Issue mints a signed access token for a principal.
func (i *Issuer) Issue(p Principal, now time.Time) (string, time.Time, error) {
	exp := now.Add(i.accessTTL)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        newJTI(),
		},
		Role:       p.Role,
		EmployeeID: p.EmployeeID,
		CompanyID:  p.CompanyID,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := tok.SignedString(i.priv)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// Verify parses and validates a token, returning the Principal.
func (i *Issuer) Verify(token string) (Principal, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method %q", t.Header["alg"])
		}
		return i.pub, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	if err != nil {
		return Principal{}, err
	}
	p := Principal{
		UserID:     claims.Subject,
		EmployeeID: claims.EmployeeID,
		Role:       claims.Role,
		CompanyID:  claims.CompanyID,
	}
	if claims.IssuedAt != nil {
		p.IssuedAt = claims.IssuedAt.Time
	}
	return p, nil
}

func newJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// GenerateKeypair is a helper for producing dev keys (base64 std). Wire it into
// a small CLI subcommand or a one-off main when bootstrapping an environment.
func GenerateKeypair() (privB64, pubB64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv), base64.StdEncoding.EncodeToString(pub), nil
}
