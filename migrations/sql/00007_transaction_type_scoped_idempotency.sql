-- +goose Up
-- Accrual and redemption are separate operations that may legitimately
-- reuse the same external_tx_id (e.g. the same receipt/check number), so
-- the idempotency key must include type — otherwise a redemption on a
-- receipt gets misread as a replay of that receipt's accrual, or vice versa
-- (see internal/service/loyalty.go's replayedAccrue/replayedRedeem).
DROP INDEX idx_transactions_store_external;
CREATE UNIQUE INDEX idx_transactions_store_type_external ON transactions (store_id, type, external_tx_id);

-- balance_after lets a replayed request return the exact balance from the
-- original operation instead of the client's current balance, which may
-- have moved since (see LedgerRepository.Post).
ALTER TABLE transactions ADD COLUMN balance_after BIGINT NOT NULL DEFAULT 0;
ALTER TABLE transactions ALTER COLUMN balance_after DROP DEFAULT;

ALTER TABLE transactions ADD CONSTRAINT transactions_type_check CHECK (type IN ('accrual', 'redeem', 'refund'));

-- +goose Down
ALTER TABLE transactions DROP CONSTRAINT transactions_type_check;

ALTER TABLE transactions DROP COLUMN balance_after;

DROP INDEX idx_transactions_store_type_external;
CREATE UNIQUE INDEX idx_transactions_store_external ON transactions (store_id, external_tx_id);
