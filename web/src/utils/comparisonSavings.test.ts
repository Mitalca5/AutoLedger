import { describe, expect, it } from 'vitest'
import { comparisonSavings } from './comparisonSavings'

describe('comparisonSavings', () => {
  it('separates energy savings from the savings after other costs', () => {
    const energy = comparisonSavings(7591, 12600, 5, 20000)
    const total = comparisonSavings(17713, 18600, 5, 20000)
    expect(energy.amount).toBe(5009)
    expect(total.amount).toBe(887)
    expect(energy.percent).toBeCloseTo(39.753968)
    expect(total.percent).toBeCloseTo(4.768817)
    expect(energy.perMonth).toBeCloseTo(83.483333)
    expect(total.perKm).toBeCloseTo(0.00887)
    expect(energy.amount - total.amount).toBe(4122)
  })
  it('keeps additional costs negative, independently for each category', () => {
    expect(comparisonSavings(1500, 1000, 1, 10000)).toEqual({ amount: -500, percent: -50, perMonth: -500 / 12, perKm: -0.05 })
    expect(comparisonSavings(2000, 3000, 1, 10000).amount).toBe(1000)
  })
  it('handles free energy without interpreting zero as missing', () => {
    expect(comparisonSavings(0, 1000, 1, 10000).percent).toBe(100)
    expect(comparisonSavings(1000, 0, 1, 10000).percent).toBeNull()
    expect(comparisonSavings(0, 0, 1, 10000).amount).toBe(0)
  })
  it('does not divide by an unavailable period or distance', () => {
    expect(comparisonSavings(1000, 2000, 0, 10000).perMonth).toBeNull()
    expect(comparisonSavings(1000, 2000, 0, 10000).perKm).toBeNull()
    expect(comparisonSavings(1000, 2000, 5, 0).perKm).toBeNull()
    expect(comparisonSavings(1000, 2000, 5, 0).perMonth).toBeCloseTo(1000 / 60)
  })
})
