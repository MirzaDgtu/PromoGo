// Package auth holds PromoGo's token issuance/verification, OTP hashing,
// phone normalization, and OIDC ID-token verification. It has no
// repository/DB access — internal/service and internal/httpserver call
// into it for the crypto, keeping that logic in one reviewable place.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned for any access token that fails signature,
// expiry, or type verification. Callers should treat it as 401 Unauthorized
// without distinguishing the specific cause (avoids leaking which check
// failed to a potential attacker).
var ErrInvalidToken = errors.New("invalid token")

// tokenType is embedded in every access token so a customer token can never
// be accepted where a staff token is required, and vice versa, even though
// both are HS256-signed with the same server secret.
type tokenType string

const (
	tokenTypeCustomerAccess tokenType = "customer_access"
	tokenTypeStaffAccess    tokenType = "staff_access"
)

type accessClaims struct {
	Type tokenType `json:"typ"`
	jwt.RegisteredClaims
}

// IssueCustomerAccessToken issues a short-lived HS256 JWT whose subject is
// the customer's stable CustomerAccountID. No phone or other PII is placed
// in the token.
func IssueCustomerAccessToken(secret []byte, customerAccountID int64, ttl time.Duration) (string, error) {
	return issueAccessToken(secret, tokenTypeCustomerAccess, strconv.FormatInt(customerAccountID, 10), ttl)
}

// ParseCustomerAccessToken verifies token and returns the CustomerAccountID
// it was issued for.
func ParseCustomerAccessToken(secret []byte, token string) (int64, error) {
	return parseAccessToken(secret, tokenTypeCustomerAccess, token)
}

// IssueStaffAccessToken issues a short-lived HS256 JWT whose subject is the
// staff member's stable StaffUserID. Org/role/store scope is deliberately
// not embedded — RequireStaff resolves current StaffMemberships from the
// database on every request, so a disabled membership takes effect
// immediately rather than only after the access token expires.
func IssueStaffAccessToken(secret []byte, staffUserID int64, ttl time.Duration) (string, error) {
	return issueAccessToken(secret, tokenTypeStaffAccess, strconv.FormatInt(staffUserID, 10), ttl)
}

// ParseStaffAccessToken verifies token and returns the StaffUserID it was
// issued for.
func ParseStaffAccessToken(secret []byte, token string) (int64, error) {
	return parseAccessToken(secret, tokenTypeStaffAccess, token)
}

func issueAccessToken(secret []byte, typ tokenType, subject string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := accessClaims{
		Type: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

func parseAccessToken(secret []byte, want tokenType, raw string) (int64, error) {
	var claims accessClaims
	_, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return 0, ErrInvalidToken
	}
	if claims.Type != want {
		return 0, ErrInvalidToken
	}

	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, ErrInvalidToken
	}
	return id, nil
}

// GenerateRefreshToken returns a new cryptographically random opaque
// refresh token (256 bits, base64url-encoded, crypto/rand) and the SHA-256
// hash that should be persisted in its place — the plaintext token is
// returned to the caller exactly once and never stored.
func GenerateRefreshToken() (token, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashOpaqueToken(token), nil
}

// HashOpaqueToken returns the hex-encoded SHA-256 hash of an opaque token
// (refresh token or store API key), matching what's stored in
// customer_sessions.refresh_token_hash / store_api_keys.key_hash. These are
// high-entropy random tokens, not user-chosen passwords, so a plain hash is
// sufficient here — see httpserver.hashAPIKey for the same reasoning
// applied to the original store API key.
func HashOpaqueToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GenerateAPIKey returns a new store API key in the form
// "<keyID>.<secret>" plus its KeyID (a non-secret, loggable prefix) and the
// hash to persist for the secret half. Splitting KeyID out lets an admin
// identify/revoke a specific key without ever seeing or logging the secret.
func GenerateAPIKey() (plaintext, keyID, hash string, err error) {
	idBuf := make([]byte, 8)
	if _, err := rand.Read(idBuf); err != nil {
		return "", "", "", fmt.Errorf("generate api key id: %w", err)
	}
	secretBuf := make([]byte, 32)
	if _, err := rand.Read(secretBuf); err != nil {
		return "", "", "", fmt.Errorf("generate api key secret: %w", err)
	}

	keyID = base64.RawURLEncoding.EncodeToString(idBuf)
	secret := base64.RawURLEncoding.EncodeToString(secretBuf)
	plaintext = keyID + "." + secret
	return plaintext, keyID, HashOpaqueToken(secret), nil
}

// constantTimeEqual reports whether a and b are equal, in constant time.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
