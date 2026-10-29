import { useEffect, useRef } from 'react'
import { Navigate } from 'react-router-dom'

import { LoadingState } from '../components/states'
import { useAuth } from './AuthProvider'

// Handles the OIDC Authorization Code + PKCE redirect back from the IdP at
// /auth/callback. Runs completeLogin exactly once — React StrictMode's
// double-invoke of effects would otherwise send the single-use
// authorization code to the token endpoint twice, and the second exchange
// always fails.
export function CallbackPage() {
  const { completeLogin, status } = useAuth()
  const started = useRef(false)

  useEffect(() => {
    if (started.current) return
    started.current = true
    void completeLogin()
  }, [completeLogin])

  if (status === 'authenticated' || status === 'no_membership' || status === 'disabled') {
    return <Navigate to="/" replace />
  }
  if (status === 'unauthenticated') {
    return <Navigate to="/login" replace />
  }

  return (
    <main className="standalone">
      <LoadingState />
    </main>
  )
}
