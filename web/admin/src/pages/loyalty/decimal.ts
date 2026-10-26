// Exact decimal helpers for the loyalty form. Values travel as decimal
// strings (OpenAPI format: decimal) and the preview must floor exactly like
// the backend's shopspring/decimal math, so arithmetic is done on BigInt
// scaled by 10^SCALE instead of floats.

const SCALE = 4
const ONE = 10n ** BigInt(SCALE)

const DECIMAL_RE = /^\d+(\.\d+)?$/

/** Accepts a comma or dot separator and surrounding spaces; returns the canonical "123.45" form. */
export function normalizeDecimal(input: string): string {
  return input.trim().replace(',', '.')
}

/** Checks a non-negative decimal against a NUMERIC(precision, scale) column. */
export function fitsNumeric(input: string, precision: number, scale: number): boolean {
  const value = normalizeDecimal(input)
  if (!DECIMAL_RE.test(value)) return false
  const [intPart = '', frac = ''] = value.split('.')
  return frac.length <= scale && intPart.replace(/^0+(?=\d)/, '').length <= precision - scale
}

/** Parses a canonical non-negative decimal string (at most SCALE decimals) to a scaled BigInt. */
export function toScaled(value: string): bigint {
  const [intPart = '0', frac = ''] = normalizeDecimal(value).split('.')
  return BigInt(intPart) * ONE + BigInt(frac.padEnd(SCALE, '0').slice(0, SCALE))
}

export function compareDecimal(a: string, b: string): number {
  const x = toScaled(a)
  const y = toScaled(b)
  return x === y ? 0 : x < y ? -1 : 1
}

const HUNDRED = 100n * ONE

/** points.Mechanic.Accrue: floor(amount * percent / 100), or 0 below the minimum purchase. */
export function accruedPoints(amount: string, accrualPercent: string, minPurchase: string): bigint {
  if (compareDecimal(amount, minPurchase) < 0) return 0n
  return (toScaled(amount) * toScaled(accrualPercent)) / (HUNDRED * ONE)
}

/**
 * LoyaltyService.Redeem's cap: floor(amount * maxPercent / 100 / rate)
 * points. null means uncapped — the backend applies no cap when
 * maxRedeemPercent is 0.
 */
export function maxRedeemablePoints(amount: string, maxRedeemPercent: string, rate: string): bigint | null {
  const pct = toScaled(maxRedeemPercent)
  const r = toScaled(rate)
  if (pct === 0n || r === 0n) return null
  return (toScaled(amount) * pct) / (HUNDRED * r)
}

/** Currency value of `points` at `rate`, as a decimal string. */
export function pointsValue(points: bigint, rate: string): string {
  const scaled = points * toScaled(rate)
  const intPart = scaled / ONE
  const frac = (scaled % ONE).toString().padStart(SCALE, '0').replace(/0+$/, '')
  return frac ? `${intPart}.${frac}` : intPart.toString()
}
