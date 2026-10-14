// What the resale value estimate still needs from the ownership of a vehicle, so the dashboard can name the
// missing figures instead of only saying that something is missing.

export type ResidualField = 'purchase_price' | 'resale_value' | 'holding_period' | 'lease_start_value' | 'lease_end_value'

export interface ResidualNeeds {
  /** The vehicle is under a running LOA: its value is read from the contract, not from an owned phase. */
  lease: boolean
  /** A rental (LLD) is never owned, so there is no resale value to estimate. */
  notOwnable: boolean
  fields: ResidualField[]
}

interface OwnershipLike {
  acquisition_type?: string
  start_date?: string
  purchase_price?: number | null
  expected_resale_value?: number | null
  expected_holding_months?: number | null
  lease_duration_months?: number | null
  option_exercised_date?: string | null
  end_date?: string | null
}

const positive = (v: number | null | undefined) => v !== null && v !== undefined && Number(v) > 0

/** A LOA whose contract is still running and whose purchase option has not been exercised. */
export function isRunningLease(o: OwnershipLike | null | undefined, now = new Date()): boolean {
  if (!o || o.acquisition_type !== 'LOA' || !positive(o.lease_duration_months) || !o.start_date || o.option_exercised_date) return false
  const end = new Date(o.start_date)
  end.setMonth(end.getMonth() + Number(o.lease_duration_months))
  if (o.end_date && new Date(o.end_date) <= now) return false
  return now < end
}

export function residualNeeds(o: OwnershipLike | null | undefined, now = new Date()): ResidualNeeds {
  if (!o) return { lease: false, notOwnable: false, fields: ['purchase_price', 'resale_value', 'holding_period'] }
  if (o.acquisition_type === 'LLD') return { lease: false, notOwnable: true, fields: [] }
  if (isRunningLease(o, now)) {
    const fields: ResidualField[] = []
    if (!positive(o.purchase_price)) fields.push('lease_start_value')
    if (!positive(o.expected_resale_value)) fields.push('lease_end_value')
    return { lease: true, notOwnable: false, fields }
  }
  const fields: ResidualField[] = []
  if (!positive(o.purchase_price)) fields.push('purchase_price')
  if (!positive(o.expected_resale_value)) fields.push('resale_value')
  if (!positive(o.expected_holding_months)) fields.push('holding_period')
  return { lease: false, notOwnable: false, fields }
}
