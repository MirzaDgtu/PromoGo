import { useState } from 'react'

import { strings } from '../i18n/strings'
import { useAuth } from '../auth/AuthProvider'

const t = strings.noMembership

// NoMembershipPage is where a new employee lands before anyone has given
// them access. It shows their access code (the OIDC subject) to hand to an
// administrator, who pastes it into "Сотрудники и доступ" — the pilot's
// enrollment flow (DEC-029).
export function NoMembershipPage() {
  const { logout, identity } = useAuth()
  const [copied, setCopied] = useState(false)

  return (
    <main className="standalone">
      <div className="standalone__card standalone__card--wide no-access__card">
        <h1>{t.title}</h1>
        <p>{t.body}</p>

        {identity && (
          <>
            {(identity.name || identity.email) && (
              <p className="muted">{t.signedInAs(identity.name || identity.email)}</p>
            )}
            <ol className="no-access__steps">
              <li>{t.step1}</li>
              <li>{t.step2}</li>
              <li>{t.step3}</li>
            </ol>
            <label htmlFor="access-code">{t.codeLabel}</label>
            <div className="secret-row">
              <input id="access-code" className="secret-row__value" type="text" readOnly value={identity.subject} onFocus={(e) => e.target.select()} />
              <button
                type="button"
                onClick={() => {
                  void navigator.clipboard.writeText(identity.subject).then(() => setCopied(true))
                }}
              >
                {copied ? strings.common.copied : t.copy}
              </button>
            </div>
          </>
        )}

        <p className="standalone__actions">
          <button type="button" onClick={() => window.location.reload()}>
            {t.retry}
          </button>
          <button type="button" onClick={() => void logout()}>
            {strings.nav.logout}
          </button>
        </p>
      </div>
    </main>
  )
}
