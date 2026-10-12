import { buildComparisonPayload, scenarioToForm, type ComparisonForm } from '@/utils/comparisonForm'
import { comparisonSavings } from '@/utils/comparisonSavings'

// The what-if section of a projection result: three temporary inputs, in the API's units (km per year, price per
// litre, price per kWh). They never replace the saved scenario.
export interface WhatIfInputs {
  annual_km: number
  fuel_price: number
  eur_per_kwh: number
}

export interface WhatIfBound {
  min: number
  max: number
  step: number
}

export type WhatIfKey = keyof WhatIfInputs

export const WHAT_IF_KEYS: WhatIfKey[] = ['annual_km', 'fuel_price', 'eur_per_kwh']

// The sliders span half to twice the saved value; the field next to each slider accepts anything the API accepts.
const SPAN = { low: 0.5, high: 2 }
const STEPS: Record<WhatIfKey, number> = { annual_km: 500, fuel_price: 0.01, eur_per_kwh: 0.01 }
// Limits travel with the computed result; the browser never guesses a currency-sized ceiling.
export interface WhatIfInputLimits {
  annual_km_max: number
  fuel_price_max: number
  electricity_price_max: number
}

// What the API accepts for each field: the limits of the text fields next to the sliders.
export function whatIfRanges(limits: WhatIfInputLimits): Record<WhatIfKey, { min: number; max: number }> {
  return {
    annual_km: { min: 1, max: limits.annual_km_max },
    fuel_price: { min: 0, max: limits.fuel_price_max },
    eur_per_kwh: { min: 0, max: limits.electricity_price_max },
  }
}

export function whatIfBounds(base: WhatIfInputs, limits: WhatIfInputLimits): Record<WhatIfKey, WhatIfBound> {
  const out = {} as Record<WhatIfKey, WhatIfBound>
  for (const key of WHAT_IF_KEYS) {
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

export function whatIfBase(scenario: any): WhatIfInputs {
  return {
    annual_km: Number(scenario?.annual_km) || 0,
    fuel_price: Number(scenario?.ice?.fuel_price) || 0,
    eur_per_kwh: Number(scenario?.tracked?.eur_per_kwh) || 0,
  }
}

export function isWhatIfDirty(base: WhatIfInputs, explored: WhatIfInputs): boolean {
  return WHAT_IF_KEYS.some((k) => Number(explored[k]) !== Number(base[k]))
}

// The explored values are in range for the API; anything else is not sent.
export function isWhatIfValid(explored: WhatIfInputs, limits: WhatIfInputLimits | undefined): boolean {
  if (!limits) return false
  const ranges = whatIfRanges(limits)
  return WHAT_IF_KEYS.every((k) => {
    const v = Number(explored[k])
    return Number.isFinite(v) && v >= ranges[k].min && v <= ranges[k].max
  })
}

// The saved scenario with the explored values: the one input of the preview and of "Save as a new comparison".
export function whatIfForm(scenario: any, explored: WhatIfInputs): ComparisonForm {
  const form = scenarioToForm(scenario, false)
  form.annual_km = Number(explored.annual_km)
  form.ice.fuel_price = Number(explored.fuel_price)
  if (form.tracked) form.tracked.eur_per_kwh = Number(explored.eur_per_kwh)
  return form
}

export function whatIfPayload(scenario: any, explored: WhatIfInputs) {
  return buildComparisonPayload(whatIfForm(scenario, explored), undefined)
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
