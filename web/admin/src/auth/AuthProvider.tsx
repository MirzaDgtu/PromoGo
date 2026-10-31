import { QueryClient } from '@tanstack/react-query'
import { User, UserManager } from 'oidc-client-ts'
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'

import { api, setAccessToken, setOn401Handler } from '../api/client'
import { CALLBACK_PATH, oidcSettings } from './oidcConfig'

export type AuthStatus =
  | 'loading'
  | 'unconfigured'
  | 'unauthenticated'
  | 'authenticating'
  | 'authenticated'
  | 'no_membership'
  | 'disabled'
  | 'session_expired'

interface AuthContextValue {
  status: AuthStatus
  /** Only set once status === 'authenticated'. */
  staffUserID: number | null
  /**
   * The signed-in person as the OIDC provider identifies them, set once a
   * login completes (whatever the outcome). subject is the access code a
   * new employee hands to an administrator from the "no access" screen —
   * the same claim the backend stores as external_subject.
   */
  identity: { subject: string; email: string; name: string } | null
  login: () => Promise<void>
  logout: () => Promise<void>
  /** Consumes the OIDC redirect callback at /auth/callback. */
  completeLogin: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}

// exchangeIDToken trades a verified OIDC ID token for PromoGo's own
// short-lived staff access token (POST /api/v1/staff/auth/oidc) — see
// docs/admin-web-plan.md 5.3. Distinguishes the two 403 causes the backend
// documents (disabled account vs. no active membership) so the UI can show
// the right empty state instead of a generic error.
async function exchangeIDToken(
  idToken: string,
): Promise<{ accessToken: string } | { forbidden: 'disabled' | 'no_membership' } | { error: string }> {
  const { data, error, response } = await api.POST('/api/v1/staff/auth/oidc', {
    body: { id_token: idToken },
  })
  if (data) return { accessToken: data.access_token ?? '' }
  if (response.status === 403) {
    // handleStaffOIDCLogin (internal/httpserver/auth_staff.go) returns the
    // literal message "account disabled" for the disabled case and a
    // different message for "no active membership" — matched on exact text
    // since there's no machine-readable reason code yet (see
    // knowledge/Project Questions.md for that as an open item).
    const message = error?.error ?? ''
    return { forbidden: message === 'account disabled' ? 'disabled' : 'no_membership' }
  }
  return { error: response.status ? `HTTP ${response.status}` : 'network error' }
}

export function AuthProvider({
  children,
  queryClient,
}: {
  children: ReactNode
  queryClient: QueryClient
}) {
  const [status, setStatus] = useState<AuthStatus>('loading')
  const [staffUserID, setStaffUserID] = useState<number | null>(null)
  const [identity, setIdentity] = useState<AuthContextValue['identity']>(null)
  // null when DEC-004 hasn't picked an OIDC provider for this deployment
  // yet (see oidcConfig.ts) — every method below no-ops or reports
  // 'unconfigured' rather than throwing in that case.
  const userManager = useMemo(() => {
    const settings = oidcSettings()
    return settings ? new UserManager(settings) : null
  }, [])
  // Coalesces concurrent 401s from multiple in-flight requests, and the
  // token-expiry event, into a single silent-renew attempt, so a page that
  // fires several queries at once doesn't open several renewal flows — see
  // the module doc on controlled re-auth (no infinite retry loops).
  const renewalInFlight = useRef<Promise<void> | null>(null)
  // Whether a 401 is waiting on the attempt in flight. An attempt started
  // ahead of expiry stays invisible while the staff token still works; a 401
  // means requests are already failing, so the outcome has to reach the user.
  const renewalAwaited = useRef(false)
  // True while this session holds a staff token, so the expiry event can't
  // bring back a session that has already expired or ended.
  const sessionLive = useRef(false)
  // Bumped whenever one session ends or another begins. Every async auth
  // operation records the generation it started in and drops its result once
  // that has changed, so a late response from a previous person can never
  // restore their token, status or cache under the next person's identity.
  const sessionGeneration = useRef(0)
  // The OIDC user of the current session. oidc-client-ts stores a silent
  // renewal's user itself, before a stale result can be dropped, so this is
  // what gets put back — otherwise a reload would revive the previous person.
  const sessionUser = useRef<User | null>(null)

  const startGeneration = useCallback((): number => {
    renewalInFlight.current = null
    renewalAwaited.current = false
    sessionLive.current = false
    sessionGeneration.current += 1
    return sessionGeneration.current
  }, [])

  const finishLogin = useCallback(
    async (user: User, generation: number) => {
      const idToken = user.id_token
      if (!idToken) {
        setStatus('unauthenticated')
        return
      }
      sessionUser.current = user
      setIdentity({ subject: user.profile.sub, email: user.profile.email ?? '', name: user.profile.name ?? '' })
      const result = await exchangeIDToken(idToken)
      if (generation !== sessionGeneration.current) return
      if ('accessToken' in result) {
        setAccessToken(result.accessToken)
        sessionLive.current = true
        const me = await api.GET('/api/v1/staff/me')
        if (generation !== sessionGeneration.current) return
        if (me.data) {
          setStaffUserID(me.data.staff_user_id ?? null)
          setStatus(me.data.memberships && me.data.memberships.length > 0 ? 'authenticated' : 'no_membership')
        } else {
          setStatus('unauthenticated')
        }
      } else if ('forbidden' in result) {
        setStatus(result.forbidden === 'disabled' ? 'disabled' : 'no_membership')
      } else {
        setStatus('unauthenticated')
      }
    },
    [],
  )

  const attemptSilentRenewal = useCallback(
    (cause: 'unauthorized' | 'expiring'): Promise<void> => {
      if (!userManager) return Promise.resolve()
      if (cause === 'unauthorized') renewalAwaited.current = true
      if (renewalInFlight.current) return renewalInFlight.current
      const generation = sessionGeneration.current
      const attempt = (async () => {
        try {
          const user = await userManager.signinSilent()
          if (generation !== sessionGeneration.current) {
            await userManager.storeUser(sessionUser.current)
            return
          }
          if (user) {
            // Recorded before the exchange: a rotating IdP has already spent
            // the previous refresh token. Stored again because a stale attempt
            // may have put the previous user back just before this resolved.
            sessionUser.current = user
            await userManager.storeUser(user)
          }
          if (user?.id_token) {
            const result = await exchangeIDToken(user.id_token)
            if (generation !== sessionGeneration.current) {
              await userManager.storeUser(sessionUser.current)
              return
            }
            if ('accessToken' in result) {
              setAccessToken(result.accessToken)
              if (renewalAwaited.current) await queryClient.invalidateQueries()
              return
            }
          }
          throw new Error('silent renewal did not yield a usable session')
        } catch {
          if (generation !== sessionGeneration.current || !renewalAwaited.current) return
          sessionLive.current = false
          setAccessToken(null)
          setStatus('session_expired')
        } finally {
          // A newer session may already have its own renewal in flight.
          if (generation === sessionGeneration.current) {
            renewalInFlight.current = null
            renewalAwaited.current = false
          }
        }
      })()
      renewalInFlight.current = attempt
      return attempt
    },
    [userManager, queryClient],
  )

  useEffect(() => {
    setOn401Handler(() => {
      void attemptSilentRenewal('unauthorized')
    })
    return () => setOn401Handler(null)
  }, [attemptSilentRenewal])

  useEffect(() => {
    return userManager?.events.addAccessTokenExpiring(() => {
      if (sessionLive.current) void attemptSilentRenewal('expiring')
    })
  }, [userManager, attemptSilentRenewal])

  useEffect(() => {
    if (!userManager) {
      setStatus('unconfigured')
      return
    }
    // On first load (not the /auth/callback route), check whether
    // oidc-client-ts already has a live session (e.g. a page refresh) — if
    // so, re-derive the PromoGo staff session from it rather than forcing a
    // fresh login every reload. On the callback route there is no stored
    // user yet; answering "unauthenticated" here would race completeLogin
    // and bounce a successful login back to /login.
    if (window.location.pathname === CALLBACK_PATH) return
    let cancelled = false
    const generation = startGeneration()
    userManager
      .getUser()
      .then(async (user) => {
        if (cancelled || generation !== sessionGeneration.current) return
        if (user && !user.expired) {
          await finishLogin(user, generation)
        } else {
          setStatus('unauthenticated')
        }
      })
      .catch(() => {
        if (!cancelled && generation === sessionGeneration.current) setStatus('unauthenticated')
      })
    return () => {
      cancelled = true
    }
    // Runs once on mount; finishLogin/startGeneration/userManager are stable for the
    // provider's lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const login = useCallback(async () => {
    if (!userManager) return
    startGeneration()
    setStatus('authenticating')
    await userManager.signinRedirect()
  }, [userManager, startGeneration])

  const completeLogin = useCallback(async () => {
    if (!userManager) {
      setStatus('unconfigured')
      return
    }
    const generation = startGeneration()
    setStatus('authenticating')
    try {
      // signinCallback dispatches on the request's type. Silent renewal loads
      // this same route in a hidden iframe: there it only notifies the parent
      // window, which exchanges the code itself, and resolves with no user.
      const user = await userManager.signinCallback()
      if (!user || generation !== sessionGeneration.current) return
      await finishLogin(user, generation)
    } catch {
      if (generation === sessionGeneration.current) setStatus('unauthenticated')
    }
  }, [userManager, finishLogin, startGeneration])

  const logout = useCallback(async () => {
    startGeneration()
    sessionUser.current = null
    setAccessToken(null)
    setStaffUserID(null)
    setIdentity(null)
    setStatus('unauthenticated')
    queryClient.clear()
    await userManager?.signoutRedirect().catch(() => {
      // No end-session endpoint configured, or the IdP rejected it — the
      // local session is already cleared above, which is the part that
      // actually matters for this app; losing the IdP-side logout redirect
      // is a degraded-but-safe outcome, not one worth surfacing an error for.
    })
  }, [userManager, queryClient, startGeneration])

  const value = useMemo<AuthContextValue>(
    () => ({ status, staffUserID, identity, login, logout, completeLogin }),
    [status, staffUserID, identity, login, logout, completeLogin],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
