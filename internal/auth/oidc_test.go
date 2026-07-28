package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testIssuer = "https://idp.example.test/"
const testAudience = "promogo-staff"
const testKid = "test-key-1"

func startTestJWKSServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()

	jwk := struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	}{
		Kty: "RSA",
		Kid: testKid,
		N:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(bigEndianBytes(key.PublicKey.E)),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func bigEndianBytes(e int) []byte {
	if e == 65537 {
		return []byte{0x01, 0x00, 0x01}
	}
	b := make([]byte, 0, 4)
	for e > 0 {
		b = append([]byte{byte(e & 0xff)}, b...)
		e >>= 8
	}
	return b
}

func signTestIDToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = testKid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign test id token: %v", err)
	}
	return signed
}

func TestOIDCVerifyIDToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	srv := startTestJWKSServer(t, key)
	verifier := NewOIDCVerifier(testIssuer, testAudience, srv.URL, time.Minute)

	now := time.Now()
	token := signTestIDToken(t, key, jwt.MapClaims{
		"iss":   testIssuer,
		"aud":   testAudience,
		"sub":   "user-123",
		"email": "staff@example.test",
		"name":  "Test Staff",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})

	claims, err := verifier.VerifyIDToken(context.Background(), token)
	if err != nil {
		t.Fatalf("VerifyIDToken() error = %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("VerifyIDToken() Subject = %q, want %q", claims.Subject, "user-123")
	}
	if claims.Email != "staff@example.test" {
		t.Errorf("VerifyIDToken() Email = %q, want %q", claims.Email, "staff@example.test")
	}
}

func TestOIDCVerifyIDTokenRejectsWrongIssuer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	srv := startTestJWKSServer(t, key)
	verifier := NewOIDCVerifier(testIssuer, testAudience, srv.URL, time.Minute)

	now := time.Now()
	token := signTestIDToken(t, key, jwt.MapClaims{
		"iss": "https://attacker.example/", "aud": testAudience, "sub": "user-123",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})

	if _, err := verifier.VerifyIDToken(context.Background(), token); err == nil {
		t.Error("VerifyIDToken() accepted a token with the wrong issuer, want error")
	}
}

func TestOIDCVerifyIDTokenRejectsWrongAudience(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	srv := startTestJWKSServer(t, key)
	verifier := NewOIDCVerifier(testIssuer, testAudience, srv.URL, time.Minute)

	now := time.Now()
	token := signTestIDToken(t, key, jwt.MapClaims{
		"iss": testIssuer, "aud": "some-other-client", "sub": "user-123",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})

	if _, err := verifier.VerifyIDToken(context.Background(), token); err == nil {
		t.Error("VerifyIDToken() accepted a token with the wrong audience, want error")
	}
}

func TestOIDCVerifyIDTokenRejectsExpired(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	srv := startTestJWKSServer(t, key)
	verifier := NewOIDCVerifier(testIssuer, testAudience, srv.URL, time.Minute)

	now := time.Now()
	token := signTestIDToken(t, key, jwt.MapClaims{
		"iss": testIssuer, "aud": testAudience, "sub": "user-123",
		"iat": now.Add(-2 * time.Hour).Unix(), "exp": now.Add(-time.Hour).Unix(),
	})

	if _, err := verifier.VerifyIDToken(context.Background(), token); err == nil {
		t.Error("VerifyIDToken() accepted an expired token, want error")
	}
}

func TestOIDCVerifyIDTokenRejectsWrongSigningKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	attackerKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate attacker rsa key: %v", err)
	}
	srv := startTestJWKSServer(t, key) // JWKS only publishes the legitimate key
	verifier := NewOIDCVerifier(testIssuer, testAudience, srv.URL, time.Minute)

	now := time.Now()
	token := signTestIDToken(t, attackerKey, jwt.MapClaims{
		"iss": testIssuer, "aud": testAudience, "sub": "user-123",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})

	if _, err := verifier.VerifyIDToken(context.Background(), token); err == nil {
		t.Error("VerifyIDToken() accepted a token signed by an unpublished key, want error")
	}
}
