package domain

import "context"

// SMSSender delivers an SMS message to a phone number. Distinct from
// NotificationChannel (which is keyed by an existing Client ID): OTP
// delivery happens before any Client or CustomerAccount necessarily exists
// for that phone, so the recipient here is the phone number itself.
type SMSSender interface {
	Send(ctx context.Context, phone, message string) error
}
