-- +goose Up
-- The Admin Web overview (DEC-031) aggregates a store's last 7 days of
-- transactions and its latest operation time; until now transactions were
-- only indexed by client, so both scanned every row of the store.
CREATE INDEX idx_transactions_store_created ON transactions (store_id, created_at DESC);

-- +goose Down
DROP INDEX idx_transactions_store_created;
