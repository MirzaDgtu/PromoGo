-- +goose Up
-- Idempotency replay checks used to compare a handful of individual fields
-- (client, amount, ...) ad hoc — easy to under-specify, and indeed
-- request_points was missed for redemptions (see
-- internal/service/loyalty.go's accrualFingerprint/redeemFingerprint). This
-- column stores one canonical fingerprint string per transaction, computed
-- identically at write time and replay-check time, so "does this replay
-- match the original request" is one string comparison instead of an
-- easy-to-miss list of fields.
ALTER TABLE transactions ADD COLUMN request_fingerprint TEXT NOT NULL DEFAULT '';

-- Backfill pre-existing rows so replays of them don't spuriously conflict
-- post-migration. The format must exactly match accrualFingerprint /
-- redeemFingerprint in internal/service/loyalty.go.
--
-- Caveat: for redeem rows, "points" here is reconstructed from
-- points_delta (the actual, possibly MaxRedeemPercent-capped amount
-- applied), not the original raw requested points, which was never stored
-- before this migration. A replay of a pre-migration, capped redemption
-- with its true original "points" value will therefore see a fingerprint
-- mismatch and get a 409 instead of a replay — a safe failure (blocks an
-- edge-case replay) rather than an unsafe one (never silently misapplies
-- money).
UPDATE transactions
SET request_fingerprint = CASE
    WHEN type = 'accrual' THEN 'client=' || client_id || ';amount=' || amount::text
    WHEN type = 'redeem'  THEN 'client=' || client_id || ';amount=' || amount::text || ';points=' || (-points_delta)::text
    ELSE 'client=' || client_id || ';amount=' || amount::text
END
WHERE request_fingerprint = '';

ALTER TABLE transactions ALTER COLUMN request_fingerprint DROP DEFAULT;

-- +goose Down
ALTER TABLE transactions DROP COLUMN request_fingerprint;
