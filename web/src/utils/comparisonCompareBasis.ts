import { buildComparisonPayload, scenarioToForm } from '@/utils/comparisonForm'
import { breakEvenDelta, type BreakEvenDelta } from '@/utils/comparisonBreakEven'

// A basis applied to every compared scenario so that their totals can be set side by side. Nothing here is saved:
// a null distance or period keeps each scenario's own, and an energy price change of 0 keeps its own prices.
export interface CompareBasis {
  annual_km: number | null
  years: number | null
  energy_price_pct: number
}

export const OWN_BASIS: CompareBasis = { annual_km: null, years: null, energy_price_pct: 0 }

export function isOwnBasis(basis: CompareBasis): boolean {
  return basis.annual_km === null && basis.years === null && Number(basis.energy_price_pct) === 0
}

export const ENERGY_PRICE_PCT_MIN = -50
export const ENERGY_PRICE_PCT_MAX = 200

export interface CompareBasisLimits {
  annual_km_max: number
  years_max: number
}

// Whether a basis can be sent: distance within the API's limit, a whole period, a price change that keeps prices positive.
export function isBasisValid(basis: CompareBasis, limits: CompareBasisLimits): boolean {
  const km = basis.annual_km
  const years = basis.years
  const pct = Number(basis.energy_price_pct)
  return (
    (km === null || (Number.isFinite(km) && km >= 1 && km <= limits.annual_km_max)) &&
    (years === null || (Number.isInteger(years) && years >= 1 && years <= limits.years_max)) &&
    Number.isFinite(pct) &&
    pct >= ENERGY_PRICE_PCT_MIN &&
    pct <= ENERGY_PRICE_PCT_MAX
  )
}

const round6 = (v: number) => Math.round(v * 1e6) / 1e6

// The price per kWh a retrospective scenario is computed with: its override, else the recorded average.
function retrospectiveElectricityPrice(scenario: any, result: any): number {
  return Number(scenario?.options?.electricity_price_override) || Number(result?.tracked_measured?.eur_per_kwh) || 0
}

// Whether the energy price change reaches the electric side of this scenario. A retrospective scenario with no
// recorded price per kWh keeps its electricity as recorded.
export function electricityPriceApplies(scenario: any, result: any): boolean {
  return scenario?.mode !== 'RETROSPECTIVE' || retrospectiveElectricityPrice(scenario, result) > 0
}

// The saved scenario under a basis, as the body of a preview.
export function basisPayload(scenario: any, result: any, basis: CompareBasis) {
  const form = scenarioToForm(scenario, false)
  const factor = 1 + Number(basis.energy_price_pct) / 100
  if (basis.annual_km !== null) form.annual_km = basis.annual_km
  if (basis.years !== null) form.years = basis.years
  if (factor !== 1) {
    form.ice.fuel_price = round6(Number(form.ice.fuel_price) * factor)
    if (scenario?.mode === 'RETROSPECTIVE') {
      const price = retrospectiveElectricityPrice(scenario, result)
      if (price > 0) form.options.electricity_price_override = round6(price * factor)
    } else if (form.tracked) {
      form.tracked.eur_per_kwh = round6(Number(form.tracked.eur_per_kwh) * factor)
    }
  }
  return buildComparisonPayload(form, undefined)
}

export interface AdvantagePoint {
  year: number
  advantage: number
}

// The cumulative outlay of the combustion vehicle minus the electric one, year by year. It crosses zero at the
// break-even, so one curve per scenario is enough to read both the size of the saving and when it starts.
export function cumulativeAdvantage(result: any): AdvantagePoint[] {
  return (result?.cumulative ?? []).map((p: any) => ({ year: Number(p.year), advantage: Number(p.ice) - Number(p.tracked) }))
}

export interface ReferenceGap {
  savings: number
  savingsPct: number | null
  breakEven: BreakEvenDelta
}

// How a scenario's saving and break-even stand against the reference scenario's. The percentage is relative to the
// size of the reference saving, and absent when the reference saves nothing.
export function gapAgainst(reference: any, other: any): ReferenceGap {
  const ref = Number(reference?.tracked_savings) || 0
  const savings = (Number(other?.tracked_savings) || 0) - ref
  const year = (r: any): number | null => (r?.break_even_year === undefined || r?.break_even_year === null ? null : Number(r.break_even_year))
  return {
    savings,
    savingsPct: ref === 0 ? null : (savings / Math.abs(ref)) * 100,
    breakEven: breakEvenDelta(year(reference), year(other)),
  }
}
