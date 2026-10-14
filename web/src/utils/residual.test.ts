import { describe, expect, it } from 'vitest'
import { isRunningLease, residualNeeds } from './residual'

const now = new Date('2026-10-14T12:00:00Z')
const loa = { acquisition_type: 'LOA', start_date: '2023-11-16', lease_duration_months: 60 }

describe('residualNeeds', () => {
  it('asks for the three owned figures when there is no ownership', () => {
    expect(residualNeeds(null, now)).toEqual({ lease: false, notOwnable: false, fields: ['purchase_price', 'resale_value', 'holding_period'] })
  })

  it('names the contract values of a running LOA, not the owned-phase ones', () => {
    expect(residualNeeds(loa, now)).toEqual({ lease: true, notOwnable: false, fields: ['lease_start_value', 'lease_end_value'] })
    expect(residualNeeds({ ...loa, purchase_price: 45000 }, now).fields).toEqual(['lease_end_value'])
    expect(residualNeeds({ ...loa, purchase_price: 45000, expected_resale_value: 24000 }, now).fields).toEqual([])
  })

  it('reads a LOA whose option was exercised as an owned vehicle', () => {
    const exercised = { ...loa, option_exercised_date: '2026-06-01', purchase_price: 20000 }
    expect(residualNeeds(exercised, now)).toEqual({ lease: false, notOwnable: false, fields: ['resale_value', 'holding_period'] })
  })

  it('reads a LOA past its end as not leased any more', () => {
    expect(isRunningLease({ ...loa, lease_duration_months: 12 }, now)).toBe(false)
    expect(isRunningLease({ ...loa, end_date: '2026-01-01' }, now)).toBe(false)
  })

  it('has no resale value to estimate for a rental', () => {
    expect(residualNeeds({ acquisition_type: 'LLD', start_date: '2025-01-01', lease_duration_months: 36 }, now)).toEqual({ lease: false, notOwnable: true, fields: [] })
  })

  it('needs only the owned figures that are missing for a cash purchase', () => {
    expect(residualNeeds({ acquisition_type: 'CASH', purchase_price: 40000, expected_resale_value: 20000 }, now).fields).toEqual(['holding_period'])
  })
})
