-- +goose Up
-- UNIQUE (staff_user_id, organization_id, store_id) from 00018 never
-- applied to organization-wide memberships: NULL store_ids are distinct in
-- Postgres, so the same person could be granted org-wide access twice.
-- This partial index closes that gap; StaffMembershipRepository.Create
-- already maps the 23505 to ErrConflict (409). Fails loudly if duplicates
-- already exist — resolve them by hand rather than guessing which to keep.
CREATE UNIQUE INDEX idx_staff_memberships_org_wide ON staff_memberships (staff_user_id, organization_id)
    WHERE store_id IS NULL;

-- +goose Down
DROP INDEX idx_staff_memberships_org_wide;
