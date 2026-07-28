-- +goose Up
-- New stores created via the admin API (internal/httpserver/admin_organizations.go)
-- rely solely on store_api_keys (migrations/00019) and never populate the
-- legacy single-key column. NULL is safe under the existing UNIQUE
-- constraint — Postgres does not treat multiple NULLs as duplicates.
ALTER TABLE stores ALTER COLUMN api_key_hash DROP NOT NULL;

-- +goose Down
-- Only safe if no store row has a NULL api_key_hash at rollback time (i.e.
-- every store created after this migration was applied has since been
-- given a legacy key, or removed).
ALTER TABLE stores ALTER COLUMN api_key_hash SET NOT NULL;
