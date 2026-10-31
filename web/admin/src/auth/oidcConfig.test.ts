import { afterEach, expect, it, vi } from 'vitest'

import { oidcSettings } from './oidcConfig'

afterEach(() => {
  vi.unstubAllEnvs()
})

// AuthProvider owns every renewal; the library's own one would write users to
// its store behind the session-generation checks.
it('leaves silent renewal to AuthProvider', () => {
  vi.stubEnv('VITE_OIDC_AUTHORITY', 'https://idp.example.test')
  vi.stubEnv('VITE_OIDC_CLIENT_ID', 'admin')
  expect(oidcSettings()?.automaticSilentRenew).toBe(false)
})
