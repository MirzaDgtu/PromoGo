-- +goose Up
-- Nullable link from a store-scoped Client to the global CustomerAccount it
-- belongs to, once the customer registers via OTP. A Client can exist
-- indefinitely with customer_account_id NULL (created by the 1C accrual
-- webhook for a phone that has never opened the mobile app) — see
-- internal/service/customerauth.go's VerifyOTP for how existing Client rows
-- get linked on registration.
ALTER TABLE clients ADD COLUMN customer_account_id BIGINT REFERENCES customer_accounts (id);
CREATE INDEX idx_clients_customer_account_id ON clients (customer_account_id);

-- +goose Down
DROP INDEX idx_clients_customer_account_id;
ALTER TABLE clients DROP COLUMN customer_account_id;
