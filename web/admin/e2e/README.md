# Admin Web browser E2E

Playwright tests for the critical Milestone 1 scenarios
(`docs/admin-web-plan.md` section 8), run against the real local stack. Nothing
is mocked in the browser: login is the real OIDC Authorization Code + PKCE
flow against `mock-oauth2-server`, and every screen calls the real Go API.

`pilot-admin.spec.ts` is one story, run in order:

1. A platform admin logs in and sees every section.
2. They create an organization and its first store.
3. Switching organization and store changes what the screens show.
4. They set up the loyalty program, change it and roll it back.
5. They issue a register API key. It is shown once, it authenticates against
   the store API, and it returns 401 after revocation.
6. A new employee sees the "no access" screen with their code. The admin
   grants them `store_manager` with that code, after which:
   - the manager sees only what that role allows;
   - a hidden section opened by URL shows "Недостаточно прав";
   - the API answers the manager's token with 403.
7. The audit log shows what the admin did.

Each screen is also checked with axe for WCAG 2.1 A/AA violations.

Every run uses fresh names, so runs don't collide. They do leave their
organizations behind in the dev database.

## Running

You need the same stack as for manual Admin Web development
(`web/admin/README.md`, "Local OIDC for dev"):

1. Fill in the OIDC values in `deployments/.env` and `web/admin/.env.local`.
2. Start the app with the mock issuer:
   ```sh
   docker compose -f deployments/docker-compose.yml --profile dev-oidc up -d
   ```
3. Run the tests from `web/admin`:
   ```sh
   npx playwright install chromium   # once
   npm run e2e
   ```

Playwright starts the Vite dev server itself, or reuses one that is already
running on port 5173. `global-setup.ts` checks that the API and the OIDC mock
answer, then runs `promogo bootstrap-admin` in the `promogo-app-1` container to
make sure the E2E platform admin `e2e-admin@promogo.test` exists.

| Variable | Default |
|---|---|
| `E2E_BASE_URL` | `http://localhost:5173`; if set, no dev server is started |
| `E2E_API_URL` | `http://127.0.0.1:8090` |
| `E2E_OIDC_ISSUER` | `http://localhost:8091/default` |
| `E2E_APP_CONTAINER` | `promogo-app-1` |
| `E2E_ADMIN_SUBJECT` | `e2e-admin@promogo.test` |

When a test fails, a screenshot and a trace are written to `test-results/`.
Open a trace with `npx playwright show-trace <path>/trace.zip`. The HTML
report is `npx playwright show-report`.
