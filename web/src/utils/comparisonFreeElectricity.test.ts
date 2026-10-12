import { describe, expect, it } from 'vitest'
import { applyComparisonDefaults, comparisonStepErrorKey, emptyComparisonForm } from './comparisonForm'

function validProjection() {
  const form = emptyComparisonForm(false)
  form.ice.purchase_price = 20000
  form.tracked.purchase_price = 25000
  return form
}

describe('free electricity in projections', () => {
  it('accepts explicit zero but not missing, negative or nonfinite prices', () => {
    const form = validProjection()
    for (const price of [0, '0']) {
      form.tracked.eur_per_kwh = price
      expect(comparisonStepErrorKey(2, form, false)).toBe('')
    }
    for (const price of ['', null, undefined, -1, NaN, Infinity]) {
      form.tracked.eur_per_kwh = price as any
      expect(comparisonStepErrorKey(2, form, false)).toBe('comparison.errors.electricityPrice')
    }
  })

  it('does not mistake an unavailable default for free electricity', () => {
    const form = validProjection()
    applyComparisonDefaults(form, { annual_km: 12000, maintenance_yearly: 0, insurance_yearly: 0, indicative_prices: false })
    expect(form.tracked.eur_per_kwh).toBe('')
    expect(comparisonStepErrorKey(2, form, false)).toBe('comparison.errors.electricityPrice')
    form.tracked.eur_per_kwh = 0
    expect(comparisonStepErrorKey(2, form, false)).toBe('')
  })

  it('preserves an explicitly free default even without indicative prices', () => {
    const form = validProjection()
    applyComparisonDefaults(form, { annual_km: 12000, maintenance_yearly: 0, insurance_yearly: 0, indicative_prices: false, tracked_eur_per_kwh: 0 })
    expect(form.tracked.eur_per_kwh).toBe(0)
    expect(comparisonStepErrorKey(2, form, false)).toBe('')
  })
})
