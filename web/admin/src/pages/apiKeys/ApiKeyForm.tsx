import { useId } from 'react'
import { useForm } from 'react-hook-form'

import type { APIKeyScope } from '../../api/queries'
import { SubmitError } from '../../components/SubmitError'
import { validateName } from '../../components/formValidation'
import { strings } from '../../i18n/strings'

const t = strings.apiKeys.form

export const ALL_SCOPES: APIKeyScope[] = ['transactions.write', 'clients.lookup', 'balances.read']

interface FormValues {
  name: string
  scopes: APIKeyScope[]
  expiresOn: string
}

export interface ApiKeyInput {
  name: string
  scopes: APIKeyScope[]
  expires_at?: string
}

/** YYYY-MM-DD of the local date `days` from today, for <input type="date">. */
function localDate(days: number): string {
  const d = new Date()
  d.setDate(d.getDate() + days)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// A key "valid until" a date stays valid through the end of that local day.
export function toApiKeyInput(values: FormValues): ApiKeyInput {
  const input: ApiKeyInput = { name: values.name.trim(), scopes: values.scopes }
  if (values.expiresOn) input.expires_at = new Date(`${values.expiresOn}T23:59:59`).toISOString()
  return input
}

export function ApiKeyForm({
  onSubmit,
  onCancel,
  pending,
  error,
}: {
  onSubmit: (input: ApiKeyInput) => Promise<unknown>
  onCancel: () => void
  pending: boolean
  error: unknown
}) {
  const nameID = useId()
  const nameHintID = useId()
  const expiresID = useId()
  const expiresHintID = useId()
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({ defaultValues: { name: '', scopes: [], expiresOn: '' } })

  const submit = handleSubmit(async (values) => {
    try {
      await onSubmit(toApiKeyInput(values))
    } catch {
      // Rendered from the error prop below.
    }
  })

  const tomorrow = localDate(1)

  return (
    <form
      className="api-key-form"
      aria-label={t.title}
      noValidate
      onSubmit={(e) => void submit(e)}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onCancel()
      }}
    >
      <h2>{t.title}</h2>

      <div className="field">
        <label htmlFor={nameID}>{t.name}</label>
        <input
          id={nameID}
          type="text"
          autoComplete="off"
          autoFocus
          aria-invalid={errors.name ? true : undefined}
          aria-describedby={nameHintID}
          {...register('name', { validate: validateName })}
        />
        <p id={nameHintID} className={errors.name ? 'field-error' : 'field-hint'}>
          {errors.name?.message ?? t.nameHint}
        </p>
      </div>

      <fieldset className="field" aria-invalid={errors.scopes ? true : undefined}>
        <legend>{t.scopes}</legend>
        {ALL_SCOPES.map((scope) => (
          <label key={scope} className="checkbox">
            <input
              type="checkbox"
              value={scope}
              {...register('scopes', { validate: (v) => v.length > 0 || t.scopesRequired })}
            />
            <span>
              <strong>{strings.apiKeys.scopes[scope].label}</strong>
              <span className="muted"> — {strings.apiKeys.scopes[scope].description}</span>
            </span>
          </label>
        ))}
        {errors.scopes && <p className="field-error">{errors.scopes.message}</p>}
      </fieldset>

      <div className="field">
        <label htmlFor={expiresID}>{t.expiresAt}</label>
        <input
          id={expiresID}
          type="date"
          min={tomorrow}
          aria-invalid={errors.expiresOn ? true : undefined}
          aria-describedby={expiresHintID}
          {...register('expiresOn', { validate: (v) => !v || v >= tomorrow || t.expiresAtPast })}
        />
        <p id={expiresHintID} className={errors.expiresOn ? 'field-error' : 'field-hint'}>
          {errors.expiresOn?.message ?? t.expiresAtHint}
        </p>
      </div>

      <div className="form-actions">
        <button type="submit" className="button-primary" disabled={pending}>
          {pending ? strings.form.saving : t.submit}
        </button>
        <button type="button" onClick={onCancel}>
          {strings.common.cancel}
        </button>
      </div>
      {error != null && <SubmitError error={error} />}
    </form>
  )
}
