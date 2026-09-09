import createClient, { type Middleware } from 'openapi-fetch'

import type { paths } from './schema.gen'

// The staff access token lives only in memory (see auth/AuthProvider.tsx) —
// never localStorage/sessionStorage — so it is injected into every request
// via this mutable holder rather than read back out of storage. Losing it
// on a hard refresh is intentional: it forces the controlled re-auth flow
// in AuthProvider instead of resurrecting a token from disk.
let currentAccessToken: string | null = null

export function setAccessToken(token: string | null): void {
  currentAccessToken = token
}

// on401 is invoked once per 401 response, after the request has already
// failed — AuthProvider wires this to its silent-renew-then-retry flow so
// this module doesn't need to know anything about OIDC.
let on401: (() => void) | null = null

export function setOn401Handler(handler: (() => void) | null): void {
  on401 = handler
}

const authMiddleware: Middleware = {
  async onRequest({ request }) {
    if (currentAccessToken) {
      request.headers.set('Authorization', `Bearer ${currentAccessToken}`)
    }
    return request
  },
  async onResponse({ response }) {
    if (response.status === 401) {
      on401?.()
    }
    return response
  },
}

// Same-origin pilot deployment (docs/admin-web-plan.md 5.2): no baseUrl
// override needed, "/api/v1/..." paths resolve against the page's own
// origin in both dev (via vite.config.ts's proxy) and production.
export const api = createClient<paths>({ baseUrl: '' })
api.use(authMiddleware)

// requestIdFrom reads the X-Request-Id response header so error UI can show
// a copyable id without leaking any other response detail (see
// docs/admin-web-implementation-prompt.md: "Ошибки должны сохранять и
// показывать копируемый X-Request-Id, не раскрывая внутренние details").
export function requestIdFrom(response: Response | undefined): string | null {
  return response?.headers.get('X-Request-Id') ?? null
}
