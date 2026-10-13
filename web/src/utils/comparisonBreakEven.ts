// Why a break-even is missing, as sent by the API (break_even_reason).
const REASONS = ['period_too_short', 'no_saving', 'reversed'] as const

export type BreakEvenReason = (typeof REASONS)[number]

// The message key of a reason, or null for an absent or unknown one (results saved by an older server).
export function breakEvenReasonKey(reason: unknown): string | null {
  return REASONS.includes(reason as BreakEvenReason) ? `comparison.breakEven.reason.${reason}` : null
}

export type BreakEvenDelta = { kind: 'none' } | { kind: 'years'; years: number } | { kind: 'reached' } | { kind: 'lost' }

// How the explored break-even moved against the original one. A missing break-even has no number to subtract.
export function breakEvenDelta(original: number | null, explored: number | null): BreakEvenDelta {
  if (original === null && explored === null) return { kind: 'none' }
  if (original === null) return { kind: 'reached' }
  if (explored === null) return { kind: 'lost' }
  return { kind: 'years', years: Math.round((explored - original) * 10) / 10 }
}
