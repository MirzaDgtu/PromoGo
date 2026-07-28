-- +goose Up
-- CustomerAccount is the global, phone-verified identity of a mobile-app
-- customer — not tied to any one store. It is created only after a
-- successful OTP verification (internal/service/customerauth.go), unlike
-- Client, which the 1C accrual webhook can create unauthenticated for an
-- unknown phone (see clients.customer_account_id in a later migration).
CREATE TABLE customer_accounts (
    id                BIGSERIAL PRIMARY KEY,
    phone             TEXT NOT NULL UNIQUE,
    phone_verified_at TIMESTAMPTZ NOT NULL,
    status            TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked', 'deleted')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE customer_accounts;
