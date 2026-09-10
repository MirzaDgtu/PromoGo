# PromoGo Admin Web

React + TypeScript (strict) + Vite frontend for staff/platform-admin
operations. See `../../docs/admin-web-plan.md` and
`../../docs/admin-web-implementation-prompt.md` for the product/architecture
plan this implements.

Status: Milestone 1 (foundation) — OIDC login shell, org/store context,
permission-aware nav skeleton. Feature screens (Milestone 2) are not built
yet; every nav destination renders a placeholder.

## Setup

```sh
npm ci
cp .env.example .env.local   # fill in your OIDC provider's values
npm run dev
```

The dev server proxies `/api/*` to the Go backend on `:8080` (see
`vite.config.ts`) — run `make dev-stack` (or however this repo's backend is
started) alongside `npm run dev`.

### Local OIDC for dev

DEC-004 (which real OIDC provider) is still open, so there's no production
IdP to log in against yet. `make oidc-mock-up` starts
[`mock-oauth2-server`](https://github.com/navikt/mock-oauth2-server) (opt-in
`dev-oidc` compose profile, never started by `docker-up`/`redeploy`) — it
shows a real login form (type any username, optionally paste claims JSON)
and issues a real signed ID token, so the whole Authorization Code + PKCE
flow, `POST /api/v1/staff/auth/oidc`, and `GET /api/v1/staff/me` all run for
real, not mocked out.

```sh
make oidc-mock-up
```

Then in `web/admin/.env.local`:

```
VITE_OIDC_AUTHORITY=http://localhost:8091/default
VITE_OIDC_CLIENT_ID=promogo-admin-web
```

And in `deployments/.env` (copy from `deployments/.env.example` — **not**
the repo-root `.env`, see that file's comment on why):

```
PROMOGO_OIDC_ISSUER_URL=http://localhost:8091/default
PROMOGO_OIDC_AUDIENCE=promogo-admin-web
PROMOGO_OIDC_JWKS_URL=http://oidc-mock:8080/default/jwks
```

Then `make redeploy` (or recreate the `app` container) so it picks up the
new env, and give your test login a membership:

```sh
docker exec promogo-app-1 promogo bootstrap-admin \
  --subject "you@example.com" --email "you@example.com" --name "Dev"
```

Log in with `username=you@example.com` on the mock's login form — the
`sub` claim must match `--subject` above. Verified end-to-end (login →
`/staff/me` returning the bootstrapped `platform_admin` membership with its
resolved permissions) while building this feature.

## Scripts

- `npm run dev` — Vite dev server.
- `npm run build` — strict typecheck (`tsc -b`) + production build.
- `npm run typecheck` — typecheck only.
- `npm run lint` — oxlint.
- `npm run test` — vitest (unit/component tests).
- `npm run openapi:generate` — regenerate `src/api/schema.gen.ts` from
  `../../docs/openapi/openapi.yaml`.
- `npm run openapi:check` — regenerate and fail if the checked-in generated
  file would change (drift check for CI).

## Architecture notes

- `src/api/schema.gen.ts` is generated, never hand-edited — see
  `npm run openapi:generate`. `src/api/client.ts` wraps it in a typed
  `openapi-fetch` client; no parallel hand-written DTOs.
- The PromoGo staff access token lives only in memory
  (`src/api/client.ts`'s `setAccessToken`), never `localStorage`/
  `sessionStorage`. `src/auth/AuthProvider.tsx` runs the OIDC Authorization
  Code + PKCE flow via `oidc-client-ts`, exchanges the ID token for the
  staff token (`POST /api/v1/staff/auth/oidc`), and does one bounded
  silent-renew attempt on a 401 (never an infinite retry loop) before
  forcing the user back to `/login`.
- `src/session/OrgStoreContext.tsx` derives the selected organization/store
  and permission checks from `GET /api/v1/staff/me` +
  `GET /api/v1/admin/organizations` + `.../stores` — the selection itself is
  a per-viewer UX convenience in `sessionStorage`, not a security boundary;
  every real authorization check happens server-side.
- Same-origin deployment (`docs/admin-web-plan.md` 5.2): the built SPA is
  served at `/`, the backend at `/api/*`, no CORS configuration needed.
