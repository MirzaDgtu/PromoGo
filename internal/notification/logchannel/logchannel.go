// Package logchannel provides a domain.NotificationChannel that logs
// messages instead of sending them. It is the default channel until a real
// FCM/SMS integration is wired up (see the add-notification-channel skill),
// so device-facing flows are testable without live push/SMS credentials.
package logchannel

import (
	"context"
	"log/slog"
)

// Channel implements domain.NotificationChannel by logging each send.
type Channel struct {
	log *slog.Logger
}

// New creates a log-based notification channel. log defaults to
// slog.Default() if nil.
func New(log *slog.Logger) *Channel {
	if log == nil {
		log = slog.Default()
	}
	return &Channel{log: log}
}

// Name returns the channel identifier.
func (c *Channel) Name() string {
	return "log"
}

// Send implements domain.NotificationChannel.
func (c *Channel) Send(ctx context.Context, clientID int64, message string) error {
	c.log.InfoContext(ctx, "notification", "client_id", clientID, "message", message)
	return nil
}
