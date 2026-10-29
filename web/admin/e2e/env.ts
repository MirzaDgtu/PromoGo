// Where the E2E stack lives. Defaults match deployments/docker-compose.yml
// and web/admin/README.md "Local OIDC for dev".
export const API_URL = process.env.E2E_API_URL ?? 'http://127.0.0.1:8090'
export const OIDC_ISSUER = process.env.E2E_OIDC_ISSUER ?? 'http://localhost:8091/default'
export const APP_CONTAINER = process.env.E2E_APP_CONTAINER ?? 'promogo-app-1'

// The platform admin every run logs in as; global-setup bootstraps it.
export const ADMIN = {
  subject: process.env.E2E_ADMIN_SUBJECT ?? 'e2e-admin@promogo.test',
  email: 'e2e-admin@promogo.test',
  name: 'E2E Админ',
  // bootstrap-admin grants platform_admin in an existing organization; this
  // one is seeded by the schema migrations.
  homeOrg: 'Default Organization',
}
