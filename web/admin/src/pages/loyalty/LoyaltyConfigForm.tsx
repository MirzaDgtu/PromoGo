import { useId, useState, type ReactNode } from 'react'
import { useForm, useWatch, type Control, type UseFormRegisterReturn } from 'react-hook-form'

import { responseOf, type LoyaltyConfig } from '../../api/queries'
import { SubmitError } from '../../components/SubmitError'
import { strings } from '../../i18n/strings'
import { accruedPoints, compareDecimal, fitsNumeric, maxRedeemablePoints, normalizeDecimal, pointsValue } from './decimal'

const t = strings.loyalty

export interface LoyaltyFormValues {
  mechanic: string
  accrualPercent: string
  minPurchaseAmount: string
  pointsExchangeRate: string
  maxRedeemPercent: string
  minBalanceToRedeem: string
}

// Starting values for a store with no configuration yet. They're only a
// prefill: nothing applies until the user saves.
const NEW_STORE_DEFAULTS: LoyaltyFormValues = {
  mechanic: 'points',
  accrualPercent: '5',
  minPurchaseAmount: '0',
  pointsExchangeRate: '1',
  maxRedeemPercent: '50',
  minBalanceToRedeem: '0',
}

function fromConfig(c: LoyaltyConfig): LoyaltyFormValues {
  return {
    mechanic: c.mechanic,
    accrualPercent: c.accrual_percent,
    minPurchaseAmount: c.min_purchase_amount,
    pointsExchangeRate: c.points_exchange_rate,
    maxRedeemPercent: c.max_redeem_percent,
    minBalanceToRedeem: String(c.min_balance_to_redeem),
  }
}

export function toConfigInput(v: LoyaltyFormValues, expectedVersion: number): LoyaltyConfig {
  return {
    mechanic: v.mechanic,
    accrual_percent: normalizeDecimal(v.accrualPercent),
    min_purchase_amount: normalizeDecimal(v.minPurchaseAmount),
    points_exchange_rate: normalizeDecimal(v.pointsExchangeRate),
    max_redeem_percent: normalizeDecimal(v.maxRedeemPercent),
    min_balance_to_redeem: Number(v.minBalanceToRedeem.trim()),
    expected_version: expectedVersion,
  }
}

// Validators mirror the backend: domain.LoyaltyConfig.Validate ranges plus
// the NUMERIC(precision, scale) column checks in admin_loyaltyconfig.go.
function decimalRule(precision: number, scale: number, extra?: (v: string) => true | string) {
  return (raw: string): true | string => {
    const v = normalizeDecimal(raw)
    if (!/^\d+([.]\d+)?$/.test(v)) return t.errors.number
    if ((v.split('.')[1] ?? '').length > scale) return t.errors.decimals(scale)
    if (!fitsNumeric(v, precision, scale)) return t.errors.tooLarge
    return extra ? extra(v) : true
  }
}
const percentRule = decimalRule(5, 2, (v) => compareDecimal(v, '100') <= 0 || t.errors.percent)
const amountRule = decimalRule(20, 2)
const rateRule = decimalRule(10, 4, (v) => compareDecimal(v, '0') > 0 || t.errors.positive)
const pointsRule = (raw: string): true | string => {
  const v = raw.trim()
  if (!/^\d+$/.test(v)) return t.errors.integer
  return v.length <= 15 || t.errors.tooLarge
}

interface Props {
  /** null when the store isn't configured yet. */
  config: LoyaltyConfig | null
  canWrite: boolean
  onSubmit: (values: LoyaltyFormValues) => Promise<unknown>
  pending: boolean
  error: unknown
  onReload: () => void
}

export function LoyaltyConfigForm({ config, canWrite, onSubmit, pending, error, onReload }: Props) {
  const {
    register,
    handleSubmit,
    reset,
    control,
    formState: { errors, isDirty },
  } = useForm<LoyaltyFormValues>({ defaultValues: config ? fromConfig(config) : NEW_STORE_DEFAULTS, mode: 'onChange' })

  const submit = handleSubmit(async (values) => {
    try {
      await onSubmit(values)
    } catch {
      // Rendered from the error prop below.
    }
  })

  const conflict = responseOf(error)?.status === 409
  const serverRejected = responseOf(error)?.status === 422

  return (
    <form className="loyalty-form" aria-label={t.title} noValidate onSubmit={(e) => void submit(e)}>
      <div className="loyalty-form__layout">
        {/* A plain fieldset disables every field at once for read-only
            callers; the preview stays outside it so it remains usable. */}
        <fieldset className="loyalty-form__fields" disabled={!canWrite}>
          <Field label={t.fields.mechanic}>
            {(id) => (
              <select id={id} {...register('mechanic')}>
                {Object.entries(t.mechanics).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            )}
          </Field>

          <h3>{t.sections.accrual}</h3>
          <TextField label={t.fields.accrualPercent} hint={t.fields.accrualPercentHint} error={errors.accrualPercent?.message}
            registration={register('accrualPercent', { validate: percentRule })} />
          <TextField label={t.fields.minPurchaseAmount} hint={t.fields.minPurchaseAmountHint} error={errors.minPurchaseAmount?.message}
            registration={register('minPurchaseAmount', { validate: amountRule })} />

          <h3>{t.sections.redemption}</h3>
          <TextField label={t.fields.pointsExchangeRate} hint={t.fields.pointsExchangeRateHint} error={errors.pointsExchangeRate?.message}
            registration={register('pointsExchangeRate', { validate: rateRule })} />
          <TextField label={t.fields.maxRedeemPercent} hint={t.fields.maxRedeemPercentHint} error={errors.maxRedeemPercent?.message}
            registration={register('maxRedeemPercent', { validate: percentRule })} />
          <TextField label={t.fields.minBalanceToRedeem} hint={t.fields.minBalanceToRedeemHint} error={errors.minBalanceToRedeem?.message}
            inputMode="numeric" registration={register('minBalanceToRedeem', { validate: pointsRule })} />
        </fieldset>

        <Preview control={control} />
      </div>

      {canWrite && (
        <>
          {conflict ? (
            <div role="alert" className="alert">
              <p>{t.errors.conflict}</p>
              <button type="button" onClick={onReload}>
                {t.errors.reload}
              </button>
            </div>
          ) : serverRejected ? (
            <p role="alert" className="field-error">
              {t.errors.server}
            </p>
          ) : (
            error != null && <SubmitError error={error} />
          )}
          <div className="form-actions">
            <button type="submit" className="button-primary" disabled={pending || conflict || (config != null && !isDirty)}>
              {pending ? strings.form.saving : config ? t.save : t.saveFirst}
            </button>
            {config && isDirty && (
              <button type="button" onClick={() => reset()}>
                {t.discard}
              </button>
            )}
          </div>
        </>
      )}
    </form>
  )
}

function Preview({ control }: { control: Control<LoyaltyFormValues> }) {
  const amountID = useId()
  const [amount, setAmount] = useState('1000')
  const {
    accrualPercent = '',
    minPurchaseAmount = '',
    pointsExchangeRate = '',
    maxRedeemPercent = '',
    minBalanceToRedeem = '',
  } = useWatch({ control })
  // Watched values render before formState.errors catches up, so these exact
  // strings are validated here: the arithmetic below throws on anything else.
  const valid =
    percentRule(accrualPercent) === true &&
    amountRule(minPurchaseAmount) === true &&
    rateRule(pointsExchangeRate) === true &&
    percentRule(maxRedeemPercent) === true &&
    pointsRule(minBalanceToRedeem) === true
  const amountOK = fitsNumeric(amount, 20, 2)

  let body: ReactNode
  if (!valid || !amountOK) {
    body = <p className="muted">{t.preview.invalid}</p>
  } else {
    const a = normalizeDecimal(amount)
    const rate = normalizeDecimal(pointsExchangeRate)
    const minPurchase = normalizeDecimal(minPurchaseAmount)
    const earned = accruedPoints(a, normalizeDecimal(accrualPercent), minPurchase)
    const cap = maxRedeemablePoints(a, normalizeDecimal(maxRedeemPercent), rate)
    body = (
      <ul>
        <li>
          {compareDecimal(a, minPurchase) < 0 ? t.preview.belowMin(minPurchase) : t.preview.earn(t.points(earned))}
        </li>
        <li>{cap == null ? t.preview.redeemUncapped : t.preview.redeemCapped(t.points(cap), pointsValue(cap, rate))}</li>
        <li>{t.preview.pointValue(rate)}</li>
      </ul>
    )
  }

  return (
    <aside className="loyalty-preview" aria-labelledby={`${amountID}-title`}>
      <h3 id={`${amountID}-title`}>{t.preview.title}</h3>
      <label htmlFor={amountID}>{t.preview.amount}</label>
      <input id={amountID} type="text" inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} />
      <div aria-live="polite">{body}</div>
    </aside>
  )
}

function Field({ label, hint, error, children }: { label: string; hint?: string; error?: string | undefined; children: (id: string, describedBy: string | undefined) => ReactNode }) {
  const id = useId()
  const hintID = useId()
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {children(id, hint || error ? hintID : undefined)}
      {(error || hint) && (
        <p id={hintID} className={error ? 'field-error' : 'field-hint'}>
          {error ?? hint}
        </p>
      )}
    </div>
  )
}

function TextField({
  label,
  hint,
  error,
  inputMode = 'decimal',
  registration,
}: {
  label: string
  hint: string
  error: string | undefined
  inputMode?: 'decimal' | 'numeric'
  registration: UseFormRegisterReturn
}) {
  return (
    <Field label={label} hint={hint} error={error}>
      {(id, describedBy) => (
        <input
          id={id}
          type="text"
          inputMode={inputMode}
          autoComplete="off"
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          {...registration}
        />
      )}
    </Field>
  )
}
