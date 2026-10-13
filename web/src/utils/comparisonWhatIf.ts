import { buildComparisonPayload, scenarioToForm, type ComparisonForm } from '@/utils/comparisonForm'
import { comparisonSavings } from '@/utils/comparisonSavings'

// The what-if section of a projection result: temporary inputs, in the API's units (km per year, price per litre,
// price per kWh, years, amounts in the scenario's currency, yearly inflation in percent). They never replace the
// saved scenario.
export interface WhatIfInputs {
  annual_km: number
  fuel_price: number
  eur_per_kwh: number
  years: number
  ice_purchase_price: number
  tracked_purchase_price: number
  ice_resale_value: number
  tracked_resale_value: number
  fuel_inflation_pct: number
  electricity_inflation_pct: number
  cost_inflation_pct: number
}

export interface WhatIfBound {
  min: number
  max: number
  step: number
}

export type WhatIfKey = keyof WhatIfInputs

// The inputs that come with a slider, the others only have a field.
export type WhatIfSliderKey = 'annual_km' | 'fuel_price' | 'eur_per_kwh'

export const WHAT_IF_SLIDER_KEYS: WhatIfSliderKey[] = ['annual_km', 'fuel_price', 'eur_per_kwh']
export const WHAT_IF_KEYS: WhatIfKey[] = [
  ...WHAT_IF_SLIDER_KEYS,
  'years',
  'ice_purchase_price',
  'tracked_purchase_price',
  'ice_resale_value',
  'tracked_resale_value',
  'fuel_inflation_pct',
  'electricity_inflation_pct',
  'cost_inflation_pct',
]

// The sliders span half to twice the saved value; the field next to each slider accepts anything the API accepts.
const SPAN = { low: 0.5, high: 2 }
const STEPS: Record<WhatIfSliderKey, number> = { annual_km: 500, fuel_price: 0.01, eur_per_kwh: 0.01 }
// Limits travel with the computed result; the browser never guesses a currency-sized ceiling.
export interface WhatIfInputLimits {
  annual_km_max: number
  fuel_price_max: number
  electricity_price_max: number
  years_max?: number
  amount_max?: number
  inflation_min?: number
  inflation_max?: number
}

// Whether the API sent the bounds of the inputs without a slider (a result computed by an older server has none).
export function hasExtendedLimits(limits: WhatIfInputLimits | undefined): boolean {
  return !!limits && limits.years_max !== undefined && limits.amount_max !== undefined && limits.inflation_min !== undefined && limits.inflation_max !== undefined
}

// What the API accepts for each field: the limits of the text fields next to the sliders.
export function whatIfRanges(limits: WhatIfInputLimits): Record<WhatIfKey, { min: number; max: number }> {
  const amount = { min: 0, max: limits.amount_max ?? 0 }
  const inflation = { min: limits.inflation_min ?? 0, max: limits.inflation_max ?? 0 }
  return {
    annual_km: { min: 1, max: limits.annual_km_max },
    fuel_price: { min: 0, max: limits.fuel_price_max },
    eur_per_kwh: { min: 0, max: limits.electricity_price_max },
    years: { min: 1, max: limits.years_max ?? 1 },
    ice_purchase_price: amount,
    tracked_purchase_price: amount,
    ice_resale_value: amount,
    tracked_resale_value: amount,
    fuel_inflation_pct: inflation,
    electricity_inflation_pct: inflation,
    cost_inflation_pct: inflation,
  }
}

export function whatIfBounds(base: WhatIfInputs, limits: WhatIfInputLimits): Record<WhatIfSliderKey, WhatIfBound> {
  const out = {} as Record<WhatIfSliderKey, WhatIfBound>
  for (const key of WHAT_IF_SLIDER_KEYS) {
    const step = STEPS[key]
    const { min: floor, max: ceiling } = whatIfRanges(limits)[key]
    const value = Number(base[key]) || 0
    const ceilingOnStep = Math.floor(ceiling / step) * step
    // Round outwards to the step so that the saved value always sits on the track
    const min = Math.min(ceilingOnStep, Math.max(floor, Math.floor((value * SPAN.low) / step) * step))
    const max = Math.min(ceilingOnStep, Math.max(Math.ceil((value * SPAN.high) / step) * step, min + step))
    out[key] = { min: round(min, step), max: round(max, step), step }
  }
  return out
}

function round(v: number, step: number): number {
  const decimals = step < 1 ? Math.ceil(-Math.log10(step)) : 0
  return Number(v.toFixed(decimals))
}

export const isRetrospective = (scenario: any): boolean => scenario?.mode === 'RETROSPECTIVE'

// The price per kWh recorded on the tracked vehicle, from the result of a retrospective comparison; 0 when unknown.
export function measuredElectricityPrice(result: any): number {
  return Number(result?.tracked_measured?.eur_per_kwh) || 0
}

// `result` supplies the recorded price of a retrospective scenario, whose tracked side has no price of its own.
export function whatIfBase(scenario: any, result?: any): WhatIfInputs {
  const n = (v: unknown) => Number(v) || 0
  const retro = isRetrospective(scenario)
  return {
    annual_km: n(scenario?.annual_km),
    fuel_price: n(scenario?.ice?.fuel_price),
    eur_per_kwh: retro ? n(scenario?.options?.electricity_price_override) || measuredElectricityPrice(result) : n(scenario?.tracked?.eur_per_kwh),
    years: n(scenario?.years),
    ice_purchase_price: n(scenario?.ice?.purchase_price),
    tracked_purchase_price: n(scenario?.tracked?.purchase_price),
    ice_resale_value: n(scenario?.ice?.resale_value),
    tracked_resale_value: n(scenario?.tracked?.resale_value),
    fuel_inflation_pct: n(scenario?.options?.fuel_inflation_pct),
    electricity_inflation_pct: n(scenario?.options?.electricity_inflation_pct),
    cost_inflation_pct: n(scenario?.options?.cost_inflation_pct),
  }
}

export function isWhatIfDirty(base: WhatIfInputs, explored: WhatIfInputs): boolean {
  return WHAT_IF_KEYS.some((k) => Number(explored[k]) !== Number(base[k]))
}

// The explored values are in range for the API; anything else is not sent. Without the extended limits only the
// three slider inputs are checked, and the others are not editable.
export function isWhatIfValid(explored: WhatIfInputs, limits: WhatIfInputLimits | undefined): boolean {
  if (!limits) return false
  const ranges = whatIfRanges(limits)
  const keys = hasExtendedLimits(limits) ? WHAT_IF_KEYS : WHAT_IF_SLIDER_KEYS
  return keys.every((k) => {
    const v = Number(explored[k])
    return Number.isFinite(v) && v >= ranges[k].min && v <= ranges[k].max && (k !== 'years' || Number.isInteger(v))
  })
}

// The saved scenario with the explored values: the one input of the preview and of "Save as a new comparison".
// `measuredPrice` is the recorded price per kWh of a retrospective scenario: an explored price equal to it is not an
// override, so the scenario keeps following the data.
export function whatIfForm(scenario: any, explored: WhatIfInputs, measuredPrice = 0): ComparisonForm {
  const form = scenarioToForm(scenario, false)
  form.annual_km = Number(explored.annual_km)
  form.years = Number(explored.years)
  form.ice.fuel_price = Number(explored.fuel_price)
  form.ice.purchase_price = Number(explored.ice_purchase_price)
  form.ice.resale_value = Number(explored.ice_resale_value)
  form.options.fuel_inflation_pct = Number(explored.fuel_inflation_pct)
  form.options.electricity_inflation_pct = Number(explored.electricity_inflation_pct)
  form.options.cost_inflation_pct = Number(explored.cost_inflation_pct)
  if (isRetrospective(scenario)) {
    const price = Number(explored.eur_per_kwh)
    form.options.electricity_price_override = price > 0 && price !== measuredPrice ? price : 0
  } else if (form.tracked) {
    form.tracked.eur_per_kwh = Number(explored.eur_per_kwh)
    form.tracked.purchase_price = Number(explored.tracked_purchase_price)
    form.tracked.resale_value = Number(explored.tracked_resale_value)
  }
  return form
}

export function whatIfPayload(scenario: any, explored: WhatIfInputs, measuredPrice = 0) {
  return buildComparisonPayload(whatIfForm(scenario, explored, measuredPrice), undefined)
}

export interface WhatIfFigures {
  energySavings: number
  totalSavings: number
  // Years until the cumulated outlay of the electric vehicle falls below the combustion one, null when never
  breakEvenYear: number | null
  breakEvenNetYear: number | null
}

// Energy savings, full-cost savings and break-even are three different figures and are never merged.
export function whatIfFigures(result: any): WhatIfFigures | null {
  if (!result?.tracked || !result?.ice) return null
  const years = result.years_count
  const km = result.annual_km
  return {
    energySavings: comparisonSavings(result.tracked.energy, result.ice.energy, years, km).amount,
    totalSavings: comparisonSavings(result.tracked.total, result.ice.total, years, km).amount,
    breakEvenYear: result.break_even_year ?? null,
    breakEvenNetYear: result.break_even_net_year ?? null,
  }
}
