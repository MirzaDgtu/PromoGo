import { strings } from '../i18n/strings'
import { useAuth } from '../auth/AuthProvider'

export function NoMembershipPage() {
  const { logout } = useAuth()
  return (
    <main style={{ display: 'grid', placeItems: 'center', minHeight: '100vh' }}>
      <div style={{ textAlign: 'center', maxWidth: 420 }}>
        <h1>{strings.noMembership.title}</h1>
        <p>{strings.noMembership.body}</p>
        <button type="button" onClick={() => void logout()}>
          {strings.nav.logout}
        </button>
      </div>
    </main>
  )
}
