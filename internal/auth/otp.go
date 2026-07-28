package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// otpMax bounds the 6-digit numeric OTP space.
var otpMax = big.NewInt(1_000_000)

// GenerateOTPCode returns a cryptographically random 6-digit numeric code
// via crypto/rand (never math/rand — OTP codes gate account access).
func GenerateOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, otpMax)
	if err != nil {
		return "", fmt.Errorf("generate otp code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashOTP returns the SHA-256 hash of an OTP code for storage — the
// plaintext code is never persisted, only ever held in memory long enough
// to hash and send it.
func HashOTP(code string) string {
	return HashOpaqueToken(code)
}

// VerifyOTP reports whether code hashes to want, in constant time.
func VerifyOTP(code, want string) bool {
	return constantTimeEqual(HashOTP(code), want)
}
