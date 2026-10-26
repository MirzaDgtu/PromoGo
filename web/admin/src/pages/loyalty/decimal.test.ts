import { describe, expect, it } from 'vitest'

import { accruedPoints, fitsNumeric, maxRedeemablePoints, normalizeDecimal, pointsValue } from './decimal'

describe('loyalty decimal math (mirrors points.Mechanic and LoyaltyService.Redeem)', () => {
  it('floors accrual and skips purchases below the minimum', () => {
    expect(accruedPoints('999.99', '5', '0')).toBe(49n) // 49.9995 → 49
    expect(accruedPoints('1000', '7.5', '0')).toBe(75n)
    expect(accruedPoints('0.29', '33.33', '0')).toBe(0n)
    expect(accruedPoints('499.99', '10', '500')).toBe(0n)
    expect(accruedPoints('500', '10', '500')).toBe(50n)
  })

  it('caps redemption by percent and rate; 0 % means uncapped, like the backend', () => {
    expect(maxRedeemablePoints('1000', '50', '1')).toBe(500n)
    expect(maxRedeemablePoints('1000', '50', '0.3')).toBe(1666n) // 500 / 0.3 = 1666.6…
    expect(maxRedeemablePoints('1000', '0', '1')).toBeNull()
  })

  it('prices points exactly', () => {
    expect(pointsValue(1666n, '0.3')).toBe('499.8')
    expect(pointsValue(500n, '1')).toBe('500')
  })

  it('accepts a comma and checks NUMERIC precision/scale', () => {
    expect(normalizeDecimal(' 2,5 ')).toBe('2.5')
    expect(fitsNumeric('10.25', 5, 2)).toBe(true)
    expect(fitsNumeric('10.255', 5, 2)).toBe(false)
    expect(fitsNumeric('999999.9999', 10, 4)).toBe(true)
    expect(fitsNumeric('1000000', 10, 4)).toBe(false)
    expect(fitsNumeric('-1', 5, 2)).toBe(false)
    expect(fitsNumeric('abc', 5, 2)).toBe(false)
  })
})
