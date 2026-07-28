// Package logsms provides a domain.SMSSender that logs the fact that an SMS
// was sent, without ever logging its content or the full destination phone
// — used for OTP delivery until a real SMS provider (SMSC/SMS.ru) is wired
// up, mirroring internal/notification/logchannel's role for push.
package logsms

import (
	"context"
	"log/slog"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
)

// Sender implements domain.SMSSender by logging each send without the
// message body — an OTP code must never reach logs (see
// internal/service/customerauth.go).
type Sender struct {
	log *slog.Logger
}

// New creates a log-based SMS sender. log defaults to slog.Default() if nil.
func New(log *slog.Logger) *Sender {
	if log == nil {
		log = slog.Default()
	}
	return &Sender{log: log}
}

// Send implements domain.SMSSender. It never logs message, which may
// contain an OTP code.
func (s *Sender) Send(ctx context.Context, phone, message string) error {
	s.log.InfoContext(ctx, "sms sent", "phone", auth.MaskPhone(phone), "length", len(message))
	return nil
}
