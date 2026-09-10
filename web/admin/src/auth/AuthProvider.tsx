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
import { oidcSettings } from './oidcConfig'

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
  // null when DEC-004 hasn't picked an OIDC provider for this deployment
  // yet (see oidcConfig.ts) — every method below no-ops or reports
  // 'unconfigured' rather than throwing in that case.
  const userManager = useMemo(() => {
    const settings = oidcSettings()
    return settings ? new UserManager(settings) : null
  }, [])
  // Coalesces concurrent 401s from multiple in-flight requests into a
  // single silent-renew attempt, so a page that fires several queries at
  // once doesn't open several renewal flows — see the module doc on
  // controlled re-auth (no infinite retry loops).
  const renewalInFlight = useRef<Promise<void> | null>(null)

  const finishLogin = useCallback(
    async (user: User) => {
      const idToken = user.id_token
      if (!idToken) {
        setStatus('unauthenticated')
        return
      }
      const result = await exchangeIDToken(idToken)
      if ('accessToken' in result) {
        setAccessToken(result.accessToken)
        const me = await api.GET('/api/v1/staff/me')
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

  const attemptSilentRenewal = useCallback((): Promise<void> => {
    if (!userManager) return Promise.resolve()
    if (renewalInFlight.current) return renewalInFlight.current
    const attempt = (async () => {
      try {
        const user = await userManager.signinSilent()
        if (user?.id_token) {
          const result = await exchangeIDToken(user.id_token)
          if ('accessToken' in result) {
            setAccessToken(result.accessToken)
            await queryClient.invalidateQueries()
            return
          }
        }
        throw new Error('silent renewal did not yield a usable session')
      } catch {
        setAccessToken(null)
        setStatus('session_expired')
      } finally {
        renewalInFlight.current = null
      }
    })()
    renewalInFlight.current = attempt
    return attempt
  }, [userManager, queryClient])

  useEffect(() => {
    setOn401Handler(() => {
      void attemptSilentRenewal()
    })
    return () => setOn401Handler(null)
  }, [attemptSilentRenewal])

  useEffect(() => {
    if (!userManager) {
      setStatus('unconfigured')
      return
    }
    // On first load (not the /auth/callback route), check whether
    // oidc-client-ts already has a live session (e.g. a page refresh) — if
    // so, re-derive the PromoGo staff session from it rather than forcing a
    // fresh login every reload.
    let cancelled = false
    userManager
      .getUser()
      .then(async (user) => {
        if (cancelled) return
        if (user && !user.expired) {
          await finishLogin(user)
        } else {
          setStatus('unauthenticated')
        }
      })
      .catch(() => {
        if (!cancelled) setStatus('unauthenticated')
      })
    return () => {
      cancelled = true
    }
    // Runs once on mount; finishLogin/userManager are stable for the
    // provider's lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const login = useCallback(async () => {
    if (!userManager) return
    setStatus('authenticating')
    await userManager.signinRedirect()
  }, [userManager])

  const completeLogin = useCallback(async () => {
    if (!userManager) {
      setStatus('unconfigured')
      return
    }
    setStatus('authenticating')
    try {
      const user = await userManager.signinRedirectCallback()
      await finishLogin(user)
    } catch {
      setStatus('unauthenticated')
    }
  }, [userManager, finishLogin])

  const logout = useCallback(async () => {
    setAccessToken(null)
    setStaffUserID(null)
    setStatus('unauthenticated')
    queryClient.clear()
    await userManager?.signoutRedirect().catch(() => {
      // No end-session endpoint configured, or the IdP rejected it — the
      // local session is already cleared above, which is the part that
      // actually matters for this app; losing the IdP-side logout redirect
      // is a degraded-but-safe outcome, not one worth surfacing an error for.
    })
  }, [userManager, queryClient])

  const value = useMemo<AuthContextValue>(
    () => ({ status, staffUserID, login, logout, completeLogin }),
    [status, staffUserID, login, logout, completeLogin],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
