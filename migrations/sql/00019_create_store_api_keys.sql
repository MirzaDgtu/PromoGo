-- +goose Up
-- Multi-key hardening of the store webhook credential. stores.api_key_hash
-- (the original single-key column, migrations/00001) is left untouched so
-- existing deployed keys keep authenticating unchanged; RequireStoreAPIKey
-- checks this table first and falls back to that column, so adopting
-- rotation is opt-in per store (knowledge/Project Questions.md Q-P0-077).
-- key_id/key_hash split lets an admin list and revoke a specific key by its
-- non-secret prefix without ever storing or logging the plaintext key.
CREATE TABLE store_api_keys (
    id            BIGSERIAL PRIMARY KEY,
    store_id      BIGINT NOT NULL REFERENCES stores (id),
    key_id        TEXT NOT NULL UNIQUE,
    key_hash      TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    scopes        TEXT[] NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,
    last_used_at  TIMESTAMPTZ,
    created_by    BIGINT REFERENCES staff_users (id)
);

CREATE INDEX idx_store_api_keys_store_id ON store_api_keys (store_id);

-- +goose Down
DROP TABLE store_api_keys;
