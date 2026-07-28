-- +goose Up
-- Without these, e.g. a negative accrual_percent silently turns accrual
-- into stealth point deduction (see internal/mechanic/points/points.go),
-- and other out-of-range values have similarly unintended effects.
ALTER TABLE loyalty_configs
    ADD CONSTRAINT loyalty_configs_accrual_percent_check CHECK (accrual_percent >= 0 AND accrual_percent <= 100),
    ADD CONSTRAINT loyalty_configs_min_purchase_amount_check CHECK (min_purchase_amount >= 0),
    ADD CONSTRAINT loyalty_configs_min_balance_to_redeem_check CHECK (min_balance_to_redeem >= 0),
    ADD CONSTRAINT loyalty_configs_max_redeem_percent_check CHECK (max_redeem_percent >= 0 AND max_redeem_percent <= 100),
    ADD CONSTRAINT loyalty_configs_points_exchange_rate_check CHECK (points_exchange_rate > 0);

-- +goose Down
ALTER TABLE loyalty_configs
    DROP CONSTRAINT loyalty_configs_accrual_percent_check,
    DROP CONSTRAINT loyalty_configs_min_purchase_amount_check,
    DROP CONSTRAINT loyalty_configs_min_balance_to_redeem_check,
    DROP CONSTRAINT loyalty_configs_max_redeem_percent_check,
    DROP CONSTRAINT loyalty_configs_points_exchange_rate_check;
