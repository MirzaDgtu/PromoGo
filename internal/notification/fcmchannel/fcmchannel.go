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

// sendTimeout bounds each per-device FCM send call — push delivery must
// never make an accrual/redeem/refund request hang waiting on a slow or
// unresponsive FCM endpoint (it's already best-effort and post-commit).
const sendTimeout = 5 * time.Second

// messagingSender is the subset of *messaging.Client's API this package
// calls — narrowed to an interface so tests can inject a fake instead of a
// real Firebase connection. *messaging.Client satisfies this (via its
// embedded fcmClient) with no adapter needed.
type messagingSender interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
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
// CustomerAccount, sends message to every active device, and revokes any
// device FCM reports as unregistered/invalid — a self-healing token store
// that needs no separate cleanup job. No devices is a successful no-op.
// Never logs a push token. A send failure for one device never affects
// another, and never propagates as an error from Send (matching
// domain.NotificationChannel's "best-effort, never fails the caller's
// request" contract) — errors are logged, not returned, except when the
// account/device lookup itself fails.
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

	for _, d := range devices {
		sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
		_, err := c.client.Send(sendCtx, &messaging.Message{
			Token:        d.PushToken,
			Notification: &messaging.Notification{Body: message},
		})
		cancel()
		if err == nil {
			continue
		}

		if c.isInvalidToken(err) {
			if revokeErr := c.devices.RevokeByToken(ctx, d.PushToken); revokeErr != nil {
				c.log.WarnContext(ctx, "revoke invalid fcm device token", "device_id", d.ID, "error", revokeErr)
			}
			continue
		}

		c.log.WarnContext(ctx, "send fcm push", "device_id", d.ID, "error", err)
	}

	return nil
}
