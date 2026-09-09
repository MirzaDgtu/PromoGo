-- +goose Up
-- Transactional outbox for accrual/redeem/refund notifications (Phase 3 of
-- docs/audit-remediation-prompt.md; DEC-014/Q-P0-103-outbox, previously
-- deferred). Rows are inserted by LedgerRepository's Post/PostRedeemChecked/
-- PostRefund in the SAME database transaction as the ledger write itself —
-- never by a separate call after the fact — so a notification is enqueued
-- if and only if the transaction it describes actually committed. A worker
-- (internal/service's outboxWorker) polls this table and delivers via the
-- existing domain.NotificationChannel, independent of and never blocking
-- the accrual/redeem/refund request path.
-- 'processing' is a claimed-but-not-yet-resolved row: the claim query (see
-- NotificationOutboxRepository.ClaimBatch) sets it atomically alongside
-- pushing next_attempt_at forward by a lease window, so a worker that
-- crashes mid-delivery self-heals — the row becomes reclaimable again once
-- its lease expires, the same pattern qrStore.claim uses for QR tokens.
CREATE TABLE notification_outbox (
    id              BIGSERIAL PRIMARY KEY,
    client_id       BIGINT NOT NULL REFERENCES clients (id),
    template_id     TEXT NOT NULL,
    template_version INT NOT NULL,
    message         TEXT NOT NULL,
    -- Ties an entry back to the ledger write that created it and prevents
    -- a double-enqueue if a caller somehow invoked the same posting logic
    -- twice for the same (store, type, external_tx_id).
    dedupe_key      TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'delivered', 'dead_letter')),
    attempts        INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at    TIMESTAMPTZ
);

-- The worker's claim query: due pending rows, or a processing row whose
-- claim lease has expired (a crashed worker's abandoned claim) — either
-- way, ordered oldest-due first.
CREATE INDEX idx_notification_outbox_claimable ON notification_outbox (next_attempt_at)
    WHERE status IN ('pending', 'processing');

-- +goose Down
DROP INDEX idx_notification_outbox_claimable;
DROP TABLE notification_outbox;
