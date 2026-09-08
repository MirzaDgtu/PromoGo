-- +goose Up
-- A refund row snapshots the original transaction's cumulative refund state
-- as of the moment it was posted (see LedgerRepository.PostRefund), so that
-- replaying an old refund request (idempotent on
-- (store_id, 'refund', external_tx_id)) returns the same response it did
-- the first time, rather than either the size of just that one refund or —
-- worse — the ORIGINAL row's current (possibly since-advanced-by-later-
-- refunds) cumulative total. Meaningless (left at defaults) on non-refund
-- rows, mirroring refunded_amount/refunded_points on accrual/redeem rows.
ALTER TABLE transactions ADD COLUMN refund_cumulative_amount NUMERIC(20, 2) NOT NULL DEFAULT 0;
ALTER TABLE transactions ADD COLUMN refund_fully_refunded BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE transactions DROP COLUMN refund_fully_refunded;
ALTER TABLE transactions DROP COLUMN refund_cumulative_amount;
