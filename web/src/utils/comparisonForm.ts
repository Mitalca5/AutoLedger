export type ComparisonMode = 'RETROSPECTIVE' | 'PROJECTION'

export const MAX_COMPARE = 3

const COST_KEYS = ['purchase_price', 'resale_value', 'maintenance_yearly', 'insurance_yearly', 'tax_yearly'] as const

export function emptyComparisonForm(canCompareTracked: boolean) {
  return {
    mode: (canCompareTracked ? 'RETROSPECTIVE' : 'PROJECTION') as ComparisonMode,
    name: '',
    annual_km: 12000,
    years: 5,
    ice: {
      fuel_type: 'SP95_E10',
      l_per_100km: 6.5,
      fuel_price: 1.75,
      purchase_price: 0,
      resale_value: 0,
      maintenance_yearly: 700,
      insurance_yearly: 650,
      tax_yearly: 0,
    },
    options: {
      fuel_inflation_pct: 0,
      electricity_inflation_pct: 0,
      cost_inflation_pct: 0,
      ev_incentives: 0,
    },
    ev: {
      kwh_per_100km: 16,
      eur_per_kwh: 0.2,
      purchase_price: 0,
      resale_value: 0,
      maintenance_yearly: 300,
      insurance_yearly: 800,
      tax_yearly: 0,
    },
  }
}

export type ComparisonForm = ReturnType<typeof emptyComparisonForm>

export interface ComparisonDefaults {
  annual_km: number
  maintenance_yearly: number
  insurance_yearly: number
  ev_kwh_per_100km?: number
  ev_eur_per_kwh?: number
  ice_l_per_100km?: number
  ice_fuel_price?: number
  indicative_prices?: boolean
  ice?: { fuel_type: string; l_per_100km: number; fuel_price: number }[]
}

// Fuel-type defaults first, then the figures measured on the tracked vehicle, which win.
export function applyComparisonDefaults(form: ComparisonForm, d: ComparisonDefaults | null | undefined) {
  if (!d) return
  form.annual_km = d.annual_km
  form.ice.maintenance_yearly = d.maintenance_yearly
  form.ice.insurance_yearly = d.insurance_yearly
  // Prices written for another currency are not shown as the vehicle's own: the user types them
  if (d.indicative_prices === false) form.ev.eur_per_kwh = 0
  if (d.ev_kwh_per_100km) form.ev.kwh_per_100km = d.ev_kwh_per_100km
  if (d.ev_eur_per_kwh) form.ev.eur_per_kwh = d.ev_eur_per_kwh
  const fuel = d.ice?.find((f) => f.fuel_type === form.ice.fuel_type)
  if (fuel) {
    form.ice.l_per_100km = fuel.l_per_100km
    form.ice.fuel_price = fuel.fuel_price
  }
  if (d.ice_l_per_100km) form.ice.l_per_100km = d.ice_l_per_100km
  if (d.ice_fuel_price) form.ice.fuel_price = d.ice_fuel_price
}

export function scenarioToForm(sc: any, canCompareTracked: boolean): ComparisonForm {
  const base = emptyComparisonForm(canCompareTracked)
  return {
    ...base,
    mode: sc.mode,
    name: sc.name,
    annual_km: sc.annual_km,
    years: sc.years,
    ice: { ...sc.ice },
    ev: sc.ev ? { ...sc.ev } : base.ev,
    options: { ...base.options, ...(sc.options || {}) },
  }
}

export function hasAdvancedOptions(form: ComparisonForm): boolean {
  return Object.values(form.options).some((v) => Number(v) !== 0)
}

export function buildComparisonPayload(form: ComparisonForm, vehicleId: string | undefined) {
  const retro = form.mode === 'RETROSPECTIVE'
  const payload: any = {
    name: form.name,
    mode: form.mode,
    annual_km: Number(form.annual_km),
    years: Number(form.years),
    ice: {
      ...form.ice,
      l_per_100km: Number(form.ice.l_per_100km),
      fuel_price: Number(form.ice.fuel_price),
    },
    options: {
      fuel_inflation_pct: Number(form.options.fuel_inflation_pct) || 0,
      electricity_inflation_pct: Number(form.options.electricity_inflation_pct) || 0,
      cost_inflation_pct: Number(form.options.cost_inflation_pct) || 0,
      // Incentives only apply when the electric vehicle is described by the user
      ev_incentives: retro ? 0 : Number(form.options.ev_incentives) || 0,
    },
  }
  if (retro) {
    payload.vehicle_id = vehicleId
  } else {
    payload.ev = {
      ...form.ev,
      kwh_per_100km: Number(form.ev.kwh_per_100km),
      eur_per_kwh: Number(form.ev.eur_per_kwh),
    }
  }
  for (const side of [payload.ice, payload.ev]) {
    if (!side) continue
    for (const k of COST_KEYS) side[k] = Number(side[k] || 0)
  }
  return payload
}

// Catalog key of the first problem on a wizard step, or '' when the step is valid.
export function comparisonStepErrorKey(step: number, form: ComparisonForm, canCompareTracked: boolean): string {
  const retro = form.mode === 'RETROSPECTIVE'
  if (step === 1) {
    if (!form.name.trim()) return 'comparison.errors.name'
    if (!(Number(form.annual_km) > 0)) return 'comparison.errors.annualKm'
    if (!(Number(form.years) >= 1 && Number(form.years) <= 15)) return 'comparison.errors.years'
    if (retro && !canCompareTracked) return 'comparison.errors.trackedNeeded'
  }
  if (step === 2) {
    if (!(Number(form.ice.l_per_100km) > 0)) return 'comparison.errors.iceConsumption'
    if (!(Number(form.ice.fuel_price) > 0)) return 'comparison.errors.fuelPrice'
    if (!(Number(form.ice.purchase_price) > 0)) return 'comparison.errors.icePrice'
    if (!retro && !(Number(form.ev.eur_per_kwh) > 0)) return 'comparison.errors.electricityPrice'
    if (!retro && !(Number(form.ev.purchase_price) > 0)) return 'comparison.errors.evPrice'
  }
  return ''
}

export function toggleSelection(selected: string[], id: string, max = MAX_COMPARE): string[] {
  if (selected.includes(id)) return selected.filter((s) => s !== id)
  return selected.length < max ? [...selected, id] : selected
}

export function costRowValues(result: any): { key: string; ev: number; ice: number }[] {
  if (!result) return []
  return ['energy', 'maintenance', 'insurance', 'tax', 'depreciation'].map((key) => ({ key, ev: result.ev[key], ice: result.ice[key] }))
}
