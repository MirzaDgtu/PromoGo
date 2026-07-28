package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCustomerAccessTokenRoundTrip(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes-long!")

	token, err := IssueCustomerAccessToken(secret, 42, time.Minute)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}

	id, err := ParseCustomerAccessToken(secret, token)
	if err != nil {
		t.Fatalf("ParseCustomerAccessToken() error = %v", err)
	}
	if id != 42 {
		t.Errorf("ParseCustomerAccessToken() id = %d, want 42", id)
	}
}

func TestStaffAccessTokenRoundTrip(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes-long!")

	token, err := IssueStaffAccessToken(secret, 7, time.Minute)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}

	id, err := ParseStaffAccessToken(secret, token)
	if err != nil {
		t.Fatalf("ParseStaffAccessToken() error = %v", err)
	}
	if id != 7 {
		t.Errorf("ParseStaffAccessToken() id = %d, want 7", id)
	}
}

// TestTokenTypeConfusionRejected verifies a customer access token can never
// be accepted as a staff access token, and vice versa, even though both are
// signed with the same secret — see accessClaims.Type.
func TestTokenTypeConfusionRejected(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes-long!")

	customerToken, err := IssueCustomerAccessToken(secret, 1, time.Minute)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}
	if _, err := ParseStaffAccessToken(secret, customerToken); err == nil {
		t.Error("ParseStaffAccessToken() accepted a customer access token, want error")
	}

	staffToken, err := IssueStaffAccessToken(secret, 1, time.Minute)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}
	if _, err := ParseCustomerAccessToken(secret, staffToken); err == nil {
		t.Error("ParseCustomerAccessToken() accepted a staff access token, want error")
	}
}

func TestExpiredAccessTokenRejected(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes-long!")

	token, err := IssueCustomerAccessToken(secret, 1, -time.Minute)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}
	if _, err := ParseCustomerAccessToken(secret, token); err == nil {
		t.Error("ParseCustomerAccessToken() accepted an expired token, want error")
	}
}

func TestAccessTokenWrongSecretRejected(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes-long!")
	otherSecret := []byte("a-completely-different-secret-32b")

	token, err := IssueCustomerAccessToken(secret, 1, time.Minute)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}
	if _, err := ParseCustomerAccessToken(otherSecret, token); err == nil {
		t.Error("ParseCustomerAccessToken() accepted a token signed with a different secret, want error")
	}
}

// TestAccessTokenAlgNoneRejected guards against the classic JWT "alg: none"
// downgrade attack — jwt.WithValidMethods pins parsing to HS256 only.
func TestAccessTokenAlgNoneRejected(t *testing.T) {
	secret := []byte("test-secret-at-least-32-bytes-long!")

	claims := accessClaims{
		Type: tokenTypeCustomerAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	unsigned, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build alg=none token: %v", err)
	}

	if _, err := ParseCustomerAccessToken(secret, unsigned); err == nil {
		t.Error("ParseCustomerAccessToken() accepted an alg=none token, want error")
	}
}

func TestGenerateRefreshTokenUniqueAndHashMatches(t *testing.T) {
	token1, hash1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	token2, hash2, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	if token1 == token2 {
		t.Error("GenerateRefreshToken() produced identical tokens across calls")
	}
	if hash1 == hash2 {
		t.Error("GenerateRefreshToken() produced identical hashes across calls")
	}
	if HashOpaqueToken(token1) != hash1 {
		t.Error("HashOpaqueToken(token1) does not match returned hash")
	}
}

func TestGenerateAPIKeyFormatAndHash(t *testing.T) {
	plaintext, keyID, hash, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}

	wantPrefix := keyID + "."
	if len(plaintext) <= len(wantPrefix) || plaintext[:len(wantPrefix)] != wantPrefix {
		t.Errorf("GenerateAPIKey() plaintext = %q, want prefix %q", plaintext, wantPrefix)
	}

	secret := plaintext[len(wantPrefix):]
	if HashOpaqueToken(secret) != hash {
		t.Error("GenerateAPIKey() hash does not match HashOpaqueToken(secret)")
	}
}
