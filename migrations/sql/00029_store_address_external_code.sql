-- +goose Up
-- Store profile fields for Admin Web onboarding (DEC-026). address is a
-- free-form display string (no parsing or geocoding); '' means not set.
-- external_code is the store's identifier in the retailer's 1C, used to
-- match stores during integration — optional, but unique within an
-- organization when present, so two stores can never claim the same 1C code.
ALTER TABLE stores
    ADD COLUMN address       TEXT NOT NULL DEFAULT '',
    ADD COLUMN external_code TEXT;

CREATE UNIQUE INDEX idx_stores_org_external_code ON stores (organization_id, external_code)
    WHERE external_code IS NOT NULL;

-- +goose Down
DROP INDEX idx_stores_org_external_code;
ALTER TABLE stores
    DROP COLUMN external_code,
    DROP COLUMN address;
