import { useId } from 'react'
import { useForm, type UseFormRegisterReturn } from 'react-hook-form'

import { responseOf, type StoreProfileInput } from '../api/queries'
import { strings } from '../i18n/strings'
import { MAX_ADDRESS_LENGTH, MAX_EXTERNAL_CODE_LENGTH, maxLength, validateName } from './formValidation'
import { SubmitError } from './SubmitError'

export interface StoreFormValues {
  name: string
  address: string
  externalCode: string
}

/** Converts form values to the API body; an empty code clears it server-side. */
export function toStoreInput(values: StoreFormValues): StoreProfileInput & { name: string } {
  return { name: values.name.trim(), address: values.address.trim(), external_code: values.externalCode.trim() }
}

interface StoreFormProps {
  /** Accessible name of the form, e.g. "Новый магазин в «Ромашка»". */
  title: string
  initial?: StoreFormValues
  submitLabel: string
  /** Reject to keep the form open with the typed values. */
  onSubmit: (values: StoreFormValues) => Promise<unknown>
  onCancel: () => void
  pending: boolean
  error: unknown
}

const EMPTY: StoreFormValues = { name: '', address: '', externalCode: '' }

export function StoreForm({ title, initial = EMPTY, submitLabel, onSubmit, onCancel, pending, error }: StoreFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<StoreFormValues>({ defaultValues: initial })

  const submit = handleSubmit(async (values) => {
    try {
      await onSubmit(values)
    } catch {
      // Rendered from the error prop below.
    }
  })

  // A 409 is always the 1C code (the only unique store field), so it is
  // shown on that field instead of as a generic failure.
  const codeConflict = responseOf(error)?.status === 409

  return (
    <form
      className="store-form"
      aria-label={title}
      noValidate
      onSubmit={(e) => void submit(e)}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onCancel()
      }}
    >
      <div className="store-form__fields">
        <Field
          label={strings.stores.name}
          error={errors.name?.message}
          autoFocus
          registration={register('name', { validate: validateName })}
        />
        <Field
          label={strings.stores.address}
          error={errors.address?.message}
          registration={register('address', { validate: maxLength(MAX_ADDRESS_LENGTH) })}
        />
        <Field
          label={strings.stores.externalCode}
          hint={strings.stores.externalCodeHint}
          error={errors.externalCode?.message ?? (codeConflict ? strings.stores.externalCodeTaken : undefined)}
          registration={register('externalCode', { validate: maxLength(MAX_EXTERNAL_CODE_LENGTH) })}
        />
      </div>
      <div className="store-form__actions">
        <button type="submit" className="button-primary" disabled={pending}>
          {pending ? strings.form.saving : submitLabel}
        </button>
        <button type="button" onClick={onCancel}>
          {strings.common.cancel}
        </button>
      </div>
      {error != null && !codeConflict && <SubmitError error={error} />}
    </form>
  )
}

function Field({
  label,
  hint,
  error,
  autoFocus,
  registration,
}: {
  label: string
  hint?: string
  error?: string | undefined
  autoFocus?: boolean
  registration: UseFormRegisterReturn
}) {
  const id = useId()
  const hintID = useId()
  const errorID = useId()
  const describedBy = [hint && hintID, error && errorID].filter(Boolean).join(' ') || undefined

  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type="text"
        autoComplete="off"
        autoFocus={autoFocus}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        {...registration}
      />
      {hint && (
        <p id={hintID} className="field-hint">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorID} className="field-error">
          {error}
        </p>
      )}
    </div>
  )
}
