// Package fcmchannel implements domain.NotificationChannel over Firebase
// Cloud Messaging, using the official Firebase Admin SDK
// (firebase.google.com/go/v4). It is the production notification channel
// (config.FCMConfig.CredentialsJSON set) — internal/notification/logchannel
// remains the development-only fallback (see DEC-014).
package fcmchannel

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// sendTimeout bounds the whole fanout to one CustomerAccount's devices —
// push delivery must never make an accrual/redeem/refund request hang
// waiting on a slow or unresponsive FCM endpoint (it's already best-effort
// and post-commit). It bounds the batch as a whole, not per device, since
// SendEach parallelizes internally rather than looping sequentially.
const sendTimeout = 5 * time.Second

// maxMessagesPerSendEach is messaging.Client.SendEach's own documented
// limit on how many messages one call accepts. A CustomerAccount is very
// unlikely to ever register this many devices, but chunking defensively
// means a pathological case degrades to multiple calls instead of an error.
const maxMessagesPerSendEach = 500

// messagingSender is the subset of *messaging.Client's API this package
// calls — narrowed to an interface so tests can inject a fake instead of a
// real Firebase connection. *messaging.Client satisfies this (via its
// embedded fcmClient) with no adapter needed. SendEach (not the deprecated
// SendAll) is used for multi-device fanout: unlike a manual per-device loop,
// it parallelizes internally with SDK-managed bounded concurrency instead
// of an unbounded sequential five-second call per device.
type messagingSender interface {
	SendEach(ctx context.Context, messages []*messaging.Message) (*messaging.BatchResponse, error)
}

// Channel implements domain.NotificationChannel over FCM.
type Channel struct {
	log     *slog.Logger
	client  messagingSender
	clients domain.ClientRepository
	devices domain.CustomerDeviceRepository
	// isInvalidToken classifies a Send error as "this token is
	// permanently dead, revoke it" — overridable in tests, since the real
	// classification (messaging.IsRegistrationTokenNotRegistered /
	// IsInvalidArgument) inspects an unexported Firebase SDK error type
	// that can't be constructed outside the SDK.
	isInvalidToken func(error) bool
}

// New constructs a Channel from raw Firebase service-account JSON
// credentials (config.FCMConfig.CredentialsJSON).
func New(ctx context.Context, credentialsJSON string, clients domain.ClientRepository, devices domain.CustomerDeviceRepository, log *slog.Logger) (*Channel, error) {
	if log == nil {
		log = slog.Default()
	}

	//lint:ignore SA1019 raw service-account JSON is the current supported
	// credential path (config.FCMConfig.CredentialsJSON); migrating to
	// Application Default Credentials/workload identity is tracked as a
	// deployment-platform decision in docs/audit-remediation-prompt.md
	// Phase 3, not a same-signature swap.
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		return nil, fmt.Errorf("init firebase app: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("init firebase messaging client: %w", err)
	}

	return &Channel{log: log, client: client, clients: clients, devices: devices, isInvalidToken: isInvalidTokenError}, nil
}

func isInvalidTokenError(err error) bool {
	return messaging.IsUnregistered(err) || messaging.IsInvalidArgument(err)
}

func (c *Channel) Name() string { return "fcm" }

// Send implements domain.NotificationChannel: resolves clientID's linked
// CustomerAccount, sends message to every active device in bounded-
// concurrency batches via SendEach (not a sequential per-device loop — see
// messagingSender's doc comment), and revokes any device FCM reports as
// unregistered/invalid — a self-healing token store that needs no separate
// cleanup job. No devices is a successful no-op. Never logs a push token. A
// send failure for one device never affects another, and never propagates
// as an error from Send (matching domain.NotificationChannel's
// "best-effort, never fails the caller's request" contract) — errors are
// logged, not returned, except when the account/device lookup itself fails.
func (c *Channel) Send(ctx context.Context, clientID int64, message string) error {
	client, err := c.clients.GetByID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("fcm: load client %d: %w", clientID, err)
	}
	if client.CustomerAccountID == nil {
		// No linked mobile-app identity yet — nothing to push to.
		return nil
	}

	devices, err := c.devices.ListActiveByCustomerAccount(ctx, *client.CustomerAccountID)
	if err != nil {
		return fmt.Errorf("fcm: list devices for account %d: %w", *client.CustomerAccountID, err)
	}
	if len(devices) == 0 {
		return nil
	}

	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	for start := 0; start < len(devices); start += maxMessagesPerSendEach {
		end := min(start+maxMessagesPerSendEach, len(devices))
		batch := devices[start:end]

		messages := make([]*messaging.Message, len(batch))
		for i, d := range batch {
			messages[i] = &messaging.Message{
				Token:        d.PushToken,
				Notification: &messaging.Notification{Body: message},
			}
		}

		resp, err := c.client.SendEach(sendCtx, messages)
		if err != nil {
			// A total failure (see SendEach's doc comment) — nothing in
			// this batch was delivered; log once rather than per device.
			c.log.WarnContext(ctx, "send fcm push batch", "batch_size", len(batch), "error", err)
			continue
		}

		for i, r := range resp.Responses {
			if r.Success {
				continue
			}
			d := batch[i]
			if c.isInvalidToken(r.Error) {
				if revokeErr := c.devices.RevokeByToken(ctx, d.PushToken); revokeErr != nil {
					c.log.WarnContext(ctx, "revoke invalid fcm device token", "device_id", d.ID, "error", revokeErr)
				}
				continue
			}
			c.log.WarnContext(ctx, "send fcm push", "device_id", d.ID, "error", r.Error)
		}
	}

	return nil
}
