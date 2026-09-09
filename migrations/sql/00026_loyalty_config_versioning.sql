-- +goose Up
-- Rule versioning (Phase 2 of docs/audit-remediation-prompt.md): every write
-- to loyalty_configs bumps version and appends the full prior effective
-- configuration to loyalty_config_history, so any past accrual/redemption
-- can be reconstructed exactly, config changes have an audit trail, and an
-- operator can roll back to an earlier version (LoyaltyConfigRepository's
-- Rollback re-applies a stored history row through the same Upsert path,
-- which itself creates a new version — history is append-only, never
-- rewritten).
ALTER TABLE loyalty_configs ADD COLUMN version BIGINT NOT NULL DEFAULT 1;

CREATE TABLE loyalty_config_history (
    id                       BIGSERIAL PRIMARY KEY,
    store_id                 BIGINT NOT NULL REFERENCES stores (id),
    version                  BIGINT NOT NULL,
    mechanic                 TEXT NOT NULL,
    accrual_percent          NUMERIC(5, 2) NOT NULL,
    min_purchase_amount      NUMERIC(20, 2) NOT NULL,
    min_balance_to_redeem    BIGINT NOT NULL,
    max_redeem_percent       NUMERIC(5, 2) NOT NULL,
    points_exchange_rate     NUMERIC(10, 4) NOT NULL,
    changed_by_staff_user_id BIGINT REFERENCES staff_users (id),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, version)
);
CREATE INDEX idx_loyalty_config_history_store ON loyalty_config_history (store_id, version DESC);

-- rule_version records which loyalty_configs.version was in effect when a
-- transaction was posted (see internal/service/loyalty.go's Accrue/Redeem),
-- so historical calculations remain reconstructable even after later
-- config changes. NULL on refund rows, which don't consult the mechanic
-- config at all (see LedgerRepository.PostRefund).
ALTER TABLE transactions ADD COLUMN rule_version BIGINT;

-- +goose Down
ALTER TABLE transactions DROP COLUMN rule_version;
DROP INDEX idx_loyalty_config_history_store;
DROP TABLE loyalty_config_history;
ALTER TABLE loyalty_configs DROP COLUMN version;
