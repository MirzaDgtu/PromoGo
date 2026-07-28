package auth

import (
	"testing"
	"unicode"
)

func TestGenerateOTPCodeIsSixDigits(t *testing.T) {
	for i := 0; i < 20; i++ {
		code, err := GenerateOTPCode()
		if err != nil {
			t.Fatalf("GenerateOTPCode() error = %v", err)
		}
		if len(code) != 6 {
			t.Fatalf("GenerateOTPCode() = %q, want length 6", code)
		}
		for _, r := range code {
			if !unicode.IsDigit(r) {
				t.Fatalf("GenerateOTPCode() = %q, contains non-digit", code)
			}
		}
	}
}

func TestVerifyOTP(t *testing.T) {
	hash := HashOTP("123456")

	if !VerifyOTP("123456", hash) {
		t.Error("VerifyOTP() = false for the correct code, want true")
	}
	if VerifyOTP("654321", hash) {
		t.Error("VerifyOTP() = true for an incorrect code, want false")
	}
}
