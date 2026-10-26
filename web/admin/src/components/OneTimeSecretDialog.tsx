import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

import { strings } from '../i18n/strings'

const t = strings.apiKeys.created

// OneTimeSecretDialog shows a secret the server returns exactly once (a new
// store API key). It is deliberately hard to dismiss by accident: no Escape,
// no backdrop click — only "Закрыть", enabled after the user confirms they
// saved the key. Everything behind it is inert while it's open, so focus and
// clicks can't leave it. The parent must drop the secret from its own state
// in onClose; this component keeps no copy of its own.
export function OneTimeSecretDialog({ secret, onClose }: { secret: string; onClose: () => void }) {
  const titleID = useId()
  const warningID = useId()
  const inputID = useId()
  const [copied, setCopied] = useState(false)
  const [confirmed, setConfirmed] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const copyRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const root = document.getElementById('root')
    root?.setAttribute('inert', '')
    copyRef.current?.focus()
    return () => root?.removeAttribute('inert')
  }, [])

  const copy = () => {
    // Clipboard API needs a secure context (localhost or HTTPS); otherwise
    // select the text so the user can copy it by hand.
    if (!navigator.clipboard) {
      inputRef.current?.select()
      return
    }
    void navigator.clipboard.writeText(secret).then(
      () => setCopied(true),
      () => inputRef.current?.select(),
    )
  }

  return createPortal(
    <div className="dialog-backdrop">
      <div className="dialog" role="dialog" aria-modal="true" aria-labelledby={titleID} aria-describedby={warningID}>
        <h2 id={titleID}>{t.title}</h2>
        <p id={warningID} className="dialog__warning">
          {t.warning}
        </p>

        <label htmlFor={inputID}>{t.keyLabel}</label>
        <div className="secret-row">
          <input
            id={inputID}
            ref={inputRef}
            className="secret-row__value"
            type="text"
            readOnly
            value={secret}
            spellCheck={false}
            onFocus={(e) => e.currentTarget.select()}
          />
          <button ref={copyRef} type="button" className="button-primary" onClick={copy}>
            {copied ? t.copied : t.copy}
          </button>
        </div>

        <p className="muted">{t.usage}</p>
        <pre className="code-block">Authorization: Bearer &lt;API-ключ&gt;</pre>

        <label className="checkbox">
          <input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} />
          {t.confirm}
        </label>

        <div className="dialog__actions">
          <button type="button" disabled={!confirmed} onClick={onClose}>
            {t.close}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
