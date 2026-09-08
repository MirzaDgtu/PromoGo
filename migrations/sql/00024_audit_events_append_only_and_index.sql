-- +goose Up
-- Composite index for the org-scoped, newest-first listing
-- (AuditEventRepository.ListByOrganization) and future keyset pagination
-- over it (see internal/service/loyalty.go's ListByClientIDs / /me/transactions
-- for the established keyset-pagination pattern this mirrors) — id is a
-- stable tiebreaker for events sharing the same occurred_at, which the
-- prior single-column indexes couldn't provide. Replaces
-- idx_audit_events_organization_id, which this covers entirely as a
-- leading-column prefix.
CREATE INDEX idx_audit_events_org_occurred_id ON audit_events (organization_id, occurred_at DESC, id DESC);
DROP INDEX idx_audit_events_organization_id;

-- Append-only enforcement: audit_events is a security/compliance record —
-- nothing in the application should ever be able to alter or remove a row
-- once written, whatever DB role it connects as (a REVOKE would need to
-- target a specific, distinct app role, which this deployment doesn't
-- assume exists). A deliberate retention/export job is expected to run
-- separately, as a maintenance operation that disables this trigger for
-- its own session (e.g. `ALTER TABLE audit_events DISABLE TRIGGER
-- audit_events_append_only`) rather than going through the app's normal
-- write path.
-- +goose StatementBegin
CREATE FUNCTION reject_audit_events_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only: % is not permitted', TG_OP;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_events_append_only
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION reject_audit_events_mutation();

-- +goose Down
DROP TRIGGER audit_events_append_only ON audit_events;
DROP FUNCTION reject_audit_events_mutation();
CREATE INDEX idx_audit_events_organization_id ON audit_events (organization_id);
DROP INDEX idx_audit_events_org_occurred_id;
