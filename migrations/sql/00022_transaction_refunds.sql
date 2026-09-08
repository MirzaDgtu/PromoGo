-- +goose Up
-- Refund linkage: a refund transaction (type='refund', already allowed by
-- transactions_type_check since 00007) must reference the original
-- accrual/redeem it reverses. refunded_amount/refunded_points are kept on
-- the ORIGINAL row and updated atomically by every partial refund against
-- it (see internal/repository/postgres/ledger_repository.go's PostRefund),
-- so the over-refund check never needs a separate read-then-write race
-- between the service layer and the database.
ALTER TABLE transactions ADD COLUMN original_transaction_id BIGINT REFERENCES transactions (id);
ALTER TABLE transactions ADD COLUMN refunded_amount NUMERIC(20, 2) NOT NULL DEFAULT 0;
ALTER TABLE transactions ADD COLUMN refunded_points BIGINT NOT NULL DEFAULT 0;

ALTER TABLE transactions ADD CONSTRAINT transactions_refund_reference_check CHECK (
    (type = 'refund' AND original_transaction_id IS NOT NULL) OR
    (type != 'refund' AND original_transaction_id IS NULL)
);

-- Refund-of-refund is rejected in Go inside the same locked transaction
-- that posts the refund (PostRefund reads and checks the original row's
-- type under SELECT ... FOR UPDATE), not here — a CHECK constraint can't
-- see another row's type without a trigger, and this repo keeps
-- business rules in Go with SQL used only for atomicity.
CREATE INDEX idx_transactions_original ON transactions (original_transaction_id) WHERE original_transaction_id IS NOT NULL;

-- Refunding an accrual is allowed to drive a balance negative (DEC-012):
-- the client must not keep points earned on a purchase that was reversed.
-- Ordinary redemption's non-negative guarantee does NOT depend on this
-- CHECK — it's enforced by LedgerRepository's guarded
-- "UPDATE ... WHERE points + $2 >= 0" independently of any DB constraint.
ALTER TABLE balances DROP CONSTRAINT balances_points_check;

-- +goose Down
-- Re-adding the non-negative CHECK will fail loudly (and correctly) if any
-- balance is currently negative as a result of an accrual refund — that is
-- intentional: an operator must resolve those balances (or accept the data
-- loss of clamping them) before this rollback can succeed. This must not be
-- made to pass silently.
ALTER TABLE balances ADD CONSTRAINT balances_points_check CHECK (points >= 0);

DROP INDEX idx_transactions_original;
ALTER TABLE transactions DROP CONSTRAINT transactions_refund_reference_check;
ALTER TABLE transactions DROP COLUMN refunded_points;
ALTER TABLE transactions DROP COLUMN refunded_amount;
ALTER TABLE transactions DROP COLUMN original_transaction_id;
