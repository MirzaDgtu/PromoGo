-- +goose Up
-- Defense in depth alongside the HTTP-layer validation in
-- internal/httpserver/transactions.go: a negative amount, or a points_delta
-- whose sign doesn't match its transaction type, should never reach the
-- database regardless of which layer let it through.
ALTER TABLE transactions ADD CONSTRAINT transactions_amount_check CHECK (amount >= 0);
ALTER TABLE transactions ADD CONSTRAINT transactions_points_delta_sign_check CHECK (
    (type = 'accrual' AND points_delta >= 0) OR
    (type = 'redeem' AND points_delta <= 0) OR
    (type = 'refund')
);

-- +goose Down
ALTER TABLE transactions DROP CONSTRAINT transactions_points_delta_sign_check;
ALTER TABLE transactions DROP CONSTRAINT transactions_amount_check;
