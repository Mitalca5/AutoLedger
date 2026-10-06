import { formatAmount } from '@/currency'
import { distanceUnit, perDistance } from '@/units'

export const NO_VALUE = '—'

/** A cost per km in the account's distance unit, or a dash when there is nothing to divide (no cost recorded, no distance). */
export function formatCostPerDistance(costPerKm: number | null | undefined, currency: string, digits = 3, withUnit = false): string {
  const v = Number(costPerKm)
  if (!Number.isFinite(v) || v <= 0) return NO_VALUE
  const amount = formatAmount(perDistance(v), currency, digits)
  return withUnit ? `${amount}/${distanceUnit()}` : amount
}
