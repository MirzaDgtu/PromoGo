import { strings } from '../i18n/strings'
import { useAuth } from '../auth/AuthProvider'

export function DisabledAccountPage() {
  const { logout } = useAuth()
  return (
    <main style={{ display: 'grid', placeItems: 'center', minHeight: '100vh' }}>
      <div style={{ textAlign: 'center', maxWidth: 420 }}>
        <h1>{strings.disabledAccount.title}</h1>
        <p>{strings.disabledAccount.body}</p>
        <button type="button" onClick={() => void logout()}>
          {strings.nav.logout}
        </button>
      </div>
    </main>
  )
}
