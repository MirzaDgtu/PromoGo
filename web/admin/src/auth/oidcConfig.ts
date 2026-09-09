import type { UserManagerSettings } from 'oidc-client-ts'

// The concrete OIDC provider (issuer, client id, redirect/logout URLs) is
// deployment configuration per knowledge/Decisions.md DEC-004 — this file
// only reads it from Vite env vars at build time, never hardcodes a
// provider. A pilot deployment sets these in web/admin/.env.production (or
// the hosting platform's env config); local dev uses .env.local (both
// gitignored — see .env.example for the required keys).
function requireEnv(name: string): string {
  const value = import.meta.env[name as keyof ImportMetaEnv]
  if (!value) {
    throw new Error(
      `Missing required env var ${name}. Copy web/admin/.env.example to .env.local and fill in your OIDC provider's values.`,
    )
  }
  return value
}

export function oidcSettings(): UserManagerSettings {
  return {
    authority: requireEnv('VITE_OIDC_AUTHORITY'),
    client_id: requireEnv('VITE_OIDC_CLIENT_ID'),
    redirect_uri: new URL('/auth/callback', window.location.origin).toString(),
    post_logout_redirect_uri: new URL('/login', window.location.origin).toString(),
    // Authorization Code + PKCE (oidc-client-ts always uses PKCE for the
    // code flow — see its README) — no implicit flow, per
    // docs/admin-web-implementation-prompt.md.
    response_type: 'code',
    scope: import.meta.env.VITE_OIDC_SCOPE || 'openid profile email',
    // The ID token is exchanged once, server-side, for PromoGo's own staff
    // access token (POST /api/v1/staff/auth/oidc) and then discarded from
    // the client's perspective — oidc-client-ts's own storage (sessionStorage,
    // its default) only ever holds the OIDC session used to silently renew,
    // never PromoGo's own access token, which AuthProvider keeps in memory.
    automaticSilentRenew: true,
    monitorSession: false,
  }
}
