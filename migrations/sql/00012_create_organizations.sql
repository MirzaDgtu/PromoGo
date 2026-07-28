-- +goose Up
-- Organization is the tenant parent of Store: a retailer or trading network.
-- Introduced to support staff RBAC scoping (a StaffMembership grants a role
-- within an organization, optionally narrowed to one store) and future
-- multi-store network mode (knowledge/Project Questions.md Q-P1-113/114).
CREATE TABLE organizations (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE organizations;
