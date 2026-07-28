-- +goose Up
CREATE TABLE transactions (
    id             BIGSERIAL PRIMARY KEY,
    store_id       BIGINT NOT NULL REFERENCES stores (id),
    client_id      BIGINT NOT NULL REFERENCES clients (id),
    external_tx_id TEXT NOT NULL,
    amount         NUMERIC(20, 2) NOT NULL,
    type           TEXT NOT NULL,
    points_delta   BIGINT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Idempotency guarantee: a replayed 1C webhook with the same external_tx_id
-- for the same store must not be processed twice (see Idea.md's
-- "Идемпотентность" and TransactionRepository.Create).
CREATE UNIQUE INDEX idx_transactions_store_external ON transactions (store_id, external_tx_id);
CREATE INDEX idx_transactions_client ON transactions (client_id, created_at DESC);

-- +goose Down
DROP TABLE transactions;
