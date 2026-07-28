-- +goose Up
-- Every store must belong to an Organization (tenant). The single-store MVP
-- has no operator-facing way to create an Organization yet, so this
-- migration seeds one default Organization and backfills every existing
-- store onto it — the pilot keeps working with zero manual ops, and new
-- stores can be assigned to a real Organization once the admin API exists.
ALTER TABLE stores ADD COLUMN organization_id BIGINT REFERENCES organizations (id);

INSERT INTO organizations (name) VALUES ('Default Organization');

UPDATE stores
SET organization_id = (SELECT id FROM organizations WHERE name = 'Default Organization')
WHERE organization_id IS NULL;

ALTER TABLE stores ALTER COLUMN organization_id SET NOT NULL;

-- +goose Down
ALTER TABLE stores ALTER COLUMN organization_id DROP NOT NULL;
ALTER TABLE stores DROP COLUMN organization_id;

-- Only removes the row this migration itself inserted, and only if nothing
-- else came to reference it in the meantime (fails loudly instead of
-- silently orphaning references if e.g. staff_memberships still points at
-- it and its own migration hasn't been rolled back yet).
DELETE FROM organizations WHERE name = 'Default Organization';
