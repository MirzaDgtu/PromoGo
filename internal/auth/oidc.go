package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrOIDCVerification is returned for any staff OIDC ID token that fails
// signature, issuer, audience, or expiry verification.
var ErrOIDCVerification = errors.New("oidc verification failed")

// OIDCClaims are the subset of ID token claims PromoGo consumes.
type OIDCClaims struct {
	Subject string
	Email   string
	Name    string
}

// OIDCVerifier verifies staff OIDC ID tokens against a configured issuer's
// JWKS: signature (RS256), issuer, audience, and expiry. It fetches and
// caches signing keys itself, keyed by "kid", rather than depending on an
// OIDC client library — see knowledge/Decisions.md for that call. The
// issuer is provider-agnostic: any OIDC-compliant IdP (Keycloak, Auth0,
// Azure AD, ...) works as long as IssuerURL/Audience/JWKSURL are configured.
type OIDCVerifier struct {
	issuer   string
	audience string
	jwksURL  string
	client   *http.Client
	cacheTTL time.Duration

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// NewOIDCVerifier constructs an OIDCVerifier. cacheTTL bounds both how long
// a fetched JWKS is trusted and, since a failed fetch also stamps
// fetchedAt, how often a failing JWKS endpoint is retried — a short-lived
// negative cache against retry storms.
func NewOIDCVerifier(issuer, audience, jwksURL string, cacheTTL time.Duration) *OIDCVerifier {
	return &OIDCVerifier{
		issuer:   issuer,
		audience: audience,
		jwksURL:  jwksURL,
		client:   &http.Client{Timeout: 5 * time.Second},
		cacheTTL: cacheTTL,
		keys:     make(map[string]*rsa.PublicKey),
	}
}

type oidcIDTokenClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

// VerifyIDToken verifies rawToken's signature against the cached JWKS and
// checks issuer, audience, and expiry. Returns ErrOIDCVerification on any
// failure.
func (v *OIDCVerifier) VerifyIDToken(ctx context.Context, rawToken string) (*OIDCClaims, error) {
	var claims oidcIDTokenClaims
	_, err := jwt.ParseWithClaims(rawToken, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid header")
		}
		return v.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOIDCVerification, err)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: missing subject claim", ErrOIDCVerification)
	}

	return &OIDCClaims{Subject: claims.Subject, Email: claims.Email, Name: claims.Name}, nil
}

func (v *OIDCVerifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	key, ok := v.keys[kid]
	fresh := time.Since(v.fetchedAt) < v.cacheTTL
	v.mu.Unlock()
	if ok && fresh {
		return key, nil
	}

	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	key, ok = v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}
	return key, nil
}

type jwksDocument struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *OIDCVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("build jwks request: %w", err)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned status %d", resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()

	return nil
}

func rsaPublicKeyFromJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
