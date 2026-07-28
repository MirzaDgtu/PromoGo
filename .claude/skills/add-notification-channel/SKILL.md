---
name: add-notification-channel
description: Scaffold a new domain.NotificationChannel implementation for PromoGo under internal/notification/<name> and wire it in via internal/app/app.go. Use when asked to add support for a new outbound notification provider (real FCM push, SMS via SMSC/SMS.ru, etc.).
---

# Adding a notification channel to PromoGo

The MVP ships only `internal/notification/logchannel`, which logs instead of sending — real
push (Firebase FCM) and SMS (SMSC/SMS.ru, per `Idea.md`'s tech stack table) are not wired up yet.
`internal/app/app.go` has a `TODO(add-notification-channel)` comment marking exactly where to
swap it in.

A channel implements `domain.NotificationChannel` (`internal/domain/notification.go`):

```go
type NotificationChannel interface {
    Name() string
    Send(ctx context.Context, clientID int64, message string) error
}
```

`Send` is called best-effort by `LoyaltyService.Accrue` (`internal/service/loyalty.go`) after a
successful points accrual — a `Send` failure is logged (`WarnContext`) and does **not** fail the
webhook response. Keep any new implementation consistent with that contract: never make `Send`
block for long or return an error that the caller would treat as fatal elsewhere.

## Steps

1. **Create the package** `internal/notification/<name>/<name>.go`. Use
   `internal/notification/logchannel/logchannel.go` as the template for shape: a constructor
   `New(...)`, a `Name() string`, and `Send`.
2. **Client lookup**: `Send` only receives `clientID` (PromoGo's internal id) and the message
   text — a real channel needs a device token (FCM) or phone number (SMS) to actually deliver.
   That means the constructor needs a `domain.ClientRepository` (for phone-based SMS) and/or a
   device-token store (for FCM — there isn't one yet; you'll need a new table + repository, e.g.
   `push_tokens`, via the db-migrate skill, mirroring how a mobile app would register a device).
3. **Credentials**: add a config section under `internal/config/config.go` following the existing
   `FCMConfig` pattern (`fcm.credentials_json`, sourced from `PROMOGO_FCM_CREDENTIALS_JSON` —
   never committed to `configs/config.yaml`). For SMS, add an `SMSConfig` the same way
   (`sms.api_key` from `PROMOGO_SMS_API_KEY`, etc.).
4. **Register in `internal/app/app.go`**: replace the
   `notifier := logchannel.New(log)` line with your channel's constructor, falling back to
   `logchannel.New(log)` when credentials are empty/invalid — mirror CoinGoBot's
   `buildPushNotifier` pattern in its own `internal/app/app.go` (log the misconfiguration as a
   warning, don't fail startup over it).
5. **Tests**: add `<name>_test.go` covering message formatting/mapping logic — mock the actual
   HTTP call to the provider, don't hit a real FCM/SMS endpoint in tests.
6. **Build and test**: `go build ./...` and `go test ./internal/notification/...`.

## Notes

- Only one channel is active at a time in the MVP wiring (`LoyaltyService` takes a single
  `domain.NotificationChannel`). If push and SMS both need to fire (e.g. push primary, SMS
  fallback), that's a small `multichannel` wrapper implementing the same interface and fanning
  out to both — not a change to the interface itself.
- `internal/service/loyalty.go` is the only caller — don't add direct `NotificationChannel` use
  elsewhere; route new notification triggers (e.g. a future "points expiring soon" job) through
  the same interface.
