import { describe, expect, it } from 'vitest'
import { tireWearComparison, tireWearComparisonTone } from './tireWearComparison'

const value = { measured_rate_per_10k_km: 2, expected_rate_per_10k_km: 1.6, measured_lifespan_km: 32000, expected_lifespan_km: 40000, delta_pct: 25, reference: 'declared' as const }

describe('tire wear comparison', () => {
  it('appears only with a valid measured comparison, including an exact match', () => {
    expect(tireWearComparison({ wear_rate_source: 'measured', wear_comparison: value })).toBe(value)
    expect(tireWearComparison({ wear_rate_source: 'measured', wear_comparison: { ...value, delta_pct: 0 } })?.delta_pct).toBe(0)
    expect(tireWearComparison({ wear_rate_source: 'default', wear_comparison: value })).toBeNull()
    expect(tireWearComparison({ wear_rate_source: 'measured' })).toBeNull()
    expect(tireWearComparison({ wear_rate_source: 'measured', wear_comparison: { ...value, measured_lifespan_km: NaN } })).toBeNull()
    expect(tireWearComparison(null)).toBeNull()
  })
  it('highlights deviations strictly outside the 25 percent band', () => {
    expect(tireWearComparisonTone(25)).toBe('text-slate-300')
    expect(tireWearComparisonTone(-25)).toBe('text-slate-300')
    expect(tireWearComparisonTone(25.1)).toBe('text-warning-400')
    expect(tireWearComparisonTone(-25.1)).toBe('text-success-400')
  })
})
