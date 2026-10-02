import { intlLocale } from '@/i18n'
import { formatAmount } from '@/currency'
import { perDistance } from '@/units'
// Shape of GET /api/vehicles/{id}/energy-stats, shared by the dashboard sections.

export interface EnergyMonth {
  month: string
  distance_km: number
  consumption_kwh_100km?: number
  price_per_kwh?: number
  cost_per_100km?: number
  cost_per_100km_trailing?: number
  kwh_added: number
  energy_cost: number
  estimated_capacity_kwh?: number
  capacity_samples?: number
}

export interface ChargeClass {
  class: 'SLOW' | 'AC' | 'DC' | 'UNKNOWN'
  sessions: number
  kwh_added: number
  energy_cost: number
  price_per_kwh?: number
  charge_efficiency?: number
  cost_per_full_charge?: number
}

export interface TemperatureBin {
  min_c: number
  max_c: number
  drives: number
  distance_km: number
  consumption_kwh_100km: number
}

export interface BatterySnapshot {
  date: string // YYYY-MM-DD
  max_capacity_kwh?: number
  current_capacity_kwh?: number
  health_percent?: number
}

// How a section's figures were obtained: read from drives or charges, derived from odometer readings, or missing
export type EnergyBasisKind = 'measured' | 'derived' | 'unavailable'

export interface EnergyBasis {
  consumption: EnergyBasisKind
  cost: EnergyBasisKind
  charging: EnergyBasisKind
  full_charge: EnergyBasisKind
  temperature: EnergyBasisKind
  battery: EnergyBasisKind
}

export interface EnergyStats {
  months: EnergyMonth[]
  charge_classes: ChargeClass[]
  summary: {
    consumption_kwh_100km?: number
    charge_efficiency?: number
    price_per_kwh?: number
    cost_per_100km?: number
    sessions_without_cost: number
    estimated_capacity_kwh?: number
    capacity_samples?: number
    cost_per_full_charge?: number
  }
  temperature_bins: TemperatureBin[]
  temperature: {
    cold_consumption_kwh_100km?: number
    mild_consumption_kwh_100km?: number
    extra_percent?: number
    extra_cost_per_100km?: number
  }
  battery_health: BatterySnapshot[]
  basis?: EnergyBasis
}

function round1(v: number) {
  return Math.round(v * 10) / 10
}

function weightedAvg(entries: Array<[number | undefined, number]>): number | undefined {
  let sum = 0
  let weight = 0
  for (const [v, w] of entries) {
    if (v === undefined || w <= 0) continue
    sum += v * w
    weight += w
  }
  return weight > 0 ? sum / weight : undefined
}

function mergeTwo(a: ChargeClass | undefined, b: ChargeClass | undefined, cls: 'AC' | 'DC'): ChargeClass | undefined {
  if (!a && !b) return undefined
  const sessions = (a?.sessions ?? 0) + (b?.sessions ?? 0)
  const kwh_added = round1((a?.kwh_added ?? 0) + (b?.kwh_added ?? 0))
  const energy_cost = (a?.energy_cost ?? 0) + (b?.energy_cost ?? 0)
  return {
    class: cls,
    sessions,
    kwh_added,
    energy_cost,
    price_per_kwh: kwh_added > 0 ? energy_cost / kwh_added : undefined,
    charge_efficiency: weightedAvg([
      [a?.charge_efficiency, a?.kwh_added ?? 0],
      [b?.charge_efficiency, b?.kwh_added ?? 0],
    ]),
    cost_per_full_charge: weightedAvg([
      [a?.cost_per_full_charge, a?.sessions ?? 0],
      [b?.cost_per_full_charge, b?.sessions ?? 0],
    ]),
  }
}

/**
 * Groups the charging sessions as AC (domestic socket and wallbox: both draw AC current, only the power differs) and
 * DC (fast charging). Sessions with no usable duration (UNKNOWN) cannot be classified by power and are counted
 * separately instead of shown as a class of their own.
 */
export function mergeAcDcClasses(classes: ChargeClass[]): { classes: ChargeClass[]; unknownSessions: number } {
  const byClass = Object.fromEntries(classes.map((c) => [c.class, c])) as Partial<Record<ChargeClass['class'], ChargeClass>>
  const merged = [mergeTwo(byClass.SLOW, byClass.AC, 'AC'), byClass.DC ? { ...byClass.DC } : undefined].filter((c): c is ChargeClass => !!c)
  return { classes: merged, unknownSessions: byClass.UNKNOWN?.sessions ?? 0 }
}

export const fmt = (v: number | undefined, digits: number) =>
  v === undefined ? '—' : v.toLocaleString(intlLocale(), { minimumFractionDigits: digits, maximumFractionDigits: digits })

// Same '—' for a missing figure, as an amount in the vehicle's currency
export const fmtMoney = (v: number | undefined, currency: string, digits = 2) =>
  v === undefined ? '—' : formatAmount(v, currency, digits)

// A figure per 100 km from the API, per 100 of the account's distance unit
export const perUnit = (v: number | undefined | null): number | undefined => (v === undefined || v === null ? undefined : perDistance(v))

export const fmtPercent = (v: number | undefined) =>
  v === undefined ? '—' : `${(v * 100).toLocaleString(intlLocale(), { maximumFractionDigits: 0 })} %`

export const AXIS_TEXT = '#94a3b8'
export const GRID_COLOR = '#1e293b'
