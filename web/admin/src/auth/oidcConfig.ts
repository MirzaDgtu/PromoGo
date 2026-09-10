import type { UserManagerSettings } from 'oidc-client-ts'

// The concrete OIDC provider (issuer, client id, redirect/logout URLs) is
// deployment configuration per knowledge/Decisions.md DEC-004 — this file
// only reads it from Vite env vars at build time, never hardcodes a
// provider. A pilot deployment sets these in web/admin/.env.production (or
// the hosting platform's env config); local dev uses .env.local (both
// gitignored — see .env.example for the required keys).
//
// DEC-004 hasn't picked a provider yet, so an unconfigured environment is
// the expected default today, not a misconfiguration — isOIDCConfigured
// lets AuthProvider render a clear "not configured" screen for that case
// instead of throwing during render (a thrown error here would take down
// the whole app before a user ever sees a login button, which is worse
// than just saying so).
export function isOIDCConfigured(): boolean {
  return Boolean(import.meta.env.VITE_OIDC_AUTHORITY && import.meta.env.VITE_OIDC_CLIENT_ID)
}

// oidcSettings returns null when isOIDCConfigured() is false — callers must
// check that first rather than relying on this to throw.
export function oidcSettings(): UserManagerSettings | null {
  if (!isOIDCConfigured()) return null
  return {
    authority: import.meta.env.VITE_OIDC_AUTHORITY,
    client_id: import.meta.env.VITE_OIDC_CLIENT_ID,
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
