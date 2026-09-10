import { useSearchParams } from 'react-router-dom'

import { strings } from '../i18n/strings'
import { useAuth } from './AuthProvider'

export function LoginPage() {
  const { login, status } = useAuth()
  const [params] = useSearchParams()
  const sessionExpired = params.get('reason') === 'session_expired'

  return (
    <main style={{ display: 'grid', placeItems: 'center', minHeight: '100vh' }}>
      <div style={{ textAlign: 'center', maxWidth: 360 }}>
        <h1>{strings.auth.loginTitle}</h1>
        {sessionExpired && (
          <p role="alert" style={{ color: 'var(--color-danger)' }}>
            {strings.auth.sessionExpiredBody}
          </p>
        )}
        {status === 'unconfigured' ? (
          <div role="alert">
            <h2>{strings.auth.unconfiguredTitle}</h2>
            <p>{strings.auth.unconfiguredBody}</p>
          </div>
        ) : (
          <button type="button" onClick={() => void login()} disabled={status === 'authenticating'}>
            {status === 'authenticating' ? strings.auth.loggingIn : strings.auth.loginButton}
          </button>
        )}
      </div>
    </main>
  )
}
