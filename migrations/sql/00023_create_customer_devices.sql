-- +goose Up
-- FCM push-token storage (Q-P0-103/DEC-014). A customer can have multiple
-- devices; push_token uniqueness is scoped to "currently active" (via the
-- partial index) rather than global, so a revoked/reinstalled token can be
-- re-registered by the same or a different account without a dead unique
-- constraint blocking it forever.
CREATE TABLE customer_devices (
    id                  BIGSERIAL PRIMARY KEY,
    customer_account_id BIGINT NOT NULL REFERENCES customer_accounts (id),
    platform            TEXT NOT NULL,
    push_token          TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at          TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_customer_devices_active_token ON customer_devices (push_token) WHERE revoked_at IS NULL;
CREATE INDEX idx_customer_devices_customer_account ON customer_devices (customer_account_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE customer_devices;
