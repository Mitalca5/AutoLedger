export interface TireWearComparison {
  measured_lifespan_km: number
  expected_lifespan_km: number
  /** Gap of the measured lifespan to the expected one, negative when the tire wears out sooner. */
  delta_pct: number
  reference: 'declared' | 'average'
}

/** A gap beyond this share of the expected lifespan is worth a colour. */
export const NOTICEABLE_LIFESPAN_DELTA_PCT = 20
/** Below this weight the measure leans on the average rate: it is too noisy to colour a verdict. */
export const MIN_CONFIDENCE_FOR_TONE = 0.5

type Stat = { wear_rate_source?: string; wear_rate_confidence?: number; wear_comparison?: TireWearComparison } | null

// Only a gap against the lifespan the owner entered, backed by enough worn tread, is coloured. This describes a
// comparison, not the tire's GOOD/WARNING/CRITICAL condition.
export function tireWearComparisonTone(value: TireWearComparison, confidence: number | undefined): string {
  if (value.reference !== 'declared' || !(Number(confidence) >= MIN_CONFIDENCE_FOR_TONE)) return 'text-slate-300'
  if (value.delta_pct < -NOTICEABLE_LIFESPAN_DELTA_PCT) return 'text-warning-400'
  if (value.delta_pct > NOTICEABLE_LIFESPAN_DELTA_PCT) return 'text-success-400'
  return 'text-slate-300'
}

export function tireWearComparison(stat: Stat): TireWearComparison | null {
  const value = stat?.wear_comparison
  if (stat?.wear_rate_source !== 'measured' || !value) return null
  const lifespans = [value.measured_lifespan_km, value.expected_lifespan_km]
  return lifespans.every((n) => Number.isFinite(n) && n > 0) && Number.isFinite(value.delta_pct) && ['declared', 'average'].includes(value.reference) ? value : null
}
