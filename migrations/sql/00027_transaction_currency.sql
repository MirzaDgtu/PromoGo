-- +goose Up
-- Currency on every transaction row (Phase 2 of docs/audit-remediation-
-- prompt.md: "Include purchase amount, currency, merchant/store label and
-- operation date in customer history as required by the MVP"). The MVP has
-- no multi-currency support — every store transacts in rubles — so this is
-- a fixed default rather than a per-store setting for now; the column
-- exists so a future multi-currency mechanic doesn't need another
-- migration, and so API responses aren't silently ambiguous about what
-- "amount" means.
ALTER TABLE transactions ADD COLUMN currency TEXT NOT NULL DEFAULT 'RUB';

-- +goose Down
ALTER TABLE transactions DROP COLUMN currency;
