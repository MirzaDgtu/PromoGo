package auth

import (
	"errors"
	"strings"
)

// ErrInvalidPhone is returned by NormalizePhone for input that isn't a
// plausible phone number.
var ErrInvalidPhone = errors.New("invalid phone number")

// NormalizePhone canonicalizes a phone number to E.164 so the same customer
// resolves to one CustomerAccount and one Client row regardless of which
// format 1C, a cashier, or the mobile app sent it in. Russian numbers are
// the primary case (PromoGo's initial market, per Idea.md): an 11-digit
// number starting with "8" or "7" and a 10-digit number starting with "9"
// (bare mobile prefix, common POS input) are both rewritten to the "+7"
// country code. Any other input is treated as an already-international
// number: digits only, "+" prefixed.
func NormalizePhone(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()

	switch {
	case len(d) == 11 && (d[0] == '8' || d[0] == '7'):
		d = "7" + d[1:]
	case len(d) == 10 && d[0] == '9':
		d = "7" + d
	}

	if len(d) < 10 || len(d) > 15 {
		return "", ErrInvalidPhone
	}
	return "+" + d, nil
}

// MaskPhone redacts all but the last 4 digits of a phone number, for admin
// APIs (support_viewer's masked client view) and logs (see
// internal/notification/logsms) that must never show a full phone number.
func MaskPhone(phone string) string {
	if len(phone) <= 4 {
		return strings.Repeat("*", len(phone))
	}
	visible := phone[len(phone)-4:]
	return strings.Repeat("*", len(phone)-4) + visible
}
