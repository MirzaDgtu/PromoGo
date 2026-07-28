-- +goose Up
-- One row per issued refresh token. Only the SHA-256 hash is stored, never
-- the token itself (internal/auth). Rotation on every /auth/refresh call
-- chains sessions via replaced_by_id; reuse of an already-rotated
-- (revoked_at IS NOT NULL) token is treated as compromise and revokes the
-- whole chain for that customer_account_id (internal/service/customerauth.go).
CREATE TABLE customer_sessions (
    id                 BIGSERIAL PRIMARY KEY,
    customer_account_id BIGINT NOT NULL REFERENCES customer_accounts (id),
    refresh_token_hash TEXT NOT NULL UNIQUE,
    replaced_by_id     BIGINT REFERENCES customer_sessions (id),
    issued_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ,
    user_agent         TEXT,
    ip                 TEXT
);

CREATE INDEX idx_customer_sessions_customer_account_id ON customer_sessions (customer_account_id);

-- +goose Down
DROP TABLE customer_sessions;
