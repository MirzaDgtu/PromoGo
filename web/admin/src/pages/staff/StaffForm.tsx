import { useId } from 'react'
import { useForm, useWatch } from 'react-hook-form'

import { responseOf, type StaffRole } from '../../api/queries'
import type { components } from '../../api/schema.gen'
import { SubmitError } from '../../components/SubmitError'
import { strings } from '../../i18n/strings'

const t = strings.staff
type Store = components['schemas']['Store']

// Roles whose permissions only work organization-wide: staff.manage and
// stores.manage are checked without a store, so a store-scoped
// retailer_admin couldn't do the job the role is for.
export const ORG_ONLY_ROLES: ReadonlySet<StaffRole> = new Set(['platform_admin', 'retailer_admin'])

export function grantableRoles(callerIsPlatformAdmin: boolean): StaffRole[] {
  const roles: StaffRole[] = ['retailer_admin', 'store_manager', 'support_viewer']
  return callerIsPlatformAdmin ? ['platform_admin', ...roles] : roles
}

interface FormValues {
  code: string
  role: StaffRole
  storeID: string // '' = the whole organization
}

export interface StaffInput {
  external_subject: string
  role: StaffRole
  store_id?: number
}

export function StaffForm({
  stores,
  callerIsPlatformAdmin,
  onSubmit,
  onCancel,
  pending,
  error,
}: {
  stores: Store[]
  callerIsPlatformAdmin: boolean
  onSubmit: (input: StaffInput) => Promise<unknown>
  onCancel: () => void
  pending: boolean
  error: unknown
}) {
  const codeID = useId()
  const codeHintID = useId()
  const roleID = useId()
  const scopeID = useId()
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<FormValues>({ defaultValues: { code: '', role: 'store_manager', storeID: '' } })

  const role = useWatch({ control, name: 'role' })
  const orgOnly = ORG_ONLY_ROLES.has(role)

  const submit = handleSubmit(async (values) => {
    const input: StaffInput = { external_subject: values.code.trim(), role: values.role }
    if (!ORG_ONLY_ROLES.has(values.role) && values.storeID) input.store_id = Number(values.storeID)
    try {
      await onSubmit(input)
    } catch {
      // Rendered from the error prop below.
    }
  })

  return (
    <form
      className="api-key-form"
      aria-label={t.form.title}
      noValidate
      onSubmit={(e) => void submit(e)}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onCancel()
      }}
    >
      <h2>{t.form.title}</h2>

      <div className="field">
        <label htmlFor={codeID}>{t.form.code}</label>
        <input
          id={codeID}
          type="text"
          autoComplete="off"
          spellCheck={false}
          autoFocus
          aria-invalid={errors.code ? true : undefined}
          aria-describedby={codeHintID}
          {...register('code', { validate: (v) => v.trim() !== '' || t.form.codeRequired })}
        />
        <p id={codeHintID} className={errors.code ? 'field-error' : 'field-hint'}>
          {errors.code?.message ?? t.form.codeHint}
        </p>
      </div>

      <div className="field">
        <label htmlFor={roleID}>{t.form.role}</label>
        <select id={roleID} {...register('role')}>
          {grantableRoles(callerIsPlatformAdmin).map((r) => (
            <option key={r} value={r}>
              {t.roles[r].label}
            </option>
          ))}
        </select>
        <p className="field-hint">{t.roles[role].description}</p>
      </div>

      <div className="field">
        <label htmlFor={scopeID}>{t.form.scope}</label>
        <select id={scopeID} disabled={orgOnly} {...register('storeID')}>
          <option value="">{t.allStores}</option>
          {stores.map((s) => (
            <option key={s.id} value={String(s.id)}>
              {t.storeScope(s.name ?? '')}
            </option>
          ))}
        </select>
        <p className="field-hint">{orgOnly ? t.form.scopeOrgOnly : t.form.scopeHint}</p>
      </div>

      <div className="form-actions">
        <button type="submit" className="button-primary" disabled={pending}>
          {pending ? strings.form.saving : t.form.submit}
        </button>
        <button type="button" onClick={onCancel}>
          {strings.common.cancel}
        </button>
      </div>
      {error != null &&
        (responseOf(error)?.status === 409 ? (
          <p role="alert" className="field-error">
            {t.form.conflict}
          </p>
        ) : (
          <SubmitError error={error} />
        ))}
    </form>
  )
}
