-- +goose Up
-- DB-level backstop so a transaction row can never reference a client
-- belonging to a different store than the transaction's own store_id —
-- defense in depth alongside the application-level check in
-- LoyaltyService.Redeem (see internal/service/loyalty.go).
ALTER TABLE clients ADD CONSTRAINT clients_id_store_id_key UNIQUE (id, store_id);

ALTER TABLE transactions DROP CONSTRAINT transactions_client_id_fkey;
ALTER TABLE transactions
    ADD CONSTRAINT transactions_client_store_fkey
    FOREIGN KEY (client_id, store_id) REFERENCES clients (id, store_id);

-- +goose Down
ALTER TABLE transactions DROP CONSTRAINT transactions_client_store_fkey;
ALTER TABLE transactions
    ADD CONSTRAINT transactions_client_id_fkey
    FOREIGN KEY (client_id) REFERENCES clients (id);

ALTER TABLE clients DROP CONSTRAINT clients_id_store_id_key;
