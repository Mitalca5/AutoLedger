export interface TireWearComparison {
  measured_rate_per_10k_km: number
  expected_rate_per_10k_km: number
  measured_lifespan_km: number
  expected_lifespan_km: number
  delta_pct: number
  reference: 'declared' | 'average'
}

export const NOTICEABLE_WEAR_DELTA_PCT = 25

// This badge describes a relative measurement, not the tire's legal/safety condition.
export function tireWearComparisonTone(delta: number): string {
  if (delta > NOTICEABLE_WEAR_DELTA_PCT) return 'text-warning-400'
  if (delta < -NOTICEABLE_WEAR_DELTA_PCT) return 'text-success-400'
  return 'text-slate-300'
}

export function tireWearComparison(stat: { wear_rate_source?: string; wear_comparison?: TireWearComparison } | null): TireWearComparison | null {
  const value = stat?.wear_comparison
  if (stat?.wear_rate_source !== 'measured' || !value) return null
  const values = [value.measured_rate_per_10k_km, value.expected_rate_per_10k_km, value.measured_lifespan_km, value.expected_lifespan_km]
  return values.every(n => Number.isFinite(n) && n > 0) && Number.isFinite(value.delta_pct) && ['declared', 'average'].includes(value.reference) ? value : null
}
