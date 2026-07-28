-- +goose Up
-- StaffUser is a retailer employee or platform admin, authenticated via
-- OIDC (internal/auth/oidc.go) — never a store-scoped Client and never
-- sharing a table with customer_accounts or store_api_keys, per the
-- three-contour split (see knowledge/Decisions.md).
CREATE TABLE staff_users (
    id                BIGSERIAL PRIMARY KEY,
    external_subject  TEXT NOT NULL UNIQUE,
    email             TEXT NOT NULL,
    display_name      TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- StaffMembership grants a fixed MVP role within an Organization, optionally
-- narrowed to one Store (store_id NULL means "all stores in the
-- organization"). Permission checks (internal/domain/rbac.go) resolve a
-- caller's permissions from active memberships matching the request's
-- organization/store scope — handlers never switch on role name directly.
CREATE TABLE staff_memberships (
    id             BIGSERIAL PRIMARY KEY,
    staff_user_id  BIGINT NOT NULL REFERENCES staff_users (id),
    organization_id BIGINT NOT NULL REFERENCES organizations (id),
    store_id       BIGINT REFERENCES stores (id),
    role           TEXT NOT NULL CHECK (role IN ('platform_admin', 'retailer_admin', 'store_manager', 'support_viewer')),
    status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (staff_user_id, organization_id, store_id)
);

CREATE INDEX idx_staff_memberships_staff_user_id ON staff_memberships (staff_user_id);
CREATE INDEX idx_staff_memberships_organization_id ON staff_memberships (organization_id);

-- +goose Down
DROP TABLE staff_memberships;
DROP TABLE staff_users;
