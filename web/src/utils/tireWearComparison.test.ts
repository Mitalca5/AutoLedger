import { describe, expect, it } from 'vitest'
import { tireWearComparison, tireWearComparisonTone, type TireWearComparison } from './tireWearComparison'

const value: TireWearComparison = { measured_lifespan_km: 32000, expected_lifespan_km: 40000, delta_pct: -20, reference: 'declared' }

describe('tire wear comparison', () => {
  it('appears only with a valid measured comparison, including an exact match', () => {
    expect(tireWearComparison({ wear_rate_source: 'measured', wear_comparison: value })).toBe(value)
    expect(tireWearComparison({ wear_rate_source: 'measured', wear_comparison: { ...value, delta_pct: 0 } })?.delta_pct).toBe(0)
    expect(tireWearComparison({ wear_rate_source: 'default', wear_comparison: value })).toBeNull()
    expect(tireWearComparison({ wear_rate_source: 'measured' })).toBeNull()
    expect(tireWearComparison({ wear_rate_source: 'measured', wear_comparison: { ...value, measured_lifespan_km: NaN } })).toBeNull()
    expect(tireWearComparison(null)).toBeNull()
  })

  it('colours a gap beyond 20 percent of the declared lifespan, in either direction', () => {
    expect(tireWearComparisonTone({ ...value, delta_pct: -20 }, 0.8)).toBe('text-slate-300')
    expect(tireWearComparisonTone({ ...value, delta_pct: 20 }, 0.8)).toBe('text-slate-300')
    expect(tireWearComparisonTone({ ...value, delta_pct: -20.1 }, 0.8)).toBe('text-warning-400')
    expect(tireWearComparisonTone({ ...value, delta_pct: 20.1 }, 0.8)).toBe('text-success-400')
  })

  it('stays neutral when the measure is thin or the reference is only the average rate', () => {
    expect(tireWearComparisonTone({ ...value, delta_pct: -50 }, 0.49)).toBe('text-slate-300')
    expect(tireWearComparisonTone({ ...value, delta_pct: -50 }, undefined)).toBe('text-slate-300')
    expect(tireWearComparisonTone({ ...value, delta_pct: -50, reference: 'average' }, 0.9)).toBe('text-slate-300')
    expect(tireWearComparisonTone({ ...value, delta_pct: -50 }, 0.5)).toBe('text-warning-400')
  })
})
