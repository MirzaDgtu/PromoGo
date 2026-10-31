import { QueryClient } from '@tanstack/react-query'
import { act, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api, setAccessToken } from '../api/client'
import { AuthProvider, useAuth, type AuthStatus } from './AuthProvider'

interface FakeUser {
  id_token: string
  profile: { sub: string; email: string; name: string }
  refresh_token?: string
}

const person = (name: string): FakeUser => ({
  id_token: `id-${name}`,
  profile: { sub: name, email: `${name}@example.test`, name },
})

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

const oidc = vi.hoisted(() => ({
  getUser: vi.fn(),
  signinCallback: vi.fn(),
  signinRedirectCallback: vi.fn(),
  signinRedirect: vi.fn(),
  signinSilent: vi.fn(),
  signoutRedirect: vi.fn(),
  removeUser: vi.fn(),
  storeUser: vi.fn(),
  events: { addAccessTokenExpiring: vi.fn((_: () => void) => () => undefined) },
}))

// openapi-fetch captures fetch and Request when api/client.ts is imported, and
// the same-origin baseUrl needs a Request that accepts relative URLs, so both
// are replaced before that import runs.
const net = vi.hoisted(() => {
  const state: { handler: (request: Request) => Promise<Response> } = {
    handler: () => Promise.reject(new Error('no handler')),
  }
  const RealRequest = globalThis.Request
  globalThis.Request = class extends RealRequest {
    constructor(input: RequestInfo | URL, init?: RequestInit) {
      super(typeof input === 'string' ? new URL(input, 'http://localhost').toString() : input, init)
    }
  }
  globalThis.fetch = ((request: Request) => state.handler(request)) as typeof fetch
  return state
})

vi.mock('oidc-client-ts', () => ({
  User: class {},
  UserManager: class {
    constructor() {
      return oidc
    }
  },
}))
vi.mock('./oidcConfig', () => ({
  CALLBACK_PATH: '/auth/callback',
  oidcSettings: () => ({ authority: 'https://idp.example.test', client_id: 'admin', redirect_uri: 'x' }),
}))

// Every API request the app makes, with the bearer token it actually carried,
// plus a per-URL response queue so a test can hold an exchange open.
let sent: { url: string; bearer: string | null }[] = []
let exchange: (idToken: string) => Promise<Response>
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

let auth: ReturnType<typeof useAuth>
function Probe() {
  auth = useAuth()
  const status: AuthStatus = auth.status
  return (
    <p>
      {status}:{auth.identity?.name ?? ''}
    </p>
  )
}

const renderProvider = async (signedIn: FakeUser | null) => {
  oidc.getUser.mockResolvedValue(signedIn)
  const queryClient = new QueryClient()
  render(
    <AuthProvider queryClient={queryClient}>
      <Probe />
    </AuthProvider>,
  )
  await screen.findByText(signedIn ? `authenticated:${signedIn.profile.name}` : 'unauthenticated:')
  return queryClient
}

const trigger401 = async () => {
  await api.GET('/api/v1/staff/me', { headers: { 'X-Test-401': '1' } })
}

const bearerOfLastCall = async () => {
  sent = []
  await api.GET('/api/v1/staff/me')
  return sent[0]?.bearer ?? null
}

beforeEach(() => {
  vi.clearAllMocks()
  oidc.signinSilent.mockReset()
  setAccessToken(null)
  sent = []
  window.history.replaceState(null, '', '/')
  oidc.signoutRedirect.mockResolvedValue(undefined)
  oidc.storeUser.mockResolvedValue(undefined)
  oidc.removeUser.mockResolvedValue(undefined)
  exchange = (idToken) => Promise.resolve(json({ access_token: idToken.replace('id-', 'staff-') }))
  net.handler = async (request) => {
    sent.push({ url: request.url, bearer: request.headers.get('Authorization') })
    if (request.url.endsWith('/staff/auth/oidc')) {
      const body = (await request.clone().json()) as { id_token: string }
      return exchange(body.id_token)
    }
    if (request.headers.get('X-Test-401')) return json({ error: 'expired' }, 401)
    return json({ staff_user_id: 1, memberships: [{ organization_id: 1 }] })
  }
})

describe('AuthProvider session generations', () => {
  it('keeps user B token and stored OIDC user when a renewal started by A finishes late', async () => {
    const renewal = deferred<FakeUser>()
    oidc.signinSilent.mockReturnValueOnce(renewal.promise)
    await renderProvider(person('a'))

    await trigger401()
    expect(oidc.signinSilent).toHaveBeenCalledTimes(1)

    await act(async () => {
      await auth.logout()
    })
    oidc.signinCallback.mockResolvedValue(person('b'))
    await act(async () => {
      await auth.completeLogin()
    })
    await screen.findByText('authenticated:b')
    expect(await bearerOfLastCall()).toBe('Bearer staff-b')

    await act(async () => {
      renewal.resolve(person('a'))
      await renewal.promise
    })

    await waitFor(() => expect(oidc.storeUser).toHaveBeenCalled())
    expect(await bearerOfLastCall()).toBe('Bearer staff-b')
    expect(screen.getByText('authenticated:b')).toBeInTheDocument()
    // The library wrote A into its user store; a reload must not revive A.
    expect(oidc.storeUser).toHaveBeenLastCalledWith(expect.objectContaining({ id_token: 'id-b' }))
  })

  it('does not restore a token or stored user after logout', async () => {
    const renewal = deferred<FakeUser>()
    oidc.signinSilent.mockReturnValueOnce(renewal.promise)
    await renderProvider(person('a'))
    await trigger401()

    await act(async () => {
      await auth.logout()
    })
    await act(async () => {
      renewal.resolve(person('a'))
      await renewal.promise
    })

    await waitFor(() => expect(oidc.storeUser).toHaveBeenLastCalledWith(null))
    expect(await bearerOfLastCall()).toBeNull()
    expect(screen.getByText('unauthenticated:')).toBeInTheDocument()
  })

  it('ignores a late renewal failure from the previous session', async () => {
    const renewal = deferred<FakeUser>()
    oidc.signinSilent.mockReturnValueOnce(renewal.promise)
    await renderProvider(person('a'))
    await trigger401()

    await act(async () => {
      await auth.logout()
    })
    oidc.signinCallback.mockResolvedValue(person('b'))
    await act(async () => {
      await auth.completeLogin()
    })
    await screen.findByText('authenticated:b')

    await act(async () => {
      renewal.reject(new Error('timeout'))
      await renewal.promise.catch(() => undefined)
    })

    expect(screen.getByText('authenticated:b')).toBeInTheDocument()
    expect(await bearerOfLastCall()).toBe('Bearer staff-b')
  })

  it('drops an ID-token exchange that resolves after logout', async () => {
    const pending = deferred<Response>()
    await renderProvider(person('a'))
    const renewal = person('a')
    oidc.signinSilent.mockResolvedValueOnce(renewal)
    exchange = () => pending.promise
    await trigger401()
    await waitFor(() => expect(sent.some((c) => c.url.endsWith('/staff/auth/oidc'))).toBe(true))

    await act(async () => {
      await auth.logout()
    })
    await act(async () => {
      pending.resolve(json({ access_token: 'staff-a' }))
      await pending.promise
    })

    expect(await bearerOfLastCall()).toBeNull()
    expect(screen.getByText('unauthenticated:')).toBeInTheDocument()
  })

  it('lets the new session start its own renewal while an old one is still pending', async () => {
    const stale = deferred<FakeUser>()
    oidc.signinSilent.mockReturnValueOnce(stale.promise)
    await renderProvider(person('a'))
    await trigger401()

    await act(async () => {
      await auth.logout()
    })
    oidc.signinCallback.mockResolvedValue(person('b'))
    await act(async () => {
      await auth.completeLogin()
    })
    await screen.findByText('authenticated:b')

    oidc.signinSilent.mockResolvedValueOnce(person('b'))
    await trigger401()
    expect(oidc.signinSilent).toHaveBeenCalledTimes(2)
  })

  it('does not start a renewal for a 401 belonging to a session that already ended', async () => {
    const response = deferred<Response>()
    await renderProvider(person('a'))
    const reached = deferred<void>()
    net.handler = () => {
      reached.resolve()
      return response.promise
    }
    const inFlight = api.GET('/api/v1/staff/me')
    await reached.promise

    await act(async () => {
      await auth.logout()
    })
    await act(async () => {
      response.resolve(json({ error: 'expired' }, 401))
      await inFlight
    })

    expect(oidc.signinSilent).not.toHaveBeenCalled()
    expect(screen.getByText('unauthenticated:')).toBeInTheDocument()
  })
})

describe('AuthProvider renewal on access-token expiry', () => {
  const tokenExpiring = async () => {
    const handler = oidc.events.addAccessTokenExpiring.mock.lastCall?.[0]
    if (!handler) throw new Error('no access-token-expiring handler registered')
    await act(async () => {
      handler()
      await Promise.resolve()
    })
  }
  const rotated = (refreshToken: string) => ({ ...person('b'), refresh_token: refreshToken })
  const signInB = async () => {
    await act(async () => {
      await auth.logout()
    })
    oidc.signinCallback.mockResolvedValue(rotated('r1'))
    await act(async () => {
      await auth.completeLogin()
    })
    await screen.findByText('authenticated:b')
  }

  it('keeps the rotated refresh token of B when a renewal started by A finishes late', async () => {
    const renewalA = deferred<FakeUser>()
    oidc.signinSilent.mockReturnValueOnce(renewalA.promise)
    await renderProvider(person('a'))
    await trigger401()
    await signInB()

    oidc.signinSilent.mockResolvedValueOnce(rotated('r2'))
    await tokenExpiring()
    await waitFor(() => expect(oidc.signinSilent).toHaveBeenCalledTimes(2))

    await act(async () => {
      renewalA.resolve(person('a'))
      await renewalA.promise
    })

    await waitFor(() => expect(oidc.storeUser).toHaveBeenLastCalledWith(expect.objectContaining({ refresh_token: 'r2' })))
    expect(screen.getByText('authenticated:b')).toBeInTheDocument()
  })

  it('keeps the rotated refresh token when A finishes during the exchange of B', async () => {
    const renewalA = deferred<FakeUser>()
    oidc.signinSilent.mockReturnValueOnce(renewalA.promise)
    await renderProvider(person('a'))
    await trigger401()
    await signInB()

    const pending = deferred<Response>()
    exchange = () => pending.promise
    oidc.signinSilent.mockResolvedValueOnce(rotated('r2'))
    await tokenExpiring()
    await waitFor(() => expect(sent.some((c) => c.url.endsWith('/staff/auth/oidc'))).toBe(true))

    await act(async () => {
      renewalA.resolve(person('a'))
      await renewalA.promise
    })
    await waitFor(() => expect(oidc.storeUser).toHaveBeenLastCalledWith(expect.objectContaining({ refresh_token: 'r2' })))
    await act(async () => {
      pending.resolve(json({ access_token: 'staff-b2' }))
      await pending.promise
    })

    await waitFor(async () => expect(await bearerOfLastCall()).toBe('Bearer staff-b2'))
  })

  it('renews without refetching what is on screen', async () => {
    const queryClient = await renderProvider(person('a'))
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    exchange = () => Promise.resolve(json({ access_token: 'staff-a2' }))
    oidc.signinSilent.mockResolvedValueOnce(person('a'))

    await tokenExpiring()

    await waitFor(async () => expect(await bearerOfLastCall()).toBe('Bearer staff-a2'))
    expect(invalidate).not.toHaveBeenCalled()
  })

  it('keeps the session when an early renewal fails, then expires it on the real 401', async () => {
    await renderProvider(person('a'))
    oidc.signinSilent.mockRejectedValueOnce(new Error('invalid_grant'))
    await tokenExpiring()
    await waitFor(() => expect(oidc.signinSilent).toHaveBeenCalledTimes(1))
    await act(async () => {
      await Promise.resolve()
    })
    expect(screen.getByText('authenticated:a')).toBeInTheDocument()
    expect(await bearerOfLastCall()).toBe('Bearer staff-a')

    oidc.signinSilent.mockRejectedValueOnce(new Error('invalid_grant'))
    await trigger401()
    await screen.findByText('session_expired:a')
  })

  it('expires the session when a 401 joined an early renewal that then fails', async () => {
    const renewal = deferred<FakeUser>()
    oidc.signinSilent.mockReturnValueOnce(renewal.promise)
    await renderProvider(person('a'))
    await tokenExpiring()
    await trigger401()
    expect(oidc.signinSilent).toHaveBeenCalledTimes(1)

    await act(async () => {
      renewal.reject(new Error('invalid_grant'))
      await renewal.promise.catch(() => undefined)
    })
    await screen.findByText('session_expired:a')
  })

  it('refetches when a 401 joined an early renewal that succeeds', async () => {
    const renewal = deferred<FakeUser>()
    oidc.signinSilent.mockReturnValueOnce(renewal.promise)
    const queryClient = await renderProvider(person('a'))
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    await tokenExpiring()
    await trigger401()

    await act(async () => {
      renewal.resolve(person('a'))
      await renewal.promise
    })
    await waitFor(() => expect(invalidate).toHaveBeenCalled())
  })

  it('keeps the session and its token when the exchange of an early renewal gets a 401', async () => {
    await renderProvider(person('a'))
    exchange = () => Promise.resolve(json({ error: 'invalid id token' }, 401))
    oidc.signinSilent.mockResolvedValueOnce(person('a'))

    await tokenExpiring()
    await waitFor(() => expect(sent.some((c) => c.url.endsWith('/staff/auth/oidc') && c.bearer === 'Bearer staff-a')).toBe(true))
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 20))
    })

    expect(oidc.signinSilent).toHaveBeenCalledTimes(1)
    expect(screen.getByText('authenticated:a')).toBeInTheDocument()
    expect(await bearerOfLastCall()).toBe('Bearer staff-a')
  })

  it('still expires the session when a rejected staff token is what started the renewal', async () => {
    await renderProvider(person('a'))
    exchange = () => Promise.resolve(json({ error: 'invalid id token' }, 401))
    oidc.signinSilent.mockResolvedValueOnce(person('a'))

    await trigger401()

    await screen.findByText('session_expired:a')
  })

  it('does not renew when the exchange of a fresh login gets a 401', async () => {
    window.history.replaceState(null, '', '/auth/callback?code=c&state=s')
    exchange = () => Promise.resolve(json({ error: 'invalid id token' }, 401))
    oidc.signinCallback.mockResolvedValue(person('a'))
    render(
      <AuthProvider queryClient={new QueryClient()}>
        <Probe />
      </AuthProvider>,
    )

    await act(async () => {
      await auth.completeLogin()
    })

    await screen.findByText('unauthenticated:a')
    expect(oidc.signinSilent).not.toHaveBeenCalled()
  })

  it('does not renew once the session has expired or ended', async () => {
    oidc.signinSilent.mockRejectedValueOnce(new Error('login_required'))
    await renderProvider(person('a'))
    await trigger401()
    await screen.findByText('session_expired:a')

    await tokenExpiring()
    expect(oidc.signinSilent).toHaveBeenCalledTimes(1)
    expect(await bearerOfLastCall()).toBeNull()
  })
})

describe('AuthProvider OIDC callback', () => {
  it('leaves the iframe branch to the parent window: no exchange, no status change', async () => {
    window.history.replaceState(null, '', '/auth/callback?code=c&state=s')
    oidc.signinCallback.mockResolvedValue(undefined)
    render(
      <AuthProvider queryClient={new QueryClient()}>
        <Probe />
      </AuthProvider>,
    )

    await act(async () => {
      await auth.completeLogin()
    })

    expect(oidc.signinCallback).toHaveBeenCalledTimes(1)
    expect(oidc.signinRedirectCallback).not.toHaveBeenCalled()
    expect(sent).toEqual([])
    expect(screen.getByText('authenticating:')).toBeInTheDocument()
  })

  it('completes a normal redirect login through signinCallback', async () => {
    window.history.replaceState(null, '', '/auth/callback?code=c&state=s')
    oidc.signinCallback.mockResolvedValue(person('a'))
    render(
      <AuthProvider queryClient={new QueryClient()}>
        <Probe />
      </AuthProvider>,
    )

    await act(async () => {
      await auth.completeLogin()
    })

    await screen.findByText('authenticated:a')
    expect(await bearerOfLastCall()).toBe('Bearer staff-a')
  })

  it('reports an expired session when the silent renewal is rejected', async () => {
    oidc.signinSilent.mockRejectedValueOnce(new Error('login_required'))
    await renderProvider(person('a'))
    await trigger401()
    await screen.findByText('session_expired:a')
    expect(await bearerOfLastCall()).toBeNull()
  })
})
