-- +goose Up
-- Immutable record of PDPA/152-FZ consent granted at registration: which
-- document version, when, from where. Never updated or deleted in place —
-- a changed consent is a new row (knowledge/Project Questions.md Q-P0-048).
CREATE TABLE customer_consents (
    id                  BIGSERIAL PRIMARY KEY,
    customer_account_id BIGINT NOT NULL REFERENCES customer_accounts (id),
    document_version    TEXT NOT NULL,
    granted_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    source              TEXT NOT NULL,
    ip                  TEXT,
    user_agent          TEXT
);

CREATE INDEX idx_customer_consents_customer_account_id ON customer_consents (customer_account_id);

-- +goose Down
DROP TABLE customer_consents;
