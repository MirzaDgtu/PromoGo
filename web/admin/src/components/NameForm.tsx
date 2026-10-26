import { useId } from 'react'
import { useForm } from 'react-hook-form'

import { strings } from '../i18n/strings'
import { validateName } from './formValidation'
import { SubmitError } from './SubmitError'

interface NameFormProps {
  label: string
  submitLabel: string
  /** Receives the trimmed name; reject to keep the input for a retry. */
  onSubmit: (name: string) => Promise<unknown>
  pending: boolean
  /** The last submit's error, rendered under the form. */
  error: unknown
  /** Shows a cancel button; the form is then expected to close on cancel. */
  onCancel?: () => void
}

export function NameForm({ label, submitLabel, onSubmit, pending, error, onCancel }: NameFormProps) {
  const inputID = useId()
  const errorID = useId()
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<{ name: string }>({ defaultValues: { name: '' } })

  const submit = handleSubmit(async ({ name }) => {
    try {
      await onSubmit(name.trim())
      reset()
    } catch {
      // Rendered from the error prop; the typed name stays for a retry.
    }
  })

  return (
    <form className="name-form" onSubmit={(e) => void submit(e)} noValidate>
      <label htmlFor={inputID}>{label}</label>
      <div className="name-form__row">
        <input
          id={inputID}
          type="text"
          autoComplete="off"
          // Opened on demand by an "add" button, so focus moves straight to the field.
          autoFocus={onCancel != null}
          aria-invalid={errors.name ? true : undefined}
          aria-describedby={errors.name ? errorID : undefined}
          onKeyDown={(e) => {
            if (e.key === 'Escape' && onCancel) onCancel()
          }}
          {...register('name', { validate: validateName })}
        />
        <button type="submit" className="button-primary" disabled={pending}>
          {pending ? strings.form.saving : submitLabel}
        </button>
        {onCancel && (
          <button type="button" onClick={onCancel}>
            {strings.common.cancel}
          </button>
        )}
      </div>
      {errors.name && (
        <p id={errorID} className="field-error">
          {errors.name.message}
        </p>
      )}
      {error != null && <SubmitError error={error} />}
    </form>
  )
}
