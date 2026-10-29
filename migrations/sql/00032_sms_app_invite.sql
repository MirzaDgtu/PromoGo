-- +goose Up
-- SMS invite to the mobile app for a client the register just registered
-- (DEC-034, MVP-scope.md "Регистрация нового клиента кассиром"). Invites are
-- off by default: the retailer turns them on per store and answers for the
-- customer's consent at the till.
ALTER TABLE stores ADD COLUMN sms_invite_enabled BOOLEAN NOT NULL DEFAULT false;

-- The outbox now carries two kinds of entries: push notifications keyed by
-- client (the existing ones) and SMS sent to the client's phone. Existing
-- rows are all push.
ALTER TABLE notification_outbox ADD COLUMN channel TEXT NOT NULL DEFAULT 'push'
    CHECK (channel IN ('push', 'sms'));

-- The per-store daily invite cap counts recent invite rows (joined to
-- clients for the store).
CREATE INDEX idx_notification_outbox_invites ON notification_outbox (created_at)
    WHERE template_id = 'app_invite';

-- +goose Down
DROP INDEX idx_notification_outbox_invites;
ALTER TABLE notification_outbox DROP COLUMN channel;
ALTER TABLE stores DROP COLUMN sms_invite_enabled;
