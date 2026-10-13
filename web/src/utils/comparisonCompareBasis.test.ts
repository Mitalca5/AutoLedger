import { describe, expect, it } from 'vitest'
import {
  OWN_BASIS,
  basisPayload,
  cumulativeAdvantage,
  electricityPriceApplies,
  gapAgainst,
  isBasisValid,
  isOwnBasis,
} from './comparisonCompareBasis'

const limits = { annual_km_max: 200_000, years_max: 15 }

const projection = {
  name: 'Projection',
  mode: 'PROJECTION',
  currency: 'EUR',
  annual_km: 12000,
  years: 5,
  ice: { fuel_type: 'SP95_E10', l_per_100km: 6.5, fuel_price: 2, purchase_price: 25000 },
  tracked: { kwh_per_100km: 16, eur_per_kwh: 0.2, purchase_price: 38000 },
  options: {},
}
const retro = {
  name: 'Réel',
  mode: 'RETROSPECTIVE',
  vehicle_id: 'veh-1',
  annual_km: 15000,
  years: 4,
  ice: { fuel_type: 'DIESEL', l_per_100km: 5.5, fuel_price: 1.8 },
  options: {},
}
const retroResult = { tracked_measured: { eur_per_kwh: 0.25 } }

describe('basis', () => {
  it('is the own basis until something is set', () => {
    expect(isOwnBasis(OWN_BASIS)).toBe(true)
    expect(isOwnBasis({ ...OWN_BASIS, years: 3 })).toBe(false)
    expect(isOwnBasis({ ...OWN_BASIS, energy_price_pct: 10 })).toBe(false)
  })
  it('only accepts what the API accepts', () => {
    expect(isBasisValid(OWN_BASIS, limits)).toBe(true)
    expect(isBasisValid({ annual_km: 20000, years: 8, energy_price_pct: 25 }, limits)).toBe(true)
    expect(isBasisValid({ ...OWN_BASIS, annual_km: 0 }, limits)).toBe(false)
    expect(isBasisValid({ ...OWN_BASIS, annual_km: 200_001 }, limits)).toBe(false)
    expect(isBasisValid({ ...OWN_BASIS, years: 2.5 }, limits)).toBe(false)
    expect(isBasisValid({ ...OWN_BASIS, years: 16 }, limits)).toBe(false)
    expect(isBasisValid({ ...OWN_BASIS, energy_price_pct: -60 }, limits)).toBe(false)
    expect(isBasisValid({ ...OWN_BASIS, energy_price_pct: Number.NaN }, limits)).toBe(false)
  })
})

describe('basisPayload', () => {
  it('keeps the scenario as saved on its own basis', () => {
    const p = basisPayload(projection, {}, OWN_BASIS)
    expect(p.annual_km).toBe(12000)
    expect(p.years).toBe(5)
    expect(p.ice.fuel_price).toBe(2)
    expect(p.tracked.eur_per_kwh).toBe(0.2)
  })
  it('puts the common distance and period on every scenario', () => {
    const basis = { annual_km: 20000, years: 8, energy_price_pct: 0 }
    const a = basisPayload(projection, {}, basis)
    const b = basisPayload(retro, retroResult, basis)
    expect([a.annual_km, a.years, b.annual_km, b.years]).toEqual([20000, 8, 20000, 8])
    expect(b.mode).toBe('RETROSPECTIVE')
    expect(b.vehicle_id).toBe('veh-1')
  })
  it('moves the fuel and electricity prices of a projection together', () => {
    const p = basisPayload(projection, {}, { ...OWN_BASIS, energy_price_pct: 10 })
    expect(p.ice.fuel_price).toBe(2.2)
    expect(p.tracked.eur_per_kwh).toBe(0.22)
  })
  it('moves a retrospective electricity price through the override, from the recorded price', () => {
    const p = basisPayload(retro, retroResult, { ...OWN_BASIS, energy_price_pct: -20 })
    expect(p.ice.fuel_price).toBe(1.44)
    expect(p.options.electricity_price_override).toBe(0.2)
    const saved = { ...retro, options: { electricity_price_override: 0.1 } }
    expect(basisPayload(saved, retroResult, { ...OWN_BASIS, energy_price_pct: 50 }).options.electricity_price_override).toBe(0.15)
  })
  it('leaves a retrospective scenario without a recorded price on its recorded electricity', () => {
    expect(electricityPriceApplies(retro, {})).toBe(false)
    expect(electricityPriceApplies(retro, retroResult)).toBe(true)
    expect(electricityPriceApplies(projection, {})).toBe(true)
    const p = basisPayload(retro, {}, { ...OWN_BASIS, energy_price_pct: 10 })
    expect(p.options.electricity_price_override).toBe(0)
  })
  it('does not modify the saved scenario', () => {
    const copy = JSON.parse(JSON.stringify(projection))
    basisPayload(projection, {}, { annual_km: 1, years: 1, energy_price_pct: 50 })
    expect(projection).toEqual(copy)
  })
})

describe('cumulativeAdvantage', () => {
  it('is the combustion outlay minus the electric one, year by year', () => {
    const r = { cumulative: [{ year: 0, tracked: 38000, ice: 25000 }, { year: 1, tracked: 39000, ice: 28000 }, { year: 2, tracked: 40000, ice: 40000 }] }
    expect(cumulativeAdvantage(r)).toEqual([
      { year: 0, advantage: -13000 },
      { year: 1, advantage: -11000 },
      { year: 2, advantage: 0 },
    ])
  })
  it('has no curve without a cumulative series', () => {
    expect(cumulativeAdvantage({})).toEqual([])
    expect(cumulativeAdvantage(null)).toEqual([])
  })
})

describe('gapAgainst', () => {
  const reference = { tracked_savings: 4000, break_even_year: 3 }
  it('gives the gap in amount, in percent of the reference saving and in break-even', () => {
    const g = gapAgainst(reference, { tracked_savings: 5000, break_even_year: 2.5 })
    expect(g.savings).toBe(1000)
    expect(g.savingsPct).toBe(25)
    expect(g.breakEven).toEqual({ kind: 'years', years: -0.5 })
  })
  it('is zero against itself', () => {
    const g = gapAgainst(reference, reference)
    expect(g.savings).toBe(0)
    expect(g.savingsPct).toBe(0)
    expect(g.breakEven).toEqual({ kind: 'years', years: 0 })
  })
  it('has no percentage when the reference saves nothing', () => {
    expect(gapAgainst({ tracked_savings: 0 }, { tracked_savings: 300 }).savingsPct).toBeNull()
  })
  it('says when a break-even is reached or lost against the reference', () => {
    expect(gapAgainst({ tracked_savings: -500 }, { tracked_savings: 100, break_even_year: 4 }).breakEven).toEqual({ kind: 'reached' })
    expect(gapAgainst(reference, { tracked_savings: -100 }).breakEven).toEqual({ kind: 'lost' })
    expect(gapAgainst({}, {}).breakEven).toEqual({ kind: 'none' })
  })
})
