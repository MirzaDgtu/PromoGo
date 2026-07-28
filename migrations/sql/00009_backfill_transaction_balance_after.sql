-- +goose Up
-- Migration 00007 added balance_after NOT NULL DEFAULT 0 then dropped the
-- default, with no backfill: any row that existed before 00007 ran would
-- carry a wrong balance_after=0, and a replay of it would report a wrong
-- balance (see internal/service/loyalty.go's replayedAccrue/replayedRedeem).
-- This recomputes every row's running balance from points_delta history —
-- correct for rows already correct too (harmlessly re-deriving the same
-- value), so it's safe to run regardless of whether any such rows exist.
UPDATE transactions AS t
SET balance_after = sub.running_balance
FROM (
    SELECT id, SUM(points_delta) OVER (PARTITION BY client_id ORDER BY created_at, id) AS running_balance
    FROM transactions
) AS sub
WHERE t.id = sub.id;

-- +goose Down
UPDATE transactions SET balance_after = 0;
