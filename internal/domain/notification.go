package domain

import "context"

// NotificationChannel sends a message to a client through one outbound
// channel (push, SMS). It is the pluggable-provider analogue of an
// exchange: LoyaltyService calls Send best-effort after a successful
// accrual and never fails the request if it errors.
type NotificationChannel interface {
	Name() string
	Send(ctx context.Context, clientID int64, message string) error
}
