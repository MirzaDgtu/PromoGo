-- +goose Up
CREATE TABLE loyalty_configs (
    store_id              BIGINT PRIMARY KEY REFERENCES stores (id),
    mechanic              TEXT NOT NULL DEFAULT 'points',
    accrual_percent       NUMERIC(5, 2) NOT NULL,
    min_purchase_amount   NUMERIC(20, 2) NOT NULL DEFAULT 0,
    min_balance_to_redeem BIGINT NOT NULL DEFAULT 0,
    max_redeem_percent    NUMERIC(5, 2) NOT NULL DEFAULT 0,
    points_exchange_rate  NUMERIC(10, 4) NOT NULL DEFAULT 1
);

-- +goose Down
DROP TABLE loyalty_configs;
