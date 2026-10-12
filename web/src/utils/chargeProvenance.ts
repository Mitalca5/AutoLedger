export type ChargeOrigin = 'TESLAMATE' | 'WEBHOOK' | 'CSV' | 'MANUAL'

interface ChargeLike {
  origin?: string | null
  is_manual?: boolean
}

const ORIGINS: ChargeOrigin[] = ['TESLAMATE', 'WEBHOOK', 'CSV', 'MANUAL']

/** Where a charge comes from; charges read before `origin` was exposed fall back on `is_manual`. */
export function chargeOrigin(c: ChargeLike): ChargeOrigin {
  const o = String(c.origin ?? '').toUpperCase() as ChargeOrigin
  if (ORIGINS.includes(o)) return o
  return c.is_manual ? 'MANUAL' : 'TESLAMATE'
}

/** Catalog key of the badge naming the source of a charge. */
export function chargeOriginKey(c: ChargeLike): string {
  return `expenses.chargeOrigin.${chargeOrigin(c)}`
}

/**
 * What can be changed on a charge. The facts a synchronised source reports (start, end, energy, place, odometer)
 * are read-only: the next synchronisation would overwrite them. Everything else belongs to AutoLedger.
 * A new charge (null) is fully editable.
 */
export function chargeFieldAccess(c: ChargeLike | null): { sourceLocked: boolean } {
  return { sourceLocked: c !== null && chargeOrigin(c) === 'TESLAMATE' }
}

/** Only charges entered by hand or imported can be deleted: a synchronised one would come back. */
export function chargeDeletable(c: ChargeLike): boolean {
  return chargeOrigin(c) !== 'TESLAMATE'
}
