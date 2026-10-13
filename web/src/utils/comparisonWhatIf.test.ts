import { describe, expect, it } from 'vitest'
import type { WhatIfInputs } from './comparisonWhatIf'
import { isWhatIfDirty, isWhatIfValid, whatIfBase, whatIfBounds, whatIfFigures, whatIfForm, whatIfPayload, whatIfRanges, hasExtendedLimits, measuredElectricityPrice } from './comparisonWhatIf'

const limits = { annual_km_max: 200_000, fuel_price_max: 999_999_999.999, electricity_price_max: 999_999.999999 }

const scenario = {
  name: 'Vs SUV',
  mode: 'PROJECTION',
  annual_km: 12000,
  years: 5,
  ice: { fuel_type: 'SP95_E10', l_per_100km: 6.5, fuel_price: 1.75, purchase_price: 25000 },
  tracked: { kwh_per_100km: 16, eur_per_kwh: 0.2, purchase_price: 38000 },
  options: { fuel_inflation_pct: 2 },
}

// The three slider inputs of a test, completed with the saved values of the other ones.
const inputs = (o: { annual_km: number; fuel_price: number; eur_per_kwh: number }): WhatIfInputs => ({ ...whatIfBase(scenario), ...o })
const extendedLimits = { ...limits, years_max: 15, amount_max: 9_999_999_999, inflation_min: -10, inflation_max: 30 }

describe('whatIfBounds', () => {
  it('spans half to twice the saved value and keeps that value on the track', () => {
    const b = whatIfBounds(whatIfBase(scenario), limits)
    expect(b.annual_km).toEqual({ min: 6000, max: 24000, step: 500 })
    expect(b.fuel_price).toEqual({ min: 0.87, max: 3.5, step: 0.01 })
    expect(b.eur_per_kwh).toEqual({ min: 0.1, max: 0.4, step: 0.01 })
  })
  it('never leaves the limits of the API', () => {
    const b = whatIfBounds(inputs({ annual_km: 150_000, fuel_price: 8, eur_per_kwh: 0 }), limits)
    expect(b.annual_km.max).toBe(200_000)
    expect(b.fuel_price.max).toBe(16)
    expect(b.eur_per_kwh.min).toBe(0)
    expect(b.eur_per_kwh.max).toBeGreaterThan(b.eur_per_kwh.min)
  })
})

describe('what-if inputs', () => {
  it('reads the base values of the saved scenario', () => {
    expect(whatIfBase(scenario)).toEqual({
      annual_km: 12000,
      fuel_price: 1.75,
      eur_per_kwh: 0.2,
      years: 5,
      ice_purchase_price: 25000,
      tracked_purchase_price: 38000,
      ice_resale_value: 0,
      tracked_resale_value: 0,
      fuel_inflation_pct: 2,
      electricity_inflation_pct: 0,
      cost_inflation_pct: 0,
    })
    expect(Object.values(whatIfBase(null)).every((v) => v === 0)).toBe(true)
  })
  it('tells an explored value from the original', () => {
    const base = whatIfBase(scenario)
    expect(isWhatIfDirty(base, { ...base })).toBe(false)
    expect(isWhatIfDirty(base, { ...base, fuel_price: 1.9 })).toBe(true)
  })
  it('only accepts values the API accepts', () => {
    expect(isWhatIfValid(inputs({ annual_km: 12000, fuel_price: 0, eur_per_kwh: 0 }), limits)).toBe(true)
    expect(isWhatIfValid(inputs({ annual_km: 0, fuel_price: 1, eur_per_kwh: 0.2 }), limits)).toBe(false)
    expect(isWhatIfValid(inputs({ annual_km: 1000, fuel_price: 1_000_000_000, eur_per_kwh: 0.2 }), limits)).toBe(false)
    expect(isWhatIfValid(inputs({ annual_km: 1000, fuel_price: 1, eur_per_kwh: Number.NaN }), limits)).toBe(false)
  })
})

describe('whatIfPayload', () => {
  it('overrides the three inputs and keeps everything else of the scenario', () => {
    const p = whatIfPayload(scenario, inputs({ annual_km: 20000, fuel_price: 2.1, eur_per_kwh: 0.18 }))
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
    whatIfPayload(scenario, inputs({ annual_km: 1, fuel_price: 1, eur_per_kwh: 1 }))
    whatIfForm(scenario, inputs({ annual_km: 1, fuel_price: 1, eur_per_kwh: 1 }))
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
  expect(isWhatIfValid(inputs({annual_km:12000,fuel_price:1400,eur_per_kwh:100}), limits)).toBe(true)
  const b=whatIfBounds(inputs({annual_km:12000,fuel_price:999_999_999.999,eur_per_kwh:999_999.999999}), limits)
  expect(b.fuel_price.max).toBeLessThanOrEqual(999_999_999.999)
  expect(b.eur_per_kwh.max).toBeLessThanOrEqual(999_999.999999)
 })

 it('uses changed API ceilings without a frontend release and refuses absent limits', () => {
   const lowerLimits = { annual_km_max: 10000, fuel_price_max: 3, electricity_price_max: 1 }
   const explored = inputs({ annual_km: 12000, fuel_price: 8, eur_per_kwh: 2 })
   const bounds = whatIfBounds(explored, lowerLimits)
   expect(bounds.annual_km.max).toBeLessThanOrEqual(10000)
   expect(bounds.fuel_price.min).toBeLessThanOrEqual(bounds.fuel_price.max)
   expect(bounds.fuel_price.max).toBeLessThanOrEqual(3)
   expect(isWhatIfValid(explored, lowerLimits)).toBe(false)
   expect(isWhatIfValid(explored, undefined)).toBe(false)
 })

describe('whatIfRanges', () => {
  it('gives the text fields the limits of the API, not a currency-sized guess', () => {
    const r = whatIfRanges(limits)
    expect(r.annual_km).toEqual({ min: 1, max: 200_000 })
    expect(r.fuel_price.max).toBe(999_999_999.999)
    expect(r.eur_per_kwh.max).toBe(999_999.999999)
    // A local-currency price above the old hard-coded field limits stays valid
    expect(isWhatIfValid(inputs({ annual_km: 12000, fuel_price: 1400, eur_per_kwh: 100 }), limits)).toBe(true)
    expect(1400).toBeLessThanOrEqual(r.fuel_price.max)
    expect(100).toBeLessThanOrEqual(r.eur_per_kwh.max)
  })
  it('follows changed API ceilings', () => {
    const r = whatIfRanges({ annual_km_max: 10000, fuel_price_max: 3, electricity_price_max: 1 })
    expect([r.annual_km.max, r.fuel_price.max, r.eur_per_kwh.max]).toEqual([10000, 3, 1])
  })
})

describe('extended what-if inputs', () => {
  const base = whatIfBase(scenario)

  it('are checked only when the API sent their limits', () => {
    expect(hasExtendedLimits(limits)).toBe(false)
    expect(hasExtendedLimits(extendedLimits)).toBe(true)
    // An older server does not describe the extra fields: they are not checked, nor editable.
    expect(isWhatIfValid({ ...base, years: 99 }, limits)).toBe(true)
    expect(isWhatIfValid({ ...base, years: 99 }, extendedLimits)).toBe(false)
  })
  it('only accepts a whole number of years within the limit', () => {
    expect(isWhatIfValid({ ...base, years: 15 }, extendedLimits)).toBe(true)
    expect(isWhatIfValid({ ...base, years: 0 }, extendedLimits)).toBe(false)
    expect(isWhatIfValid({ ...base, years: 2.5 }, extendedLimits)).toBe(false)
    expect(isWhatIfValid({ ...base, years: 16 }, extendedLimits)).toBe(false)
  })
  it('refuses negative amounts and out-of-range inflation', () => {
    expect(isWhatIfValid({ ...base, ice_resale_value: -1 }, extendedLimits)).toBe(false)
    expect(isWhatIfValid({ ...base, tracked_purchase_price: 10_000_000_000 }, extendedLimits)).toBe(false)
    expect(isWhatIfValid({ ...base, fuel_inflation_pct: -10 }, extendedLimits)).toBe(true)
    expect(isWhatIfValid({ ...base, cost_inflation_pct: 30.5 }, extendedLimits)).toBe(false)
  })
  it('detects a change of any extra field', () => {
    expect(isWhatIfDirty(base, { ...base, years: 7 })).toBe(true)
    expect(isWhatIfDirty(base, { ...base, tracked_resale_value: 12000 })).toBe(true)
  })
  it('go into the payload and leave the rest of the scenario untouched', () => {
    const p = whatIfPayload(scenario, {
      ...base,
      years: 8,
      ice_purchase_price: 21000,
      tracked_purchase_price: 33000,
      ice_resale_value: 6000,
      tracked_resale_value: 14000,
      fuel_inflation_pct: 4,
      electricity_inflation_pct: 1.5,
      cost_inflation_pct: 2.5,
    })
    expect(p.years).toBe(8)
    expect(p.ice.purchase_price).toBe(21000)
    expect(p.tracked.purchase_price).toBe(33000)
    expect(p.ice.resale_value).toBe(6000)
    expect(p.tracked.resale_value).toBe(14000)
    expect(p.options.fuel_inflation_pct).toBe(4)
    expect(p.options.electricity_inflation_pct).toBe(1.5)
    expect(p.options.cost_inflation_pct).toBe(2.5)
    expect(p.ice.l_per_100km).toBe(6.5)
    expect(p.annual_km).toBe(12000)
  })
  it('give the text fields the extended limits', () => {
    const r = whatIfRanges(extendedLimits)
    expect(r.years).toEqual({ min: 1, max: 15 })
    expect(r.ice_resale_value).toEqual({ min: 0, max: 9_999_999_999 })
    expect(r.fuel_inflation_pct).toEqual({ min: -10, max: 30 })
  })
})

describe('retrospective what-if', () => {
  const retro = {
    name: 'Mon véhicule',
    mode: 'RETROSPECTIVE',
    vehicle_id: 'veh-1',
    annual_km: 15000,
    years: 4,
    ice: { fuel_type: 'SP95_E10', l_per_100km: 6, fuel_price: 1.8, purchase_price: 24000 },
    options: {},
  }
  const result = { tracked_measured: { eur_per_kwh: 0.215, kwh_per_100km: 17.6 } }

  it('starts from the recorded price per kWh, which is not an override', () => {
    const base = whatIfBase(retro, result)
    expect(base.eur_per_kwh).toBe(0.215)
    const form = whatIfForm(retro, base, 0.215)
    expect(form.mode).toBe('RETROSPECTIVE')
    expect(form.options.electricity_price_override).toBe(0)
    expect(whatIfPayload(retro, base, 0.215).options.electricity_price_override).toBe(0)
  })
  it('sends another price as an override and leaves the tracked side to the records', () => {
    const p = whatIfPayload(retro, { ...whatIfBase(retro, result), eur_per_kwh: 0.1, years: 6 }, 0.215)
    expect(p.mode).toBe('RETROSPECTIVE')
    expect(p.vehicle_id).toBe('veh-1')
    expect(p.options.electricity_price_override).toBe(0.1)
    expect(p.years).toBe(6)
    expect(p.tracked).toBeUndefined()
  })
  it('starts from a saved override and keeps sending it', () => {
    const saved = { ...retro, options: { electricity_price_override: 0.12 } }
    const base = whatIfBase(saved, result)
    expect(base.eur_per_kwh).toBe(0.12)
    expect(whatIfPayload(saved, base, 0.215).options.electricity_price_override).toBe(0.12)
    expect(isWhatIfDirty(base, { ...base, eur_per_kwh: 0.215 })).toBe(true)
  })
  it('has no recorded price when the vehicle never priced a kWh', () => {
    expect(measuredElectricityPrice({ tracked_measured: {} })).toBe(0)
    expect(measuredElectricityPrice(null)).toBe(0)
  })
  it('does not send an override from a projection', () => {
    const p = whatIfPayload(scenario, { ...whatIfBase(scenario), eur_per_kwh: 0.3 })
    expect(p.options.electricity_price_override).toBe(0)
    expect(p.tracked.eur_per_kwh).toBe(0.3)
  })
})
