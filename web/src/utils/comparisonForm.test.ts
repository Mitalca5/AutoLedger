import { describe, expect, it } from 'vitest'
import {
  applyComparisonDefaults,
  buildComparisonPayload,
  comparisonStepErrorKey,
  costRowValues,
  emptyComparisonForm,
  hasAdvancedOptions,
  scenarioToForm,
  toggleSelection,
} from './comparisonForm'

describe('emptyComparisonForm', () => {
  it('starts a tracked comparison only when the vehicle can be compared', () => {
    expect(emptyComparisonForm(true).mode).toBe('RETROSPECTIVE')
    expect(emptyComparisonForm(false).mode).toBe('PROJECTION')
  })
})

describe('applyComparisonDefaults', () => {
  const defaults = {
    annual_km: 15000,
    maintenance_yearly: 500,
    insurance_yearly: 600,
    ev_kwh_per_100km: 17.5,
    ice: [
      { fuel_type: 'SP95_E10', l_per_100km: 7, fuel_price: 1.8 },
      { fuel_type: 'DIESEL', l_per_100km: 5.5, fuel_price: 1.7 },
    ],
  }

  it('fills the form from the defaults and the fuel type', () => {
    const form = emptyComparisonForm(false)
    applyComparisonDefaults(form, defaults)
    expect(form.annual_km).toBe(15000)
    expect(form.ice.maintenance_yearly).toBe(500)
    expect(form.ev.kwh_per_100km).toBe(17.5)
    expect(form.ev.eur_per_kwh).toBe(0.2)
    expect(form.ice.l_per_100km).toBe(7)
    expect(form.ice.fuel_price).toBe(1.8)
  })

  it('lets measured figures win over the fuel type', () => {
    const form = emptyComparisonForm(false)
    applyComparisonDefaults(form, { ...defaults, ice_l_per_100km: 4.2, ice_fuel_price: 1.5, ev_eur_per_kwh: 0.31 })
    expect(form.ice.l_per_100km).toBe(4.2)
    expect(form.ice.fuel_price).toBe(1.5)
    expect(form.ev.eur_per_kwh).toBe(0.31)
  })

  it('leaves prices empty when the defaults carry none for the currency', () => {
    const form = emptyComparisonForm(false)
    applyComparisonDefaults(form, { ...defaults, indicative_prices: false, ice: [{ fuel_type: 'SP95_E10', l_per_100km: 7, fuel_price: 0 }] })
    expect(form.ev.eur_per_kwh).toBe(0)
    expect(form.ice.l_per_100km).toBe(7)
    expect(form.ice.fuel_price).toBe(0)
  })

  it('keeps the form untouched without defaults or a matching fuel type', () => {
    const form = emptyComparisonForm(false)
    applyComparisonDefaults(form, null)
    expect(form).toEqual(emptyComparisonForm(false))
    applyComparisonDefaults(form, { ...defaults, ice: [{ fuel_type: 'LPG', l_per_100km: 9, fuel_price: 0.9 }] })
    expect(form.ice.l_per_100km).toBe(6.5)
  })
})

describe('scenarioToForm', () => {
  it('copies a saved scenario and fills the missing parts', () => {
    const form = scenarioToForm({ mode: 'RETROSPECTIVE', name: 'A', annual_km: 9000, years: 4, ice: { fuel_type: 'DIESEL' }, options: { fuel_inflation_pct: 3 } }, true)
    expect(form.mode).toBe('RETROSPECTIVE')
    expect(form.ice.fuel_type).toBe('DIESEL')
    expect(form.ev).toEqual(emptyComparisonForm(true).ev)
    expect(form.options).toEqual({ fuel_inflation_pct: 3, electricity_inflation_pct: 0, cost_inflation_pct: 0, ev_incentives: 0 })
    expect(hasAdvancedOptions(form)).toBe(true)
    expect(hasAdvancedOptions(emptyComparisonForm(true))).toBe(false)
  })

  it('keeps the entered electric side of a projection', () => {
    const ev = { kwh_per_100km: 15 }
    expect(scenarioToForm({ mode: 'PROJECTION', name: 'B', annual_km: 1, years: 1, ice: {}, ev }, false).ev).toEqual(ev)
  })
})

describe('buildComparisonPayload', () => {
  it('sends the vehicle for a tracked comparison and no electric side or incentives', () => {
    const form = emptyComparisonForm(true)
    form.name = 'Tracked'
    form.options.ev_incentives = 4000
    const payload = buildComparisonPayload(form, 'veh-1')
    expect(payload.vehicle_id).toBe('veh-1')
    expect(payload.ev).toBeUndefined()
    expect(payload.options.ev_incentives).toBe(0)
  })

  it('sends the electric side and incentives for a projection, with numbers coerced', () => {
    const form = emptyComparisonForm(false)
    form.annual_km = '10000' as any
    form.ev.kwh_per_100km = '17' as any
    form.ev.purchase_price = '30000' as any
    form.options.ev_incentives = '1000' as any
    const payload = buildComparisonPayload(form, 'ignored')
    expect(payload.vehicle_id).toBeUndefined()
    expect(payload.annual_km).toBe(10000)
    expect(payload.ev.kwh_per_100km).toBe(17)
    expect(payload.ev.purchase_price).toBe(30000)
    expect(payload.options.ev_incentives).toBe(1000)
    expect(payload.ice.tax_yearly).toBe(0)
  })
})

describe('comparisonStepErrorKey', () => {
  it('validates the general step', () => {
    const form = emptyComparisonForm(false)
    expect(comparisonStepErrorKey(1, form, false)).toBe('comparison.errors.name')
    form.name = 'x'
    form.annual_km = 0
    expect(comparisonStepErrorKey(1, form, false)).toBe('comparison.errors.annualKm')
    form.annual_km = 1000
    form.years = 16
    expect(comparisonStepErrorKey(1, form, false)).toBe('comparison.errors.years')
    form.years = 5
    expect(comparisonStepErrorKey(1, form, false)).toBe('')
    form.mode = 'RETROSPECTIVE'
    expect(comparisonStepErrorKey(1, form, false)).toBe('comparison.errors.trackedNeeded')
    expect(comparisonStepErrorKey(1, form, true)).toBe('')
  })

  it('validates the vehicles step', () => {
    const form = emptyComparisonForm(false)
    form.ice.l_per_100km = 0
    expect(comparisonStepErrorKey(2, form, false)).toBe('comparison.errors.iceConsumption')
    form.ice.l_per_100km = 6
    expect(comparisonStepErrorKey(2, form, false)).toBe('comparison.errors.icePrice')
    form.ice.fuel_price = 0
    expect(comparisonStepErrorKey(2, form, false)).toBe('comparison.errors.fuelPrice')
    form.ice.fuel_price = 1.7
    form.ice.purchase_price = 20000
    form.ev.eur_per_kwh = 0
    expect(comparisonStepErrorKey(2, form, false)).toBe('comparison.errors.electricityPrice')
    form.ev.eur_per_kwh = 0.2
    expect(comparisonStepErrorKey(2, form, false)).toBe('comparison.errors.evPrice')
    form.mode = 'RETROSPECTIVE'
    expect(comparisonStepErrorKey(2, form, true)).toBe('')
  })
})

describe('toggleSelection', () => {
  it('adds, removes and caps the selection', () => {
    expect(toggleSelection([], 'a')).toEqual(['a'])
    expect(toggleSelection(['a', 'b'], 'a')).toEqual(['b'])
    expect(toggleSelection(['a', 'b', 'c'], 'd')).toEqual(['a', 'b', 'c'])
    expect(toggleSelection(['a'], 'b', 2)).toEqual(['a', 'b'])
  })
})

describe('costRowValues', () => {
  it('lists the five cost categories of both sides', () => {
    const side = (n: number) => ({ energy: n, maintenance: n + 1, insurance: n + 2, tax: n + 3, depreciation: n + 4 })
    const rows = costRowValues({ ev: side(1), ice: side(10) })
    expect(rows.map((r) => r.key)).toEqual(['energy', 'maintenance', 'insurance', 'tax', 'depreciation'])
    expect(rows[4]).toEqual({ key: 'depreciation', ev: 5, ice: 14 })
    expect(costRowValues(null)).toEqual([])
  })
})
