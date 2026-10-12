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

const DONUT_COLORS: Record<(typeof DONUT_KEYS)[number], string> = {
  energy: '#38bdf8',
  tolls: '#f59e0b',
  tires: '#10b981',
  maintenance: '#ec4899',
  insurance: '#a855f7',
  financing: '#f97316',
  depreciation: '#e11d48',
  other: '#64748b',
}

/** The slices with an amount, largest first, each with its share of the total shown. */
export function donutRows(tco: any, mode: DonutMode) {
  const amounts = donutAmounts(tco || {}, mode)
  const rows = DONUT_KEYS.map((key, i) => ({ key, color: DONUT_COLORS[key], amount: amounts[i] })).filter((r) => r.amount > 0.005)
  const total = rows.reduce((sum, r) => sum + r.amount, 0)
  return rows.map((r) => ({ ...r, sharePct: total > 0 ? (r.amount / total) * 100 : 0 })).sort((a, b) => b.amount - a.amount)
}
