-- +goose Up
-- Append-only audit trail for security-relevant events across all three
-- auth contours: customer login/logout/OTP failures, staff role changes,
-- API key rotation/revocation. actor_type distinguishes which contour
-- produced the event ('customer', 'staff', 'store_api_key', 'system');
-- actor_id is that contour's own id space, not a foreign key, since it can
-- point at any of three unrelated tables.
CREATE TABLE audit_events (
    id              BIGSERIAL PRIMARY KEY,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_type      TEXT NOT NULL CHECK (actor_type IN ('customer', 'staff', 'store_api_key', 'system')),
    actor_id        BIGINT,
    organization_id BIGINT REFERENCES organizations (id),
    store_id        BIGINT REFERENCES stores (id),
    action          TEXT NOT NULL,
    target_type     TEXT,
    target_id       BIGINT,
    metadata        JSONB NOT NULL DEFAULT '{}',
    ip              TEXT,
    user_agent      TEXT
);

CREATE INDEX idx_audit_events_occurred_at ON audit_events (occurred_at);
CREATE INDEX idx_audit_events_organization_id ON audit_events (organization_id);

-- +goose Down
DROP TABLE audit_events;
