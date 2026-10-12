import { describe, expect, it } from 'vitest'
import { donutAmounts, donutRows } from './donutBreakdown'

const tco = {
  energy_cost: 100, tolls_cost: 10, tires_cost: 400, tires_amortized_cost: 150, maintenance_cost: 50, repair_cost: 20,
  insurance_cost: 300, financing_cost: 900, financing_full_cost: 1000, depreciation_cost: 500,
  subscription_cost: 5, tax_cost: 6, other_cost: 7,
}

describe('donutAmounts', () => {
  it('full cost uses the amortized tires, the full financing and the depreciation', () => {
    expect(donutAmounts(tco, 'full')).toEqual([100, 10, 150, 70, 300, 1000, 500, 18])
  })

  it('cash keeps what was paid: tires at purchase, cash financing, no depreciation', () => {
    expect(donutAmounts(tco, 'cash')).toEqual([100, 10, 400, 70, 300, 900, 0, 18])
  })

  it('tolerates missing fields', () => {
    expect(donutAmounts({}, 'cash')).toEqual([0, 0, 0, 0, 0, 0, 0, 0])
  })
})

describe('donutRows', () => {
  it('lists the slices with an amount, largest first, with their share', () => {
    const rows = donutRows(tco, 'full')
    expect(rows.map((r) => r.key)).toEqual(['financing', 'depreciation', 'insurance', 'tires', 'energy', 'maintenance', 'other', 'tolls'])
    expect(rows[0].sharePct).toBeCloseTo((1000 / 2148) * 100, 5)
    expect(rows.reduce((sum, r) => sum + r.sharePct, 0)).toBeCloseTo(100, 5)
  })

  it('leaves the depreciation out of the cash view', () => {
    expect(donutRows(tco, 'cash').some((r) => r.key === 'depreciation')).toBe(false)
  })

  it('is empty without any cost', () => {
    expect(donutRows({}, 'cash')).toEqual([])
    expect(donutRows(null, 'full')).toEqual([])
  })
})
