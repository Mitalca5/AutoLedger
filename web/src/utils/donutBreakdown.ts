export type DonutMode = 'full' | 'cash'

export const DONUT_KEYS = ['energy', 'tolls', 'tires', 'maintenance', 'insurance', 'financing', 'depreciation', 'other'] as const

/** Amount of each cost slice: the full economic cost, or only what was actually paid (no wear spreading, no depreciation). */
export function donutAmounts(tco: any, mode: DonutMode): number[] {
  const full = mode === 'full'
  return [
    tco.energy_cost || 0,
    tco.tolls_cost || 0,
    (full ? tco.tires_amortized_cost : tco.tires_cost) || 0,
    (tco.maintenance_cost || 0) + (tco.repair_cost || 0),
    tco.insurance_cost || 0,
    Math.max(0, (full ? tco.financing_full_cost : tco.financing_cost) || 0),
    full ? tco.depreciation_cost || 0 : 0,
    (tco.subscription_cost || 0) + (tco.tax_cost || 0) + (tco.other_cost || 0),
  ]
}
