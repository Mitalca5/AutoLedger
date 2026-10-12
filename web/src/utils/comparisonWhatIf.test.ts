import { describe, expect, it } from 'vitest'
import { isWhatIfDirty, isWhatIfValid, whatIfBase, whatIfBounds, whatIfFigures, whatIfForm, whatIfPayload } from './comparisonWhatIf'

const scenario = {
  name: 'Vs SUV',
  mode: 'PROJECTION',
  annual_km: 12000,
  years: 5,
  ice: { fuel_type: 'SP95_E10', l_per_100km: 6.5, fuel_price: 1.75, purchase_price: 25000 },
  tracked: { kwh_per_100km: 16, eur_per_kwh: 0.2, purchase_price: 38000 },
  options: { fuel_inflation_pct: 2 },
}

describe('whatIfBounds', () => {
  it('spans half to twice the saved value and keeps that value on the track', () => {
    const b = whatIfBounds(whatIfBase(scenario))
    expect(b.annual_km).toEqual({ min: 6000, max: 24000, step: 500 })
    expect(b.fuel_price).toEqual({ min: 0.87, max: 3.5, step: 0.01 })
    expect(b.eur_per_kwh).toEqual({ min: 0.1, max: 0.4, step: 0.01 })
  })
  it('never leaves the limits of the API', () => {
    const b = whatIfBounds({ annual_km: 150_000, fuel_price: 8, eur_per_kwh: 0 })
    expect(b.annual_km.max).toBe(200_000)
    expect(b.fuel_price.max).toBe(16)
    expect(b.eur_per_kwh.min).toBe(0)
    expect(b.eur_per_kwh.max).toBeGreaterThan(b.eur_per_kwh.min)
  })
})

describe('what-if inputs', () => {
  it('reads the base values of the saved scenario', () => {
    expect(whatIfBase(scenario)).toEqual({ annual_km: 12000, fuel_price: 1.75, eur_per_kwh: 0.2 })
    expect(whatIfBase(null)).toEqual({ annual_km: 0, fuel_price: 0, eur_per_kwh: 0 })
  })
  it('tells an explored value from the original', () => {
    const base = whatIfBase(scenario)
    expect(isWhatIfDirty(base, { ...base })).toBe(false)
    expect(isWhatIfDirty(base, { ...base, fuel_price: 1.9 })).toBe(true)
  })
  it('only accepts values the API accepts', () => {
    expect(isWhatIfValid({ annual_km: 12000, fuel_price: 0, eur_per_kwh: 0 })).toBe(true)
    expect(isWhatIfValid({ annual_km: 0, fuel_price: 1, eur_per_kwh: 0.2 })).toBe(false)
    expect(isWhatIfValid({ annual_km: 1000, fuel_price: 1_000_000_000, eur_per_kwh: 0.2 })).toBe(false)
    expect(isWhatIfValid({ annual_km: 1000, fuel_price: 1, eur_per_kwh: Number.NaN })).toBe(false)
  })
})

describe('whatIfPayload', () => {
  it('overrides the three inputs and keeps everything else of the scenario', () => {
    const p = whatIfPayload(scenario, { annual_km: 20000, fuel_price: 2.1, eur_per_kwh: 0.18 })
    expect(p.mode).toBe('PROJECTION')
    expect(p.annual_km).toBe(20000)
    expect(p.ice.fuel_price).toBe(2.1)
    expect(p.tracked.eur_per_kwh).toBe(0.18)
    expect(p.ice.l_per_100km).toBe(6.5)
    expect(p.tracked.purchase_price).toBe(38000)
    expect(p.years).toBe(5)
    expect(p.options.fuel_inflation_pct).toBe(2)
  })
  it('does not modify the saved scenario', () => {
    const copy = JSON.parse(JSON.stringify(scenario))
    whatIfPayload(scenario, { annual_km: 1, fuel_price: 1, eur_per_kwh: 1 })
    whatIfForm(scenario, { annual_km: 1, fuel_price: 1, eur_per_kwh: 1 })
    expect(scenario).toEqual(copy)
  })
})

describe('whatIfFigures', () => {
  const result = {
    years_count: 5,
    annual_km: 12000,
    tracked: { energy: 2000, total: 30000 },
    ice: { energy: 5500, total: 29000 },
    break_even_year: null,
    break_even_net_year: 6.5,
  }
  it('keeps the energy savings, the full-cost savings and the break-even apart', () => {
    expect(whatIfFigures(result)).toEqual({ energySavings: 3500, totalSavings: -1000, breakEvenYear: null, breakEvenNetYear: 6.5 })
  })
  it('has nothing to show before a result', () => {
    expect(whatIfFigures(null)).toBeNull()
  })
})

 it('accepts local currency prices and keeps slider endpoints within storage bounds',()=>{
  expect(isWhatIfValid({annual_km:12000,fuel_price:1400,eur_per_kwh:100})).toBe(true)
  const b=whatIfBounds({annual_km:12000,fuel_price:999_999_999.999,eur_per_kwh:999_999.999999})
  expect(b.fuel_price.max).toBeLessThanOrEqual(999_999_999.999)
  expect(b.eur_per_kwh.max).toBeLessThanOrEqual(999_999.999999)
 })
