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
