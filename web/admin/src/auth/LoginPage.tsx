import { useSearchParams } from 'react-router-dom'

import { BrandMark } from '../components/icons'
import { strings } from '../i18n/strings'
import { useAuth } from './AuthProvider'

export function LoginPage() {
  const { login, status } = useAuth()
  const [params] = useSearchParams()
  const sessionExpired = params.get('reason') === 'session_expired'

  return (
    <main className="standalone">
      <div className="standalone__card">
        <div className="brand">
          <BrandMark className="brand__mark" />
          <span className="brand__name">PromoGo</span>
          <span className="brand__product">{strings.nav.product}</span>
        </div>
        <h1>{strings.auth.loginTitle}</h1>
        {sessionExpired && (
          <p role="alert" className="field-error">
            {strings.auth.sessionExpiredBody}
          </p>
        )}
        {status === 'unconfigured' ? (
          <div role="alert" className="alert">
            <strong>{strings.auth.unconfiguredTitle}</strong>
            <p>{strings.auth.unconfiguredBody}</p>
          </div>
        ) : (
          <button
            type="button"
            className="button-primary"
            onClick={() => void login()}
            disabled={status === 'authenticating'}
          >
            {status === 'authenticating' ? strings.auth.loggingIn : strings.auth.loginButton}
          </button>
        )}
      </div>
    </main>
  )
}
