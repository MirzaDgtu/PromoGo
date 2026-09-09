import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'

import { useAuth } from '../auth/AuthProvider'
import { LoadingState } from '../components/states'
import { DisabledAccountPage } from '../pages/DisabledAccountPage'
import { NoMembershipPage } from '../pages/NoMembershipPage'

// Route guard for everything under AppShell. Frontend-side gating here is
// UX only — it decides what this page renders, never what the backend
// allows; every actual authorization/tenant-scope/IDOR check happens
// server-side regardless of what this component does (see
// docs/admin-web-implementation-prompt.md).
export function RequireAuth({ children }: { children: ReactNode }) {
  const { status } = useAuth()

  switch (status) {
    case 'loading':
    case 'authenticating':
      return <LoadingState />
    case 'unauthenticated':
      return <Navigate to="/login" replace />
    case 'session_expired':
      return <Navigate to="/login?reason=session_expired" replace />
    case 'disabled':
      return <DisabledAccountPage />
    case 'no_membership':
      return <NoMembershipPage />
    case 'authenticated':
      return <>{children}</>
  }
}
